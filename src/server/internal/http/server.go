package http

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	stdhttp "net/http"
	"sort"
	"sync"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"

	"oblivious/server/internal/admin"
	"oblivious/server/internal/agent"
	publishingchannel "oblivious/server/internal/channel"
	"oblivious/server/internal/chat"
	"oblivious/server/internal/config"
	"oblivious/server/internal/mcp"
	"oblivious/server/internal/notification"
	"oblivious/server/internal/observability"
	"oblivious/server/internal/quota"
	"oblivious/server/internal/relay"
	relaychannel "oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/ratelimit"
	"oblivious/server/internal/relay/types"
	"oblivious/server/internal/releasecontract"
	"oblivious/server/internal/schedule"
	"oblivious/server/internal/usage"
	"oblivious/server/internal/workflow"
)

type relayBatchPollingWorkerRunner interface {
	Run(context.Context)
}

type relayBatchPollingWorkerFactory func(relay.BatchPollingWorkerStore, relay.BatchStatusClient, relay.BatchPollingWorkerConfig) relayBatchPollingWorkerRunner

type shutdownRegistrar interface {
	RegisterOnShutdown(func())
}

type Runtime struct {
	Server            *stdhttp.Server
	StartBackground   func(context.Context) error
	Close             func(context.Context) error
	effectDescriptors []releasecontract.EffectDescriptor
}

// EffectDescriptors returns the immutable construction-time evidence captured
// after every strict runtime constructor has completed successfully.
func (r *Runtime) EffectDescriptors() []releasecontract.EffectDescriptor {
	if r == nil {
		return nil
	}
	return append([]releasecontract.EffectDescriptor(nil), r.effectDescriptors...)
}

type RuntimeOptions struct {
	Readiness   releasecontract.ReadinessManager
	Guard       releasecontract.Guard
	Effects     releasecontract.EffectRegistrar
	Authorities releasecontract.RuntimeAuthorities
}

type runtimeLifecycle struct {
	mu        sync.Mutex
	started   bool
	closed    bool
	cancel    context.CancelFunc
	wait      sync.WaitGroup
	close     sync.Once
	resources sync.Once
	runners   []func(context.Context)
	closers   []func()
	server    *stdhttp.Server
}

func BuildRuntime(cfg config.Config, database *sql.DB, options RuntimeOptions) (*Runtime, error) {
	if database == nil {
		return nil, fmt.Errorf("build runtime: database is required")
	}
	routerOptions := RouterOptions{Readiness: options.Readiness, Guard: options.Guard, Effects: options.Effects, Authorities: options.Authorities}
	if err := routerOptions.ValidateReadinessAuthorities(); err != nil {
		return nil, fmt.Errorf("build runtime: %w", err)
	}
	return buildRuntime(cfg, database, options)
}

// NewServer remains a side-effect-free compatibility constructor for tests
// that do not exercise the production readiness boundary.
func NewServer(cfg config.Config, database *sql.DB) *stdhttp.Server {
	runtime, err := buildCompatibilityRuntime(cfg, database, RuntimeOptions{})
	if err != nil {
		panic(err)
	}
	return runtime.Server
}

func buildRuntime(cfg config.Config, database *sql.DB, options RuntimeOptions) (*Runtime, error) {
	return buildRuntimeWithRouter(cfg, database, options, true, func(cfg config.Config, database *sql.DB, options RouterOptions) (stdhttp.Handler, error) {
		return NewReadinessRouterWithOptions(cfg, database, options)
	})
}

func buildCompatibilityRuntime(cfg config.Config, database *sql.DB, options RuntimeOptions) (*Runtime, error) {
	return buildRuntimeWithRouter(cfg, database, options, false, func(cfg config.Config, database *sql.DB, options RouterOptions) (stdhttp.Handler, error) {
		return NewRouterWithOptions(cfg, database, options), nil
	})
}

