package agent

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
	"oblivious/server/internal/mcp"
	"oblivious/server/internal/memory"
	"oblivious/server/internal/metrics"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

type fakeGateway struct {
	plainReply             string
	plainErr               error
	structured             []*chat.CompletionResponse
	lastMetadata           chat.RelayRequestMetadata
	plainCalls             int
	streamCalls            int
	structuredCalls        int
	structIndex            int
	sawMetadata            bool
	lastPlainMessages      []chat.Message
	lastStructuredMessages []chat.Message
}

func (g *fakeGateway) GenerateReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig) (string, error) {
	g.plainCalls++
	g.lastMetadata, g.sawMetadata = chat.RelayRequestMetadataFromContext(ctx)
	g.lastPlainMessages = append([]chat.Message(nil), messages...)
	if g.plainErr != nil {
		return "", g.plainErr
	}
	return g.plainReply, nil
}

func (g *fakeGateway) GenerateReplyStream(ctx context.Context, messages []chat.Message, config chat.ConversationConfig, onChunk func(string) error) error {
	g.streamCalls++
	g.lastMetadata, g.sawMetadata = chat.RelayRequestMetadataFromContext(ctx)
	g.lastPlainMessages = append([]chat.Message(nil), messages...)
	return onChunk(g.plainReply)
}

func (g *fakeGateway) GenerateStructuredReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig, tools []map[string]any) (*chat.CompletionResponse, error) {
	g.structuredCalls++
	g.lastMetadata, g.sawMetadata = chat.RelayRequestMetadataFromContext(ctx)
	g.lastStructuredMessages = append([]chat.Message(nil), messages...)
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

type fakeAgentMemoryEmbedder struct {
	embeddings map[string][]float32
	texts      []string
}

func (e *fakeAgentMemoryEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	e.texts = append(e.texts, text)
	if e.embeddings != nil {
		if embedding, ok := e.embeddings[text]; ok {
			return append([]float32(nil), embedding...), nil
		}
	}
	return []float32{0.5, 0.5}, nil
}

type fakeStore struct {
	agent                    *Agent
	conversation             *Conversation
	createMemoryEmbedding    []float32
	listMemoryAgentID        string
	listMemoryLimit          int
	listMemoryOrganizationID string
	listMemoryQuery          string
	searchMemoryAgentID      string
	searchMemoryEmbedding    []float32
	searchMemoryLimit        int
	searchMemoryMinScore     float64
	searchMemoryResults      []*MemorySearchResult
	listMemoryUserID         string
	memories                 []*Memory
	messages                 []*Message
	planSteps                []*PlanStep
	runs                     []*Run
	toolRuns                 []*ToolRun
	updateMemoryEmbedding    []float32
}

func (s *fakeStore) CreateAgent(ctx context.Context, userID, organizationID string, req *CreateAgentRequest) (*Agent, error) {
	agent := &Agent{
		ID:             "agent_created",
		OrganizationID: organizationID,
		UserID:         userID,
		Name:           req.Name,
		Description:    req.Description,
		Model:          req.Model,
		SystemPrompt:   req.SystemPrompt,
		Tools:          append([]Tool(nil), req.Tools...),
		Config:         req.Config,
		IsPublic:       req.IsPublic,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}
	s.agent = agent
	return agent, nil
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
	if s.agent == nil || s.agent.ID != id || s.agent.OrganizationID != organizationID {
		return nil, errors.New("agent not found")
	}
	if req.Name != nil {
		s.agent.Name = *req.Name
	}
	if req.Description != nil {
		s.agent.Description = *req.Description
	}
	if req.Model != nil {
		s.agent.Model = *req.Model
	}
	if req.SystemPrompt != nil {
		s.agent.SystemPrompt = *req.SystemPrompt
	}
	if req.Tools != nil {
		s.agent.Tools = append([]Tool(nil), req.Tools...)
	}
	if req.Config != nil {
		s.agent.Config = *req.Config
	}
	if req.IsPublic != nil {
		s.agent.IsPublic = *req.IsPublic
	}
	s.agent.UpdatedAt = time.Now().UTC()
	return s.agent, nil
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
		Mode:              NormalizeExecutionMode(req.Mode),
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
	if req.ClearCompletedAt {
		run.CompletedAt = nil
	} else if req.CompletedAt != nil {
		run.CompletedAt = req.CompletedAt
	} else if req.Status != nil && *req.Status != RunStatusCompleted && *req.Status != RunStatusFailed && *req.Status != RunStatusMaxIterationsReached && *req.Status != RunStatusTokenBudgetExceeded {
		run.CompletedAt = nil
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
		RiskLevel:              req.RiskLevel,
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

func (s *fakeStore) CreatePlanStep(ctx context.Context, req *CreatePlanStepRequest) (*PlanStep, error) {
	now := time.Now().UTC()
	status := req.Status
	if status == "" {
		status = PlanStepStatusPending
	}
	approvalStatus := req.ApprovalStatus
	if approvalStatus == "" {
		approvalStatus = ApprovalStatusNotRequired
	}
	input := req.Input
	if input == nil {
		input = map[string]any{}
	}
	step := &PlanStep{
		ID:             "planstep_" + time.Now().UTC().Format("150405.000000"),
		RunID:          req.RunID,
		OrganizationID: req.OrganizationID,
		Index:          req.Index,
		Title:          req.Title,
		Status:         status,
		ApprovalStatus: approvalStatus,
		ToolName:       req.ToolName,
		Input:          input,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	s.planSteps = append(s.planSteps, step)
	return step, nil
}

func (s *fakeStore) ListPlanSteps(ctx context.Context, organizationID, runID string) ([]*PlanStep, error) {
	var steps []*PlanStep
	for _, step := range s.planSteps {
		if step.OrganizationID == organizationID && step.RunID == runID {
			steps = append(steps, step)
		}
	}
	return steps, nil
}

func (s *fakeStore) GetPlanStep(ctx context.Context, organizationID, id string) (*PlanStep, error) {
	for _, step := range s.planSteps {
		if step.ID == id && step.OrganizationID == organizationID {
			return step, nil
		}
	}
	return nil, nil
}

func (s *fakeStore) UpdatePlanStep(ctx context.Context, organizationID, id string, req UpdatePlanStepRequest) (*PlanStep, error) {
	step, _ := s.GetPlanStep(ctx, organizationID, id)
	if step == nil {
		return nil, errors.New("agent plan step not found")
	}
	if req.Index != nil {
		step.Index = *req.Index
	}
	if req.Title != nil {
		step.Title = *req.Title
	}
	if req.Status != nil {
		step.Status = *req.Status
	}
	if req.ApprovalStatus != nil {
		step.ApprovalStatus = *req.ApprovalStatus
	}
	if req.ToolName != nil {
		step.ToolName = *req.ToolName
	}
	if req.ReplaceInput {
		step.Input = map[string]any{}
		for key, value := range req.Input {
			step.Input[key] = value
		}
	}
	if req.ResultContent != nil {
		step.ResultContent = *req.ResultContent
	}
	if req.Error != nil {
		step.Error = *req.Error
	}
	if req.StartedAt != nil {
		step.StartedAt = req.StartedAt
	}
	if req.ClearCompletedAt {
		step.CompletedAt = nil
	} else if req.CompletedAt != nil {
		step.CompletedAt = req.CompletedAt
	}
	step.UpdatedAt = time.Now().UTC()
	return step, nil
}

func (s *fakeStore) DeletePlanStep(ctx context.Context, organizationID, id string) (*PlanStep, error) {
	for index, step := range s.planSteps {
		if step.ID == id && step.OrganizationID == organizationID {
			s.planSteps = append(s.planSteps[:index], s.planSteps[index+1:]...)
			return step, nil
		}
	}
	return nil, errors.New("agent plan step not found")
}

type fakePlanStepExecutor struct {
	calls         int
	err           error
	resultContent string
	seenStep      *PlanStep
}

func TestServiceUpdatePlanStepDraftResetsApprovedStepForReview(t *testing.T) {
	session := auth.Session{
		User:           auth.User{ID: "user_1"},
		OrganizationID: "org_1",
	}
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusRunning,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          1,
			Title:          "Draft original implementation",
			Status:         PlanStepStatusApproved,
			ApprovalStatus: ApprovalStatusApproved,
			ToolName:       "write_file",
			Input:          map[string]any{"path": "old.go"},
		}},
	}
	service := NewService(store, &fakeGateway{})

	updated, err := service.UpdatePlanStepDraft(context.Background(), session, "step_1", UpdatePlanStepDraftRequest{
		Title:    stringPointer("Draft safer implementation"),
		ToolName: stringPointer("read_file"),
		Input:    map[string]any{"path": "new.go"},
	})
	if err != nil {
		t.Fatalf("UpdatePlanStepDraft returned error: %v", err)
	}

	if updated.Title != "Draft safer implementation" || updated.ToolName != "read_file" || updated.Input["path"] != "new.go" {
		t.Fatalf("expected edited plan step content, got %+v", updated)
	}
	if updated.Status != PlanStepStatusPending || updated.ApprovalStatus != ApprovalStatusPending {
		t.Fatalf("approved plan edits should require fresh review, got status=%s approval=%s", updated.Status, updated.ApprovalStatus)
	}
}

func TestServiceUpdatePlanStepDraftRejectsRunningStep(t *testing.T) {
	session := auth.Session{
		User:           auth.User{ID: "user_1"},
		OrganizationID: "org_1",
	}
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusRunning,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Title:          "Already running",
			Status:         PlanStepStatusRunning,
			ApprovalStatus: ApprovalStatusApproved,
		}},
	}
	service := NewService(store, &fakeGateway{})

	if _, err := service.UpdatePlanStepDraft(context.Background(), session, "step_1", UpdatePlanStepDraftRequest{
		Title: stringPointer("Too late"),
	}); err == nil || !strings.Contains(err.Error(), "cannot be adjusted") {
		t.Fatalf("expected running step edit rejection, got %v", err)
	}
}

func TestServiceMovePlanStepSwapsPendingStepsAndResetsApproval(t *testing.T) {
	session := auth.Session{
		User:           auth.User{ID: "user_1"},
		OrganizationID: "org_1",
	}
	now := time.Now().UTC()
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusPendingApproval,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          1,
			Title:          "Gather requirements",
			Status:         PlanStepStatusCompleted,
			ApprovalStatus: ApprovalStatusNotRequired,
			CreatedAt:      now,
			UpdatedAt:      now,
		}, {
			ID:             "step_2",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          2,
			Title:          "Draft patch",
			Status:         PlanStepStatusApproved,
			ApprovalStatus: ApprovalStatusApproved,
			CreatedAt:      now,
			UpdatedAt:      now,
		}, {
			ID:             "step_3",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          3,
			Title:          "Verify patch",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusPending,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	service := NewService(store, &fakeGateway{})

	steps, err := service.MovePlanStep(context.Background(), session, "step_3", MovePlanStepDirectionUp)
	if err != nil {
		t.Fatalf("MovePlanStep returned error: %v", err)
	}

	if len(steps) != 3 {
		t.Fatalf("expected refreshed plan steps, got %+v", steps)
	}
	if steps[0].ID != "step_1" || steps[0].Index != 1 {
		t.Fatalf("completed first step should stay fixed, got %+v", steps)
	}
	if steps[1].ID != "step_3" || steps[1].Index != 2 {
		t.Fatalf("expected step_3 to move up to index 2, got %+v", steps)
	}
	if steps[2].ID != "step_2" || steps[2].Index != 3 {
		t.Fatalf("expected step_2 to move down to index 3, got %+v", steps)
	}
	if steps[2].Status != PlanStepStatusPending || steps[2].ApprovalStatus != ApprovalStatusPending {
		t.Fatalf("moving an approved neighbor should require fresh review, got %+v", steps[2])
	}
}

func TestServiceMovePlanStepRejectsMovingAcrossCompletedBoundary(t *testing.T) {
	session := auth.Session{
		User:           auth.User{ID: "user_1"},
		OrganizationID: "org_1",
	}
	now := time.Now().UTC()
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusPendingApproval,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          1,
			Title:          "Gather requirements",
			Status:         PlanStepStatusCompleted,
			ApprovalStatus: ApprovalStatusNotRequired,
			CreatedAt:      now,
			UpdatedAt:      now,
		}, {
			ID:             "step_2",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          2,
			Title:          "Draft patch",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusPending,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	service := NewService(store, &fakeGateway{})

	if _, err := service.MovePlanStep(context.Background(), session, "step_2", MovePlanStepDirectionUp); err == nil || !strings.Contains(err.Error(), "cannot move across") {
		t.Fatalf("expected move across completed step rejection, got %v", err)
	}
}

func TestServiceCreatePlanStepDraftInsertsAfterDraftAndResetsShiftedApproval(t *testing.T) {
	session := auth.Session{
		User:           auth.User{ID: "user_1"},
		OrganizationID: "org_1",
	}
	now := time.Now().UTC()
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusPendingApproval,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          1,
			Title:          "Draft patch",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusPending,
			CreatedAt:      now,
			UpdatedAt:      now,
		}, {
			ID:             "step_2",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          2,
			Title:          "Verify patch",
			Status:         PlanStepStatusApproved,
			ApprovalStatus: ApprovalStatusApproved,
			CreatedAt:      now.Add(time.Second),
			UpdatedAt:      now.Add(time.Second),
		}},
	}
	service := NewService(store, &fakeGateway{})
	afterID := "step_1"

	steps, err := service.CreatePlanStepDraft(context.Background(), session, "run_1", CreatePlanStepDraftRequest{
		AfterPlanStepID: &afterID,
		Title:           "Run static checks",
		ToolName:        "execute_code",
		Input:           map[string]any{"command": "go test ./internal/agent"},
	})
	if err != nil {
		t.Fatalf("CreatePlanStepDraft returned error: %v", err)
	}

	if len(steps) != 3 {
		t.Fatalf("expected three steps, got %+v", steps)
	}
	if steps[0].ID != "step_1" || steps[0].Index != 1 {
		t.Fatalf("expected original first step to stay first, got %+v", steps[0])
	}
	if steps[1].Title != "Run static checks" || steps[1].Index != 2 || steps[1].Status != PlanStepStatusPending || steps[1].ApprovalStatus != ApprovalStatusPending {
		t.Fatalf("expected inserted pending step at index 2, got %+v", steps[1])
	}
	if steps[1].ToolName != "execute_code" || steps[1].Input["command"] != "go test ./internal/agent" {
		t.Fatalf("expected inserted step tool/input, got %+v", steps[1])
	}
	if steps[2].ID != "step_2" || steps[2].Index != 3 || steps[2].Status != PlanStepStatusPending || steps[2].ApprovalStatus != ApprovalStatusPending {
		t.Fatalf("expected shifted approved step to require fresh review, got %+v", steps[2])
	}
}

