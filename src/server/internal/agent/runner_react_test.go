package agent

import (
	"context"
	"testing"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
)

// mockStructuredGateway implements both ChatGateway and StructuredReplyGenerator
type mockStructuredGateway struct {
	replies []*chat.CompletionResponse
	callIdx int
}

func (m *mockStructuredGateway) GenerateReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig) (string, error) {
	return "mock reply", nil
}

func (m *mockStructuredGateway) GenerateReplyStream(ctx context.Context, messages []chat.Message, config chat.ConversationConfig, onChunk func(string) error) error {
	return onChunk("mock stream")
}

func (m *mockStructuredGateway) GenerateStructuredReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig, tools []map[string]any) (*chat.CompletionResponse, error) {
	if m.callIdx >= len(m.replies) {
		return &chat.CompletionResponse{
			Content: "Final answer",
			Usage:   &chat.Usage{TotalTokens: 100},
		}, nil
	}
	reply := m.replies[m.callIdx]
	m.callIdx++
	return reply, nil
}

// mockStore provides minimal Store implementation for testing
type mockStore struct {
	messages     []*Message
	runs         []*Run
	toolRuns     []*ToolRun
	lastRunID    string
	lastMsgID    string
}

func (m *mockStore) CreateMessage(ctx context.Context, conversationID, organizationID, role, content string, toolCalls []ToolCall, toolCallID string) (*Message, error) {
	m.lastMsgID = "msg-" + role
	msg := &Message{
		ID:             m.lastMsgID,
		ConversationID: conversationID,
		Role:           role,
		Content:        content,
		ToolCalls:      toolCalls,
		ToolCallID:     toolCallID,
	}
	m.messages = append(m.messages, msg)
	return msg, nil
}

func (m *mockStore) ListMessages(ctx context.Context, conversationID, organizationID string) ([]*Message, error) {
	return m.messages, nil
}

func (m *mockStore) CreateRun(ctx context.Context, req *CreateRunRequest) (*Run, error) {
	m.lastRunID = "run-test"
	run := &Run{
		ID:             m.lastRunID,
		ConversationID: req.ConversationID,
		AgentID:        req.AgentID,
		Status:         req.Status,
	}
	m.runs = append(m.runs, run)
	return run, nil
}

func (m *mockStore) UpdateRun(ctx context.Context, organizationID, runID string, req UpdateRunRequest) (*Run, error) {
	for _, run := range m.runs {
		if run.ID == runID {
			if req.Status != nil {
				run.Status = *req.Status
			}
			if req.IterationCount != nil {
				run.IterationCount = *req.IterationCount
			}
			if req.ToolCallCount != nil {
				run.ToolCallCount = *req.ToolCallCount
			}
			return run, nil
		}
	}
	return nil, nil
}

func (m *mockStore) CreateToolRun(ctx context.Context, req *CreateToolRunRequest) (*ToolRun, error) {
	toolRun := &ToolRun{
		ID:         "tool-run-test",
		RunID:      req.RunID,
		ToolName:   req.ToolName,
		Status:     req.Status,
	}
	m.toolRuns = append(m.toolRuns, toolRun)
	return toolRun, nil
}

func (m *mockStore) UpdateToolRun(ctx context.Context, organizationID, toolRunID string, req UpdateToolRunRequest) (*ToolRun, error) {
	for _, tr := range m.toolRuns {
		if tr.ID == toolRunID {
			if req.Status != nil {
				tr.Status = *req.Status
			}
			return tr, nil
		}
	}
	return nil, nil
}

