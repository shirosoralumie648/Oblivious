package relay

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"oblivious/server/internal/observability"
	"oblivious/server/internal/quota"
	relaycache "oblivious/server/internal/relay/cache"
	"oblivious/server/internal/relay/handler"
	"oblivious/server/internal/relay/ratelimit"
	"oblivious/server/internal/relay/types"
)

func TestNewRelayRegistersCommercialChatRoute(t *testing.T) {
	relayInstance, err := NewRelay(&Config{Production: true})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"ping"}]}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	relayInstance.Engine().ServeHTTP(recorder, request)

	if recorder.Code == http.StatusNotFound {
		t.Fatalf("commercial chat route must be registered, got 404 with body %s", recorder.Body.String())
	}
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("production chat route should reject missing trusted identity with 401, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func TestNewRelayAcceptsConfiguredAPITokenAuthenticator(t *testing.T) {
	authenticator := &recordingRelayAuthenticator{
		identity: types.RelayAPITokenIdentity{
			TokenID:        "tok_1",
			UserID:         "user_1",
			OrganizationID: "org_1",
		},
	}
	relayInstance, err := NewRelay(&Config{
		Production:            true,
		APITokenAuthenticator: authenticator,
	})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"ping"}]}`))
	request.Header.Set("Authorization", "Bearer obv_test")
	recorder := httptest.NewRecorder()

	relayInstance.Engine().ServeHTTP(recorder, request)

	if authenticator.rawToken != "obv_test" || authenticator.model != "gpt-4o-mini" || authenticator.apiType != types.APITypeChat {
		t.Fatalf("authenticator saw token=%q model=%q apiType=%s", authenticator.rawToken, authenticator.model, authenticator.apiType.String())
	}
	if recorder.Code == http.StatusUnauthorized {
		t.Fatalf("configured API token authenticator should bypass trusted-header 401, got body %s", recorder.Body.String())
	}
}

func TestNewRelayUsesConfiguredPricingStore(t *testing.T) {
	pricing := NewPricingStoreWithDefaults()
	relayInstance, err := NewRelay(&Config{PricingStore: pricing})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}

	if relayInstance.router == nil || relayInstance.router.billingHook == nil {
		t.Fatalf("expected relay billing hook to be configured")
	}
	if relayInstance.router.billingHook.pricing != pricing {
		t.Fatalf("relay should use configured pricing store")
	}
}

func TestNewRelayUsesConfiguredRateLimiter(t *testing.T) {
	limiter := &recordingRelayRateLimiter{}
	relayInstance, err := NewRelay(&Config{RateLimiter: limiter})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}

	if relayInstance.router == nil {
		t.Fatal("expected relay router to be configured")
	}
	if relayInstance.router.rateLimiter != limiter {
		t.Fatalf("relay should use configured rate limiter, got %T", relayInstance.router.rateLimiter)
	}
}

func TestNewRelayUsesConfiguredConversationAffinityStore(t *testing.T) {
	store := &recordingConversationAffinityStore{}
	relayInstance, err := NewRelay(&Config{ConversationAffinityStore: store})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}

	if relayInstance.router == nil {
		t.Fatal("expected relay router to be configured")
	}
	if relayInstance.router.affinityStore != store {
		t.Fatalf("relay should use configured conversation affinity store")
	}
}

func TestNewRelayUsesConfiguredSemanticCacheStore(t *testing.T) {
	store := &recordingSemanticCacheStore{}
	relayInstance, err := NewRelay(&Config{SemanticCacheStore: store})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}

	cacheReq := relaycache.SemanticCacheRequest{
		Model: "gpt-4o-mini",
		Query: "stable relay semantic cache",
	}
	if _, err := relayInstance.cache.Store(context.Background(), cacheReq, json.RawMessage(`{"cached":true}`)); err != nil {
		t.Fatalf("store semantic cache: %v", err)
	}

	if store.puts != 1 {
		t.Fatalf("expected configured semantic cache store to receive put, got %d", store.puts)
	}
}

func TestNewRelayCanDisableSemanticCache(t *testing.T) {
	relayInstance, err := NewRelay(&Config{SemanticCacheDisabled: true})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}

	if relayInstance.cache != nil {
		t.Fatalf("expected semantic cache to be disabled, got %+v", relayInstance.cache)
	}
	if relayInstance.router != nil && relayInstance.router.semanticCache != nil {
		t.Fatalf("expected router semantic cache to be disabled, got %+v", relayInstance.router.semanticCache)
	}
}

func TestNewRelayFilesUploadUsesConfiguredChannelAndMappingStore(t *testing.T) {
	var upstreamPath string
	var upstreamAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		upstreamAuth = r.Header.Get("Authorization")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("parse upstream multipart: %v", err)
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			t.Fatalf("upstream missing file: %v", err)
		}
		defer file.Close()
		if header.Filename != "payload.jsonl" {
			t.Fatalf("upstream filename = %q, want payload.jsonl", header.Filename)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"file_openai_relay","object":"file","bytes":5}`))
	}))
	t.Cleanup(upstream.Close)

	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{
		ID:          "ch_files",
		Name:        "Files channel",
		Provider:    "openai",
		BaseURL:     upstream.URL,
		APIKey:      "sk-files",
		Models:      []string{types.APITypeFiles.String()},
		Enabled:     true,
		CBThreshold: 5,
	}, 100)
	mappingStore := &recordingRelayFilesMappingStore{}
	relayInstance, err := NewRelay(&Config{
		Pool:              pool,
		FilesMappingStore: mappingStore,
	})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}

	body, contentType := relayMultipartFileBody(t, "payload.jsonl", "hello", "assistants")
	request := httptest.NewRequest(http.MethodPost, "/v1/files", body)
	request.Header.Set("Content-Type", contentType)
	request.Header.Set(types.HeaderInternalAuth, types.SharedInternalToken)
	request.Header.Set(types.HeaderInternalUserID, "user_files")
	request.Header.Set(types.HeaderInternalOrganization, "org_files")
	request.Header.Set(types.HeaderRequestID, "req_files")
	recorder := httptest.NewRecorder()

	relayInstance.Engine().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if upstreamPath != "/v1/files" || upstreamAuth != "Bearer sk-files" {
		t.Fatalf("upstream path/auth = %q/%q, want /v1/files/Bearer sk-files", upstreamPath, upstreamAuth)
	}
	if len(mappingStore.records) != 1 {
		t.Fatalf("mapping records = %d, want 1", len(mappingStore.records))
	}
	if mappingStore.records[0].OpenAIFileID != "file_openai_relay" ||
		mappingStore.records[0].UserID != "user_files" ||
		mappingStore.records[0].OrganizationID != "org_files" ||
		mappingStore.records[0].RequestID != "req_files" {
		t.Fatalf("unexpected file mapping record: %+v", mappingStore.records[0])
	}
}

