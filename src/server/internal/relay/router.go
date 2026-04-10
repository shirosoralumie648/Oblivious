package relay

import (
	"context"
	"fmt"
	"net/http"

	"oblivious/server/internal/relay/types"
)

type Router struct {
	pool            *ChannelPool
	loadBalancer    *LoadBalancer
	circuitBreakers map[string]*CircuitBreaker
	tokenBucket     *TokenBucket
	healthChecker   *HealthChecker
}

func NewRouter(
	pool *ChannelPool,
	lb *LoadBalancer,
	cbs map[string]*CircuitBreaker,
	tb *TokenBucket,
	hc *HealthChecker,
) *Router {
	return &Router{
		pool:            pool,
		loadBalancer:    lb,
		circuitBreakers: cbs,
		tokenBucket:     tb,
		healthChecker:   hc,
	}
}

func (r *Router) SelectChannel(ctx context.Context, apiType string) *types.RouteChannel {
	// Check rate limit first
	if r.tokenBucket != nil {
		ok, _ := r.tokenBucket.TryAcquire("rpm")
		if !ok {
			return nil
		}
	}

	// Select via load balancer
	ch := r.loadBalancer.Select(apiType)
	if ch == nil {
		return nil
	}

	// Check circuit breaker
	if cb, ok := r.circuitBreakers[ch.Channel.ID]; ok {
		if cb.State() == StateOpen {
			return nil
		}
	}

	return ch
}

func (r *Router) Route(ctx context.Context, apiType string, fn func(ch *types.RouteChannel) (*types.ProviderResponse, error)) (*types.ProviderResponse, error) {
	ch := r.SelectChannel(ctx, apiType)
	if ch == nil {
		return nil, &RouterError{
			Code:       http.StatusServiceUnavailable,
			Message:    "no healthy channel available",
			RetryAfter: 30,
		}
	}

	resp, err := fn(ch)
	if err != nil {
		if cb, ok := r.circuitBreakers[ch.Channel.ID]; ok {
			cb.RecordFailure()
		}
		return nil, err
	}

	if resp != nil && resp.StatusCode >= 500 {
		if cb, ok := r.circuitBreakers[ch.Channel.ID]; ok {
			cb.RecordFailure()
		}
	} else if resp != nil && resp.StatusCode < 500 {
		if cb, ok := r.circuitBreakers[ch.Channel.ID]; ok {
			cb.RecordSuccess()
		}
	}

	return resp, nil
}

func (r *Router) RecordChannelSuccess(channelID string) {
	if cb, ok := r.circuitBreakers[channelID]; ok {
		cb.RecordSuccess()
	}
}

func (r *Router) RecordChannelFailure(channelID string) {
	if cb, ok := r.circuitBreakers[channelID]; ok {
		cb.RecordFailure()
	}
}

type RouterError struct {
	Code       int
	Message    string
	RetryAfter int
}

func (e *RouterError) Error() string {
	return fmt.Sprintf("router error %d: %s (retry after %ds)", e.Code, e.Message, e.RetryAfter)
}

func (e *RouterError) RetryAfterSeconds() int {
	return e.RetryAfter
}
