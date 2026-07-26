package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"oblivious/server/internal/metrics"
	relaycache "oblivious/server/internal/relay/cache"
	"oblivious/server/internal/relay/ratelimit"
	"oblivious/server/internal/relay/types"
	"oblivious/server/internal/releasecontract"
)

const maxRouteBillingAttempts = 4

type ConversationAffinityStore interface {
	SaveConversationAffinity(ctx context.Context, conversationID, channelID string) error
	GetConversationAffinity(ctx context.Context, conversationID string) (string, error)
}

type Router struct {
	pool                   *ChannelPool
	loadBalancer           *LoadBalancer
	circuitBreakers        map[string]*CircuitBreaker
	tokenBucket            *TokenBucket
	healthChecker          *HealthChecker
	billingHook            *BillingHook
	billingRedisAddr       string
	rateLimiter            ratelimit.RateLimiter
	rateLimitResolver      RateLimitResolver
	affinityStore          ConversationAffinityStore
	semanticCache          *relaycache.SemanticCache
	quotaManager           QuotaManager
	apiTokenQuotaManager   APITokenQuotaManager
	usageLogger            UsageLogger
	retrySleep             func(time.Duration)
	readiness              *routerReadiness
	compensationStore      QuotaCompensationStore
	routeAttemptIDGen      func() (string, error)
}

// RouterRuntimeOptions is the one startup-built readiness carrier accepted by
// the Relay router. The router never loads a contract or selects capability
// IDs from request/persisted data.
type RouterRuntimeOptions struct {
	Guard       releasecontract.Guard
	Authorities releasecontract.RuntimeAuthorities
	Effects     releasecontract.EffectRegistrar
}

type routerReadiness struct {
	guard          releasecontract.Guard
	authorities    releasecontract.RuntimeAuthorities
	providerEffect releasecontract.CapabilityID
}

func newRouterReadiness(options RouterRuntimeOptions) (*routerReadiness, error) {
	if options.Guard == nil || options.Effects == nil || !options.Authorities.Valid() {
		return nil, &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessUnavailable, Field: "relay.router"}
	}
	providerEffect, err := options.Authorities.CapabilityBindings.Resolve(releasecontract.EffectRelayProvider)
	if err != nil {
		return nil, err
	}
	if err := options.Effects.Register(releasecontract.EffectDescriptor{
		ID:           "relay.provider.dispatch",
		CapabilityID: string(providerEffect),
		Boundary:     releasecontract.BoundaryOutbound,
		Owner:        "relay.Router",
	}); err != nil {
		return nil, err
	}
	return &routerReadiness{guard: options.Guard, authorities: options.Authorities, providerEffect: providerEffect}, nil
}

func (r *routerReadiness) requireModel(ctx context.Context, model string) error {
	if r == nil {
		return nil
	}
	model = strings.TrimSpace(model)
	if model == "" {
		return &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityUnknown, Field: "relay.model"}
	}
	capabilityID, err := r.authorities.CatalogAuthorizer.ResolveAndRequire(ctx, releasecontract.CatalogSubject{
		Kind:    releasecontract.CatalogSubjectModel,
		ID:      model,
		Runtime: releasecontract.CatalogRuntimeServerModel,
	}, releasecontract.BoundaryOutbound)
	if err != nil {
		return err
	}
	if capabilityID != r.providerEffect {
		return &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityUnknown, Field: "relay.modelCapability"}
	}
	return nil
}

func (r *routerReadiness) requireProvider(ctx context.Context) error {
	if r == nil {
		return nil
	}
	return r.guard.Require(ctx, string(r.providerEffect), releasecontract.BoundaryOutbound)
}

// NewRouterWithOptions constructs a Relay router with immutable startup
// readiness authority. It fails closed when the carrier or provider binding is
// incomplete.
func NewRouterWithOptions(pool *ChannelPool, lb *LoadBalancer, cbs map[string]*CircuitBreaker, tb *TokenBucket, hc *HealthChecker, options RouterRuntimeOptions) (*Router, error) {
	readiness, err := newRouterReadiness(options)
	if err != nil {
		return nil, err
	}
	router := NewRouter(pool, lb, cbs, tb, hc)
	router.readiness = readiness
	return router, nil
}

// NewRouterWithBillingOptions is the billing-enabled counterpart used by the
// runtime composition layer.
func NewRouterWithBillingOptions(pool *ChannelPool, lb *LoadBalancer, cbs map[string]*CircuitBreaker, tb *TokenBucket, hc *HealthChecker, billingHook *BillingHook, billingRedisAddr string, options RouterRuntimeOptions) (*Router, error) {
	readiness, err := newRouterReadiness(options)
	if err != nil {
		return nil, err
	}
	router := NewRouterWithBilling(pool, lb, cbs, tb, hc, billingHook, billingRedisAddr)
	router.readiness = readiness
	return router, nil
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
	if r.tokenBucket != nil {
		ok, _ := r.tokenBucket.TryAcquire("rpm")
		if !ok {
			return nil
		}
	}

	organizationID, _ := types.TrustedOrganizationIDFromContext(ctx)
	ch := r.loadBalancer.SelectForOrganization(apiType, organizationID)
	if ch == nil {
		return nil
	}

	if cb, ok := r.circuitBreakers[ch.Channel.ID]; ok && cb.State() == StateOpen {
		return nil
	}

	return ch
}

func (r *Router) requireModel(ctx context.Context, model string) error {
	if r == nil || r.readiness == nil {
		return nil
	}
	return r.readiness.requireModel(ctx, model)
}

func (r *Router) requireProvider(ctx context.Context) error {
	if r == nil || r.readiness == nil {
		return nil
	}
	return r.readiness.requireProvider(ctx)
}

func readinessRouteModel(ch *types.RouteChannel) (string, error) {
	if ch == nil || ch.Channel == nil || len(ch.Channel.Models) != 1 || strings.TrimSpace(ch.Channel.Models[0]) == "" {
		return "", &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityUnknown, Field: "relay.routeModel"}
	}
	return strings.TrimSpace(ch.Channel.Models[0]), nil
}