func TestNewRelayFilesUploadRemainsFailClosedWithoutMappingStore(t *testing.T) {
	relayInstance, err := NewRelay(&Config{})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}

	body, contentType := relayMultipartFileBody(t, "payload.jsonl", "hello", "assistants")
	request := httptest.NewRequest(http.MethodPost, "/v1/files", body)
	request.Header.Set("Content-Type", contentType)
	recorder := httptest.NewRecorder()

	relayInstance.Engine().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusNotImplemented, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "relay_file_mapping_store_required") {
		t.Fatalf("expected mapping store guard, got %s", recorder.Body.String())
	}
}

func TestRelayStartHealthChecksMarksUnhealthyChannelInvalid(t *testing.T) {
	probeCalls := make(chan struct{}, 4)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected health probe path %q", r.URL.Path)
		}
		probeCalls <- struct{}{}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(upstream.Close)

	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{
		ID:       "ch_unhealthy",
		Name:     "Unhealthy OpenAI",
		Provider: "openai",
		BaseURL:  upstream.URL,
		APIKey:   "sk-test",
		Models:   []string{"gpt-4o-mini"},
		Enabled:  true,
	}, 100)
	relayInstance, err := NewRelay(&Config{
		Pool:                        pool,
		HealthCheckFailureThreshold: 1,
		HealthCheckInterval:         10 * time.Millisecond,
		HealthCheckTimeout:          time.Second,
	})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	relayInstance.StartHealthChecks(ctx)

	select {
	case <-probeCalls:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("expected background health checker to probe channel")
	}

	deadline := time.After(300 * time.Millisecond)
	for {
		stats, _ := pool.GetStats("ch_unhealthy")
		if stats != nil && stats.Invalid {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("expected unhealthy channel to be marked invalid, got %+v", stats)
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func TestRelayHealthChecksRouteChannelUnhealthyAlertAndResolveOnRecovery(t *testing.T) {
	failProbes := true
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Fatalf("unexpected health probe path %q", r.URL.Path)
		}
		if failProbes {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"}]}`))
	}))
	t.Cleanup(upstream.Close)

	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{
		ID:       "ch_relay_health",
		Name:     "Relay Health Channel",
		Provider: "openai",
		BaseURL:  upstream.URL,
		APIKey:   "sk-health",
		Models:   []string{"gpt-4o-mini"},
		Enabled:  true,
	}, 100)
	alertStore := observability.NewInMemoryAlertStateStore()
	relayInstance, err := NewRelay(&Config{
		Pool:                        pool,
		HealthCheckFailureThreshold: 1,
		HealthAlertSink:             observability.NewAlertRouter(observability.AlertRouterOptions{StateStore: alertStore}),
		HealthAlertStateStore:       alertStore,
		HealthRecoveryController: observability.NewRecoveryController(observability.RecoveryControllerOptions{
			StateStore: alertStore,
			Policies: []observability.RecoveryPolicy{
				{
					Name:       "record-relay-channel-unhealthy",
					Severity:   observability.AlertSeverityWarning,
					Component:  observability.ComponentRelay,
					ActionType: observability.RecoveryActionFailover,
				},
			},
		}),
	})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}

	relayInstance.runHealthCheckOnce(context.Background())

	const alertKey = "relay:channel:ch_relay_health:unhealthy"
	state, found, err := alertStore.GetAlertState(context.Background(), alertKey)
	if err != nil {
		t.Fatalf("get alert state: %v", err)
	}
	if !found || state.Status != observability.AlertStatusOpen || state.Component != observability.ComponentRelay {
		t.Fatalf("expected open relay health alert, found=%v state=%+v", found, state)
	}
	actions, err := alertStore.ListRecoveryActions(context.Background(), observability.RecoveryActionFilter{AlertKey: alertKey})
	if err != nil {
		t.Fatalf("list recovery actions: %v", err)
	}
	if len(actions) != 1 || actions[0].PolicyName != "record-relay-channel-unhealthy" || actions[0].Type != observability.RecoveryActionFailover {
		t.Fatalf("expected relay health failover recovery action, got %+v", actions)
	}

	failProbes = false
	relayInstance.runHealthCheckOnce(context.Background())

	resolved, found, err := alertStore.GetAlertState(context.Background(), alertKey)
	if err != nil {
		t.Fatalf("get resolved alert state: %v", err)
	}
	if !found || resolved.Status != observability.AlertStatusResolved || resolved.ResolvedAt.IsZero() {
		t.Fatalf("expected resolved relay health alert, found=%v state=%+v", found, resolved)
	}
}

func TestRouterRouteWithBillingRejectsRateLimitedRequestBeforeUpstream(t *testing.T) {
	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{
		ID:       "ch_rate_limited",
		Name:     "Rate Limited",
		Provider: "openai",
		BaseURL:  "https://upstream.example",
		APIKey:   "sk-rate",
		Models:   []string{"gpt-4o-mini"},
		Enabled:  true,
		RPMLimit: 1,
		TPMLimit: 1000,
	}, 100)
	router := NewRouterWithBilling(
		pool,
		NewLoadBalancer(pool, "weighted"),
		map[string]*CircuitBreaker{"ch_rate_limited": NewCircuitBreaker("ch_rate_limited", 5, time.Second, time.Minute)},
		nil,
		NewHealthChecker(HealthCheckDisabled, time.Second),
		nil,
		"",
	)
	limiter := &recordingRelayRateLimiter{
		allowErr: &ratelimit.LimitError{
			Key: ratelimit.Key{ChannelID: "ch_rate_limited", Model: "gpt-4o-mini", TokenID: "tok_rate"},
			Decision: ratelimit.Decision{
				Allowed:    false,
				Dimension:  ratelimit.DimensionRPM,
				Limit:      1,
				Current:    1,
				RetryAfter: 15 * time.Second,
			},
		},
	}
	router.rateLimiter = limiter
	router.rateLimitResolver = func(ctx context.Context, ch *types.RouteChannel, model string, usage *types.Usage) RateLimitResolution {
		return RateLimitResolution{
			Key: ratelimit.Key{
				ChannelID: routeChannelID(ch),
				Model:     model,
				TokenID:   "tok_rate",
			},
			Limits: ratelimit.Limits{RPM: 1, TPM: 1000, MaxConcurrent: 1},
		}
	}
	usageLogger := &recordingUsageLogger{}
	router.SetUsageLogger(usageLogger)

	upstreamCalls := 0
	ctx := types.WithTrustedFeatureType(context.Background(), "workflow")
	resp, err := router.RouteWithBilling(ctx, types.APITypeChat, "gpt-4o-mini", "", "idem_rate", &types.Usage{TotalTokens: 20}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		upstreamCalls++
		return types.NewOKResponse([]byte(`{"ok":true}`), nil), nil
	})

	if resp != nil {
		t.Fatalf("expected no upstream response for rate-limited request, got %+v", resp)
	}
	var routeErr *RouterError
	if !errors.As(err, &routeErr) || routeErr.Code != http.StatusTooManyRequests || routeErr.ErrorCode != "relay_rate_limited" || routeErr.RetryAfter != 15 {
		t.Fatalf("expected 429 relay_rate_limited router error, got %#v", err)
	}
	if upstreamCalls != 0 {
		t.Fatalf("rate-limited request should not call upstream, got %d calls", upstreamCalls)
	}
	if limiter.allowCalls != 1 || limiter.beginCalls != 0 || limiter.endCalls != 0 {
		t.Fatalf("unexpected limiter calls allow=%d begin=%d end=%d", limiter.allowCalls, limiter.beginCalls, limiter.endCalls)
	}
	if len(usageLogger.records) != 1 || usageLogger.records[0].StatusCode != http.StatusTooManyRequests || usageLogger.records[0].ErrorCode != "relay_rate_limited" {
		t.Fatalf("expected rate limit usage record, got %+v", usageLogger.records)
	}
	if usageLogger.records[0].FeatureType != "workflow" {
		t.Fatalf("expected rate limit usage to keep workflow feature attribution, got %+v", usageLogger.records[0])
	}
}

