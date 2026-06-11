package tools

import (
	"context"
	"testing"
)

func TestCallAgentToolRegistration(t *testing.T) {
	mock := &mockAgentService{
		createSubAgentRunFunc: func(ctx context.Context, req *SubAgentRunRequest) (*SubAgentRunResult, error) {
			return &SubAgentRunResult{
				FinalResponse: "Agent " + req.AgentID + " completed: " + req.RequestText,
				UsedTokens:    50,
			}, nil
		},
	}

	registry := NewRegistry()
	RegisterCallAgentTool(registry, mock)

	if !registry.Has("call_agent") {
		t.Fatal("call_agent tool not registered")
	}

	meta, executor, ok := registry.Get("call_agent")
	if !ok {
		t.Fatal("call_agent tool not found in registry")
	}

	if meta.Name != "call_agent" {
		t.Errorf("expected name 'call_agent', got %s", meta.Name)
	}
	if meta.Category != CategoryCustom {
		t.Errorf("expected category %s, got %s", CategoryCustom, meta.Category)
	}
	if meta.RiskLevel != "low" {
		t.Errorf("expected risk level 'low', got %s", meta.RiskLevel)
	}

	result, err := executor(context.Background(), map[string]any{
		"agentId":     "test-agent",
		"requestText": "analyze data",
		"maxDepth":    float64(3),
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content)
	}
	if result.Content != "Agent test-agent completed: analyze data" {
		t.Errorf("unexpected result: %s", result.Content)
	}
}

func TestCallAgentToolRegistration_RecursionLimit(t *testing.T) {
	mock := &mockAgentService{
		createSubAgentRunFunc: func(ctx context.Context, req *SubAgentRunRequest) (*SubAgentRunResult, error) {
			t.Error("should not call service when depth limit reached")
			return nil, nil
		},
	}

	registry := NewRegistry()
	RegisterCallAgentTool(registry, mock)

	_, executor, _ := registry.Get("call_agent")

	result, err := executor(context.Background(), map[string]any{
		"agentId":        "test-agent",
		"requestText":    "test",
		"recursionDepth": float64(3),
		"maxDepth":       float64(3),
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected error when recursion limit reached")
	}
	if result.Content != "max sub-agent depth 3 reached" {
		t.Errorf("unexpected error message: %s", result.Content)
	}
}
