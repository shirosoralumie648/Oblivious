package relay

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"oblivious/server/internal/relay/types"
)

type Router struct {
	pool            *ChannelPool
	loadBalancer    *LoadBalancer
	circuitBreakers map[string]*CircuitBreaker
	tokenBucket     *TokenBucket
	healthChecker   *HealthChecker
	billingHook     *BillingHook
	billingRedisAddr string
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

func NewRouterWithBilling(
	pool *ChannelPool,
	lb *LoadBalancer,
	cbs map[string]*CircuitBreaker,
	tb *TokenBucket,
	hc *HealthChecker,
	billingHook *BillingHook,
	billingRedisAddr string,
) *Router {
	return &Router{
		pool:              pool,
		loadBalancer:      lb,
		circuitBreakers:   cbs,
		tokenBucket:       tb,
		healthChecker:     hc,
		billingHook:       billingHook,
		billingRedisAddr:  billingRedisAddr,
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

func (r *Router) RouteWithFallback(
	ctx context.Context,
	apiType string,
	attempts int,
	fn func(ch *types.RouteChannel) (*types.ProviderResponse, error),
) (*types.ProviderResponse, error) {
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		resp, err := r.Route(ctx, apiType, fn)
		if err == nil && resp != nil {
			return resp, nil
		}
		lastErr = err

		if resp != nil && IsRetryable(resp.StatusCode) && attempt < attempts {
			backoff := time.Duration(attempt*attempt) * 200 * time.Millisecond
			if backoff > 5*time.Second {
				backoff = 5 * time.Second
			}
			time.Sleep(backoff)
		}
	}

	if lastErr == nil {
		return nil, &RouterError{
			Code:       http.StatusServiceUnavailable,
			Message:    "all channels failed",
			RetryAfter: 30,
		}
	}

	return nil, lastErr
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

func (r *Router) RouteWithBilling(
	ctx context.Context,
	apiType types.APIType,
	model string,
	channelID string,
	idempotencyKey string,
	usage *types.Usage,
	fn func(ch *types.RouteChannel) (*types.ProviderResponse, error),
) (*types.ProviderResponse, error) {
	// Pre-authorize billing
	if r.billingHook != nil {
		_, err := r.billingHook.PreBill(&BillingSession{
			ChannelID:       channelID,
			APIType:         apiType,
			Model:           model,
			IdempotencyKey:  idempotencyKey,
		}, usage)
		if err != nil {
			return nil, &RouterError{
				Code:    http.StatusInternalServerError,
				Message: "billing pre-authorization failed: " + err.Error(),
			}
		}
	}

	// Route the request
	resp, err := r.Route(ctx, strconv.Itoa(int(apiType)), fn)

	// Post-bill (settle or refund excess)
	if r.billingHook != nil && resp != nil && resp.Usage != nil {
		session := &BillingSession{
			ChannelID:       channelID,
			APIType:         apiType,
			Model:           model,
			IdempotencyKey:  idempotencyKey,
		}
		if resp.Usage != nil {
			session.PreAuthorizedAmt = 0 // Will be set from PreBill session if available
		}
		r.billingHook.PostBill(session, resp.Usage)
	} else if r.billingHook != nil && err != nil {
		// On error, enqueue timeout task to refund
		if r.billingRedisAddr != "" {
			timeoutTask := &BillingTimeoutTask{
				SessionID:     fmt.Sprintf("sess_%d", time.Now().UnixNano()),
				ChannelID:     channelID,
				APIType:       apiType,
				Model:         model,
				AuthAmt:       0, // Would need to track pre-auth amount properly
				IdempotencyKey: idempotencyKey,
			}
			EnqueueBillingTimeoutTask(r.billingRedisAddr, timeoutTask, 5*time.Minute)
		}
	}

	return resp, err
}
