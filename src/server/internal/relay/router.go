package relay

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"oblivious/server/internal/metrics"
	"oblivious/server/internal/observability"
	"oblivious/server/internal/relay/types"
)

type Router struct {
	pool             *ChannelPool
	loadBalancer     *LoadBalancer
	circuitBreakers  map[string]*CircuitBreaker
	tokenBucket      *TokenBucket
	healthChecker    *HealthChecker
	billingHook      *BillingHook
	billingRedisAddr string
}

var relayObservabilityLogger = observability.NewJSONLogger(os.Stdout)
var relayObservabilityLoggerMu sync.RWMutex

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
		pool:             pool,
		loadBalancer:     lb,
		circuitBreakers:  cbs,
		tokenBucket:      tb,
		healthChecker:    hc,
		billingHook:      billingHook,
		billingRedisAddr: billingRedisAddr,
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
	ctx, span := observability.StartSpan(ctx, "relay.route", observability.String("relay.api_type", apiType))
	defer span.End()

	ch := r.SelectChannel(ctx, apiType)
	if ch == nil {
		return nil, &RouterError{
			Code:       http.StatusServiceUnavailable,
			Message:    "no healthy channel available",
			RetryAfter: 30,
		}
	}

	return r.executeOnChannel(ctx, ch, apiType, fn)
}