func TestServiceCreatePlanStepDraftRejectsInsertAfterExecutedStep(t *testing.T) {
	session := auth.Session{
		User:           auth.User{ID: "user_1"},
		OrganizationID: "org_1",
	}
	now := time.Now().UTC()
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusPendingApproval,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          1,
			Title:          "Gather requirements",
			Status:         PlanStepStatusCompleted,
			ApprovalStatus: ApprovalStatusNotRequired,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	service := NewService(store, &fakeGateway{})
	afterID := "step_1"

	if _, err := service.CreatePlanStepDraft(context.Background(), session, "run_1", CreatePlanStepDraftRequest{
		AfterPlanStepID: &afterID,
		Title:           "Too late",
	}); err == nil || !strings.Contains(err.Error(), "cannot be inserted after executed step") {
		t.Fatalf("expected executed anchor rejection, got %v", err)
	}
}

func TestServiceDeletePlanStepDraftRemovesStepAndReindexesDrafts(t *testing.T) {
	session := auth.Session{
		User:           auth.User{ID: "user_1"},
		OrganizationID: "org_1",
	}
	now := time.Now().UTC()
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusPendingApproval,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          1,
			Title:          "Draft patch",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusPending,
			CreatedAt:      now,
			UpdatedAt:      now,
		}, {
			ID:             "step_2",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          2,
			Title:          "Run checks",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusPending,
			CreatedAt:      now.Add(time.Second),
			UpdatedAt:      now.Add(time.Second),
		}, {
			ID:             "step_3",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          3,
			Title:          "Verify patch",
			Status:         PlanStepStatusApproved,
			ApprovalStatus: ApprovalStatusApproved,
			CreatedAt:      now.Add(2 * time.Second),
			UpdatedAt:      now.Add(2 * time.Second),
		}},
	}
	service := NewService(store, &fakeGateway{})

	steps, err := service.DeletePlanStepDraft(context.Background(), session, "step_2")
	if err != nil {
		t.Fatalf("DeletePlanStepDraft returned error: %v", err)
	}

	if len(steps) != 2 {
		t.Fatalf("expected two remaining steps, got %+v", steps)
	}
	if steps[0].ID != "step_1" || steps[0].Index != 1 {
		t.Fatalf("expected first step unchanged, got %+v", steps[0])
	}
	if steps[1].ID != "step_3" || steps[1].Index != 2 || steps[1].Status != PlanStepStatusPending || steps[1].ApprovalStatus != ApprovalStatusPending {
		t.Fatalf("expected shifted approved step to require fresh review, got %+v", steps[1])
	}
}

func TestServiceDeletePlanStepDraftRejectsExecutedStep(t *testing.T) {
	session := auth.Session{
		User:           auth.User{ID: "user_1"},
		OrganizationID: "org_1",
	}
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusPendingApproval,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          1,
			Title:          "Already completed",
			Status:         PlanStepStatusCompleted,
			ApprovalStatus: ApprovalStatusNotRequired,
		}},
	}
	service := NewService(store, &fakeGateway{})

	if _, err := service.DeletePlanStepDraft(context.Background(), session, "step_1"); err == nil || !strings.Contains(err.Error(), "cannot be deleted") {
		t.Fatalf("expected executed step delete rejection, got %v", err)
	}
}

func (e *fakePlanStepExecutor) ExecutePlanStep(ctx context.Context, step *PlanStep) (*PlanStepExecutionResult, error) {
	e.calls++
	if step != nil {
		copied := *step
		e.seenStep = &copied
	}
	if e.err != nil {
		return nil, e.err
	}
	return &PlanStepExecutionResult{ResultContent: e.resultContent}, nil
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

func (s *fakeStore) CreateMemory(ctx context.Context, req *CreateMemoryStoreRequest) (*Memory, error) {
	now := time.Now().UTC()
	s.createMemoryEmbedding = append([]float32(nil), req.Embedding...)
	memory := &Memory{
		ID:             "memory_" + time.Now().UTC().Format("150405.000000"),
		OrganizationID: req.OrganizationID,
		UserID:         req.UserID,
		AgentID:        req.AgentID,
		Type:           req.Type,
		Content:        req.Content,
		Importance:     req.Importance,
		Metadata:       copyMetadata(req.Metadata),
		ExpiresAt:      req.ExpiresAt,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	s.memories = append(s.memories, memory)
	return memory, nil
}

func (s *fakeStore) ListMemories(ctx context.Context, organizationID, userID string, req ListMemoriesRequest) ([]*Memory, error) {
	s.listMemoryOrganizationID = organizationID
	s.listMemoryUserID = userID
	s.listMemoryAgentID = req.AgentID
	if req.Query != "" {
		s.listMemoryQuery = req.Query
		s.listMemoryLimit = req.Limit
	}

	var memories []*Memory
	for _, memory := range s.memories {
		if memory.OrganizationID != organizationID || memory.UserID != userID {
			continue
		}
		if req.AgentID != "" && memory.AgentID != req.AgentID {
			continue
		}
		if req.Type != "" && memory.Type != req.Type {
			continue
		}
		if req.Query != "" && !strings.Contains(strings.ToLower(memory.Content), strings.ToLower(req.Query)) {
			continue
		}
		memories = append(memories, memory)
		if req.Limit > 0 && len(memories) >= req.Limit {
			break
		}
	}
	return memories, nil
}

func (s *fakeStore) SearchMemories(ctx context.Context, organizationID, userID string, req SearchMemoriesRequest) ([]*MemorySearchResult, error) {
	s.searchMemoryAgentID = req.AgentID
	s.searchMemoryEmbedding = append([]float32(nil), req.Embedding...)
	s.searchMemoryLimit = req.Limit
	s.searchMemoryMinScore = req.MinScore
	return s.searchMemoryResults, nil
}

func TestServiceApproveAndExecutePlanStepCompletesWithExecutorResult(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusCompleted,
			StartedAt:      now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          1,
			Title:          "Implement the service method",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusPending,
			ToolName:       "agent_step",
			Input:          map[string]any{"scope": "agent"},
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	executor := &fakePlanStepExecutor{resultContent: "step completed by fake executor"}
	service := NewService(store, &fakeGateway{})
	service.SetPlanStepExecutor(executor)
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}

	approved, err := service.ApprovePlanStep(context.Background(), session, "step_1", "ready")
	if err != nil {
		t.Fatalf("ApprovePlanStep returned error: %v", err)
	}
	if approved.Status != PlanStepStatusApproved || approved.ApprovalStatus != ApprovalStatusApproved {
		t.Fatalf("expected approved plan step, got %+v", approved)
	}
	if approved.Title != "Implement the service method" || approved.Input["scope"] != "agent" {
		t.Fatalf("ApprovePlanStep should preserve existing fields, got %+v", approved)
	}

	completed, err := service.ExecutePlanStep(context.Background(), session, "step_1")
	if err != nil {
		t.Fatalf("ExecutePlanStep returned error: %v", err)
	}
	if executor.calls != 1 {
		t.Fatalf("expected executor to be called once, got %d", executor.calls)
	}
	if executor.seenStep == nil || executor.seenStep.Status != PlanStepStatusRunning {
		t.Fatalf("expected executor to receive running step, got %+v", executor.seenStep)
	}
	if completed.Status != PlanStepStatusCompleted || completed.ApprovalStatus != ApprovalStatusApproved {
		t.Fatalf("expected completed approved step, got %+v", completed)
	}
	if completed.ResultContent != "step completed by fake executor" || completed.Error != "" {
		t.Fatalf("expected executor result and cleared error, got %+v", completed)
	}
	if completed.StartedAt == nil || completed.CompletedAt == nil {
		t.Fatalf("expected execution timestamps, got %+v", completed)
	}
}

func TestServiceExecutePlanStepMarksFailedWhenExecutorErrors(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusCompleted,
			StartedAt:      now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          1,
			Title:          "Run risky step",
			Status:         PlanStepStatusApproved,
			ApprovalStatus: ApprovalStatusApproved,
			Input:          map[string]any{},
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	executor := &fakePlanStepExecutor{err: errors.New("executor failed")}
	service := NewService(store, &fakeGateway{})
	service.SetPlanStepExecutor(executor)

	updated, err := service.ExecutePlanStep(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		"step_1",
	)
	if err == nil {
		t.Fatal("expected ExecutePlanStep to return executor error")
	}
	if updated == nil || updated.Status != PlanStepStatusFailed {
		t.Fatalf("expected failed plan step to be returned, got %+v", updated)
	}
	if !strings.Contains(updated.Error, "executor failed") || updated.CompletedAt == nil {
		t.Fatalf("expected failure error and completion timestamp, got %+v", updated)
	}
}

func TestServiceRetryPlanStepReopensRunAndExecutesFailedStep(t *testing.T) {
	now := time.Now().UTC()
	completedAt := now.Add(time.Minute)
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusTokenBudgetExceeded,
			Error:          "token_budget_exceeded: old budget",
			StartedAt:      now,
			CompletedAt:    &completedAt,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          1,
			Title:          "Retry failed implementation",
			Status:         PlanStepStatusFailed,
			ApprovalStatus: ApprovalStatusApproved,
			ResultContent:  "stale output",
			Error:          "old failure",
			StartedAt:      &now,
			CompletedAt:    &completedAt,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	executor := &fakePlanStepExecutor{resultContent: "retry passed"}
	service := NewService(store, &fakeGateway{})
	service.SetPlanStepExecutor(executor)
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}

	completed, err := service.RetryPlanStep(context.Background(), session, "step_1")
	if err != nil {
		t.Fatalf("RetryPlanStep returned error: %v", err)
	}
	if executor.calls != 1 {
		t.Fatalf("expected retry to execute once, got %d calls", executor.calls)
	}
	if completed.Status != PlanStepStatusCompleted || completed.ResultContent != "retry passed" || completed.Error != "" {
		t.Fatalf("expected retried plan step to complete cleanly, got %+v", completed)
	}
	if executor.seenStep == nil || executor.seenStep.Status != PlanStepStatusRunning || executor.seenStep.Error != "" || executor.seenStep.ResultContent != "" || executor.seenStep.CompletedAt != nil {
		t.Fatalf("expected executor to receive reset running step, got %+v", executor.seenStep)
	}
	run, err := store.GetRun(context.Background(), "org_1", "run_1")
	if err != nil {
		t.Fatalf("GetRun returned error: %v", err)
	}
	if run.Status != RunStatusCompleted || run.Error != "" || run.CompletedAt == nil {
		t.Fatalf("expected retried final step to complete reopened run, got %+v", run)
	}
}

func TestServiceRetryPlanStepReopensPendingApprovalStepWithoutExecuting(t *testing.T) {
	now := time.Now().UTC()
	completedAt := now.Add(time.Minute)
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusFailed,
			Error:          "approval step failed",
			StartedAt:      now,
			CompletedAt:    &completedAt,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          1,
			Title:          "Needs fresh approval",
			Status:         PlanStepStatusFailed,
			ApprovalStatus: ApprovalStatusPending,
			ResultContent:  "stale output",
			Error:          "old failure",
			StartedAt:      &now,
			CompletedAt:    &completedAt,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	executor := &fakePlanStepExecutor{resultContent: "should not run without approval"}
	service := NewService(store, &fakeGateway{})
	service.SetPlanStepExecutor(executor)
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}

	reopened, err := service.RetryPlanStep(context.Background(), session, "step_1")
	if err != nil {
		t.Fatalf("RetryPlanStep returned error: %v", err)
	}
	if executor.calls != 0 {
		t.Fatalf("expected pending approval retry not to execute, got %d calls", executor.calls)
	}
	if reopened.Status != PlanStepStatusPending || reopened.ApprovalStatus != ApprovalStatusPending || reopened.ResultContent != "" || reopened.Error != "" || reopened.CompletedAt != nil {
		t.Fatalf("expected failed pending-approval step to reopen for approval, got %+v", reopened)
	}
	run, err := store.GetRun(context.Background(), "org_1", "run_1")
	if err != nil {
		t.Fatalf("GetRun returned error: %v", err)
	}
	if run.Status != RunStatusPendingApproval || run.Error != "" || run.CompletedAt != nil {
		t.Fatalf("expected run to reopen pending approval, got %+v", run)
	}
}

func TestServiceRetryPlanStepRejectsNonFailedStep(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusPendingApproval,
			StartedAt:      now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          1,
			Title:          "Not failed",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusNotRequired,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	executor := &fakePlanStepExecutor{resultContent: "should not run"}
	service := NewService(store, &fakeGateway{})
	service.SetPlanStepExecutor(executor)

	updated, err := service.RetryPlanStep(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, "step_1")
	if err == nil || !strings.Contains(err.Error(), "plan step is not failed") {
		t.Fatalf("expected non-failed retry rejection, got step=%+v err=%v", updated, err)
	}
	if executor.calls != 0 {
		t.Fatalf("expected executor not to run, got %d calls", executor.calls)
	}
	if store.planSteps[0].Status != PlanStepStatusPending {
		t.Fatalf("expected non-failed step to remain unchanged, got %+v", store.planSteps[0])
	}
}

func TestExecutePlanStepRunsBuiltinTool(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusCompleted,
			StartedAt:      now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          1,
			Title:          "Calculate answer",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusNotRequired,
			ToolName:       "calculator",
			Input:          map[string]any{"expression": "2 + 3"},
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	service := NewService(store, &fakeGateway{})

	completed, err := service.ExecutePlanStep(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		"step_1",
	)
	if err != nil {
		t.Fatalf("ExecutePlanStep returned error: %v", err)
	}
	if completed.Status != PlanStepStatusCompleted {
		t.Fatalf("expected completed plan step, got %+v", completed)
	}
	if completed.ResultContent != "Result: 5" {
		t.Fatalf("expected builtin calculator result, got %q", completed.ResultContent)
	}
	if strings.Contains(completed.ResultContent, "plan step marked completed") {
		t.Fatalf("expected real builtin result instead of placeholder, got %q", completed.ResultContent)
	}
	if len(store.toolRuns) != 1 {
		t.Fatalf("expected ExecutePlanStep to persist one tool run audit record, got %+v", store.toolRuns)
	}
	toolRun := store.toolRuns[0]
	if toolRun.RunID != "run_1" || toolRun.ConversationID != "conv_1" || toolRun.AgentID != "agent_1" {
		t.Fatalf("plan step tool run linkage = %+v, want run/conversation/agent ids", toolRun)
	}
	if toolRun.ToolName != "calculator" || toolRun.ToolType != "builtin" || toolRun.ToolCallID != "plan_step_step_1" {
		t.Fatalf("plan step tool run identity = %+v, want builtin calculator plan-step call", toolRun)
	}
	if toolRun.Status != ToolRunStatusCompleted || toolRun.ApprovalStatus != ApprovalStatusNotRequired {
		t.Fatalf("plan step tool run status = %+v, want completed without approval", toolRun)
	}
	if toolRun.AttemptCount != 1 || toolRun.ResultContent != "Result: 5" || toolRun.StartedAt == nil || toolRun.CompletedAt == nil {
		t.Fatalf("plan step tool run result metadata = %+v, want attempt/result/timestamps", toolRun)
	}
	if toolRun.Arguments["expression"] != "2 + 3" {
		t.Fatalf("plan step tool run arguments = %+v, want original step input", toolRun.Arguments)
	}
}

