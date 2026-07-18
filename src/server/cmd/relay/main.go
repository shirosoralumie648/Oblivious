package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"oblivious/server/internal/admin"
	"oblivious/server/internal/buildinfo"
	"oblivious/server/internal/config"
	"oblivious/server/internal/db"
	"oblivious/server/internal/memory"
	"oblivious/server/internal/migrations"
	"oblivious/server/internal/observability"
	"oblivious/server/internal/quota"
	"oblivious/server/internal/relay"
	relaycache "oblivious/server/internal/relay/cache"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/ratelimit"
	"oblivious/server/internal/relay/types"
	"oblivious/server/internal/releasecontract"
	"oblivious/server/internal/usage"
)

type relayBatchPollingWorkerRunner interface {
	Run(context.Context)
}

type relayBatchPollingWorkerFactory func(relay.BatchPollingWorkerStore, relay.BatchStatusClient, relay.BatchPollingWorkerConfig) relayBatchPollingWorkerRunner

func main() {
	if err := config.RunEntrypoint(context.Background(), releasecontract.EntrypointID("relay"), config.EntrypointPreflightOptions{
		RepoRoot: "/app", ContractPath: "config/release/contract.v1.json", SchemaPath: "config/release/contract.schema.json",
		ProfileID: os.Getenv("OBLIVIOUS_DEPLOYMENT_PROFILE"), Contracts: config.FileContractLoader{},
		Identities: buildinfo.NewEmbeddedProvider(), Profiles: releasecontract.NewFileProfileResolver(),
	}, func(context.Context, config.ResolvedEntrypointInputs) error {
		cfg, err := config.Load()
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
		if !cfg.RelayEnabled {
			log.Fatal("Relay service requires RELAY_ENABLED=true")
		}

		databaseURL := relayDatabaseURL(cfg)
		database, err := db.Open(databaseURL)
		if err != nil {
			log.Fatalf("open relay database: %v", err)
		}
		defer database.Close()

		migrationCtx, cancelMigrations := context.WithTimeout(context.Background(), 10*time.Minute)
		migrationResult, err := migrations.Apply(migrationCtx, database, "migrations")
		cancelMigrations()
		if err != nil {
			log.Fatalf("apply relay database migrations: %v", err)
		}
		log.Printf("relay migrations ready: applied=%d skipped=%d", migrationResult.Applied, migrationResult.Skipped)

		gin.SetMode(gin.ReleaseMode)

		relayStore := relay.NewRelayStore(database)
		pool := relay.NewChannelPool()
		if err := relayStore.LoadPoolFromStore(pool); err != nil {
			log.Fatalf("load relay channels from database: %v", err)
		}
		ensureDefaultChannel(relayStore, pool, cfg)

		pricingStore := loadRelayPricingStore(cfg, database)

		apiTokenStore := relay.NewRelayAPITokenSQLStore(database)
		apiTokenAuthenticator := relay.NewAPITokenAuthenticator(apiTokenStore)
		rateLimiter, closeRateLimiter := buildRelayRateLimiter(cfg)
		if closeRateLimiter != nil {
			defer func() {
				if err := closeRateLimiter(); err != nil {
					log.Printf("warning: failed to close relay rate limiter: %v", err)
				}
			}()
		}

		relayCfg := buildStandaloneRelayConfig(
			cfg,
			pool,
			pricingStore,
			apiTokenAuthenticator,
			rateLimiter,
			relayStore,
			observability.NewSQLAlertStateStore(database),
		)
		applyRelaySemanticCacheConfig(relayCfg, cfg, database)

		r, err := relay.NewRelay(relayCfg)
		if err != nil {
			log.Fatalf("Failed to create relay: %v", err)
		}

		quotaStore := quota.NewSQLStore(database)
		quotaService := quota.NewService(quotaStore)
		r.Router().SetQuotaManager(quotaService)
		r.Router().SetAPITokenQuotaManager(apiTokenStore)
		r.Router().SetUsageLogger(usage.NewSQLRecorder(database))
		r.Router().SetRateLimitResolver(buildRelayUsageLimitResolver(quotaService))

		relayHealthCtx, cancelHealthChecks := context.WithCancel(context.Background())
		defer cancelHealthChecks()
		r.StartHealthChecks(relayHealthCtx)
		batchFinalizer := relay.NewBatchUsageFinalizer(usage.NewSQLRecorder(database), relay.BatchUsageFinalizerConfig{
			PricingStore:         pricingStore,
			QuotaManager:         quotaService,
			APITokenQuotaManager: apiTokenStore,
		})
		cancelBatchPollingWorker, batchPollingWorkerStarted := startStandaloneRelayBatchPollingWorker(
			cfg,
			relayStore,
			relay.NewOpenAIBatchStatusClient(relayInstanceDefaultAdapter(pool)),
			batchFinalizer,
			batchFinalizer,
			nil,
		)
		if batchPollingWorkerStarted {
			defer cancelBatchPollingWorker()
		}

		srv := &http.Server{
			Addr:    fmt.Sprintf(":%d", cfg.Port),
			Handler: r.Engine(),
		}

		go func() {
			log.Printf("Relay service listening on port %d env=%s dbMode=%s", cfg.Port, cfg.Env, cfg.DBMode)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("Server error: %v", err)
			}
		}()

		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cancelHealthChecks()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
		log.Println("Relay service stopped")
		return nil
	}); err != nil {
		log.Fatalf("relay preflight failed: %v", err)
	}
}