func TestRouterRouteWithBillingReducesChannelWeightNearLocalRPMLimit(t *testing.T) {
	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{
		ID:       "primary",
		Name:     "Primary",
		Provider: "openai",
		BaseURL:  "https://primary.example",
		APIKey:   "sk-primary",
		Models:   []string{"gpt-4o-mini"},
		Enabled:  true,
		RPMLimit: 10,
	}, 100)
	pool.AddChannel(&types.Channel{
		ID:       "backup",
		Name:     "Backup",
		Provider: "openai",
		BaseURL:  "https://backup.example",
		APIKey:   "sk-backup",
		Models:   []string{"gpt-4o-mini"},
		Enabled:  true,
		RPMLimit: 10,
	}, 10)

	router := NewRouterWithBilling(
		pool,
		NewLoadBalancer(pool, "weighted"),
		map[string]*CircuitBreaker{
			"primary": NewCircuitBreaker("primary", 5, time.Second, time.Minute),
			"backup":  NewCircuitBreaker("backup", 5, time.Second, time.Minute),
		},
		nil,
		NewHealthChecker(HealthCheckDisabled, time.Second),
		nil,
		"",
	)
	limiter := &recordingRelayRateLimiter{
		checkDecisions: map[string]ratelimit.Decision{
			"primary": {Allowed: true, Dimension: ratelimit.DimensionRPM, Limit: 10, Current: 9, Remaining: 1},
			"backup":  {Allowed: true, Dimension: ratelimit.DimensionRPM, Limit: 10, Current: 0, Remaining: 10},
		},
	}
	router.rateLimiter = limiter

	counts := map[string]int{}
	for i := 0; i < 20; i++ {
		resp, err := router.RouteWithBilling(context.Background(), types.APITypeChat, "gpt-4o-mini", "", "idem_soft_limit", &types.Usage{TotalTokens: 20}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
			counts[routeChannelID(ch)]++
			return types.NewOKResponse([]byte(`{"ok":true}`), &types.Usage{TotalTokens: 20}), nil
		})
		if err != nil {
			t.Fatalf("RouteWithBilling returned error on attempt %d: %v", i+1, err)
		}
		if resp == nil || resp.StatusCode != http.StatusOK {
			t.Fatalf("expected successful response on attempt %d, got %+v", i+1, resp)
		}
	}

	if counts["backup"] == 0 {
		t.Fatalf("expected near-limit primary channel to be down-weighted enough for backup traffic, got counts=%v", counts)
	}
	if counts["primary"] >= 20 {
		t.Fatalf("near-limit primary channel should not keep full static weight, got counts=%v", counts)
	}
}

func TestRouterRouteWithBillingReleasesConcurrencyAfterUpstream(t *testing.T) {
	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{
		ID:       "ch_concurrent",
		Name:     "Concurrent",
		Provider: "openai",
		BaseURL:  "https://upstream.example",
		APIKey:   "sk-concurrent",
		Models:   []string{"gpt-4o-mini"},
		Enabled:  true,
	}, 100)
	router := NewRouterWithBilling(
		pool,
		NewLoadBalancer(pool, "weighted"),
		map[string]*CircuitBreaker{"ch_concurrent": NewCircuitBreaker("ch_concurrent", 5, time.Second, time.Minute)},
		nil,
		NewHealthChecker(HealthCheckDisabled, time.Second),
		nil,
		"",
	)
	limiter := &recordingRelayRateLimiter{}
	router.rateLimiter = limiter
	router.rateLimitResolver = func(ctx context.Context, ch *types.RouteChannel, model string, usage *types.Usage) RateLimitResolution {
		return RateLimitResolution{
			Key:    ratelimit.Key{ChannelID: routeChannelID(ch), Model: model, TokenID: "tok_concurrent"},
			Limits: ratelimit.Limits{RPM: 10, TPM: 1000, MaxConcurrent: 1},
		}
	}

	_, err := router.RouteWithBilling(context.Background(), types.APITypeChat, "gpt-4o-mini", "", "idem_concurrent", &types.Usage{TotalTokens: 20}, func(ch *types.RouteChannel) (*types.ProviderResponse, error) {
		return types.NewOKResponse([]byte(`{"ok":true}`), &types.Usage{TotalTokens: 20}), nil
	})
	if err != nil {
		t.Fatalf("route with billing: %v", err)
	}
	if limiter.allowCalls != 1 || limiter.beginCalls != 1 || limiter.endCalls != 1 {
		t.Fatalf("expected allow/begin/end once, got allow=%d begin=%d end=%d", limiter.allowCalls, limiter.beginCalls, limiter.endCalls)
	}
	if limiter.lastEndKey != limiter.lastBeginKey {
		t.Fatalf("expected End to release Begin key, begin=%+v end=%+v", limiter.lastBeginKey, limiter.lastEndKey)
	}
}