func (r *Router) selectChannelForBilling(ctx context.Context, apiType, model string, usage *types.Usage, excluded map[string]bool) *types.RouteChannel {
	organizationID, _ := types.TrustedOrganizationIDFromContext(ctx)
	conversationID, _ := types.TrustedConversationIDFromContext(ctx)
	if conversationID != "" && r.affinityStore != nil {
		if channelID, err := r.affinityStore.GetConversationAffinity(ctx, conversationID); err == nil && channelID != "" && !excluded[channelID] {
			if ch := r.loadBalancer.SelectModelChannelByIDForOrganization(apiType, model, channelID, organizationID); ch != nil {
				if cb, ok := r.circuitBreakers[routeChannelID(ch)]; !ok || cb.State() != StateOpen {
					return ch
				}
			}
		}
	}

	var ch *types.RouteChannel
	if r.rateLimiter != nil {
		ch = r.loadBalancer.SelectModelExcludingWithWeightsForOrganization(apiType, model, organizationID, excluded, r.localRateLimitWeightAdjuster(ctx, model, usage))
	} else {
		ch = r.loadBalancer.SelectModelExcludingForOrganization(apiType, model, organizationID, excluded)
	}
	if ch == nil {
		return nil
	}
	if cb, ok := r.circuitBreakers[routeChannelID(ch)]; ok && cb.State() == StateOpen {
		excluded[routeChannelID(ch)] = true
		return r.selectChannelForBilling(ctx, apiType, model, usage, excluded)
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
			ErrorCode:  "no_available_channel",
		}
	}
	if r.readiness != nil {
		model, err := readinessRouteModel(ch)
		if err != nil {
			return nil, err
		}
		if err := r.requireModel(ctx, model); err != nil {
			return nil, err
		}
	}
	if err := r.requireProvider(ctx); err != nil {
		return nil, err
	}

	resp, err := fn(ch)
	if err != nil {
		r.recordSelectedFailure(routeChannelID(ch))
		return nil, err
	}

	if resp != nil && resp.StatusCode >= 500 {
		r.recordSelectedFailure(routeChannelID(ch))
	} else if resp != nil && resp.StatusCode < 500 {
		r.recordSelectedSuccess(routeChannelID(ch))
	}

	return resp, nil
}

func (r *Router) RecordChannelSuccess(channelID string) {
	r.recordSelectedSuccess(channelID)
}

func (r *Router) RecordChannelFailure(channelID string) {
	r.recordSelectedFailure(channelID)
}

func (r *Router) RouteWithFallback(
	ctx context.Context,
	apiType string,
	attempts int,
	fn func(ch *types.RouteChannel) (*types.ProviderResponse, error),
) (*types.ProviderResponse, error) {
	var lastErr error
	var lastResp *types.ProviderResponse
	for attempt := 1; attempt <= attempts; attempt++ {
		resp, err := r.Route(ctx, apiType, fn)
		if err == nil && resp != nil && !IsRetryable(resp.StatusCode) {
			return resp, nil
		}
		lastErr = err
		if resp != nil {
			lastResp = resp
		}

		if resp != nil && IsRetryable(resp.StatusCode) && attempt < attempts {
			r.sleepBeforeRetry(attempt)
		}
	}

	if lastErr == nil {
		if lastResp != nil {
			return lastResp, nil
		}
		return nil, &RouterError{
			Code:       http.StatusServiceUnavailable,
			Message:    "all channels failed",
			RetryAfter: 30,
			ErrorCode:  "no_available_channel",
		}
	}

	return nil, lastErr
}

type RouterError struct {
	Code       int
	Message    string
	RetryAfter int
	ErrorCode  string
}

func (e *RouterError) Error() string {
	return fmt.Sprintf("router error %d: %s (retry after %ds)", e.Code, e.Message, e.RetryAfter)
}

func (e *RouterError) RetryAfterSeconds() int {
	return e.RetryAfter
}

func (e *RouterError) StatusCode() int {
	return e.Code
}

func (e *RouterError) RelayErrorCode() string {
	return e.ErrorCode
}

type relayErrorCoder interface {
	RelayErrorCode() string
}

func upstreamUsageErrorCode(err error) string {
	var coded relayErrorCoder
	if errors.As(err, &coded) {
		if code := strings.TrimSpace(coded.RelayErrorCode()); code != "" {
			return code
		}
	}
	return "upstream_error"
}

func (r *Router) SetQuotaManager(manager QuotaManager) {
	r.quotaManager = manager
}

func (r *Router) SetAPITokenQuotaManager(manager APITokenQuotaManager) {
	r.apiTokenQuotaManager = manager
}

func (r *Router) SetUsageLogger(logger UsageLogger) {
	r.usageLogger = logger
}

func (r *Router) SetRateLimitResolver(resolver RateLimitResolver) {
	r.rateLimitResolver = resolver
}

// SetCompensationStore wires the durable quota compensation store into the
// router. When set, late readiness denials commit a compensation job before
// any immediate refund attempt.
func (r *Router) SetCompensationStore(store QuotaCompensationStore) {
	r.compensationStore = store
}

// generateRouteAttemptID returns a cryptographically random route-attempt
// identity or the result of an injected generator (used in tests).
func (r *Router) generateRouteAttemptID() (string, error) {
	if r.routeAttemptIDGen != nil {
		return r.routeAttemptIDGen()
	}
	return GenerateRouteAttemptID()
}

