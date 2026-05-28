package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
	"oblivious/server/internal/mcp"
	"oblivious/server/internal/memory"
)

type fakeGateway struct {
	plainReply      string
	structured      []*chat.CompletionResponse
	lastMetadata    chat.RelayRequestMetadata
	plainCalls      int
	streamCalls     int
	structuredCalls int
	structIndex     int
	sawMetadata     bool
}

func (g *fakeGateway) GenerateReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig) (string, error) {
	g.plainCalls++
	g.lastMetadata, g.sawMetadata = chat.RelayRequestMetadataFromContext(ctx)
	return g.plainReply, nil
}

func (g *fakeGateway) GenerateReplyStream(ctx context.Context, messages []chat.Message, config chat.ConversationConfig, onChunk func(string) error) error {
	g.streamCalls++
	g.lastMetadata, g.sawMetadata = chat.RelayRequestMetadataFromContext(ctx)
	return onChunk(g.plainReply)
}

func (g *fakeGateway) GenerateStructuredReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig, tools []map[string]any) (*chat.CompletionResponse, error) {
	g.structuredCalls++
	g.lastMetadata, g.sawMetadata = chat.RelayRequestMetadataFromContext(ctx)
	if g.structIndex >= len(g.structured) {
		return &chat.CompletionResponse{Content: g.plainReply}, nil
	}
	resp := g.structured[g.structIndex]
	g.structIndex++
	return resp, nil
}

type fakeMemorySearcher struct {
	results []*memory.SearchResult
	calls   int
}

func (m *fakeMemorySearcher) Search(ctx context.Context, session auth.Session, req *memory.SearchRequest) ([]*memory.SearchResult, error) {
	m.calls++
	return m.results, nil
}

type fakeStore struct {
	agent        *Agent
	conversation *Conversation
	messages     []*Message
	runs         []*Run
	toolRuns     []*ToolRun
}

func (s *fakeStore) CreateAgent(ctx context.Context, userID, organizationID string, req *CreateAgentRequest) (*Agent, error) {
	panic("not used")
}

func (s *fakeStore) GetAgent(ctx context.Context, id, organizationID string) (*Agent, error) {
	if s.agent != nil && s.agent.ID == id && (organizationID == "" || s.agent.OrganizationID == "" || s.agent.OrganizationID == organizationID) {
		return s.agent, nil
	}
	return nil, nil
}

func (s *fakeStore) ListAgents(ctx context.Context, userID, organizationID string) ([]*Agent, error) {
	panic("not used")
}

func (s *fakeStore) UpdateAgent(ctx context.Context, id, organizationID string, req *UpdateAgentRequest) (*Agent, error) {
	panic("not used")
}

func (s *fakeStore) DeleteAgent(ctx context.Context, id, organizationID string) error {
	panic("not used")
}

func (s *fakeStore) CreateConversation(ctx context.Context, agentID, userID, organizationID string, title string) (*Conversation, error) {
	panic("not used")
}

func (s *fakeStore) GetConversation(ctx context.Context, id, organizationID string) (*Conversation, error) {
	if s.conversation != nil && s.conversation.ID == id && (organizationID == "" || s.conversation.OrganizationID == "" || s.conversation.OrganizationID == organizationID) {
		return s.conversation, nil
	}
	return nil, nil
}

func (s *fakeStore) ListConversations(ctx context.Context, agentID, userID, organizationID string) ([]*Conversation, error) {
	panic("not used")
}

func (s *fakeStore) DeleteConversation(ctx context.Context, id, organizationID string) error {
	panic("not used")
}

func (s *fakeStore) CreateMessage(ctx context.Context, conversationID, organizationID, role, content string, toolCalls []ToolCall, toolCallID string) (*Message, error) {
	msg := &Message{
		ID:             role + "-" + time.Now().UTC().Format("150405.000000"),
		ConversationID: conversationID,
		OrganizationID: organizationID,
		Role:           role,
		Content:        content,
		ToolCalls:      append([]ToolCall(nil), toolCalls...),
		ToolCallID:     toolCallID,
		CreatedAt:      time.Now().UTC(),
	}
	s.messages = append(s.messages, msg)
	return msg, nil
}