func relayMultipartFileBody(t *testing.T, filename, content, purpose string) (*bytes.Buffer, string) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("CreateFormFile error = %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("write multipart file error = %v", err)
	}
	if purpose != "" {
		if err := writer.WriteField("purpose", purpose); err != nil {
			t.Fatalf("write purpose error = %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("multipart close error = %v", err)
	}
	return &body, writer.FormDataContentType()
}

type recordingRelayFilesMappingStore struct {
	records []handler.FileMappingRecord
}

func (s *recordingRelayFilesMappingStore) SaveFileMapping(ctx context.Context, record handler.FileMappingRecord) error {
	s.records = append(s.records, record)
	return nil
}

type recordingConversationAffinityStore struct {
	channelID string
}

func (s *recordingConversationAffinityStore) SaveConversationAffinity(_ context.Context, _, channelID string) error {
	s.channelID = channelID
	return nil
}

func (s *recordingConversationAffinityStore) GetConversationAffinity(_ context.Context, _ string) (string, error) {
	return s.channelID, nil
}

var _ ConversationAffinityStore = (*recordingConversationAffinityStore)(nil)

type recordingUsageLogger struct {
	records []RelayUsageLogRecord
}

func (l *recordingUsageLogger) RecordRelayUsage(_ context.Context, record RelayUsageLogRecord) error {
	l.records = append(l.records, record)
	return nil
}

var _ UsageLogger = (*recordingUsageLogger)(nil)

type recordingAPITokenQuotaManager struct {
	preauthorizedTokenID string
	preauthorizedAmount  float64
	settledTokenID       string
	settledAmount        float64
	refundedTokenID      string
	refundedAmount       float64
	settleCalls          int
	refundCalls          int
}

func (m *recordingAPITokenQuotaManager) PreAuthorizeRelayAPITokenQuota(_ context.Context, tokenID string, amount float64) error {
	m.preauthorizedTokenID = tokenID
	m.preauthorizedAmount = amount
	return nil
}

func (m *recordingAPITokenQuotaManager) SettleRelayAPITokenQuota(_ context.Context, tokenID string, preauthorizedAmount, actualAmount float64) error {
	m.settledTokenID = tokenID
	m.preauthorizedAmount = preauthorizedAmount
	m.settledAmount = actualAmount
	m.settleCalls++
	return nil
}

func (m *recordingAPITokenQuotaManager) RefundRelayAPITokenQuota(_ context.Context, tokenID string, amount float64) error {
	m.refundedTokenID = tokenID
	m.refundedAmount = amount
	m.refundCalls++
	return nil
}

var _ APITokenQuotaManager = (*recordingAPITokenQuotaManager)(nil)

type stubQuotaManager struct {
	preconsumeCalls int
	settleCalls     int
	refundCalls     int
	settleErr       error
}

func (m *stubQuotaManager) PreConsume(_ context.Context, userID, organizationID string, amount float64, idempotencyKey string, channelID, model, apiType string) (*quota.BillingSession, error) {
	m.preconsumeCalls++
	return &quota.BillingSession{
		ID:               "bill_test",
		UserID:           userID,
		OrganizationID:   organizationID,
		ChannelID:        channelID,
		Model:            model,
		APIType:          apiType,
		IdempotencyKey:   idempotencyKey,
		PreAuthorizedAmt: amount,
		Status:           "preauthorized",
		CreatedAt:        time.Now().UTC(),
	}, nil
}

func (m *stubQuotaManager) Settle(_ context.Context, _ string, _ float64) error {
	m.settleCalls++
	return m.settleErr
}

func (m *stubQuotaManager) Refund(_ context.Context, _ string) error {
	m.refundCalls++
	return nil
}

var _ QuotaManager = (*stubQuotaManager)(nil)

func TestNewRelayProductionChatReturnsQuotaCodeWhenAPITokenPreAuthorizationFails(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"unexpected_upstream_call"}}`))
	}))
	t.Cleanup(upstream.Close)

	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{
		ID:          "ch_openai_quota",
		Name:        "OpenAI quota test",
		Provider:    "openai",
		BaseURL:     upstream.URL,
		APIKey:      "sk-upstream",
		Models:      []string{"gpt-4o-mini"},
		CBThreshold: 5,
		Enabled:     true,
	}, 100)
	authenticator := &recordingRelayAuthenticator{
		identity: types.RelayAPITokenIdentity{
			TokenID:        "tok_quota",
			UserID:         "user_quota",
			OrganizationID: "org_quota",
			UserGroup:      "default",
		},
	}
	relayInstance, err := NewRelay(&Config{
		Pool:                  pool,
		Production:            true,
		APITokenAuthenticator: authenticator,
	})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}
	tokenQuota := &preauthFailingAPITokenQuotaManager{err: types.ErrRelayAPITokenQuotaExceeded}
	usageLogger := &recordingUsageLogger{}
	relayInstance.Router().SetAPITokenQuotaManager(tokenQuota)
	relayInstance.Router().SetUsageLogger(usageLogger)

	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"ping quota"}]}`))
	request.Header.Set("Authorization", "Bearer obv_quota")
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	relayInstance.Engine().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusPaymentRequired, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "relay_api_token_quota_exceeded") {
		t.Fatalf("expected quota error code, got %s", recorder.Body.String())
	}
	if upstreamCalls != 0 {
		t.Fatalf("quota failure should not call upstream, got %d calls", upstreamCalls)
	}
	if tokenQuota.tokenID != "tok_quota" || tokenQuota.amount <= 0 {
		t.Fatalf("expected API token quota preauthorization for tok_quota, got token=%q amount=%f", tokenQuota.tokenID, tokenQuota.amount)
	}
	if tokenQuota.settleCalls != 0 || tokenQuota.refundCalls != 0 {
		t.Fatalf("unexpected token quota settlement/refund calls: settle=%d refund=%d", tokenQuota.settleCalls, tokenQuota.refundCalls)
	}
	if len(usageLogger.records) != 1 {
		t.Fatalf("expected 1 usage log record, got %d", len(usageLogger.records))
	}
	record := usageLogger.records[0]
	if record.UserID != "user_quota" || record.OrganizationID != "org_quota" || record.APITokenID != "tok_quota" {
		t.Fatalf("usage identity mismatch: %+v", record)
	}
	if record.Model != "gpt-4o-mini" || record.ChannelID != "ch_openai_quota" || record.Provider != "openai" {
		t.Fatalf("usage route mismatch: %+v", record)
	}
	if record.Status != RelayUsageStatusError || record.StatusCode != http.StatusPaymentRequired || record.ErrorCode != "relay_api_token_quota_exceeded" {
		t.Fatalf("usage error mismatch: %+v", record)
	}
	if record.RequestID == "" || recorder.Header().Get(types.HeaderRequestID) != record.RequestID {
		t.Fatalf("usage request id %q did not match response header %q", record.RequestID, recorder.Header().Get(types.HeaderRequestID))
	}
}