func startStandaloneRelayBatchPollingWorker(
	cfg config.Config,
	store relay.BatchPollingWorkerStore,
	client relay.BatchStatusClient,
	finalizer relay.BatchCompletionFinalizer,
	failureFinalizer relay.BatchFailureFinalizer,
	factory relayBatchPollingWorkerFactory,
) (func(), bool) {
	if !cfg.RelayBatchPollingWorkerEnabled || store == nil || client == nil {
		return nil, false
	}
	if factory == nil {
		factory = func(store relay.BatchPollingWorkerStore, client relay.BatchStatusClient, config relay.BatchPollingWorkerConfig) relayBatchPollingWorkerRunner {
			return relay.NewBatchPollingWorker(store, client, config)
		}
	}
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	worker := factory(store, client, relay.BatchPollingWorkerConfig{
		Interval:            time.Duration(cfg.RelayBatchPollingWorkerIntervalMS) * time.Millisecond,
		Limit:               cfg.RelayBatchPollingWorkerClaimLimit,
		CompletionFinalizer: finalizer,
		FailureFinalizer:    failureFinalizer,
		OnError: func(err error) {
			log.Printf("warning: relay batch polling worker failed: %v", err)
		},
	})
	if worker == nil {
		cancelWorker()
		return nil, false
	}
	go worker.Run(workerCtx)
	return cancelWorker, true
}

func relayInstanceDefaultAdapter(pool *relay.ChannelPool) *channel.OpenAIAdapter {
	for _, ch := range pool.ListChannels() {
		if ch == nil || !ch.Enabled {
			continue
		}
		return channel.NewOpenAICompatibleAdapter(ch.Provider, ch.BaseURL, ch.APIKey)
	}
	return channel.NewOpenAIAdapter("", "")
}

func buildStandaloneRelayConfig(
	cfg config.Config,
	pool *relay.ChannelPool,
	pricingStore *relay.PricingStore,
	apiTokenAuthenticator types.RelayAPITokenAuthenticator,
	rateLimiter ratelimit.RateLimiter,
	relayStore *relay.RelayStore,
	alertStateStore observability.AlertStateStore,
) *relay.Config {
	return &relay.Config{
		Pool:                               pool,
		Production:                         cfg.Env == "production",
		APITokenAuthenticator:              apiTokenAuthenticator,
		PricingStore:                       pricingStore,
		RateLimiter:                        rateLimiter,
		ConversationAffinityStore:          relayStore,
		FilesMappingStore:                  relayStore,
		BatchPollingRegistrar:              relayStore,
		BatchCommercialLifecycleEnabled:    cfg.RelayBatchCommercialLifecycleEnabled,
		RealtimeCommercialLifecycleEnabled: cfg.RelayRealtimeCommercialLifecycleEnabled,
		CORSAllowedOrigins:                 cfg.CORSAllowedOrigins,
		HealthAlertStateStore:              alertStateStore,
	}
}