func TestExecutePlanStepRunsPlainLLMStepWithPlanningContext(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			SystemPrompt:   "You are a careful migration agent.",
			Config: Config{
				Temperature:      0.2,
				MaxTokens:        900,
				ApprovalMode:     "manual",
				KnowledgeBaseIDs: []string{"kb_1"},
			},
		},
		conversation: &Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
		messages: []*Message{
			{
				ID:             "msg_user",
				ConversationID: "conv_1",
				OrganizationID: "org_1",
				Role:           "user",
				Content:        "Ship the tenant migration safely.",
				CreatedAt:      now,
			},
			{
				ID:             "msg_plan",
				ConversationID: "conv_1",
				OrganizationID: "org_1",
				Role:           "assistant",
				Content:        "1. Inspect schema drift\n2. Implement migration guard",
				CreatedAt:      now,
			},
		},
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusPendingApproval,
			StartedAt:      now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		planSteps: []*PlanStep{
			{
				ID:             "step_1",
				RunID:          "run_1",
				OrganizationID: "org_1",
				Index:          1,
				Title:          "Inspect schema drift",
				Status:         PlanStepStatusCompleted,
				ApprovalStatus: ApprovalStatusApproved,
				ResultContent:  "No tenant tables are missing.",
				CreatedAt:      now,
				UpdatedAt:      now,
			},
			{
				ID:             "step_2",
				RunID:          "run_1",
				OrganizationID: "org_1",
				Index:          2,
				Title:          "Implement migration guard",
				Status:         PlanStepStatusApproved,
				ApprovalStatus: ApprovalStatusApproved,
				Input:          map[string]any{"risk": "tenant outage"},
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		},
	}
	gateway := &fakeGateway{plainReply: "Added migration guard with tenant-safe rollback."}
	service := NewService(store, gateway)

	completed, err := service.ExecutePlanStep(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		"step_2",
	)
	if err != nil {
		t.Fatalf("ExecutePlanStep returned error: %v", err)
	}
	if gateway.plainCalls != 1 {
		t.Fatalf("expected no-tool plan step to call plain gateway once, got %d", gateway.plainCalls)
	}
	if completed.Status != PlanStepStatusCompleted || completed.ResultContent != "Added migration guard with tenant-safe rollback." {
		t.Fatalf("expected gateway result to complete plan step, got %+v", completed)
	}
	if len(store.toolRuns) != 0 {
		t.Fatalf("expected no-tool plan step not to create tool runs, got %+v", store.toolRuns)
	}
	if store.runs[0].Status != RunStatusCompleted || store.runs[0].CompletedAt == nil {
		t.Fatalf("expected run to complete after all plan steps are done, got %+v", store.runs[0])
	}
	prompt := chatMessagesContent(gateway.lastPlainMessages)
	for _, want := range []string{
		"You are a careful migration agent.",
		"Execute exactly one approved plan step",
		"Ship the tenant migration safely.",
		"Current plan step",
		"Implement migration guard",
		"tenant outage",
		"Prior completed plan steps",
		"Inspect schema drift",
		"No tenant tables are missing.",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("expected execution prompt to contain %q, got %q", want, prompt)
		}
	}
}

func TestExecutePlanStepStopsBeforePlainLLMWhenTokenBudgetExceeded(t *testing.T) {
	now := time.Now().UTC()
	largeContext := strings.Repeat("tenant migration risk ", 900)
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config: Config{
				TokenBudget: 1000,
			},
		},
		conversation: &Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
		messages: []*Message{{
			ID:             "msg_user",
			ConversationID: "conv_1",
			OrganizationID: "org_1",
			Role:           "user",
			Content:        largeContext,
			CreatedAt:      now,
		}},
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusPendingApproval,
			StartedAt:      now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          1,
			Title:          "Summarize oversized context",
			Status:         PlanStepStatusApproved,
			ApprovalStatus: ApprovalStatusApproved,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	gateway := &fakeGateway{plainReply: "should not be called"}
	service := NewService(store, gateway)

	step, err := service.ExecutePlanStep(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		"step_1",
	)
	if !errors.Is(err, ErrTokenBudgetExceeded) {
		t.Fatalf("expected ErrTokenBudgetExceeded, got %v", err)
	}
	if gateway.plainCalls != 0 {
		t.Fatalf("expected token budget guard to stop before gateway call, got %d calls", gateway.plainCalls)
	}
	if step == nil || step.Status != PlanStepStatusFailed || !strings.Contains(step.Error, "token_budget_exceeded") {
		t.Fatalf("expected failed plan step with token budget evidence, got %+v", step)
	}
	run := store.runs[0]
	if run.Status != RunStatusTokenBudgetExceeded || !strings.Contains(run.Error, "token_budget_exceeded") || run.IterationCount != 1 {
		t.Fatalf("expected planning run to record token budget stop, got %+v", run)
	}
}

func TestServiceCreateUpdateAgentNormalizesIterationAndTokenBudgetConfig(t *testing.T) {
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}
	store := &fakeStore{}
	service := NewService(store, &fakeGateway{})

	created, err := service.CreateAgent(context.Background(), session, &CreateAgentRequest{
		Name: "budgeted",
		Config: Config{
			MaxIterations: -1,
			TokenBudget:   10_000_001,
		},
	})
	if err != nil {
		t.Fatalf("CreateAgent returned error: %v", err)
	}
	if created.Config.MaxIterations != DefaultRunnerConfig().MaxIterations {
		t.Fatalf("expected default max iterations, got %d", created.Config.MaxIterations)
	}
	if created.Config.TokenBudget != 1_000_000 {
		t.Fatalf("expected oversized token budget to clamp to 1000000, got %d", created.Config.TokenBudget)
	}

	updated, err := service.UpdateAgent(context.Background(), session, created.ID, &UpdateAgentRequest{
		Config: &Config{
			MaxIterations: 101,
			TokenBudget:   1,
		},
	})
	if err != nil {
		t.Fatalf("UpdateAgent returned error: %v", err)
	}
	if updated.Config.MaxIterations != 100 {
		t.Fatalf("expected max iterations to clamp to 100, got %d", updated.Config.MaxIterations)
	}
	if updated.Config.TokenBudget != 1_000 {
		t.Fatalf("expected low positive token budget to normalize to 1000, got %d", updated.Config.TokenBudget)
	}

	updated, err = service.UpdateAgent(context.Background(), session, created.ID, &UpdateAgentRequest{
		Config: &Config{
			MaxIterations: 1,
			TokenBudget:   0,
		},
	})
	if err != nil {
		t.Fatalf("UpdateAgent with zero token budget returned error: %v", err)
	}
	if updated.Config.MaxIterations != 1 {
		t.Fatalf("expected explicit min max iterations to remain 1, got %d", updated.Config.MaxIterations)
	}
	if updated.Config.TokenBudget != 0 {
		t.Fatalf("expected non-positive token budget to remain unlimited/default 0, got %d", updated.Config.TokenBudget)
	}
}

func TestServiceCreateUpdateAgentNormalizesDefaultExecutionMode(t *testing.T) {
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}
	store := &fakeStore{}
	service := NewService(store, &fakeGateway{})

	created, err := service.CreateAgent(context.Background(), session, &CreateAgentRequest{
		Name: "default mode",
	})
	if err != nil {
		t.Fatalf("CreateAgent returned error: %v", err)
	}
	if created.Config.DefaultExecutionMode != ExecutionModeReact {
		t.Fatalf("expected empty default execution mode to normalize to react, got %q", created.Config.DefaultExecutionMode)
	}

	updated, err := service.UpdateAgent(context.Background(), session, created.ID, &UpdateAgentRequest{
		Config: &Config{DefaultExecutionMode: " PLANNING "},
	})
	if err != nil {
		t.Fatalf("UpdateAgent returned error: %v", err)
	}
	if updated.Config.DefaultExecutionMode != ExecutionModePlanning {
		t.Fatalf("expected default execution mode to normalize to planning, got %q", updated.Config.DefaultExecutionMode)
	}
}

func TestServiceCreateUpdateAgentRejectsInvalidDefaultExecutionMode(t *testing.T) {
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}
	store := &fakeStore{}
	service := NewService(store, &fakeGateway{})

	if _, err := service.CreateAgent(context.Background(), session, &CreateAgentRequest{
		Name: "bad mode",
		Config: Config{
			DefaultExecutionMode: "manual",
		},
	}); err == nil || !strings.Contains(err.Error(), "defaultExecutionMode must be react or planning") {
		t.Fatalf("expected invalid default execution mode error on create, got %v", err)
	}

	store.agent = &Agent{
		ID:             "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Name:           "existing",
		Config:         Config{DefaultExecutionMode: ExecutionModeReact},
	}
	if _, err := service.UpdateAgent(context.Background(), session, "agent_1", &UpdateAgentRequest{
		Config: &Config{DefaultExecutionMode: "manual"},
	}); err == nil || !strings.Contains(err.Error(), "defaultExecutionMode must be react or planning") {
		t.Fatalf("expected invalid default execution mode error on update, got %v", err)
	}
}

func TestRunWithToolsUsesAgentMaxIterationsConfig(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config: Config{
				MaxIterations: 1,
			},
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
					{ID: "call_datetime_1", Type: "function", Function: chat.ToolFunction{Name: "datetime", Arguments: "{}"}},
				},
				FinishReason: "tool_calls",
			},
			{
				ToolCalls: []chat.ToolCall{
					{ID: "call_datetime_2", Type: "function", Function: chat.ToolFunction{Name: "datetime", Arguments: "{}"}},
				},
				FinishReason: "tool_calls",
			},
		},
	}
	service := NewService(store, gateway)

	_, err := service.SendMessage(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, "conv_1", "loop")
	if !errors.Is(err, ErrMaxIterationsExceeded) {
		t.Fatalf("expected ErrMaxIterationsExceeded, got %v", err)
	}
	if gateway.structuredCalls != 1 {
		t.Fatalf("expected agent maxIterations to stop after one structured call, got %d", gateway.structuredCalls)
	}
	if len(store.runs) != 1 {
		t.Fatalf("expected one durable run, got %+v", store.runs)
	}
	run := store.runs[0]
	if run.Status != RunStatusMaxIterationsReached || run.IterationCount != 1 || !strings.Contains(run.Error, ErrMaxIterationsExceeded.Error()) {
		t.Fatalf("expected failed run with max-iterations evidence, got %+v", run)
	}
}

func TestRunWithToolsStopsBeforeToolsWhenTokenBudgetExceeded(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config: Config{
				TokenBudget: 1000,
			},
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
					{ID: "call_datetime_1", Type: "function", Function: chat.ToolFunction{Name: "datetime", Arguments: "{}"}},
				},
				FinishReason: "tool_calls",
				Usage:        &chat.CompletionUsage{TotalTokens: 1200},
			},
		},
	}
	service := NewService(store, gateway)

	_, err := service.SendMessage(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, "conv_1", "loop")
	if !errors.Is(err, ErrTokenBudgetExceeded) {
		t.Fatalf("expected ErrTokenBudgetExceeded, got %v", err)
	}
	if gateway.structuredCalls != 1 {
		t.Fatalf("expected one structured call before budget stop, got %d", gateway.structuredCalls)
	}
	if len(store.toolRuns) != 0 {
		t.Fatalf("expected no tool execution after budget stop, got %+v", store.toolRuns)
	}
	if len(store.runs) != 1 {
		t.Fatalf("expected one durable run, got %+v", store.runs)
	}
	run := store.runs[0]
	if run.Status != RunStatusTokenBudgetExceeded || run.IterationCount != 1 || !strings.Contains(run.Error, "token_budget_exceeded") {
		t.Fatalf("expected failed run with token budget evidence, got %+v", run)
	}
}

func TestServiceSendMessageOverrideMaxIterationsUsesTemporaryAgentConfig(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config: Config{
				MaxIterations: 5,
			},
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
					{ID: "call_datetime_1", Type: "function", Function: chat.ToolFunction{Name: "datetime", Arguments: "{}"}},
				},
				FinishReason: "tool_calls",
			},
			{
				ToolCalls: []chat.ToolCall{
					{ID: "call_datetime_2", Type: "function", Function: chat.ToolFunction{Name: "datetime", Arguments: "{}"}},
				},
				FinishReason: "tool_calls",
			},
		},
	}
	service := NewService(store, gateway)

	overrideMaxIterations := 1
	_, err := service.SendMessage(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, "conv_1", "loop", SendMessageOptions{
		MaxIterations: &overrideMaxIterations,
	})
	if !errors.Is(err, ErrMaxIterationsExceeded) {
		t.Fatalf("expected ErrMaxIterationsExceeded, got %v", err)
	}
	if gateway.structuredCalls != 1 {
		t.Fatalf("expected override maxIterations to stop after one structured call, got %d", gateway.structuredCalls)
	}
	if store.agent.Config.MaxIterations != 5 {
		t.Fatalf("expected stored agent config to remain unchanged, got maxIterations=%d", store.agent.Config.MaxIterations)
	}
}

