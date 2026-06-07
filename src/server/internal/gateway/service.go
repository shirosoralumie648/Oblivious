package gateway

import (
	"encoding/json"
	"fmt"
	stdhttp "net/http"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Service is the core gateway that routes incoming requests to downstream
// microservices. It implements stdhttp.Handler.
type Service struct {
	routes         []RouteEntry
	middleware     []Middleware
	healthChecker  *HealthAggregator
	circuitBreaker *CircuitBreaker
	rateLimiter    *SlidingWindowLimiter
}

// Middleware is a function that wraps an stdhttp.Handler.
type Middleware func(stdhttp.Handler) stdhttp.Handler

// ServiceConfig holds all dependencies needed to construct a gateway Service.
type ServiceConfig struct {
	JWTSecret          []byte
	AllowedOrigins     []string
	RedisClient        *redis.Client
	RateLimitRPM       int
	RateLimitTPM       int
	RateLimitOrgRPM    int
	RateLimitOrgTPM    int
	CBThreshold        float64
	CBOpenDuration     time.Duration
	HealthCheckTargets map[ServiceTarget]string
	HealthCheckTimeout time.Duration
}

// NewService creates a new gateway Service. Routes are registered after
// construction via RegisterRoute.
func NewService(cfg ServiceConfig) *Service {
	var rateLimiter *SlidingWindowLimiter
	if cfg.RedisClient != nil {
		rateLimiter = NewSlidingWindowLimiter(cfg.RedisClient, SlidingWindowConfig{
			DefaultRPM:    cfg.RateLimitRPM,
			DefaultTPM:    cfg.RateLimitTPM,
			DefaultOrgRPM: cfg.RateLimitOrgRPM,
			DefaultOrgTPM: cfg.RateLimitOrgTPM,
		})
	}

	cbThreshold := cfg.CBThreshold
	if cbThreshold <= 0 {
		cbThreshold = 50.0
	}
	cbOpenDuration := cfg.CBOpenDuration
	if cbOpenDuration <= 0 {
		cbOpenDuration = 30 * time.Second
	}

	svc := &Service{
		circuitBreaker: NewCircuitBreaker(cbThreshold, cbOpenDuration),
		rateLimiter:    rateLimiter,
	}

	// Build default middleware chain.
	svc.middleware = []Middleware{
		WithRequestID,
		WithLogging,
		WithCORS(cfg.AllowedOrigins),
	}
	if len(cfg.JWTSecret) > 0 {
		svc.middleware = append(svc.middleware, WithJWTAuth(cfg.JWTSecret))
	}
	if rateLimiter != nil {
		svc.middleware = append(svc.middleware, WithRateLimit(rateLimiter))
	}

	// Health aggregator.
	timeout := cfg.HealthCheckTimeout
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	svc.healthChecker = NewHealthAggregator(cfg.HealthCheckTargets, timeout)

	return svc
}

// RegisterRoute adds a route entry mapping a URL prefix to a downstream handler.
func (s *Service) RegisterRoute(prefix string, target ServiceTarget, handler stdhttp.Handler) {
	s.routes = append(s.routes, RouteEntry{
		Prefix:  prefix,
		Target:  target,
		Handler: handler,
	})
}

// RegisterMiddleware prepends a middleware to the chain (applied outermost first).
func (s *Service) RegisterMiddleware(mw Middleware) {
	s.middleware = append([]Middleware{mw}, s.middleware...)
}

// ServeHTTP is the unified entry point for all gateway traffic.
func (s *Service) ServeHTTP(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	// Health check endpoint bypasses routing and auth.
	if r.URL.Path == "/health" || r.URL.Path == "/healthz" {
		s.handleHealth(w, r)
		return
	}

	// Apply the middleware chain then route.
	handler := s.applyMiddleware(stdhttp.HandlerFunc(s.route))
	handler.ServeHTTP(w, r)
}

// route selects the downstream handler based on URL prefix.
func (s *Service) route(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	path := r.URL.Path

	for _, entry := range s.routes {
		if strings.HasPrefix(path, entry.Prefix) {
			// Check circuit breaker for this service target.
			if !s.circuitBreaker.Allow(entry.Target) {
				writeGatewayError(w, stdhttp.StatusServiceUnavailable, "service_unavailable",
					fmt.Sprintf("service %s is temporarily unavailable", entry.Target))
				return
			}

			// Wrap the downstream handler to record success/failure for the circuit breaker.
			recorder := &gatewayStatusRecorder{ResponseWriter: w, status: stdhttp.StatusOK}
			entry.Handler.ServeHTTP(recorder, r)

			if recorder.status >= 500 {
				s.circuitBreaker.RecordFailure(entry.Target)
			} else {
				s.circuitBreaker.RecordSuccess(entry.Target)
			}
			return
		}
	}

	writeGatewayError(w, stdhttp.StatusNotFound, "not_found", "no route matched for path: "+path)
}

// applyMiddleware wraps the handler with the middleware chain.
func (s *Service) applyMiddleware(handler stdhttp.Handler) stdhttp.Handler {
	wrapped := handler
	for i := len(s.middleware) - 1; i >= 0; i-- {
		wrapped = s.middleware[i](wrapped)
	}
	return wrapped
}

// handleHealth serves the aggregated health check.
func (s *Service) handleHealth(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	if r.Method != stdhttp.MethodGet {
		writeGatewayError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	ctx := r.Context()
	result := s.healthChecker.Check(ctx)

	status := stdhttp.StatusOK
	if result.Status != "ok" {
		status = stdhttp.StatusServiceUnavailable
	}

	writeGatewayJSON(w, status, result)
}

// gatewayStatusRecorder captures the response status code for circuit breaker tracking.
type gatewayStatusRecorder struct {
	stdhttp.ResponseWriter
	status int
}

func (r *gatewayStatusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// --- JSON response helpers ---

func writeGatewayJSON(w stdhttp.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":    true,
		"data":  data,
		"error": nil,
	})
}

func writeGatewayError(w stdhttp.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":   false,
		"data": nil,
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
