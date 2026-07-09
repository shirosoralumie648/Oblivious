package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
)

func TestRealtimeHandlerFailsClosedWhenCommercialLifecycleDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamDialCount int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamDialCount, 1)
		upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err == nil {
			_ = conn.Close()
		}
	}))
	defer upstream.Close()

	router := &realtimeLifecycleRouter{
		selected: &types.RouteChannel{
			Channel: &types.Channel{
				ID:       "ch_realtime_disabled",
				Provider: "openai",
				BaseURL:  upstream.URL,
				APIKey:   "sk-realtime",
				Enabled:  true,
			},
		},
	}
	restoreRouter := setRouterForRealtimeTest(router)
	defer restoreRouter()

	handler := NewRealtimeHandler(nil, channel.NewOpenAIAdapter(upstream.URL, "sk-direct"))
	engine := gin.New()
	engine.GET("/v1/realtime", func(c *gin.Context) {
		_ = handler.Handle(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/realtime?model=gpt-4o-realtime-preview", nil)
	rec := httptest.NewRecorder()
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotImplemented, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "unsupported_api") {
		t.Fatalf("expected unsupported_api response, got %s", rec.Body.String())
	}
	if router.routeWithBillingCalls != 0 {
		t.Fatalf("RouteWithBilling calls = %d, want 0 while lifecycle is disabled", router.routeWithBillingCalls)
	}
	if got := atomic.LoadInt32(&upstreamDialCount); got != 0 {
		t.Fatalf("upstream realtime dials = %d, want 0 while lifecycle is disabled", got)
	}
}

func TestRealtimeRejectsMissingModelBeforeUpstreamDial(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewRealtimeHandler(nil, channel.NewOpenAIAdapter("https://direct.example.invalid", "sk-direct")).
		WithCommercialLifecycleEnabled(true)
	engine := gin.New()
	engine.GET("/v1/realtime", func(c *gin.Context) {
		_ = handler.Handle(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/realtime", nil)
	rec := httptest.NewRecorder()

	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "realtime_model_required") {
		t.Fatalf("expected realtime_model_required response, got %s", rec.Body.String())
	}
}

func TestRealtimeRejectsDisallowedOriginBeforeBillingAndUpstreamDial(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamDialCount int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&upstreamDialCount, 1)
		upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err == nil {
			_ = conn.Close()
		}
	}))
	defer upstream.Close()

	router := &realtimeLifecycleRouter{
		selected: &types.RouteChannel{
			ChannelID: "ch_realtime_origin",
			Channel: &types.Channel{
				ID:       "ch_realtime_origin",
				Provider: "openai",
				BaseURL:  upstream.URL,
				APIKey:   "sk-realtime",
				Enabled:  true,
			},
		},
	}
	restoreRouter := setRouterForRealtimeTest(router)
	defer restoreRouter()

	handler := NewRealtimeHandler(nil, channel.NewOpenAIAdapter("https://direct.example.invalid", "sk-direct")).
		WithCommercialLifecycleEnabled(true)
	engine := gin.New()
	engine.GET("/v1/realtime", func(c *gin.Context) {
		_ = handler.Handle(c)
	})
	server := httptest.NewServer(engine)
	defer server.Close()

	headers := http.Header{}
	headers.Set("Origin", "https://evil.example.test")
	headers.Set("OpenAI-Realtime-Connection-ID", "conn_realtime_origin_block")
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/realtime?model=gpt-4o-realtime-preview"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err == nil {
		_ = conn.Close()
		t.Fatal("expected disallowed realtime origin to be rejected")
	}
	if resp == nil {
		t.Fatalf("expected forbidden response for disallowed origin, got dial error: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
	if router.routeWithBillingCalls != 0 {
		t.Fatalf("RouteWithBilling calls = %d, want 0 before origin is allowed", router.routeWithBillingCalls)
	}
	if got := atomic.LoadInt32(&upstreamDialCount); got != 0 {
		t.Fatalf("upstream realtime dials = %d, want 0 before origin is allowed", got)
	}
}

func TestRealtimeUsesAdapterRealtimeEndpointWithoutDuplicatePath(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/realtime" {
			t.Fatalf("upstream path = %q, want /v1/realtime", r.URL.Path)
		}
		upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade upstream websocket: %v", err)
		}
		_ = conn.Close()
	}))
	defer upstream.Close()

	handler := NewRealtimeHandler(nil, channel.NewOpenAIAdapter(upstream.URL, "sk-test")).
		WithCommercialLifecycleEnabled(true)
	engine := gin.New()
	engine.GET("/v1/realtime", func(c *gin.Context) {
		_ = handler.Handle(c)
	})

	server := httptest.NewServer(engine)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/realtime?model=gpt-4o-realtime-preview"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		_ = conn.Close()
		return
	}
	if resp != nil {
		t.Fatalf("client dial failed with status %d: %v", resp.StatusCode, err)
	}
	t.Fatalf("client dial failed before response: %v", err)
}

