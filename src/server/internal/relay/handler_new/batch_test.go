package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
)

func TestBatchHandlerFailsClosedWhenCommercialLifecycleDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"batch_should_not_run"}`))
	}))
	t.Cleanup(upstream.Close)

	router := &batchSubmitBillingRouter{}
	previous := GetRouter()
	SetRouter(router)
	t.Cleanup(func() {
		SetRouter(previous)
	})

	registrar := &recordingBatchPollingRegistrar{}
	handler := NewBatchHandler(nil, channel.NewOpenAIAdapter(upstream.URL, "sk-test")).
		WithPollingRegistrar(registrar)
	engine := gin.New()
	engine.POST("/v1/batch", func(c *gin.Context) {
		_ = handler.Handle(c)
	})
	engine.GET("/v1/batches", func(c *gin.Context) {
		_ = handler.Handle(c)
	})
	engine.GET("/v1/batches/:id", func(c *gin.Context) {
		_ = handler.Handle(c)
	})

	requests := []*http.Request{
		httptest.NewRequest(http.MethodPost, "/v1/batch", strings.NewReader(`{"model":"gpt-4.1-mini","input_file_id":"file_123"}`)),
		httptest.NewRequest(http.MethodGet, "/v1/batches", nil),
		httptest.NewRequest(http.MethodGet, "/v1/batches/batch_123", nil),
	}
	for _, req := range requests {
		rec := httptest.NewRecorder()
		engine.ServeHTTP(rec, req)
		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("%s %s status = %d, want %d; body=%s", req.Method, req.URL.Path, rec.Code, http.StatusNotImplemented, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), "unsupported_api") {
			t.Fatalf("expected unsupported_api response for %s, got %s", req.URL.Path, rec.Body.String())
		}
	}
	if upstreamCalls != 0 {
		t.Fatalf("upstream batch calls = %d, want 0 while lifecycle is disabled", upstreamCalls)
	}
	if router.routeWithBillingCalls != 0 {
		t.Fatalf("RouteWithBilling calls = %d, want 0 while lifecycle is disabled", router.routeWithBillingCalls)
	}
	if registrar.task.BatchID != "" {
		t.Fatalf("unexpected polling registration while lifecycle disabled: %+v", registrar.task)
	}
}

func TestBatchSubmitRoutesThroughBillingRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/batch" {
			t.Fatalf("upstream path = %q, want /v1/batch", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"batch_billed","object":"batch","status":"validating"}`))
	}))
	t.Cleanup(upstream.Close)

	router := &batchSubmitBillingRouter{
		channel: &types.RouteChannel{
			Channel: &types.Channel{
				ID:       "ch_batch",
				Provider: "openai",
				BaseURL:  upstream.URL,
				APIKey:   "sk-batch",
				Enabled:  true,
			},
			ChannelID: "ch_batch",
			Enabled:   true,
			Healthy:   true,
		},
	}
	previous := GetRouter()
	SetRouter(router)
	t.Cleanup(func() {
		SetRouter(previous)
	})

	handler := NewBatchHandler(nil, channel.NewOpenAIAdapter("https://direct.example.invalid", "sk-direct")).
		WithCommercialLifecycleEnabled(true)
	engine := gin.New()
	engine.POST("/v1/batch", func(c *gin.Context) {
		_ = handler.Handle(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/batch", strings.NewReader(`{"model":"gpt-4.1-mini","input_file_id":"file_123","endpoint":"/v1/chat/completions"}`))
	req.Header.Set(types.HeaderRequestID, "req_batch_billed")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if router.routeWithBillingCalls != 1 {
		t.Fatalf("RouteWithBilling calls = %d, want 1", router.routeWithBillingCalls)
	}
	if router.apiType != types.APITypeBatch {
		t.Fatalf("apiType = %s, want %s", router.apiType.String(), types.APITypeBatch.String())
	}
	if router.model != "gpt-4.1-mini" {
		t.Fatalf("model = %q, want gpt-4.1-mini", router.model)
	}
	if router.idempotencyKey != "req_batch_billed" {
		t.Fatalf("idempotency key = %q, want req_batch_billed", router.idempotencyKey)
	}
	if router.usage == nil {
		t.Fatal("expected batch submit to pass non-nil usage into RouteWithBilling")
	}
	if !strings.Contains(rec.Body.String(), `"id":"batch_billed"`) {
		t.Fatalf("expected billed upstream body, got %s", rec.Body.String())
	}
}

func TestBatchSubmitRegistersPollingTaskFromUpstreamBatchID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/batch" {
			t.Fatalf("upstream path = %q, want /v1/batch", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"batch_new_123","object":"batch","status":"validating"}`))
	}))
	t.Cleanup(upstream.Close)

	router := &batchSubmitBillingRouter{
		billingSessionID:         "bill_batch_new",
		preauthorizedAmount:      1.25,
		tokenPreauthorizedAmount: 1.5,
		channel: &types.RouteChannel{
			Channel: &types.Channel{
				ID:       "ch_batch_new_polling",
				Provider: "openai",
				BaseURL:  upstream.URL,
				APIKey:   "sk-batch",
				Enabled:  true,
			},
			ChannelID: "ch_batch_new_polling",
			Enabled:   true,
			Healthy:   true,
		},
	}
	previous := GetRouter()
	SetRouter(router)
	t.Cleanup(func() {
		SetRouter(previous)
	})

	registrar := &recordingBatchPollingRegistrar{}
	handler := NewBatchHandler(nil, channel.NewOpenAIAdapter("https://direct.example.invalid", "sk-direct")).
		WithCommercialLifecycleEnabled(true).
		WithPollingRegistrar(registrar)
	engine := gin.New()
	engine.POST("/v1/batch", func(c *gin.Context) {
		_ = handler.Handle(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/batch", strings.NewReader(`{"model":"gpt-4.1-mini","input_file_id":"file_123","endpoint":"/v1/chat/completions"}`))
	req.Header.Set(types.HeaderRequestID, "req_batch_new_polling")
	ctx := types.WithTrustedUserID(req.Context(), "user_batch")
	ctx = types.WithTrustedOrganizationID(ctx, "org_batch")
	ctx = types.WithTrustedAPITokenID(ctx, "tok_batch")
	ctx = types.WithTrustedFeatureType(ctx, "workflow")
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if registrar.task.BatchID != "batch_new_123" {
		t.Fatalf("registered batch id = %q, want batch_new_123", registrar.task.BatchID)
	}
	if registrar.task.RequestID != "req_batch_new_polling" {
		t.Fatalf("registered request id = %q, want req_batch_new_polling", registrar.task.RequestID)
	}
	if registrar.task.UserID != "user_batch" ||
		registrar.task.OrganizationID != "org_batch" ||
		registrar.task.APITokenID != "tok_batch" ||
		registrar.task.FeatureType != "workflow" {
		t.Fatalf("registered identity context = user:%q org:%q token:%q feature:%q", registrar.task.UserID, registrar.task.OrganizationID, registrar.task.APITokenID, registrar.task.FeatureType)
	}
	if registrar.task.Model != "gpt-4.1-mini" || registrar.task.APIType != types.APITypeBatch {
		t.Fatalf("registered model/api type = %q/%s", registrar.task.Model, registrar.task.APIType.String())
	}
	if registrar.task.BillingSessionID != "bill_batch_new" ||
		registrar.task.PreauthorizedAmount != 1.25 ||
		registrar.task.TokenPreauthorizedAmount != 1.5 {
		t.Fatalf("registered settlement context mismatch: %+v", registrar.task)
	}
}

func TestBatchSubmitFailsClosedWhenPollingRegistrationCannotExtractBatchID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"object":"batch","status":"validating"}`))
	}))
	t.Cleanup(upstream.Close)

	router := &batchSubmitBillingRouter{
		channel: &types.RouteChannel{
			Channel: &types.Channel{
				ID:       "ch_batch_new_missing_id",
				Provider: "openai",
				BaseURL:  upstream.URL,
				APIKey:   "sk-batch",
				Enabled:  true,
			},
			ChannelID: "ch_batch_new_missing_id",
			Enabled:   true,
			Healthy:   true,
		},
	}
	previous := GetRouter()
	SetRouter(router)
	t.Cleanup(func() {
		SetRouter(previous)
	})

	registrar := &recordingBatchPollingRegistrar{}
	handler := NewBatchHandler(nil, channel.NewOpenAIAdapter("https://direct.example.invalid", "sk-direct")).
		WithCommercialLifecycleEnabled(true).
		WithPollingRegistrar(registrar)
	engine := gin.New()
	engine.POST("/v1/batch", func(c *gin.Context) {
		_ = handler.Handle(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/batch", strings.NewReader(`{"model":"gpt-4.1-mini","input_file_id":"file_123","endpoint":"/v1/chat/completions"}`))
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadGateway, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "batch_polling_registration_failed") {
		t.Fatalf("expected batch_polling_registration_failed response, got %s", rec.Body.String())
	}
	if registrar.task.BatchID != "" {
		t.Fatalf("unexpected polling registration for missing batch id: %+v", registrar.task)
	}
}

type batchSubmitBillingRouter struct {
	routeWithBillingCalls    int
	apiType                  types.APIType
	model                    string
	idempotencyKey           string
	usage                    *types.Usage
	channel                  *types.RouteChannel
	billingSessionID         string
	preauthorizedAmount      float64
	tokenPreauthorizedAmount float64
}

func (r *batchSubmitBillingRouter) Route(_ context.Context, _ string, fn func(ch *types.RouteChannel) (*types.ProviderResponse, error)) (*types.ProviderResponse, error) {
	return fn(r.channel)
}

func (r *batchSubmitBillingRouter) RouteWithBilling(_ context.Context, apiType types.APIType, model, _, idempotencyKey string, usage *types.Usage, fn func(ch *types.RouteChannel) (*types.ProviderResponse, error)) (*types.ProviderResponse, error) {
	r.routeWithBillingCalls++
	r.apiType = apiType
	r.model = model
	r.idempotencyKey = idempotencyKey
	r.usage = usage
	resp, err := fn(r.channel)
	if resp != nil {
		resp.BillingSessionID = r.billingSessionID
		resp.PreauthorizedAmount = r.preauthorizedAmount
		resp.TokenPreauthorizedAmount = r.tokenPreauthorizedAmount
	}
	return resp, err
}

func (r *batchSubmitBillingRouter) RecordChannelSuccess(_ string) {}

func (r *batchSubmitBillingRouter) RecordChannelFailure(_ string) {}

type recordingBatchPollingRegistrar struct {
	task BatchPollingRegistration
}

func (r *recordingBatchPollingRegistrar) RegisterBatchPolling(_ context.Context, task BatchPollingRegistration) error {
	r.task = task
	return nil
}
