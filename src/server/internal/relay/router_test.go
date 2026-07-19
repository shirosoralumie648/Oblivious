package relay

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"oblivious/server/internal/metrics"
	relaycache "oblivious/server/internal/relay/cache"
	"oblivious/server/internal/relay/types"
	"oblivious/server/internal/releasecontract"
)

type codedTestError struct {
	message string
	code    string
}

type relaySemanticCacheStoreSpy struct {
	inner         *relaycache.InMemorySemanticCacheStore
	getCalls      int
	hitIncrements int
}

func newRelaySemanticCacheStoreSpy() *relaySemanticCacheStoreSpy {
	return &relaySemanticCacheStoreSpy{inner: relaycache.NewInMemorySemanticCacheStore()}
}

func (s *relaySemanticCacheStoreSpy) Get(ctx context.Context, key relaycache.SemanticCacheKey) (*relaycache.SemanticCacheEntry, error) {
	s.getCalls++
	return s.inner.Get(ctx, key)
}

func (s *relaySemanticCacheStoreSpy) Put(ctx context.Context, entry relaycache.SemanticCacheEntry) error {
	return s.inner.Put(ctx, entry)
}

func (s *relaySemanticCacheStoreSpy) IncrementHit(ctx context.Context, key relaycache.SemanticCacheKey) error {
	s.hitIncrements++
	return s.inner.IncrementHit(ctx, key)
}

func newRelaySemanticCacheReadinessRouter(t *testing.T, guard *batchGuardSpy) *Router {
	t.Helper()
	contract, profile := loadBatchReadinessAuthority(t)
	authorities, err := releasecontract.NewRuntimeAuthorities(contract, profile, guard)
	if err != nil {
		t.Fatalf("compile runtime authorities: %v", err)
	}
	pricing := NewPricingStore()
	for _, dimension := range []types.UsageDimension{types.DimPromptTokens, types.DimCompletionTokens, types.DimTotalTokens} {
		pricing.SetPrice("gpt-4o-mini", types.APITypeChat, dimension, 0.01)
	}
	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{
		ID:       "relay-cache-primary",
		Provider: "openai",
		BaseURL:  "https://primary.invalid",
		Models:   []string{"gpt-4o-mini"},
		Enabled:  true,
	}, 1)
	router, err := NewRouterWithBillingOptions(
		pool,
		NewLoadBalancer(pool, "priority"),
		map[string]*CircuitBreaker{"relay-cache-primary": NewCircuitBreaker("relay-cache-primary", 5, time.Second, time.Minute)},
		nil,
		NewHealthChecker(HealthCheckDisabled, time.Second),
		NewBillingHook(pricing, nil),
		"",
		RouterRuntimeOptions{Guard: guard, Authorities: authorities, Effects: &batchEffectRegistrar{}},
	)
	if err != nil {
		t.Fatalf("construct guarded router: %v", err)
	}
	router.retrySleep = func(time.Duration) {}
	return router
}

func TestRelaySemanticCacheReadinessContract(t *testing.T) {
	t.Run("stale model denies a seeded hit before cache or success effects", func(t *testing.T) {
		guard := &batchGuardSpy{
			denyAtCall: 1,
			denial: &releasecontract.ReadinessError{
				Code:  releasecontract.CodeReadinessStale,
				Field: "generation",
			},
		}
		router := newRelaySemanticCacheReadinessRouter(t, guard)
		quotaManager := &stubQuotaManager{}
		usageLogger := &recordingUsageLogger{}
		router.SetQuotaManager(quotaManager)
		router.SetUsageLogger(usageLogger)

		store := newRelaySemanticCacheStoreSpy()
		router.semanticCache = relaycache.NewSemanticCache(store, relaycache.SemanticCacheOptions{
			Now: func() time.Time { return time.Date(2026, 7, 19, 5, 0, 0, 0, time.UTC) },
		})
		cacheReq := relaycache.SemanticCacheRequest{
			OrganizationID: "org_stale_cache",
			Model:          "gpt-4o-mini",
			Query:          "stale cache payload must not escape",
		}
		if _, err := router.semanticCache.Store(context.Background(), cacheReq, json.RawMessage(`{"cached":"secret-payload"}`)); err != nil {
			t.Fatalf("seed semantic cache: %v", err)
		}

		apiType := types.APITypeChat.String()
		beforeSuccess := testutil.ToFloat64(metrics.RelayRequestTotal.WithLabelValues("semantic_cache", "semantic_cache", apiType, "success", "hit"))
		beforeHit := testutil.ToFloat64(metrics.RelaySemanticCacheEventsTotal.WithLabelValues("hit", apiType, cacheReq.Model))
		providerCalls := 0
		ctx := types.WithSemanticCacheRequest(context.Background(), cacheReq)
		resp, err := router.RouteWithBilling(ctx, types.APITypeChat, cacheReq.Model, "", "stale-cache-hit", &types.Usage{TotalTokens: 2}, func(*types.RouteChannel) (*types.ProviderResponse, error) {
			providerCalls++
			return types.NewOKResponse(nil, nil), nil
		})
		if resp != nil || !releasecontract.IsReadinessCode(err, releasecontract.CodeReadinessStale) {
			t.Fatalf("expected stale readiness denial without cached response, resp=%+v err=%v", resp, err)
		}
		if store.getCalls != 0 || store.hitIncrements != 0 {
			t.Fatalf("readiness denial reached semantic cache: gets=%d hit_increments=%d", store.getCalls, store.hitIncrements)
		}
		if providerCalls != 0 || quotaManager.preconsumeCalls != 0 || quotaManager.settleCalls != 0 || quotaManager.refundCalls != 0 || len(usageLogger.records) != 0 {
			t.Fatalf("readiness denial leaked effects: provider=%d quota=%+v usage=%+v", providerCalls, quotaManager, usageLogger.records)
		}
		if got := testutil.ToFloat64(metrics.RelayRequestTotal.WithLabelValues("semantic_cache", "semantic_cache", apiType, "success", "hit")); got != beforeSuccess {
			t.Fatalf("readiness denial changed cache success metric: before=%v after=%v", beforeSuccess, got)
		}
		if got := testutil.ToFloat64(metrics.RelaySemanticCacheEventsTotal.WithLabelValues("hit", apiType, cacheReq.Model)); got != beforeHit {
			t.Fatalf("readiness denial changed cache hit metric: before=%v after=%v", beforeHit, got)
		}
		if err == nil || strings.Contains(err.Error(), "primary.invalid") || strings.Contains(err.Error(), "secret-payload") {
			t.Fatalf("readiness error leaked provider or cache details: %v", err)
		}
	})

	t.Run("unknown model denies a seeded hit before cache lookup", func(t *testing.T) {
		guard := &batchGuardSpy{}
		router := newRelaySemanticCacheReadinessRouter(t, guard)
		usageLogger := &recordingUsageLogger{}
		router.SetUsageLogger(usageLogger)
		store := newRelaySemanticCacheStoreSpy()
		router.semanticCache = relaycache.NewSemanticCache(store, relaycache.SemanticCacheOptions{
			Now: func() time.Time { return time.Date(2026, 7, 19, 5, 0, 0, 0, time.UTC) },
		})
		cacheReq := relaycache.SemanticCacheRequest{
			OrganizationID: "org_deleted_cache",
			Model:          "deleted-model",
			Query:          "deleted model cache payload",
		}
		if _, err := router.semanticCache.Store(context.Background(), cacheReq, json.RawMessage(`{"cached":true}`)); err != nil {
			t.Fatalf("seed semantic cache: %v", err)
		}

		providerCalls := 0
		ctx := types.WithSemanticCacheRequest(context.Background(), cacheReq)
		resp, err := router.RouteWithBilling(ctx, types.APITypeChat, cacheReq.Model, "", "deleted-cache-hit", &types.Usage{TotalTokens: 2}, func(*types.RouteChannel) (*types.ProviderResponse, error) {
			providerCalls++
			return types.NewOKResponse(nil, nil), nil
		})
		if resp != nil || !releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			t.Fatalf("expected unknown-model denial without cached response, resp=%+v err=%v", resp, err)
		}
		if store.getCalls != 0 || store.hitIncrements != 0 || providerCalls != 0 || len(usageLogger.records) != 0 {
			t.Fatalf("unknown model reached effects: gets=%d hits=%d provider=%d usage=%+v", store.getCalls, store.hitIncrements, providerCalls, usageLogger.records)
		}
	})
}

