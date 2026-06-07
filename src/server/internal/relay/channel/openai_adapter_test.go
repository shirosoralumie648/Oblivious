package channel

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oblivious/server/internal/relay/types"
)

func TestOpenAIAdapter_DoRequestBuildsAndExecutesProviderRequest(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotBody struct {
		Model    string `json:"model"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		MaxTokens int `json:"max_tokens"`
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode provider request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-adapter","usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	t.Cleanup(upstream.Close)

	adapter := NewOpenAIAdapter(upstream.URL, "sk-adapter")
	resp, err := adapter.DoRequest(context.Background(), &types.ProviderRequest{
		APIType: types.APITypeChat,
		Model:   "gpt-4o-mini",
		Messages: []types.Message{{
			Role:    "user",
			Content: "adapter request",
		}},
		MaxTokens: 7,
	})
	if err != nil {
		t.Fatalf("DoRequest returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("DoRequest returned nil response")
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", resp.StatusCode, body)
	}
	if gotPath != "/v1/chat/completions" {
		t.Fatalf("path = %q, want /v1/chat/completions", gotPath)
	}
	if gotAuth != "Bearer sk-adapter" {
		t.Fatalf("authorization = %q, want adapter API key", gotAuth)
	}
	if gotBody.Model != "gpt-4o-mini" || gotBody.MaxTokens != 7 {
		t.Fatalf("unexpected provider body: %+v", gotBody)
	}
	if len(gotBody.Messages) != 1 || gotBody.Messages[0].Content != "adapter request" {
		t.Fatalf("messages not preserved: %+v", gotBody.Messages)
	}
}

func TestOpenAIAdapter_DoRequestRequestsUsageForChatStreams(t *testing.T) {
	var gotBody struct {
		Model         string `json:"model"`
		Stream        bool   `json:"stream"`
		StreamOptions struct {
			IncludeUsage bool `json:"include_usage"`
		} `json:"stream_options"`
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode provider request: %v", err)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	t.Cleanup(upstream.Close)

	adapter := NewOpenAIAdapter(upstream.URL, "sk-stream-usage")
	resp, err := adapter.DoRequest(context.Background(), &types.ProviderRequest{
		APIType: types.APITypeChat,
		Model:   "gpt-4o-mini",
		Stream:  true,
		Messages: []types.Message{{
			Role:    "user",
			Content: "stream usage",
		}},
	})
	if err != nil {
		t.Fatalf("DoRequest returned error: %v", err)
	}
	if resp == nil {
		t.Fatal("DoRequest returned nil response")
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)

	if gotBody.Model != "gpt-4o-mini" || !gotBody.Stream {
		t.Fatalf("unexpected stream request body: %+v", gotBody)
	}
	if !gotBody.StreamOptions.IncludeUsage {
		t.Fatalf("expected stream_options.include_usage=true, got %+v", gotBody.StreamOptions)
	}
}

func TestOpenAIAdapter_DoRequestPreservesToolCallingPayload(t *testing.T) {
	var gotBody struct {
		Tools []map[string]any `json:"tools"`
		ToolChoice any `json:"tool_choice"`
		Messages []struct {
			Role string `json:"role"`
			Content string `json:"content"`
			ToolCalls []struct {
				ID string `json:"id"`
				Type string `json:"type"`
				Function struct {
					Name string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"messages"`
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode provider request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-tools","usage":{"prompt_tokens":8,"completion_tokens":1,"total_tokens":9}}`))
	}))
	t.Cleanup(upstream.Close)

	adapter := NewOpenAIAdapter(upstream.URL, "sk-tools")
	_, err := adapter.DoRequest(context.Background(), &types.ProviderRequest{
		APIType: types.APITypeChat,
		Model: "gpt-4o-mini",
		Messages: []types.Message{{
			Role: "assistant",
			ToolCalls: []types.ToolCall{{
				ID: "call_weather",
				Type: "function",
				Function: types.ToolFunction{Name: "weather.lookup", Arguments: `{"city":"Shanghai"}`},
			}},
		}},
		Tools: []map[string]any{{
			"type": "function",
			"function": map[string]any{
				"name": "weather.lookup",
			},
		}},
		ToolChoice: map[string]any{"type": "function"},
	})
	if err != nil {
		t.Fatalf("DoRequest returned error: %v", err)
	}

	if len(gotBody.Tools) != 1 || gotBody.Tools[0]["type"] != "function" {
		t.Fatalf("tools not preserved: %+v", gotBody.Tools)
	}
	if gotBody.ToolChoice == nil {
		t.Fatal("tool_choice should be preserved")
	}
	if len(gotBody.Messages) != 1 || len(gotBody.Messages[0].ToolCalls) != 1 {
		t.Fatalf("tool_calls not preserved: %+v", gotBody.Messages)
	}
	if gotBody.Messages[0].ToolCalls[0].Function.Name != "weather.lookup" {
		t.Fatalf("tool call function not preserved: %+v", gotBody.Messages[0].ToolCalls[0])
	}
}