// compensateLateReadiness arms the durable job and attempts all required quota
// refunds independently, returning the original readiness error joined with any
// stable compensation/persistence failures.
func (r *Router) compensateLateReadiness(
	ctx context.Context,
	readinessErr error,
	stage string,
	routeAttemptID string,
	organizationID string,
	billingSessionID string,
	apiTokenID string,
	amount float64,
	idempotencyKey string,
) error {
	req := QuotaCompensationRequest{
		RouteAttemptID:       routeAttemptID,
		Stage:                stage,
		OrganizationID:       organizationID,
		BillingSessionID:     billingSessionID,
		APITokenID:           apiTokenID,
		Amount:               amount,
		CallerIdempotencyKey: idempotencyKey,
	}
	coordinator := NewQuotaCompensationCoordinator(r.compensationStore, r.quotaManager, r.apiTokenQuotaManager)
	return coordinator.CompensateLateReadiness(ctx, readinessErr, req)
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
	startedAt := time.Now()
	requestID, _ := types.TrustedRequestIDFromContext(ctx)
	userID, _ := types.TrustedUserIDFromContext(ctx)
	organizationID, _ := types.TrustedOrganizationIDFromContext(ctx)
	apiTokenID, _ := types.TrustedAPITokenIDFromContext(ctx)
	conversationID, _ := types.TrustedConversationIDFromContext(ctx)
	featureType, _ := types.TrustedFeatureTypeFromContext(ctx)
	userGroup, _ := types.TrustedUserGroupFromContext(ctx)
	streamingResponse, _ := types.TrustedStreamingFromContext(ctx)

	// Mint a mandatory server-generated route-attempt identity before either
	// organization or API-token preauthorization begins. A generation failure
	// returns a stable sanitized error before any reservation side effect.
	routeAttemptID, idGenErr := r.generateRouteAttemptID()
	if idGenErr != nil {
		return nil, fmt.Errorf("route_attempt_identity_generation_failed")
	}

	if err := r.requireModel(ctx, model); err != nil {
		return nil, err
	}

	if cacheReq, ok := types.SemanticCacheRequestFromContext(ctx); ok && r.semanticCache != nil {
		if strings.TrimSpace(cacheReq.Model) != strings.TrimSpace(model) {
			return nil, &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityUnknown, Field: "relay.semanticCacheModel"}
		}
		hit, err := r.semanticCache.Lookup(ctx, cacheReq)
		if err != nil {
			return nil, err
		}
		metrics.RecordRelaySemanticCacheLookup(apiType.String(), model, hit != nil)
		if hit != nil {
			_ = r.recordUsage(ctx, RelayUsageLogRecord{
				UserID:         userID,
				OrganizationID: organizationID,
				APITokenID:     apiTokenID,
				RequestID:      requestID,
				APIType:        apiType.String(),
				FeatureType:    featureType,
				Model:          model,
				Provider:       "semantic_cache",
				Status:         RelayUsageStatusSuccess,
				StatusCode:     http.StatusOK,
				LatencyMS:      time.Since(startedAt).Milliseconds(),
				CreatedAt:      time.Now().UTC(),
			})
			r.recordRelayRuntimeMetrics("semantic_cache", "semantic_cache", apiType, model, http.StatusOK, "hit", startedAt)
			return types.NewOKResponse(hit.Response, nil), nil
		}
	}

	excludedChannels := make(map[string]bool)
	var lastErr error
	var lastResp *types.ProviderResponse
	for attempt := 1; attempt <= maxRouteBillingAttempts; attempt++ {
		ch := r.selectChannelForBilling(ctx, apiType.String(), model, usage, excludedChannels)
		if ch == nil {
			routeErr := &RouterError{Code: http.StatusServiceUnavailable, Message: "no healthy channel available", RetryAfter: 30, ErrorCode: "no_available_channel"}
			_ = r.recordUsage(ctx, r.errorUsageRecord(userID, organizationID, apiTokenID, requestID, apiType, model, nil, routeErr.Code, routeErr.ErrorCode, startedAt))
			r.recordRelayRuntimeMetrics("router", "none", apiType, model, routeErr.Code, "miss", startedAt)
			if lastErr != nil {
				return nil, lastErr
			}
			if lastResp != nil {
				return lastResp, nil
			}
			return nil, routeErr
		}
		selectedChannelID := routeChannelID(ch)
		billingChannelID := channelID
		if billingChannelID == "" {
			billingChannelID = selectedChannelID
		}

		rateLimitChecks := rateLimitChecksForResolution(r.resolveRateLimit(ctx, ch, model, usage))
		releaseRateLimit := func() {}
		if r.rateLimiter != nil && len(rateLimitChecks) > 0 {
			begunRateLimitKeys := []ratelimit.Key{}
			for _, check := range rateLimitChecks {
				if rateLimitCheckEmpty(check) || check.Limits.MaxConcurrent <= 0 {
					continue
				}
				if err := r.rateLimiter.Begin(ctx, check.Key, check.Limits); err != nil {
					for i := len(begunRateLimitKeys) - 1; i >= 0; i-- {
						_ = r.rateLimiter.End(ctx, begunRateLimitKeys[i])
					}
					routeErr := rateLimitRouterError(err)
					_ = r.recordUsage(ctx, r.errorUsageRecord(userID, organizationID, apiTokenID, requestID, apiType, model, ch, routeErr.Code, routeErr.ErrorCode, startedAt))
					r.recordRelayRuntimeMetricsForChannel(ch, apiType, model, routeErr.Code, "miss", startedAt)
					return nil, routeErr
				}
				begunRateLimitKeys = append(begunRateLimitKeys, check.Key)
			}
			releaseRateLimit = func() {
				for i := len(begunRateLimitKeys) - 1; i >= 0; i-- {
					_ = r.rateLimiter.End(ctx, begunRateLimitKeys[i])
				}
			}
			for _, check := range rateLimitChecks {
				if rateLimitCheckEmpty(check) {
					continue
				}
				if err := r.rateLimiter.Allow(ctx, check.Key, allowOnlyLimits(check.Limits), check.Usage); err != nil {
					releaseRateLimit()
					routeErr := rateLimitRouterError(err)
					_ = r.recordUsage(ctx, r.errorUsageRecord(userID, organizationID, apiTokenID, requestID, apiType, model, ch, routeErr.Code, routeErr.ErrorCode, startedAt))
					r.recordRelayRuntimeMetricsForChannel(ch, apiType, model, routeErr.Code, "miss", startedAt)
					return nil, routeErr
				}
			}
		}

		attemptIdempotencyKey := attemptScopedIdempotencyKey(idempotencyKey, attempt)
		var billingSessionID string
		if r.quotaManager != nil {
			preauthQuote, err := r.estimatedUsagePriceQuote(model, apiType, usage, ch, userGroup)
			if err != nil {
				releaseRateLimit()
				routeErr := &RouterError{Code: http.StatusInternalServerError, Message: "relay pricing is not configured: " + err.Error(), ErrorCode: "relay_pricing_not_configured"}
				_ = r.recordUsage(ctx, r.errorUsageRecord(userID, organizationID, apiTokenID, requestID, apiType, model, ch, routeErr.Code, routeErr.ErrorCode, startedAt))
				r.recordRelayRuntimeMetricsForChannel(ch, apiType, model, routeErr.Code, "miss", startedAt)
				return nil, routeErr
			}
			session, err := r.quotaManager.PreConsume(ctx, userID, organizationID, preauthQuote.TotalCost, attemptIdempotencyKey, billingChannelID, model, apiType.String())
			if err != nil {
				releaseRateLimit()
				routeErr := &RouterError{Code: http.StatusPaymentRequired, Message: "billing pre-authorization failed: " + err.Error(), ErrorCode: "billing_pre_authorization_failed"}
				_ = r.recordUsage(ctx, r.errorUsageRecord(userID, organizationID, apiTokenID, requestID, apiType, model, ch, routeErr.Code, routeErr.ErrorCode, startedAt))
				r.recordRelayRuntimeMetricsForChannel(ch, apiType, model, routeErr.Code, "miss", startedAt)
				return nil, routeErr
			}
			if session != nil {
				billingSessionID = session.ID
			}
		}

		if r.billingHook != nil {
			_, err := r.billingHook.PreBill(&BillingSession{
				ChannelID:      billingChannelID,
				APIType:        apiType,
				Model:          model,
				UserGroup:      userGroup,
				IdempotencyKey: attemptIdempotencyKey,
			}, usage)
			if err != nil {
				releaseRateLimit()
				routeErr := billingPreAuthorizationRouterError(err)
				_ = r.recordUsage(ctx, r.errorUsageRecord(userID, organizationID, apiTokenID, requestID, apiType, model, ch, routeErr.Code, routeErr.ErrorCode, startedAt))
				r.recordRelayRuntimeMetricsForChannel(ch, apiType, model, routeErr.Code, "miss", startedAt)
				return nil, routeErr
			}
		}

		tokenPreauthorizedQuote, err := r.estimatedUsagePriceQuote(model, apiType, usage, ch, userGroup)
		if err != nil {
			releaseRateLimit()
			if r.quotaManager != nil && billingSessionID != "" {
				_ = r.quotaManager.Refund(ctx, organizationID, billingSessionID)
			}
			routeErr := &RouterError{Code: http.StatusInternalServerError, Message: "relay pricing is not configured: " + err.Error(), ErrorCode: "relay_pricing_not_configured"}
			_ = r.recordUsage(ctx, r.errorUsageRecord(userID, organizationID, apiTokenID, requestID, apiType, model, ch, routeErr.Code, routeErr.ErrorCode, startedAt))
			r.recordRelayRuntimeMetricsForChannel(ch, apiType, model, routeErr.Code, "miss", startedAt)
			return nil, routeErr
		}
		tokenPreauthorizedAmount := tokenPreauthorizedQuote.TotalCost
		if r.apiTokenQuotaManager != nil && apiTokenID != "" {
			if err := r.apiTokenQuotaManager.PreAuthorizeRelayAPITokenQuota(ctx, apiTokenID, tokenPreauthorizedAmount); err != nil {
				releaseRateLimit()
				if r.quotaManager != nil && billingSessionID != "" {
					_ = r.quotaManager.Refund(ctx, organizationID, billingSessionID)
				}
				status, code := relayAPITokenQuotaRouterError(err)
				routeErr := &RouterError{Code: status, Message: "API token quota pre-authorization failed: " + err.Error(), ErrorCode: code}
				_ = r.recordUsage(ctx, r.errorUsageRecord(userID, organizationID, apiTokenID, requestID, apiType, model, ch, routeErr.Code, routeErr.ErrorCode, startedAt))
				r.recordRelayRuntimeMetricsForChannel(ch, apiType, model, routeErr.Code, "miss", startedAt)
				return nil, routeErr
			}
		}

		if streamingResponse {
			pendingRecord := r.successUsageRecord(userID, organizationID, apiTokenID, requestID, apiType, model, ch, usage, tokenPreauthorizedQuote, startedAt)
			pendingRecord.Status = RelayUsageStatusPending
			pendingRecord.StatusCode = 0
			if err := r.recordUsage(ctx, pendingRecord); err != nil {
				releaseRateLimit()
				if r.quotaManager != nil && billingSessionID != "" {
					_ = r.quotaManager.Refund(ctx, organizationID, billingSessionID)
				}
				if r.apiTokenQuotaManager != nil && apiTokenID != "" {
					_ = r.apiTokenQuotaManager.RefundRelayAPITokenQuota(ctx, apiTokenID, tokenPreauthorizedAmount)
				}
				routeErr := &RouterError{Code: http.StatusInternalServerError, Message: "usage recording failed: " + err.Error(), ErrorCode: "usage_recording_failed"}
				r.recordRelayRuntimeMetricsForChannel(ch, apiType, model, routeErr.Code, "miss", startedAt)
				return nil, routeErr
			}
		}

		// A readiness generation may expire while billing/pending usage work is
		// in progress. Re-authorize immediately before the provider callback and
		// only refund effects that have already started.
		if err := r.requireModel(ctx, model); err != nil {
			releaseRateLimit()
			return nil, r.compensateLateReadiness(ctx, err, QuotaCompensationStageLateModelReadiness,
				routeAttemptID, organizationID, billingSessionID, apiTokenID, tokenPreauthorizedAmount, attemptIdempotencyKey)
		}
		if err := r.requireProvider(ctx); err != nil {
			releaseRateLimit()
			return nil, r.compensateLateReadiness(ctx, err, QuotaCompensationStageLateProviderReadiness,
				routeAttemptID, organizationID, billingSessionID, apiTokenID, tokenPreauthorizedAmount, attemptIdempotencyKey)
		}

		resp, err := fn(ch)
		releaseRateLimit()
		if err != nil {
			r.recordSelectedFailure(selectedChannelID)
			if r.quotaManager != nil && billingSessionID != "" {
				_ = r.quotaManager.Refund(ctx, organizationID, billingSessionID)
			}
			if r.apiTokenQuotaManager != nil && apiTokenID != "" {
				_ = r.apiTokenQuotaManager.RefundRelayAPITokenQuota(ctx, apiTokenID, tokenPreauthorizedAmount)
			}
			errorRecord := r.errorUsageRecord(userID, organizationID, apiTokenID, requestID, apiType, model, ch, http.StatusBadGateway, upstreamUsageErrorCode(err), startedAt)
			if streamingResponse {
				_ = r.replaceUsage(ctx, errorRecord)
				recordRequestLogBillingMetadata(ctx, model, billingSessionID, tokenPreauthorizedQuote, tokenPreauthorizedAmount, errorRecord)
				r.recordRelayRuntimeMetricsForChannel(ch, apiType, model, errorRecord.StatusCode, "miss", startedAt)
				return nil, err
			}
			_ = r.recordUsage(ctx, errorRecord)
			lastErr = err
			excludedChannels[selectedChannelID] = true
			if attempt < maxRouteBillingAttempts {
				r.sleepBeforeRetry(attempt)
				continue
			}
			return nil, err
		}

		if resp != nil && isRetryableProviderResponse(resp) {
			r.recordSelectedFailure(selectedChannelID)
			r.markRetryableProviderFailure(ch, resp)
			if r.quotaManager != nil && billingSessionID != "" {
				_ = r.quotaManager.Refund(ctx, organizationID, billingSessionID)
			}
			if r.apiTokenQuotaManager != nil && apiTokenID != "" {
				_ = r.apiTokenQuotaManager.RefundRelayAPITokenQuota(ctx, apiTokenID, tokenPreauthorizedAmount)
			}
			if resp.Error != nil {
				lastErr = resp.Error
			}
			lastResp = resp
			excludedChannels[selectedChannelID] = true
			if attempt < maxRouteBillingAttempts {
				r.sleepBeforeRetry(attempt)
				continue
			}
			return resp, nil
		}

		if resp != nil && isNonRetryableInvalidProviderResponse(resp) {
			r.markInvalidProviderFailure(ch, resp)
		}

		if resp != nil && resp.StatusCode >= 500 {
			r.recordSelectedFailure(selectedChannelID)
		} else {
			r.recordSelectedSuccess(selectedChannelID)
		}
		if resp != nil {
			resp.BillingSessionID = billingSessionID
			if tokenPreauthorizedQuote != nil {
				resp.PreauthorizedAmount = tokenPreauthorizedQuote.TotalCost
			}
			resp.TokenPreauthorizedAmount = tokenPreauthorizedAmount
		}

		if conversationID != "" && r.affinityStore != nil {
			_ = r.affinityStore.SaveConversationAffinity(ctx, conversationID, selectedChannelID)
		}

		actualUsage := usage
		if resp != nil && resp.Usage != nil {
			actualUsage = resp.Usage
		}
		actualQuote, err := r.estimatedUsagePriceQuote(model, apiType, actualUsage, ch, userGroup)
		if err != nil {
			if r.quotaManager != nil && billingSessionID != "" {
				_ = r.quotaManager.Refund(ctx, organizationID, billingSessionID)
			}
			if r.apiTokenQuotaManager != nil && apiTokenID != "" {
				_ = r.apiTokenQuotaManager.RefundRelayAPITokenQuota(ctx, apiTokenID, tokenPreauthorizedAmount)
			}
			routeErr := &RouterError{Code: http.StatusInternalServerError, Message: "relay pricing is not configured: " + err.Error(), ErrorCode: "relay_pricing_not_configured"}
			_ = r.replaceUsage(ctx, r.errorUsageRecord(userID, organizationID, apiTokenID, requestID, apiType, model, ch, routeErr.Code, routeErr.ErrorCode, startedAt))
			r.recordRelayRuntimeMetricsForChannel(ch, apiType, model, routeErr.Code, "miss", startedAt)
			return nil, routeErr
		}

		statusCode := http.StatusOK
		if resp != nil && resp.StatusCode >= http.StatusContinue {
			statusCode = resp.StatusCode
		}
		actualCost := actualQuote.TotalCost
		record := r.successUsageRecord(userID, organizationID, apiTokenID, requestID, apiType, model, ch, actualUsage, actualQuote, startedAt)
		record.StatusCode = statusCode
		if resp != nil && resp.StatusCode >= http.StatusBadRequest {
			record.Status = RelayUsageStatusError
			if resp.Error != nil {
				record.ErrorCode = resp.Error.Code
			}
		}
		recordRequestLogBillingMetadata(ctx, model, billingSessionID, tokenPreauthorizedQuote, tokenPreauthorizedAmount, record)
		var usageErr error
		if streamingResponse {
			usageErr = r.replaceUsage(ctx, record)
		} else {
			usageErr = r.recordUsage(ctx, record)
		}
		if usageErr != nil {
			if !streamingResponse {
				if r.quotaManager != nil && billingSessionID != "" {
					_ = r.quotaManager.Refund(ctx, organizationID, billingSessionID)
				}
				if r.apiTokenQuotaManager != nil && apiTokenID != "" {
					_ = r.apiTokenQuotaManager.RefundRelayAPITokenQuota(ctx, apiTokenID, tokenPreauthorizedAmount)
				}
				routeErr := &RouterError{Code: http.StatusInternalServerError, Message: "usage recording failed: " + usageErr.Error(), ErrorCode: "usage_recording_failed"}
				r.recordRelayRuntimeMetricsForChannel(ch, apiType, model, routeErr.Code, "miss", startedAt)
				return nil, routeErr
			}
			// Streaming handlers may already have sent response bytes by this point.
			// A pending record was written before calling the provider, so settlement can continue
			// while the durable ledger is reconciled by the pending/final lifecycle.
		}

		if r.quotaManager != nil && billingSessionID != "" {
			if err := r.quotaManager.Settle(ctx, organizationID, billingSessionID, actualCost); err != nil {
				if r.apiTokenQuotaManager != nil && apiTokenID != "" {
					_ = r.apiTokenQuotaManager.RefundRelayAPITokenQuota(ctx, apiTokenID, tokenPreauthorizedAmount)
				}
				routeErr := &RouterError{Code: http.StatusInternalServerError, Message: "billing settlement failed: " + err.Error(), ErrorCode: "billing_settlement_failed"}
				_ = r.replaceUsage(ctx, r.errorUsageRecord(userID, organizationID, apiTokenID, requestID, apiType, model, ch, routeErr.Code, routeErr.ErrorCode, startedAt))
				r.recordRelayRuntimeMetricsForChannel(ch, apiType, model, routeErr.Code, "miss", startedAt)
				return nil, routeErr
			}
		}

		if r.apiTokenQuotaManager != nil && apiTokenID != "" {
			if err := r.apiTokenQuotaManager.SettleRelayAPITokenQuota(ctx, apiTokenID, tokenPreauthorizedAmount, actualCost); err != nil {
				if r.quotaManager != nil && billingSessionID != "" {
					_ = r.quotaManager.Refund(ctx, organizationID, billingSessionID)
				}
				routeErr := &RouterError{Code: http.StatusInternalServerError, Message: "API token quota settlement failed: " + err.Error(), ErrorCode: "api_token_quota_settlement_failed"}
				_ = r.replaceUsage(ctx, r.errorUsageRecord(userID, organizationID, apiTokenID, requestID, apiType, model, ch, routeErr.Code, routeErr.ErrorCode, startedAt))
				r.recordRelayRuntimeMetricsForChannel(ch, apiType, model, routeErr.Code, "miss", startedAt)
				return nil, routeErr
			}
		}

		if r.billingHook != nil && resp != nil && resp.Usage != nil {
			session := &BillingSession{
				ChannelID:      billingChannelID,
				APIType:        apiType,
				Model:          model,
				UserGroup:      userGroup,
				IdempotencyKey: attemptIdempotencyKey,
			}
			_, _ = r.billingHook.PostBill(session, resp.Usage)
		}

		if resp != nil && resp.StatusCode < http.StatusBadRequest {
			if cacheReq, ok := types.SemanticCacheRequestFromContext(ctx); ok && r.semanticCache != nil && len(resp.Content) > 0 {
				_, _ = r.semanticCache.Store(ctx, cacheReq, json.RawMessage(resp.Content))
			}
		}

		r.recordRelayRuntimeMetricsForChannel(ch, apiType, model, statusCode, "miss", startedAt)

		return resp, nil
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return lastResp, nil
}

