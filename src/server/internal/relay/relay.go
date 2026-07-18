package relay

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/observability"
	relaycache "oblivious/server/internal/relay/cache"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/handler"
	"oblivious/server/internal/relay/ratelimit"
	"oblivious/server/internal/relay/types"
	appws "oblivious/server/internal/ws"
)

type Relay struct {
	engine                             *gin.Engine
	pool                               *ChannelPool
	handlers                           map[types.APIType]types.Handler
	router                             *Router
	cache                              *relaycache.SemanticCache
	production                         bool
	auditSink                          handler.RouteAuditSink
	apiTokenAuthenticator              types.RelayAPITokenAuthenticator
	batchCommercialLifecycleEnabled    bool
	realtimeCommercialLifecycleEnabled bool
	webSocketOriginPolicy              appws.OriginPolicy
	healthCheckInterval                time.Duration
	healthCheckTimeout                 time.Duration
	healthCheckFailureThreshold        int
	healthAlertSink                    observability.AlertSink
	healthRecoveryController           *observability.RecoveryController
	healthAlertStateStore              observability.AlertStateStore
}

type Config struct {
	Pool                               *ChannelPool
	Production                         bool
	APITokenAuthenticator              types.RelayAPITokenAuthenticator
	PricingStore                       *PricingStore
	RateLimiter                        ratelimit.RateLimiter
	ConversationAffinityStore          ConversationAffinityStore
	SemanticCacheStore                 relaycache.SemanticCacheStore
	SemanticCacheEmbedder              handler.SemanticCacheEmbedder
	SemanticCacheDisabled              bool
	FilesMappingStore                  handler.FilesMappingStore
	BatchPollingRegistrar              handler.BatchPollingRegistrar
	BatchCommercialLifecycleEnabled    bool
	RealtimeCommercialLifecycleEnabled bool
	CORSAllowedOrigins                 []string
	RouteAuditSink                     handler.RouteAuditSink
	HealthCheckInterval                time.Duration
	HealthCheckTimeout                 time.Duration
	HealthCheckFailureThreshold        int
	HealthAlertSink                    observability.AlertSink
	HealthRecoveryController           *observability.RecoveryController
	HealthAlertStateStore              observability.AlertStateStore
}

func NewRelay(cfg *Config) (*Relay, error) {
	return newRelay(cfg, RouterRuntimeOptions{}, false)
}

// NewRelayWithOptions constructs the production Relay with the single
// startup-built readiness carrier. It never starts health checks; lifecycle
// ownership remains with the caller.
func NewRelayWithOptions(cfg *Config, options RouterRuntimeOptions) (*Relay, error) {
	return newRelay(cfg, options, true)
}

func newRelay(cfg *Config, runtimeOptions RouterRuntimeOptions, requireReadiness bool) (*Relay, error) {
	if cfg == nil {
		cfg = &Config{}
	}
	r := &Relay{
		handlers:                           make(map[types.APIType]types.Handler),
		production:                         cfg.Production,
		auditSink:                          cfg.RouteAuditSink,
		apiTokenAuthenticator:              cfg.APITokenAuthenticator,
		batchCommercialLifecycleEnabled:    cfg.BatchCommercialLifecycleEnabled,
		realtimeCommercialLifecycleEnabled: cfg.RealtimeCommercialLifecycleEnabled,
		webSocketOriginPolicy:              appws.NewOriginPolicy(cfg.CORSAllowedOrigins),
		healthCheckInterval:                cfg.HealthCheckInterval,
		healthCheckTimeout:                 cfg.HealthCheckTimeout,
		healthCheckFailureThreshold:        cfg.HealthCheckFailureThreshold,
		healthAlertSink:                    cfg.HealthAlertSink,
		healthRecoveryController:           cfg.HealthRecoveryController,
		healthAlertStateStore:              cfg.HealthAlertStateStore,
	}
	if r.healthCheckInterval <= 0 {
		r.healthCheckInterval = 30 * time.Second
	}
	if r.healthCheckTimeout <= 0 {
		r.healthCheckTimeout = 10 * time.Second
	}
	if r.healthCheckFailureThreshold <= 0 {
		r.healthCheckFailureThreshold = 3
	}

	if cfg.Pool != nil {
		r.pool = cfg.Pool
	} else {
		r.pool = NewChannelPool()
	}

	lb := NewLoadBalancer(r.pool, "weighted")
	cbs := make(map[string]*CircuitBreaker)
	for _, ch := range r.pool.ListChannels() {
		cbs[ch.ID] = NewCircuitBreaker(ch.ID, ch.CBThreshold, 10*time.Second, 60*time.Second)
	}

	tb := NewTokenBucket(1000, 60000)
	hc := NewHealthChecker(HealthCheckModelsAPI, r.healthCheckTimeout)

	pricing := cfg.PricingStore
	if pricing == nil {
		if cfg.Production {
			return nil, errors.New("production relay requires configured pricing store")
		}
		pricing = NewPricingStoreWithDefaults()
	}
	if cfg.Production && pricing.IsEmpty() {
		return nil, errors.New("production relay requires active pricing entries")
	}
	if cfg.Production && cfg.BatchCommercialLifecycleEnabled && cfg.BatchPollingRegistrar == nil {
		return nil, errors.New("production batch commercial lifecycle requires configured polling registrar")
	}
	if cfg.Production && cfg.RealtimeCommercialLifecycleEnabled && !hasActiveRealtimePricingForPool(pricing, r.pool) {
		return nil, errors.New("production realtime commercial lifecycle requires active realtime pricing")
	}
	seenIdem := make(map[string]bool)
	billingHook := NewBillingHook(pricing, &seenIdem)

	if requireReadiness {
		var err error
		r.router, err = NewRouterWithBillingOptions(r.pool, lb, cbs, tb, hc, billingHook, "", runtimeOptions)
		if err != nil {
			return nil, err
		}
	} else {
		r.router = NewRouterWithBilling(r.pool, lb, cbs, tb, hc, billingHook, "")
	}
	r.router.rateLimiter = cfg.RateLimiter
	r.router.affinityStore = cfg.ConversationAffinityStore
	if !cfg.SemanticCacheDisabled {
		r.cache = relaycache.NewSemanticCache(cfg.SemanticCacheStore, relaycache.SemanticCacheOptions{})
		r.router.semanticCache = r.cache
	}

	handler.SetRouter(r.router)
	r.registerHandlers(cfg)
	r.initRouter()
	return r, nil
}