func TestNewRelayProductionChatSettlesAPITokenQuotaAndRecordsUsageOnSuccess(t *testing.T) {
	var upstreamPath string
	var upstreamAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		upstreamAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-success",
			"choices":[{"message":{"role":"assistant","content":"ok"}}],
			"usage":{"prompt_tokens":1000,"completion_tokens":250,"total_tokens":1250}
		}`))
	}))
	t.Cleanup(upstream.Close)

	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{
		ID:                 "ch_openai_success",
		Name:               "OpenAI success test",
		Provider:           "openai",
		BaseURL:            upstream.URL,
		APIKey:             "sk-upstream-success",
		Models:             []string{"gpt-4o-mini"},
		CBThreshold:        5,
		EstimatedCostPer1K: 0.0003,
		CostMultiplier:     2,
		Enabled:            true,
	}, 100)
	authenticator := &recordingRelayAuthenticator{
		identity: types.RelayAPITokenIdentity{
			TokenID:        "tok_success",
			UserID:         "user_success",
			OrganizationID: "org_success",
			UserGroup:      "default",
		},
	}
	relayInstance, err := NewRelay(&Config{
		Pool:                  pool,
		Production:            true,
		APITokenAuthenticator: authenticator,
	})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}
	tokenQuota := &recordingAPITokenQuotaManager{}
	usageLogger := &recordingUsageLogger{}
	relayInstance.Router().SetAPITokenQuotaManager(tokenQuota)
	relayInstance.Router().SetUsageLogger(usageLogger)

	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"ping success"}]}`))
	request.Header.Set("Authorization", "Bearer obv_success")
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	relayInstance.Engine().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if upstreamPath != "/v1/chat/completions" || upstreamAuth != "Bearer sk-upstream-success" {
		t.Fatalf("unexpected upstream request path=%q auth=%q", upstreamPath, upstreamAuth)
	}
	if !strings.Contains(recorder.Body.String(), "chatcmpl-success") {
		t.Fatalf("expected upstream response body, got %s", recorder.Body.String())
	}
	if tokenQuota.preauthorizedTokenID != "tok_success" || tokenQuota.preauthorizedAmount <= 0 {
		t.Fatalf("expected API token quota preauthorization, got %+v", tokenQuota)
	}
	if tokenQuota.settledTokenID != "tok_success" || tokenQuota.settledAmount <= 0 {
		t.Fatalf("expected API token quota settlement, got %+v", tokenQuota)
	}
	if tokenQuota.refundedTokenID != "" {
		t.Fatalf("success path should not refund API token quota, got %+v", tokenQuota)
	}
	if len(usageLogger.records) != 1 {
		t.Fatalf("expected 1 usage log record, got %d", len(usageLogger.records))
	}
	record := usageLogger.records[0]
	if record.UserID != "user_success" || record.OrganizationID != "org_success" || record.APITokenID != "tok_success" {
		t.Fatalf("usage identity mismatch: %+v", record)
	}
	if record.Model != "gpt-4o-mini" || record.ChannelID != "ch_openai_success" || record.Provider != "openai" {
		t.Fatalf("usage route mismatch: %+v", record)
	}
	if record.Status != RelayUsageStatusSuccess || record.StatusCode != http.StatusOK {
		t.Fatalf("usage status mismatch: %+v", record)
	}
	if record.PromptTokens != 1000 || record.CompletionTokens != 250 || record.TotalTokens != 1250 {
		t.Fatalf("usage tokens should come from provider response, got %+v", record)
	}
	if record.Cost <= 0 || math.Abs(record.Cost-tokenQuota.settledAmount) > 0.000001 {
		t.Fatalf("usage cost should match token quota settlement, usage=%+v tokenQuota=%+v", record, tokenQuota)
	}
	if record.ChannelCost <= 0 {
		t.Fatalf("expected channel cost to be recorded, got %+v", record)
	}
	if record.RequestID == "" || recorder.Header().Get(types.HeaderRequestID) != record.RequestID {
		t.Fatalf("usage request id %q did not match response header %q", record.RequestID, recorder.Header().Get(types.HeaderRequestID))
	}
}

func TestNewRelayProductionChatRecordsTrustedFeatureTypeOnUsage(t *testing.T) {
	t.Setenv("OBLIVIOUS_INTERNAL_AUTH_TOKEN", types.SharedInternalToken)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-workflow",
			"choices":[{"message":{"role":"assistant","content":"ok"}}],
			"usage":{"prompt_tokens":40,"completion_tokens":10,"total_tokens":50}
		}`))
	}))
	t.Cleanup(upstream.Close)

	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{
		ID:          "ch_openai_workflow",
		Name:        "OpenAI workflow attribution test",
		Provider:    "openai",
		BaseURL:     upstream.URL,
		APIKey:      "sk-upstream-workflow",
		Models:      []string{"gpt-4o-mini"},
		CBThreshold: 5,
		Enabled:     true,
	}, 100)
	relayInstance, err := NewRelay(&Config{
		Pool:       pool,
		Production: true,
	})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}
	usageLogger := &recordingUsageLogger{}
	relayInstance.Router().SetUsageLogger(usageLogger)

	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"summarize workflow"}]}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(types.HeaderInternalAuth, types.SharedInternalToken)
	request.Header.Set(types.HeaderInternalUserID, "user_workflow")
	request.Header.Set(types.HeaderInternalOrganization, "org_workflow")
	request.Header.Set(types.HeaderInternalFeatureType, "workflow")
	recorder := httptest.NewRecorder()

	relayInstance.Engine().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if len(usageLogger.records) != 1 {
		t.Fatalf("expected 1 usage log record, got %d", len(usageLogger.records))
	}
	record := usageLogger.records[0]
	if record.UserID != "user_workflow" || record.OrganizationID != "org_workflow" {
		t.Fatalf("usage identity mismatch: %+v", record)
	}
	if record.APIType != "chat" || record.FeatureType != "workflow" {
		t.Fatalf("usage feature attribution mismatch: %+v", record)
	}
}

func TestNewRelayProductionChatStreamsProviderSSEEndToEnd(t *testing.T) {
	var upstreamPath string
	var upstreamAuth string
	var upstreamBody struct {
		Model   string `json:"model"`
		Stream  bool   `json:"stream"`
		Options struct {
			IncludeUsage bool `json:"include_usage"`
		} `json:"stream_options"`
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		upstreamAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}],\"usage\":{\"prompt_tokens\":7,\"completion_tokens\":3,\"total_tokens\":10}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	t.Cleanup(upstream.Close)

	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{
		ID:                 "ch_openai_stream",
		Name:               "OpenAI stream test",
		Provider:           "openai",
		BaseURL:            upstream.URL,
		APIKey:             "sk-upstream-stream",
		Models:             []string{"gpt-4o-mini"},
		CBThreshold:        5,
		EstimatedCostPer1K: 0.0003,
		Enabled:            true,
	}, 100)
	authenticator := &recordingRelayAuthenticator{
		identity: types.RelayAPITokenIdentity{
			TokenID:        "tok_stream",
			UserID:         "user_stream",
			OrganizationID: "org_stream",
			UserGroup:      "default",
		},
	}
	relayInstance, err := NewRelay(&Config{
		Pool:                  pool,
		Production:            true,
		APITokenAuthenticator: authenticator,
	})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}
	tokenQuota := &recordingAPITokenQuotaManager{}
	usageLogger := &recordingUsageLogger{}
	relayInstance.Router().SetAPITokenQuotaManager(tokenQuota)
	relayInstance.Router().SetUsageLogger(usageLogger)

	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o-mini","stream":true,"messages":[{"role":"user","content":"stream success"}]}`))
	request.Header.Set("Authorization", "Bearer obv_stream")
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	relayInstance.Engine().ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", contentType)
	}
	if upstreamPath != "/v1/chat/completions" || upstreamAuth != "Bearer sk-upstream-stream" {
		t.Fatalf("unexpected upstream request path=%q auth=%q", upstreamPath, upstreamAuth)
	}
	if upstreamBody.Model != "gpt-4o-mini" || !upstreamBody.Stream || !upstreamBody.Options.IncludeUsage {
		t.Fatalf("upstream streaming request not preserved with usage option: %+v", upstreamBody)
	}
	if !strings.Contains(recorder.Body.String(), `data: {"choices":[{"delta":{"content":"hel"}}]}`) || !strings.Contains(recorder.Body.String(), "data: [DONE]") {
		t.Fatalf("expected provider SSE body, got %s", recorder.Body.String())
	}
	if len(usageLogger.records) != 1 {
		t.Fatalf("expected 1 usage log record, got %d", len(usageLogger.records))
	}
	record := usageLogger.records[0]
	if record.Provider != "openai" || record.ChannelID != "ch_openai_stream" || record.Status != RelayUsageStatusSuccess || record.StatusCode != http.StatusOK {
		t.Fatalf("usage route/status mismatch: %+v", record)
	}
	if record.PromptTokens != 7 || record.CompletionTokens != 3 || record.TotalTokens != 10 {
		t.Fatalf("streaming usage tokens should come from SSE usage chunk, got %+v", record)
	}
	if tokenQuota.settledTokenID != "tok_stream" || tokenQuota.settledAmount <= 0 {
		t.Fatalf("expected streaming request to settle API token quota, got %+v", tokenQuota)
	}
}