func recordRequestLogBillingMetadata(ctx context.Context, requestedModel string, billingSessionID string, preauthQuote *PricingQuote, tokenPreauthorizedAmount float64, record RelayUsageLogRecord) {
	scope, ok := types.RequestLogScopeFromContext(ctx)
	if !ok {
		return
	}
	preauthorizedAmount := tokenPreauthorizedAmount
	if preauthQuote != nil {
		preauthorizedAmount = preauthQuote.TotalCost
	}
	requestLogTokenPreauthorizedAmount := 0.0
	if record.APITokenID != "" {
		requestLogTokenPreauthorizedAmount = tokenPreauthorizedAmount
	}
	scope.Record(types.RequestLogMetadata{
		RequestID:                record.RequestID,
		OrganizationID:           record.OrganizationID,
		UserID:                   record.UserID,
		Model:                    record.Model,
		RequestedModel:           requestedModel,
		ResolvedModel:            record.Model,
		ChannelID:                record.ChannelID,
		Provider:                 record.Provider,
		BillingSessionID:         billingSessionID,
		PreauthorizedAmount:      preauthorizedAmount,
		TokenPreauthorizedAmount: requestLogTokenPreauthorizedAmount,
		Cost:                     record.Cost,
		ChannelCost:              record.ChannelCost,
		RequestTokens:            record.PromptTokens,
		ResponseTokens:           record.CompletionTokens,
		TotalTokens:              record.TotalTokens,
		Status:                   string(record.Status),
		ErrorCode:                record.ErrorCode,
		PriceCurrency:            record.PriceCurrency,
		PriceSource:              record.PriceSource,
		PriceSnapshot:            record.PriceSnapshot,
	})
}