func (r *Relay) registerHandlers(cfg *Config) {
	poolInterface := types.ChannelPoolInterface(r.pool)
	defaultAdapter := r.defaultAdapter()
	if cfg.SemanticCacheEmbedder != nil {
		r.handlers[types.APITypeChat] = handler.NewChatHandlerWithSemanticCacheEmbedder(&poolInterface, defaultAdapter, cfg.SemanticCacheEmbedder)
	} else {
		r.handlers[types.APITypeChat] = handler.NewChatHandler(&poolInterface, defaultAdapter)
	}
	r.handlers[types.APITypeResponses] = handler.NewResponsesHandler(&poolInterface, defaultAdapter)
	r.handlers[types.APITypeModels] = handler.NewModelsHandler(&poolInterface)
	r.handlers[types.APITypeRealtime] = handler.NewRealtimeHandlerWithOriginPolicy(&poolInterface, defaultAdapter, r.webSocketOriginPolicy).
		WithCommercialLifecycleEnabled(r.realtimeCommercialLifecycleEnabled)
	r.handlers[types.APITypeEmbeddings] = handler.NewEmbeddingsHandler(&poolInterface, defaultAdapter)
	imagesHandler := handler.NewImagesHandler(&poolInterface, defaultAdapter)
	r.handlers[types.APITypeImageGen] = imagesHandler
	r.handlers[types.APITypeImageEdit] = imagesHandler
	r.handlers[types.APITypeImageVar] = imagesHandler
	audioHandler := handler.NewAudioHandler(&poolInterface, defaultAdapter)
	r.handlers[types.APITypeAudioSpeech] = audioHandler
	r.handlers[types.APITypeAudioSTT] = audioHandler
	r.handlers[types.APITypeAudioTranslate] = audioHandler
	r.handlers[types.APITypeModeration] = handler.NewModerationsHandler(&poolInterface, defaultAdapter)
	if cfg.SemanticCacheEmbedder != nil {
		r.handlers[types.APITypeCompletions] = handler.NewLegacyCompletionsHandlerWithSemanticCacheEmbedder(&poolInterface, defaultAdapter, cfg.SemanticCacheEmbedder)
	} else {
		r.handlers[types.APITypeCompletions] = handler.NewLegacyCompletionsHandler(&poolInterface, defaultAdapter)
	}
	r.handlers[types.APITypeBatch] = handler.NewBatchHandler(&poolInterface, defaultAdapter).
		WithCommercialLifecycleEnabled(r.batchCommercialLifecycleEnabled).
		WithPollingRegistrar(cfg.BatchPollingRegistrar)
	r.handlers[types.APITypeFiles] = handler.NewFilesHandler(&poolInterface, defaultAdapter, ".tmp/relay").WithMappingStore(cfg.FilesMappingStore)
	fineTuningHandler := handler.NewFineTuningHandler(defaultAdapter)
	r.handlers[types.APITypeFineTuning] = fineTuningHandler
	assistantsHandler := handler.NewAssistantsHandler(defaultAdapter)
	r.handlers[types.APITypeAssistants] = assistantsHandler
	r.handlers[types.APITypeThreads] = assistantsHandler
	r.handlers[types.APITypeRuns] = assistantsHandler
}

func (r *Relay) defaultAdapter() *channel.OpenAIAdapter {
	for _, ch := range r.pool.ListChannels() {
		if ch == nil || !ch.Enabled {
			continue
		}
		return channel.NewOpenAICompatibleAdapter(ch.Provider, ch.BaseURL, ch.APIKey)
	}
	return channel.NewOpenAIAdapter("", "")
}