func TestRelaySemanticCacheProviderRecheckContract(t *testing.T) {
	t.Run("model expiry during billing refunds before provider callback", func(t *testing.T) {
		guard := &batchGuardSpy{
			denyAtCall: 2,
			denial: &releasecontract.ReadinessError{
				Code:  releasecontract.CodeReadinessStale,
				Field: "generation",
			},
		}
		router := newRelaySemanticCacheReadinessRouter(t, guard)
		quotaManager := &stubQuotaManager{}
		usageLogger := &recordingUsageLogger{}
		router.SetQuotaManager(quotaManager)
		router.SetUsageLogger(usageLogger)

		providerCalls := 0
		resp, err := router.RouteWithBilling(context.Background(), types.APITypeChat, "gpt-4o-mini", "", "provider-recheck-model", &types.Usage{TotalTokens: 20}, func(*types.RouteChannel) (*types.ProviderResponse, error) {
			providerCalls++
			return types.NewOKResponse(nil, nil), nil
		})
		if resp != nil || !releasecontract.IsReadinessCode(err, releasecontract.CodeReadinessStale) {
			t.Fatalf("expected stale model denial before provider callback, resp=%+v err=%v", resp, err)
		}
		if len(guard.calls) != 2 || guard.calls[0].capabilityID != guard.calls[1].capabilityID || guard.calls[0].boundary != releasecontract.BoundaryOutbound || guard.calls[1].boundary != releasecontract.BoundaryOutbound {
			t.Fatalf("expected pre-cache and pre-provider model guards, calls=%#v", guard.calls)
		}
		if quotaManager.preconsumeCalls != 1 || quotaManager.refundCalls != 1 || quotaManager.settleCalls != 0 {
			t.Fatalf("expected bounded quota compensation after late denial, quota=%+v", quotaManager)
		}
		if providerCalls != 0 || len(usageLogger.records) != 0 {
			t.Fatalf("late readiness denial reached downstream effects: provider=%d usage=%+v", providerCalls, usageLogger.records)
		}
	})

	t.Run("provider expiry remains a distinct guard after model reauthorization", func(t *testing.T) {
		guard := &batchGuardSpy{
			denyAtCall: 3,
			denial: &releasecontract.ReadinessError{
				Code:  releasecontract.CodeReadinessStale,
				Field: "generation",
			},
		}
		router := newRelaySemanticCacheReadinessRouter(t, guard)
		quotaManager := &stubQuotaManager{}
		router.SetQuotaManager(quotaManager)

		providerCalls := 0
		resp, err := router.RouteWithBilling(context.Background(), types.APITypeChat, "gpt-4o-mini", "", "provider-recheck-transport", &types.Usage{TotalTokens: 20}, func(*types.RouteChannel) (*types.ProviderResponse, error) {
			providerCalls++
			return types.NewOKResponse(nil, nil), nil
		})
		if resp != nil || !releasecontract.IsReadinessCode(err, releasecontract.CodeReadinessStale) {
			t.Fatalf("expected stale provider denial before callback, resp=%+v err=%v", resp, err)
		}
		if len(guard.calls) != 3 || guard.calls[0].capabilityID != guard.calls[1].capabilityID || guard.calls[1].capabilityID != guard.calls[2].capabilityID {
			t.Fatalf("expected model/model/provider guard sequence, calls=%#v", guard.calls)
		}
		if quotaManager.preconsumeCalls != 1 || quotaManager.refundCalls != 1 || providerCalls != 0 {
			t.Fatalf("provider denial compensation/call counts incorrect: quota=%+v provider=%d", quotaManager, providerCalls)
		}
	})
}

func TestRelayReadinessPerAttemptContract(t *testing.T) {
	contract, profile := loadBatchReadinessAuthority(t)
	newAuthorities := func(t *testing.T, guard releasecontract.Guard) releasecontract.RuntimeAuthorities {
		t.Helper()
		authorities, err := releasecontract.NewRuntimeAuthorities(contract, profile, guard)
		if err != nil {
			t.Fatalf("compile runtime authorities: %v", err)
		}
		return authorities
	}
	newBillingRouter := func(t *testing.T, guard *batchGuardSpy, registrar *batchEffectRegistrar) *Router {
		t.Helper()
		pricing := NewPricingStore()
		for _, dimension := range []types.UsageDimension{types.DimPromptTokens, types.DimCompletionTokens, types.DimTotalTokens} {
			pricing.SetPrice("gpt-4o-mini", types.APITypeChat, dimension, 0.01)
		}
		pool := NewChannelPool()
		pool.AddChannel(&types.Channel{ID: "relay-primary", Provider: "openai", BaseURL: "https://primary.invalid", Models: []string{"gpt-4o-mini"}, Enabled: true, Priority: 0}, 1)
		pool.AddChannel(&types.Channel{ID: "relay-fallback", Provider: "openai", BaseURL: "https://fallback.invalid", Models: []string{"gpt-4o-mini"}, Enabled: true, Priority: 10}, 1)
		router, err := NewRouterWithBillingOptions(
			pool,
			NewLoadBalancer(pool, "priority"),
			map[string]*CircuitBreaker{
				"relay-primary":  NewCircuitBreaker("relay-primary", 5, time.Second, time.Minute),
				"relay-fallback": NewCircuitBreaker("relay-fallback", 5, time.Second, time.Minute),
			},
			nil,
			NewHealthChecker(HealthCheckDisabled, time.Second),
			NewBillingHook(pricing, nil),
			"",
			RouterRuntimeOptions{Guard: guard, Authorities: newAuthorities(t, guard), Effects: registrar},
		)
		if err != nil {
			t.Fatalf("construct guarded router: %v", err)
		}
		router.retrySleep = func(time.Duration) {}
		return router
	}

	t.Run("constructor fails closed without startup authority", func(t *testing.T) {
		_, err := NewRouterWithOptions(nil, nil, nil, nil, nil, RouterRuntimeOptions{})
		if !releasecontract.IsReadinessCode(err, releasecontract.CodeReadinessUnavailable) {
			t.Fatalf("expected readiness unavailable, got %v", err)
		}
	})

	for _, code := range []releasecontract.ReadinessCode{
		releasecontract.CodeCapabilityDisabled,
		releasecontract.CodeCapabilityBlocked,
		releasecontract.CodeReadinessStale,
		releasecontract.CodeReadinessUnavailable,
		releasecontract.CodeBuildIdentityMismatch,
	} {
		code := code
		t.Run("initial denial "+string(code)+" has zero downstream effects", func(t *testing.T) {
			guard := &batchGuardSpy{denyAtCall: 1, denial: &releasecontract.ReadinessError{Code: code, Field: "generation"}}
			router := newBillingRouter(t, guard, &batchEffectRegistrar{})
			quotaManager := &stubQuotaManager{}
			usageLogger := &recordingUsageLogger{}
			router.SetQuotaManager(quotaManager)
			router.SetUsageLogger(usageLogger)
			providerCalls := 0
			_, err := router.RouteWithBilling(context.Background(), types.APITypeChat, "gpt-4o-mini", "", "initial-denial", &types.Usage{TotalTokens: 2}, func(*types.RouteChannel) (*types.ProviderResponse, error) {
				providerCalls++
				return types.NewOKResponse(nil, nil), nil
			})
			if !releasecontract.IsReadinessCode(err, code) {
				t.Fatalf("expected %s denial, got %v", code, err)
			}
			if providerCalls != 0 || quotaManager.preconsumeCalls != 0 || quotaManager.settleCalls != 0 || quotaManager.refundCalls != 0 || len(usageLogger.records) != 0 {
				t.Fatalf("denial leaked effects: provider=%d quota=%+v usage=%+v", providerCalls, quotaManager, usageLogger.records)
			}
		})
	}

	t.Run("unknown current model is rejected before effects", func(t *testing.T) {
		guard := &batchGuardSpy{}
		router := newBillingRouter(t, guard, &batchEffectRegistrar{})
		quotaManager := &stubQuotaManager{}
		router.SetQuotaManager(quotaManager)
		providerCalls := 0
		_, err := router.RouteWithBilling(context.Background(), types.APITypeChat, "caller-capability", "", "unknown-model", &types.Usage{TotalTokens: 2}, func(*types.RouteChannel) (*types.ProviderResponse, error) {
			providerCalls++
			return nil, nil
		})
		if !releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) || providerCalls != 0 || quotaManager.preconsumeCalls != 0 {
			t.Fatalf("unknown model did not fail before effects: err=%v provider=%d quota=%d", err, providerCalls, quotaManager.preconsumeCalls)
		}
	})

	t.Run("billing retry re-authorizes the next model attempt", func(t *testing.T) {
		guard := &batchGuardSpy{denyAtCall: 4, denial: &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessStale, Field: "generation"}}
		registrar := &batchEffectRegistrar{}
		router := newBillingRouter(t, guard, registrar)
		quotaManager := &stubQuotaManager{}
		router.SetQuotaManager(quotaManager)
		providerCalls := 0
		_, err := router.RouteWithBilling(context.Background(), types.APITypeChat, "gpt-4o-mini", "", "readiness-attempt", &types.Usage{TotalTokens: 2}, func(*types.RouteChannel) (*types.ProviderResponse, error) {
			providerCalls++
			return &types.ProviderResponse{StatusCode: http.StatusBadGateway, Error: &types.ProviderError{Code: "retryable", Retryable: true}}, nil
		})
		if !releasecontract.IsReadinessCode(err, releasecontract.CodeReadinessStale) || providerCalls != 1 || quotaManager.preconsumeCalls != 2 || quotaManager.refundCalls != 2 {
			t.Fatalf("expected next-attempt denial, got err=%v calls=%#v provider=%d quota=%d", err, guard.calls, providerCalls, quotaManager.preconsumeCalls)
		}
		if len(registrar.descriptors) != 1 || registrar.descriptors[0].CapabilityID != "relay.provider_inference" {
			t.Fatalf("unexpected readiness descriptors: %#v", registrar.descriptors)
		}
	})

	t.Run("non billing fallback re-authorizes the next selected route", func(t *testing.T) {
		guard := &batchGuardSpy{denyAtCall: 3, denial: &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessStale, Field: "generation"}}
		pool := NewChannelPool()
		pool.AddChannel(&types.Channel{ID: "relay-fallback", Provider: "openai", Models: []string{"gpt-4o-mini"}, Enabled: true}, 1)
		router, err := NewRouterWithOptions(pool, NewLoadBalancer(pool, "weighted"), nil, nil, NewHealthChecker(HealthCheckDisabled, time.Second), RouterRuntimeOptions{
			Guard: guard, Authorities: newAuthorities(t, guard), Effects: &batchEffectRegistrar{},
		})
		if err != nil {
			t.Fatalf("construct guarded router: %v", err)
		}
		providerCalls := 0
		_, err = router.RouteWithFallback(context.Background(), types.APITypeChat.String(), 2, func(*types.RouteChannel) (*types.ProviderResponse, error) {
			providerCalls++
			return nil, errors.New("retryable transport error")
		})
		if !releasecontract.IsReadinessCode(err, releasecontract.CodeReadinessStale) || providerCalls != 1 {
			t.Fatalf("expected fallback denial, got err=%v calls=%#v provider=%d", err, guard.calls, providerCalls)
		}
	})
}

