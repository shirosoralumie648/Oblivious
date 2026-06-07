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

func TestResponsesStreamProxiesSelectedProviderSSEThroughBillingRoute(t *testing.T) {
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

	selectedChannel := &types.RouteChannel{
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
	}
	testRouter := &chatTestRouter{selected: selectedChannel}
	restoreRouter := setRouterForChatTest(testRouter)
	t.Cleanup(restoreRouter)

	handler := NewResponsesHandler(nil, &channel.OpenAIAdapter{})
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
	if upstreamAuth != "Bearer sk-responses-stream" {
		t.Fatalf("authorization = %q, want selected channel key", upstreamAuth)
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

func TestResponsesHandlerBuildsMessagesFromInputText(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamBody struct {
		Model    string            `json:"model"`
		Messages []channel.Message `json:"messages"`
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_test","object":"response","output_text":"ok"}`))
	}))
	t.Cleanup(upstream.Close)

	selectedChannel := &types.RouteChannel{
		Channel: &types.Channel{
			ID:       "ch-responses-input",
			Name:     "Responses Input",
			Provider: "openai",
			BaseURL:  upstream.URL,
			APIKey:   "sk-responses-input",
			Enabled:  true,
		},
		ChannelID: "ch-responses-input",
		Enabled:   true,
		Healthy:   true,
	}
	testRouter := &chatTestRouter{selected: selectedChannel}
	restoreRouter := setRouterForChatTest(testRouter)
	t.Cleanup(restoreRouter)

	handler := NewResponsesHandler(nil, &channel.OpenAIAdapter{})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-4o-mini","input":"hello"}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if upstreamBody.Model != "gpt-4o-mini" {
		t.Fatalf("model = %q, want gpt-4o-mini", upstreamBody.Model)
	}
	if len(upstreamBody.Messages) != 1 || upstreamBody.Messages[0].Role != "user" || upstreamBody.Messages[0].Content != "hello" {
		t.Fatalf("messages not derived from Responses input: %+v", upstreamBody.Messages)
	}
	if testRouter.routeWithBillingCalls != 1 {
		t.Fatalf("RouteWithBilling calls = %d, want 1", testRouter.routeWithBillingCalls)
	}
}

func TestResponsesHandlerAttachesSemanticCacheRequestToRouterContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testRouter := &chatTestRouter{
		selected: &types.RouteChannel{
			Channel:   &types.Channel{ID: "ch-responses-cache", Provider: "openai", BaseURL: "http://provider.test", APIKey: "sk-cache", Enabled: true},
			ChannelID: "ch-responses-cache",
			Enabled:   true,
			Healthy:   true,
		},
		resp: types.NewOKResponse([]byte(`{"id":"resp_cache","object":"response","output_text":"fresh"}`), &types.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2}),
	}
	restoreRouter := setRouterForChatTest(testRouter)
	t.Cleanup(restoreRouter)

	handler := NewResponsesHandler(nil, &channel.OpenAIAdapter{})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{
		"model":"gpt-4o-mini",
		"input":[{"role":"user","content":[{"type":"input_text","text":"What is AI?"}]}],
		"max_tokens":64
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
	cacheReq := testRouter.lastSemanticCacheRequest
	if cacheReq.Model != "gpt-4o-mini" || cacheReq.OrganizationID != "org_1" || cacheReq.UserID != "user_1" || cacheReq.UserScoped {
		t.Fatalf("semantic cache identity/model not attached: %+v", cacheReq)
	}
	if !strings.Contains(cacheReq.Query, `"route":"/v1/responses"`) || !strings.Contains(cacheReq.Query, `"max_tokens":64`) || !strings.Contains(cacheReq.Query, "What is AI?") {
		t.Fatalf("semantic cache query should include canonical responses request context, got %s", cacheReq.Query)
	}
}