func (r *Router) sleepBeforeRetry(attempt int) {
	backoff := routeRetryBackoff(attempt)
	if r.retrySleep != nil {
		r.retrySleep(backoff)
		return
	}
	time.Sleep(backoff)
}

func routeRetryBackoff(attempt int) time.Duration {
	switch attempt {
	case 1:
		return 0
	case 2:
		return time.Second
	default:
		return 3 * time.Second
	}
}

func billingPreAuthorizationRouterError(err error) *RouterError {
	if errors.Is(err, ErrRelayPriceNotConfigured) {
		return &RouterError{Code: http.StatusInternalServerError, Message: "relay pricing is not configured: " + err.Error(), ErrorCode: "relay_pricing_not_configured"}
	}
	return &RouterError{Code: http.StatusInternalServerError, Message: "billing pre-authorization failed: " + err.Error(), ErrorCode: "billing_pre_authorization_failed"}
}

func (r *Router) localRateLimitWeightAdjuster(ctx context.Context, model string, usage *types.Usage) func(*types.RouteChannel) int {
	return func(ch *types.RouteChannel) int {
		weight := ch.Weight
		if weight <= 0 {
			weight = 1
		}
		if r == nil || r.rateLimiter == nil || ch == nil {
			return weight
		}
		checks := rateLimitChecksForResolution(r.resolveRateLimit(ctx, ch, model, usage))
		for _, check := range checks {
			if rateLimitCheckEmpty(check) || check.Key.ChannelID != routeChannelID(ch) {
				continue
			}
			decision := r.rateLimiter.Check(ctx, check.Key, check.Limits, check.Usage)
			if rateLimitDecisionNearSoftThreshold(decision, check) {
				return softThresholdWeight(weight)
			}
		}
		return weight
	}
}