func (s *fakeStore) ListMessages(ctx context.Context, conversationID, organizationID string) ([]*Message, error) {
	result := make([]*Message, len(s.messages))
	copy(result, s.messages)
	return result, nil
}

func (s *fakeStore) CreateRun(ctx context.Context, req *CreateRunRequest) (*Run, error) {
	now := time.Now().UTC()
	startedAt := req.StartedAt
	if startedAt.IsZero() {
		startedAt = now
	}
	status := req.Status
	if status == "" {
		status = RunStatusRunning
	}
	run := &Run{
		ID:                "run_" + time.Now().UTC().Format("150405.000000"),
		OrganizationID:    req.OrganizationID,
		ConversationID:    req.ConversationID,
		AgentID:           req.AgentID,
		UserID:            req.UserID,
		RequestID:         req.RequestID,
		Status:            status,
		MemoryEnabled:     req.MemoryEnabled,
		MemorySearched:    req.MemorySearched,
		MemoryResultCount: req.MemoryResultCount,
		StartedAt:         startedAt,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	s.runs = append(s.runs, run)
	return run, nil
}

func (s *fakeStore) GetRun(ctx context.Context, organizationID, id string) (*Run, error) {
	for _, run := range s.runs {
		if run.ID == id && run.OrganizationID == organizationID {
			return run, nil
		}
	}
	return nil, nil
}

func (s *fakeStore) ListRuns(ctx context.Context, organizationID, conversationID string) ([]*Run, error) {
	var runs []*Run
	for _, run := range s.runs {
		if run.OrganizationID == organizationID && run.ConversationID == conversationID {
			runs = append(runs, run)
		}
	}
	return runs, nil
}

func (s *fakeStore) UpdateRun(ctx context.Context, organizationID, id string, req UpdateRunRequest) (*Run, error) {
	run, _ := s.GetRun(ctx, organizationID, id)
	if run == nil {
		return nil, errors.New("agent run not found")
	}
	if req.Status != nil {
		run.Status = *req.Status
	}
	if req.MemoryEnabled != nil {
		run.MemoryEnabled = *req.MemoryEnabled
	}
	if req.MemorySearched != nil {
		run.MemorySearched = *req.MemorySearched
	}
	if req.MemoryResultCount != nil {
		run.MemoryResultCount = *req.MemoryResultCount
	}
	if req.IterationCount != nil {
		run.IterationCount = *req.IterationCount
	}
	if req.ToolCallCount != nil {
		run.ToolCallCount = *req.ToolCallCount
	}
	if req.FinalMessageID != nil {
		run.FinalMessageID = *req.FinalMessageID
	}
	if req.Error != nil {
		run.Error = *req.Error
	}
	if req.CompletedAt != nil {
		run.CompletedAt = req.CompletedAt
	}
	run.UpdatedAt = time.Now().UTC()
	return run, nil
}

func (s *fakeStore) CreateToolRun(ctx context.Context, req *CreateToolRunRequest) (*ToolRun, error) {
	now := time.Now().UTC()
	status := req.Status
	if status == "" {
		status = ToolRunStatusRunning
	}
	approvalStatus := req.ApprovalStatus
	if approvalStatus == "" {
		approvalStatus = ApprovalStatusNotRequired
	}
	arguments := req.Arguments
	if arguments == nil {
		arguments = map[string]any{}
	}
	toolRun := &ToolRun{
		ID:                     "toolrun_" + time.Now().UTC().Format("150405.000000"),
		OrganizationID:         req.OrganizationID,
		RunID:                  req.RunID,
		ConversationID:         req.ConversationID,
		AgentID:                req.AgentID,
		ToolCallID:             req.ToolCallID,
		ToolName:               req.ToolName,
		ToolType:               req.ToolType,
		ServerID:               req.ServerID,
		Arguments:              arguments,
		Status:                 status,
		ApprovalStatus:         approvalStatus,
		ApprovedByUserID:       req.ApprovedByUserID,
		ApprovalDecisionReason: req.ApprovalDecisionReason,
		AttemptCount:           req.AttemptCount,
		ResultContent:          req.ResultContent,
		Error:                  req.Error,
		StartedAt:              req.StartedAt,
		CompletedAt:            req.CompletedAt,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	s.toolRuns = append(s.toolRuns, toolRun)
	return toolRun, nil
}

func (s *fakeStore) GetToolRun(ctx context.Context, organizationID, id string) (*ToolRun, error) {
	for _, toolRun := range s.toolRuns {
		if toolRun.ID == id && toolRun.OrganizationID == organizationID {
			return toolRun, nil
		}
	}
	return nil, nil
}

func (s *fakeStore) ListToolRuns(ctx context.Context, organizationID, runID string) ([]*ToolRun, error) {
	var toolRuns []*ToolRun
	for _, toolRun := range s.toolRuns {
		if toolRun.OrganizationID == organizationID && toolRun.RunID == runID {
			toolRuns = append(toolRuns, toolRun)
		}
	}
	return toolRuns, nil
}

func (s *fakeStore) UpdateToolRun(ctx context.Context, organizationID, id string, req UpdateToolRunRequest) (*ToolRun, error) {
	toolRun, _ := s.GetToolRun(ctx, organizationID, id)
	if toolRun == nil {
		return nil, errors.New("agent tool run not found")
	}
	if req.Status != nil {
		toolRun.Status = *req.Status
	}
	if req.ApprovalStatus != nil {
		toolRun.ApprovalStatus = *req.ApprovalStatus
	}
	if req.ApprovedByUserID != nil {
		toolRun.ApprovedByUserID = *req.ApprovedByUserID
	}
	if req.ApprovalDecisionReason != nil {
		toolRun.ApprovalDecisionReason = *req.ApprovalDecisionReason
	}
	if req.AttemptCount != nil {
		toolRun.AttemptCount = *req.AttemptCount
	}
	if req.ResultContent != nil {
		toolRun.ResultContent = *req.ResultContent
	}
	if req.Error != nil {
		toolRun.Error = *req.Error
	}
	if req.StartedAt != nil {
		toolRun.StartedAt = req.StartedAt
	}
	if req.ClearCompletedAt {
		toolRun.CompletedAt = nil
	} else if req.CompletedAt != nil {
		toolRun.CompletedAt = req.CompletedAt
	}
	toolRun.UpdatedAt = time.Now().UTC()
	return toolRun, nil
}

func TestServiceSendMessageUsesRunnerForToolEnabledAgents(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Tools: []Tool{
				{Name: "datetime", Type: "builtin", Enabled: true},
			},
		},
		conversation: &Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
	}

	gateway := &fakeGateway{
		plainReply: "plain path used",
		structured: []*chat.CompletionResponse{
			{
				ToolCalls: []chat.ToolCall{
					{
						ID:   "call_datetime",
						Type: "function",
						Function: chat.ToolFunction{
							Name:      "datetime",
							Arguments: "{}",
						},
					},
				},
				FinishReason: "tool_calls",
			},
			{
				Content:      "final answer after tool",
				FinishReason: "stop",
			},
		},
	}

	service := NewService(store, gateway)
	service.runner.config.MaxIterations = 4

	msg, err := service.SendMessage(context.Background(), auth.Session{
		User: auth.User{ID: "user_1"},
	}, "conv_1", "What time is it?")
	if err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}

	if msg.Content != "final answer after tool" {
		t.Fatalf("expected final tool-assisted answer, got %q", msg.Content)
	}
	if gateway.structuredCalls != 2 {
		t.Fatalf("expected tool path to use configured gateway twice, got %d calls", gateway.structuredCalls)
	}
	if gateway.plainCalls != 0 {
		t.Fatalf("expected structured tool path not to bypass into plain gateway calls, got %d", gateway.plainCalls)
	}

	var sawToolMessage bool
	for _, message := range store.messages {
		if message.Role == "tool" && message.ToolCallID == "call_datetime" {
			sawToolMessage = true
			break
		}
	}
	if !sawToolMessage {
		t.Fatalf("expected tool result message to be persisted, got %+v", store.messages)
	}
}