func (e codedTestError) Error() string {
	return e.message
}

func (e codedTestError) RelayErrorCode() string {
	return e.code
}

func TestRouter_SelectsHealthyChannel(t *testing.T) {
	// Setup: create pool with two channels, one healthy, one not
	pool := NewChannelPool()
	healthyCh := &types.Channel{ID: "healthy", BaseURL: "http://healthy", Enabled: true}
	unhealthyCh := &types.Channel{ID: "unhealthy", BaseURL: "http://unhealthy", Enabled: false}
	pool.AddChannel(healthyCh, 1)
	pool.AddChannel(unhealthyCh, 1)

	lb := NewLoadBalancer(pool, "weighted")
	cb := NewCircuitBreaker("test", 3, time.Second, 30*time.Second)
	tb := NewTokenBucket(60, 1000)
	hc := NewHealthChecker(HealthCheckDisabled, 5*time.Second)

	router := NewRouter(pool, lb, map[string]*CircuitBreaker{"healthy": cb, "unhealthy": cb}, tb, hc)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()
	healthyCh.BaseURL = ts.URL

	ch := router.SelectChannel(context.Background(), "chat")
	if ch == nil {
		t.Fatal("should return a channel")
	}
	if !ch.Channel.Enabled {
		t.Fatal("should not return disabled channel")
	}
}

func TestRouter_AllChannelsFailed(t *testing.T) {
	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{ID: "fail", BaseURL: "http://fail", Enabled: false}, 1)

	lb := NewLoadBalancer(pool, "weighted")
	hc := NewHealthChecker(HealthCheckDisabled, 5*time.Second)
	router := NewRouter(pool, lb, nil, nil, hc)

	_, err := router.Route(context.Background(), "chat", nil)
	if err == nil {
		t.Fatal("expected error when all channels fail")
	}
	// Should return 503 with Retry-After header
	var re *RouterError
	if !errors.As(err, &re) {
		t.Fatalf("expected RouterError, got %T", err)
	}
	if re.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", re.Code)
	}
}

func TestRouter_RouteWithFallback_RetriesAllChannels(t *testing.T) {
	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{ID: "a", BaseURL: "http://a", Enabled: true}, 1)
	pool.AddChannel(&types.Channel{ID: "b", BaseURL: "http://b", Enabled: true}, 1)

	lb := NewLoadBalancer(pool, "weighted")
	router := NewRouter(pool, lb, nil, nil, NewHealthChecker(HealthCheckDisabled, 5*time.Second))

	callCount := 0
	fn := func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		callCount++
		return nil, errors.New("error")
	}

	_, err := router.RouteWithFallback(context.Background(), "chat", 2, fn)
	if err == nil {
		t.Fatal("expected error")
	}
	// Both attempts use the same channel due to LB; with 2 channels it varies
}

func TestRouterRouteWithBillingFailsClosedWhenPricingMissing(t *testing.T) {
	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{
		ID:       "priced",
		Provider: "openai",
		BaseURL:  "https://upstream.example",
		APIKey:   "sk-priced",
		Models:   []string{"gpt-4o-mini"},
		Enabled:  true,
	}, 1)
	pricing := NewPricingStore()
	pricing.SetPrice("gpt-4o-mini", types.APITypeChat, types.DimPromptTokens, 0.0002)
	router := NewRouterWithBilling(
		pool,
		NewLoadBalancer(pool, "weighted"),
		map[string]*CircuitBreaker{"priced": NewCircuitBreaker("priced", 5, time.Second, time.Minute)},
		nil,
		NewHealthChecker(HealthCheckDisabled, time.Second),
		NewBillingHook(pricing, nil),
		"",
	)

	upstreamCalls := 0
	resp, err := router.RouteWithBilling(context.Background(), types.APITypeChat, "gpt-4o-mini", "", "idem_missing_price", &types.Usage{TotalTokens: 20}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		upstreamCalls++
		return types.NewOKResponse([]byte(`{"ok":true}`), &types.Usage{TotalTokens: 20}), nil
	})

	if resp != nil {
		t.Fatalf("expected no response when pricing is missing, got %+v", resp)
	}
	var routeErr *RouterError
	if !errors.As(err, &routeErr) || routeErr.Code != http.StatusInternalServerError || routeErr.ErrorCode != "relay_pricing_not_configured" {
		t.Fatalf("expected relay_pricing_not_configured router error, got %#v", err)
	}
	if upstreamCalls != 0 {
		t.Fatalf("pricing must fail closed before upstream, got %d upstream calls", upstreamCalls)
	}
}

func TestRouterRouteWithFallbackRetriesRetryableProviderResponse(t *testing.T) {
	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{ID: "a", BaseURL: "http://a", Enabled: true}, 1)

	router := NewRouter(pool, NewLoadBalancer(pool, "weighted"), nil, nil, NewHealthChecker(HealthCheckDisabled, 5*time.Second))

	callCount := 0
	resp, err := router.RouteWithFallback(context.Background(), "chat", 2, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		callCount++
		if callCount == 1 {
			return &types.ProviderResponse{StatusCode: http.StatusServiceUnavailable}, nil
		}
		return types.NewOKResponse([]byte(`{"ok":true}`), nil), nil
	})

	if err != nil {
		t.Fatalf("RouteWithFallback returned error: %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected retry to return successful response, got %+v", resp)
	}
	if callCount != 2 {
		t.Fatalf("expected two attempts, got %d", callCount)
	}
}

func TestRouterRouteWithFallbackRetriesFirstRetryableProviderResponseImmediately(t *testing.T) {
	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{ID: "a", BaseURL: "http://a", Enabled: true}, 1)

	router := NewRouter(pool, NewLoadBalancer(pool, "weighted"), nil, nil, NewHealthChecker(HealthCheckDisabled, 5*time.Second))
	var sleeps []time.Duration
	router.retrySleep = func(d time.Duration) {
		sleeps = append(sleeps, d)
	}

	callCount := 0
	resp, err := router.RouteWithFallback(context.Background(), "chat", 2, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		callCount++
		if callCount == 1 {
			return &types.ProviderResponse{StatusCode: http.StatusServiceUnavailable}, nil
		}
		return types.NewOKResponse([]byte(`{"ok":true}`), nil), nil
	})

	if err != nil {
		t.Fatalf("RouteWithFallback returned error: %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected retry to return successful response, got %+v", resp)
	}
	if len(sleeps) != 1 || sleeps[0] != 0 {
		t.Fatalf("expected first retry to use immediate production backoff, got %v", sleeps)
	}
}