func TestNewRelayProductionChatUsesSharedSemanticCacheOnSecondRequest(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-cache-source",
			"choices":[{"message":{"role":"assistant","content":"cached after first call"}}],
			"usage":{"prompt_tokens":1000,"completion_tokens":250,"total_tokens":1250}
		}`))
	}))
	t.Cleanup(upstream.Close)

	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{
		ID:          "ch_openai_cache",
		Name:        "OpenAI cache test",
		Provider:    "openai",
		BaseURL:     upstream.URL,
		APIKey:      "sk-upstream-cache",
		Models:      []string{"gpt-4o-mini"},
		CBThreshold: 5,
		Enabled:     true,
	}, 100)
	authenticator := &recordingRelayAuthenticator{
		identity: types.RelayAPITokenIdentity{
			TokenID:        "tok_cache",
			UserID:         "user_cache",
			OrganizationID: "org_cache",
			UserGroup:      "default",
		},
	}
	relayInstance, err := NewRelay(&Config{
		Pool:                  pool,
		Production:            true,
		APITokenAuthenticator: authenticator,
	})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}
	tokenQuota := &recordingAPITokenQuotaManager{}
	usageLogger := &recordingUsageLogger{}
	relayInstance.Router().SetAPITokenQuotaManager(tokenQuota)
	relayInstance.Router().SetUsageLogger(usageLogger)

	body := `{"model":"gpt-4o-mini","messages":[{"role":"system","content":"be stable"},{"role":"user","content":"ping cache"}],"max_tokens":32}`
	first := httptest.NewRecorder()
	firstReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	firstReq.Header.Set("Authorization", "Bearer obv_cache")
	firstReq.Header.Set("Content-Type", "application/json")
	relayInstance.Engine().ServeHTTP(first, firstReq)
	if first.Code != http.StatusOK {
		t.Fatalf("first status = %d, want 200; body=%s", first.Code, first.Body.String())
	}

	second := httptest.NewRecorder()
	secondReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	secondReq.Header.Set("Authorization", "Bearer obv_cache")
	secondReq.Header.Set("Content-Type", "application/json")
	relayInstance.Engine().ServeHTTP(second, secondReq)
	if second.Code != http.StatusOK {
		t.Fatalf("second status = %d, want 200; body=%s", second.Code, second.Body.String())
	}
	if upstreamCalls != 1 {
		t.Fatalf("expected second identical request to use semantic cache, upstream calls=%d", upstreamCalls)
	}
	if !strings.Contains(second.Body.String(), "chatcmpl-cache-source") {
		t.Fatalf("expected cached upstream body on second response, got %s", second.Body.String())
	}
	if len(usageLogger.records) != 2 {
		t.Fatalf("expected first provider usage and second cache-hit usage, got %d records", len(usageLogger.records))
	}
	if usageLogger.records[0].Provider != "openai" || usageLogger.records[0].Status != RelayUsageStatusSuccess {
		t.Fatalf("first record should be provider success, got %+v", usageLogger.records[0])
	}
	cacheRecord := usageLogger.records[1]
	if cacheRecord.Provider != "semantic_cache" || cacheRecord.ChannelID != "" || cacheRecord.Cost != 0 || cacheRecord.ChannelCost != 0 {
		t.Fatalf("second record should be zero-cost semantic cache hit, got %+v", cacheRecord)
	}
	if cacheRecord.PromptTokens != 0 || cacheRecord.CompletionTokens != 0 || cacheRecord.TotalTokens != 0 {
		t.Fatalf("cache hit usage record should not reuse embedded provider usage, got %+v", cacheRecord)
	}
}

func TestNewRelayProductionChatReturnsCodeWhenAPITokenSettlementFails(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-settlement-fails",
			"choices":[{"message":{"role":"assistant","content":"ok"}}],
			"usage":{"prompt_tokens":1000,"completion_tokens":250,"total_tokens":1250}
		}`))
	}))
	t.Cleanup(upstream.Close)

	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{
		ID:          "ch_openai_settlement",
		Name:        "OpenAI settlement test",
		Provider:    "openai",
		BaseURL:     upstream.URL,
		APIKey:      "sk-upstream-settlement",
		Models:      []string{"gpt-4o-mini"},
		CBThreshold: 5,
		Enabled:     true,
	}, 100)
	authenticator := &recordingRelayAuthenticator{
		identity: types.RelayAPITokenIdentity{
			TokenID:        "tok_settlement",
			UserID:         "user_settlement",
			OrganizationID: "org_settlement",
		},
	}
	relayInstance, err := NewRelay(&Config{
		Pool:                  pool,
		Production:            true,
		APITokenAuthenticator: authenticator,
	})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}
	tokenQuota := &settleFailingAPITokenQuotaManager{err: errors.New("token quota store unavailable")}
	usageLogger := &recordingUsageLogger{}
	relayInstance.Router().SetAPITokenQuotaManager(tokenQuota)
	relayInstance.Router().SetUsageLogger(usageLogger)

	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"ping settlement"}]}`))
	request.Header.Set("Authorization", "Bearer obv_settlement")
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	relayInstance.Engine().ServeHTTP(recorder, request)

	if tokenQuota.preauthorizedTokenID != "tok_settlement" || tokenQuota.preauthorizedAmount <= 0 {
		t.Fatalf("expected token preauthorization, got %+v", tokenQuota)
	}
	if tokenQuota.settledTokenID != "tok_settlement" || tokenQuota.settledAmount <= 0 {
		t.Fatalf("expected token settlement attempt, got %+v", tokenQuota)
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; tokenQuota=%+v body=%s", recorder.Code, http.StatusInternalServerError, tokenQuota, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "api_token_quota_settlement_failed") {
		t.Fatalf("expected settlement error code, got %s", recorder.Body.String())
	}
	if upstreamCalls != 1 {
		t.Fatalf("expected one successful upstream call before settlement failure, got %d", upstreamCalls)
	}
	if len(usageLogger.records) != 1 {
		t.Fatalf("expected 1 usage log record, got %d", len(usageLogger.records))
	}
	record := usageLogger.records[0]
	if record.Status != RelayUsageStatusError || record.StatusCode != http.StatusInternalServerError || record.ErrorCode != "api_token_quota_settlement_failed" {
		t.Fatalf("usage settlement error mismatch: %+v", record)
	}
	if record.UserID != "user_settlement" || record.OrganizationID != "org_settlement" || record.APITokenID != "tok_settlement" {
		t.Fatalf("usage identity mismatch: %+v", record)
	}
}

func TestNewRelayProductionChatReturnsCodeWhenBillingSettlementFails(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"chatcmpl-billing-settlement-fails",
			"choices":[{"message":{"role":"assistant","content":"ok"}}],
			"usage":{"prompt_tokens":1000,"completion_tokens":250,"total_tokens":1250}
		}`))
	}))
	t.Cleanup(upstream.Close)

	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{
		ID:          "ch_openai_billing_settlement",
		Name:        "OpenAI billing settlement test",
		Provider:    "openai",
		BaseURL:     upstream.URL,
		APIKey:      "sk-upstream-billing-settlement",
		Models:      []string{"gpt-4o-mini"},
		CBThreshold: 5,
		Enabled:     true,
	}, 100)
	authenticator := &recordingRelayAuthenticator{
		identity: types.RelayAPITokenIdentity{
			TokenID:        "tok_billing_settlement",
			UserID:         "user_billing_settlement",
			OrganizationID: "org_billing_settlement",
		},
	}
	relayInstance, err := NewRelay(&Config{
		Pool:                  pool,
		Production:            true,
		APITokenAuthenticator: authenticator,
	})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}
	billingQuota := &stubQuotaManager{settleErr: errors.New("billing quota store unavailable")}
	tokenQuota := &recordingAPITokenQuotaManager{}
	usageLogger := &recordingUsageLogger{}
	relayInstance.Router().SetQuotaManager(billingQuota)
	relayInstance.Router().SetAPITokenQuotaManager(tokenQuota)
	relayInstance.Router().SetUsageLogger(usageLogger)

	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"ping billing settlement"}]}`))
	request.Header.Set("Authorization", "Bearer obv_billing_settlement")
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	relayInstance.Engine().ServeHTTP(recorder, request)

	if billingQuota.preconsumeCalls != 1 || billingQuota.settleCalls != 1 {
		t.Fatalf("expected billing preconsume and settle attempt, got %+v", billingQuota)
	}
	if tokenQuota.preauthorizedTokenID != "tok_billing_settlement" || tokenQuota.preauthorizedAmount <= 0 {
		t.Fatalf("expected API token preauthorization, got %+v", tokenQuota)
	}
	if tokenQuota.refundedTokenID != "tok_billing_settlement" || tokenQuota.refundedAmount != tokenQuota.preauthorizedAmount {
		t.Fatalf("expected API token quota refund after billing settlement failure, got %+v", tokenQuota)
	}
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusInternalServerError, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "billing_settlement_failed") {
		t.Fatalf("expected billing settlement error code, got %s", recorder.Body.String())
	}
	if upstreamCalls != 1 {
		t.Fatalf("expected one upstream call before billing settlement failure, got %d", upstreamCalls)
	}
	if len(usageLogger.records) != 1 {
		t.Fatalf("expected 1 usage log record, got %d", len(usageLogger.records))
	}
	record := usageLogger.records[0]
	if record.Status != RelayUsageStatusError || record.StatusCode != http.StatusInternalServerError || record.ErrorCode != "billing_settlement_failed" {
		t.Fatalf("usage billing settlement error mismatch: %+v", record)
	}
	if record.UserID != "user_billing_settlement" || record.OrganizationID != "org_billing_settlement" || record.APITokenID != "tok_billing_settlement" {
		t.Fatalf("usage identity mismatch: %+v", record)
	}
}

func TestNewRelayProductionChatReturnsCodeWhenBillingPreAuthorizationFails(t *testing.T) {
	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"code":"unexpected_upstream_call"}}`))
	}))
	t.Cleanup(upstream.Close)

	pool := NewChannelPool()
	pool.AddChannel(&types.Channel{
		ID:          "ch_openai_billing_preauth",
		Name:        "OpenAI billing preauth test",
		Provider:    "openai",
		BaseURL:     upstream.URL,
		APIKey:      "sk-upstream-billing-preauth",
		Models:      []string{"gpt-4o-mini"},
		CBThreshold: 5,
		Enabled:     true,
	}, 100)
	authenticator := &recordingRelayAuthenticator{
		identity: types.RelayAPITokenIdentity{
			TokenID:        "tok_billing_preauth",
			UserID:         "user_billing_preauth",
			OrganizationID: "org_billing_preauth",
		},
	}
	relayInstance, err := NewRelay(&Config{
		Pool:                  pool,
		Production:            true,
		APITokenAuthenticator: authenticator,
	})
	if err != nil {
		t.Fatalf("new relay: %v", err)
	}
	billingQuota := &preconsumeFailingQuotaManager{err: errors.New("insufficient balance")}
	tokenQuota := &recordingAPITokenQuotaManager{}
	usageLogger := &recordingUsageLogger{}
	relayInstance.Router().SetQuotaManager(billingQuota)
	relayInstance.Router().SetAPITokenQuotaManager(tokenQuota)
	relayInstance.Router().SetUsageLogger(usageLogger)

	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-4o-mini","messages":[{"role":"user","content":"ping billing preauth"}]}`))
	request.Header.Set("Authorization", "Bearer obv_billing_preauth")
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	relayInstance.Engine().ServeHTTP(recorder, request)

	if billingQuota.calls != 1 {
		t.Fatalf("expected one billing preauthorization attempt, got %d", billingQuota.calls)
	}
	if tokenQuota.preauthorizedTokenID != "" || tokenQuota.settledTokenID != "" || tokenQuota.refundedTokenID != "" {
		t.Fatalf("API token quota should not be touched when billing preauth fails first, got %+v", tokenQuota)
	}
	if recorder.Code != http.StatusPaymentRequired {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusPaymentRequired, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "billing_pre_authorization_failed") {
		t.Fatalf("expected billing preauthorization error code, got %s", recorder.Body.String())
	}
	if upstreamCalls != 0 {
		t.Fatalf("billing preauthorization failure should not call upstream, got %d calls", upstreamCalls)
	}
	if len(usageLogger.records) != 1 {
		t.Fatalf("expected 1 usage log record, got %d", len(usageLogger.records))
	}
	record := usageLogger.records[0]
	if record.Status != RelayUsageStatusError || record.StatusCode != http.StatusPaymentRequired || record.ErrorCode != "billing_pre_authorization_failed" {
		t.Fatalf("usage billing preauth error mismatch: %+v", record)
	}
	if record.UserID != "user_billing_preauth" || record.OrganizationID != "org_billing_preauth" || record.APITokenID != "tok_billing_preauth" {
		t.Fatalf("usage identity mismatch: %+v", record)
	}
}

