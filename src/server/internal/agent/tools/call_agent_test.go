package tools

import (
	"context"
	"errors"
	"testing"
)

type mockAgentService struct {
	createSubAgentRunFunc func(ctx context.Context, req *SubAgentRunRequest) (*SubAgentRunResult, error)
}

func (m *mockAgentService) CreateSubAgentRun(ctx context.Context, req *SubAgentRunRequest) (*SubAgentRunResult, error) {
	if m.createSubAgentRunFunc != nil {
		return m.createSubAgentRunFunc(ctx, req)
	}
	return nil, errors.New("not implemented")
}

func TestCallAgentTool_RecursionDepthGuard(t *testing.T) {
	mock := &mockAgentService{}
	tool := NewCallAgentTool(mock)

	ctx := context.Background()

	t.Run("blocks at max depth", func(t *testing.T) {
		input := CallAgentInput{
			AgentID:        "agent-123",
			RequestText:    "test",
			RecursionDepth: 5,
			MaxDepth:       5,
		}

		_, err := tool.Execute(ctx, input)
		if err == nil {
			t.Fatal("expected error when recursion depth >= max depth")
		}
		if err.Error() != "max sub-agent depth 5 reached" {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("allows execution below max depth", func(t *testing.T) {
		mock.createSubAgentRunFunc = func(ctx context.Context, req *SubAgentRunRequest) (*SubAgentRunResult, error) {
			if req.RecursionDepth != 3 {
				t.Errorf("expected recursion depth 3, got %d", req.RecursionDepth)
			}
			if req.MaxDepth != 5 {
				t.Errorf("expected max depth 5, got %d", req.MaxDepth)
			}
			return &SubAgentRunResult{
				FinalResponse: "completed",
				UsedTokens:    100,
			}, nil
		}

		input := CallAgentInput{
			AgentID:        "agent-123",
			RequestText:    "test request",
			RecursionDepth: 2,
			MaxDepth:       5,
		}

		output, err := tool.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if output.Result != "completed" {
			t.Errorf("expected result 'completed', got %s", output.Result)
		}
		if output.TokensUsed != 100 {
			t.Errorf("expected 100 tokens, got %d", output.TokensUsed)
		}
	})

	t.Run("increments recursion depth", func(t *testing.T) {
		mock.createSubAgentRunFunc = func(ctx context.Context, req *SubAgentRunRequest) (*SubAgentRunResult, error) {
			if req.RecursionDepth != 1 {
				t.Errorf("expected recursion depth incremented to 1, got %d", req.RecursionDepth)
			}
			return &SubAgentRunResult{FinalResponse: "ok", UsedTokens: 50}, nil
		}

		input := CallAgentInput{
			AgentID:        "agent-456",
			RequestText:    "nested call",
			RecursionDepth: 0,
			MaxDepth:       3,
		}

		_, err := tool.Execute(ctx, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