func TestRouterRouteWithBillingRetries5xxAcrossChannelsAndUpdatesConversationAffinity(t *testing.T) {
	router, affinityStore := newRetryAffinityTestRouter()
	ctx := types.WithTrustedConversationID(context.Background(), "conv_5xx")

	var attempts []string
	resp, err := router.RouteWithBilling(ctx, types.APITypeChat, "gpt-4o-mini", "", "idem_retry_5xx", &types.Usage{TotalTokens: 20}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		channelID := routeChannelID(ch)
		attempts = append(attempts, channelID)
		if channelID == "primary" {
			return &types.ProviderResponse{StatusCode: http.StatusInternalServerError}, nil
		}
		return types.NewOKResponse([]byte(`{"ok":true}`), &types.Usage{TotalTokens: 20}), nil
	})

	if err != nil {
		t.Fatalf("RouteWithBilling returned error: %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected successful failover response, got %+v", resp)
	}
	assertAttemptedChannels(t, attempts, []string{"primary", "backup"})
	if got := affinityStore.channelFor("conv_5xx"); got != "backup" {
		t.Fatalf("expected conversation affinity to update to backup after failover, got %q", got)
	}
}

func TestRouterRouteWithBillingRetriesBare502ProviderResponseAcrossChannels(t *testing.T) {
	router, _ := newRetryAffinityTestRouter()

	var attempts []string
	resp, err := router.RouteWithBilling(context.Background(), types.APITypeChat, "gpt-4o-mini", "", "idem_retry_502", &types.Usage{TotalTokens: 20}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		channelID := routeChannelID(ch)
		attempts = append(attempts, channelID)
		if channelID == "primary" {
			return &types.ProviderResponse{StatusCode: http.StatusBadGateway}, nil
		}
		return types.NewOKResponse([]byte(`{"ok":true}`), &types.Usage{TotalTokens: 20}), nil
	})

	if err != nil {
		t.Fatalf("RouteWithBilling returned error: %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected successful failover response, got %+v", resp)
	}
	assertAttemptedChannels(t, attempts, []string{"primary", "backup"})
}

func TestRouterRouteWithBillingUsesModelRoute(t *testing.T) {
	pool := NewChannelPool()
	defaultChannel := &types.Channel{
		ID:       "default",
		Name:     "Default",
		Provider: "openai",
		BaseURL:  "https://default.example",
		APIKey:   "sk-default",
		Models:   []string{"gpt-3.5"},
		Enabled:  true,
	}
	modelChannel := &types.Channel{
		ID:       "gpt4",
		Name:     "GPT4",
		Provider: "openai",
		BaseURL:  "https://gpt4.example",
		APIKey:   "sk-gpt4",
		Models:   []string{"gpt-4o"},
		Enabled:  true,
	}
	pool.AddChannel(defaultChannel, 100)
	pool.UpdateChannel(modelChannel)
	pool.UpdateRoute(&types.ModelRoute{
		Model:    "gpt-4o",
		Strategy: "weighted",
		Channels: []types.RouteChannel{{
			Channel:   modelChannel,
			ChannelID: modelChannel.ID,
			Weight:    1,
			Enabled:   true,
			Healthy:   true,
		}},
	})
	router := NewRouterWithBilling(
		pool,
		NewLoadBalancer(pool, "weighted"),
		nil,
		nil,
		NewHealthChecker(HealthCheckDisabled, time.Second),
		testBillingHook(),
		"",
	)

	var attempts []string
	resp, err := router.RouteWithBilling(context.Background(), types.APITypeChat, "gpt-4o", "", "idem_model_route", &types.Usage{TotalTokens: 20}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		attempts = append(attempts, routeChannelID(ch))
		return types.NewOKResponse([]byte(`{"ok":true}`), &types.Usage{TotalTokens: 20}), nil
	})

	if err != nil {
		t.Fatalf("RouteWithBilling returned error: %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected successful response, got %+v", resp)
	}
	assertAttemptedChannels(t, attempts, []string{"gpt4"})
}

func TestRouterRouteWithBillingDoesNotUseCrossModelAffinity(t *testing.T) {
	pool := NewChannelPool()
	defaultChannel := &types.Channel{
		ID:       "default",
		Name:     "Default",
		Provider: "openai",
		BaseURL:  "https://default.example",
		APIKey:   "sk-default",
		Models:   []string{"gpt-3.5"},
		Enabled:  true,
	}
	modelChannel := &types.Channel{
		ID:       "gpt4",
		Name:     "GPT4",
		Provider: "openai",
		BaseURL:  "https://gpt4.example",
		APIKey:   "sk-gpt4",
		Models:   []string{"gpt-4o"},
		Enabled:  true,
	}
	pool.AddChannel(defaultChannel, 100)
	pool.UpdateChannel(modelChannel)
	pool.UpdateRoute(&types.ModelRoute{
		Model:    "gpt-4o",
		Strategy: "weighted",
		Channels: []types.RouteChannel{{
			Channel:   modelChannel,
			ChannelID: modelChannel.ID,
			Weight:    1,
			Enabled:   true,
			Healthy:   true,
		}},
	})
	router := NewRouterWithBilling(
		pool,
		NewLoadBalancer(pool, "weighted"),
		nil,
		nil,
		NewHealthChecker(HealthCheckDisabled, time.Second),
		testBillingHook(),
		"",
	)
	affinityStore := newMemoryConversationAffinityStore()
	if err := affinityStore.SaveConversationAffinity(context.Background(), "conv_model", "default"); err != nil {
		t.Fatalf("save affinity: %v", err)
	}
	router.affinityStore = affinityStore
	ctx := types.WithTrustedConversationID(context.Background(), "conv_model")

	var attempts []string
	resp, err := router.RouteWithBilling(ctx, types.APITypeChat, "gpt-4o", "", "idem_cross_model_affinity", &types.Usage{TotalTokens: 20}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		attempts = append(attempts, routeChannelID(ch))
		return types.NewOKResponse([]byte(`{"ok":true}`), &types.Usage{TotalTokens: 20}), nil
	})

	if err != nil {
		t.Fatalf("RouteWithBilling returned error: %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected successful response, got %+v", resp)
	}
	assertAttemptedChannels(t, attempts, []string{"gpt4"})
}

func TestRouterRouteWithBillingAllowsThreeCrossChannelRetries(t *testing.T) {
	router, _ := newRetryAffinityTestRouter()
	router.pool.AddChannel(&types.Channel{
		ID:       "third",
		Name:     "Third",
		Provider: "openai",
		BaseURL:  "https://third.example",
		APIKey:   "sk-third",
		Models:   []string{"gpt-4o-mini"},
		Priority: 20,
		Enabled:  true,
	}, 100)
	router.pool.AddChannel(&types.Channel{
		ID:       "fourth",
		Name:     "Fourth",
		Provider: "openai",
		BaseURL:  "https://fourth.example",
		APIKey:   "sk-fourth",
		Models:   []string{"gpt-4o-mini"},
		Priority: 30,
		Enabled:  true,
	}, 100)

	var attempts []string
	resp, err := router.RouteWithBilling(context.Background(), types.APITypeChat, "gpt-4o-mini", "", "idem_retry_3_times", &types.Usage{TotalTokens: 20}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		channelID := routeChannelID(ch)
		attempts = append(attempts, channelID)
		if channelID != "fourth" {
			return &types.ProviderResponse{StatusCode: http.StatusServiceUnavailable}, nil
		}
		return types.NewOKResponse([]byte(`{"ok":true}`), &types.Usage{TotalTokens: 20}), nil
	})

	if err != nil {
		t.Fatalf("RouteWithBilling returned error: %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected successful response after third retry, got %+v", resp)
	}
	assertAttemptedChannels(t, attempts, []string{"primary", "backup", "third", "fourth"})
}

