package chat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHTTPReplyGeneratorDoesNotCallDirectProviderEndpoint(t *testing.T) {
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		t.Fatalf("direct provider endpoint should not be called: %s %s", r.Method, r.URL.Path)
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
	if reply != "Assistant reply: hello" {
		t.Fatalf("expected demo reply without provider call, got %s", reply)
	}
	if calls != 0 {
		t.Fatalf("direct provider endpoint was called %d times", calls)
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