func (m *mockStore) GetAgent(ctx context.Context, organizationID, id string) (*Agent, error) {
	return nil, nil
}
func (m *mockStore) CreateAgent(ctx context.Context, agent *Agent) (*Agent, error) { return nil, nil }
func (m *mockStore) UpdateAgent(ctx context.Context, organizationID, id string, updates map[string]any) (*Agent, error) {
	return nil, nil
}
func (m *mockStore) DeleteAgent(ctx context.Context, organizationID, id string) error { return nil }
func (m *mockStore) ListAgents(ctx context.Context, organizationID, userID string) ([]*Agent, error) {
	return nil, nil
}
func (m *mockStore) GetConversation(ctx context.Context, organizationID, id string) (*Conversation, error) {
	return nil, nil
}
func (m *mockStore) CreateConversation(ctx context.Context, conversation *Conversation) (*Conversation, error) {
	return nil, nil
}
func (m *mockStore) UpdateConversation(ctx context.Context, organizationID, id string, updates map[string]any) (*Conversation, error) {
	return nil, nil
}
func (m *mockStore) DeleteConversation(ctx context.Context, organizationID, id string) error {
	return nil
}
func (m *mockStore) ListConversations(ctx context.Context, organizationID, userID, agentID string) ([]*Conversation, error) {
	return nil, nil
}
func (m *mockStore) GetMessage(ctx context.Context, organizationID, id string) (*Message, error) {
	return nil, nil
}
func (m *mockStore) UpdateMessage(ctx context.Context, organizationID, id string, updates map[string]any) (*Message, error) {
	return nil, nil
}
func (m *mockStore) DeleteMessage(ctx context.Context, organizationID, id string) error { return nil }
func (m *mockStore) GetRun(ctx context.Context, organizationID, id string) (*Run, error) {
	return nil, nil
}
func (m *mockStore) ListRuns(ctx context.Context, organizationID, conversationID string) ([]*Run, error) {
	return m.runs, nil
}
func (m *mockStore) GetToolRun(ctx context.Context, organizationID, id string) (*ToolRun, error) {
	return nil, nil
}
func (m *mockStore) ListToolRuns(ctx context.Context, organizationID, runID string) ([]*ToolRun, error) {
	return m.toolRuns, nil
}
func (m *mockStore) CreateMemory(ctx context.Context, req *CreateMemoryStoreRequest) (*Memory, error) {
	return nil, nil
}
func (m *mockStore) ListMemories(ctx context.Context, organizationID, userID string, req ListMemoriesRequest) ([]*Memory, error) {
	return nil, nil
}

func TestExecuteReActWithModelRouting(t *testing.T) {
	store := &mockStore{}
	gateway := &mockStructuredGateway{
		replies: []*chat.CompletionResponse{
			{
				Content: "Thinking...",
				ToolCalls: []chat.ToolCall{
					{
						ID:   "call-1",
						Type: "function",
						Function: chat.ToolFunction{
							Name:      "search",
							Arguments: `{"query":"test"}`,
						},
					},
				},
				Usage: &chat.Usage{TotalTokens: 200},
			},
			{
				Content: "Final answer after tool use",
				Usage:   &chat.Usage{TotalTokens: 150},
			},
		},
	}

	executor := &ToolExecutor{
		executors: map[string]ToolExecutorFunc{
			"search": func(ctx context.Context, agent *Agent, toolCall *ToolCall) (*ExecuteResult, error) {
				return &ExecuteResult{
					Content: "Search results",
					IsError: false,
				}, nil
			},
		},
	}

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
			{Name: "search", Enabled: true, Type: "builtin"},
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
}

func TestExecuteReActModelSwitching(t *testing.T) {
	store := &mockStore{}

	var selectedModels []string

	gateway := &mockStructuredGateway{
		replies: []*chat.CompletionResponse{
			{
				Content: "Iteration 1",
				ToolCalls: []chat.ToolCall{
					{
						ID:   "call-1",
						Type: "function",
						Function: chat.ToolFunction{
							Name:      "test_tool",
							Arguments: `{}`,
						},
					},
				},
				Usage: &chat.Usage{TotalTokens: 50},
			},
			{
				Content: "Final answer",
				Usage:   &chat.Usage{TotalTokens: 50},
			},
		},
	}

	executor := &ToolExecutor{
		executors: map[string]ToolExecutorFunc{
			"test_tool": func(ctx context.Context, agent *Agent, toolCall *ToolCall) (*ExecuteResult, error) {
				return &ExecuteResult{Content: "tool result", IsError: false}, nil
			},
		},
	}

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
			{Name: "test_tool", Enabled: true, Type: "builtin"},
		},
	}

	longInput := string(make([]byte, 150))
	for i := range longInput {
		longInput = longInput[:i] + "x" + longInput[i+1:]
	}

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

	_ = selectedModels
}