func TestServiceSendMessagePropagatesRelayMetadataThroughRunWithTools(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Tools: []Tool{
				{Name: "datetime", Type: "builtin", Enabled: true},
			},
		},
		conversation: &Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
	}
	gateway := &fakeGateway{
		structured: []*chat.CompletionResponse{
			{
				Content:      "metadata-aware answer",
				FinishReason: "stop",
			},
		},
	}
	service := NewService(store, gateway)

	msg, err := service.SendMessage(
		chat.WithRelayRequestMetadata(context.Background(), chat.RelayRequestMetadata{RequestID: "req_456"}),
		auth.Session{
			OrganizationID: "org_1",
			WorkspaceID:    "workspace_1",
			User:           auth.User{ID: "user_1"},
		},
		"conv_1",
		"Use metadata",
	)
	if err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	if msg.Content != "metadata-aware answer" {
		t.Fatalf("expected metadata-aware answer, got %q", msg.Content)
	}
	if gateway.structuredCalls != 1 {
		t.Fatalf("expected RunWithTools structured path, got %d calls", gateway.structuredCalls)
	}
	if !gateway.sawMetadata {
		t.Fatal("expected RunWithTools gateway call to receive Relay metadata")
	}
	if gateway.lastMetadata.UserID != "user_1" {
		t.Fatalf("expected user metadata user_1, got %q", gateway.lastMetadata.UserID)
	}
	if gateway.lastMetadata.WorkspaceID != "workspace_1" {
		t.Fatalf("expected workspace metadata workspace_1, got %q", gateway.lastMetadata.WorkspaceID)
	}
	if gateway.lastMetadata.OrganizationID != "org_1" {
		t.Fatalf("expected organization metadata org_1, got %q", gateway.lastMetadata.OrganizationID)
	}
	if gateway.lastMetadata.RequestID != "req_456" {
		t.Fatalf("expected request id req_456, got %q", gateway.lastMetadata.RequestID)
	}
}

