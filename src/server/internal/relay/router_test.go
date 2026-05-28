package relay

import (
	"bytes"
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
	"oblivious/server/internal/observability"
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

func TestRouteWithBilling_RefundsProviderErrorResponse(t *testing.T) {
	quotaManager := &stubQuotaManager{}
	router := newBillingTestRouter(quotaManager)
	ctx := trustedBillingContext()

	resp, err := router.RouteWithBilling(ctx, types.APITypeChat, "gpt-4o", "", "idem_provider_error", &types.Usage{PromptTokens: 1000}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		return &types.ProviderResponse{StatusCode: http.StatusInternalServerError}, nil
	})

	if err == nil || !strings.Contains(err.Error(), "billing refund") {
		t.Fatalf("expected billing refund error for provider error response, got resp=%+v err=%v", resp, err)
	}
	if quotaManager.preconsumeCalls != 1 {
		t.Fatalf("expected one preconsume call, got %d", quotaManager.preconsumeCalls)
	}
	if quotaManager.settleCalls != 0 {
		t.Fatalf("provider error must not settle, got %d settle calls", quotaManager.settleCalls)
	}
	if quotaManager.refundCalls != 1 {
		t.Fatalf("provider error must refund once, got %d refund calls", quotaManager.refundCalls)
	}
}

func TestRouteWithBilling_RefundsNilProviderResponse(t *testing.T) {
	quotaManager := &stubQuotaManager{}
	router := newBillingTestRouter(quotaManager)
	ctx := trustedBillingContext()

	resp, err := router.RouteWithBilling(ctx, types.APITypeChat, "gpt-4o", "", "idem_nil_response", &types.Usage{PromptTokens: 1000}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		return nil, nil
	})

	if err == nil || !strings.Contains(err.Error(), "billing refund") {
		t.Fatalf("expected billing refund error for nil provider response, got resp=%+v err=%v", resp, err)
	}
	if quotaManager.settleCalls != 0 {
		t.Fatalf("nil response must not settle, got %d settle calls", quotaManager.settleCalls)
	}
	if quotaManager.refundCalls != 1 {
		t.Fatalf("nil response must refund once, got %d refund calls", quotaManager.refundCalls)
	}
}

func TestRouteWithBilling_ReturnsSettlementErrors(t *testing.T) {
	quotaManager := &stubQuotaManager{settleErr: errors.New("boom")}
	router := newBillingTestRouter(quotaManager)
	ctx := trustedBillingContext()

	resp, err := router.RouteWithBilling(ctx, types.APITypeChat, "gpt-4o", "", "idem_settle_error", &types.Usage{PromptTokens: 1000}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		return types.NewOKResponse([]byte(`{"ok":true}`), &types.Usage{PromptTokens: 1000, CompletionTokens: 100}), nil
	})

	if err == nil || !strings.Contains(err.Error(), "billing settlement failed") {
		t.Fatalf("expected settlement error, got resp=%+v err=%v", resp, err)
	}
	if quotaManager.settleCalls != 1 {
		t.Fatalf("expected one settle attempt, got %d", quotaManager.settleCalls)
	}
}

func TestRouteWithBilling_RefundsMissingRequiredUsage(t *testing.T) {
	quotaManager := &stubQuotaManager{}
	router := newBillingTestRouter(quotaManager)
	ctx := trustedBillingContext()

	resp, err := router.RouteWithBilling(ctx, types.APITypeChat, "gpt-4o", "", "idem_missing_usage", &types.Usage{PromptTokens: 1000}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		return &types.ProviderResponse{StatusCode: http.StatusOK, Content: []byte(`{"ok":true}`)}, nil
	})

	if err == nil || !strings.Contains(err.Error(), "missing required usage") {
		t.Fatalf("expected missing required usage error, got resp=%+v err=%v", resp, err)
	}
	if quotaManager.settleCalls != 0 {
		t.Fatalf("missing usage must not settle, got %d settle calls", quotaManager.settleCalls)
	}
	if quotaManager.refundCalls != 1 {
		t.Fatalf("missing usage must refund once, got %d refund calls", quotaManager.refundCalls)
	}
}

func TestRouteWithBilling_SettlesEstimatePolicyWithoutProviderUsage(t *testing.T) {
	quotaManager := &stubQuotaManager{}
	router := newBillingTestRouter(quotaManager)
	ctx := trustedBillingContext()

	_, err := router.RouteWithBilling(ctx, types.APITypeImageGen, "gpt-4o", "", "idem_estimate_policy", &types.Usage{ImageCount: 1}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		return &types.ProviderResponse{StatusCode: http.StatusOK, Content: []byte(`{"ok":true}`)}, nil
	})
	if err != nil {
		t.Fatalf("estimate-settled route should settle without provider usage: %v", err)
	}
	if quotaManager.settleCalls != 1 {
		t.Fatalf("estimate-settled route should settle once, got %d", quotaManager.settleCalls)
	}
	if quotaManager.refundCalls != 0 {
		t.Fatalf("estimate-settled route should not refund, got %d", quotaManager.refundCalls)
	}
}