func buildRuntimeWithRouter(
	cfg config.Config,
	database *sql.DB,
	options RuntimeOptions,
	requireReadiness bool,
	buildRouter func(config.Config, *sql.DB, RouterOptions) (stdhttp.Handler, error),
) (*Runtime, error) {
	requestLogEvidenceStore, requestLogCloser := configureRequestLogSink(cfg)
	closers := []func(){}
	if requestLogCloser != nil {
		closers = append(closers, requestLogCloser)
	}
	var relayPricingStore *relay.PricingStore
	var relayPool *relay.ChannelPool
	var relayStore *relay.RelayStore
	if cfg.RelayEnabled {
		relayPricingStore = loadRelayPricingStore(cfg, database)
		relayStore = relay.NewRelayStore(database)
		relayPool = relay.NewChannelPool()

		// Load channels from database
		if err := relayStore.LoadPoolFromStore(relayPool); err != nil {
			handleRelayPoolConfigurationError(cfg, fmt.Errorf("load relay channels from database: %w", err))
		}

		// Ensure default channel for development
		ensureDefaultChannel(relayStore, relayPool, cfg)
	}

	notificationService := notification.NewService(notification.NewSQLStore(database))
	alertStateStore := observability.NewSQLAlertStateStore(database)
	alertRoutingRuleStore := observability.NewSQLAlertRoutingRuleStore(database)
	alertProviderConfigStore := observability.NewSQLAlertProviderConfigStore(database)
	alertingCloser := configureHTTPAlerting(cfg, alertStateStore, alertRoutingRuleStore, alertProviderConfigStore)
	if alertingCloser != nil {
		closers = append(closers, alertingCloser)
	}
	workflowService := newConfiguredWorkflowServiceWithStoreNotifierAndAlerts(cfg, workflow.NewSQLStore(database), notificationService, currentHTTPAlertSink())
	var agentService *agent.Service
	var mcpClient *mcp.Client
	if requireReadiness {
		provider, err := buildAgentWebSearchProviderWithOptions(cfg, mcp.WebSearchRuntimeOptions{
			Guard: options.Guard, Authorities: options.Authorities, Effects: options.Effects,
		})
		if err != nil {
			closeRuntimeResources(closers)
			return nil, fmt.Errorf("build runtime Agent web search: %w", err)
		}
		runtimeCarrier := agent.ToolRuntimeOptions{
			Authorities: options.Authorities, Guard: options.Guard, Effects: options.Effects,
			HTTPClient: stdhttp.DefaultClient, WebSearchProvider: provider,
		}
		mcpClient, err = mcp.NewClientWithOptions(mcp.NewSQLStore(database), mcp.ClientRuntimeOptions{
			Guard: options.Guard, Authorities: options.Authorities, Effects: options.Effects,
		})
		if err != nil {
			closeRuntimeResources(closers)
			return nil, fmt.Errorf("build runtime MCP client: %w", err)
		}
		scheduleAgentGateway, err := newReadinessAgentGateway(cfg, options)
		if err != nil {
			closeRuntimeResources(closers)
			return nil, fmt.Errorf("build runtime Agent gateway: %w", err)
		}
		agentService, err = agent.NewServiceWithRuntimeOptions(agent.NewSQLStore(database), scheduleAgentGateway, mcpClient, runtimeCarrier)
		if err != nil {
			closeRuntimeResources(closers)
			return nil, fmt.Errorf("build runtime Agent service: %w", err)
		}
	} else {
		agentService = agent.NewService(agent.NewSQLStore(database), newAgentGateway(cfg))
		if provider := buildAgentWebSearchProvider(cfg); provider != nil {
			agentService.SetWebSearchProvider(provider)
		}
	}
	registerWorkflowAgentExecutor(workflowService, agentService)
	scheduleService := newScheduleService(schedule.NewSQLStore(database), workflowService, agentService)
	var channelService *publishingchannel.Service
	if requireReadiness {
		var err error
		channelService, err = publishingchannel.NewReadinessServiceWithOptions(
			publishingchannel.NewAdapterRegistry(nil), options.Guard, options.Authorities, options.Effects,
			publishingchannel.WithChannelHealthNotifier(publishingChannelHealthNotifier),
		)
		if err != nil {
			closeRuntimeResources(closers)
			return nil, fmt.Errorf("build runtime channel service: %w", err)
		}
	} else {
		channelService = publishingchannel.NewServiceWithOptions(
			publishingchannel.NewAdapterRegistry(nil), publishingchannel.WithChannelHealthNotifier(publishingChannelHealthNotifier),
		)
	}

	var relayConfigApplier admin.RelayConfigApplier
	if relayStore != nil && relayPool != nil {
		relayConfigApplier = func(ctx context.Context, change admin.RelayConfigChange) error {
			_ = ctx
			_ = change
			return relayStore.ReloadPoolFromStore(relayPool)
		}
	}

	var relayEngine stdhttp.Handler
	var relayInstance *relay.Relay
	var relayQuotaService *quota.Service
	var relayAPITokenStore *relay.RelayAPITokenSQLStore
	var closeRateLimiter func() error
	if cfg.RelayEnabled {
		apiTokenStore := relay.NewRelayAPITokenSQLStore(database)
		apiTokenAuthenticator := relay.NewAPITokenAuthenticator(apiTokenStore)
		rateLimiter, rateLimiterCloser, rateLimiterErr := buildRelayRateLimiter(cfg)
		if rateLimiterErr != nil {
			closeRuntimeResources(closers)
			return nil, fmt.Errorf("build runtime relay rate limiter: %w", rateLimiterErr)
		}
		closeRateLimiter = rateLimiterCloser
		var createdRelay *relay.Relay
		var err error
		if requireReadiness {
			createdRelay, err = relay.NewRelayWithOptions(
				buildRelayConfig(cfg, database, relayPool, relayPricingStore, apiTokenAuthenticator, rateLimiter, alertStateStore),
				relay.RouterRuntimeOptions{Guard: options.Guard, Authorities: options.Authorities, Effects: options.Effects},
			)
		} else {
			createdRelay, err = relay.NewRelay(buildRelayConfig(cfg, database, relayPool, relayPricingStore, apiTokenAuthenticator, rateLimiter, alertStateStore))
		}
		if err != nil {
			if closeRateLimiter != nil {
				if closeErr := closeRateLimiter(); closeErr != nil {
					log.Printf("warning: failed to close relay rate limiter: %v", closeErr)
				}
				closeRateLimiter = nil
			}
			handleRelayStartupError(cfg, fmt.Errorf("create relay: %w", err))
		} else {
			relayInstance = createdRelay
			relayQuotaService = quota.NewService(quota.NewSQLStore(database))
			relayAPITokenStore = apiTokenStore
			relayInstance.Router().SetQuotaManager(relayQuotaService)
			relayInstance.Router().SetAPITokenQuotaManager(relayAPITokenStore)
			relayInstance.Router().SetUsageLogger(usage.NewSQLRecorder(database))
			relayInstance.Router().SetRateLimitResolver(buildRelayUsageLimitResolver(relayQuotaService))
			relayEngine = relayInstance.Engine()
		}
	}

	// Create main router after Relay is prepared so gateway proxy routes can
	// forward session-authenticated traffic into the same Relay engine.
	routerOptions := RouterOptions{
		Readiness:                   options.Readiness,
		Guard:                       options.Guard,
		Effects:                     options.Effects,
		Authorities:                 options.Authorities,
		AgentService:                agentService,
		MCPClient:                   mcpClient,
		ChannelService:              channelService,
		RelayPricingStore:           relayPricingStore,
		RelayPool:                   relayPool,
		GatewayRelayHandler:         relayEngine,
		ChannelRuntimeStatsProvider: relayPool,
		RelayConfigApplier:          relayConfigApplier,
		RequestLogEvidenceStore:     requestLogEvidenceStore,
		WorkflowService:             workflowService,
		ScheduleService:             scheduleService,
		AlertStateStore:             alertStateStore,
		AlertRoutingRuleStore:       alertRoutingRuleStore,
		AlertProviderConfigStore:    alertProviderConfigStore,
	}
	mainHandler, err := buildRouter(cfg, database, routerOptions)
	if err != nil {
		closeRuntimeResources(closers)
		return nil, fmt.Errorf("build runtime router: %w", err)
	}

	if relayEngine != nil {
		// Mount Relay under /v1/*.
		mainHandler = combineHandlers(mainHandler, relayEngine)
	}

	server := &stdhttp.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           mainHandler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	runners := []func(context.Context){}
	if relayInstance != nil {
		runners = append(runners, relayInstance.StartHealthChecks)
	}
	if cfg.ScheduleWorkerEnabled {
		workerConfig := schedule.WorkerConfig{
			Interval: time.Duration(cfg.ScheduleWorkerIntervalMS) * time.Millisecond,
			Limit:    cfg.ScheduleWorkerClaimLimit,
			OnError: func(err error) {
				log.Printf("warning: scheduled task worker failed: %v", err)
			},
		}
		var worker *schedule.Worker
		var err error
		if requireReadiness {
			workerConfig.Guard, workerConfig.Authorities, workerConfig.Effects = options.Guard, options.Authorities, options.Effects
			worker, err = schedule.NewReadinessWorker(scheduleService, workerConfig)
		} else {
			worker = schedule.NewWorker(scheduleService, workerConfig)
		}
		if err != nil {
			closeRuntimeResources(closers)
			return nil, fmt.Errorf("build runtime schedule worker: %w", err)
		}
		runners = append(runners, worker.Run)
	}
	if cfg.RelayEnabled && cfg.RelayBatchPollingWorkerEnabled && relayStore != nil && relayPool != nil {
		batchFinalizer := relay.NewBatchUsageFinalizer(usage.NewSQLRecorder(database), relay.BatchUsageFinalizerConfig{
			PricingStore: relayPricingStore, QuotaManager: relayQuotaService, APITokenQuotaManager: relayAPITokenStore,
		})
		batchConfig := relay.BatchPollingWorkerConfig{
			Interval: time.Duration(cfg.RelayBatchPollingWorkerIntervalMS) * time.Millisecond,
			Limit:    cfg.RelayBatchPollingWorkerClaimLimit, CompletionFinalizer: batchFinalizer, FailureFinalizer: batchFinalizer,
			OnError: func(err error) { log.Printf("warning: relay batch polling worker failed: %v", err) },
		}
		batchClient := relay.NewOpenAIBatchStatusClient(defaultRelayAdapter(relayPool))
		var batchWorker *relay.BatchPollingWorker
		var err error
		if requireReadiness {
			batchConfig.Guard, batchConfig.Authorities, batchConfig.Effects = options.Guard, options.Authorities, options.Effects
			batchWorker, err = relay.NewReadinessBatchPollingWorker(relayStore, batchClient, batchConfig)
		} else {
			batchWorker = relay.NewBatchPollingWorker(relayStore, batchClient, batchConfig)
		}
		if err != nil {
			closeRuntimeResources(closers)
			return nil, fmt.Errorf("build runtime Relay batch worker: %w", err)
		}
		runners = append(runners, batchWorker.Run)
	}
	if cfg.Env != "test" {
		channelRetryStore := publishingchannel.NewSQLStore(database)
		channelRetryWorker := publishingchannel.NewRetryWorker(
			channelService,
			channelRetryStore,
			publishingchannel.RetryWorkerConfig{
				OnError: func(err error) {
					log.Printf("warning: channel retry worker failed: %v", err)
				},
			},
		)
		runners = append(runners, channelRetryWorker.Run)
	}
	if cfg.Env != "test" && cfg.ChannelMessageLogArchiveEnabled {
		archiveSink, archiveSinkErr := buildChannelMessageLogArchiveSink(cfg)
		if archiveSinkErr != nil {
			log.Printf("warning: channel message log archive worker disabled: %v", archiveSinkErr)
		} else {
			archiveConfig := publishingchannel.ArchiveWorkerConfig{
				Interval:  time.Duration(cfg.ChannelMessageLogArchiveIntervalMS) * time.Millisecond,
				Retention: time.Duration(cfg.ChannelMessageLogRetentionHours) * time.Hour,
				Limit:     cfg.ChannelMessageLogArchiveLimit,
				OnError: func(err error) {
					log.Printf("warning: channel message log archive worker failed: %v", err)
				},
			}
			var channelArchiveWorker *publishingchannel.ArchiveWorker
			if requireReadiness {
				archiveConfig.Guard, archiveConfig.Authorities, archiveConfig.Effects = options.Guard, options.Authorities, options.Effects
				channelArchiveWorker, archiveSinkErr = publishingchannel.NewReadinessArchiveWorker(
					publishingchannel.NewService(publishingchannel.NewAdapterRegistry(nil)), publishingchannel.NewSQLStore(database), archiveSink, archiveConfig,
				)
			} else {
				channelArchiveWorker = publishingchannel.NewArchiveWorker(
					publishingchannel.NewService(publishingchannel.NewAdapterRegistry(nil)), publishingchannel.NewSQLStore(database), archiveSink, archiveConfig,
				)
			}
			if archiveSinkErr != nil {
				closeRuntimeResources(closers)
				return nil, fmt.Errorf("build runtime channel archive worker: %w", archiveSinkErr)
			}
			runners = append(runners, channelArchiveWorker.Run)
		}
	}
	if closeRateLimiter != nil {
		closers = append(closers, func() {
			if err := closeRateLimiter(); err != nil {
				log.Printf("warning: failed to close relay rate limiter: %v", err)
			}
		})
	}

	lifecycle := &runtimeLifecycle{server: server, runners: runners, closers: closers}
	var descriptors []releasecontract.EffectDescriptor
	if snapshotter, ok := options.Effects.(interface {
		Snapshot() []releasecontract.EffectDescriptor
	}); ok {
		descriptors = append(descriptors, snapshotter.Snapshot()...)
		sort.Slice(descriptors, func(i, j int) bool { return descriptors[i].ID < descriptors[j].ID })
	}
	runtime := &Runtime{Server: server, effectDescriptors: descriptors}
	runtime.StartBackground = lifecycle.startBackground
	runtime.Close = lifecycle.closeRuntime
	return runtime, nil
}
func (l *runtimeLifecycle) startBackground(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("start background: context is required")
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return fmt.Errorf("start background: runtime is closed")
	}
	if l.started {
		return fmt.Errorf("start background: already started")
	}
	l.started = true
	backgroundCtx, cancel := context.WithCancel(ctx)
	l.cancel = cancel
	for _, run := range l.runners {
		if run == nil {
			continue
		}
		l.wait.Add(1)
		go func(run func(context.Context)) {
			defer l.wait.Done()
			run(backgroundCtx)
		}(run)
	}
	return nil
}