func TestServiceSendMessageOverrideTokenBudgetUsesTemporaryAgentConfig(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config: Config{
				TokenBudget: 5000,
			},
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
					{ID: "call_datetime_1", Type: "function", Function: chat.ToolFunction{Name: "datetime", Arguments: "{}"}},
				},
				FinishReason: "tool_calls",
				Usage:        &chat.CompletionUsage{TotalTokens: 1200},
			},
		},
	}
	service := NewService(store, gateway)

	overrideTokenBudget := 1000
	_, err := service.SendMessage(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, "conv_1", "loop", SendMessageOptions{
		TokenBudget: &overrideTokenBudget,
	})
	if !errors.Is(err, ErrTokenBudgetExceeded) {
		t.Fatalf("expected ErrTokenBudgetExceeded, got %v", err)
	}
	if len(store.toolRuns) != 0 {
		t.Fatalf("expected override tokenBudget to stop before tool execution, got %+v", store.toolRuns)
	}
	if store.agent.Config.TokenBudget != 5000 {
		t.Fatalf("expected stored agent config to remain unchanged, got tokenBudget=%d", store.agent.Config.TokenBudget)
	}
}

func TestServiceSendMessageUsesDefaultPlanningModeWhenModeOmitted(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config: Config{
				DefaultExecutionMode: ExecutionModePlanning,
			},
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
		plainReply: "Plan:\n1. Inspect the workspace\n2. Verify the release",
		structured: []*chat.CompletionResponse{
			{
				Content:      "react answer",
				FinishReason: "stop",
			},
		},
	}
	service := NewService(store, gateway)

	msg, err := service.SendMessage(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, "conv_1", "hello")
	if err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	if msg == nil || !strings.Contains(msg.Content, "Verify the release") {
		t.Fatalf("expected default planning response, got %+v", msg)
	}
	if gateway.plainCalls != 1 || gateway.structuredCalls != 0 {
		t.Fatalf("expected default planning mode to use plain planning call, got plain=%d structured=%d", gateway.plainCalls, gateway.structuredCalls)
	}
	if len(store.runs) != 1 || store.runs[0].Mode != ExecutionModePlanning || store.runs[0].Status != RunStatusPendingApproval {
		t.Fatalf("expected pending planning run from default mode, got %+v", store.runs)
	}
	if len(store.planSteps) != 2 {
		t.Fatalf("expected parsed planning steps, got %+v", store.planSteps)
	}
}

func TestServiceSendMessageExplicitReactModeOverridesPlanningDefault(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config: Config{
				DefaultExecutionMode: ExecutionModePlanning,
			},
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
		plainReply: "planning path should not be used with explicit react mode",
		structured: []*chat.CompletionResponse{
			{
				Content:      "react answer",
				FinishReason: "stop",
			},
		},
	}
	service := NewService(store, gateway)

	msg, err := service.SendMessage(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, "conv_1", "hello", SendMessageOptions{
		Mode: ExecutionModeReact,
	})
	if err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	if msg == nil || msg.Content != "react answer" {
		t.Fatalf("expected explicit react answer, got %+v", msg)
	}
	if gateway.structuredCalls != 1 || gateway.plainCalls != 0 {
		t.Fatalf("expected explicit react mode to use structured runner, got plain=%d structured=%d", gateway.plainCalls, gateway.structuredCalls)
	}
	if len(store.runs) != 1 || store.runs[0].Mode != ExecutionModeReact || store.runs[0].Status != RunStatusCompleted {
		t.Fatalf("expected completed react run from explicit override, got %+v", store.runs)
	}
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

func TestRunWithToolsRecordsAgentObservabilityMetrics(t *testing.T) {
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

	runBefore := testutil.ToFloat64(metrics.AgentRunTotal.WithLabelValues(string(RunStatusCompleted)))
	toolBefore := testutil.ToFloat64(metrics.AgentToolCallTotal.WithLabelValues("datetime", string(ToolRunStatusCompleted)))
	_, err := service.SendMessage(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		"conv_1",
		"What time is it?",
	)
	if err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}

	runAfter := testutil.ToFloat64(metrics.AgentRunTotal.WithLabelValues(string(RunStatusCompleted)))
	if runAfter != runBefore+1 {
		t.Fatalf("expected agent run metric increment, before=%v after=%v", runBefore, runAfter)
	}
	toolAfter := testutil.ToFloat64(metrics.AgentToolCallTotal.WithLabelValues("datetime", string(ToolRunStatusCompleted)))
	if toolAfter != toolBefore+1 {
		t.Fatalf("expected agent tool call metric increment, before=%v after=%v", toolBefore, toolAfter)
	}
	if count := testutil.CollectAndCount(metrics.AgentIterationCount, "agent_iteration_count"); count == 0 {
		t.Fatal("expected agent iteration count metric to be collectable")
	}
}

func TestServiceStartRunWithoutToolsCreatesDurableFetchableRun(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
		},
		conversation: &Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
	}
	gateway := &fakeGateway{plainReply: "plain durable answer"}
	service := NewService(store, gateway)
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}

	result, err := service.StartRun(
		chat.WithRelayRequestMetadata(context.Background(), chat.RelayRequestMetadata{RequestID: "req_plain_start"}),
		session,
		StartRunRequest{AgentID: "agent_1", ConversationID: "conv_1", Input: "hello"},
	)
	if err != nil {
		t.Fatalf("StartRun returned error: %v", err)
	}
	if result.Run == nil {
		t.Fatal("expected durable run in StartRun response")
	}
	if len(store.runs) != 1 {
		t.Fatalf("expected one persisted run, got %+v", store.runs)
	}
	run := store.runs[0]
	if result.Run.ID != run.ID {
		t.Fatalf("expected StartRun run id %q to match persisted run id %q", result.Run.ID, run.ID)
	}
	if run.Status != RunStatusCompleted || run.FinalMessageID == "" || run.CompletedAt == nil {
		t.Fatalf("expected completed run with final message, got %+v", run)
	}
	if run.Mode != ExecutionModeReact {
		t.Fatalf("expected durable no-tool run mode %q, got %q", ExecutionModeReact, run.Mode)
	}
	if run.IterationCount != 1 || run.ToolCallCount != 0 {
		t.Fatalf("expected no-tool run to record one iteration and zero tool calls, got iterations=%d tools=%d", run.IterationCount, run.ToolCallCount)
	}
	if run.RequestID != "req_plain_start" {
		t.Fatalf("expected request id to persist, got %q", run.RequestID)
	}

	fetched, err := service.GetRunWithMessages(context.Background(), session, result.Run.ID)
	if err != nil {
		t.Fatalf("GetRunWithMessages returned error: %v", err)
	}
	if fetched.Run == nil || fetched.Run.ID != result.Run.ID {
		t.Fatalf("expected fetched run %q, got %+v", result.Run.ID, fetched.Run)
	}
	if len(fetched.Messages) != 2 || fetched.Messages[1].ID != run.FinalMessageID {
		t.Fatalf("expected fetched messages to include final assistant message, got %+v", fetched.Messages)
	}
}

func TestServiceStartPlanningRunCreatesDurablePlanWithoutExecutingTools(t *testing.T) {
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
	gateway := &fakeGateway{plainReply: "Plan:\n1. Inspect current behavior\n2. Implement minimal endpoint"}
	service := NewService(store, gateway)

	result, err := service.StartPlanningRun(
		chat.WithRelayRequestMetadata(context.Background(), chat.RelayRequestMetadata{RequestID: "req_plan"}),
		auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1", User: auth.User{ID: "user_1"}},
		StartRunRequest{AgentID: "agent_1", ConversationID: "conv_1", Input: "make a plan"},
	)
	if err != nil {
		t.Fatalf("StartPlanningRun returned error: %v", err)
	}
	if result.Run == nil || result.Run.Status != RunStatusPendingApproval {
		t.Fatalf("expected planning run to wait for plan step execution, got %+v", result.Run)
	}
	if result.Run.Mode != ExecutionModePlanning {
		t.Fatalf("expected planning run mode %q, got %q", ExecutionModePlanning, result.Run.Mode)
	}
	if len(store.toolRuns) != 0 {
		t.Fatalf("expected planning mode not to execute tools, got %+v", store.toolRuns)
	}
	if gateway.plainCalls != 1 || gateway.structuredCalls != 0 {
		t.Fatalf("expected one plain planning call and no structured tool loop, got plain=%d structured=%d", gateway.plainCalls, gateway.structuredCalls)
	}
	if result.Run.RequestID != "req_plan" || result.Run.FinalMessageID == "" || result.Run.CompletedAt != nil {
		t.Fatalf("expected request/final evidence without completion before plan execution, got %+v", result.Run)
	}
	if len(result.Messages) != 2 {
		t.Fatalf("expected user and assistant planning messages, got %+v", result.Messages)
	}
	if result.Messages[0].Role != "user" || result.Messages[0].Content != "make a plan" {
		t.Fatalf("expected persisted user planning request, got %+v", result.Messages[0])
	}
	if result.Messages[1].Role != "assistant" || !strings.Contains(result.Messages[1].Content, "Implement minimal endpoint") {
		t.Fatalf("expected persisted planning response, got %+v", result.Messages[1])
	}
}

func TestServiceStartPlanningRunRecordsPendingApprovalMetrics(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
		},
		conversation: &Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
	}
	gateway := &fakeGateway{plainReply: "1. Gather requirements\n2. Verify"}
	service := NewService(store, gateway)

	runBefore := testutil.ToFloat64(metrics.AgentRunTotal.WithLabelValues(string(RunStatusPendingApproval)))
	_, err := service.StartPlanningRun(
		context.Background(),
		auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1", User: auth.User{ID: "user_1"}},
		StartRunRequest{AgentID: "agent_1", ConversationID: "conv_1", Input: "make a plan"},
	)
	if err != nil {
		t.Fatalf("StartPlanningRun returned error: %v", err)
	}

	runAfter := testutil.ToFloat64(metrics.AgentRunTotal.WithLabelValues(string(RunStatusPendingApproval)))
	if runAfter != runBefore+1 {
		t.Fatalf("expected pending approval planning run metric increment, before=%v after=%v", runBefore, runAfter)
	}
	if count := testutil.CollectAndCount(metrics.AgentIterationCount, "agent_iteration_count"); count == 0 {
		t.Fatal("expected agent iteration count metric to be collectable")
	}
}

func TestServiceStartPlanningRunPersistsParsedPlanSteps(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
		},
		conversation: &Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
	}
	gateway := &fakeGateway{plainReply: "1. Gather requirements\n2. Draft implementation\n3. Verify tests"}
	service := NewService(store, gateway)

	result, err := service.StartPlanningRun(
		context.Background(),
		auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1", User: auth.User{ID: "user_1"}},
		StartRunRequest{AgentID: "agent_1", ConversationID: "conv_1", Input: "make a plan"},
	)
	if err != nil {
		t.Fatalf("StartPlanningRun returned error: %v", err)
	}
	if result.Run == nil {
		t.Fatal("expected planning run")
	}
	if len(store.planSteps) != 3 {
		t.Fatalf("expected 3 persisted plan steps, got %+v", store.planSteps)
	}
	wantTitles := []string{"Gather requirements", "Draft implementation", "Verify tests"}
	for i, step := range store.planSteps {
		if step.RunID != result.Run.ID || step.OrganizationID != "org_1" {
			t.Fatalf("step %d did not persist run/org scope: %+v", i, step)
		}
		if step.Index != i+1 || step.Title != wantTitles[i] {
			t.Fatalf("step %d = index %d title %q, want index %d title %q", i, step.Index, step.Title, i+1, wantTitles[i])
		}
		if step.Status != PlanStepStatusPending || step.ApprovalStatus != ApprovalStatusNotRequired {
			t.Fatalf("step %d status mismatch: %+v", i, step)
		}
		if step.ToolName != "" || len(step.Input) != 0 {
			t.Fatalf("step %d should not infer tool/input from plain text: %+v", i, step)
		}
	}
	if len(result.PlanSteps) != 3 {
		t.Fatalf("expected returned RunWithMessages to include plan steps, got %+v", result.PlanSteps)
	}
	if result.Run.Status != RunStatusPendingApproval || result.Run.CompletedAt != nil {
		t.Fatalf("expected planning run to remain open for step execution, got %+v", result.Run)
	}
}

func TestServiceExecutePlanStepCompletesPlanningRunAfterLastStep(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusPendingApproval,
			StartedAt:      now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          1,
			Title:          "Gather requirements",
			Status:         PlanStepStatusCompleted,
			ApprovalStatus: ApprovalStatusNotRequired,
			CreatedAt:      now,
			UpdatedAt:      now,
		}, {
			ID:             "step_2",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          2,
			Title:          "Verify implementation",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusNotRequired,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	executor := &fakePlanStepExecutor{resultContent: "verification passed"}
	service := NewService(store, &fakeGateway{})
	service.SetPlanStepExecutor(executor)
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}

	completedStep, err := service.ExecutePlanStep(context.Background(), session, "step_2")
	if err != nil {
		t.Fatalf("ExecutePlanStep returned error: %v", err)
	}
	if completedStep.Status != PlanStepStatusCompleted || completedStep.ResultContent != "verification passed" {
		t.Fatalf("expected final step to complete, got %+v", completedStep)
	}

	run, err := store.GetRun(context.Background(), "org_1", "run_1")
	if err != nil {
		t.Fatalf("GetRun returned error: %v", err)
	}
	if run.Status != RunStatusCompleted || run.CompletedAt == nil {
		t.Fatalf("expected planning run to complete after final step, got %+v", run)
	}
}

func TestServiceExecutePlanStepRejectsOutOfOrderStep(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusPendingApproval,
			StartedAt:      now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          1,
			Title:          "Gather requirements",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusNotRequired,
			CreatedAt:      now,
			UpdatedAt:      now,
		}, {
			ID:             "step_2",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          2,
			Title:          "Verify implementation",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusNotRequired,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	executor := &fakePlanStepExecutor{resultContent: "verification passed"}
	service := NewService(store, &fakeGateway{})
	service.SetPlanStepExecutor(executor)
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}

	updated, err := service.ExecutePlanStep(context.Background(), session, "step_2")
	if err == nil {
		t.Fatal("expected ExecutePlanStep to reject out-of-order execution")
	}
	if updated != nil {
		t.Fatalf("expected no updated step when execution is rejected, got %+v", updated)
	}
	if !strings.Contains(err.Error(), "prior plan step 1 must be completed or skipped before executing step 2") {
		t.Fatalf("expected prior-step validation error, got %v", err)
	}
	if executor.calls != 0 {
		t.Fatalf("expected executor not to run, got %d calls", executor.calls)
	}
	if store.planSteps[1].Status != PlanStepStatusPending || store.planSteps[1].StartedAt != nil || store.planSteps[1].CompletedAt != nil {
		t.Fatalf("expected rejected step to remain untouched, got %+v", store.planSteps[1])
	}
}