func rateLimitDecisionNearSoftThreshold(decision ratelimit.Decision, check RateLimitCheck) bool {
	if !decision.Allowed || decision.Limit <= 0 {
		return false
	}
	switch decision.Dimension {
	case ratelimit.DimensionRPM, ratelimit.DimensionTPM:
	default:
		return false
	}
	projected := decision.Current
	switch decision.Dimension {
	case ratelimit.DimensionRPM:
		projected++
	case ratelimit.DimensionTPM:
		tokens := check.Usage.Tokens
		if tokens < 0 {
			tokens = 0
		}
		projected += tokens
	}
	return projected*10 >= decision.Limit*9
}

func softThresholdWeight(weight int) int {
	if weight <= 1 {
		return 1
	}
	reduced := weight / 10
	if reduced < 1 {
		return 1
	}
	return reduced
}

func attemptScopedIdempotencyKey(idempotencyKey string, attempt int) string {
	if attempt <= 1 || idempotencyKey == "" {
		return idempotencyKey
	}
	return fmt.Sprintf("%s:attempt:%d", idempotencyKey, attempt)
}

func isRetryableProviderResponse(resp *types.ProviderResponse) bool {
	if resp == nil {
		return false
	}
	if resp.Error != nil && resp.Error.Retryable {
		return true
	}
	return IsRetryable(resp.StatusCode)
}

func isNonRetryableInvalidProviderResponse(resp *types.ProviderResponse) bool {
	if resp == nil {
		return false
	}
	return resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden
}

func (r *Router) markRetryableProviderFailure(ch *types.RouteChannel, resp *types.ProviderResponse) {
	if resp == nil || ch == nil {
		return
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		stats, _ := r.pool.GetStats(routeChannelID(ch))
		if stats != nil {
			stats.RateLimitedUntil = time.Now().UTC().Add(retryAfterDuration(resp))
		}
	}
}

func retryAfterDuration(resp *types.ProviderResponse) time.Duration {
	if resp != nil && resp.Headers != nil {
		if raw := strings.TrimSpace(resp.Headers.Get("Retry-After")); raw != "" {
			var seconds int
			if _, err := fmt.Sscanf(raw, "%d", &seconds); err == nil && seconds > 0 {
				return time.Duration(seconds) * time.Second
			}
		}
	}
	return time.Minute
}