func relayDatabaseURL(cfg config.Config) string {
	if cfg.DBMode != "monolith" {
		if value := strings.TrimSpace(cfg.DBURLRelay); value != "" {
			return value
		}
	}
	return cfg.DatabaseURL
}

func loadRelayPricingStore(cfg config.Config, database *sql.DB) *relay.PricingStore {
	pricingStore, err := relay.LoadPricingStoreFromSQL(context.Background(), database)
	if err != nil {
		if cfg.Env == "production" {
			log.Fatalf("load relay pricing catalog: %v", err)
		}
		log.Printf("warning: failed to load relay pricing catalog; using development defaults: %v", err)
		pricingStore = relay.NewPricingStoreWithDefaults()
	}
	if settings, err := admin.NewSQLStore(database).GetRelayPricingSettings(context.Background()); err != nil {
		if cfg.Env == "production" {
			log.Fatalf("load relay pricing settings: %v", err)
		}
		log.Printf("warning: failed to load relay pricing settings: %v", err)
	} else if settings != nil {
		pricingStore.ApplyMultipliers(settings.ModelMultipliers, settings.GroupMultipliers)
	}
	return pricingStore
}

func ensureDefaultChannel(store *relay.RelayStore, pool *relay.ChannelPool, cfg config.Config) {
	if len(pool.ListChannels()) > 0 || cfg.OpenAIAPIKey == "" {
		return
	}
	defaultChannel := &types.Channel{
		ID:       uuid.New().String(),
		Name:     "Default OpenAI",
		Provider: "openai",
		BaseURL:  cfg.OpenAIBaseURL,
		APIKey:   cfg.OpenAIAPIKey,
		Models:   []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo", "gpt-3.5-turbo"},
		RPMLimit: 1000,
		TPMLimit: 100000,
		Enabled:  true,
		Strategy: "weighted",
		Priority: 0,
		Weight:   100,
	}
	if err := store.CreateChannel(defaultChannel); err != nil {
		log.Printf("warning: failed to create default relay channel: %v", err)
		return
	}
	pool.UpdateChannel(defaultChannel)
	log.Printf("created default relay channel: %s", defaultChannel.ID)
}

func applyRelaySemanticCacheConfig(relayCfg *relay.Config, cfg config.Config, database *sql.DB) {
	switch strings.ToLower(strings.TrimSpace(cfg.RelaySemanticCacheBackend)) {
	case "none":
		relayCfg.SemanticCacheDisabled = true
	case "sql":
		relayCfg.SemanticCacheStore = relaycache.NewSQLSemanticCacheStore(database)
		relayCfg.SemanticCacheEmbedder = memory.NewRelayEmbedder(
			"http://localhost:"+fmt.Sprintf("%d", cfg.Port)+"/v1",
			"text-embedding-3-small",
		)
	default:
		relayCfg.SemanticCacheStore = relaycache.NewInMemorySemanticCacheStore()
	}
}

func buildRelayRateLimiter(cfg config.Config) (ratelimit.RateLimiter, func() error) {
	switch strings.ToLower(strings.TrimSpace(cfg.RelayRateLimitBackend)) {
	case "", "memory":
		return ratelimit.NewInMemoryRateLimiter(ratelimit.InMemoryOptions{}), nil
	case "none":
		return nil, nil
	case "redis":
		addr := strings.TrimSpace(cfg.RedisAddr)
		if addr == "" {
			addr = "localhost:6379"
		}
		client := redis.NewClient(&redis.Options{
			Addr:     addr,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})
		limiter := ratelimit.NewRedisRateLimiter(client, ratelimit.RedisOptions{
			KeyPrefix: cfg.RelayRateLimitRedisKeyPrefix,
		})
		return limiter, client.Close
	default:
		return nil, nil
	}
}

type relayUsageLimitService interface {
	ResolveUsageLimit(ctx context.Context, organizationID string, userID string) (quota.UsageLimit, error)
}

