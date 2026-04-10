package relay

import (
	"time"

	"github.com/gin-gonic/gin"
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
	Pool *ChannelPool
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

	r.initRouter()
	return r, nil
}

func (r *Relay) initRouter() {
	r.engine = gin.New()
	handler.RegisterRoutes(r.engine, r.handlers)
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