func TestRealtimeUsesRouteWithBillingForLifecycle(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade upstream websocket: %v", err)
		}
		_ = conn.Close()
	}))
	defer upstream.Close()

	router := &realtimeLifecycleRouter{
		selected: &types.RouteChannel{
			ChannelID: "ch_realtime",
			Channel: &types.Channel{
				ID:       "ch_realtime",
				Provider: "openai",
				BaseURL:  upstream.URL,
				APIKey:   "sk-realtime",
				Enabled:  true,
			},
		},
	}
	restoreRouter := setRouterForRealtimeTest(router)
	defer restoreRouter()

	handler := NewRealtimeHandler(nil, channel.NewOpenAIAdapter("https://direct.example.invalid", "sk-direct")).
		WithCommercialLifecycleEnabled(true)
	engine := gin.New()
	engine.GET("/v1/realtime", func(c *gin.Context) {
		_ = handler.Handle(c)
	})
	server := httptest.NewServer(engine)
	defer server.Close()

	headers := http.Header{}
	headers.Set(types.HeaderRequestID, "req_realtime")
	headers.Set("OpenAI-Realtime-Connection-ID", "conn_realtime")
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/realtime?model=gpt-4o-realtime-preview"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err == nil {
		_ = conn.Close()
	} else if resp != nil {
		t.Fatalf("client dial failed with status %d: %v", resp.StatusCode, err)
	} else {
		t.Fatalf("client dial failed before response: %v", err)
	}

	if router.routeWithBillingCalls != 1 {
		t.Fatalf("RouteWithBilling calls = %d, want 1", router.routeWithBillingCalls)
	}
	if router.apiType != types.APITypeRealtime {
		t.Fatalf("apiType = %s, want realtime", router.apiType.String())
	}
	if router.model != "gpt-4o-realtime-preview" {
		t.Fatalf("model = %q, want realtime model", router.model)
	}
	if router.idempotencyKey != "conn_realtime" {
		t.Fatalf("idempotency key = %q, want connection id", router.idempotencyKey)
	}
	if !router.trustedStreaming {
		t.Fatal("expected realtime lifecycle to mark request as trusted streaming")
	}
	if router.usage == nil {
		t.Fatal("expected realtime lifecycle to pass estimated usage")
	}
	if upstreamPath != "/v1/realtime" {
		t.Fatalf("upstream path = %q, want /v1/realtime", upstreamPath)
	}
}

