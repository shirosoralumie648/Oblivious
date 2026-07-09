package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"oblivious/server/internal/relay/channel"
	"oblivious/server/internal/relay/types"
)

func TestChatStreamingProxiesSelectedProviderSSEThroughBillingRoute(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamPath string
	var upstreamAuth string
	var upstreamBody struct {
		Model    string `json:"model"`
		Stream   bool   `json:"stream"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		upstreamAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	t.Cleanup(upstream.Close)

	selectedChannel := &types.RouteChannel{
		Channel: &types.Channel{
			ID:       "ch-chat-stream",
			Name:     "Chat Stream",
			Provider: "openai",
			BaseURL:  upstream.URL,
			APIKey:   "sk-chat-stream",
			Enabled:  true,
		},
		ChannelID: "ch-chat-stream",
		Enabled:   true,
		Healthy:   true,
	}
	testRouter := &chatTestRouter{selected: selectedChannel}
	restoreRouter := setRouterForChatTest(testRouter)
	t.Cleanup(restoreRouter)

	handler := NewChatHandler(nil, &channel.OpenAIAdapter{})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"gpt-4o-mini",
		"stream":true,
		"messages":[{"role":"user","content":"stream hello"}]
	}`))
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
	if upstreamPath != "/v1/chat/completions" {
		t.Fatalf("upstream path = %q, want /v1/chat/completions", upstreamPath)
	}
	if upstreamAuth != "Bearer sk-chat-stream" {
		t.Fatalf("authorization = %q, want selected channel key", upstreamAuth)
	}
	if upstreamBody.Model != "gpt-4o-mini" || !upstreamBody.Stream {
		t.Fatalf("upstream stream request not preserved: %+v", upstreamBody)
	}
	if len(upstreamBody.Messages) != 1 || upstreamBody.Messages[0].Content != "stream hello" {
		t.Fatalf("upstream messages not preserved: %+v", upstreamBody.Messages)
	}
	if testRouter.routeWithBillingCalls != 1 {
		t.Fatalf("RouteWithBilling calls = %d, want 1", testRouter.routeWithBillingCalls)
	}
	if !testRouter.lastTrustedStreaming {
		t.Fatal("expected handler to mark streaming requests for router metering")
	}
	if testRouter.lastProviderUsage == nil {
		t.Fatal("expected streaming response usage to be parsed for billing settlement")
	}
	if testRouter.lastProviderUsage.PromptTokens != 1 || testRouter.lastProviderUsage.CompletionTokens != 1 || testRouter.lastProviderUsage.TotalTokens != 2 {
		t.Fatalf("unexpected streaming usage: %+v", testRouter.lastProviderUsage)
	}
	if !strings.Contains(rec.Body.String(), `data: {"choices":[{"delta":{"content":"hel"}}]}`) || !strings.Contains(rec.Body.String(), "data: [DONE]") {
		t.Fatalf("expected provider SSE body, got %s", rec.Body.String())
	}
}