func (r *Router) markInvalidProviderFailure(ch *types.RouteChannel, resp *types.ProviderResponse) {
	stats, _ := r.pool.GetStats(routeChannelID(ch))
	if stats != nil {
		switch resp.StatusCode {
		case http.StatusForbidden:
			stats.Forbidden = true
		case http.StatusUnauthorized:
			stats.Invalid = true
		default:
			stats.Invalid = true
		}
	}
}

func (r *Router) recordSelectedSuccess(channelID string) {
	if cb, ok := r.circuitBreakers[channelID]; ok {
		cb.RecordSuccess()
	}
}

func (r *Router) recordSelectedFailure(channelID string) {
	if cb, ok := r.circuitBreakers[channelID]; ok {
		cb.RecordFailure()
	}
}

func (r *Router) recordRelayRuntimeMetricsForChannel(ch *types.RouteChannel, apiType types.APIType, model string, statusCode int, cacheStatus string, startedAt time.Time) {
	r.recordRelayRuntimeMetrics(routeChannelProvider(ch), routeChannelID(ch), apiType, model, statusCode, cacheStatus, startedAt)
}

func (r *Router) recordRelayRuntimeMetrics(provider, channelID string, apiType types.APIType, model string, statusCode int, cacheStatus string, startedAt time.Time) {
	if statusCode < http.StatusContinue {
		statusCode = http.StatusOK
	}
	status := "success"
	if statusCode >= http.StatusBadRequest {
		status = "error"
	}
	durationSeconds := time.Since(startedAt).Seconds()
	metrics.RecordRelayRequest(provider, channelID, apiType.String(), status, cacheStatus)
	metrics.RecordRequest(channelID, model, apiType.String(), status)
	metrics.RecordDuration(channelID, model, apiType.String(), durationSeconds)
	r.updateRuntimeChannelStats(channelID, status == "success", durationSeconds)
}

func (r *Router) updateRuntimeChannelStats(channelID string, success bool, durationSeconds float64) {
	if r == nil || r.pool == nil || channelID == "" || channelID == "none" || channelID == "semantic_cache" {
		return
	}
	stats, ok := r.pool.GetStats(channelID)
	if !ok || stats == nil {
		return
	}
	stats.TotalRequests++
	if success {
		stats.SuccessCount++
	} else {
		stats.FailureCount++
	}
	if durationSeconds >= 0 {
		latencyUs := int64(durationSeconds * 1_000_000)
		stats.LatencySumUs += latencyUs
		stats.LatencyCount++
		recordAdaptiveRuntimeSample(stats, time.Now().UTC(), success, latencyUs)
	}
	metrics.SetRelayChannelHealthScore(channelID, runtimeHealthScore(stats))
}

const adaptiveRuntimeWindow = 5 * time.Minute

func recordAdaptiveRuntimeSample(stats *types.ChannelStats, now time.Time, success bool, latencyUs int64) {
	if stats == nil {
		return
	}
	cutoff := now.Add(-adaptiveRuntimeWindow)
	samples := stats.RuntimeSamples[:0]
	for _, sample := range stats.RuntimeSamples {
		if !sample.At.IsZero() && sample.At.After(cutoff) {
			samples = append(samples, sample)
		}
	}
	stats.RuntimeSamples = append(samples, types.ChannelRuntimeSample{
		At:        now,
		Success:   success,
		LatencyUs: latencyUs,
	})
}