func TestRouterRouteWithBillingRetriesNetworkErrorsAcrossChannels(t *testing.T) {
	router, affinityStore := newRetryAffinityTestRouter()
	ctx := types.WithTrustedConversationID(context.Background(), "conv_network")

	var attempts []string
	resp, err := router.RouteWithBilling(ctx, types.APITypeChat, "gpt-4o-mini", "", "idem_retry_network", &types.Usage{TotalTokens: 20}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		channelID := routeChannelID(ch)
		attempts = append(attempts, channelID)
		if channelID == "primary" {
			return nil, errors.New("connection refused")
		}
		return types.NewOKResponse([]byte(`{"ok":true}`), &types.Usage{TotalTokens: 20}), nil
	})

	if err != nil {
		t.Fatalf("RouteWithBilling returned error: %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected successful failover response, got %+v", resp)
	}
	assertAttemptedChannels(t, attempts, []string{"primary", "backup"})
	if got := affinityStore.channelFor("conv_network"); got != "backup" {
		t.Fatalf("expected conversation affinity to update to backup after network failover, got %q", got)
	}
}

func TestRouterRouteWithBillingRetries429AcrossChannelsAndMarksRateLimitedUntil(t *testing.T) {
	router, affinityStore := newRetryAffinityTestRouter()
	ctx := types.WithTrustedConversationID(context.Background(), "conv_429")

	var attempts []string
	headers := http.Header{}
	headers.Set("Retry-After", "45")
	resp, err := router.RouteWithBilling(ctx, types.APITypeChat, "gpt-4o-mini", "", "idem_retry_429", &types.Usage{TotalTokens: 20}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		channelID := routeChannelID(ch)
		attempts = append(attempts, channelID)
		if channelID == "primary" {
			return &types.ProviderResponse{
				StatusCode: http.StatusTooManyRequests,
				Headers:    headers,
				Error:      &types.ProviderError{Code: "rate_limited", StatusCode: http.StatusTooManyRequests, Retryable: true},
			}, nil
		}
		return types.NewOKResponse([]byte(`{"ok":true}`), &types.Usage{TotalTokens: 20}), nil
	})

	if err != nil {
		t.Fatalf("RouteWithBilling returned error: %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected successful failover response, got %+v", resp)
	}
	assertAttemptedChannels(t, attempts, []string{"primary", "backup"})
	stats, ok := router.pool.GetStats("primary")
	if !ok || stats == nil || time.Until(stats.RateLimitedUntil) < 40*time.Second {
		t.Fatalf("expected primary to be marked rate limited for retry-after window, ok=%v stats=%+v", ok, stats)
	}
	if got := affinityStore.channelFor("conv_429"); got != "backup" {
		t.Fatalf("expected conversation affinity to update to backup after 429 failover, got %q", got)
	}
}

func TestRouterRouteWithBillingDoesNotRetryUnauthorizedProviderResponse(t *testing.T) {
	router, affinityStore := newRetryAffinityTestRouter()
	ctx := types.WithTrustedConversationID(context.Background(), "conv_401")

	var attempts []string
	resp, err := router.RouteWithBilling(ctx, types.APITypeChat, "gpt-4o-mini", "", "idem_no_retry_401", &types.Usage{TotalTokens: 20}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		channelID := routeChannelID(ch)
		attempts = append(attempts, channelID)
		return &types.ProviderResponse{
			StatusCode: http.StatusUnauthorized,
			Error:      &types.ProviderError{Code: "unauthorized", StatusCode: http.StatusUnauthorized, Retryable: false},
		}, nil
	})

	if err != nil {
		t.Fatalf("non-retryable provider response should return response without router error, got %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected original 401 response, got %+v", resp)
	}
	assertAttemptedChannels(t, attempts, []string{"primary"})
	if got := affinityStore.channelFor("conv_401"); got != "primary" {
		t.Fatalf("expected initial affinity to remain primary, got %q", got)
	}
	stats, ok := router.pool.GetStats("primary")
	if !ok || stats == nil || !stats.Invalid {
		t.Fatalf("expected unauthorized channel to be marked invalid, got ok=%v stats=%+v", ok, stats)
	}
}

func TestRouterRouteWithBillingDoesNotRetryForbiddenProviderResponse(t *testing.T) {
	router, affinityStore := newRetryAffinityTestRouter()
	ctx := types.WithTrustedConversationID(context.Background(), "conv_403")

	var attempts []string
	resp, err := router.RouteWithBilling(ctx, types.APITypeChat, "gpt-4o-mini", "", "idem_no_retry_403", &types.Usage{TotalTokens: 20}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		channelID := routeChannelID(ch)
		attempts = append(attempts, channelID)
		return &types.ProviderResponse{
			StatusCode: http.StatusForbidden,
			Error:      &types.ProviderError{Code: "forbidden", StatusCode: http.StatusForbidden, Retryable: false},
		}, nil
	})

	if err != nil {
		t.Fatalf("non-retryable provider response should return response without router error, got %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected original 403 response, got %+v", resp)
	}
	assertAttemptedChannels(t, attempts, []string{"primary"})
	if got := affinityStore.channelFor("conv_403"); got != "primary" {
		t.Fatalf("expected initial affinity to remain primary, got %q", got)
	}
	stats, ok := router.pool.GetStats("primary")
	if !ok || stats == nil || !stats.Forbidden || stats.Invalid {
		t.Fatalf("expected forbidden channel to be marked forbidden but not invalid, got ok=%v stats=%+v", ok, stats)
	}
}

func TestRouterRouteWithBillingUsesConversationAffinityBeforeLoadBalancer(t *testing.T) {
	router, affinityStore := newRetryAffinityTestRouter()
	if err := affinityStore.SaveConversationAffinity(context.Background(), "conv_sticky", "backup"); err != nil {
		t.Fatalf("seed affinity: %v", err)
	}
	ctx := types.WithTrustedConversationID(context.Background(), "conv_sticky")

	var attempts []string
	resp, err := router.RouteWithBilling(ctx, types.APITypeChat, "gpt-4o-mini", "", "idem_affinity", &types.Usage{TotalTokens: 20}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		attempts = append(attempts, routeChannelID(ch))
		return types.NewOKResponse([]byte(`{"ok":true}`), &types.Usage{TotalTokens: 20}), nil
	})

	if err != nil {
		t.Fatalf("RouteWithBilling returned error: %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected successful response, got %+v", resp)
	}
	assertAttemptedChannels(t, attempts, []string{"backup"})
	if got := affinityStore.channelFor("conv_sticky"); got != "backup" {
		t.Fatalf("expected sticky affinity to remain backup, got %q", got)
	}
}

func TestRouterRouteWithBillingUsesTrustedOrganizationForChannelSelectionAndAffinity(t *testing.T) {
	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{
		ID:             "org_a_channel",
		OrganizationID: "org_a",
		Name:           "Org A",
		Provider:       "openai",
		BaseURL:        "https://org-a.example",
		APIKey:         "sk-org-a",
		Models:         []string{"gpt-4o-mini"},
		Priority:       10,
		Enabled:        true,
	}, 1)
	pool.AddChannel(&types.Channel{
		ID:             "org_b_channel",
		OrganizationID: "org_b",
		Name:           "Org B",
		Provider:       "openai",
		BaseURL:        "https://org-b.example",
		APIKey:         "sk-org-b",
		Models:         []string{"gpt-4o-mini"},
		Priority:       0,
		Enabled:        true,
	}, 100)
	router := NewRouterWithBilling(
		pool,
		NewLoadBalancer(pool, "priority"),
		nil,
		nil,
		NewHealthChecker(HealthCheckDisabled, time.Second),
		testBillingHook(),
		"",
	)
	affinityStore := newMemoryConversationAffinityStore()
	if err := affinityStore.SaveConversationAffinity(context.Background(), "conv_tenant", "org_b_channel"); err != nil {
		t.Fatalf("seed affinity: %v", err)
	}
	router.affinityStore = affinityStore

	ctx := types.WithTrustedOrganizationID(context.Background(), "org_a")
	ctx = types.WithTrustedConversationID(ctx, "conv_tenant")
	var attempts []string
	resp, err := router.RouteWithBilling(ctx, types.APITypeChat, "gpt-4o-mini", "", "idem_tenant_channel", &types.Usage{TotalTokens: 20}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		attempts = append(attempts, routeChannelID(ch))
		return types.NewOKResponse([]byte(`{"ok":true}`), &types.Usage{TotalTokens: 20}), nil
	})

	if err != nil {
		t.Fatalf("RouteWithBilling returned error: %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected successful response, got %+v", resp)
	}
	assertAttemptedChannels(t, attempts, []string{"org_a_channel"})
	if got := affinityStore.channelFor("conv_tenant"); got != "org_a_channel" {
		t.Fatalf("expected cross-organization affinity to be replaced with org_a channel, got %q", got)
	}
}

