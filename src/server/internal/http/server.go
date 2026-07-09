package http

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	stdhttp "net/http"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"

	"oblivious/server/internal/admin"
	"oblivious/server/internal/agent"
	publishingchannel "oblivious/server/internal/channel"
	"oblivious/server/internal/chat"
	"oblivious/server/internal/config"
	"oblivious/server/internal/notification"
	"oblivious/server/internal/observability"
	"oblivious/server/internal/quota"
	"oblivious/server/internal/relay"
	"oblivious/server/internal/relay/ratelimit"
	"oblivious/server/internal/relay/types"
	"oblivious/server/internal/schedule"
	"oblivious/server/internal/usage"
	"oblivious/server/internal/workflow"
)

func NewServer(cfg config.Config, database *sql.DB) *stdhttp.Server {
	requestLogEvidenceStore, requestLogCloser := configureRequestLogSink(cfg)
	var relayPricingStore *relay.PricingStore
	var relayPool *relay.ChannelPool
	var relayStore *relay.RelayStore
	if cfg.RelayEnabled {
		relayPricingStore = relay.NewPricingStoreWithDefaults()
		if settings, err := admin.NewSQLStore(database).GetRelayPricingSettings(context.Background()); err != nil {
			log.Printf("warning: failed to load relay pricing settings: %v", err)
		} else if settings != nil {
			relayPricingStore.ApplyMultipliers(settings.ModelMultipliers, settings.GroupMultipliers)
		}
		relayStore = relay.NewRelayStore(database)
		relayPool = relay.NewChannelPool()

		// Load channels from database
		if err := relayStore.LoadPoolFromStore(relayPool); err != nil {
			log.Printf("warning: failed to load channels from database: %v", err)
		}

		// Ensure default channel for development
		ensureDefaultChannel(relayStore, relayPool, cfg)
	}

	notificationService := notification.NewService(notification.NewSQLStore(database))
	alertStateStore := observability.NewSQLAlertStateStore(database)
	alertRoutingRuleStore := observability.NewSQLAlertRoutingRuleStore(database)
	alertProviderConfigStore := observability.NewSQLAlertProviderConfigStore(database)
	alertingCloser := configureHTTPAlerting(cfg, alertStateStore, alertRoutingRuleStore, alertProviderConfigStore)
	workflowService := newConfiguredWorkflowServiceWithStoreNotifierAndAlerts(cfg, workflow.NewSQLStore(database), notificationService, currentHTTPAlertSink())
	scheduleAgentGateway := newAgentGateway(cfg)
	agentService := agent.NewService(agent.NewSQLStore(database), scheduleAgentGateway)
	if provider := buildAgentWebSearchProvider(cfg); provider != nil {
		agentService.SetWebSearchProvider(provider)
	}
	registerWorkflowAgentExecutor(workflowService, agentService)
	scheduleService := newScheduleService(schedule.NewSQLStore(database), workflowService, agentService)

	// Create main router
	var relayConfigApplier admin.RelayConfigApplier
	if relayStore != nil && relayPool != nil {
		relayConfigApplier = func(ctx context.Context, change admin.RelayConfigChange) error {
			_ = ctx
			_ = change
			return relayStore.ReloadPoolFromStore(relayPool)
		}
	}
	mainHandler := NewRouterWithOptions(cfg, database, RouterOptions{
		RelayPricingStore:           relayPricingStore,
		ChannelRuntimeStatsProvider: relayPool,
		RelayConfigApplier:          relayConfigApplier,
		RequestLogEvidenceStore:     requestLogEvidenceStore,
		WorkflowService:             workflowService,
		ScheduleService:             scheduleService,
		AlertStateStore:             alertStateStore,
		AlertRoutingRuleStore:       alertRoutingRuleStore,
		AlertProviderConfigStore:    alertProviderConfigStore,
	})

	// If Relay is enabled, integrate it
	var closeRateLimiter func() error
	var cancelRelayHealthChecks context.CancelFunc
	if cfg.RelayEnabled {

		// Create Relay instance
		apiTokenStore := relay.NewRelayAPITokenSQLStore(database)
		apiTokenAuthenticator := relay.NewAPITokenAuthenticator(apiTokenStore)
		rateLimiter, rateLimiterCloser := buildRelayRateLimiter(cfg)
		closeRateLimiter = rateLimiterCloser
		relayInstance, err := relay.NewRelay(buildRelayConfig(cfg, database, relayPool, relayPricingStore, apiTokenAuthenticator, rateLimiter, alertStateStore))
		if err != nil {
			if closeRateLimiter != nil {
				if closeErr := closeRateLimiter(); closeErr != nil {
					log.Printf("warning: failed to close relay rate limiter: %v", closeErr)
				}
				closeRateLimiter = nil
			}
			log.Printf("warning: failed to create relay: %v", err)
		} else {
			// Wire quota.Service into the relay billing lifecycle so that
			// successful calls settle and failed calls refund correctly.
			quotaStore := quota.NewSQLStore(database)
			quotaService := quota.NewService(quotaStore)
			relayInstance.Router().SetQuotaManager(quotaService)
			relayInstance.Router().SetAPITokenQuotaManager(apiTokenStore)
			relayInstance.Router().SetUsageLogger(usage.NewSQLRecorder(database))
			relayInstance.Router().SetRateLimitResolver(buildRelayUsageLimitResolver(quotaService))
			relayHealthCheckCtx, cancelHealthChecks := context.WithCancel(context.Background())
			cancelRelayHealthChecks = cancelHealthChecks
			relayInstance.StartHealthChecks(relayHealthCheckCtx)

			// Mount Relay under /v1/*
			mainHandler = combineHandlers(mainHandler, relayInstance.Engine())
		}
	}

	server := &stdhttp.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           mainHandler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	if cfg.ScheduleWorkerEnabled {
		workerCtx, cancelWorker := context.WithCancel(context.Background())
		worker := schedule.NewWorker(scheduleService, schedule.WorkerConfig{
			Interval: time.Duration(cfg.ScheduleWorkerIntervalMS) * time.Millisecond,
			Limit:    cfg.ScheduleWorkerClaimLimit,
			OnError: func(err error) {
				log.Printf("warning: scheduled task worker failed: %v", err)
			},
		})
		go worker.Run(workerCtx)
		server.RegisterOnShutdown(cancelWorker)
	}
	if cfg.Env != "test" {
		channelRetryWorkerCtx, cancelChannelRetryWorker := context.WithCancel(context.Background())
		channelRetryStore := publishingchannel.NewSQLStore(database)
		channelRetryWorker := publishingchannel.NewRetryWorker(
			publishingchannel.NewServiceWithOptions(
				publishingchannel.NewAdapterRegistry(nil),
				publishingchannel.WithChannelHealthNotifier(publishingChannelHealthNotifier),
			),
			channelRetryStore,
			publishingchannel.RetryWorkerConfig{
				OnError: func(err error) {
					log.Printf("warning: channel retry worker failed: %v", err)
				},
			},
		)
		go channelRetryWorker.Run(channelRetryWorkerCtx)
		server.RegisterOnShutdown(cancelChannelRetryWorker)
	}
	if cfg.Env != "test" && cfg.ChannelMessageLogArchiveEnabled {
		archiveSink, archiveSinkErr := buildChannelMessageLogArchiveSink(cfg)
		if archiveSinkErr != nil {
			log.Printf("warning: channel message log archive worker disabled: %v", archiveSinkErr)
		} else {
			channelArchiveWorkerCtx, cancelChannelArchiveWorker := context.WithCancel(context.Background())
			channelArchiveWorker := publishingchannel.NewArchiveWorker(
				publishingchannel.NewService(publishingchannel.NewAdapterRegistry(nil)),
				publishingchannel.NewSQLStore(database),
				archiveSink,
				publishingchannel.ArchiveWorkerConfig{
					Interval:  time.Duration(cfg.ChannelMessageLogArchiveIntervalMS) * time.Millisecond,
					Retention: time.Duration(cfg.ChannelMessageLogRetentionHours) * time.Hour,
					Limit:     cfg.ChannelMessageLogArchiveLimit,
					OnError: func(err error) {
						log.Printf("warning: channel message log archive worker failed: %v", err)
					},
				},
			)
			go channelArchiveWorker.Run(channelArchiveWorkerCtx)
			server.RegisterOnShutdown(cancelChannelArchiveWorker)
		}
	}
	if closeRateLimiter != nil {
		server.RegisterOnShutdown(func() {
			if err := closeRateLimiter(); err != nil {
				log.Printf("warning: failed to close relay rate limiter: %v", err)
			}
		})
	}
	if cancelRelayHealthChecks != nil {
		server.RegisterOnShutdown(cancelRelayHealthChecks)
	}
	if requestLogCloser != nil {
		server.RegisterOnShutdown(requestLogCloser)
	}
	if alertingCloser != nil {
		server.RegisterOnShutdown(alertingCloser)
	}
	return server
}