func TestServiceSkipPlanStepAllowsLaterExecution(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusPendingApproval,
			StartedAt:      now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          1,
			Title:          "Optional discovery",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusNotRequired,
			CreatedAt:      now,
			UpdatedAt:      now,
		}, {
			ID:             "step_2",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          2,
			Title:          "Verify implementation",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusNotRequired,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	executor := &fakePlanStepExecutor{resultContent: "verification passed"}
	service := NewService(store, &fakeGateway{})
	service.SetPlanStepExecutor(executor)
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}

	skipped, err := service.SkipPlanStep(context.Background(), session, "step_1", "not needed")
	if err != nil {
		t.Fatalf("SkipPlanStep returned error: %v", err)
	}
	if skipped.Status != PlanStepStatusSkipped || skipped.Error != "not needed" || skipped.CompletedAt == nil {
		t.Fatalf("expected skipped step with reason and completion time, got %+v", skipped)
	}

	completed, err := service.ExecutePlanStep(context.Background(), session, "step_2")
	if err != nil {
		t.Fatalf("ExecutePlanStep after skipped prior step returned error: %v", err)
	}
	if completed.Status != PlanStepStatusCompleted || completed.ResultContent != "verification passed" {
		t.Fatalf("expected second step to execute after skipped prior step, got %+v", completed)
	}
	run, err := store.GetRun(context.Background(), "org_1", "run_1")
	if err != nil {
		t.Fatalf("GetRun returned error: %v", err)
	}
	if run.Status != RunStatusCompleted || run.CompletedAt == nil {
		t.Fatalf("expected run to complete after skipped/completed steps, got %+v", run)
	}
}

func TestServiceSkipPlanStepCompletesRunWhenAllStepsAreDoneOrSkipped(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusPendingApproval,
			StartedAt:      now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          1,
			Title:          "Gather requirements",
			Status:         PlanStepStatusCompleted,
			ApprovalStatus: ApprovalStatusNotRequired,
			CreatedAt:      now,
			UpdatedAt:      now,
		}, {
			ID:             "step_2",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          2,
			Title:          "Optional polish",
			Status:         PlanStepStatusFailed,
			ApprovalStatus: ApprovalStatusNotRequired,
			Error:          "not required",
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	service := NewService(store, &fakeGateway{})
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}

	skipped, err := service.SkipPlanStep(context.Background(), session, "step_2", "")
	if err != nil {
		t.Fatalf("SkipPlanStep returned error: %v", err)
	}
	if skipped.Status != PlanStepStatusSkipped || skipped.Error != "" || skipped.CompletedAt == nil {
		t.Fatalf("expected failed step to be skipped cleanly, got %+v", skipped)
	}
	run, err := store.GetRun(context.Background(), "org_1", "run_1")
	if err != nil {
		t.Fatalf("GetRun returned error: %v", err)
	}
	if run.Status != RunStatusCompleted || run.CompletedAt == nil {
		t.Fatalf("expected run to complete after final skip, got %+v", run)
	}
}

func TestServiceSkipPlanStepRejectsStartedOrTerminalSteps(t *testing.T) {
	tests := []struct {
		name   string
		status string
	}{
		{name: "running", status: PlanStepStatusRunning},
		{name: "completed", status: PlanStepStatusCompleted},
		{name: "skipped", status: PlanStepStatusSkipped},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := time.Now().UTC()
			store := &fakeStore{
				runs: []*Run{{
					ID:             "run_1",
					OrganizationID: "org_1",
					ConversationID: "conv_1",
					AgentID:        "agent_1",
					UserID:         "user_1",
					Status:         RunStatusPendingApproval,
					StartedAt:      now,
					CreatedAt:      now,
					UpdatedAt:      now,
				}},
				planSteps: []*PlanStep{{
					ID:             "step_1",
					RunID:          "run_1",
					OrganizationID: "org_1",
					Index:          1,
					Title:          "Guarded step",
					Status:         tt.status,
					ApprovalStatus: ApprovalStatusNotRequired,
					CreatedAt:      now,
					UpdatedAt:      now,
				}},
			}
			service := NewService(store, &fakeGateway{})
			session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}

			_, err := service.SkipPlanStep(context.Background(), session, "step_1", "late skip")
			if err == nil || !strings.Contains(err.Error(), "plan step cannot be skipped after execution starts") {
				t.Fatalf("expected skip rejection for %s step, got %v", tt.status, err)
			}
			if store.planSteps[0].Status != tt.status || store.planSteps[0].Error != "" {
				t.Fatalf("skip rejection mutated guarded step: %+v", store.planSteps[0])
			}
		})
	}
}

func TestStartPlanningRunPersistsStructuredToolSteps(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Tools: []Tool{
				{Name: "calculator", Type: "builtin", Enabled: true},
			},
		},
		conversation: &Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
	}
	gateway := &fakeGateway{plainReply: `[{"title":"计算","toolName":"calculator","input":{"expression":"2+3"}}]`}
	service := NewService(store, gateway)

	result, err := service.StartPlanningRun(
		context.Background(),
		auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1", User: auth.User{ID: "user_1"}},
		StartRunRequest{AgentID: "agent_1", ConversationID: "conv_1", Input: "make a plan"},
	)
	if err != nil {
		t.Fatalf("StartPlanningRun returned error: %v", err)
	}
	if len(store.planSteps) != 1 {
		t.Fatalf("expected 1 structured plan step, got %+v", store.planSteps)
	}
	step := store.planSteps[0]
	if step.RunID != result.Run.ID || step.Index != 1 || step.Title != "计算" {
		t.Fatalf("structured step scope/index/title mismatch: %+v", step)
	}
	if step.ToolName != "calculator" {
		t.Fatalf("expected calculator tool name to persist, got %+v", step)
	}
	if step.Input["expression"] != "2+3" {
		t.Fatalf("expected structured input to persist, got %+v", step.Input)
	}
	if len(result.PlanSteps) != 1 || result.PlanSteps[0].ToolName != "calculator" {
		t.Fatalf("expected returned plan steps to include structured tool metadata, got %+v", result.PlanSteps)
	}
}

func TestServiceStartRunWithoutToolsMarksDurableRunFailedOnRunnerError(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
		},
		conversation: &Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
	}
	service := NewService(store, &fakeGateway{plainErr: errors.New("relay down")})

	_, err := service.StartRun(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		StartRunRequest{AgentID: "agent_1", ConversationID: "conv_1", Input: "hello"},
	)
	if err == nil {
		t.Fatal("expected StartRun error")
	}
	if len(store.runs) != 1 {
		t.Fatalf("expected one persisted run, got %+v", store.runs)
	}
	run := store.runs[0]
	if run.Status != RunStatusFailed || run.Error == "" || run.CompletedAt == nil {
		t.Fatalf("expected failed completed run with error, got %+v", run)
	}
	if run.IterationCount != 1 || run.ToolCallCount != 0 {
		t.Fatalf("expected failed no-tool run to record one iteration and zero tool calls, got iterations=%d tools=%d", run.IterationCount, run.ToolCallCount)
	}
}