func (r *Router) executeOnChannel(ctx context.Context, ch *types.RouteChannel, apiType string, fn func(ch *types.RouteChannel) (*types.ProviderResponse, error)) (*types.ProviderResponse, error) {
	provider := providerForRouteChannel(ch)
	channelID := routeChannelID(ch)
	ctx, span := observability.StartSpan(
		ctx,
		"relay.provider_call",
		observability.String("provider", provider),
		observability.String("channel_id", channelID),
		observability.String("relay.api_type", apiType),
	)
	defer span.End()

	startedAt := time.Now()
	resp, err := fn(ch)
	duration := time.Since(startedAt)
	metrics.ObserveProviderRequestDuration(provider, channelID, apiType, duration.Seconds())
	if err != nil {
		if cb, ok := r.circuitBreakers[ch.Channel.ID]; ok {
			cb.RecordFailure()
		}
		metrics.RecordProviderFailure(provider, channelID, apiType, "request_error")
		logProviderFailure(ctx, provider, channelID, apiType, "request_error", duration)
		return nil, err
	}

	if resp != nil && resp.StatusCode >= 500 {
		if cb, ok := r.circuitBreakers[ch.Channel.ID]; ok {
			cb.RecordFailure()
		}
		metrics.RecordProviderFailure(provider, channelID, apiType, "provider_5xx")
		logProviderFailure(ctx, provider, channelID, apiType, "provider_5xx", duration)
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

func (r *Router) SetQuotaManager(manager QuotaManager) {
	if r.billingHook != nil {
		r.billingHook.SetQuotaManager(manager)
	}
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
	// Resolve trusted internal user identity for app-originated requests.
	userID, _ := types.TrustedUserIDFromContext(ctx)
	organizationID, _ := types.TrustedOrganizationIDFromContext(ctx)
	requestID, _ := types.TrustedRequestIDFromContext(ctx)

	ch := r.SelectChannel(ctx, apiType.String())
	if ch == nil {
		return nil, &RouterError{
			Code:       http.StatusServiceUnavailable,
			Message:    "no healthy channel available",
			RetryAfter: 30,
		}
	}
	selectedChannelID := routeChannelID(ch)

	// Create a single billing session carried through the full lifecycle:
	// PreBill -> PostBill (on success) or Refund (on failure).
	session := &BillingSession{
		ChannelID:      selectedChannelID,
		APIType:        apiType,
		Model:          model,
		IdempotencyKey: idempotencyKey,
		OrganizationID: organizationID,
		UserID:         userID,
		RequestID:      requestID,
	}
	if channelID != "" {
		session.ChannelID = channelID
	}

	// Pre-authorize billing
	if r.billingHook != nil {
		_, err := r.billingHook.PreBill(session, usage)
		if err != nil {
			return nil, &RouterError{
				Code:    http.StatusInternalServerError,
				Message: "billing pre-authorization failed: " + err.Error(),
			}
		}
	}

	// Route the request
	resp, err := r.executeOnChannel(ctx, ch, apiType.String(), fn)

	// Post-bill (settle) on success, or refund on failure
	if r.billingHook != nil {
		if err != nil {
			if refundErr := r.refundBillingSession(session); refundErr != nil {
				return nil, refundErr
			}
			// Also enqueue a timeout-based refund as a safety net for
			// cases where the upstream may have consumed resources
			// despite the client-side error.
			if r.billingRedisAddr != "" {
				timeoutTask := &BillingTimeoutTask{
					SessionID:      session.ID,
					ChannelID:      session.ChannelID,
					APIType:        apiType,
					Model:          model,
					AuthAmt:        session.PreAuthorizedAmt,
					IdempotencyKey: idempotencyKey,
					QuotaSessionID: session.QuotaSessionID,
					UserID:         userID,
					OrganizationID: organizationID,
				}
				EnqueueBillingTimeoutTask(r.billingRedisAddr, timeoutTask, 5*time.Minute)
			}
			return resp, err
		}

		if resp == nil {
			return nil, r.refundBillingFailure(session, "nil provider response", http.StatusBadGateway)
		}

		if resp.StatusCode >= http.StatusBadRequest {
			return resp, r.refundBillingFailure(session, fmt.Sprintf("provider error response status %d", resp.StatusCode), resp.StatusCode)
		}

		settlementUsage, usageErr := settlementUsageForResponse(apiType, usage, resp)
		if usageErr != nil {
			return resp, r.refundBillingFailure(session, usageErr.Error(), http.StatusBadGateway)
		}

		if _, err := r.billingHook.PostBill(session, settlementUsage); err != nil {
			return resp, &RouterError{
				Code:    http.StatusInternalServerError,
				Message: "billing settlement failed: " + err.Error(),
			}
		}
	}

	return resp, err
}

func (r *Router) refundBillingSession(session *BillingSession) error {
	if _, err := r.billingHook.Refund(session); err != nil {
		return &RouterError{
			Code:    http.StatusInternalServerError,
			Message: "billing refund failed: " + err.Error(),
		}
	}
	return nil
}

func (r *Router) refundBillingFailure(session *BillingSession, reason string, statusCode int) error {
	if err := r.refundBillingSession(session); err != nil {
		return err
	}
	if statusCode < http.StatusBadRequest {
		statusCode = http.StatusBadGateway
	}
	return &RouterError{
		Code:    statusCode,
		Message: "billing refund completed: " + reason,
	}
}

func routeChannelID(ch *types.RouteChannel) string {
	if ch == nil {
		return ""
	}
	if ch.ChannelID != "" {
		return ch.ChannelID
	}
	if ch.Channel != nil {
		return ch.Channel.ID
	}
	return ""
}

func providerForRouteChannel(ch *types.RouteChannel) string {
	if ch == nil || ch.Channel == nil || ch.Channel.Provider == "" {
		return "unknown"
	}
	return ch.Channel.Provider
}

func logProviderFailure(ctx context.Context, provider, channelID, apiType, reason string, latency time.Duration) {
	userID, _ := types.TrustedUserIDFromContext(ctx)
	organizationID, _ := types.TrustedOrganizationIDFromContext(ctx)
	requestID, _ := types.TrustedRequestIDFromContext(ctx)
	currentObservabilityLogger().Log(ctx, observability.Event{
		Component:      "relay",
		Event:          "relay.provider_failure",
		RequestID:      requestID,
		OrganizationID: organizationID,
		UserID:         userID,
		RelayAPIType:   apiType,
		ChannelID:      channelID,
		Provider:       provider,
		FailureReason:  reason,
		Latency:        latency,
	})
}

func currentObservabilityLogger() *observability.Logger {
	relayObservabilityLoggerMu.RLock()
	defer relayObservabilityLoggerMu.RUnlock()
	return relayObservabilityLogger
}

func setObservabilityLoggerForTest(logger *observability.Logger) func() {
	relayObservabilityLoggerMu.Lock()
	previous := relayObservabilityLogger
	relayObservabilityLogger = logger
	relayObservabilityLoggerMu.Unlock()

	return func() {
		relayObservabilityLoggerMu.Lock()
		relayObservabilityLogger = previous
		relayObservabilityLoggerMu.Unlock()
	}
}

type billingSettlementPolicy int

const (
	billingSettlementRequiresUsage billingSettlementPolicy = iota
	billingSettlementAllowsEstimate
	billingSettlementProductionDisabled
)

func settlementUsageForResponse(apiType types.APIType, estimate *types.Usage, resp *types.ProviderResponse) (*types.Usage, error) {
	if resp != nil && resp.Usage != nil {
		return resp.Usage, nil
	}

	switch settlementPolicyForAPIType(apiType) {
	case billingSettlementAllowsEstimate:
		if estimate != nil {
			return estimate, nil
		}
		return nil, fmt.Errorf("missing settlement estimate")
	case billingSettlementRequiresUsage:
		return nil, fmt.Errorf("missing required usage")
	default:
		return nil, fmt.Errorf("billing settlement policy is production disabled")
	}
}

func settlementPolicyForAPIType(apiType types.APIType) billingSettlementPolicy {
	switch apiType {
	case types.APITypeImageGen, types.APITypeImageEdit, types.APITypeImageVar,
		types.APITypeAudioSpeech, types.APITypeAudioSTT, types.APITypeAudioTranslate,
		types.APITypeModeration:
		return billingSettlementAllowsEstimate
	case types.APITypeChat, types.APITypeResponses, types.APITypeEmbeddings, types.APITypeCompletions:
		return billingSettlementRequiresUsage
	default:
		return billingSettlementProductionDisabled
	}
}