func TestChatStreamingFlushesFirstProviderChunkBeforeUpstreamCompletes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	firstChunkWritten := make(chan struct{})
	releaseUpstream := make(chan struct{})
	var releaseOnce sync.Once
	release := func() {
		releaseOnce.Do(func() { close(releaseUpstream) })
	}
	t.Cleanup(release)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&struct{}{}); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("upstream writer does not support flushing")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\n"))
		flusher.Flush()
		close(firstChunkWritten)
		<-releaseUpstream
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	t.Cleanup(upstream.Close)

	testRouter := &chatTestRouter{selected: &types.RouteChannel{
		Channel:   &types.Channel{ID: "ch-live-stream", Provider: "openai", BaseURL: upstream.URL, APIKey: "sk-live-stream", Enabled: true},
		ChannelID: "ch-live-stream",
		Enabled:   true,
		Healthy:   true,
	}}
	restoreRouter := setRouterForChatTest(testRouter)
	t.Cleanup(restoreRouter)

	handler := NewChatHandler(nil, &channel.OpenAIAdapter{})
	engine := gin.New()
	engine.POST("/v1/chat/completions", func(c *gin.Context) {
		if err := handler.Handle(c); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
	})
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(server.URL+"/v1/chat/completions", "application/json", strings.NewReader(`{
		"model":"gpt-4o-mini",
		"stream":true,
		"messages":[{"role":"user","content":"stream now"}]
	}`))
	if err != nil {
		t.Fatalf("post streaming chat: %v", err)
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	lineCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		line, err := reader.ReadString('\n')
		if err != nil {
			errCh <- err
			return
		}
		lineCh <- line
	}()

	select {
	case <-firstChunkWritten:
	case <-time.After(time.Second):
		t.Fatal("upstream did not write first chunk")
	}

	select {
	case line := <-lineCh:
		if !strings.Contains(line, `"content":"hel"`) {
			t.Fatalf("first streamed line = %q, want first upstream chunk", line)
		}
	case err := <-errCh:
		t.Fatalf("read first streamed line: %v", err)
	case <-time.After(time.Second):
		t.Fatal("handler did not flush first provider chunk before upstream completed")
	}

	release()
	rest, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read remaining stream: %v", err)
	}
	if !strings.Contains(string(rest), `"content":"lo"`) || !strings.Contains(string(rest), "data: [DONE]") {
		t.Fatalf("remaining stream missing final provider chunks: %s", string(rest))
	}
	if testRouter.lastProviderUsage == nil || testRouter.lastProviderUsage.TotalTokens != 2 {
		t.Fatalf("expected stream usage to be parsed after upstream completed, got %+v", testRouter.lastProviderUsage)
	}
}

func TestChatHandlerUsesSelectedRouteChannelForUpstreamRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamPath string
	var upstreamAuth string
	var upstreamBody struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		upstreamAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-selected","choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	t.Cleanup(upstream.Close)

	selectedChannel := &types.RouteChannel{
		Channel: &types.Channel{
			ID:       "ch-selected",
			Name:     "Selected upstream",
			Provider: "openai",
			BaseURL:  upstream.URL,
			APIKey:   "sk-selected",
			Enabled:  true,
		},
		ChannelID: "ch-selected",
		Enabled:   true,
		Healthy:   true,
	}
	restoreRouter := setRouterForChatTest(&chatTestRouter{selected: selectedChannel})
	t.Cleanup(restoreRouter)

	handler := NewChatHandler(nil, &channel.OpenAIAdapter{})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"gpt-4o-mini",
		"messages":[{"role":"user","content":"ping selected channel"}]
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if upstreamPath != "/v1/chat/completions" {
		t.Fatalf("upstream path = %q, want /v1/chat/completions", upstreamPath)
	}
	if upstreamAuth != "Bearer sk-selected" {
		t.Fatalf("upstream auth = %q, want selected channel key", upstreamAuth)
	}
	if upstreamBody.Model != "gpt-4o-mini" {
		t.Fatalf("upstream model = %q, want gpt-4o-mini", upstreamBody.Model)
	}
	if len(upstreamBody.Messages) != 1 || upstreamBody.Messages[0].Content != "ping selected channel" {
		t.Fatalf("upstream messages not preserved: %+v", upstreamBody.Messages)
	}
	if !strings.Contains(rec.Body.String(), "chatcmpl-selected") {
		t.Fatalf("expected provider response body to be returned, got %s", rec.Body.String())
	}
}

func TestChatHandlerPropagatesTrustedConversationIDToRouter(t *testing.T) {
	gin.SetMode(gin.TestMode)

	selectedChannel := &types.RouteChannel{
		Channel: &types.Channel{
			ID:       "ch-conversation",
			Name:     "Conversation upstream",
			Provider: "openai",
			BaseURL:  "http://provider.test",
			APIKey:   "sk-conversation",
			Enabled:  true,
		},
		ChannelID: "ch-conversation",
		Enabled:   true,
		Healthy:   true,
	}
	testRouter := &chatTestRouter{
		selected: selectedChannel,
		resp:     types.NewOKResponse([]byte(`{"id":"chatcmpl-conversation","choices":[{"message":{"role":"assistant","content":"ok"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`), &types.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2}),
	}
	restoreRouter := setRouterForChatTest(testRouter)
	t.Cleanup(restoreRouter)

	handler := NewChatHandler(nil, &channel.OpenAIAdapter{})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"gpt-4o-mini",
		"messages":[{"role":"user","content":"remember channel"}]
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Request.Header.Set(types.HeaderInternalAuth, types.SharedInternalToken)
	ctx.Request.Header.Set(types.HeaderInternalUserID, "user_1")
	ctx.Request.Header.Set(types.HeaderInternalOrganization, "org_1")
	ctx.Request.Header.Set(types.HeaderInternalConversation, "conversation_1")

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if testRouter.lastConversationID != "conversation_1" {
		t.Fatalf("router saw conversation id %q, want conversation_1", testRouter.lastConversationID)
	}
}