func newAgentGateway(cfg config.Config) chat.ChatGateway {
	relayGateway := chat.NewRelayGateway(
		chat.WithRelayURL("http://localhost:"+fmt.Sprintf("%d", cfg.Port)+"/v1"),
		chat.WithDefaultModel(cfg.RelayDefaultModel),
	)
	if cfg.RelayEnabled {
		if cfg.Env != "production" {
			localGenerator := chat.NewHTTPReplyGenerator("", "", cfg.ModelDefaultName, time.Duration(cfg.LLMTimeoutMS)*time.Millisecond)
			return chat.NewCompositeGateway(relayGateway, localGenerator)
		}
		return relayGateway
	}
	localGenerator := chat.NewHTTPReplyGenerator("", "", cfg.ModelDefaultName, time.Duration(cfg.LLMTimeoutMS)*time.Millisecond)
	return chat.NewLocalGateway(localGenerator)
}

func buildChannelMessageLogArchiveSink(cfg config.Config) (publishingchannel.MessageLogArchiveSink, error) {
	backend := cfg.ChannelMessageLogArchiveBackend
	if backend == "" && cfg.ChannelMessageLogArchiveRoot != "" {
		backend = "file"
	}
	switch backend {
	case "file":
		if cfg.ChannelMessageLogArchiveRoot == "" {
			return nil, fmt.Errorf("channel message log archive root is required")
		}
		return publishingchannel.NewFileMessageLogArchiveSink(cfg.ChannelMessageLogArchiveRoot), nil
	case "s3":
		return publishingchannel.NewS3MessageLogArchiveSink(publishingchannel.S3MessageLogArchiveSinkOptions{
			Endpoint:  cfg.ChannelMessageLogArchiveS3Endpoint,
			Region:    cfg.ChannelMessageLogArchiveS3Region,
			Bucket:    cfg.ChannelMessageLogArchiveS3Bucket,
			AccessKey: cfg.ChannelMessageLogArchiveS3AccessKey,
			SecretKey: cfg.ChannelMessageLogArchiveS3SecretKey,
		}), nil
	default:
		return nil, fmt.Errorf("unsupported channel message log archive backend %q", backend)
	}
}