func TestRealtimeCapturesUpstreamUsageForBillingSettlement(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade upstream websocket: %v", err)
		}
		defer conn.Close()
		if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"response.done","response":{"usage":{"input_tokens":11,"output_tokens":7,"total_tokens":18}}}`)); err != nil {
			t.Fatalf("write upstream usage event: %v", err)
		}
	}))
	defer upstream.Close()

	router := &realtimeLifecycleRouter{
		done: make(chan struct{}),
		selected: &types.RouteChannel{
			ChannelID: "ch_realtime_usage",
			Channel: &types.Channel{
				ID:       "ch_realtime_usage",
				Provider: "openai",
				BaseURL:  upstream.URL,
				APIKey:   "sk-realtime",
				Enabled:  true,
			},
		},
	}
	restoreRouter := setRouterForRealtimeTest(router)
	defer restoreRouter()

	handler := NewRealtimeHandler(nil, channel.NewOpenAIAdapter("https://direct.example.invalid", "sk-direct")).
		WithCommercialLifecycleEnabled(true)
	engine := gin.New()
	engine.GET("/v1/realtime", func(c *gin.Context) {
		_ = handler.Handle(c)
	})
	server := httptest.NewServer(engine)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/realtime?model=gpt-4o-realtime-preview"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"OpenAI-Realtime-Connection-ID": []string{"conn_realtime_usage"}})
	if err != nil {
		if resp != nil {
			t.Fatalf("client dial failed with status %d: %v", resp.StatusCode, err)
		}
		t.Fatalf("client dial failed before response: %v", err)
	}
	_, _, _ = conn.ReadMessage()
	_ = conn.Close()
	select {
	case <-router.done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for realtime billing response")
	}

	if router.responseUsage == nil {
		t.Fatal("expected realtime billing response usage to be captured")
	}
	if router.responseUsage.PromptTokens != 11 || router.responseUsage.CompletionTokens != 7 || router.responseUsage.TotalTokens != 18 {
		t.Fatalf("unexpected realtime billing usage: %+v", router.responseUsage)
	}
}

func TestRealtimeFailsClosedWhenUpstreamClosesWithoutUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade upstream websocket: %v", err)
		}
		_ = conn.Close()
	}))
	defer upstream.Close()

	router := &realtimeLifecycleRouter{
		done: make(chan struct{}),
		selected: &types.RouteChannel{
			ChannelID: "ch_realtime_no_usage",
			Channel: &types.Channel{
				ID:       "ch_realtime_no_usage",
				Provider: "openai",
				BaseURL:  upstream.URL,
				APIKey:   "sk-realtime",
				Enabled:  true,
			},
		},
	}
	restoreRouter := setRouterForRealtimeTest(router)
	defer restoreRouter()

	handler := NewRealtimeHandler(nil, channel.NewOpenAIAdapter("https://direct.example.invalid", "sk-direct")).
		WithCommercialLifecycleEnabled(true)
	engine := gin.New()
	engine.GET("/v1/realtime", func(c *gin.Context) {
		_ = handler.Handle(c)
	})
	server := httptest.NewServer(engine)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/v1/realtime?model=gpt-4o-realtime-preview"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"OpenAI-Realtime-Connection-ID": []string{"conn_realtime_no_usage"}})
	if err != nil {
		if resp != nil {
			t.Fatalf("client dial failed with status %d: %v", resp.StatusCode, err)
		}
		t.Fatalf("client dial failed before response: %v", err)
	}
	_ = conn.Close()
	select {
	case <-router.done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for realtime billing response")
	}

	if router.responseErr == nil {
		t.Fatal("expected realtime billing callback to fail without upstream usage")
	}
	if !strings.Contains(router.responseErr.Error(), "realtime_usage_missing") {
		t.Fatalf("expected realtime_usage_missing error, got %v", router.responseErr)
	}
	if router.responseUsage != nil && router.responseUsage.TotalTokens != 0 {
		t.Fatalf("missing-usage realtime response must not settle token usage, got %+v", router.responseUsage)
	}
}

type realtimeLifecycleRouter struct {
	selected              *types.RouteChannel
	routeWithBillingCalls int
	apiType               types.APIType
	model                 string
	idempotencyKey        string
	usage                 *types.Usage
	responseUsage         *types.Usage
	responseErr           error
	trustedStreaming      bool
	done                  chan struct{}
	doneOnce              sync.Once
}

func (r *realtimeLifecycleRouter) Route(_ context.Context, _ string, _ func(ch *types.RouteChannel) (*types.ProviderResponse, error)) (*types.ProviderResponse, error) {
	return nil, errors.New("unexpected non-billing realtime route")
}

func (r *realtimeLifecycleRouter) RouteWithBilling(ctx context.Context, apiType types.APIType, model, _, idempotencyKey string, usage *types.Usage, fn func(ch *types.RouteChannel) (*types.ProviderResponse, error)) (*types.ProviderResponse, error) {
	r.routeWithBillingCalls++
	r.apiType = apiType
	r.model = model
	r.idempotencyKey = idempotencyKey
	r.usage = usage
	r.trustedStreaming, _ = types.TrustedStreamingFromContext(ctx)
	resp, err := fn(r.selected)
	if resp != nil {
		r.responseUsage = resp.Usage
	}
	r.responseErr = err
	if r.done != nil {
		r.doneOnce.Do(func() { close(r.done) })
	}
	return resp, err
}

func (r *realtimeLifecycleRouter) RecordChannelSuccess(_ string) {}

func (r *realtimeLifecycleRouter) RecordChannelFailure(_ string) {}

func setRouterForRealtimeTest(router types.RouterInterface) func() {
	previous := GetRouter()
	SetRouter(router)
	return func() {
		SetRouter(previous)
	}
}