func TestChatHandlerAttachesSemanticCacheRequestToRouterContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testRouter := &chatTestRouter{
		selected: &types.RouteChannel{
			Channel:   &types.Channel{ID: "ch-cache-meta", Provider: "openai", BaseURL: "http://provider.test", APIKey: "sk-cache", Enabled: true},
			ChannelID: "ch-cache-meta",
			Enabled:   true,
			Healthy:   true,
		},
		resp: types.NewOKResponse([]byte(`{"id":"fresh-chat","choices":[{"message":{"role":"assistant","content":"fresh"}}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`), &types.Usage{PromptTokens: 1, CompletionTokens: 1, TotalTokens: 2}),
	}
	restoreRouter := setRouterForChatTest(testRouter)
	t.Cleanup(restoreRouter)

	handler := NewChatHandler(nil, &channel.OpenAIAdapter{})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"gpt-4o-mini",
		"messages":[
			{"role":"system","content":"answer as JSON"},
			{"role":"user","content":"What is AI?"}
		],
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
	if testRouter.routeWithBillingCalls != 1 {
		t.Fatalf("expected handler to route through billing, got %d calls", testRouter.routeWithBillingCalls)
	}
	cacheReq := testRouter.lastSemanticCacheRequest
	if cacheReq.Model != "gpt-4o-mini" || cacheReq.OrganizationID != "org_1" || cacheReq.UserID != "user_1" || cacheReq.UserScoped {
		t.Fatalf("semantic cache identity/model not attached: %+v", cacheReq)
	}
	if !strings.Contains(cacheReq.Query, `"role":"system"`) || !strings.Contains(cacheReq.Query, `"max_tokens":64`) || !strings.Contains(cacheReq.Query, "What is AI?") {
		t.Fatalf("semantic cache query should include canonical request context, got %s", cacheReq.Query)
	}
}

func TestChatHandlerAttachesSemanticCacheEmbeddingWhenConfigured(t *testing.T) {
	gin.SetMode(gin.TestMode)

	testRouter := &chatTestRouter{
		selected: &types.RouteChannel{
			Channel:   &types.Channel{ID: "ch-cache-embedding", Provider: "openai", BaseURL: "http://provider.test", APIKey: "sk-cache", Enabled: true},
			ChannelID: "ch-cache-embedding",
			Enabled:   true,
			Healthy:   true,
		},
		resp: types.NewOKResponse([]byte(`{"id":"fresh-chat","choices":[{"message":{"role":"assistant","content":"fresh"}}]}`), nil),
	}
	restoreRouter := setRouterForChatTest(testRouter)
	t.Cleanup(restoreRouter)

	embedder := &semanticCacheTestEmbedder{embedding: []float32{0.1, 0.2, 0.3}}
	handler := NewChatHandlerWithSemanticCacheEmbedder(nil, &channel.OpenAIAdapter{}, embedder)
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"gpt-4o-mini",
		"messages":[{"role":"user","content":"What is semantic cache?"}]
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if len(embedder.texts) != 1 || !strings.Contains(embedder.texts[0], "What is semantic cache?") {
		t.Fatalf("expected embedder to receive canonical cache query, got %#v", embedder.texts)
	}
	if got := testRouter.lastSemanticCacheRequest.QueryEmbedding; len(got) != 3 || got[0] != 0.1 || got[2] != 0.3 {
		t.Fatalf("expected semantic cache embedding on routed request, got %+v", got)
	}
}