func (l *runtimeLifecycle) closeRuntime(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("close runtime: context is required")
	}
	l.close.Do(func() {
		l.mu.Lock()
		l.closed = true
		if l.cancel != nil {
			l.cancel()
		}
		l.mu.Unlock()
	})

	shutdownErr := l.server.Shutdown(ctx)
	done := make(chan struct{})
	go func() {
		l.wait.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	l.resources.Do(func() { closeRuntimeResources(l.closers) })
	if shutdownErr == stdhttp.ErrServerClosed {
		return nil
	}
	return shutdownErr
}

func defaultRelayAdapter(pool *relay.ChannelPool) *relaychannel.OpenAIAdapter {
	if pool != nil {
		for _, configured := range pool.ListChannels() {
			if configured != nil && configured.Enabled {
				return relaychannel.NewOpenAICompatibleAdapter(configured.Provider, configured.BaseURL, configured.APIKey)
			}
		}
	}
	return relaychannel.NewOpenAIAdapter("", "")
}

func closeRuntimeResources(closers []func()) {
	for index := len(closers) - 1; index >= 0; index-- {
		if closers[index] != nil {
			closers[index]()
		}
	}
}

func newReadinessAgentGateway(cfg config.Config, options RuntimeOptions) (chat.ChatGateway, error) {
	runtime := chat.RelayGatewayRuntimeOptions{
		Guard: options.Guard, Authorities: options.Authorities, Effects: options.Effects,
		SkipEffectRegistration: true,
	}
	primary, err := chat.NewReadinessRelayGateway(runtime,
		chat.WithRelayURL(configuredChatRelayBaseURL(cfg)), chat.WithDefaultModel(cfg.RelayDefaultModel),
	)
	if err != nil {
		return nil, err
	}
	if cfg.RelayEnabled && cfg.Env != "production" {
		fallback := chat.NewHTTPReplyGenerator("", "", cfg.ModelDefaultName, time.Duration(cfg.LLMTimeoutMS)*time.Millisecond)
		return chat.NewCompositeGatewayWithOptions(primary, fallback, runtime)
	}
	return primary, nil
}