func configureRequestLogSink(cfg config.Config) (admin.RequestLogEvidenceStore, func()) {
	if cfg.ObservabilityRequestLogBackend != "clickhouse" {
		return nil, nil
	}
	clickHouseDB, err := sql.Open(cfg.ClickHouseDriver, cfg.ClickHouseDSN)
	if err != nil {
		handleRequestLogSinkConfigurationError(cfg, fmt.Errorf("open ClickHouse request log sink: %w", err))
		return nil, nil
	}
	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := clickHouseDB.PingContext(pingCtx); err != nil {
		_ = clickHouseDB.Close()
		handleRequestLogSinkConfigurationError(cfg, fmt.Errorf("ping ClickHouse request log sink: %w", err))
		return nil, nil
	}
	requestLogEvidenceStore := admin.NewClickHouseUsageAnalyticsStore(clickHouseDB)
	restoreSink := setRequestLogSink(observability.NewSQLRequestLogSink(clickHouseDB))
	return requestLogEvidenceStore, func() {
		restoreSink()
		if err := clickHouseDB.Close(); err != nil {
			log.Printf("warning: failed to close ClickHouse request log sink: %v", err)
		}
	}
}

func handleRequestLogSinkConfigurationError(cfg config.Config, err error) {
	if cfg.Env == "production" {
		panic(err)
	}
	log.Printf("warning: %v", err)
}