func TestResponsesHandlerBuildsMessagesFromInputArrayAndObjectText(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamBody struct {
		Messages []channel.Message `json:"messages"`
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"resp_test","object":"response","output_text":"ok"}`))
	}))
	t.Cleanup(upstream.Close)

	selectedChannel := &types.RouteChannel{
		Channel: &types.Channel{
			ID:       "ch-responses-input-array",
			Name:     "Responses Input Array",
			Provider: "openai",
			BaseURL:  upstream.URL,
			APIKey:   "sk-responses-input-array",
			Enabled:  true,
		},
		ChannelID: "ch-responses-input-array",
		Enabled:   true,
		Healthy:   true,
	}
	restoreRouter := setRouterForChatTest(&chatTestRouter{selected: selectedChannel})
	t.Cleanup(restoreRouter)

	handler := NewResponsesHandler(nil, &channel.OpenAIAdapter{})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{
		"model":"gpt-4o-mini",
		"input":[
			{"role":"user","content":[{"type":"input_text","text":"first"}]},
			{"role":"developer","content":"second"}
		]
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if len(upstreamBody.Messages) != 2 {
		t.Fatalf("messages len = %d, want 2: %+v", len(upstreamBody.Messages), upstreamBody.Messages)
	}
	if upstreamBody.Messages[0].Role != "user" || upstreamBody.Messages[0].Content != "first" {
		t.Fatalf("first message not derived from input object: %+v", upstreamBody.Messages[0])
	}
	if upstreamBody.Messages[1].Role != "developer" || upstreamBody.Messages[1].Content != "second" {
		t.Fatalf("second message not derived from input object: %+v", upstreamBody.Messages[1])
	}
}

func TestResponsesHandlerUsesSelectedGeminiAdapterAndNormalizesUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamPath string
	var upstreamAPIKey string
	var upstreamBody struct {
		Contents []struct {
			Role  string `json:"role"`
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"contents"`
		GenerationConfig struct {
			MaxOutputTokens int `json:"maxOutputTokens"`
		} `json:"generationConfig"`
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		upstreamAPIKey = r.Header.Get("x-goog-api-key")
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"candidates":[{"content":{"parts":[{"text":"responses gemini ok"}]}}],
			"usageMetadata":{"promptTokenCount":13,"candidatesTokenCount":8,"totalTokenCount":21}
		}`))
	}))
	t.Cleanup(upstream.Close)

	selectedChannel := &types.RouteChannel{
		Channel: &types.Channel{
			ID:       "ch-responses-gemini",
			Name:     "Responses Gemini",
			Provider: "gemini",
			BaseURL:  upstream.URL,
			APIKey:   "sk-gemini-responses",
			Enabled:  true,
		},
		ChannelID: "ch-responses-gemini",
		Enabled:   true,
		Healthy:   true,
	}
	testRouter := &chatTestRouter{selected: selectedChannel}
	restoreRouter := setRouterForChatTest(testRouter)
	t.Cleanup(restoreRouter)

	handler := NewResponsesHandler(nil, &channel.OpenAIAdapter{})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{
		"model":"gemini-1.5-flash",
		"messages":[{"role":"user","content":"ping responses gemini"}],
		"max_tokens":48
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if upstreamPath != "/v1beta/models/gemini-1.5-flash:generateContent" {
		t.Fatalf("upstream path = %q, want Gemini generateContent path", upstreamPath)
	}
	if upstreamAPIKey != "sk-gemini-responses" {
		t.Fatalf("x-goog-api-key = %q, want selected channel key", upstreamAPIKey)
	}
	if len(upstreamBody.Contents) != 1 || upstreamBody.Contents[0].Parts[0].Text != "ping responses gemini" {
		t.Fatalf("Gemini request body not built from Responses payload: %+v", upstreamBody.Contents)
	}
	if upstreamBody.GenerationConfig.MaxOutputTokens != 48 {
		t.Fatalf("maxOutputTokens = %d, want 48", upstreamBody.GenerationConfig.MaxOutputTokens)
	}
	if testRouter.lastProviderUsage == nil {
		t.Fatal("expected Gemini usageMetadata to be normalized into provider usage")
	}
	if testRouter.lastProviderUsage.PromptTokens != 13 || testRouter.lastProviderUsage.CompletionTokens != 8 || testRouter.lastProviderUsage.TotalTokens != 21 {
		t.Fatalf("unexpected normalized usage: %+v", testRouter.lastProviderUsage)
	}
	if !strings.Contains(rec.Body.String(), "responses gemini ok") {
		t.Fatalf("expected Gemini provider response body, got %s", rec.Body.String())
	}
}