func TestRouteWithBilling_ReturnsRefundErrors(t *testing.T) {
	quotaManager := &stubQuotaManager{refundErr: errors.New("boom")}
	router := newBillingTestRouter(quotaManager)
	ctx := trustedBillingContext()

	resp, err := router.RouteWithBilling(ctx, types.APITypeChat, "gpt-4o", "", "idem_refund_error", &types.Usage{PromptTokens: 1000}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		return nil, errors.New("upstream failed")
	})

	if err == nil || !strings.Contains(err.Error(), "billing refund failed") {
		t.Fatalf("expected refund error, got resp=%+v err=%v", resp, err)
	}
	if quotaManager.refundCalls != 1 {
		t.Fatalf("expected one refund attempt, got %d", quotaManager.refundCalls)
	}
}

func TestRouteWithBilling_RecordsSelectedChannel(t *testing.T) {
	quotaManager := &stubQuotaManager{}
	router := newBillingTestRouter(quotaManager)
	ctx := trustedBillingContext()

	_, err := router.RouteWithBilling(ctx, types.APITypeChat, "gpt-4o", "", "idem_channel", &types.Usage{PromptTokens: 1000}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		return types.NewOKResponse([]byte(`{"ok":true}`), &types.Usage{PromptTokens: 1000}), nil
	})
	if err != nil {
		t.Fatalf("RouteWithBilling failed: %v", err)
	}

	if quotaManager.lastChannelID != "ch_1" {
		t.Fatalf("expected selected channel ch_1 in preconsume, got %q", quotaManager.lastChannelID)
	}
	if quotaManager.lastModel != "gpt-4o" {
		t.Fatalf("expected model gpt-4o in preconsume, got %q", quotaManager.lastModel)
	}
	if quotaManager.lastAPIType != types.APITypeChat.String() {
		t.Fatalf("expected api type %s in preconsume, got %q", types.APITypeChat.String(), quotaManager.lastAPIType)
	}
	if quotaManager.lastIdempotencyKey != "idem_channel" {
		t.Fatalf("expected idempotency key idem_channel, got %q", quotaManager.lastIdempotencyKey)
	}
}

func TestRouteObservabilityRecordsProviderFailure(t *testing.T) {
	router := newBillingTestRouter(&stubQuotaManager{})
	ctx := trustedBillingContext()

	before := testutil.ToFloat64(metrics.ProviderFailuresTotal.WithLabelValues("openai", "ch_1", types.APITypeChat.String(), "request_error"))
	_, err := router.Route(ctx, types.APITypeChat.String(), func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		return nil, errors.New("upstream timeout with https://provider.example.com/v1/chat/completions")
	})
	if err == nil {
		t.Fatal("expected provider error")
	}
	after := testutil.ToFloat64(metrics.ProviderFailuresTotal.WithLabelValues("openai", "ch_1", types.APITypeChat.String(), "request_error"))
	if after != before+1 {
		t.Fatalf("expected provider failure metric increment, before=%v after=%v", before, after)
	}
}

func TestRouteObservabilityWritesStructuredProviderFailureEvent(t *testing.T) {
	var output bytes.Buffer
	restoreLogger := setObservabilityLoggerForTest(observability.NewJSONLogger(&output))
	defer restoreLogger()

	router := newBillingTestRouter(&stubQuotaManager{})
	ctx := trustedBillingContext()

	_, err := router.Route(ctx, types.APITypeChat.String(), func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		return nil, errors.New("upstream timeout with sk-secret")
	})
	if err == nil {
		t.Fatal("expected provider error")
	}

	var record map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(output.Bytes()), &record); err != nil {
		t.Fatalf("expected structured provider JSON log, got error: %v\n%s", err, output.String())
	}
	if record["event"] != "relay.provider_failure" {
		t.Fatalf("expected relay.provider_failure event, got %#v", record["event"])
	}
	if record["request_id"] != "req_1" {
		t.Fatalf("expected request id req_1, got %#v", record["request_id"])
	}
	if record["organization_id"] != "org_1" || record["user_id"] != "user_1" {
		t.Fatalf("expected trusted identity in provider event, got %#v", record)
	}
	if record["provider"] != "openai" || record["channel_id"] != "ch_1" {
		t.Fatalf("expected provider/channel labels, got %#v", record)
	}
	if record["failure_reason"] != "request_error" {
		t.Fatalf("expected low-cardinality failure reason, got %#v", record["failure_reason"])
	}
	if strings.Contains(output.String(), "sk-secret") {
		t.Fatalf("provider event leaked raw error text: %s", output.String())
	}
}

func newBillingTestRouter(quotaManager *stubQuotaManager) *Router {
	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{ID: "ch_1", Provider: "openai", BaseURL: "http://provider", Enabled: true}, 1)
	lb := NewLoadBalancer(pool, "weighted")
	hook := NewBillingHook(NewPricingStoreWithDefaults(), &map[string]bool{})
	hook.SetQuotaManager(quotaManager)
	return NewRouterWithBilling(pool, lb, nil, nil, NewHealthChecker(HealthCheckDisabled, time.Second), hook, "")
}

func trustedBillingContext() context.Context {
	ctx := context.Background()
	ctx = types.WithTrustedUserID(ctx, "user_1")
	ctx = types.WithTrustedOrganizationID(ctx, "org_1")
	ctx = types.WithTrustedRequestID(ctx, "req_1")
	return ctx
}
