package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/channel"
)

func TestFineTuningHandlerFailsClosedWithoutUpstreamPassthrough(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.WriteHeader(http.StatusTeapot)
	}))
	defer upstream.Close()

	handler := NewFineTuningHandler(channel.NewOpenAIAdapter(upstream.URL, "sk-test"))
	engine := gin.New()
	engine.POST("/v1/fine_tuning/jobs", func(c *gin.Context) { _ = handler.Handle(c) })
	engine.GET("/v1/fine_tuning/jobs", func(c *gin.Context) { _ = handler.Handle(c) })
	engine.GET("/v1/fine_tuning/jobs/:id", func(c *gin.Context) { _ = handler.Handle(c) })
	engine.POST("/v1/fine_tuning/jobs/:id/cancel", func(c *gin.Context) { _ = handler.Handle(c) })
	engine.GET("/v1/fine_tuning/jobs/:id/events", func(c *gin.Context) { _ = handler.Handle(c) })

	for _, tc := range []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/v1/fine_tuning/jobs"},
		{method: http.MethodGet, path: "/v1/fine_tuning/jobs"},
		{method: http.MethodGet, path: "/v1/fine_tuning/jobs/ftjob_123"},
		{method: http.MethodPost, path: "/v1/fine_tuning/jobs/ftjob_123/cancel"},
		{method: http.MethodGet, path: "/v1/fine_tuning/jobs/ftjob_123/events"},
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
		t.Fatalf("fine-tuning handler must not call upstream while disabled, got %d calls", upstreamCalls)
	}
}