func TestChatHandlerMarksSensitiveSemanticCacheRequestsUserScoped(t *testing.T) {
	req := &channel.ProviderRequest{
		APIType:  types.APITypeChat,
		Model:    "gpt-4o-mini",
		Messages: []types.Message{{Role: "user", Content: "我的账户余额是多少"}},
	}
	ctx := types.WithTrustedUserID(context.Background(), "user_1")
	ctx = types.WithTrustedOrganizationID(ctx, "org_1")

	cacheReq, ok := semanticCacheRequestFromChat(ctx, req)
	if !ok {
		t.Fatal("expected sensitive chat request to be cacheable")
	}
	if !cacheReq.UserScoped {
		t.Fatalf("expected sensitive cache request to be user scoped, got %+v", cacheReq)
	}
}

func TestChatHandlerSemanticCacheKeyIncludesSystemPrompt(t *testing.T) {
	baseReq := &channel.ProviderRequest{
		APIType:   types.APITypeChat,
		Model:     "gpt-4o-mini",
		Messages:  []types.Message{{Role: "system", Content: "answer as JSON"}, {Role: "user", Content: "What is AI?"}},
		MaxTokens: 64,
	}
	otherReq := &channel.ProviderRequest{
		APIType:   types.APITypeChat,
		Model:     "gpt-4o-mini",
		Messages:  []types.Message{{Role: "system", Content: "answer as haiku"}, {Role: "user", Content: "What is AI?"}},
		MaxTokens: 64,
	}
	baseCacheReq, ok := semanticCacheRequestFromChat(context.Background(), baseReq)
	if !ok {
		t.Fatal("expected base chat request to be cacheable")
	}
	otherCacheReq, ok := semanticCacheRequestFromChat(context.Background(), otherReq)
	if !ok {
		t.Fatal("expected other chat request to be cacheable")
	}
	if baseCacheReq.Query == otherCacheReq.Query {
		t.Fatalf("semantic cache query must include system prompt; both were %s", baseCacheReq.Query)
	}
}

func TestChatHandlerPreservesProviderErrorStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"bad upstream key","code":"invalid_api_key"}}`))
	}))
	t.Cleanup(upstream.Close)

	selectedChannel := &types.RouteChannel{
		Channel: &types.Channel{
			ID:       "ch-error",
			Name:     "Error upstream",
			Provider: "openai",
			BaseURL:  upstream.URL,
			APIKey:   "sk-error",
			Enabled:  true,
		},
		ChannelID: "ch-error",
		Enabled:   true,
		Healthy:   true,
	}
	restoreRouter := setRouterForChatTest(&chatTestRouter{selected: selectedChannel})
	t.Cleanup(restoreRouter)

	handler := NewChatHandler(nil, &channel.OpenAIAdapter{})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"gpt-4o-mini",
		"messages":[{"role":"user","content":"trigger provider auth error"}]
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusUnauthorized, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "invalid_api_key") {
		t.Fatalf("expected upstream error body, got %s", rec.Body.String())
	}
}

func TestChatHandlerPreservesProviderErrorWhenBillingRefunds(t *testing.T) {
	gin.SetMode(gin.TestMode)

	restoreRouter := setRouterForChatTest(&providerErrorRouter{
		resp: &types.ProviderResponse{
			StatusCode: http.StatusTooManyRequests,
			Content:    []byte(`{"error":{"message":"provider quota exceeded","code":"rate_limit_exceeded"}}`),
			Error: &types.ProviderError{
				Code:       "rate_limit_exceeded",
				Message:    "provider quota exceeded",
				StatusCode: http.StatusTooManyRequests,
				Retryable:  true,
			},
		},
		err: errors.New("billing refund completed: provider error response status 429"),
	})
	t.Cleanup(restoreRouter)

	handler := NewChatHandler(nil, &channel.OpenAIAdapter{})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"gpt-4o-mini",
		"messages":[{"role":"user","content":"hit provider quota"}]
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusTooManyRequests, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "rate_limit_exceeded") {
		t.Fatalf("expected upstream error body, got %s", rec.Body.String())
	}
}

func TestChatHandlerUsesClaudeAdapterForSelectedRouteChannel(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var upstreamPath string
	var upstreamAPIKey string
	var upstreamVersion string
	var upstreamAuth string
	var upstreamBody struct {
		Model     string `json:"model"`
		MaxTokens int    `json:"max_tokens"`
		Messages  []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamPath = r.URL.Path
		upstreamAPIKey = r.Header.Get("x-api-key")
		upstreamVersion = r.Header.Get("anthropic-version")
		upstreamAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&upstreamBody); err != nil {
			t.Fatalf("decode upstream body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_claude","type":"message","role":"assistant","content":[{"type":"text","text":"ok"}],"usage":{"input_tokens":5,"output_tokens":7}}`))
	}))
	t.Cleanup(upstream.Close)

	selectedChannel := &types.RouteChannel{
		Channel: &types.Channel{
			ID:       "ch-claude",
			Name:     "Claude upstream",
			Provider: "claude",
			BaseURL:  upstream.URL,
			APIKey:   "sk-claude",
			Enabled:  true,
		},
		ChannelID: "ch-claude",
		Enabled:   true,
		Healthy:   true,
	}
	restoreRouter := setRouterForChatTest(&chatTestRouter{selected: selectedChannel})
	t.Cleanup(restoreRouter)

	handler := NewChatHandler(nil, &channel.OpenAIAdapter{})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"claude-3-5-sonnet-latest",
		"messages":[{"role":"user","content":"ping claude"}],
		"max_tokens":64
	}`))
	ctx.Request.Header.Set("Content-Type", "application/json")

	if err := handler.Handle(ctx); err != nil {
		t.Fatalf("Handle returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if upstreamPath != "/v1/messages" {
		t.Fatalf("upstream path = %q, want /v1/messages", upstreamPath)
	}
	if upstreamAPIKey != "sk-claude" {
		t.Fatalf("x-api-key = %q, want selected channel key", upstreamAPIKey)
	}
	if upstreamVersion != "2023-06-01" {
		t.Fatalf("anthropic-version = %q, want default Claude API version", upstreamVersion)
	}
	if upstreamAuth != "" {
		t.Fatalf("authorization header should not be used for Claude, got %q", upstreamAuth)
	}
	if upstreamBody.Model != "claude-3-5-sonnet-latest" || upstreamBody.MaxTokens != 64 {
		t.Fatalf("unexpected Claude body: %+v", upstreamBody)
	}
	if len(upstreamBody.Messages) != 1 || upstreamBody.Messages[0].Role != "user" || upstreamBody.Messages[0].Content != "ping claude" {
		t.Fatalf("Claude messages not preserved: %+v", upstreamBody.Messages)
	}
	if !strings.Contains(rec.Body.String(), "msg_claude") {
		t.Fatalf("expected Claude response body to be returned, got %s", rec.Body.String())
	}
}

func TestChatHandlerUsesGeminiAdapterAndNormalizesUsage(t *testing.T) {
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
			"candidates":[{"content":{"parts":[{"text":"gemini ok"}]}}],
			"usageMetadata":{"promptTokenCount":11,"candidatesTokenCount":7,"totalTokenCount":18}
		}`))
	}))
	t.Cleanup(upstream.Close)

	selectedChannel := &types.RouteChannel{
		Channel: &types.Channel{
			ID:       "ch-gemini",
			Name:     "Gemini upstream",
			Provider: "gemini",
			BaseURL:  upstream.URL,
			APIKey:   "sk-gemini",
			Enabled:  true,
		},
		ChannelID: "ch-gemini",
		Enabled:   true,
		Healthy:   true,
	}
	testRouter := &chatTestRouter{selected: selectedChannel}
	restoreRouter := setRouterForChatTest(testRouter)
	t.Cleanup(restoreRouter)

	handler := NewChatHandler(nil, &channel.OpenAIAdapter{})
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"gemini-1.5-flash",
		"messages":[
			{"role":"system","content":"be concise"},
			{"role":"user","content":"ping gemini"}
		],
		"max_tokens":32
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
	if upstreamAPIKey != "sk-gemini" {
		t.Fatalf("x-goog-api-key = %q, want selected channel key", upstreamAPIKey)
	}
	if len(upstreamBody.Contents) != 2 || upstreamBody.Contents[0].Role != "user" {
		t.Fatalf("Gemini contents not built correctly: %+v", upstreamBody.Contents)
	}
	if upstreamBody.GenerationConfig.MaxOutputTokens != 32 {
		t.Fatalf("maxOutputTokens = %d, want 32", upstreamBody.GenerationConfig.MaxOutputTokens)
	}
	if testRouter.lastProviderUsage == nil {
		t.Fatal("expected Gemini usageMetadata to be normalized into provider usage")
	}
	if testRouter.lastProviderUsage.PromptTokens != 11 || testRouter.lastProviderUsage.CompletionTokens != 7 || testRouter.lastProviderUsage.TotalTokens != 18 {
		t.Fatalf("unexpected normalized usage: %+v", testRouter.lastProviderUsage)
	}
	if !strings.Contains(rec.Body.String(), "gemini ok") {
		t.Fatalf("expected Gemini response body to be returned, got %s", rec.Body.String())
	}
}

