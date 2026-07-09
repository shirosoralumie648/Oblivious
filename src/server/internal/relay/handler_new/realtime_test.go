package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"oblivious/server/internal/relay/channel"
)

func TestRealtimeHandlerFailsClosedWhenCommercialLifecycleDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err == nil {
			_ = conn.Close()
		}
	}))
	t.Cleanup(upstream.Close)

	handler := NewRealtimeHandler(nil, channel.NewOpenAIAdapter(upstream.URL, "sk-test"))
	engine := gin.New()
	engine.GET("/v1/realtime", func(c *gin.Context) {
		_ = handler.HandleStream(c)
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
	if upstreamCalls != 0 {
		t.Fatalf("upstream realtime calls = %d, want 0 while lifecycle is disabled", upstreamCalls)
	}
}

func TestRealtimeRejectsMissingModelBeforeUpstreamDial(t *testing.T) {
	gin.SetMode(gin.TestMode)

	handler := NewRealtimeHandler(nil, channel.NewOpenAIAdapter("https://direct.example.invalid", "sk-direct")).
		WithCommercialLifecycleEnabled(true)
	engine := gin.New()
	engine.GET("/v1/realtime", func(c *gin.Context) {
		_ = handler.HandleStream(c)
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
	t.Cleanup(upstream.Close)

	handler := NewRealtimeHandler(nil, channel.NewOpenAIAdapter(upstream.URL, "sk-test")).
		WithCommercialLifecycleEnabled(true)
	engine := gin.New()
	engine.GET("/v1/realtime", func(c *gin.Context) {
		_ = handler.HandleStream(c)
	})

	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)

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
