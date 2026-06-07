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

func TestEmbeddingsHandlerPreservesStringArrayInputAndEstimatesUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamBody struct {
		Model string   `json:"model"`
		Input []string `json:"input"`
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"list","data":[{"embedding":[0.1]},{"embedding":[0.2]}],"usage":{"prompt_tokens":2,"total_tokens":2}}`))
	}))
	t.Cleanup(upstream.Close)

	selectedChannel := &types.RouteChannel{
		Channel: &types.Channel{
			ID:       "ch-embedding-openai-array",
			Name:     "Embedding OpenAI Array",
			Provider: "openai",
			BaseURL:  upstream.URL,
			APIKey:   "sk-openai-embedding-array",
			Enabled:  true,
		},
		ChannelID: "ch-embedding-openai-array",
		Enabled:   true,
		Healthy:   true,
	}
	testRouter := &embeddingsUsageRouter{selected: selectedChannel}
	restoreRouter := setRouterForChatTest(testRouter)
	t.Cleanup(restoreRouter)

	handler := NewEmbeddingsHandler(nil, &channel.OpenAIAdapter{})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", strings.NewReader(`{
		"model":"text-embedding-3-small",
		"input":["a","b"]
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if upstreamBody.Model != "text-embedding-3-small" {
		t.Fatalf("model = %q, want text-embedding-3-small", upstreamBody.Model)
	}
	if len(upstreamBody.Input) != 2 || upstreamBody.Input[0] != "a" || upstreamBody.Input[1] != "b" {
		t.Fatalf("array input not preserved upstream: %+v", upstreamBody.Input)
	}
	if testRouter.lastUsage == nil {
		t.Fatal("expected usage estimate to be passed to RouteWithBilling")
	}
	if testRouter.lastUsage.PromptTokens < 2 || testRouter.lastUsage.TotalTokens != testRouter.lastUsage.PromptTokens {
		t.Fatalf("unexpected usage estimate for array input: %+v", testRouter.lastUsage)
	}
}

func TestEmbeddingsHandlerUsesSelectedGeminiAdapterAndNormalizesUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamPath string
	var upstreamAPIKey string
	var upstreamBody struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		upstreamAPIKey = r.Header.Get("x-goog-api-key")
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"embedding":{"values":[0.1,0.2,0.3]},
			"usageMetadata":{"promptTokenCount":9,"totalTokenCount":9}
		}`))
	}))
	t.Cleanup(upstream.Close)

	selectedChannel := &types.RouteChannel{
		Channel: &types.Channel{
			ID:       "ch-embedding-gemini",
			Name:     "Embedding Gemini",
			Provider: "gemini",
			BaseURL:  upstream.URL,
			APIKey:   "sk-gemini-embedding",
			Enabled:  true,
		},
		ChannelID: "ch-embedding-gemini",
		Enabled:   true,
		Healthy:   true,
	}
	testRouter := &chatTestRouter{selected: selectedChannel}
	restoreRouter := setRouterForChatTest(testRouter)
	t.Cleanup(restoreRouter)

	handler := NewEmbeddingsHandler(nil, &channel.OpenAIAdapter{})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/embeddings", strings.NewReader(`{
		"model":"text-embedding-004",
		"input":"embed this document"
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if upstreamPath != "/v1beta/models/text-embedding-004:embedContent" {
		t.Fatalf("upstream path = %q, want Gemini embedContent path", upstreamPath)
	}
	if upstreamAPIKey != "sk-gemini-embedding" {
		t.Fatalf("x-goog-api-key = %q, want selected channel key", upstreamAPIKey)
	}
	if len(upstreamBody.Content.Parts) != 1 || upstreamBody.Content.Parts[0].Text != "embed this document" {
		t.Fatalf("Gemini embedding body not built from input: %+v", upstreamBody.Content.Parts)
	}
	if testRouter.lastProviderUsage == nil {
		t.Fatal("expected Gemini usageMetadata to be normalized into provider usage")
	}
	if testRouter.lastProviderUsage.PromptTokens != 9 || testRouter.lastProviderUsage.TotalTokens != 9 {
		t.Fatalf("unexpected normalized usage: %+v", testRouter.lastProviderUsage)
	}
	if !strings.Contains(rec.Body.String(), "embedding") {
		t.Fatalf("expected Gemini provider response body, got %s", rec.Body.String())
	}
}

type embeddingsUsageRouter struct {
	selected  *types.RouteChannel
	lastUsage *types.Usage
}

func (r *embeddingsUsageRouter) Route(_ context.Context, _ string, fn func(ch *types.RouteChannel) (*types.ProviderResponse, error)) (*types.ProviderResponse, error) {
	return fn(r.selected)
}

func (r *embeddingsUsageRouter) RouteWithBilling(_ context.Context, _ types.APIType, _, _, _ string, usage *types.Usage, fn func(ch *types.RouteChannel) (*types.ProviderResponse, error)) (*types.ProviderResponse, error) {
	r.lastUsage = usage
	return fn(r.selected)
}

func (r *embeddingsUsageRouter) RecordChannelSuccess(_ string) {}

func (r *embeddingsUsageRouter) RecordChannelFailure(_ string) {}
