package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
)

func TestLegacyCompletionsHandlerUsesSelectedOpenAICompatibleAdapter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamPath string
	var upstreamAuth string
	var upstreamBody struct {
		Model     string `json:"model"`
		Prompt    string `json:"prompt"`
		MaxTokens int    `json:"max_tokens"`
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		upstreamAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id":"cmpl_openrouter",
			"choices":[{"text":"ok"}],
			"usage":{"prompt_tokens":6,"completion_tokens":4,"total_tokens":10}
		}`))
	}))
	t.Cleanup(upstream.Close)

	selectedChannel := &types.RouteChannel{
		Channel: &types.Channel{
			ID:       "ch-openrouter-completions",
			Name:     "OpenRouter completions",
			Provider: "openrouter",
			BaseURL:  upstream.URL,
			APIKey:   "sk-openrouter-completions",
			Enabled:  true,
		},
		ChannelID: "ch-openrouter-completions",
		Enabled:   true,
		Healthy:   true,
	}
	testRouter := &chatTestRouter{selected: selectedChannel}
	restoreRouter := setRouterForChatTest(testRouter)
	t.Cleanup(restoreRouter)

	handler := NewLegacyCompletionsHandler(nil, &channel.OpenAIAdapter{})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/completions", strings.NewReader(`{
		"model":"openrouter/auto",
		"prompt":"complete this",
		"max_tokens":24
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if upstreamPath != "/v1/completions" {
		t.Fatalf("upstream path = %q, want /v1/completions", upstreamPath)
	}
	if upstreamAuth != "Bearer sk-openrouter-completions" {
		t.Fatalf("authorization = %q, want selected channel key", upstreamAuth)
	}
	if upstreamBody.Model != "openrouter/auto" || upstreamBody.Prompt != "complete this" || upstreamBody.MaxTokens != 24 {
		t.Fatalf("OpenAI-compatible completions body not preserved: %+v", upstreamBody)
	}
	if testRouter.lastProviderUsage == nil {
		t.Fatal("expected provider usage to be normalized")
	}
	if testRouter.lastProviderUsage.PromptTokens != 6 || testRouter.lastProviderUsage.CompletionTokens != 4 || testRouter.lastProviderUsage.TotalTokens != 10 {
		t.Fatalf("unexpected provider usage: %+v", testRouter.lastProviderUsage)
	}
	if !strings.Contains(rec.Body.String(), "cmpl_openrouter") {
		t.Fatalf("expected provider response body, got %s", rec.Body.String())
	}
}

func TestLegacyCompletionsHandlerAttachesSemanticCacheRequestToRouterContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testRouter := &chatTestRouter{
		selected: &types.RouteChannel{
			Channel:   &types.Channel{ID: "ch-completion-cache", Provider: "openai", BaseURL: "http://provider.test", APIKey: "sk-cache", Enabled: true},
			ChannelID: "ch-completion-cache",
			Enabled:   true,
			Healthy:   true,
		},
		resp: types.NewOKResponse([]byte(`{"id":"fresh-completion","choices":[{"text":"fresh"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`), &types.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2}),
	}
	restoreRouter := setRouterForChatTest(testRouter)
	t.Cleanup(restoreRouter)

	handler := NewLegacyCompletionsHandler(nil, &channel.OpenAIAdapter{})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/completions", strings.NewReader(`{
		"model":"gpt-4o-mini",
		"prompt":"complete this"
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set(types.HeaderInternalAuth, types.SharedInternalToken)
	ctx.Request.Header.Set(types.HeaderInternalOrganization, "org_1")
	ctx.Request.Header.Set(types.HeaderInternalUserID, "user_1")

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if testRouter.routeWithBillingCalls != 1 {
		t.Fatalf("expected handler to route through billing, got %d calls", testRouter.routeWithBillingCalls)
	}
	cacheReq := testRouter.lastSemanticCacheRequest
	if cacheReq.Model != "gpt-4o-mini" || cacheReq.OrganizationID != "org_1" || cacheReq.UserID != "user_1" || cacheReq.UserScoped {
		t.Fatalf("semantic cache request not attached correctly: %+v", cacheReq)
	}
	if !strings.Contains(cacheReq.Query, `"api_type":"completions"`) || !strings.Contains(cacheReq.Query, `"route":"/v1/completions"`) || !strings.Contains(cacheReq.Query, `"prompt":"complete this"`) {
		t.Fatalf("semantic cache query should include canonical completion request context, got %s", cacheReq.Query)
	}
}

