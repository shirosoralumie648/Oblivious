package relay

import (
	"time"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/handler"
	"oblivious/server/internal/relay/types"
)

type Relay struct {
	engine   *gin.Engine
	pool     *ChannelPool
	handlers map[types.APIType]types.Handler
	router   *Router
}

type Config struct {
	Pool       *ChannelPool
	Production bool
}

func NewRelay(cfg *Config) (*Relay, error) {
	r := &Relay{
		handlers: make(map[types.APIType]types.Handler),
	}
	if cfg != nil && cfg.Pool != nil {
		r.pool = cfg.Pool
	} else {
		r.pool = NewChannelPool()
	}

	// Build dependency chain: pool -> lb -> router
	lb := NewLoadBalancer(r.pool, "weighted")

	// Circuit breakers per channel
	cbs := make(map[string]*CircuitBreaker)
	for _, ch := range r.pool.ListChannels() {
		cbs[ch.ID] = NewCircuitBreaker(ch.ID, ch.CBThreshold, 10*time.Second, 60*time.Second)
	}

	// Token bucket for global rate limit
	tb := NewTokenBucket(1000, 60000) // 1K RPM

	// Health checker
	hc := NewHealthChecker(HealthCheckModelsAPI, 30*time.Second)

	// Pricing store with defaults
	pricing := NewPricingStoreWithDefaults()

	// Billing hook
	seenIdem := make(map[string]bool)
	billingHook := NewBillingHook(pricing, &seenIdem)

	// Create router with full dependency graph
	r.router = NewRouterWithBilling(r.pool, lb, cbs, tb, hc, billingHook, "")

	// Register router with handlers
	handler.SetRouter(r.router)
	r.registerHandlers()

	r.initRouter(cfg != nil && cfg.Production)
	return r, nil
}

func (r *Relay) registerHandlers() {
	var pool types.ChannelPoolInterface = r.pool
	adapter := &channel.OpenAIAdapter{}
	imageHandler := handler.NewImagesHandler(&pool, adapter)
	audioHandler := handler.NewAudioHandler(&pool, adapter)
	batchHandler := handler.NewBatchHandler(&pool, adapter)
	filesHandler := handler.NewFilesHandler(&pool, adapter, "/tmp/oblivious-relay-files")
	assistantsHandler := handler.NewAssistantsHandler(adapter)

	r.handlers = map[types.APIType]types.Handler{
		types.APITypeChat:           handler.NewChatHandler(&pool, adapter),
		types.APITypeResponses:      handler.NewResponsesHandler(&pool, adapter),
		types.APITypeRealtime:       handler.NewRealtimeHandler(&pool, adapter),
		types.APITypeEmbeddings:     handler.NewEmbeddingsHandler(&pool, adapter),
		types.APITypeImageGen:       imageHandler,
		types.APITypeImageEdit:      imageHandler,
		types.APITypeImageVar:       imageHandler,
		types.APITypeAudioSpeech:    audioHandler,
		types.APITypeAudioSTT:       audioHandler,
		types.APITypeAudioTranslate: audioHandler,
		types.APITypeModeration:     handler.NewModerationsHandler(&pool, adapter),
		types.APITypeCompletions:    handler.NewLegacyCompletionsHandler(&pool, adapter),
		types.APITypeBatch:          batchHandler,
		types.APITypeFiles:          filesHandler,
		types.APITypeFineTuning:     handler.NewFineTuningHandler(adapter),
		types.APITypeAssistants:     assistantsHandler,
		types.APITypeThreads:        assistantsHandler,
		types.APITypeRuns:           assistantsHandler,
	}
}

func (r *Relay) initRouter(production bool) {
	r.engine = gin.New()
	handler.RegisterRoutesWithOptions(r.engine, r.handlers, handler.RouteRegistrationOptions{Production: production})
}

func (r *Relay) Engine() *gin.Engine {
	return r.engine
}

func (r *Relay) Router() *Router {
	return r.router
}

func (r *Relay) Run(addr string) error {
	return r.engine.Run(addr)
}
