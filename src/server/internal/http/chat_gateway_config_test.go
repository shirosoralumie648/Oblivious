package http

import (
	"context"
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oblivious/server/internal/chat"
	"oblivious/server/internal/config"
)

func TestNewConfiguredChatGatewaysProductionRelayDisabledFailsClosed(t *testing.T) {
	replyGenerator, agentGateway := newConfiguredChatGateways(chatGatewayConfig("production", false))
	messages := []chat.Message{{Role: "user", Content: "hello"}}

	reply, err := replyGenerator.GenerateReply(context.Background(), messages, chat.ConversationConfig{})
	if !errors.Is(err, chat.ErrModelGatewayUnavailable) {
		t.Fatalf("expected production chat reply to fail closed when Relay is disabled, got reply=%q err=%v", reply, err)
	}
	if strings.Contains(reply, "Assistant reply") {
		t.Fatalf("production Relay-disabled reply must not use demo text, got %q", reply)
	}

	reply, err = agentGateway.GenerateReply(context.Background(), messages, chat.ConversationConfig{})
	if !errors.Is(err, chat.ErrModelGatewayUnavailable) {
		t.Fatalf("expected production agent gateway to fail closed when Relay is disabled, got reply=%q err=%v", reply, err)
	}
	if strings.Contains(reply, "Assistant reply") {
		t.Fatalf("production Relay-disabled agent gateway must not use demo text, got %q", reply)
	}

	var stream strings.Builder
	err = agentGateway.GenerateReplyStream(context.Background(), messages, chat.ConversationConfig{}, func(chunk string) error {
		_, _ = stream.WriteString(chunk)
		return nil
	})
	if !errors.Is(err, chat.ErrModelGatewayUnavailable) {
		t.Fatalf("expected production agent stream to fail closed when Relay is disabled, got stream=%q err=%v", stream.String(), err)
	}
	if strings.Contains(stream.String(), "Assistant reply") {
		t.Fatalf("production Relay-disabled stream must not use demo text, got %q", stream.String())
	}

	structuredGateway, ok := agentGateway.(chat.StructuredReplyGenerator)
	if !ok {
		t.Fatal("expected production agent gateway to expose structured replies for fail-closed tool runs")
	}
	structuredReply, err := structuredGateway.GenerateStructuredReply(context.Background(), messages, chat.ConversationConfig{}, nil)
	if !errors.Is(err, chat.ErrModelGatewayUnavailable) {
		t.Fatalf("expected production structured reply to fail closed when Relay is disabled, got reply=%+v err=%v", structuredReply, err)
	}
}

func TestNewConfiguredChatGatewaysDevelopmentRelayDisabledKeepsDemoFallback(t *testing.T) {
	replyGenerator, agentGateway := newConfiguredChatGateways(chatGatewayConfig("development", false))
	messages := []chat.Message{{Role: "user", Content: "hello"}}

	reply, err := replyGenerator.GenerateReply(context.Background(), messages, chat.ConversationConfig{})
	if err != nil {
		t.Fatalf("expected development chat demo reply, got err=%v", err)
	}
	if reply != "Assistant reply: hello" {
		t.Fatalf("expected development demo reply, got %q", reply)
	}

	var stream strings.Builder
	err = agentGateway.GenerateReplyStream(context.Background(), messages, chat.ConversationConfig{}, func(chunk string) error {
		_, _ = stream.WriteString(chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("expected development agent stream demo reply, got err=%v", err)
	}
	if stream.String() != "Assistant reply: hello" {
		t.Fatalf("expected development demo stream, got %q", stream.String())
	}

	structuredGateway, ok := agentGateway.(chat.StructuredReplyGenerator)
	if !ok {
		t.Fatal("expected development agent gateway to expose structured replies for local tool-run resumes")
	}
	structuredReply, err := structuredGateway.GenerateStructuredReply(context.Background(), messages, chat.ConversationConfig{}, nil)
	if err != nil {
		t.Fatalf("expected development structured demo reply, got err=%v", err)
	}
	if structuredReply.Content != "Assistant reply: hello" || structuredReply.FinishReason != "stop" || len(structuredReply.ToolCalls) != 0 {
		t.Fatalf("unexpected development structured reply: %+v", structuredReply)
	}
}

func TestNewConfiguredChatGatewaysUsesChatRelayBaseURL(t *testing.T) {
	var gotPath string
	relay := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"relay reply"},"finish_reason":"stop"}]}`))
	}))
	defer relay.Close()

	cfg := chatGatewayConfig("production", true)
	cfg.ChatRelayBaseURL = relay.URL + "/v1/"
	replyGenerator, _ := newConfiguredChatGateways(cfg)

	reply, err := replyGenerator.GenerateReply(context.Background(), []chat.Message{{Role: "user", Content: "hello"}}, chat.ConversationConfig{})
	if err != nil {
		t.Fatalf("expected chat relay reply, got err=%v", err)
	}
	if reply != "relay reply" {
		t.Fatalf("expected relay reply, got %q", reply)
	}
	if gotPath != "/v1/chat/completions" {
		t.Fatalf("expected configured chat relay base URL to be used without duplicate slash, got %q", gotPath)
	}
}

func chatGatewayConfig(env string, relayEnabled bool) config.Config {
	return config.Config{
		Env:               env,
		Port:              8080,
		RelayEnabled:      relayEnabled,
		RelayDefaultModel: "gpt-4o-mini",
		ModelDefaultName:  "demo-reply",
		LLMTimeoutMS:      30000,
	}
}