type recordingRelayAuthenticator struct {
	identity types.RelayAPITokenIdentity
	rawToken string
	model    string
	apiType  types.APIType
}

func (a *recordingRelayAuthenticator) AuthenticateRelayAPIToken(_ context.Context, rawToken, model string, apiType types.APIType) (types.RelayAPITokenIdentity, error) {
	a.rawToken = rawToken
	a.model = model
	a.apiType = apiType
	return a.identity, nil
}

type recordingRelayRateLimiter struct {
	mu             sync.Mutex
	allowErr       error
	beginErr       error
	checkDecisions map[string]ratelimit.Decision
	allowCalls     int
	beginCalls     int
	endCalls       int
	lastAllowKey   ratelimit.Key
	lastBeginKey   ratelimit.Key
	lastEndKey     ratelimit.Key
}

func (l *recordingRelayRateLimiter) Allow(_ context.Context, key ratelimit.Key, _ ratelimit.Limits, _ ratelimit.Usage) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.allowCalls++
	l.lastAllowKey = key
	return l.allowErr
}

func (l *recordingRelayRateLimiter) Begin(_ context.Context, key ratelimit.Key, _ ratelimit.Limits) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.beginCalls++
	l.lastBeginKey = key
	return l.beginErr
}

func (l *recordingRelayRateLimiter) End(_ context.Context, key ratelimit.Key) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.endCalls++
	l.lastEndKey = key
	return nil
}