func TestRouterRouteWithBillingAppliesTrustedUserGroupPricingMultiplier(t *testing.T) {
	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{
		ID:             "vip_channel",
		OrganizationID: "org_vip",
		Name:           "VIP Channel",
		Provider:       "openai",
		BaseURL:        "https://vip.example",
		APIKey:         "sk-vip",
		Models:         []string{"gpt-4o"},
		Enabled:        true,
	}, 1)

	pricing := NewPricingStore()
	pricing.SetPrice("gpt-4o", types.APITypeChat, types.DimPromptTokens, 2.0)
	pricing.SetPrice("gpt-4o", types.APITypeChat, types.DimCompletionTokens, 8.0)
	pricing.ApplyMultipliers(nil, map[string]float64{"vip": 0.5})
	router := NewRouterWithBilling(
		pool,
		NewLoadBalancer(pool, "weighted"),
		nil,
		nil,
		NewHealthChecker(HealthCheckDisabled, time.Second),
		NewBillingHook(pricing, nil),
		"",
	)
	quotaManager := &stubQuotaManager{}
	apiTokenQuotaManager := &recordingAPITokenQuotaManager{}
	usageLogger := &recordingUsageLogger{}
	router.SetQuotaManager(quotaManager)
	router.SetAPITokenQuotaManager(apiTokenQuotaManager)
	router.SetUsageLogger(usageLogger)

	ctx := types.WithTrustedUserID(context.Background(), "user_vip")
	ctx = types.WithTrustedOrganizationID(ctx, "org_vip")
	ctx = types.WithTrustedAPITokenID(ctx, "tok_vip")
	ctx = types.WithTrustedUserGroup(ctx, "vip")
	usage := &types.Usage{PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500}

	resp, err := router.RouteWithBilling(ctx, types.APITypeChat, "gpt-4o", "", "idem_vip_group", usage, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		if got := routeChannelID(ch); got != "vip_channel" {
			t.Fatalf("expected vip channel, got %q", got)
		}
		return types.NewOKResponse([]byte(`{"ok":true}`), usage), nil
	})

	if err != nil {
		t.Fatalf("RouteWithBilling returned error: %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected successful response, got %+v", resp)
	}
	if quotaManager.preconsumeAmount != 3000 {
		t.Fatalf("expected quota preconsume to use vip group cost 3000, got %f", quotaManager.preconsumeAmount)
	}
	if apiTokenQuotaManager.preauthorizedAmount != 3000 {
		t.Fatalf("expected API token preauthorization to use vip group cost 3000, got %f", apiTokenQuotaManager.preauthorizedAmount)
	}
	if apiTokenQuotaManager.settledAmount != 3000 {
		t.Fatalf("expected API token settlement to use vip group cost 3000, got %f", apiTokenQuotaManager.settledAmount)
	}
	if len(usageLogger.records) != 1 {
		t.Fatalf("expected one usage record, got %d", len(usageLogger.records))
	}
	if usageLogger.records[0].Cost != 3000 {
		t.Fatalf("expected usage record to use vip group cost 3000, got %f", usageLogger.records[0].Cost)
	}
	if usageLogger.records[0].PriceSnapshot == nil || usageLogger.records[0].PriceSnapshot.GroupMultiplier != 0.5 || usageLogger.records[0].PriceSnapshot.TotalCost != 3000 {
		t.Fatalf("expected usage record to include vip price snapshot, got %+v", usageLogger.records[0].PriceSnapshot)
	}
	if len(usageLogger.records[0].PriceSnapshot.Dimensions) != 2 || usageLogger.records[0].PriceSnapshot.Dimensions[0].Currency != "quota" {
		t.Fatalf("expected prompt/completion price dimensions in usage snapshot, got %+v", usageLogger.records[0].PriceSnapshot.Dimensions)
	}
}

func TestRouterRouteWithBillingRecordsRequestLogBillingMetadata(t *testing.T) {
	router, _ := newRetryAffinityTestRouter()
	quotaManager := &stubQuotaManager{}
	apiTokenQuotaManager := &recordingAPITokenQuotaManager{}
	usageLogger := &recordingUsageLogger{}
	router.SetQuotaManager(quotaManager)
	router.SetAPITokenQuotaManager(apiTokenQuotaManager)
	router.SetUsageLogger(usageLogger)

	requestLogScope := types.NewRequestLogScope()
	ctx := types.WithTrustedUserID(context.Background(), "user_metered")
	ctx = types.WithTrustedOrganizationID(ctx, "org_metered")
	ctx = types.WithTrustedAPITokenID(ctx, "tok_metered")
	ctx = types.WithTrustedRequestID(ctx, "req_metered")
	ctx = types.WithRequestLogScope(ctx, requestLogScope)

	resp, err := router.RouteWithBilling(ctx, types.APITypeChat, "gpt-4o-mini", "", "idem_request_log_scope", &types.Usage{TotalTokens: 200}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		return types.NewOKResponse([]byte(`{"ok":true}`), &types.Usage{PromptTokens: 80, CompletionTokens: 40, TotalTokens: 120}), nil
	})

	if err != nil {
		t.Fatalf("RouteWithBilling returned error: %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected successful response, got %+v", resp)
	}
	metadata, ok := requestLogScope.Snapshot()
	if !ok {
		t.Fatal("expected request log metadata to be recorded")
	}
	if metadata.Model != "gpt-4o-mini" ||
		metadata.RequestedModel != "gpt-4o-mini" ||
		metadata.ResolvedModel != "gpt-4o-mini" ||
		metadata.ChannelID != "primary" ||
		metadata.Provider != "openai" ||
		metadata.BillingSessionID != "bill_test" ||
		metadata.RequestTokens != 80 ||
		metadata.ResponseTokens != 40 ||
		metadata.TotalTokens != 120 ||
		metadata.Cost != usageLogger.records[0].Cost ||
		metadata.ChannelCost != usageLogger.records[0].ChannelCost ||
		metadata.PreauthorizedAmount != quotaManager.preconsumeAmount ||
		metadata.TokenPreauthorizedAmount != apiTokenQuotaManager.preauthorizedAmount ||
		metadata.Status != string(RelayUsageStatusSuccess) ||
		metadata.PriceCurrency != usageLogger.records[0].PriceCurrency ||
		metadata.PriceSource != usageLogger.records[0].PriceSource {
		t.Fatalf("unexpected request log metadata: %+v usage=%+v quota=%+v token=%+v", metadata, usageLogger.records[0], quotaManager, apiTokenQuotaManager)
	}
	if metadata.PriceSnapshot == nil {
		t.Fatalf("expected price snapshot to be copied into request log metadata: %+v", metadata)
	}
}

func TestRouterRouteWithBillingFailsClosedWhenUsageRecordingFails(t *testing.T) {
	router, _ := newRetryAffinityTestRouter()
	quotaManager := &stubQuotaManager{}
	apiTokenQuotaManager := &recordingAPITokenQuotaManager{}
	usageLogger := &recordingUsageLogger{err: errors.New("usage store unavailable")}
	router.SetQuotaManager(quotaManager)
	router.SetAPITokenQuotaManager(apiTokenQuotaManager)
	router.SetUsageLogger(usageLogger)

	ctx := types.WithTrustedUserID(context.Background(), "user_metered")
	ctx = types.WithTrustedOrganizationID(ctx, "org_metered")
	ctx = types.WithTrustedAPITokenID(ctx, "tok_metered")
	ctx = types.WithTrustedRequestID(ctx, "req_metered")

	resp, err := router.RouteWithBilling(ctx, types.APITypeChat, "gpt-4o-mini", "", "idem_usage_fail", &types.Usage{TotalTokens: 100}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		return types.NewOKResponse([]byte(`{"ok":true}`), &types.Usage{PromptTokens: 40, CompletionTokens: 60, TotalTokens: 100}), nil
	})

	if err == nil {
		t.Fatal("expected usage recording failure")
	}
	if resp != nil {
		t.Fatalf("expected nil provider response on fail-closed metering error, got %+v", resp)
	}
	var routeErr *RouterError
	if !errors.As(err, &routeErr) {
		t.Fatalf("expected RouterError, got %T: %v", err, err)
	}
	if routeErr.Code != http.StatusInternalServerError || routeErr.ErrorCode != "usage_recording_failed" {
		t.Fatalf("expected usage_recording_failed 500, got %+v", routeErr)
	}
	if quotaManager.settleCalls != 0 {
		t.Fatalf("expected quota settlement to be skipped, got %d calls", quotaManager.settleCalls)
	}
	if quotaManager.refundCalls != 1 {
		t.Fatalf("expected quota preauthorization refund, got %d calls", quotaManager.refundCalls)
	}
	if apiTokenQuotaManager.settleCalls != 0 {
		t.Fatalf("expected API token settlement to be skipped, got %d calls", apiTokenQuotaManager.settleCalls)
	}
	if apiTokenQuotaManager.refundCalls != 1 || apiTokenQuotaManager.refundedTokenID != "tok_metered" {
		t.Fatalf("expected API token preauthorization refund, got %+v", apiTokenQuotaManager)
	}
	if len(usageLogger.records) != 1 {
		t.Fatalf("expected one attempted usage record, got %+v", usageLogger.records)
	}
	if usageLogger.records[0].RequestID != "req_metered" || usageLogger.records[0].Status != RelayUsageStatusSuccess {
		t.Fatalf("expected attempted success usage evidence with request id, got %+v", usageLogger.records[0])
	}
}