func TestServiceApproveToolRunExecutesToolAndCompletesDurableRun(t *testing.T) {
	now := time.Now().UTC()
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
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusPendingApproval,
			IterationCount: 1,
			ToolCallCount:  1,
			StartedAt:      now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		toolRuns: []*ToolRun{{
			ID:             "tool_run_pending",
			OrganizationID: "org_1",
			RunID:          "run_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			ToolCallID:     "call_datetime",
			ToolName:       "datetime",
			ToolType:       "builtin",
			Arguments:      map[string]any{},
			Status:         ToolRunStatusPendingApproval,
			ApprovalStatus: ApprovalStatusPending,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		messages: []*Message{{
			ID:             "assistant_tool_call",
			ConversationID: "conv_1",
			OrganizationID: "org_1",
			Role:           "assistant",
			ToolCalls:      []ToolCall{{ID: "call_datetime", Name: "datetime", Arguments: map[string]any{}}},
			CreatedAt:      now,
		}},
	}
	service := NewService(store, &fakeGateway{plainReply: "unused"})

	updated, err := service.ApproveToolRun(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, "tool_run_pending", "operator approved")
	if err != nil {
		t.Fatalf("ApproveToolRun returned error: %v", err)
	}
	if updated.Status != ToolRunStatusCompleted || updated.ApprovalStatus != ApprovalStatusApproved || updated.AttemptCount != 1 || updated.ResultContent == "" {
		t.Fatalf("expected approved tool to execute and complete, got %+v", updated)
	}
	foundToolMessage := false
	for _, message := range store.messages {
		if message.Role == "tool" && message.ToolCallID == "call_datetime" && message.Content != "" {
			foundToolMessage = true
		}
	}
	if !foundToolMessage {
		t.Fatalf("expected persisted tool result message, got %+v", store.messages)
	}
	run := store.runs[0]
	if run.Status != RunStatusCompleted || run.CompletedAt == nil || run.Error != "" {
		t.Fatalf("expected durable run completed after approved tool execution, got %+v", run)
	}
}

func TestServiceApproveToolRunResumesReactLoopToFinalAnswer(t *testing.T) {
	now := time.Now().UTC()
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
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusPendingApproval,
			IterationCount: 1,
			ToolCallCount:  1,
			StartedAt:      now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		toolRuns: []*ToolRun{{
			ID:             "tool_run_pending",
			OrganizationID: "org_1",
			RunID:          "run_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			ToolCallID:     "call_datetime",
			ToolName:       "datetime",
			ToolType:       "builtin",
			Arguments:      map[string]any{},
			Status:         ToolRunStatusPendingApproval,
			ApprovalStatus: ApprovalStatusPending,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		messages: []*Message{
			{
				ID:             "user_1",
				ConversationID: "conv_1",
				OrganizationID: "org_1",
				Role:           "user",
				Content:        "what time is it?",
				CreatedAt:      now,
			},
			{
				ID:             "assistant_tool_call",
				ConversationID: "conv_1",
				OrganizationID: "org_1",
				Role:           "assistant",
				ToolCalls:      []ToolCall{{ID: "call_datetime", Name: "datetime", Arguments: map[string]any{}}},
				CreatedAt:      now,
			},
		},
	}
	gateway := &fakeGateway{
		structured: []*chat.CompletionResponse{
			{Content: "The approved tool result says it is noon.", FinishReason: "stop"},
		},
	}
	service := NewService(store, gateway)

	updated, err := service.ApproveToolRun(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, "tool_run_pending", "operator approved")
	if err != nil {
		t.Fatalf("ApproveToolRun returned error: %v", err)
	}
	if updated.Status != ToolRunStatusCompleted || updated.ResultContent == "" {
		t.Fatalf("expected approved tool to execute before loop resume, got %+v", updated)
	}
	if gateway.structuredCalls != 1 {
		t.Fatalf("expected approval resume to call model once for final answer, got %d calls", gateway.structuredCalls)
	}
	if len(gateway.lastStructuredMessages) == 0 || gateway.lastStructuredMessages[len(gateway.lastStructuredMessages)-1].Role != "tool" {
		t.Fatalf("expected resumed model call to include latest tool result, got %+v", gateway.lastStructuredMessages)
	}
	lastMessage := store.messages[len(store.messages)-1]
	if lastMessage.Role != "assistant" || lastMessage.Content != "The approved tool result says it is noon." {
		t.Fatalf("expected final assistant answer after approved tool, got messages=%+v", store.messages)
	}
	run := store.runs[0]
	if run.Status != RunStatusCompleted || run.FinalMessageID != lastMessage.ID || run.CompletedAt == nil || run.Error != "" {
		t.Fatalf("expected durable run completed with final assistant message, got %+v", run)
	}
}

func TestServiceApproveToolRunResumePersistsNextPendingApprovalToolRun(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Tools: []Tool{
				{Name: "datetime", Type: "builtin", Enabled: true, RequiresApproval: true},
				{Name: "write_file", Type: "builtin", Enabled: true, RequiresApproval: true},
			},
		},
		conversation: &Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusPendingApproval,
			IterationCount: 1,
			ToolCallCount:  1,
			StartedAt:      now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		toolRuns: []*ToolRun{{
			ID:             "tool_run_datetime",
			OrganizationID: "org_1",
			RunID:          "run_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			ToolCallID:     "call_datetime",
			ToolName:       "datetime",
			ToolType:       "builtin",
			Arguments:      map[string]any{},
			Status:         ToolRunStatusPendingApproval,
			ApprovalStatus: ApprovalStatusPending,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		messages: []*Message{
			{
				ID:             "user_1",
				ConversationID: "conv_1",
				OrganizationID: "org_1",
				Role:           "user",
				Content:        "check time then write the result",
				CreatedAt:      now,
			},
			{
				ID:             "assistant_tool_call",
				ConversationID: "conv_1",
				OrganizationID: "org_1",
				Role:           "assistant",
				ToolCalls:      []ToolCall{{ID: "call_datetime", Name: "datetime", Arguments: map[string]any{}}},
				CreatedAt:      now,
			},
		},
	}
	gateway := &fakeGateway{
		structured: []*chat.CompletionResponse{
			{
				ToolCalls: []chat.ToolCall{
					{ID: "call_write_file", Type: "function", Function: chat.ToolFunction{Name: "write_file", Arguments: `{"path":"result.txt","content":"noon"}`}},
				},
				FinishReason: "tool_calls",
			},
		},
	}
	service := NewService(store, gateway)

	updated, err := service.ApproveToolRun(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, "tool_run_datetime", "operator approved")
	if err != nil {
		t.Fatalf("ApproveToolRun returned error: %v", err)
	}
	if updated.Status != ToolRunStatusCompleted || updated.ToolName != "datetime" {
		t.Fatalf("expected first approved tool to complete before next pause, got %+v", updated)
	}
	if len(store.toolRuns) != 2 {
		t.Fatalf("expected second pending approval tool run, got %+v", store.toolRuns)
	}
	nextToolRun := store.toolRuns[1]
	if nextToolRun.ToolName != "write_file" || nextToolRun.Status != ToolRunStatusPendingApproval || nextToolRun.ApprovalStatus != ApprovalStatusPending || nextToolRun.AttemptCount != 0 {
		t.Fatalf("expected write_file pending approval evidence, got %+v", nextToolRun)
	}
	run := store.runs[0]
	if run.Status != RunStatusPendingApproval || run.IterationCount != 2 || run.ToolCallCount != 2 || run.Error != "" || run.CompletedAt != nil {
		t.Fatalf("expected durable run to remain pending for next approval, got %+v", run)
	}
}

func TestServiceApproveToolRunResumeStopsWhenTokenBudgetExceeded(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config: Config{
				TokenBudget: 1000,
			},
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
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusPendingApproval,
			IterationCount: 1,
			ToolCallCount:  1,
			StartedAt:      now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		toolRuns: []*ToolRun{{
			ID:             "tool_run_pending",
			OrganizationID: "org_1",
			RunID:          "run_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			ToolCallID:     "call_datetime",
			ToolName:       "datetime",
			ToolType:       "builtin",
			Arguments:      map[string]any{},
			Status:         ToolRunStatusPendingApproval,
			ApprovalStatus: ApprovalStatusPending,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		messages: []*Message{
			{
				ID:             "user_1",
				ConversationID: "conv_1",
				OrganizationID: "org_1",
				Role:           "user",
				Content:        "what time is it?",
				CreatedAt:      now,
			},
			{
				ID:             "assistant_tool_call",
				ConversationID: "conv_1",
				OrganizationID: "org_1",
				Role:           "assistant",
				ToolCalls:      []ToolCall{{ID: "call_datetime", Name: "datetime", Arguments: map[string]any{}}},
				CreatedAt:      now,
			},
		},
	}
	gateway := &fakeGateway{
		structured: []*chat.CompletionResponse{
			{Content: "This final answer is too expensive.", FinishReason: "stop", Usage: &chat.CompletionUsage{TotalTokens: 1200}},
		},
	}
	service := NewService(store, gateway)

	_, err := service.ApproveToolRun(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, "tool_run_pending", "operator approved")
	if !errors.Is(err, ErrTokenBudgetExceeded) {
		t.Fatalf("expected ErrTokenBudgetExceeded, got %v", err)
	}
	if gateway.structuredCalls != 1 {
		t.Fatalf("expected resume to call model once before budget stop, got %d calls", gateway.structuredCalls)
	}
	for _, message := range store.messages {
		if message.Role == "assistant" && message.Content == "This final answer is too expensive." {
			t.Fatalf("budget-exceeded resume should not persist final assistant answer, got messages=%+v", store.messages)
		}
	}
	run := store.runs[0]
	if run.Status != RunStatusTokenBudgetExceeded || run.IterationCount != 2 || run.ToolCallCount != 1 || !strings.Contains(run.Error, "token_budget_exceeded") {
		t.Fatalf("expected durable run token budget evidence after resume, got %+v", run)
	}
}

func TestServiceContinueRunWithTokenBudgetResumesTokenBudgetExceededRun(t *testing.T) {
	now := time.Now().UTC()
	completedAt := now.Add(time.Minute)
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config: Config{
				TokenBudget: 1000,
			},
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
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusTokenBudgetExceeded,
			IterationCount: 1,
			ToolCallCount:  1,
			Error:          "token_budget_exceeded: used 1200 tokens exceeds budget 1000",
			CompletedAt:    &completedAt,
			StartedAt:      now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		messages: []*Message{
			{
				ID:             "user_1",
				ConversationID: "conv_1",
				OrganizationID: "org_1",
				Role:           "user",
				Content:        "what time is it?",
				CreatedAt:      now,
			},
			{
				ID:             "assistant_tool_call",
				ConversationID: "conv_1",
				OrganizationID: "org_1",
				Role:           "assistant",
				ToolCalls:      []ToolCall{{ID: "call_datetime", Name: "datetime", Arguments: map[string]any{}}},
				CreatedAt:      now,
			},
			{
				ID:             "tool_1",
				ConversationID: "conv_1",
				OrganizationID: "org_1",
				Role:           "tool",
				Content:        "Current time: noon",
				ToolCallID:     "call_datetime",
				CreatedAt:      now,
			},
		},
	}
	gateway := &fakeGateway{
		structured: []*chat.CompletionResponse{
			{Content: "Final answer after budget increase.", FinishReason: "stop", Usage: &chat.CompletionUsage{TotalTokens: 1300}},
		},
	}
	service := NewService(store, gateway)

	result, err := service.ContinueRunWithTokenBudget(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, "run_1", 2500)
	if err != nil {
		t.Fatalf("ContinueRunWithTokenBudget returned error: %v", err)
	}
	if result == nil || result.Message == nil || result.Message.Content != "Final answer after budget increase." {
		t.Fatalf("expected final assistant result after budget increase, got %+v", result)
	}
	if gateway.structuredCalls != 1 {
		t.Fatalf("expected one resumed structured call, got %d", gateway.structuredCalls)
	}
	if store.agent.Config.TokenBudget != 1000 {
		t.Fatalf("continue budget override should not mutate agent config, got %d", store.agent.Config.TokenBudget)
	}
	run := store.runs[0]
	if run.Status != RunStatusCompleted || run.Error != "" || run.CompletedAt == nil || run.IterationCount != 2 || run.ToolCallCount != 1 {
		t.Fatalf("expected continued run to complete cleanly, got %+v", run)
	}
	if store.messages[len(store.messages)-1].Content != "Final answer after budget increase." {
		t.Fatalf("expected final answer persisted on same conversation, got %+v", store.messages)
	}
}

func TestServiceContinueRunWithTokenBudgetRejectsNonBudgetExceededRun(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusFailed,
			StartedAt:      now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	service := NewService(store, &fakeGateway{})

	result, err := service.ContinueRunWithTokenBudget(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, "run_1", 2500)
	if err == nil || !strings.Contains(err.Error(), "agent run is not token budget exceeded") {
		t.Fatalf("expected non-budget run rejection, got result=%+v err=%v", result, err)
	}
	if store.runs[0].Status != RunStatusFailed {
		t.Fatalf("expected rejected continue to leave run unchanged, got %+v", store.runs[0])
	}
}

func TestServiceContinueRunWithTokenBudgetRejectsOutOfRangeBudget(t *testing.T) {
	for _, tokenBudget := range []int{999, 1_000_001} {
		name := "below_minimum"
		if tokenBudget > maxTokenBudget {
			name = "above_maximum"
		}
		t.Run(name, func(t *testing.T) {
			now := time.Now().UTC()
			completedAt := now.Add(time.Minute)
			store := &fakeStore{
				runs: []*Run{{
					ID:             "run_1",
					OrganizationID: "org_1",
					ConversationID: "conv_1",
					AgentID:        "agent_1",
					UserID:         "user_1",
					Status:         RunStatusTokenBudgetExceeded,
					Error:          "token_budget_exceeded: used 1200 tokens exceeds budget 1000",
					CompletedAt:    &completedAt,
					StartedAt:      now,
					CreatedAt:      now,
					UpdatedAt:      now,
				}},
			}
			gateway := &fakeGateway{}
			service := NewService(store, gateway)

			result, err := service.ContinueRunWithTokenBudget(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, "run_1", tokenBudget)
			if err == nil || !strings.Contains(err.Error(), "tokenBudget must be between 1000 and 1000000") {
				t.Fatalf("expected budget range rejection, got result=%+v err=%v", result, err)
			}
			run := store.runs[0]
			if run.Status != RunStatusTokenBudgetExceeded || run.Error == "" || run.CompletedAt == nil {
				t.Fatalf("expected rejected budget to leave run unchanged, got %+v", run)
			}
			if gateway.structuredCalls != 0 {
				t.Fatalf("expected invalid budget to skip model resume, got %d calls", gateway.structuredCalls)
			}
		})
	}
}

func TestServiceApproveToolRunResumeStopsWhenMaxIterationsReached(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config: Config{
				MaxIterations: 1,
			},
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
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusPendingApproval,
			IterationCount: 1,
			ToolCallCount:  1,
			StartedAt:      now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		toolRuns: []*ToolRun{{
			ID:             "tool_run_pending",
			OrganizationID: "org_1",
			RunID:          "run_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			ToolCallID:     "call_datetime",
			ToolName:       "datetime",
			ToolType:       "builtin",
			Arguments:      map[string]any{},
			Status:         ToolRunStatusPendingApproval,
			ApprovalStatus: ApprovalStatusPending,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		messages: []*Message{
			{
				ID:             "user_1",
				ConversationID: "conv_1",
				OrganizationID: "org_1",
				Role:           "user",
				Content:        "what time is it?",
				CreatedAt:      now,
			},
			{
				ID:             "assistant_tool_call",
				ConversationID: "conv_1",
				OrganizationID: "org_1",
				Role:           "assistant",
				ToolCalls:      []ToolCall{{ID: "call_datetime", Name: "datetime", Arguments: map[string]any{}}},
				CreatedAt:      now,
			},
		},
	}
	gateway := &fakeGateway{
		structured: []*chat.CompletionResponse{
			{Content: "This final answer should not be requested.", FinishReason: "stop"},
		},
	}
	service := NewService(store, gateway)

	_, err := service.ApproveToolRun(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, "tool_run_pending", "operator approved")
	if !errors.Is(err, ErrMaxIterationsExceeded) {
		t.Fatalf("expected ErrMaxIterationsExceeded, got %v", err)
	}
	if gateway.structuredCalls != 0 {
		t.Fatalf("expected max-iteration guard to stop before resumed model call, got %d calls", gateway.structuredCalls)
	}
	run := store.runs[0]
	if run.Status != RunStatusMaxIterationsReached || run.IterationCount != 1 || run.ToolCallCount != 1 || !strings.Contains(run.Error, ErrMaxIterationsExceeded.Error()) {
		t.Fatalf("expected durable run max-iterations evidence after resume, got %+v", run)
	}
}

func TestServiceRetryToolRunReexecutesFailedToolAndRestoresRunDetail(t *testing.T) {
	now := time.Now().UTC()
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
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusFailed,
			Error:          "tool datetime failed: temporary",
			IterationCount: 1,
			ToolCallCount:  1,
			StartedAt:      now,
			CompletedAt:    &now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		toolRuns: []*ToolRun{{
			ID:             "tool_run_failed",
			OrganizationID: "org_1",
			RunID:          "run_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			ToolCallID:     "call_datetime",
			ToolName:       "datetime",
			ToolType:       "builtin",
			Arguments:      map[string]any{},
			Status:         ToolRunStatusFailed,
			ApprovalStatus: ApprovalStatusNotRequired,
			AttemptCount:   1,
			Error:          "temporary",
			StartedAt:      &now,
			CompletedAt:    &now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	service := NewService(store, &fakeGateway{})
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}

	updated, err := service.RetryToolRun(context.Background(), session, "tool_run_failed")
	if err != nil {
		t.Fatalf("RetryToolRun returned error: %v", err)
	}
	if updated.Status != ToolRunStatusCompleted || updated.AttemptCount != 2 || updated.Error != "" || updated.ResultContent == "" {
		t.Fatalf("expected failed tool to be re-executed successfully, got %+v", updated)
	}

	detail, err := service.GetRunWithMessages(context.Background(), session, "run_1")
	if err != nil {
		t.Fatalf("GetRunWithMessages returned error: %v", err)
	}
	if detail.Run.Status != RunStatusCompleted || detail.Run.Error != "" || detail.Run.CompletedAt == nil {
		t.Fatalf("expected recovered completed run detail, got %+v", detail.Run)
	}
	if len(detail.ToolRuns) != 1 || detail.ToolRuns[0].Status != ToolRunStatusCompleted {
		t.Fatalf("expected recovered completed tool run detail, got %+v", detail.ToolRuns)
	}
	foundToolMessage := false
	for _, message := range detail.Messages {
		if message.Role == "tool" && message.ToolCallID == "call_datetime" {
			foundToolMessage = true
		}
	}
	if !foundToolMessage {
		t.Fatalf("expected restored detail to include retried tool result message, got %+v", detail.Messages)
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

	toolMetricBefore := testutil.ToFloat64(metrics.AgentToolCallTotal.WithLabelValues("web_search", string(ToolRunStatusFailed)))
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
	toolMetricAfter := testutil.ToFloat64(metrics.AgentToolCallTotal.WithLabelValues("web_search", string(ToolRunStatusFailed)))
	if toolMetricAfter != toolMetricBefore+1 {
		t.Fatalf("expected failed tool metric to preserve tool name, before=%v after=%v", toolMetricBefore, toolMetricAfter)
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

func TestRunWithToolsRecordsPendingApprovalMetrics(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Tools: []Tool{
				{Name: "write_file", Type: "builtin", Enabled: true, RequiresApproval: true},
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
					{ID: "call_write_file", Type: "function", Function: chat.ToolFunction{Name: "write_file", Arguments: "{}"}},
				},
				FinishReason: "tool_calls",
			},
		},
	}
	service := NewService(store, gateway)

	runBefore := testutil.ToFloat64(metrics.AgentRunTotal.WithLabelValues(string(RunStatusPendingApproval)))
	toolBefore := testutil.ToFloat64(metrics.AgentToolCallTotal.WithLabelValues("write_file", string(ToolRunStatusPendingApproval)))
	_, err := service.SendMessage(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, "conv_1", "needs approval")
	if !errors.Is(err, ErrToolApprovalRequired) {
		t.Fatalf("expected ErrToolApprovalRequired, got %v", err)
	}

	runAfter := testutil.ToFloat64(metrics.AgentRunTotal.WithLabelValues(string(RunStatusPendingApproval)))
	if runAfter != runBefore+1 {
		t.Fatalf("expected pending approval run metric increment, before=%v after=%v", runBefore, runAfter)
	}
	toolAfter := testutil.ToFloat64(metrics.AgentToolCallTotal.WithLabelValues("write_file", string(ToolRunStatusPendingApproval)))
	if toolAfter != toolBefore+1 {
		t.Fatalf("expected pending approval tool metric increment, before=%v after=%v", toolBefore, toolAfter)
	}
}

func TestServiceRejectToolRunRecordsRejectedToolAndFailedRunMetrics(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusPendingApproval,
			IterationCount: 3,
			ToolCallCount:  1,
			StartedAt:      now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		toolRuns: []*ToolRun{{
			ID:             "tool_run_pending",
			OrganizationID: "org_1",
			RunID:          "run_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			ToolCallID:     "call_shell",
			ToolName:       "shell_exec",
			ToolType:       "builtin",
			Arguments:      map[string]any{},
			Status:         ToolRunStatusPendingApproval,
			ApprovalStatus: ApprovalStatusPending,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	service := NewService(store, &fakeGateway{})

	toolBefore := testutil.ToFloat64(metrics.AgentToolCallTotal.WithLabelValues("shell_exec", string(ToolRunStatusRejected)))
	runBefore := testutil.ToFloat64(metrics.AgentRunTotal.WithLabelValues(string(RunStatusFailed)))
	updated, err := service.RejectToolRun(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, "tool_run_pending", "too risky")
	if err != nil {
		t.Fatalf("RejectToolRun returned error: %v", err)
	}
	if updated.Status != ToolRunStatusRejected || updated.ApprovalStatus != ApprovalStatusRejected {
		t.Fatalf("expected rejected tool run, got %+v", updated)
	}
	if store.runs[0].Status != RunStatusFailed || store.runs[0].CompletedAt == nil {
		t.Fatalf("expected rejected tool to fail parent run, got %+v", store.runs[0])
	}

	toolAfter := testutil.ToFloat64(metrics.AgentToolCallTotal.WithLabelValues("shell_exec", string(ToolRunStatusRejected)))
	if toolAfter != toolBefore+1 {
		t.Fatalf("expected rejected tool metric increment, before=%v after=%v", toolBefore, toolAfter)
	}
	runAfter := testutil.ToFloat64(metrics.AgentRunTotal.WithLabelValues(string(RunStatusFailed)))
	if runAfter != runBefore+1 {
		t.Fatalf("expected failed run metric increment, before=%v after=%v", runBefore, runAfter)
	}
	if count := testutil.CollectAndCount(metrics.AgentIterationCount, "agent_iteration_count"); count == 0 {
		t.Fatal("expected agent iteration count metric to be collectable")
	}
}

func TestToolApprovalPolicyModesAndRiskLevels(t *testing.T) {
	tests := []struct {
		name             string
		config           Config
		tool             Tool
		toolCallName     string
		wantApproval     bool
		wantRisk         string
		wantToolExecuted bool
	}{
		{
			name:             "all mode requires approval for safe tool",
			config:           Config{ApprovalMode: ApprovalModeAll},
			tool:             Tool{Name: "datetime", Type: "builtin", Enabled: true, RiskLevel: ToolRiskSafe},
			toolCallName:     "datetime",
			wantApproval:     true,
			wantRisk:         ToolRiskSafe,
			wantToolExecuted: false,
		},
		{
			name:             "none mode auto executes dangerous tool",
			config:           Config{ApprovalMode: ApprovalModeNone},
			tool:             Tool{Name: "datetime", Type: "builtin", Enabled: true, RiskLevel: ToolRiskDangerous},
			toolCallName:     "datetime",
			wantApproval:     false,
			wantRisk:         ToolRiskDangerous,
			wantToolExecuted: true,
		},
		{
			name:             "tiered mode auto executes safe tool",
			config:           Config{ApprovalMode: ApprovalModeTiered},
			tool:             Tool{Name: "datetime", Type: "builtin", Enabled: true, RiskLevel: ToolRiskSafe},
			toolCallName:     "datetime",
			wantApproval:     false,
			wantRisk:         ToolRiskSafe,
			wantToolExecuted: true,
		},
		{
			name:             "tiered mode requires first approval for medium tool",
			config:           Config{ApprovalMode: ApprovalModeTiered},
			tool:             Tool{Name: "write_note", Type: "builtin", Enabled: true, RiskLevel: ToolRiskMedium},
			toolCallName:     "write_note",
			wantApproval:     true,
			wantRisk:         ToolRiskMedium,
			wantToolExecuted: false,
		},
		{
			name:             "tiered mode requires approval for dangerous tool",
			config:           Config{ApprovalMode: ApprovalModeTiered},
			tool:             Tool{Name: "execute_code", Type: "builtin", Enabled: true, RiskLevel: ToolRiskDangerous},
			toolCallName:     "execute_code",
			wantApproval:     true,
			wantRisk:         ToolRiskDangerous,
			wantToolExecuted: false,
		},
		{
			name: "custom mode auto executes overridden dangerous tool",
			config: Config{
				ApprovalMode: ApprovalModeCustom,
				ToolApprovalOverrides: map[string]ToolApprovalOverride{
					"datetime": {RequiresApproval: boolPointer(false)},
				},
			},
			tool:             Tool{Name: "datetime", Type: "builtin", Enabled: true, RiskLevel: ToolRiskDangerous},
			toolCallName:     "datetime",
			wantApproval:     false,
			wantRisk:         ToolRiskDangerous,
			wantToolExecuted: true,
		},
		{
			name: "custom mode requires approval for overridden safe tool",
			config: Config{
				ApprovalMode: ApprovalModeCustom,
				ToolApprovalOverrides: map[string]ToolApprovalOverride{
					"datetime": {RequiresApproval: boolPointer(true)},
				},
			},
			tool:             Tool{Name: "datetime", Type: "builtin", Enabled: true, RiskLevel: ToolRiskSafe},
			toolCallName:     "datetime",
			wantApproval:     true,
			wantRisk:         ToolRiskSafe,
			wantToolExecuted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeStore{
				agent: &Agent{
					ID:             "agent_1",
					OrganizationID: "org_1",
					UserID:         "user_1",
					Model:          "gpt-4o-mini",
					Config:         tt.config,
					Tools:          []Tool{tt.tool},
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
							{ID: "call_approval_policy", Type: "function", Function: chat.ToolFunction{Name: tt.toolCallName, Arguments: "{}"}},
						},
						FinishReason: "tool_calls",
					},
					{Content: "final after tool", FinishReason: "stop"},
				},
			}
			executor := &ToolExecutor{builtinTools: map[string]mcp.BuiltinTool{
				tt.toolCallName: &recordingBuiltinTool{name: tt.toolCallName},
			}}
			runner := NewRunner(store, gateway, executor, nil, DefaultRunnerConfig())

			result, err := runner.RunWithTools(
				context.Background(),
				auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
				store.agent,
				store.conversation.ID,
				"run tool",
				nil,
			)
			if tt.wantApproval {
				if !errors.Is(err, ErrToolApprovalRequired) {
					t.Fatalf("expected ErrToolApprovalRequired, got result=%+v err=%v", result, err)
				}
			} else if err != nil {
				t.Fatalf("RunWithTools returned error: %v", err)
			}

			if len(store.toolRuns) != 1 {
				t.Fatalf("expected one persisted tool run, got %+v", store.toolRuns)
			}
			toolRun := store.toolRuns[0]
			if toolRun.RiskLevel != tt.wantRisk {
				t.Fatalf("expected risk level %q, got %+v", tt.wantRisk, toolRun)
			}
			if tt.wantApproval {
				if toolRun.Status != ToolRunStatusPendingApproval || toolRun.ApprovalStatus != ApprovalStatusPending || toolRun.AttemptCount != 0 {
					t.Fatalf("expected pending approval tool run, got %+v", toolRun)
				}
			} else {
				if toolRun.Status != ToolRunStatusCompleted || toolRun.ApprovalStatus != ApprovalStatusNotRequired || toolRun.AttemptCount != 1 {
					t.Fatalf("expected completed auto-executed tool run, got %+v", toolRun)
				}
			}
			sawToolMessage := false
			for _, message := range store.messages {
				if message.Role == "tool" {
					sawToolMessage = true
				}
			}
			if sawToolMessage != tt.wantToolExecuted {
				t.Fatalf("tool execution mismatch: sawToolMessage=%v want=%v messages=%+v", sawToolMessage, tt.wantToolExecuted, store.messages)
			}
		})
	}
}

func TestTieredMediumToolAutoExecutesAfterPriorApproval(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config:         Config{ApprovalMode: ApprovalModeTiered},
			Tools: []Tool{
				{Name: "datetime", Type: "builtin", Enabled: true, RiskLevel: ToolRiskMedium},
			},
		},
		conversation: &Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
		runs: []*Run{{
			ID:             "prior_run",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusCompleted,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		toolRuns: []*ToolRun{{
			ID:             "prior_approved_tool_run",
			OrganizationID: "org_1",
			RunID:          "prior_run",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			ToolName:       "datetime",
			RiskLevel:      ToolRiskMedium,
			Status:         ToolRunStatusCompleted,
			ApprovalStatus: ApprovalStatusApproved,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	gateway := &fakeGateway{
		structured: []*chat.CompletionResponse{
			{
				ToolCalls: []chat.ToolCall{
					{ID: "call_datetime_medium", Type: "function", Function: chat.ToolFunction{Name: "datetime", Arguments: "{}"}},
				},
				FinishReason: "tool_calls",
			},
			{Content: "final after medium tool", FinishReason: "stop"},
		},
	}
	runner := NewRunner(store, gateway, NewToolExecutor(nil), nil, DefaultRunnerConfig())

	result, err := runner.RunWithTools(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		store.agent,
		store.conversation.ID,
		"run medium again",
		nil,
	)
	if err != nil {
		t.Fatalf("RunWithTools returned error: %v", err)
	}
	if result == nil || result.Message == nil || result.Message.Content != "final after medium tool" {
		t.Fatalf("expected final message after auto-executed medium tool, got %+v", result)
	}
	toolRun := store.toolRuns[len(store.toolRuns)-1]
	if toolRun.Status != ToolRunStatusCompleted || toolRun.ApprovalStatus != ApprovalStatusNotRequired || toolRun.AttemptCount != 1 {
		t.Fatalf("expected medium tool to auto-execute after prior approval, got %+v", toolRun)
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

func TestRunWithToolsStoresLongTermInteractionMemoryWhenEnabled(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config:         Config{EnableMemory: true},
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
			{Content: "Use the tenant-safe migration guard.", FinishReason: "stop"},
		},
	}
	embedder := &fakeAgentMemoryEmbedder{
		embeddings: map[string][]float32{
			"User: What should we remember about migrations?\nAssistant: Use the tenant-safe migration guard.": {0.4, 0.6},
		},
	}
	runner := NewRunner(store, gateway, NewToolExecutor(nil), nil, DefaultRunnerConfig())
	runner.SetMemoryEmbedder(embedder)

	_, err := runner.RunWithTools(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		store.agent,
		store.conversation.ID,
		"What should we remember about migrations?",
		nil,
	)
	if err != nil {
		t.Fatalf("RunWithTools returned error: %v", err)
	}
	if len(store.memories) != 1 {
		t.Fatalf("expected one automatic long-term memory, got %+v", store.memories)
	}
	created := store.memories[0]
	if created.Type != MemoryTypeLongTerm || created.AgentID != "agent_1" || created.UserID != "user_1" || created.OrganizationID != "org_1" {
		t.Fatalf("expected scoped long-term agent memory, got %+v", created)
	}
	if created.Content != "User: What should we remember about migrations?\nAssistant: Use the tenant-safe migration guard." {
		t.Fatalf("unexpected automatic memory content: %q", created.Content)
	}
	if created.Importance != 3 || created.Metadata["source"] != "agent_run" || created.Metadata["conversation_id"] != "conv_1" {
		t.Fatalf("expected automatic memory metadata and default importance, got %+v", created)
	}
	if !reflect.DeepEqual(store.createMemoryEmbedding, []float32{0.4, 0.6}) {
		t.Fatalf("expected embedded automatic memory, got %+v", store.createMemoryEmbedding)
	}
	if len(embedder.texts) == 0 || embedder.texts[len(embedder.texts)-1] != created.Content {
		t.Fatalf("expected automatic memory content to be embedded, got %+v", embedder.texts)
	}
}

func TestRunWithToolsDeduplicatesAutomaticLongTermInteractionMemory(t *testing.T) {
	content := "User: What should we remember about migrations?\nAssistant: Use the tenant-safe migration guard."
	oldUpdatedAt := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config:         Config{EnableMemory: true},
		},
		conversation: &Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
		memories: []*Memory{{
			ID:             "memory_existing",
			OrganizationID: "org_1",
			UserID:         "user_1",
			AgentID:        "agent_1",
			Type:           MemoryTypeLongTerm,
			Content:        content,
			Importance:     3,
			Metadata: map[string]any{
				"source":          "agent_run",
				"conversation_id": "conv_1",
			},
			CreatedAt: oldUpdatedAt,
			UpdatedAt: oldUpdatedAt,
		}},
	}
	gateway := &fakeGateway{
		structured: []*chat.CompletionResponse{
			{Content: "Use the tenant-safe migration guard.", FinishReason: "stop"},
		},
	}
	embedder := &fakeAgentMemoryEmbedder{
		embeddings: map[string][]float32{
			content: {0.4, 0.6},
		},
	}
	runner := NewRunner(store, gateway, NewToolExecutor(nil), nil, DefaultRunnerConfig())
	runner.SetMemoryEmbedder(embedder)

	_, err := runner.RunWithTools(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		store.agent,
		store.conversation.ID,
		"What should we remember about migrations?",
		nil,
	)
	if err != nil {
		t.Fatalf("RunWithTools returned error: %v", err)
	}
	if len(store.memories) != 1 {
		t.Fatalf("expected existing automatic memory to be updated instead of duplicated, got %+v", store.memories)
	}
	updated := store.memories[0]
	if updated.ID != "memory_existing" || updated.UpdatedAt.Equal(oldUpdatedAt) {
		t.Fatalf("expected existing memory to be refreshed, got %+v", updated)
	}
	if !reflect.DeepEqual(store.updateMemoryEmbedding, []float32{0.4, 0.6}) {
		t.Fatalf("expected duplicate memory refresh to update embedding, got %+v", store.updateMemoryEmbedding)
	}
}

func TestRunWithToolsUsesOnlyRecentShortTermMessages(t *testing.T) {
	history := make([]*Message, 0, 60)
	baseTime := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 60; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		history = append(history, &Message{
			ID:             "history_msg",
			ConversationID: "conv_1",
			OrganizationID: "org_1",
			Role:           role,
			Content:        "history message " + string(rune('A'+i%26)),
			CreatedAt:      baseTime.Add(time.Duration(i) * time.Minute),
		})
	}
	history[10].Content = "old message outside short term window"
	history[11].Content = "first retained history message"
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
		messages: history,
	}
	gateway := &fakeGateway{
		structured: []*chat.CompletionResponse{
			{Content: "short term bounded answer", FinishReason: "stop"},
		},
	}
	runner := NewRunner(store, gateway, NewToolExecutor(nil), nil, DefaultRunnerConfig())

	_, err := runner.RunWithTools(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		store.agent,
		store.conversation.ID,
		"current request stays in context",
		nil,
	)
	if err != nil {
		t.Fatalf("RunWithTools returned error: %v", err)
	}
	if len(gateway.lastStructuredMessages) != 50 {
		t.Fatalf("expected short-term context to keep 50 messages, got %d", len(gateway.lastStructuredMessages))
	}
	prompt := chatMessagesContent(gateway.lastStructuredMessages)
	if strings.Contains(prompt, "old message outside short term window") {
		t.Fatalf("expected old message to be pruned from short-term context, got %q", prompt)
	}
	if !strings.Contains(prompt, "first retained history message") || !strings.Contains(prompt, "current request stays in context") {
		t.Fatalf("expected recent history and current request to remain, got %q", prompt)
	}
}

func TestRunWithToolsLimitsShortTermMessagesByEstimatedTokens(t *testing.T) {
	longContent := strings.Repeat("x", 45_000)
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
		messages: []*Message{
			{
				ID:             "old_large_1",
				ConversationID: "conv_1",
				OrganizationID: "org_1",
				Role:           "user",
				Content:        "oversized old message one " + longContent,
				CreatedAt:      time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC),
			},
			{
				ID:             "old_large_2",
				ConversationID: "conv_1",
				OrganizationID: "org_1",
				Role:           "assistant",
				Content:        "oversized old message two " + longContent,
				CreatedAt:      time.Date(2026, 6, 8, 10, 1, 0, 0, time.UTC),
			},
		},
	}
	gateway := &fakeGateway{
		structured: []*chat.CompletionResponse{
			{Content: "token bounded answer", FinishReason: "stop"},
		},
	}
	runner := NewRunner(store, gateway, NewToolExecutor(nil), nil, DefaultRunnerConfig())

	_, err := runner.RunWithTools(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		store.agent,
		store.conversation.ID,
		"compact current request",
		nil,
	)
	if err != nil {
		t.Fatalf("RunWithTools returned error: %v", err)
	}
	prompt := chatMessagesContent(gateway.lastStructuredMessages)
	if strings.Contains(prompt, "oversized old message") {
		t.Fatalf("expected oversized history to be pruned by token budget, got %q", prompt[:120])
	}
	if !strings.Contains(prompt, "compact current request") {
		t.Fatalf("expected current request to remain after token pruning, got %q", prompt)
	}
}

