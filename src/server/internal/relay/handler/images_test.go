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

func TestImagesGenerationHandlerUsesSelectedOpenAICompatibleAdapterAndPreservesBody(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamPath string
	var upstreamAuth string
	var upstreamBody struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
		Size   string `json:"size"`
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		upstreamAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"created":123,
			"data":[{"url":"https://images.example/generated.png"}]
		}`))
	}))
	t.Cleanup(upstream.Close)

	selectedChannel := &types.RouteChannel{
		Channel: &types.Channel{
			ID:       "ch-openrouter-image",
			Name:     "OpenRouter image",
			Provider: "openrouter",
			BaseURL:  upstream.URL,
			APIKey:   "sk-openrouter-image",
			Enabled:  true,
		},
		ChannelID: "ch-openrouter-image",
		Enabled:   true,
		Healthy:   true,
	}
	restoreRouter := setRouterForChatTest(&chatTestRouter{selected: selectedChannel})
	t.Cleanup(restoreRouter)

	handler := NewImagesHandler(nil, &channel.OpenAIAdapter{})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", strings.NewReader(`{
		"model":"dall-e-3",
		"prompt":"a diagram of a relay gateway",
		"size":"1024x1024"
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if upstreamPath != "/v1/images/generations" {
		t.Fatalf("upstream path = %q, want /v1/images/generations", upstreamPath)
	}
	if upstreamAuth != "Bearer sk-openrouter-image" {
		t.Fatalf("authorization = %q, want selected channel key", upstreamAuth)
	}
	if upstreamBody.Model != "dall-e-3" || upstreamBody.Prompt != "a diagram of a relay gateway" || upstreamBody.Size != "1024x1024" {
		t.Fatalf("image generation body not preserved: %+v", upstreamBody)
	}
	if !strings.Contains(rec.Body.String(), "generated.png") {
		t.Fatalf("expected provider response body, got %s", rec.Body.String())
	}
}