func (l *recordingRelayRateLimiter) Check(_ context.Context, key ratelimit.Key, _ ratelimit.Limits, _ ratelimit.Usage) ratelimit.Decision {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.checkDecisions != nil {
		if decision, ok := l.checkDecisions[key.ChannelID]; ok {
			return decision
		}
	}
	return ratelimit.Decision{Allowed: true}
}

var _ ratelimit.RateLimiter = (*recordingRelayRateLimiter)(nil)

type recordingSemanticCacheStore struct {
	puts int
}

func (s *recordingSemanticCacheStore) Get(context.Context, relaycache.SemanticCacheKey) (*relaycache.SemanticCacheEntry, error) {
	return nil, nil
}

func (s *recordingSemanticCacheStore) Put(context.Context, relaycache.SemanticCacheEntry) error {
	s.puts++
	return nil
}

func (s *recordingSemanticCacheStore) IncrementHit(context.Context, relaycache.SemanticCacheKey) error {
	return nil
}

var _ relaycache.SemanticCacheStore = (*recordingSemanticCacheStore)(nil)

type preauthFailingAPITokenQuotaManager struct {
	err         error
	tokenID     string
	amount      float64
	settleCalls int
	refundCalls int
}

func (m *preauthFailingAPITokenQuotaManager) PreAuthorizeRelayAPITokenQuota(_ context.Context, tokenID string, amount float64) error {
	m.tokenID = tokenID
	m.amount = amount
	return m.err
}

func (m *preauthFailingAPITokenQuotaManager) SettleRelayAPITokenQuota(_ context.Context, _ string, _, _ float64) error {
	m.settleCalls++
	return nil
}

func (m *preauthFailingAPITokenQuotaManager) RefundRelayAPITokenQuota(_ context.Context, _ string, _ float64) error {
	m.refundCalls++
	return nil
}

var _ APITokenQuotaManager = (*preauthFailingAPITokenQuotaManager)(nil)

type settleFailingAPITokenQuotaManager struct {
	recordingAPITokenQuotaManager
	err error
}

func (m *settleFailingAPITokenQuotaManager) SettleRelayAPITokenQuota(ctx context.Context, tokenID string, preauthorizedAmount, actualAmount float64) error {
	_ = m.recordingAPITokenQuotaManager.SettleRelayAPITokenQuota(ctx, tokenID, preauthorizedAmount, actualAmount)
	return m.err
}

var _ APITokenQuotaManager = (*settleFailingAPITokenQuotaManager)(nil)

type preconsumeFailingQuotaManager struct {
	err   error
	calls int
}

func (m *preconsumeFailingQuotaManager) PreConsume(ctx context.Context, userID, organizationID string, amount float64, idempotencyKey string, channelID, model, apiType string) (*quota.BillingSession, error) {
	m.calls++
	return nil, m.err
}

func (m *preconsumeFailingQuotaManager) Settle(ctx context.Context, sessionID string, actualAmount float64) error {
	return nil
}

func (m *preconsumeFailingQuotaManager) Refund(ctx context.Context, sessionID string) error {
	return nil
}

var _ QuotaManager = (*preconsumeFailingQuotaManager)(nil)
