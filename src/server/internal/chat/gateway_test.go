package chat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestToOpenAIMessagesPrependsPersonaAndSystemPrompt(t *testing.T) {
	messages := toOpenAIMessages([]Message{{Role: "user", Content: "hello"}}, ConversationConfig{
		SystemPromptOverride: "Follow the workspace policy.",
		PersonaRole:          "Product coach",
		PersonaStyle:         "Socratic",
		PersonaTone:          "Calm and direct",
		PersonaConstraints:   "Ask one question at a time.",
	})

	if len(messages) != 2 {
		t.Fatalf("expected system and user messages, got %+v", messages)
	}
	if messages[0].Role != "system" {
		t.Fatalf("expected first message to be system, got %+v", messages[0])
	}
	systemPrompt := messages[0].Content
	for _, want := range []string{
		"Follow the workspace policy.",
		"Role: Product coach",
		"Style: Socratic",
		"Tone: Calm and direct",
		"Constraints: Ask one question at a time.",
	} {
		if !strings.Contains(systemPrompt, want) {
			t.Fatalf("expected system prompt to contain %q, got %q", want, systemPrompt)
		}
	}
	if messages[1].Role != "user" || messages[1].Content != "hello" {
		t.Fatalf("expected user message to be preserved, got %+v", messages[1])
	}
}