func configureHTTPAlerting(
	cfg config.Config,
	stateStore observability.AlertStateStore,
	routingStore observability.AlertRoutingRuleStore,
	providerStore observability.AlertProviderConfigStore,
) func() {
	if !cfg.ObservabilityHTTPAlertsEnabled && !cfg.ObservabilityHTTPRecoveryEnabled {
		return nil
	}
	var restoreCallbacks []func()
	var alertSink observability.AlertSink
	if cfg.ObservabilityHTTPAlertsEnabled {
		sinks := []observability.AlertDeliverySink{}
		if cfg.AlertWebhookURL != "" {
			sinks = append(sinks, observability.NewWebhookAlertDeliverySink(observability.AlertWebhookDeliverySinkOptions{
				EndpointURL: cfg.AlertWebhookURL,
				Secret:      cfg.AlertWebhookSecret,
			}))
		}
		dispatcher := observability.NewAlertDeliveryDispatcher(observability.AlertDeliveryDispatcherOptions{
			RoutingRules: routingStore,
			Sinks:        sinks,
			SinkResolver: observability.NewAlertProviderDeliverySinkResolver(observability.AlertProviderDeliverySinkResolverOptions{
				ProviderStore: providerStore,
			}),
			HistoryStore: stateStore,
		})
		alertSink = observability.NewAlertRouter(observability.AlertRouterOptions{
			StateStore: stateStore,
			NotifySink: dispatcher,
		})
		restoreCallbacks = append(restoreCallbacks, setHTTPAlertSink(alertSink))
		restoreCallbacks = append(restoreCallbacks, setPublishingChannelAlertSink(alertSink))
	}
	if cfg.ObservabilityHTTPRecoveryEnabled {
		cooldown := time.Duration(cfg.ObservabilityHTTPRecoveryCooldownMS) * time.Millisecond
		recovery := observability.NewRecoveryController(observability.RecoveryControllerOptions{
			StateStore: stateStore,
			Policies: []observability.RecoveryPolicy{
				{
					Name:       "record-http-5xx",
					Severity:   observability.AlertSeverityWarning,
					Component:  observability.ComponentHTTP,
					ActionType: observability.RecoveryActionRestart,
					Cooldown:   cooldown,
				},
				{
					Name:         "record-http-panic",
					Severity:     observability.AlertSeverityCritical,
					Component:    observability.ComponentHTTP,
					FieldMatches: map[string]string{"failure_kind": "panic"},
					ActionType:   observability.RecoveryActionRestart,
					Cooldown:     cooldown,
				},
				{
					Name:         "record-runtime-oom",
					Severity:     observability.AlertSeverityCritical,
					Component:    observability.ComponentHTTP,
					FieldMatches: map[string]string{"failure_kind": "oom"},
					ActionType:   observability.RecoveryActionRestart,
					Cooldown:     cooldown,
				},
				{
					Name:       "record-http-critical-5xx",
					Severity:   observability.AlertSeverityCritical,
					Component:  observability.ComponentHTTP,
					ActionType: observability.RecoveryActionRestart,
					Cooldown:   cooldown,
				},
				{
					Name:       "record-channel-degraded",
					Severity:   observability.AlertSeverityWarning,
					Component:  "publishing_channel",
					ActionType: observability.RecoveryActionFailover,
					Cooldown:   cooldown,
				},
				{
					Name:       "record-relay-channel-unhealthy",
					Severity:   observability.AlertSeverityWarning,
					Component:  observability.ComponentRelay,
					ActionType: observability.RecoveryActionFailover,
					Cooldown:   cooldown,
				},
			},
		})
		restoreCallbacks = append(restoreCallbacks, setHTTPRecoveryController(recovery))
		restoreCallbacks = append(restoreCallbacks, setPublishingChannelRecoveryController(recovery))
	}
	if len(restoreCallbacks) == 0 {
		return nil
	}
	return func() {
		for index := len(restoreCallbacks) - 1; index >= 0; index-- {
			restoreCallbacks[index]()
		}
	}
}