func TestOpenAIAdapter_MapErrorParsesProviderError(t *testing.T) {
	adapter := &OpenAIAdapter{}

	providerErr := adapter.MapError(http.StatusUnauthorized, []byte(`{
		"error": {
			"message": "Incorrect API key provided",
			"type": "invalid_request_error",
			"code": "invalid_api_key"
		}
	}`))

	if providerErr == nil {
		t.Fatal("expected provider error")
	}
	if providerErr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", providerErr.StatusCode)
	}
	if providerErr.Code != "invalid_api_key" {
		t.Fatalf("code = %q, want invalid_api_key", providerErr.Code)
	}
	if !strings.Contains(providerErr.Message, "Incorrect API key") {
		t.Fatalf("message = %q, want upstream message", providerErr.Message)
	}
	if providerErr.Retryable {
		t.Fatal("401 invalid API key must not be retryable")
	}

	rateLimitErr := adapter.MapError(http.StatusTooManyRequests, []byte(`{"error":{"message":"rate limited"}}`))
	if rateLimitErr == nil || !rateLimitErr.Retryable {
		t.Fatalf("429 must be retryable, got %+v", rateLimitErr)
	}
}

func TestOpenAIAdapter_ConvertResponseExtractsUsage(t *testing.T) {
	adapter := &OpenAIAdapter{}

	resp, err := adapter.ConvertResponse([]byte(`{
		"id":"chatcmpl-usage",
		"usage":{"prompt_tokens":13,"completion_tokens":8,"total_tokens":21}
	}`), false)
	if err != nil {
		t.Fatalf("ConvertResponse returned error: %v", err)
	}
	if resp == nil || resp.Usage == nil {
		t.Fatalf("expected normalized usage, got %+v", resp)
	}
	if resp.Usage.PromptTokens != 13 || resp.Usage.CompletionTokens != 8 || resp.Usage.TotalTokens != 21 {
		t.Fatalf("unexpected usage: %+v", resp.Usage)
	}
}

func TestOpenAIAdapter_ConvertResponseExtractsUsageFromChatStream(t *testing.T) {
	adapter := &OpenAIAdapter{}

	resp, err := adapter.ConvertResponse([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"hel\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":2,\"total_tokens\":5}}\n\ndata: [DONE]\n\n"), true)
	if err != nil {
		t.Fatalf("ConvertResponse returned error: %v", err)
	}
	if resp == nil || resp.Usage == nil {
		t.Fatalf("expected streaming usage, got %+v", resp)
	}
	if resp.Usage.PromptTokens != 3 || resp.Usage.CompletionTokens != 2 || resp.Usage.TotalTokens != 5 {
		t.Fatalf("unexpected streaming usage: %+v", resp.Usage)
	}
}

func TestOpenAIAdapter_HealthCheckCallsModelsEndpoint(t *testing.T) {
	var gotPath string
	var gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"message":"maintenance"}}`))
	}))
	t.Cleanup(upstream.Close)

	adapter := NewOpenAIAdapter(upstream.URL, "sk-health")
	err := adapter.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("expected health check error for unhealthy upstream")
	}
	if gotPath != "/v1/models" {
		t.Fatalf("health path = %q, want /v1/models", gotPath)
	}
	if gotAuth != "Bearer sk-health" {
		t.Fatalf("health auth = %q, want adapter API key", gotAuth)
	}
	if !strings.Contains(err.Error(), "maintenance") {
		t.Fatalf("health error = %q, want upstream reason", err.Error())
	}
}

func TestOpenAIAdapter_EstimateUsageForSupportedRequests(t *testing.T) {
	adapter := &OpenAIAdapter{}

	tests := []struct {
		name   string
		req    *types.ProviderRequest
		assert func(t *testing.T, usage *types.Usage)
	}{
		{
			name: "chat tokens",
			req: &types.ProviderRequest{
				APIType: types.APITypeChat,
				Messages: []types.Message{
					{Role: "user", Content: "hello world"},
					{Role: "assistant", Content: "hi"},
				},
				MaxTokens: 25,
			},
			assert: func(t *testing.T, usage *types.Usage) {
				if usage.PromptTokens <= 0 || usage.CompletionTokens != 25 {
					t.Fatalf("unexpected chat usage: %+v", usage)
				}
			},
		},
		{
			name: "image count",
			req:  &types.ProviderRequest{APIType: types.APITypeImageGen},
			assert: func(t *testing.T, usage *types.Usage) {
				if usage.ImageCount != 1 {
					t.Fatalf("expected one image, got %+v", usage)
				}
			},
		},
		{
			name: "audio estimate",
			req:  &types.ProviderRequest{APIType: types.APITypeAudioSpeech, Input: "hello world"},
			assert: func(t *testing.T, usage *types.Usage) {
				if usage.AudioSeconds <= 0 {
					t.Fatalf("expected positive audio seconds, got %+v", usage)
				}
			},
		},
		{
			name: "moderation tokens",
			req:  &types.ProviderRequest{APIType: types.APITypeModeration, Input: "check this"},
			assert: func(t *testing.T, usage *types.Usage) {
				if usage.PromptTokens <= 0 {
					t.Fatalf("expected moderation prompt tokens, got %+v", usage)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usage := adapter.EstimateUsage(tt.req)
			if usage == nil {
				t.Fatal("EstimateUsage returned nil")
			}
			tt.assert(t, usage)
		})
	}
}