func TestLegacyCompletionsHandlerFallsBackWhenSemanticCacheEmbeddingFails(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testRouter := &chatTestRouter{
		selected: &types.RouteChannel{
			Channel:   &types.Channel{ID: "ch-completion-cache-fallback", Provider: "openai", BaseURL: "http://provider.test", APIKey: "sk-cache", Enabled: true},
			ChannelID: "ch-completion-cache-fallback",
			Enabled:   true,
			Healthy:   true,
		},
		resp: types.NewOKResponse([]byte(`{"id":"fresh-completion","choices":[{"text":"fresh"}]}`), nil),
	}
	restoreRouter := setRouterForChatTest(testRouter)
	t.Cleanup(restoreRouter)

	embedder := &semanticCacheTestEmbedder{err: errors.New("embedding provider unavailable")}
	handler := NewLegacyCompletionsHandlerWithSemanticCacheEmbedder(nil, &channel.OpenAIAdapter{}, embedder)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/completions", strings.NewReader(`{
		"model":"gpt-4o-mini",
		"prompt":"complete this"
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if len(embedder.texts) != 1 {
		t.Fatalf("expected embedder to be called once, got %d", len(embedder.texts))
	}
	cacheReq := testRouter.lastSemanticCacheRequest
	if cacheReq.Query == "" {
		t.Fatalf("expected semantic cache request to remain cacheable after embedding failure, got %+v", cacheReq)
	}
	if len(cacheReq.QueryEmbedding) != 0 {
		t.Fatalf("expected embedding failure to degrade without vector, got %+v", cacheReq.QueryEmbedding)
	}
}

func TestLegacyCompletionsHandlerMarksSensitiveSemanticCacheRequestsUserScoped(t *testing.T) {
	req := &channel.ProviderRequest{
		APIType: types.APITypeCompletions,
		Model:   "gpt-4o-mini",
		Prompt:  "Summarize shiro@example.com account",
	}
	ctx := types.WithTrustedUserID(context.Background(), "user_1")
	ctx = types.WithTrustedOrganizationID(ctx, "org_1")

	cacheReq, ok := semanticCacheRequestFromCompletion(ctx, req)
	if !ok {
		t.Fatal("expected sensitive completion request to be cacheable")
	}
	if !cacheReq.UserScoped {
		t.Fatalf("expected sensitive cache request to be user scoped, got %+v", cacheReq)
	}
}

func TestLegacyCompletionsSemanticCacheQueryIncludesMaxTokens(t *testing.T) {
	baseReq := &channel.ProviderRequest{
		APIType:   types.APITypeCompletions,
		Model:     "gpt-4o-mini",
		Prompt:    "complete this",
		MaxTokens: 16,
	}
	otherReq := &channel.ProviderRequest{
		APIType:   types.APITypeCompletions,
		Model:     "gpt-4o-mini",
		Prompt:    "complete this",
		MaxTokens: 32,
	}

	baseCacheReq, ok := semanticCacheRequestFromCompletion(context.Background(), baseReq)
	if !ok {
		t.Fatal("expected base completion request to be cacheable")
	}
	otherCacheReq, ok := semanticCacheRequestFromCompletion(context.Background(), otherReq)
	if !ok {
		t.Fatal("expected other completion request to be cacheable")
	}

	if baseCacheReq.Query == otherCacheReq.Query {
		t.Fatalf("semantic cache query must include max_tokens; both were %s", baseCacheReq.Query)
	}
	if !strings.Contains(baseCacheReq.Query, `"api_type":"completions"`) || !strings.Contains(baseCacheReq.Query, `"route":"/v1/completions"`) || !strings.Contains(baseCacheReq.Query, `"prompt":"complete this"`) || !strings.Contains(baseCacheReq.Query, `"max_tokens":16`) {
		t.Fatalf("semantic cache query should include canonical completion request fields, got %s", baseCacheReq.Query)
	}
	if !strings.Contains(otherCacheReq.Query, `"max_tokens":32`) {
		t.Fatalf("semantic cache query should include other max_tokens, got %s", otherCacheReq.Query)
	}
}