func handleRelayPoolConfigurationError(cfg config.Config, err error) {
	if err == nil {
		return
	}
	if cfg.Env == "production" {
		panic(err)
	}
	log.Printf("warning: %v", err)
}

func handleRelayStartupError(cfg config.Config, err error) {
	if err == nil {
		return
	}
	if cfg.Env == "production" {
		panic(err)
	}
	log.Printf("warning: %v", err)
}

func startRelayBatchPollingWorkerIfEnabled(
	server shutdownRegistrar,
	cfg config.Config,
	store relay.BatchPollingWorkerStore,
	client relay.BatchStatusClient,
	completionFinalizer relay.BatchCompletionFinalizer,
	failureFinalizer relay.BatchFailureFinalizer,
	newWorker relayBatchPollingWorkerFactory,
) bool {
	if !cfg.RelayEnabled || !cfg.RelayBatchPollingWorkerEnabled || server == nil || store == nil || client == nil || newWorker == nil {
		return false
	}
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	worker := newWorker(store, client, relay.BatchPollingWorkerConfig{
		Interval:            time.Duration(cfg.RelayBatchPollingWorkerIntervalMS) * time.Millisecond,
		Limit:               cfg.RelayBatchPollingWorkerClaimLimit,
		CompletionFinalizer: completionFinalizer,
		FailureFinalizer:    failureFinalizer,
	})
	go worker.Run(workerCtx)
	server.RegisterOnShutdown(cancelWorker)
	return true
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
		if cfg.Env == "production" && len(sinks) == 0 && !hasActiveAlertProviderConfig(providerStore) {
			panic("HTTP alert delivery sink is required in production")
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
		if cfg.ObservabilityHTTPLatencySLOThresholdMS > 0 {
			restoreCallbacks = append(restoreCallbacks, setHTTPAlertLatencySLOThreshold(time.Duration(cfg.ObservabilityHTTPLatencySLOThresholdMS)*time.Millisecond))
		}
	}
	if cfg.ObservabilityHTTPRecoveryEnabled {
		cooldown := time.Duration(cfg.ObservabilityHTTPRecoveryCooldownMS) * time.Millisecond
		recovery := observability.NewRecoveryController(observability.RecoveryControllerOptions{
			StateStore: stateStore,
			Policies: []observability.RecoveryPolicy{
				{
					Name:         "record-http-latency-slo",
					Severity:     observability.AlertSeverityWarning,
					Component:    observability.ComponentHTTP,
					FieldMatches: map[string]string{"failure_kind": "latency_slo"},
					ActionType:   observability.RecoveryActionScaleOut,
					Cooldown:     cooldown,
				},
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

func hasActiveAlertProviderConfig(providerStore observability.AlertProviderConfigStore) bool {
	if providerStore == nil {
		return false
	}
	configs, err := providerStore.ListAlertProviderConfigs(context.Background())
	if err != nil {
		return false
	}
	for _, config := range configs {
		if config.Status == observability.AlertProviderStatusActive {
			return true
		}
	}
	return false
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
		Pool:                               relayPool,
		PricingStore:                       relayPricingStore,
		Production:                         cfg.Env == "production",
		APITokenAuthenticator:              apiTokenAuthenticator,
		RateLimiter:                        rateLimiter,
		RouteAuditSink:                     newRelayRouteAuditRequestLogSink(currentRequestLogSink()),
		HealthAlertSink:                    currentHTTPAlertSink(),
		HealthRecoveryController:           currentHTTPRecoveryController(),
		HealthAlertStateStore:              alertStateStore,
		BatchCommercialLifecycleEnabled:    cfg.RelayBatchCommercialLifecycleEnabled,
		RealtimeCommercialLifecycleEnabled: cfg.RelayRealtimeCommercialLifecycleEnabled,
		CORSAllowedOrigins:                 cfg.CORSAllowedOrigins,
	}
	if database != nil {
		relayStore := relay.NewRelayStore(database)
		relayConfig.FilesMappingStore = relayStore
		relayConfig.ConversationAffinityStore = relayStore
	}
	applyRelaySemanticCacheConfig(relayConfig, buildRelaySemanticCacheConfig(cfg, database))
	return relayConfig
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
