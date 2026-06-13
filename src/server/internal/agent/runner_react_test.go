package agent

import (
	"context"
	"strings"
	"testing"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
)

// mockStructuredGateway implements both ChatGateway and StructuredReplyGenerator
type mockStructuredGateway struct {
	replies []*chat.CompletionResponse
	callIdx int
	models  []string
}

func (m *mockStructuredGateway) GenerateReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig) (string, error) {
	return "mock reply", nil
}

func (m *mockStructuredGateway) GenerateReplyStream(ctx context.Context, messages []chat.Message, config chat.ConversationConfig, onChunk func(string) error) error {
	return onChunk("mock stream")
}

func (m *mockStructuredGateway) GenerateStructuredReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig, tools []map[string]any) (*chat.CompletionResponse, error) {
	m.models = append(m.models, config.ModelID)
	if m.callIdx >= len(m.replies) {
		return &chat.CompletionResponse{
			Content: "Final answer",
			Usage:   &chat.CompletionUsage{TotalTokens: 100},
		}, nil
	}
	reply := m.replies[m.callIdx]
	m.callIdx++
	return reply, nil
}

func TestExecuteReActWithModelRouting(t *testing.T) {
	store := &fakeStore{}
	gateway := &mockStructuredGateway{
		replies: []*chat.CompletionResponse{
			{
				Content: "Thinking...",
				ToolCalls: []chat.ToolCall{
					{
						ID:   "call-1",
						Type: "function",
						Function: chat.ToolFunction{
							Name:      "calculator",
							Arguments: `{"expression":"2+3"}`,
						},
					},
				},
				Usage: &chat.CompletionUsage{TotalTokens: 200},
			},
			{
				Content: "Final answer after tool use",
				Usage:   &chat.CompletionUsage{TotalTokens: 150},
			},
		},
	}

	executor := NewToolExecutor(nil)

	routingRules := []ModelRoutingRule{
		{
			TargetModel:   "gpt-4",
			MinInputChars: 100,
		},
		{
			TargetModel:        "gpt-3.5-turbo",
			MinIteration:       2,
			RequiresToolResult: true,
		},
	}

	runner := &Runner{
		store:    store,
		gateway:  gateway,
		executor: executor,
		config:   DefaultRunnerConfig(),
		ModelRouter: &ModelRouter{
			Rules:    routingRules,
			Fallback: "gpt-4o-mini",
		},
	}

	agent := &Agent{
		ID:    "agent-1",
		Model: "gpt-4o-mini",
		Tools: []Tool{
			{Name: "calculator", Enabled: true, Type: "builtin", RiskLevel: ToolRiskSafe},
		},
	}

	req := &RunRequest{
		Session: auth.Session{
			OrganizationID: "org-1",
			User:           auth.User{ID: "user-1"},
		},
		Agent:             agent,
		ConversationID:    "conv-1",
		InputText:         "short query",
		ModelRoutingRules: routingRules,
	}

	result, err := runner.ExecuteReAct(context.Background(), req)
	if err != nil {
		t.Fatalf("ExecuteReAct failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if result.Message == nil {
		t.Fatal("Expected final message")
	}

	if result.ToolCalls != 1 {
		t.Errorf("Expected 1 tool call, got %d", result.ToolCalls)
	}

	if len(store.messages) < 3 {
		t.Errorf("Expected at least 3 messages (user, assistant with tool, tool result, final), got %d", len(store.messages))
	}

	if len(store.runs) != 1 {
		t.Errorf("Expected 1 run, got %d", len(store.runs))
	}

	if store.runs[0].Status != RunStatusCompleted {
		t.Errorf("Expected run status %s, got %s", RunStatusCompleted, store.runs[0].Status)
	}

	if len(gateway.models) < 2 || gateway.models[1] != "gpt-3.5-turbo" {
		t.Fatalf("expected second ReAct iteration to route to gpt-3.5-turbo, got %v", gateway.models)
	}
}

func TestExecuteReActModelSwitching(t *testing.T) {
	store := &fakeStore{}

	gateway := &mockStructuredGateway{
		replies: []*chat.CompletionResponse{
			{
				Content: "Iteration 1",
				ToolCalls: []chat.ToolCall{
					{
						ID:   "call-1",
						Type: "function",
						Function: chat.ToolFunction{
							Name:      "calculator",
							Arguments: `{"expression":"7*6"}`,
						},
					},
				},
				Usage: &chat.CompletionUsage{TotalTokens: 50},
			},
			{
				Content: "Final answer",
				Usage:   &chat.CompletionUsage{TotalTokens: 50},
			},
		},
	}

	executor := NewToolExecutor(nil)

	routingRules := []ModelRoutingRule{
		{
			TargetModel:        "claude-3-sonnet",
			MinIteration:       2,
			RequiresToolResult: true,
		},
	}

	runner := &Runner{
		store:    store,
		gateway:  gateway,
		executor: executor,
		config:   DefaultRunnerConfig(),
		ModelRouter: &ModelRouter{
			Rules:    routingRules,
			Fallback: "gpt-4o-mini",
		},
	}

	agent := &Agent{
		ID:    "agent-1",
		Model: "gpt-4o-mini",
		Tools: []Tool{
			{Name: "calculator", Enabled: true, Type: "builtin", RiskLevel: ToolRiskSafe},
		},
	}

	longInput := strings.Repeat("x", 150)

	req := &RunRequest{
		Session: auth.Session{
			OrganizationID: "org-1",
			User:           auth.User{ID: "user-1"},
		},
		Agent:             agent,
		ConversationID:    "conv-1",
		InputText:         longInput,
		ModelRoutingRules: routingRules,
	}

	result, err := runner.ExecuteReAct(context.Background(), req)
	if err != nil {
		t.Fatalf("ExecuteReAct failed: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}

	if len(gateway.models) < 2 || gateway.models[1] != "claude-3-sonnet" {
		t.Fatalf("expected second ReAct iteration to route to claude-3-sonnet, got %v", gateway.models)
	}
}