func TestRunnerInjectsUserManagedAgentMemoriesIntoPrompt(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config:         Config{EnableMemory: true},
		},
		conversation: &Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
		memories: []*Memory{
			{
				ID:             "memory_match",
				OrganizationID: "org_1",
				UserID:         "user_1",
				AgentID:        "agent_1",
				Type:           MemoryTypeUserManaged,
				Content:        "Always answer concise style questions in bullet points.",
			},
			{
				ID:             "memory_other_agent",
				OrganizationID: "org_1",
				UserID:         "user_1",
				AgentID:        "agent_2",
				Type:           MemoryTypeUserManaged,
				Content:        "Leaked other agent memory about concise style.",
			},
			{
				ID:             "memory_other_user",
				OrganizationID: "org_1",
				UserID:         "user_2",
				AgentID:        "agent_1",
				Type:           MemoryTypeUserManaged,
				Content:        "Leaked other user memory about concise style.",
			},
		},
	}
	gateway := &fakeGateway{plainReply: "ok"}
	runner := NewRunner(store, gateway, NewToolExecutor(nil), nil, DefaultRunnerConfig())

	result, err := runner.Run(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		store.agent,
		store.conversation.ID,
		"concise style",
	)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result == nil || !result.UsedMemory {
		t.Fatalf("expected user-managed memory to be used, got %+v", result)
	}
	if store.listMemoryOrganizationID != "org_1" || store.listMemoryUserID != "user_1" || store.listMemoryAgentID != "agent_1" {
		t.Fatalf("expected scoped memory search, got org=%q user=%q agent=%q", store.listMemoryOrganizationID, store.listMemoryUserID, store.listMemoryAgentID)
	}
	if store.listMemoryQuery != "concise style" || store.listMemoryLimit != 5 {
		t.Fatalf("expected query and limit to reach store, got query=%q limit=%d", store.listMemoryQuery, store.listMemoryLimit)
	}
	prompt := chatMessagesContent(gateway.lastPlainMessages)
	if !strings.Contains(prompt, "Always answer concise style questions in bullet points.") {
		t.Fatalf("expected matching user-managed memory in prompt, got %q", prompt)
	}
	if strings.Contains(prompt, "Leaked other agent memory") || strings.Contains(prompt, "Leaked other user memory") {
		t.Fatalf("prompt leaked out-of-scope memory: %q", prompt)
	}
}