func TestRunWithToolsCreatesDurableRunAndToolRuns(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Tools: []Tool{
				{Name: "datetime", Type: "builtin", Enabled: true},
			},
		},
		conversation: &Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
	}
	gateway := &fakeGateway{
		structured: []*chat.CompletionResponse{
			{
				ToolCalls: []chat.ToolCall{
					{ID: "call_datetime", Type: "function", Function: chat.ToolFunction{Name: "datetime", Arguments: "{}"}},
				},
				FinishReason: "tool_calls",
			},
			{
				Content:      "final answer after durable tool",
				FinishReason: "stop",
			},
		},
	}
	service := NewService(store, gateway)

	msg, err := service.SendMessage(
		chat.WithRelayRequestMetadata(context.Background(), chat.RelayRequestMetadata{RequestID: "req_durable_success"}),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		"conv_1",
		"What time is it?",
	)
	if err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	if msg == nil || msg.Content != "final answer after durable tool" {
		t.Fatalf("expected final assistant message, got %+v", msg)
	}
	if len(store.runs) != 1 {
		t.Fatalf("expected one durable run, got %+v", store.runs)
	}
	run := store.runs[0]
	if run.OrganizationID != "org_1" || run.ConversationID != "conv_1" || run.Status != RunStatusCompleted {
		t.Fatalf("unexpected durable run: %+v", run)
	}
	if run.RequestID != "req_durable_success" || run.FinalMessageID != msg.ID || run.IterationCount != 2 || run.ToolCallCount != 1 {
		t.Fatalf("run did not persist request/final/count evidence: %+v", run)
	}
	if len(store.toolRuns) != 1 {
		t.Fatalf("expected one durable tool run, got %+v", store.toolRuns)
	}
	toolRun := store.toolRuns[0]
	if toolRun.RunID != run.ID || toolRun.ToolCallID != "call_datetime" || toolRun.Status != ToolRunStatusCompleted {
		t.Fatalf("unexpected durable tool run: %+v", toolRun)
	}
	if toolRun.AttemptCount != 1 || toolRun.ResultContent == "" || toolRun.CompletedAt == nil {
		t.Fatalf("tool run did not persist attempt/result evidence: %+v", toolRun)
	}
}

