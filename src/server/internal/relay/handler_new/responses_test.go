package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
)

func TestResponsesStreamProxiesProviderSSEThroughBillingRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamPath string
	var upstreamAuth string
	var upstreamBody struct {
		Model  string `json:"model"`
		Stream bool   `json:"stream"`
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		upstreamAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"type\":\"response.output_text.delta\"}\n\n"))
	}))
	t.Cleanup(upstream.Close)

	testRouter := &responsesTestRouter{
		selected: &types.RouteChannel{
			Channel: &types.Channel{
				ID:       "ch-responses-stream",
				Name:     "Responses Stream",
				Provider: "openai",
				BaseURL:  upstream.URL,
				APIKey:   "sk-responses-stream",
				Enabled:  true,
			},
			ChannelID: "ch-responses-stream",
			Enabled:   true,
			Healthy:   true,
		},
	}
	restoreRouter := setResponsesTestRouter(testRouter)
	t.Cleanup(restoreRouter)

	handler := NewResponsesHandler(nil, channel.NewOpenAIAdapter(upstream.URL, "sk-default"))
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-4o","stream":true}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", contentType)
	}
	if upstreamPath != "/v1/responses" {
		t.Fatalf("upstream path = %q, want /v1/responses", upstreamPath)
	}
	if upstreamAuth != "Bearer sk-default" {
		t.Fatalf("authorization = %q, want configured adapter key", upstreamAuth)
	}
	if upstreamBody.Model != "gpt-4o" || !upstreamBody.Stream {
		t.Fatalf("upstream stream request not preserved: %+v", upstreamBody)
	}
	if testRouter.routeWithBillingCalls != 1 {
		t.Fatalf("RouteWithBilling calls = %d, want 1", testRouter.routeWithBillingCalls)
	}
	if !strings.Contains(rec.Body.String(), `data: {"type":"response.output_text.delta"}`) {
		t.Fatalf("expected provider SSE body, got %s", rec.Body.String())
	}
}

type responsesTestRouter struct {
	selected              *types.RouteChannel
	routeWithBillingCalls int
}

func (r *responsesTestRouter) Route(_ context.Context, _ string, fn func(ch *types.RouteChannel) (*types.ProviderResponse, error)) (*types.ProviderResponse, error) {
	return fn(r.selected)
}

func (r *responsesTestRouter) RouteWithBilling(_ context.Context, _ types.APIType, _, _, _ string, _ *types.Usage, fn func(ch *types.RouteChannel) (*types.ProviderResponse, error)) (*types.ProviderResponse, error) {
	r.routeWithBillingCalls++
	return fn(r.selected)
}

func (r *responsesTestRouter) RecordChannelSuccess(_ string) {}

func (r *responsesTestRouter) RecordChannelFailure(_ string) {}

func setResponsesTestRouter(router types.RouterInterface) func() {
	previous := GetRouter()
	SetRouter(router)
	return func() {
		SetRouter(previous)
	}
}