func TestRouterRouteWithBillingRecordsPendingUsageBeforeStreamingProvider(t *testing.T) {
	router, _ := newRetryAffinityTestRouter()
	quotaManager := &stubQuotaManager{}
	apiTokenQuotaManager := &recordingAPITokenQuotaManager{}
	usageLogger := &recordingUsageLogger{}
	router.SetQuotaManager(quotaManager)
	router.SetAPITokenQuotaManager(apiTokenQuotaManager)
	router.SetUsageLogger(usageLogger)

	ctx := types.WithTrustedUserID(context.Background(), "user_stream")
	ctx = types.WithTrustedOrganizationID(ctx, "org_stream")
	ctx = types.WithTrustedAPITokenID(ctx, "tok_stream")
	ctx = types.WithTrustedRequestID(ctx, "req_stream")
	ctx = types.WithTrustedStreaming(ctx, true)
	estimatedUsage := &types.Usage{TotalTokens: 100}

	providerCalled := false
	resp, err := router.RouteWithBilling(ctx, types.APITypeChat, "gpt-4o-mini", "", "idem_stream_pending", estimatedUsage, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		providerCalled = true
		if len(usageLogger.records) != 1 {
			t.Fatalf("expected pending usage record before provider callback, got %+v", usageLogger.records)
		}
		pending := usageLogger.records[0]
		if pending.Status != RelayUsageStatusPending || pending.StatusCode != 0 {
			t.Fatalf("expected pending status before provider callback, got %+v", pending)
		}
		if pending.RequestID != "req_stream" || pending.APITokenID != "tok_stream" || pending.OrganizationID != "org_stream" {
			t.Fatalf("pending usage identity not preserved: %+v", pending)
		}
		if pending.PriceSnapshot == nil || pending.PriceSnapshot.TotalCost != pending.Cost || len(pending.PriceSnapshot.Dimensions) != 1 || pending.PriceSnapshot.Dimensions[0].Dimension != types.DimTotalTokens {
			t.Fatalf("pending usage should carry estimated total-token price snapshot, got %+v", pending.PriceSnapshot)
		}
		return types.NewOKResponse([]byte(`{"ok":true}`), &types.Usage{PromptTokens: 40, CompletionTokens: 60, TotalTokens: 100}), nil
	})

	if err != nil {
		t.Fatalf("RouteWithBilling returned error: %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected successful provider response, got %+v", resp)
	}
	if !providerCalled {
		t.Fatal("provider callback was not called")
	}
	if len(usageLogger.records) != 1 {
		t.Fatalf("expected pending usage record to be finalized in place, got %+v", usageLogger.records)
	}
	if usageLogger.records[0].Status != RelayUsageStatusSuccess || usageLogger.records[0].StatusCode != http.StatusOK {
		t.Fatalf("expected finalized success usage record, got %+v", usageLogger.records)
	}
	if usageLogger.records[0].PriceSnapshot == nil || len(usageLogger.records[0].PriceSnapshot.Dimensions) != 2 {
		t.Fatalf("expected finalized usage to replace pending snapshot with actual prompt/completion dimensions, got %+v", usageLogger.records[0].PriceSnapshot)
	}
	if quotaManager.settleCalls != 1 || apiTokenQuotaManager.settleCalls != 1 {
		t.Fatalf("expected quota and API-token settlement after final usage record, quota=%d token=%d", quotaManager.settleCalls, apiTokenQuotaManager.settleCalls)
	}
}

func TestRouterRouteWithBillingFinalizesStreamingAbortUsageAndRequestLogScope(t *testing.T) {
	router, _ := newRetryAffinityTestRouter()
	router.billingHook.pricing.SetPrice("gpt-4o-mini", types.APITypeRealtime, types.DimTotalTokens, 0.0001)
	quotaManager := &stubQuotaManager{}
	apiTokenQuotaManager := &recordingAPITokenQuotaManager{}
	usageLogger := &recordingUsageLogger{}
	router.SetQuotaManager(quotaManager)
	router.SetAPITokenQuotaManager(apiTokenQuotaManager)
	router.SetUsageLogger(usageLogger)

	requestLogScope := types.NewRequestLogScope()
	ctx := types.WithTrustedUserID(context.Background(), "user_realtime_abort")
	ctx = types.WithTrustedOrganizationID(ctx, "org_realtime_abort")
	ctx = types.WithTrustedAPITokenID(ctx, "tok_realtime_abort")
	ctx = types.WithTrustedRequestID(ctx, "req_realtime_abort")
	ctx = types.WithTrustedStreaming(ctx, true)
	ctx = types.WithRequestLogScope(ctx, requestLogScope)
	estimatedUsage := &types.Usage{TotalTokens: 100}

	resp, err := router.RouteWithBilling(ctx, types.APITypeRealtime, "gpt-4o-mini", "", "idem_realtime_abort", estimatedUsage, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		if len(usageLogger.records) == 0 {
			t.Fatal("expected pending usage record before realtime provider callback")
		}
		return nil, codedTestError{
			message: "realtime_usage_missing: upstream realtime stream closed without usage",
			code:    "realtime_usage_missing",
		}
	})

	if err == nil {
		t.Fatal("expected realtime streaming provider error")
	}
	if resp != nil {
		t.Fatalf("expected nil response on realtime abort, got %+v", resp)
	}
	if len(usageLogger.records) != 1 {
		t.Fatalf("expected pending usage record to be replaced by one terminal error record, got %+v", usageLogger.records)
	}
	record := usageLogger.records[0]
	if record.RequestID != "req_realtime_abort" || record.APITokenID != "tok_realtime_abort" || record.OrganizationID != "org_realtime_abort" {
		t.Fatalf("terminal realtime usage identity not preserved: %+v", record)
	}
	if record.APIType != types.APITypeRealtime.String() || record.Status != RelayUsageStatusError || record.StatusCode != http.StatusBadGateway || record.ErrorCode != "realtime_usage_missing" {
		t.Fatalf("expected terminal realtime_usage_missing usage record, got %+v", record)
	}
	if record.Cost != 0 || record.ChannelCost != 0 {
		t.Fatalf("realtime abort usage must not settle cost, got %+v", record)
	}
	if quotaManager.settleCalls != 0 || apiTokenQuotaManager.settleCalls != 0 {
		t.Fatalf("realtime abort must not settle quota, quota=%d token=%d", quotaManager.settleCalls, apiTokenQuotaManager.settleCalls)
	}
	if quotaManager.refundCalls == 0 || apiTokenQuotaManager.refundCalls == 0 {
		t.Fatalf("expected realtime abort to refund preauthorization, quota=%d token=%d", quotaManager.refundCalls, apiTokenQuotaManager.refundCalls)
	}
	metadata, ok := requestLogScope.Snapshot()
	if !ok {
		t.Fatal("expected realtime abort to record request-log billing metadata")
	}
	if metadata.RequestID != "req_realtime_abort" || metadata.OrganizationID != "org_realtime_abort" || metadata.UserID != "user_realtime_abort" {
		t.Fatalf("request-log metadata identity mismatch: %+v", metadata)
	}
	if metadata.Status != string(RelayUsageStatusError) || metadata.ErrorCode != "realtime_usage_missing" || metadata.ChannelID == "" || metadata.Provider == "" {
		t.Fatalf("request-log metadata should expose realtime abort failure join fields, got %+v", metadata)
	}
}