func TestRunWithToolsPersistsFailedToolRun(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Tools: []Tool{
				{Name: "web_search", Type: "builtin", Enabled: true},
			},
		},
		conversation: &Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
	}
	gateway := &fakeGateway{
		plainReply: "should not become final",
		structured: []*chat.CompletionResponse{
			{
				ToolCalls: []chat.ToolCall{
					{ID: "call_search", Type: "function", Function: chat.ToolFunction{Name: "web_search", Arguments: `{"query":"phase 26"}`}},
				},
				FinishReason: "tool_calls",
			},
		},
	}
	service := NewService(store, gateway)

	_, err := service.SendMessage(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, "conv_1", "search")
	if err == nil {
		t.Fatal("expected failed tool run error, got nil")
	}
	if len(store.runs) != 1 || store.runs[0].Status != RunStatusFailed || !strings.Contains(strings.ToLower(store.runs[0].Error), "web_search") {
		t.Fatalf("expected failed durable run with web_search error, got %+v", store.runs)
	}
	if len(store.toolRuns) != 1 {
		t.Fatalf("expected one failed tool run, got %+v", store.toolRuns)
	}
	toolRun := store.toolRuns[0]
	if toolRun.Status != ToolRunStatusFailed || toolRun.AttemptCount != 1 || !strings.Contains(strings.ToLower(toolRun.Error), "disabled") {
		t.Fatalf("expected failed disabled tool evidence, got %+v", toolRun)
	}
	for _, message := range store.messages {
		if message.Role == "assistant" && message.Content == "should not become final" {
			t.Fatalf("failed tool run should not claim a final assistant message: %+v", store.messages)
		}
	}
}

func TestRunWithToolsPausesForApprovalRequiredTool(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Tools: []Tool{
				{Name: "datetime", Type: "builtin", Enabled: true, RequiresApproval: true},
			},
		},
		conversation: &Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
	}
	gateway := &fakeGateway{
		structured: []*chat.CompletionResponse{
			{
				ToolCalls: []chat.ToolCall{
					{ID: "call_datetime_approval", Type: "function", Function: chat.ToolFunction{Name: "datetime", Arguments: "{}"}},
				},
				FinishReason: "tool_calls",
			},
		},
	}
	service := NewService(store, gateway)

	_, err := service.SendMessage(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, "conv_1", "needs approval")
	if !errors.Is(err, ErrToolApprovalRequired) {
		t.Fatalf("expected ErrToolApprovalRequired, got %v", err)
	}
	if len(store.runs) != 1 || store.runs[0].Status != RunStatusPendingApproval {
		t.Fatalf("expected pending approval run, got %+v", store.runs)
	}
	if len(store.toolRuns) != 1 {
		t.Fatalf("expected one pending approval tool run, got %+v", store.toolRuns)
	}
	toolRun := store.toolRuns[0]
	if toolRun.Status != ToolRunStatusPendingApproval || toolRun.ApprovalStatus != ApprovalStatusPending || toolRun.AttemptCount != 0 {
		t.Fatalf("expected pending approval tool evidence without attempts, got %+v", toolRun)
	}
	for _, message := range store.messages {
		if message.Role == "tool" {
			t.Fatalf("approval-required tool executed before approval: %+v", store.messages)
		}
	}
}

func TestRunWithToolsRecordsMemoryEvidence(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config:         Config{EnableMemory: true},
			Tools: []Tool{
				{Name: "datetime", Type: "builtin", Enabled: true},
			},
		},
		conversation: &Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
	}
	gateway := &fakeGateway{
		structured: []*chat.CompletionResponse{
			{Content: "memory aware answer", FinishReason: "stop"},
		},
	}
	mem := &fakeMemorySearcher{results: []*memory.SearchResult{
		{DocumentID: "doc_1", ChunkContent: "first memory", Score: 0.91},
		{DocumentID: "doc_2", ChunkContent: "second memory", Score: 0.85},
	}}
	service := NewServiceWithMemory(store, gateway, nil, mem)

	msg, err := service.SendMessage(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, "conv_1", "remember this")
	if err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	if msg == nil || msg.Content != "memory aware answer" {
		t.Fatalf("expected memory answer, got %+v", msg)
	}
	if mem.calls != 1 {
		t.Fatalf("expected one memory search, got %d", mem.calls)
	}
	if len(store.runs) != 1 {
		t.Fatalf("expected one durable run, got %+v", store.runs)
	}
	run := store.runs[0]
	if !run.MemoryEnabled || !run.MemorySearched || run.MemoryResultCount != 2 {
		t.Fatalf("expected memory evidence on run, got %+v", run)
	}
}

