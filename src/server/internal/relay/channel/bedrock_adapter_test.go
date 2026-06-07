package channel

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"oblivious/server/internal/relay/types"
)

func TestBedrockAdapter_DoRequestBuildsConverseRequest(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotBody struct {
		System []struct {
			Text string `json:"text"`
		} `json:"system"`
		Messages []struct {
			Role    string `json:"role"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"messages"`
		InferenceConfig struct {
			MaxTokens int `json:"maxTokens"`
		} `json:"inferenceConfig"`
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode provider request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"message":{"content":[{"text":"pong"}]}},"usage":{"inputTokens":4,"outputTokens":2,"totalTokens":6}}`))
	}))
	t.Cleanup(upstream.Close)

	adapter := NewBedrockAdapter(upstream.URL, "bedrock-key|us-east-1")
	resp, err := adapter.DoRequest(context.Background(), &types.ProviderRequest{
		APIType: types.APITypeChat,
		Model:   "claude-3-5-sonnet-20241022",
		Messages: []types.Message{
			{Role: "system", Content: "be exact"},
			{Role: "user", Content: "ping"},
			{Role: "assistant", Content: "pong draft"},
		},
		MaxTokens: 32,
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
	if gotPath != "/model/anthropic.claude-3-5-sonnet-20241022-v2:0/converse" {
		t.Fatalf("path = %q, want Bedrock Converse path", gotPath)
	}
	if gotAuth != "Bearer bedrock-key" {
		t.Fatalf("Authorization = %q, want Bedrock API key bearer token", gotAuth)
	}
	if len(gotBody.System) != 1 || gotBody.System[0].Text != "be exact" {
		t.Fatalf("unexpected system payload: %+v", gotBody.System)
	}
	if len(gotBody.Messages) != 2 {
		t.Fatalf("expected 2 non-system messages, got %+v", gotBody.Messages)
	}
	if gotBody.Messages[0].Role != "user" || gotBody.Messages[0].Content[0].Text != "ping" {
		t.Fatalf("unexpected user message payload: %+v", gotBody.Messages[0])
	}
	if gotBody.Messages[1].Role != "assistant" || gotBody.Messages[1].Content[0].Text != "pong draft" {
		t.Fatalf("unexpected assistant message payload: %+v", gotBody.Messages[1])
	}
	if gotBody.InferenceConfig.MaxTokens != 32 {
		t.Fatalf("maxTokens = %d, want 32", gotBody.InferenceConfig.MaxTokens)
	}
}

func TestBedrockAdapter_ConvertResponseExtractsUsage(t *testing.T) {
	adapter := NewBedrockAdapter("https://bedrock-runtime.us-east-1.amazonaws.com", "bedrock-key|us-east-1")

	resp, err := adapter.ConvertResponse([]byte(`{
		"output":{"message":{"content":[{"text":"pong"}]}},
		"usage":{"inputTokens":4,"outputTokens":2,"totalTokens":6}
	}`), false)
	if err != nil {
		t.Fatalf("ConvertResponse returned error: %v", err)
	}
	if resp == nil || resp.Usage == nil {
		t.Fatalf("expected normalized usage, got %+v", resp)
	}
	if resp.Usage.PromptTokens != 4 || resp.Usage.CompletionTokens != 2 || resp.Usage.TotalTokens != 6 {
		t.Fatalf("unexpected usage: %+v", resp.Usage)
	}
}