func TestRouterRouteWithBillingRecordsRuntimeMetricsForSuccess(t *testing.T) {
	router, _ := newRetryAffinityTestRouter()
	apiType := types.APITypeChat.String()
	before := testutil.ToFloat64(metrics.RelayRequestTotal.WithLabelValues("openai", "primary", apiType, "success", "miss"))

	resp, err := router.RouteWithBilling(context.Background(), types.APITypeChat, "gpt-4o-mini", "", "idem_metrics_success", &types.Usage{TotalTokens: 20}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		if routeChannelID(ch) != "primary" {
			t.Fatalf("expected primary channel, got %s", routeChannelID(ch))
		}
		return types.NewOKResponse([]byte(`{"ok":true}`), &types.Usage{TotalTokens: 20}), nil
	})

	if err != nil {
		t.Fatalf("RouteWithBilling returned error: %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected successful response, got %+v", resp)
	}
	after := testutil.ToFloat64(metrics.RelayRequestTotal.WithLabelValues("openai", "primary", apiType, "success", "miss"))
	if after != before+1 {
		t.Fatalf("expected relay request success metric to increment by 1, before=%v after=%v", before, after)
	}
	if count := testutil.CollectAndCount(metrics.RequestDuration, "relay_request_duration_seconds"); count == 0 {
		t.Fatal("expected relay request duration histogram to be collectable")
	}
	if got := testutil.ToFloat64(metrics.RelayChannelHealthScore.WithLabelValues("primary")); got <= 0 || got > 1 {
		t.Fatalf("expected primary health score in (0,1], got %v", got)
	}
}

func TestRouterRouteWithBillingRecordsRuntimeMetricsForTerminalProviderError(t *testing.T) {
	router, _ := newRetryAffinityTestRouter()
	apiType := types.APITypeChat.String()
	before := testutil.ToFloat64(metrics.RelayRequestTotal.WithLabelValues("openai", "primary", apiType, "error", "miss"))

	resp, err := router.RouteWithBilling(context.Background(), types.APITypeChat, "gpt-4o-mini", "", "idem_metrics_error", &types.Usage{TotalTokens: 20}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		return &types.ProviderResponse{
			StatusCode: http.StatusUnauthorized,
			Error:      &types.ProviderError{Code: "unauthorized", StatusCode: http.StatusUnauthorized, Retryable: false},
		}, nil
	})

	if err != nil {
		t.Fatalf("non-retryable provider response should return response without router error, got %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized response, got %+v", resp)
	}
	after := testutil.ToFloat64(metrics.RelayRequestTotal.WithLabelValues("openai", "primary", apiType, "error", "miss"))
	if after != before+1 {
		t.Fatalf("expected relay request error metric to increment by 1, before=%v after=%v", before, after)
	}
	if got := testutil.ToFloat64(metrics.RelayChannelHealthScore.WithLabelValues("primary")); got != 0 {
		t.Fatalf("expected invalid primary health score to be 0, got %v", got)
	}
}

func TestRouterRouteWithBillingRecordsRuntimeMetricsForSemanticCacheHitAndMiss(t *testing.T) {
	router, _ := newRetryAffinityTestRouter()
	router.semanticCache = relaycache.NewSemanticCache(relaycache.NewInMemorySemanticCacheStore(), relaycache.SemanticCacheOptions{
		Now: func() time.Time { return time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC) },
	})
	cacheReq := relaycache.SemanticCacheRequest{
		OrganizationID: "org_metrics_cache",
		Model:          "gpt-4o-mini",
		Query:          "What is relay semantic cache?",
	}
	if _, err := router.semanticCache.Store(context.Background(), cacheReq, json.RawMessage(`{"cached":true}`)); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	ctx := types.WithSemanticCacheRequest(context.Background(), cacheReq)
	apiType := types.APITypeChat.String()
	model := cacheReq.Model
	beforeHit := testutil.ToFloat64(metrics.RelayRequestTotal.WithLabelValues("semantic_cache", "semantic_cache", apiType, "success", "hit"))
	beforeCacheHit := testutil.ToFloat64(metrics.RelaySemanticCacheEventsTotal.WithLabelValues("hit", apiType, model))

	resp, err := router.RouteWithBilling(ctx, types.APITypeChat, model, "", "idem_metrics_cache_hit", &types.Usage{TotalTokens: 20}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		t.Fatalf("provider callback should not run on semantic cache hit, got %s", routeChannelID(ch))
		return nil, nil
	})

	if err != nil {
		t.Fatalf("RouteWithBilling returned error: %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected cached successful response, got %+v", resp)
	}
	afterHit := testutil.ToFloat64(metrics.RelayRequestTotal.WithLabelValues("semantic_cache", "semantic_cache", apiType, "success", "hit"))
	if afterHit != beforeHit+1 {
		t.Fatalf("expected relay request cache-hit metric to increment by 1, before=%v after=%v", beforeHit, afterHit)
	}
	afterCacheHit := testutil.ToFloat64(metrics.RelaySemanticCacheEventsTotal.WithLabelValues("hit", apiType, model))
	if afterCacheHit != beforeCacheHit+1 {
		t.Fatalf("expected cache hit event to increment by 1, before=%v after=%v", beforeCacheHit, afterCacheHit)
	}
}

func TestRouterRouteWithBillingRecordsFeatureTypeForSemanticCacheHit(t *testing.T) {
	router, _ := newRetryAffinityTestRouter()
	router.semanticCache = relaycache.NewSemanticCache(relaycache.NewInMemorySemanticCacheStore(), relaycache.SemanticCacheOptions{
		Now: func() time.Time { return time.Date(2026, 6, 6, 10, 0, 0, 0, time.UTC) },
	})
	usageLogger := &recordingUsageLogger{}
	router.SetUsageLogger(usageLogger)
	cacheReq := relaycache.SemanticCacheRequest{
		OrganizationID: "org_workflow_cache",
		Model:          "gpt-4o-mini",
		Query:          "Summarize workflow cache attribution",
	}
	if _, err := router.semanticCache.Store(context.Background(), cacheReq, json.RawMessage(`{"cached":true}`)); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	ctx := types.WithSemanticCacheRequest(context.Background(), cacheReq)
	ctx = types.WithTrustedFeatureType(ctx, "workflow")

	resp, err := router.RouteWithBilling(ctx, types.APITypeChat, cacheReq.Model, "", "idem_workflow_cache_hit", &types.Usage{TotalTokens: 20}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		t.Fatalf("provider callback should not run on semantic cache hit, got %s", routeChannelID(ch))
		return nil, nil
	})

	if err != nil {
		t.Fatalf("RouteWithBilling returned error: %v", err)
	}
	if resp == nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("expected cached successful response, got %+v", resp)
	}
	if len(usageLogger.records) != 1 {
		t.Fatalf("expected 1 semantic cache usage record, got %+v", usageLogger.records)
	}
	record := usageLogger.records[0]
	if record.Provider != "semantic_cache" || record.FeatureType != "workflow" {
		t.Fatalf("expected semantic cache usage to keep workflow feature attribution, got %+v", record)
	}
}

func newRetryAffinityTestRouter() (*Router, *memoryConversationAffinityStore) {
	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{
		ID:       "primary",
		Name:     "Primary",
		Provider: "openai",
		BaseURL:  "https://primary.example",
		APIKey:   "sk-primary",
		Models:   []string{"gpt-4o-mini"},
		Priority: 0,
		Enabled:  true,
	}, 100)
	pool.AddChannel(&types.Channel{
		ID:       "backup",
		Name:     "Backup",
		Provider: "openai",
		BaseURL:  "https://backup.example",
		APIKey:   "sk-backup",
		Models:   []string{"gpt-4o-mini"},
		Priority: 10,
		Enabled:  true,
	}, 100)
	router := NewRouterWithBilling(
		pool,
		NewLoadBalancer(pool, "priority"),
		map[string]*CircuitBreaker{
			"primary": NewCircuitBreaker("primary", 5, time.Second, time.Minute),
			"backup":  NewCircuitBreaker("backup", 5, time.Second, time.Minute),
		},
		nil,
		NewHealthChecker(HealthCheckDisabled, time.Second),
		testBillingHook(),
		"",
	)
	affinityStore := newMemoryConversationAffinityStore()
	router.affinityStore = affinityStore
	return router, affinityStore
}

func assertAttemptedChannels(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("attempted channels = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("attempted channels = %v, want %v", got, want)
		}
	}
}

type memoryConversationAffinityStore struct {
	bindings map[string]string
}

func newMemoryConversationAffinityStore() *memoryConversationAffinityStore {
	return &memoryConversationAffinityStore{bindings: make(map[string]string)}
}

func (s *memoryConversationAffinityStore) SaveConversationAffinity(_ context.Context, conversationID, channelID string) error {
	if conversationID == "" || channelID == "" {
		return nil
	}
	s.bindings[conversationID] = channelID
	return nil
}

func (s *memoryConversationAffinityStore) GetConversationAffinity(_ context.Context, conversationID string) (string, error) {
	return s.bindings[conversationID], nil
}

func (s *memoryConversationAffinityStore) channelFor(conversationID string) string {
	return s.bindings[conversationID]
}