// TestRunnerExhaustsIterationCap verifies that the Runner returns
// ErrMaxIterationsExceeded when the model never produces a final
// non-tool response within the iteration budget.
func TestRunnerExhaustsIterationCap(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:     "agent_1",
			UserID: "user_1",
			Model:  "gpt-4o-mini",
			Tools: []Tool{
				{Name: "datetime", Type: "builtin", Enabled: true},
			},
		},
		conversation: &Conversation{
			ID:      "conv_1",
			AgentID: "agent_1",
			UserID:  "user_1",
		},
	}

	// Gateway that always returns tool calls (never a final answer).
	gateway := &fakeGateway{
		structured: []*chat.CompletionResponse{
			{ToolCalls: []chat.ToolCall{{ID: "c1", Type: "function", Function: chat.ToolFunction{Name: "datetime", Arguments: "{}"}}}, FinishReason: "tool_calls"},
			{ToolCalls: []chat.ToolCall{{ID: "c2", Type: "function", Function: chat.ToolFunction{Name: "datetime", Arguments: "{}"}}}, FinishReason: "tool_calls"},
			{ToolCalls: []chat.ToolCall{{ID: "c3", Type: "function", Function: chat.ToolFunction{Name: "datetime", Arguments: "{}"}}}, FinishReason: "tool_calls"},
		},
	}

	service := NewService(store, gateway)
	service.runner.config.MaxIterations = 2

	_, err := service.SendMessage(context.Background(), auth.Session{
		User: auth.User{ID: "user_1"},
	}, "conv_1", "loop forever")
	if err == nil {
		t.Fatal("expected iteration cap error, got nil")
	}
	if !errors.Is(err, ErrMaxIterationsExceeded) {
		t.Fatalf("expected ErrMaxIterationsExceeded, got %v", err)
	}
}

// TestRunWithToolsStreaming verifies that the final assistant answer is
// streamed via the onChunk callback in word-level chunks.
func TestRunWithToolsStreaming(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:     "agent_1",
			UserID: "user_1",
			Model:  "gpt-4o-mini",
			Tools: []Tool{
				{Name: "datetime", Type: "builtin", Enabled: true},
			},
		},
		conversation: &Conversation{
			ID:      "conv_1",
			AgentID: "agent_1",
			UserID:  "user_1",
		},
	}

	gateway := &fakeGateway{
		structured: []*chat.CompletionResponse{
			{
				ToolCalls: []chat.ToolCall{
					{ID: "c1", Type: "function", Function: chat.ToolFunction{Name: "datetime", Arguments: "{}"}},
				},
				FinishReason: "tool_calls",
			},
			{
				Content:      "hello world",
				FinishReason: "stop",
			},
		},
	}

	service := NewService(store, gateway)
	service.runner.config.MaxIterations = 4

	var chunks []string
	err := service.SendMessageStream(context.Background(), auth.Session{
		User: auth.User{ID: "user_1"},
	}, "conv_1", "hi", func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("SendMessageStream returned error: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("expected streamed chunks, got none")
	}
	// Word-level chunking should produce at least 2 chunks for "hello world".
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 streamed chunks for 'hello world', got %d: %v", len(chunks), chunks)
	}
	combined := ""
	for _, c := range chunks {
		combined += c
	}
	if combined != "hello world" {
		t.Fatalf("expected streamed content 'hello world', got %q", combined)
	}

	var sawToolMessage bool
	for _, msg := range store.messages {
		if msg.Role == "tool" {
			sawToolMessage = true
			break
		}
	}
	if !sawToolMessage {
		t.Fatal("expected tool result message in streaming path")
	}
}

// TestRunWithToolsFallbackStreaming verifies that the non-structured
// gateway fallback path still calls onChunk when provided.
func TestRunWithToolsFallbackStreaming(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:     "agent_1",
			UserID: "user_1",
			Model:  "gpt-4o-mini",
			Tools: []Tool{
				{Name: "datetime", Type: "builtin", Enabled: true},
			},
		},
		conversation: &Conversation{
			ID:      "conv_1",
			AgentID: "agent_1",
			UserID:  "user_1",
		},
	}

	// Gateway that does NOT implement StructuredReplyGenerator
	gateway := &plainOnlyGateway{reply: "fallback reply"}

	service := NewService(store, gateway)
	service.runner.config.MaxIterations = 4

	var chunks []string
	err := service.SendMessageStream(context.Background(), auth.Session{
		User: auth.User{ID: "user_1"},
	}, "conv_1", "hi", func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("SendMessageStream returned error: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk in fallback path, got none")
	}
	if chunks[0] != "fallback reply" {
		t.Fatalf("expected 'fallback reply', got %q", chunks[0])
	}
}