func runtimeHealthScore(stats *types.ChannelStats) float64 {
	if stats == nil {
		return 1
	}
	total := stats.SuccessCount + stats.FailureCount
	if total <= 0 {
		return 1
	}
	errorRate := float64(stats.FailureCount) / float64(total)
	score := 1 - errorRate
	if stats.Invalid {
		score = 0
	}
	if stats.Forbidden {
		score = 0
	}
	if !stats.RateLimitedUntil.IsZero() && time.Now().Before(stats.RateLimitedUntil) && score > 0.2 {
		score = 0.2
	}
	if stats.LatencyCount > 0 {
		avgLatencySeconds := float64(stats.LatencySumUs) / float64(stats.LatencyCount) / 1_000_000
		if avgLatencySeconds > 0.5 {
			score *= 0.5 / avgLatencySeconds
		}
	}
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func (r *Router) estimatedUsageCost(model string, apiType types.APIType, usage *types.Usage, ch *types.RouteChannel, userGroup string) (float64, error) {
	quote, err := r.estimatedUsagePriceQuote(model, apiType, usage, ch, userGroup)
	if err != nil {
		return 0, err
	}
	return quote.TotalCost, nil
}

func (r *Router) estimatedUsagePriceQuote(model string, apiType types.APIType, usage *types.Usage, ch *types.RouteChannel, userGroup string) (*PricingQuote, error) {
	var pricing *PricingStore
	if r.billingHook != nil && r.billingHook.pricing != nil {
		pricing = r.billingHook.pricing
		quote, err := pricing.QuoteUsageForGroupStrict(model, apiType, usage, userGroup)
		if err != nil {
			return nil, err
		}
		return applyRouteChannelPriceQuote(quote, ch), nil
	}
	return nil, fmt.Errorf("%w: pricing store is not configured", ErrRelayPriceNotConfigured)
}

func applyRouteChannelPriceQuote(quote *PricingQuote, ch *types.RouteChannel) *PricingQuote {
	if quote == nil {
		return nil
	}
	multiplier := routeChannelCostMultiplier(ch)
	quote.ChannelMultiplier = multiplier
	quote.TotalCost *= multiplier
	if quote.TotalCost <= 0 {
		quote.TotalCost = 0.000001
	}
	return quote
}

func applyRouteChannelCostMultiplier(cost float64, ch *types.RouteChannel) float64 {
	cost *= routeChannelCostMultiplier(ch)
	if cost <= 0 {
		return 0.000001
	}
	return cost
}

func routeChannelCostMultiplier(ch *types.RouteChannel) float64 {
	if ch == nil {
		return 1
	}
	multiplier := ch.CostMultiplier
	if multiplier <= 0 && ch.Channel != nil {
		multiplier = ch.Channel.CostMultiplier
	}
	if multiplier <= 0 {
		return 1
	}
	return multiplier
}

func (r *Router) channelCost(cost float64, ch *types.RouteChannel) float64 {
	if ch == nil {
		return 0
	}
	multiplier := routeChannelCostMultiplier(ch)
	return cost / multiplier
}

func (r *Router) successUsageRecord(userID, organizationID, apiTokenID, requestID string, apiType types.APIType, model string, ch *types.RouteChannel, usage *types.Usage, quote *PricingQuote, startedAt time.Time) RelayUsageLogRecord {
	var cost float64
	if quote != nil {
		cost = quote.TotalCost
	}
	record := RelayUsageLogRecord{
		UserID:         userID,
		OrganizationID: organizationID,
		APITokenID:     apiTokenID,
		RequestID:      requestID,
		APIType:        apiType.String(),
		Model:          model,
		Status:         RelayUsageStatusSuccess,
		StatusCode:     http.StatusOK,
		LatencyMS:      time.Since(startedAt).Milliseconds(),
		Cost:           cost,
		ChannelCost:    r.channelCost(cost, ch),
		PriceSnapshot:  quote,
		CreatedAt:      time.Now().UTC(),
	}
	if quote != nil {
		record.PriceCurrency = quote.Currency
		record.PriceSource = quote.Source
		record.PriceEffectiveFrom = cloneTime(quote.EffectiveFrom)
	}
	if ch != nil {
		record.ChannelID = routeChannelID(ch)
		if ch.Channel != nil {
			record.Provider = ch.Channel.Provider
		}
	}
	if usage != nil {
		record.PromptTokens = usage.PromptTokens
		record.CompletionTokens = usage.CompletionTokens
		record.TotalTokens = usage.TotalTokens
	}
	return record
}

func (r *Router) errorUsageRecord(userID, organizationID, apiTokenID, requestID string, apiType types.APIType, model string, ch *types.RouteChannel, statusCode int, errorCode string, startedAt time.Time) RelayUsageLogRecord {
	record := r.successUsageRecord(userID, organizationID, apiTokenID, requestID, apiType, model, ch, nil, nil, startedAt)
	record.Status = RelayUsageStatusError
	record.StatusCode = statusCode
	record.ErrorCode = errorCode
	record.Cost = 0
	record.ChannelCost = 0
	return record
}

func (r *Router) recordUsage(ctx context.Context, record RelayUsageLogRecord) error {
	if r == nil || r.usageLogger == nil {
		return nil
	}
	if record.FeatureType == "" {
		if featureType, ok := types.TrustedFeatureTypeFromContext(ctx); ok {
			record.FeatureType = featureType
		}
	}
	return r.usageLogger.RecordRelayUsage(ctx, record)
}

func (r *Router) replaceUsage(ctx context.Context, record RelayUsageLogRecord) error {
	if r == nil || r.usageLogger == nil {
		return nil
	}
	if record.FeatureType == "" {
		if featureType, ok := types.TrustedFeatureTypeFromContext(ctx); ok {
			record.FeatureType = featureType
		}
	}
	if replacer, ok := r.usageLogger.(RelayUsageReplacer); ok && record.RequestID != "" {
		return replacer.ReplaceRelayUsage(ctx, record)
	}
	return r.usageLogger.RecordRelayUsage(ctx, record)
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

func routeChannelProvider(ch *types.RouteChannel) string {
	if ch == nil || ch.Channel == nil {
		return ""
	}
	return ch.Channel.Provider
}

func relayAPITokenQuotaRouterError(err error) (int, string) {
	switch {
	case errors.Is(err, types.ErrRelayAPITokenQuotaExceeded):
		return http.StatusPaymentRequired, "relay_api_token_quota_exceeded"
	default:
		return http.StatusPaymentRequired, "relay_api_token_quota_exceeded"
	}
}

func (r *Router) resolveRateLimit(ctx context.Context, ch *types.RouteChannel, model string, usage *types.Usage) RateLimitResolution {
	if r == nil || ch == nil {
		return RateLimitResolution{}
	}
	if r.rateLimitResolver != nil {
		resolution := r.rateLimitResolver(ctx, ch, model, usage)
		if resolution.Key.ChannelID == "" {
			resolution.Key.ChannelID = routeChannelID(ch)
		}
		if resolution.Key.Model == "" {
			resolution.Key.Model = model
		}
		if resolution.Usage.Tokens == 0 && usage != nil {
			resolution.Usage.Tokens = usage.TotalTokens
		}
		return resolution
	}
	limits := ratelimit.Limits{}
	if ch.Channel != nil {
		limits.RPM = ch.Channel.RPMLimit
		limits.TPM = ch.Channel.TPMLimit
	}
	tokens := 0
	if usage != nil {
		tokens = usage.TotalTokens
	}
	return RateLimitResolution{
		Key: ratelimit.Key{
			ChannelID: routeChannelID(ch),
			Model:     model,
		},
		Limits: limits,
		Usage:  ratelimit.Usage{Tokens: tokens},
	}
}

func (r RateLimitResolution) empty() bool {
	if !rateLimitCheckEmpty(RateLimitCheck{Key: r.Key, Limits: r.Limits, Usage: r.Usage}) {
		return false
	}
	for _, check := range r.Additional {
		if !rateLimitCheckEmpty(check) {
			return false
		}
	}
	return true
}

func rateLimitChecksForResolution(resolution RateLimitResolution) []RateLimitCheck {
	checks := []RateLimitCheck{}
	primary := RateLimitCheck{Key: resolution.Key, Limits: resolution.Limits, Usage: resolution.Usage}
	if !rateLimitCheckEmpty(primary) {
		checks = append(checks, primary)
	}
	for _, check := range resolution.Additional {
		if !rateLimitCheckEmpty(check) {
			checks = append(checks, check)
		}
	}
	return checks
}

func allowOnlyLimits(limits ratelimit.Limits) ratelimit.Limits {
	limits.MaxConcurrent = 0
	return limits
}

func rateLimitCheckEmpty(check RateLimitCheck) bool {
	return check.Limits.RPM <= 0 &&
		check.Limits.TPM <= 0 &&
		check.Limits.MaxConcurrent <= 0 &&
		check.Limits.MaxTokensPerRequest <= 0
}

func rateLimitRouterError(err error) *RouterError {
	routeErr := &RouterError{
		Code:      http.StatusTooManyRequests,
		Message:   "relay rate limit exceeded",
		ErrorCode: "relay_rate_limited",
	}
	var limitErr *ratelimit.LimitError
	if errors.As(err, &limitErr) {
		if limitErr.RetryAfter > 0 {
			routeErr.RetryAfter = int(limitErr.RetryAfter.Round(time.Second) / time.Second)
			if routeErr.RetryAfter <= 0 {
				routeErr.RetryAfter = 1
			}
		}
		if limitErr.Dimension != "" {
			routeErr.Message = fmt.Sprintf("relay rate limit exceeded: %s", limitErr.Dimension)
		}
	}
	return routeErr
}