type chatTestRouter struct {
	selected                 *types.RouteChannel
	lastProviderUsage        *types.Usage
	lastPrebillUsage         *types.Usage
	lastConversationID       string
	lastSemanticCacheRequest types.SemanticCacheRequest
	lastTrustedStreaming     bool
	routeWithBillingCalls    int
	resp                     *types.ProviderResponse
}

func (r *chatTestRouter) Route(_ context.Context, _ string, fn func(ch *types.RouteChannel) (*types.ProviderResponse, error)) (*types.ProviderResponse, error) {
	return fn(r.selected)
}

func (r *chatTestRouter) RouteWithBilling(ctx context.Context, _ types.APIType, _, _, _ string, usage *types.Usage, fn func(ch *types.RouteChannel) (*types.ProviderResponse, error)) (*types.ProviderResponse, error) {
	r.routeWithBillingCalls++
	r.lastPrebillUsage = usage
	r.lastConversationID, _ = types.TrustedConversationIDFromContext(ctx)
	r.lastSemanticCacheRequest, _ = types.SemanticCacheRequestFromContext(ctx)
	r.lastTrustedStreaming, _ = types.TrustedStreamingFromContext(ctx)
	if r.resp != nil {
		r.lastProviderUsage = r.resp.Usage
		return r.resp, nil
	}
	resp, err := fn(r.selected)
	if resp != nil {
		r.lastProviderUsage = resp.Usage
	}
	return resp, err
}

func (r *chatTestRouter) RecordChannelSuccess(_ string) {}

func (r *chatTestRouter) RecordChannelFailure(_ string) {}

func setRouterForChatTest(router types.RouterInterface) func() {
	previous := GetRouter()
	SetRouter(router)
	return func() {
		SetRouter(previous)
	}
}

type providerErrorRouter struct {
	resp *types.ProviderResponse
	err  error
}

func (r *providerErrorRouter) Route(_ context.Context, _ string, _ func(ch *types.RouteChannel) (*types.ProviderResponse, error)) (*types.ProviderResponse, error) {
	return r.resp, r.err
}

func (r *providerErrorRouter) RouteWithBilling(_ context.Context, _ types.APIType, _, _, _ string, _ *types.Usage, _ func(ch *types.RouteChannel) (*types.ProviderResponse, error)) (*types.ProviderResponse, error) {
	return r.resp, r.err
}

type semanticCacheTestEmbedder struct {
	embedding []float32
	err       error
	texts     []string
}

func (e *semanticCacheTestEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	e.texts = append(e.texts, text)
	if e.err != nil {
		return nil, e.err
	}
	return append([]float32(nil), e.embedding...), nil
}

func (r *providerErrorRouter) RecordChannelSuccess(_ string) {}

func (r *providerErrorRouter) RecordChannelFailure(_ string) {}

type countingRouter struct {
	routeWithBillingCalls int
}

func (r *countingRouter) Route(_ context.Context, _ string, _ func(ch *types.RouteChannel) (*types.ProviderResponse, error)) (*types.ProviderResponse, error) {
	return nil, errors.New("unexpected route call")
}

func (r *countingRouter) RouteWithBilling(_ context.Context, _ types.APIType, _, _, _ string, _ *types.Usage, _ func(ch *types.RouteChannel) (*types.ProviderResponse, error)) (*types.ProviderResponse, error) {
	r.routeWithBillingCalls++
	return nil, errors.New("unexpected route with billing call")
}

func (r *countingRouter) RecordChannelSuccess(_ string) {}

func (r *countingRouter) RecordChannelFailure(_ string) {}