// TestServiceSendMessagePlainPath verifies the non-tool plain chat path.
func TestServiceSendMessagePlainPath(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:     "agent_plain",
			UserID: "user_1",
			Model:  "gpt-4o-mini",
			Tools:  nil, // No tools.
		},
		conversation: &Conversation{
			ID:      "conv_plain",
			AgentID: "agent_plain",
			UserID:  "user_1",
		},
	}

	gateway := &fakeGateway{
		plainReply: "plain response with no tools",
	}

	service := NewService(store, gateway)
	msg, err := service.SendMessage(context.Background(), auth.Session{
		User: auth.User{ID: "user_1"},
	}, "conv_plain", "hello")
	if err != nil {
		t.Fatalf("SendMessage plain path returned error: %v", err)
	}
	if msg.Content != "plain response with no tools" {
		t.Fatalf("expected plain response, got %q", msg.Content)
	}
	if gateway.plainCalls != 1 {
		t.Fatalf("expected plain path to use configured Relay-compatible gateway once, got %d calls", gateway.plainCalls)
	}
}

// TestServiceSendMessageAllDisabledTools verifies that agents with all tools
// disabled take the plain path.
func TestServiceSendMessageAllDisabledTools(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:     "agent_disabled",
			UserID: "user_1",
			Model:  "gpt-4o-mini",
			Tools: []Tool{
				{Name: "datetime", Type: "builtin", Enabled: false},
				{Name: "web_search", Type: "builtin", Enabled: false},
			},
		},
		conversation: &Conversation{
			ID:      "conv_disabled",
			AgentID: "agent_disabled",
			UserID:  "user_1",
		},
	}

	gateway := &fakeGateway{
		plainReply: "plain path for disabled tools",
	}

	service := NewService(store, gateway)
	msg, err := service.SendMessage(context.Background(), auth.Session{
		User: auth.User{ID: "user_1"},
	}, "conv_disabled", "test")
	if err != nil {
		t.Fatalf("SendMessage with disabled tools returned error: %v", err)
	}
	if msg.Content != "plain path for disabled tools" {
		t.Fatalf("expected plain path reply, got %q", msg.Content)
	}
}

func TestToolDefinitionsFilterDisabledCommercialBuiltins(t *testing.T) {
	executor := NewToolExecutor(nil)
	agent := &Agent{
		ID: "agent_policy",
		Tools: []Tool{
			{Name: "web_search", Type: "builtin", Enabled: true},
			{Name: "http_request", Type: "builtin", Enabled: true},
			{Name: "calculator", Type: "builtin", Enabled: true},
			{Name: "datetime", Type: "builtin", Enabled: true},
		},
	}

	definitions, err := executor.GetToolDefinitions(context.Background(), agent)
	if err != nil {
		t.Fatalf("GetToolDefinitions returned error: %v", err)
	}

	names := make(map[string]bool, len(definitions))
	for _, definition := range definitions {
		names[definition.Name] = true
	}
	for _, disabled := range []string{"web_search", "http_request"} {
		if names[disabled] {
			t.Fatalf("disabled commercial builtin %s was exposed to the model: %+v", disabled, definitions)
		}
	}
	for _, enabled := range []string{"calculator", "datetime"} {
		if !names[enabled] {
			t.Fatalf("enabled commercial builtin %s was not exposed: %+v", enabled, definitions)
		}
	}
}