func TestRunnerPrefersVectorAgentMemorySearchWhenEmbedderConfigured(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config:         Config{EnableMemory: true},
		},
		conversation: &Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
		memories: []*Memory{
			{
				ID:             "fallback_memory",
				OrganizationID: "org_1",
				UserID:         "user_1",
				AgentID:        "agent_1",
				Type:           MemoryTypeUserManaged,
				Content:        "Fallback query memory should not be injected.",
			},
		},
		searchMemoryResults: []*MemorySearchResult{
			{
				Memory: Memory{
					ID:             "vector_memory",
					OrganizationID: "org_1",
					UserID:         "user_1",
					AgentID:        "agent_1",
					Type:           MemoryTypeUserManaged,
					Content:        "Vector-selected preference should be injected.",
				},
				Score: 0.92,
			},
		},
	}
	gateway := &fakeGateway{plainReply: "ok"}
	runner := NewRunner(store, gateway, NewToolExecutor(nil), nil, DefaultRunnerConfig())
	runner.SetMemoryEmbedder(&fakeAgentMemoryEmbedder{
		embeddings: map[string][]float32{"concise style": {0.25, 0.75}},
	})

	result, err := runner.Run(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		store.agent,
		store.conversation.ID,
		"concise style",
	)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result == nil || !result.UsedMemory || !result.MemorySearched || result.MemoryResultCount != 1 {
		t.Fatalf("expected one vector memory result, got %+v", result)
	}
	if store.listMemoryQuery != "" {
		t.Fatalf("expected vector memory search to avoid query fallback, got fallback query %q", store.listMemoryQuery)
	}
	if store.searchMemoryAgentID != "agent_1" || store.searchMemoryLimit != 5 || store.searchMemoryMinScore != 0.5 {
		t.Fatalf("unexpected vector search request: agent=%q limit=%d minScore=%v", store.searchMemoryAgentID, store.searchMemoryLimit, store.searchMemoryMinScore)
	}
	if !reflect.DeepEqual(store.searchMemoryEmbedding, []float32{0.25, 0.75}) {
		t.Fatalf("expected embedded query to reach vector search, got %+v", store.searchMemoryEmbedding)
	}
	prompt := chatMessagesContent(gateway.lastPlainMessages)
	if !strings.Contains(prompt, "Vector-selected preference should be injected.") {
		t.Fatalf("expected vector memory in prompt, got %q", prompt)
	}
	if strings.Contains(prompt, "Fallback query memory should not be injected.") {
		t.Fatalf("expected prompt to avoid fallback memory when vector search succeeds, got %q", prompt)
	}
}

func TestStartRunRecordsUserManagedAgentMemoryEvidence(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config:         Config{EnableMemory: true},
		},
		conversation: &Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
		memories: []*Memory{
			{
				ID:             "memory_match",
				OrganizationID: "org_1",
				UserID:         "user_1",
				AgentID:        "agent_1",
				Type:           MemoryTypeUserManaged,
				Content:        "Use concise style for release notes.",
			},
		},
	}
	gateway := &fakeGateway{plainReply: "memory aware answer"}
	service := NewService(store, gateway)

	runWithMessages, err := service.StartRun(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, StartRunRequest{
		AgentID:        "agent_1",
		ConversationID: "conv_1",
		Input:          "concise style",
	})
	if err != nil {
		t.Fatalf("StartRun returned error: %v", err)
	}
	if runWithMessages == nil || runWithMessages.Run == nil {
		t.Fatalf("expected run with messages, got %+v", runWithMessages)
	}
	if !runWithMessages.Run.MemoryEnabled || !runWithMessages.Run.MemorySearched || runWithMessages.Run.MemoryResultCount != 1 {
		t.Fatalf("expected user-managed memory evidence on run, got %+v", runWithMessages.Run)
	}
}

func chatMessagesContent(messages []chat.Message) string {
	var builder strings.Builder
	for _, message := range messages {
		builder.WriteString(message.Content)
		builder.WriteString("\n")
	}
	return builder.String()
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

func TestToolDefinitionsExposeCustomToolInputSchema(t *testing.T) {
	executor := NewToolExecutor(nil)
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"customer_id": map[string]any{"type": "string"},
		},
		"required": []string{"customer_id"},
	}
	agent := &Agent{
		ID: "agent_custom_schema",
		Tools: []Tool{
			{
				Name:        "crm_lookup",
				Description: "Lookup customer records",
				Type:        "custom",
				ServerID:    "https://tools.example.com/crm_lookup",
				Enabled:     true,
				InputSchema: schema,
			},
		},
	}

	definitions, err := executor.GetToolDefinitions(context.Background(), agent)
	if err != nil {
		t.Fatalf("GetToolDefinitions returned error: %v", err)
	}
	if len(definitions) != 1 {
		t.Fatalf("expected one custom tool definition, got %+v", definitions)
	}
	if definitions[0].ToolType != "custom" || definitions[0].InputSchema == nil {
		t.Fatalf("custom tool definition missing type/schema: %+v", definitions[0])
	}
	properties, ok := definitions[0].InputSchema.(map[string]any)["properties"].(map[string]any)
	if !ok || properties["customer_id"] == nil {
		t.Fatalf("custom input schema was not preserved: %+v", definitions[0].InputSchema)
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

func TestListAvailableToolsExposesCustomToolInputSchema(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"customer_id": map[string]any{"type": "string"},
		},
		"required": []string{"customer_id"},
	}
	store := &fakeStore{
		agent: &Agent{
			ID:     "agent_custom_schema",
			UserID: "user_1",
			Tools: []Tool{
				{
					Name:        "crm_lookup",
					Description: "Lookup customer records",
					Type:        "custom",
					ServerID:    "https://tools.example.com/crm_lookup",
					Enabled:     true,
					InputSchema: schema,
				},
			},
		},
	}
	service := NewService(store, &fakeGateway{})

	definitions, err := service.ListAvailableTools(context.Background(), auth.Session{
		User: auth.User{ID: "user_1"},
	}, "agent_custom_schema")
	if err != nil {
		t.Fatalf("ListAvailableTools returned error: %v", err)
	}
	if len(definitions) != 1 {
		t.Fatalf("expected one custom tool definition, got %+v", definitions)
	}
	if definitions[0].ToolType != "custom" || definitions[0].InputSchema == nil {
		t.Fatalf("custom available tool missing type/schema: %+v", definitions[0])
	}
	properties, ok := definitions[0].InputSchema.(map[string]any)["properties"].(map[string]any)
	if !ok || properties["customer_id"] == nil {
		t.Fatalf("custom input schema was not preserved: %+v", definitions[0].InputSchema)
	}
}

func TestListAvailableToolsAllowsWebSearchWhenProviderConfigured(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:     "agent_policy",
			UserID: "user_1",
			Tools: []Tool{
				{Name: "web_search", Type: "builtin", Enabled: true},
			},
		},
	}
	service := NewService(store, &fakeGateway{})
	service.SetWebSearchProvider(fakeAgentWebSearchProvider{
		results: []mcp.WebSearchResult{{Title: "Search ready", URL: "https://search.example.test", Snippet: "configured"}},
	})

	definitions, err := service.ListAvailableTools(context.Background(), auth.Session{
		User: auth.User{ID: "user_1"},
	}, "agent_policy")
	if err != nil {
		t.Fatalf("ListAvailableTools returned error: %v", err)
	}
	if len(definitions) != 1 || definitions[0].Name != "web_search" {
		t.Fatalf("expected web_search to be exposed with provider configured, got %+v", definitions)
	}

	result, err := service.ExecuteTool(context.Background(), auth.Session{
		User: auth.User{ID: "user_1"},
	}, "agent_policy", "web_search", map[string]any{"query": "configured search"})
	if err != nil {
		t.Fatalf("ExecuteTool returned error: %v", err)
	}
	if result == nil || result.IsError || !strings.Contains(result.Content, "Search ready") {
		t.Fatalf("expected provider-backed web_search result, got %+v", result)
	}
}

func TestListAvailableToolsIncludesApprovalMetadata(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:     "agent_policy",
			UserID: "user_1",
			Tools: []Tool{
				{
					Name:             "datetime",
					Type:             "builtin",
					Enabled:          true,
					RequiresApproval: true,
					RiskLevel:        ToolRiskMedium,
				},
				{
					Name:    "delete_file",
					Type:    "custom",
					Enabled: true,
				},
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

	byName := make(map[string]ToolDefinition, len(definitions))
	for _, definition := range definitions {
		byName[definition.Name] = definition
	}
	writeNote, ok := byName["datetime"]
	if !ok {
		t.Fatalf("expected datetime in tool definitions, got %+v", definitions)
	}
	if writeNote.ToolType != "builtin" || !writeNote.RequiresApproval || writeNote.RiskLevel != ToolRiskMedium {
		t.Fatalf("datetime approval metadata = type %q approval %v risk %q, want builtin true medium", writeNote.ToolType, writeNote.RequiresApproval, writeNote.RiskLevel)
	}
	deleteFile, ok := byName["delete_file"]
	if !ok {
		t.Fatalf("expected delete_file in tool definitions, got %+v", definitions)
	}
	if deleteFile.ToolType != "custom" || deleteFile.RequiresApproval || deleteFile.RiskLevel != ToolRiskDangerous {
		t.Fatalf("delete_file approval metadata = type %q approval %v risk %q, want custom false dangerous", deleteFile.ToolType, deleteFile.RequiresApproval, deleteFile.RiskLevel)
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
			ID:             "agent_stream",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config:         Config{EnableMemory: true},
		},
		conversation: &Conversation{
			ID:             "conv_stream",
			AgentID:        "agent_stream",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
	}

	gateway := &fakeGateway{
		plainReply: "streaming plain reply",
	}

	service := NewService(store, gateway)
	var chunks []string
	err := service.SendMessageStream(context.Background(), auth.Session{
		OrganizationID: "org_1",
		User:           auth.User{ID: "user_1"},
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
	if len(store.memories) != 1 {
		t.Fatalf("expected one automatic long-term memory, got %+v", store.memories)
	}
	created := store.memories[0]
	if created.Type != MemoryTypeLongTerm || created.AgentID != "agent_stream" || created.UserID != "user_1" || created.OrganizationID != "org_1" {
		t.Fatalf("expected scoped long-term streaming memory, got %+v", created)
	}
	if created.Content != "User: hello\nAssistant: streaming plain reply" {
		t.Fatalf("unexpected streaming memory content: %q", created.Content)
	}
	if created.Importance != 3 || created.Metadata["source"] != "agent_run" || created.Metadata["conversation_id"] != "conv_stream" {
		t.Fatalf("expected automatic memory metadata and default importance, got %+v", created)
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