func buildRelayConfig(
	cfg config.Config,
	database *sql.DB,
	relayPool *relay.ChannelPool,
	relayPricingStore *relay.PricingStore,
	apiTokenAuthenticator types.RelayAPITokenAuthenticator,
	rateLimiter ratelimit.RateLimiter,
	alertStateStore observability.AlertStateStore,
) *relay.Config {
	relayConfig := &relay.Config{
		Pool:                     relayPool,
		PricingStore:             relayPricingStore,
		Production:               cfg.Env == "production",
		APITokenAuthenticator:    apiTokenAuthenticator,
		RateLimiter:              rateLimiter,
		RouteAuditSink:           newRelayRouteAuditRequestLogSink(currentRequestLogSink()),
		HealthAlertSink:          currentHTTPAlertSink(),
		HealthRecoveryController: currentHTTPRecoveryController(),
		HealthAlertStateStore:    alertStateStore,
	}
	if database != nil {
		relayStore := relay.NewRelayStore(database)
		relayConfig.FilesMappingStore = relayStore
		relayConfig.ConversationAffinityStore = relayStore
	}
	applyRelaySemanticCacheConfig(relayConfig, buildRelaySemanticCacheConfig(cfg, database))
	return relayConfig
}

// ensureDefaultChannel creates a default OpenAI channel if no channels exist
func ensureDefaultChannel(store *relay.RelayStore, pool *relay.ChannelPool, cfg config.Config) {
	channels := pool.ListChannels()
	if len(channels) == 0 && cfg.OpenAIAPIKey != "" {
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
			log.Printf("warning: failed to create default channel: %v", err)
		} else {
			pool.UpdateChannel(defaultChannel)
			log.Printf("created default OpenAI channel: %s", defaultChannel.ID)
		}
	}
}

// combineHandlers combines main router with relay engine
func combineHandlers(main stdhttp.Handler, relayEngine stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		// Route /v1/* to Relay
		if len(r.URL.Path) >= 3 && r.URL.Path[:3] == "/v1" {
			relayEngine.ServeHTTP(w, r)
			return
		}
		if aliasPath, ok := relayAliasTargetPath(r.Method, r.URL.Path); ok {
			aliasRequest := r.Clone(r.Context())
			aliasRequest.URL.Path = aliasPath
			aliasRequest.RequestURI = ""
			relayEngine.ServeHTTP(w, aliasRequest)
			return
		}
		// Everything else goes to main router
		main.ServeHTTP(w, r)
	})
}

func relayAliasTargetPath(method, path string) (string, bool) {
	switch method + " " + path {
	case stdhttp.MethodPost + " /api/v1/relay/chat/completions":
		return "/v1/chat/completions", true
	case stdhttp.MethodPost + " /api/v1/relay/embeddings":
		return "/v1/embeddings", true
	case stdhttp.MethodPost + " /api/v1/relay/responses":
		return "/v1/responses", true
	case stdhttp.MethodPost + " /api/v1/relay/images/generations":
		return "/v1/images/generations", true
	case stdhttp.MethodPost + " /api/v1/relay/images/edits":
		return "/v1/images/edits", true
	case stdhttp.MethodPost + " /api/v1/relay/images/variations":
		return "/v1/images/variations", true
	case stdhttp.MethodPost + " /api/v1/relay/audio/speech":
		return "/v1/audio/speech", true
	case stdhttp.MethodPost + " /api/v1/relay/audio/transcriptions":
		return "/v1/audio/transcriptions", true
	case stdhttp.MethodPost + " /api/v1/relay/audio/translations":
		return "/v1/audio/translations", true
	case stdhttp.MethodGet + " /api/v1/relay/models":
		return "/v1/models", true
	default:
		return "", false
	}
}
