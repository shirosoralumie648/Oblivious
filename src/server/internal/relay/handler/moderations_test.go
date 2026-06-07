package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
)

func TestModerationsHandlerUsesSelectedOpenAICompatibleAdapter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamPath string
	var upstreamAuth string
	var upstreamBody struct {
		Model string `json:"model"`
		Input string `json:"input"`
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		upstreamAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"modr_openrouter",
			"model":"omni-moderation-latest",
			"results":[{"flagged":false}]
		}`))
	}))
	t.Cleanup(upstream.Close)

	selectedChannel := &types.RouteChannel{
		Channel: &types.Channel{
			ID:       "ch-openrouter-moderations",
			Name:     "OpenRouter moderations",
			Provider: "openrouter",
			BaseURL:  upstream.URL,
			APIKey:   "sk-openrouter-moderations",
			Enabled:  true,
		},
		ChannelID: "ch-openrouter-moderations",
		Enabled:   true,
		Healthy:   true,
	}
	restoreRouter := setRouterForChatTest(&chatTestRouter{selected: selectedChannel})
	t.Cleanup(restoreRouter)

	handler := NewModerationsHandler(nil, &channel.OpenAIAdapter{})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/moderations", strings.NewReader(`{
		"model":"omni-moderation-latest",
		"input":"screen this text"
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if upstreamPath != "/v1/moderations" {
		t.Fatalf("upstream path = %q, want /v1/moderations", upstreamPath)
	}
	if upstreamAuth != "Bearer sk-openrouter-moderations" {
		t.Fatalf("authorization = %q, want selected channel key", upstreamAuth)
	}
	if upstreamBody.Model != "omni-moderation-latest" || upstreamBody.Input != "screen this text" {
		t.Fatalf("moderation body not preserved: %+v", upstreamBody)
	}
	if !strings.Contains(rec.Body.String(), "modr_openrouter") {
		t.Fatalf("expected provider response body, got %s", rec.Body.String())
	}
}
