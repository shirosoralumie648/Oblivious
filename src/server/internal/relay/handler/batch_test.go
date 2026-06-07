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

func TestBatchSubmitRegistersPollingTaskFromUpstreamBatchID(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/batch" {
			t.Fatalf("upstream path = %q, want /v1/batch", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("upstream method = %q, want POST", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"batch_123","object":"batch","status":"validating"}`))
	}))
	defer upstream.Close()

	registrar := &recordingBatchPollingRegistrar{}
	handler := NewBatchHandler(nil, channel.NewOpenAIAdapter(upstream.URL, "sk-test")).
		WithPollingRegistrar(registrar)

	engine := gin.New()
	engine.POST("/v1/batch", func(c *gin.Context) {
		_ = handler.Handle(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/batch", strings.NewReader(`{"input_file_id":"file_123","endpoint":"/v1/chat/completions"}`))
	req.Header.Set(types.HeaderRequestID, "req_batch_1")
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"id":"batch_123"`) {
		t.Fatalf("expected upstream batch body to be returned, got %s", rec.Body.String())
	}
	if registrar.task.BatchID != "batch_123" {
		t.Fatalf("registered batch id = %q, want batch_123", registrar.task.BatchID)
	}
	if registrar.task.RequestID != "req_batch_1" {
		t.Fatalf("registered request id = %q, want req_batch_1", registrar.task.RequestID)
	}
	if registrar.task.Model != "gpt-4o" || registrar.task.APIType != types.APITypeBatch {
		t.Fatalf("registered model/api type = %q/%s", registrar.task.Model, registrar.task.APIType.String())
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
	defer upstream.Close()

	router := &batchBillingRouter{
		channel: &types.RouteChannel{
			Channel: &types.Channel{
				ID:       "ch_batch",
				Provider: "openai",
				BaseURL:  upstream.URL,
				APIKey:   "sk-test",
			},
		},
	}
	previous := GetRouter()
	SetRouter(router)
	defer SetRouter(previous)

	handler := NewBatchHandler(nil, channel.NewOpenAIAdapter("https://direct.example.invalid", "sk-direct"))
	engine := gin.New()
	engine.POST("/v1/batch", func(c *gin.Context) {
		_ = handler.Handle(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/batch", strings.NewReader(`{"input_file_id":"file_123","endpoint":"/v1/chat/completions"}`))
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
	if router.model != "gpt-4o" {
		t.Fatalf("model = %q, want gpt-4o", router.model)
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

type recordingBatchPollingRegistrar struct {
	task BatchPollingRegistration
}

func (r *recordingBatchPollingRegistrar) RegisterBatchPolling(ctx context.Context, task BatchPollingRegistration) error {
	r.task = task
	return nil
}

type batchBillingRouter struct {
	routeWithBillingCalls int
	apiType               types.APIType
	model                 string
	idempotencyKey        string
	usage                 *types.Usage
	channel               *types.RouteChannel
}

func (r *batchBillingRouter) Route(ctx context.Context, apiType string, fn func(ch *types.RouteChannel) (*types.ProviderResponse, error)) (*types.ProviderResponse, error) {
	return fn(r.channel)
}

func (r *batchBillingRouter) RouteWithBilling(ctx context.Context, apiType types.APIType, model, channelID, idempotencyKey string, usage *types.Usage, fn func(ch *types.RouteChannel) (*types.ProviderResponse, error)) (*types.ProviderResponse, error) {
	r.routeWithBillingCalls++
	r.apiType = apiType
	r.model = model
	r.idempotencyKey = idempotencyKey
	r.usage = usage
	return fn(r.channel)
}

func (r *batchBillingRouter) RecordChannelSuccess(channelID string) {}

func (r *batchBillingRouter) RecordChannelFailure(channelID string) {}