func TestListAvailableToolsFiltersDisabledCommercialBuiltins(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:     "agent_policy",
			UserID: "user_1",
			Tools: []Tool{
				{Name: "web_search", Type: "builtin", Enabled: true},
				{Name: "http_request", Type: "builtin", Enabled: true},
				{Name: "calculator", Type: "builtin", Enabled: true},
			},
		},
	}
	service := NewService(store, &fakeGateway{})

	definitions, err := service.ListAvailableTools(context.Background(), auth.Session{
		User: auth.User{ID: "user_1"},
	}, "agent_policy")
	if err != nil {
		t.Fatalf("ListAvailableTools returned error: %v", err)
	}

	names := make(map[string]bool, len(definitions))
	for _, definition := range definitions {
		names[definition.Name] = true
	}
	for _, disabled := range []string{"web_search", "http_request"} {
		if names[disabled] {
			t.Fatalf("disabled commercial builtin %s was exposed through ListAvailableTools: %+v", disabled, definitions)
		}
	}
	if !names["calculator"] {
		t.Fatalf("calculator should remain available, got %+v", definitions)
	}
}

func TestExecuteToolRejectsDisabledCommercialBuiltinBeforeCallingTool(t *testing.T) {
	recording := &recordingBuiltinTool{name: "web_search"}
	executor := &ToolExecutor{
		builtinTools: map[string]mcp.BuiltinTool{
			"web_search": recording,
		},
	}
	agent := &Agent{
		ID: "agent_policy",
		Tools: []Tool{
			{Name: "web_search", Type: "builtin", Enabled: true},
		},
	}

	result, err := executor.Execute(context.Background(), agent, &ToolCall{
		ID:        "call_search",
		Name:      "web_search",
		Arguments: map[string]any{"query": "commercial policy"},
	})
	if err != nil {
		t.Fatalf("Execute returned transport error: %v", err)
	}
	if recording.called {
		t.Fatal("disabled commercial builtin was called before executor rejected it")
	}
	if result == nil || !result.IsError {
		t.Fatalf("Execute result = %+v, want disabled tool error result", result)
	}
	if !strings.Contains(strings.ToLower(result.Content), "disabled") {
		t.Fatalf("Execute result content = %q, want disabled message", result.Content)
	}
}

func TestExecuteToolAllowsEnabledCommercialBuiltin(t *testing.T) {
	executor := NewToolExecutor(nil)
	agent := &Agent{
		ID: "agent_policy",
		Tools: []Tool{
			{Name: "calculator", Type: "builtin", Enabled: true},
		},
	}

	result, err := executor.Execute(context.Background(), agent, &ToolCall{
		ID:        "call_calculator",
		Name:      "calculator",
		Arguments: map[string]any{"expression": "2 + 3"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result == nil || result.IsError || result.Content != "Result: 5" {
		t.Fatalf("calculator result = %+v, want Result: 5", result)
	}
}

type recordingBuiltinTool struct {
	name   string
	called bool
}

func (t *recordingBuiltinTool) Name() string {
	return t.name
}

func (t *recordingBuiltinTool) Description() string {
	return "recording builtin"
}

func (t *recordingBuiltinTool) InputSchema() any {
	return map[string]any{"type": "object"}
}

func (t *recordingBuiltinTool) Execute(ctx context.Context, args map[string]any) (*mcp.ToolResult, error) {
	t.called = true
	return &mcp.ToolResult{Content: "called"}, nil
}

// TestSendMessageStreamPlainPath verifies streaming for non-tool agents.
func TestSendMessageStreamPlainPath(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:     "agent_stream",
			UserID: "user_1",
			Model:  "gpt-4o-mini",
		},
		conversation: &Conversation{
			ID:      "conv_stream",
			AgentID: "agent_stream",
			UserID:  "user_1",
		},
	}

	gateway := &fakeGateway{
		plainReply: "streaming plain reply",
	}

	service := NewService(store, gateway)
	var chunks []string
	err := service.SendMessageStream(context.Background(), auth.Session{
		User: auth.User{ID: "user_1"},
	}, "conv_stream", "hello", func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("SendMessageStream plain path returned error: %v", err)
	}
	if len(chunks) == 0 || chunks[0] != "streaming plain reply" {
		t.Fatalf("expected 'streaming plain reply' chunk, got %v", chunks)
	}
}

// plainOnlyGateway implements ChatGateway but NOT StructuredReplyGenerator.
type plainOnlyGateway struct {
	reply string
}

func (g *plainOnlyGateway) GenerateReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig) (string, error) {
	return g.reply, nil
}

func (g *plainOnlyGateway) GenerateReplyStream(ctx context.Context, messages []chat.Message, config chat.ConversationConfig, onChunk func(string) error) error {
	return onChunk(g.reply)
}
