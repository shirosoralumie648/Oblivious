package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/channel"
)

func TestAssistantsHandlerFailsClosedWithoutUpstreamPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.WriteHeader(http.StatusTeapot)
	}))
	defer upstream.Close()

	handler := NewAssistantsHandler(channel.NewOpenAIAdapter(upstream.URL, "sk-test"))
	engine := gin.New()
	engine.POST("/v1/assistants", func(c *gin.Context) { _ = handler.Handle(c) })
	engine.GET("/v1/assistants", func(c *gin.Context) { _ = handler.Handle(c) })
	engine.GET("/v1/assistants/:id", func(c *gin.Context) { _ = handler.Handle(c) })
	engine.POST("/v1/assistants/:id", func(c *gin.Context) { _ = handler.Handle(c) })
	engine.DELETE("/v1/assistants/:id", func(c *gin.Context) { _ = handler.Handle(c) })
	engine.POST("/v1/threads", func(c *gin.Context) { _ = handler.Handle(c) })
	engine.GET("/v1/threads/:id", func(c *gin.Context) { _ = handler.Handle(c) })
	engine.POST("/v1/threads/:id/runs", func(c *gin.Context) { _ = handler.Handle(c) })
	engine.GET("/v1/threads/:id/runs/:rid", func(c *gin.Context) { _ = handler.Handle(c) })
	engine.POST("/v1/threads/:id/runs/:rid/submit", func(c *gin.Context) { _ = handler.Handle(c) })

	for _, tc := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/v1/assistants"},
		{method: http.MethodGet, path: "/v1/assistants"},
		{method: http.MethodGet, path: "/v1/assistants/asst_123"},
		{method: http.MethodPost, path: "/v1/assistants/asst_123"},
		{method: http.MethodDelete, path: "/v1/assistants/asst_123"},
		{method: http.MethodPost, path: "/v1/threads"},
		{method: http.MethodGet, path: "/v1/threads/thread_123"},
		{method: http.MethodPost, path: "/v1/threads/thread_123/runs"},
		{method: http.MethodGet, path: "/v1/threads/thread_123/runs/run_123"},
		{method: http.MethodPost, path: "/v1/threads/thread_123/runs/run_123/submit"},
	} {
		req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(`{}`))
		rec := httptest.NewRecorder()

		engine.ServeHTTP(rec, req)

		if rec.Code != http.StatusNotImplemented {
			t.Fatalf("%s %s status = %d, want 501; body=%s", tc.method, tc.path, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), `"code":"unsupported_api"`) {
			t.Fatalf("%s %s expected unsupported_api response, got %s", tc.method, tc.path, rec.Body.String())
		}
	}

	if upstreamCalls != 0 {
		t.Fatalf("assistants/threads/runs handler must not call upstream while disabled, got %d calls", upstreamCalls)
	}
}
