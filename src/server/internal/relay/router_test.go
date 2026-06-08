package relay

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"oblivious/server/internal/metrics"
	relaycache "oblivious/server/internal/relay/cache"
	"oblivious/server/internal/relay/types"
)

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
		nil,
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