func hasActiveRealtimePricingForPool(pricing *PricingStore, pool *ChannelPool) bool {
	if pricing == nil || pool == nil {
		return false
	}
	for _, ch := range pool.ListChannels() {
		if ch == nil || !ch.Enabled {
			continue
		}
		for _, model := range ch.Models {
			model = strings.TrimSpace(model)
			if model == "" {
				continue
			}
			if _, err := pricing.GetPrice(model, types.APITypeRealtime, types.DimTotalTokens); err == nil {
				return true
			}
		}
	}
	return false
}

func (r *Relay) initRouter() {
	r.engine = gin.New()
	handler.RegisterRoutesWithOptions(r.engine, r.handlers, handler.RouteRegistrationOptions{
		Production:                         r.production,
		AuditSink:                          r.auditSink,
		APITokenAuthenticator:              r.apiTokenAuthenticator,
		BatchCommercialLifecycleEnabled:    r.batchCommercialLifecycleEnabled,
		RealtimeCommercialLifecycleEnabled: r.realtimeCommercialLifecycleEnabled,
	})
}

func (r *Relay) Engine() *gin.Engine {
	return r.engine
}

func (r *Relay) Router() *Router {
	return r.router
}

func (r *Relay) StartHealthChecks(ctx context.Context) {
	interval := r.healthCheckInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		r.runHealthCheckOnce(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.runHealthCheckOnce(ctx)
			}
		}
	}()
}

func (r *Relay) runHealthCheckOnce(ctx context.Context) {
	for _, ch := range r.pool.ListChannels() {
		if ch == nil || !ch.Enabled || strings.EqualFold(ch.HealthCheckStrategy, string(HealthCheckDisabled)) {
			continue
		}
		adapter := channel.NewOpenAICompatibleAdapter(ch.Provider, ch.BaseURL, ch.APIKey)
		err := adapter.HealthCheck(ctx)
		stats, _ := r.pool.GetStats(ch.ID)
		if stats == nil {
			continue
		}
		stats.LastProbeTime = time.Now().UTC()
		if err != nil {
			stats.CBFailures++
			if stats.CBFailures >= r.healthCheckFailureThreshold {
				wasInvalid := stats.Invalid
				stats.Invalid = true
				r.pool.SetChannelHealthy(ch.ID, false)
				if !wasInvalid {
					r.routeHealthUnhealthy(ctx, ch, stats, err)
				}
			}
			continue
		}
		wasInvalid := stats.Invalid
		stats.CBFailures = 0
		stats.Invalid = false
		stats.LastProbeSuccess = time.Now().UTC()
		r.pool.SetChannelHealthy(ch.ID, true)
		if wasInvalid {
			r.resolveHealthUnhealthy(ctx, ch, stats)
		}
	}
}

func (r *Relay) routeHealthUnhealthy(ctx context.Context, ch *types.Channel, stats *types.ChannelStats, probeErr error) {
	if r == nil || ch == nil {
		return
	}
	if r.healthAlertSink == nil && r.healthRecoveryController == nil {
		return
	}
	event := relayChannelUnhealthyAlertEvent(ch, stats, probeErr)
	if r.healthAlertSink != nil {
		_ = r.healthAlertSink.Notify(ctx, event)
	}
	if r.healthRecoveryController != nil {
		_, _ = r.healthRecoveryController.HandleAlert(ctx, event)
	}
}

func (r *Relay) resolveHealthUnhealthy(ctx context.Context, ch *types.Channel, stats *types.ChannelStats) {
	if r == nil || ch == nil || r.healthAlertStateStore == nil {
		return
	}
	_, _ = r.healthAlertStateStore.ResolveAlert(ctx, relayChannelUnhealthyAlertKey(ch.ID), time.Now().UTC())
}

func relayChannelUnhealthyAlertEvent(ch *types.Channel, stats *types.ChannelStats, probeErr error) observability.AlertEvent {
	occurredAt := time.Now().UTC()
	message := "relay channel health check failed repeatedly"
	if probeErr != nil {
		message = probeErr.Error()
	}
	fields := map[string]any{
		"channel_id": ch.ID,
		"provider":   ch.Provider,
		"base_url":   ch.BaseURL,
		"source":     "relay.health_check",
	}
	if stats != nil {
		fields["failure_count"] = stats.CBFailures
		fields["last_probe_time"] = stats.LastProbeTime
	}
	return observability.AlertEvent{
		Key:        relayChannelUnhealthyAlertKey(ch.ID),
		Severity:   observability.AlertSeverityWarning,
		Title:      "Relay channel unhealthy",
		Message:    message,
		Component:  observability.ComponentRelay,
		OccurredAt: occurredAt,
		Fields:     fields,
	}
}

func relayChannelUnhealthyAlertKey(channelID string) string {
	return "relay:channel:" + strings.TrimSpace(channelID) + ":unhealthy"
}

func (r *Relay) Run(addr string) error {
	return r.engine.Run(addr)
}