func buildRelayUsageLimitResolver(service relayUsageLimitService) relay.RateLimitResolver {
	return func(ctx context.Context, channel *types.RouteChannel, model string, usageValue *types.Usage) relay.RateLimitResolution {
		channelCheck := relayChannelRateLimitCheck(channel, model, usageValue)
		if service == nil {
			return relayRateLimitResolutionFromCheck(channelCheck)
		}
		organizationID, _ := types.TrustedOrganizationIDFromContext(ctx)
		userID, _ := types.TrustedUserIDFromContext(ctx)
		organizationID = strings.TrimSpace(organizationID)
		userID = strings.TrimSpace(userID)
		if organizationID == "" {
			return relayRateLimitResolutionFromCheck(channelCheck)
		}

		limit, err := service.ResolveUsageLimit(ctx, organizationID, userID)
		if err != nil {
			log.Printf("warning: failed to resolve relay usage limit for organization %q user %q: %v", organizationID, userID, err)
			return relayRateLimitResolutionFromCheck(channelCheck)
		}
		resolution := relay.RateLimitResolution{
			Limits: ratelimit.Limits{
				MaxConcurrent:       limit.MaxConcurrentRequests,
				TPM:                 limit.MaxTokensPerWindow,
				MaxTokensPerRequest: limit.MaxTokensPerRequest,
			},
			Key:   relayUsageLimitRateKey(limit, organizationID),
			Usage: relayRateLimitUsage(usageValue),
		}
		if !relayRateLimitCheckEmpty(channelCheck) {
			resolution.Additional = append(resolution.Additional, channelCheck)
		}
		return resolution
	}
}

func relayChannelRateLimitCheck(channel *types.RouteChannel, model string, usageValue *types.Usage) relay.RateLimitCheck {
	limits := ratelimit.Limits{}
	if channel != nil && channel.Channel != nil {
		limits.RPM = channel.Channel.RPMLimit
		limits.TPM = channel.Channel.TPMLimit
	}
	return relay.RateLimitCheck{
		Key: ratelimit.Key{
			ChannelID: relayRateLimitChannelID(channel),
			Model:     model,
		},
		Limits: limits,
		Usage:  relayRateLimitUsage(usageValue),
	}
}

func relayRateLimitUsage(usageValue *types.Usage) ratelimit.Usage {
	if usageValue == nil {
		return ratelimit.Usage{}
	}
	requestTokens := usageValue.PromptTokens
	if requestTokens <= 0 {
		requestTokens = usageValue.TotalTokens
	}
	return ratelimit.Usage{
		Tokens:        usageValue.TotalTokens,
		RequestTokens: requestTokens,
	}
}

func relayRateLimitResolutionFromCheck(check relay.RateLimitCheck) relay.RateLimitResolution {
	return relay.RateLimitResolution{
		Key:    check.Key,
		Limits: check.Limits,
		Usage:  check.Usage,
	}
}

func relayRateLimitCheckEmpty(check relay.RateLimitCheck) bool {
	return check.Limits.RPM <= 0 &&
		check.Limits.TPM <= 0 &&
		check.Limits.MaxConcurrent <= 0 &&
		check.Limits.MaxTokensPerRequest <= 0
}

func relayRateLimitChannelID(channel *types.RouteChannel) string {
	if channel == nil {
		return ""
	}
	if strings.TrimSpace(channel.ChannelID) != "" {
		return strings.TrimSpace(channel.ChannelID)
	}
	if channel.Channel != nil {
		return strings.TrimSpace(channel.Channel.ID)
	}
	return ""
}

func relayUsageLimitRateKey(limit quota.UsageLimit, fallbackOrganizationID string) ratelimit.Key {
	organizationID := strings.TrimSpace(limit.OrganizationID)
	if organizationID == "" {
		organizationID = strings.TrimSpace(fallbackOrganizationID)
	}
	tokenID := strings.TrimSpace(limit.UserID)
	if tokenID == "" {
		tokenID = organizationID
	}
	return ratelimit.Key{
		ChannelID: "quota",
		Model:     organizationID,
		TokenID:   tokenID,
	}
}
