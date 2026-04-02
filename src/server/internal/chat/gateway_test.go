package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGenerateReplyUsesOpenAICompatibleRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Fatalf("expected /chat/completions, got %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("expected Authorization header, got %q", got)
		}

		var payload struct {
			MaxTokens   int     `json:"max_tokens"`
			Messages    []struct {
				Content string `json:"content"`
				Role    string `json:"role"`
			} `json:"messages"`
			Model       string  `json:"model"`
			Temperature float64 `json:"temperature"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.Model != "quality-chat" {
			t.Fatalf("expected model quality-chat, got %s", payload.Model)
		}
		if payload.MaxTokens != 512 {
			t.Fatalf("expected max tokens 512, got %d", payload.MaxTokens)
		}
		if payload.Temperature != 0.7 {
			t.Fatalf("expected temperature 0.7, got %v", payload.Temperature)
		}
		if len(payload.Messages) != 3 {
			t.Fatalf("unexpected messages payload length: %+v", payload.Messages)
		}
		if payload.Messages[0].Role != "system" || payload.Messages[0].Content != "Be concise" {
			t.Fatalf("expected system prompt override, got %+v", payload.Messages[0])
		}
		if payload.Messages[1].Role != "system" || payload.Messages[1].Content != "Tools are enabled for this conversation." {
			t.Fatalf("expected tools enabled system marker, got %+v", payload.Messages[1])
		}
		if payload.Messages[2].Content != "hello" {
			t.Fatalf("unexpected user message payload: %+v", payload.Messages[2])
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": "real reply"}},
			},
		})
	}))
	defer server.Close()

	generator := NewHTTPReplyGenerator(server.URL, "test-key", "demo-reply", time.Second)
	reply, err := generator.GenerateReply(context.Background(), []Message{{Role: "user", Content: "hello"}}, ConversationConfig{
		ModelID:              "quality-chat",
		SystemPromptOverride: "Be concise",
		Temperature:          0.7,
		MaxOutputTokens:      512,
		ToolsEnabled:         true,
	})
	if err != nil {
		t.Fatalf("generate reply: %v", err)
	}
	if reply != "real reply" {
		t.Fatalf("expected real reply, got %s", reply)
	}
}

func TestGenerateReplyFallsBackToDemoWithoutProviderConfig(t *testing.T) {
	generator := NewHTTPReplyGenerator("", "", "demo-reply", time.Second)
	reply, err := generator.GenerateReply(context.Background(), []Message{{Role: "user", Content: "hello"}}, ConversationConfig{
		ModelID:         "quality-chat",
		Temperature:     1,
		MaxOutputTokens: 1024,
	})
	if err != nil {
		t.Fatalf("generate reply: %v", err)
	}
	if reply != "Assistant reply: hello" {
		t.Fatalf("expected demo fallback, got %s", reply)
	}
}
