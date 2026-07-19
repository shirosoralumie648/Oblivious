package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
	"oblivious/server/internal/mcp"
	"oblivious/server/internal/memory"
	"oblivious/server/internal/metrics"
	"oblivious/server/internal/releasecontract"

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
	agent                     *Agent
	conversation              *Conversation
	createMemoryEmbedding     []float32
	listMemoryAgentID         string
	listMemoryLimit           int
	listMemoryOrganizationID  string
	listMemoryQuery           string
	listMemoryTypes           []string
	searchMemoryAgentID       string
	searchMemoryEmbedding     []float32
	searchMemoryLimit         int
	searchMemoryMinScore      float64
	searchMemoryResults       []*MemorySearchResult
	searchMemoryResultsByType map[string][]*MemorySearchResult
	searchMemoryTypes         []string
	listMemoryUserID          string
	memories                  []*Memory
	messages                  []*Message
	planSteps                 []*PlanStep
	runs                      []*Run
	toolRuns                  []*ToolRun
	updateMemoryEmbedding     []float32
	updateRunRequests         []UpdateRunRequest
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
	s.updateRunRequests = append(s.updateRunRequests, req)
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
		Description:    req.Description,
		Status:         status,
		ApprovalStatus: approvalStatus,
		ToolName:       req.ToolName,
		Input:          input,
		DependsOn:      normalizePlanStepDependsOn(req.DependsOn),
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
	if req.Description != nil {
		step.Description = *req.Description
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
	if req.ReplaceDependsOn {
		step.DependsOn = normalizePlanStepDependsOn(req.DependsOn)
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

func clonePlanStepSnapshots(steps []*PlanStep) []*PlanStep {
	snapshots := make([]*PlanStep, 0, len(steps))
	for _, step := range steps {
		if step == nil {
			snapshots = append(snapshots, nil)
			continue
		}
		copied := *step
		if step.Input != nil {
			copied.Input = copyPlanStepInput(step.Input)
		}
		snapshots = append(snapshots, &copied)
	}
	return snapshots
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
			Mode:           ExecutionModePlanning,
			Status:         RunStatusPendingApproval,
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

func TestServiceUpdatePlanStepDraftRecomputesApprovalForPendingDraft(t *testing.T) {
	session := auth.Session{
		User:           auth.User{ID: "user_1"},
		OrganizationID: "org_1",
	}
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Config:         Config{ApprovalMode: ApprovalModeTiered},
			Tools: []Tool{{
				Name:      "write_file",
				Type:      "builtin",
				Enabled:   true,
				RiskLevel: ToolRiskMedium,
			}, {
				Name:      "read_file",
				Type:      "builtin",
				Enabled:   true,
				RiskLevel: ToolRiskSafe,
			}},
		},
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Mode:           ExecutionModePlanning,
			Status:         RunStatusPendingApproval,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          1,
			Title:          "Inspect config",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusNotRequired,
			ToolName:       "read_file",
			Input:          map[string]any{"path": "config.yaml"},
		}},
	}
	service := NewService(store, &fakeGateway{})

	writeFile := "write_file"
	updated, err := service.UpdatePlanStepDraft(context.Background(), session, "step_1", UpdatePlanStepDraftRequest{
		ToolName: &writeFile,
		Input:    map[string]any{"path": "config.yaml", "content": "new"},
	})
	if err != nil {
		t.Fatalf("UpdatePlanStepDraft returned error: %v", err)
	}
	if updated.Status != PlanStepStatusPending || updated.ApprovalStatus != ApprovalStatusPending {
		t.Fatalf("write_file edit should require approval, got %+v", updated)
	}

	readFile := "read_file"
	updated, err = service.UpdatePlanStepDraft(context.Background(), session, "step_1", UpdatePlanStepDraftRequest{
		ToolName: &readFile,
		Input:    map[string]any{"path": "config.yaml"},
	})
	if err != nil {
		t.Fatalf("UpdatePlanStepDraft returned error on safe edit: %v", err)
	}
	if updated.Status != PlanStepStatusPending || updated.ApprovalStatus != ApprovalStatusNotRequired {
		t.Fatalf("read_file edit should not require approval, got %+v", updated)
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
			Mode:           ExecutionModePlanning,
			Status:         RunStatusPendingApproval,
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
			Mode:           ExecutionModePlanning,
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

func TestServiceMovePlanStepRewritesDependenciesToMovedLogicalSteps(t *testing.T) {
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
			Mode:           ExecutionModePlanning,
			Status:         RunStatusPendingApproval,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          1,
			Title:          "Collect evidence",
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
			DependsOn:      []int{1},
			CreatedAt:      now.Add(time.Second),
			UpdatedAt:      now.Add(time.Second),
		}, {
			ID:             "step_3",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          3,
			Title:          "Verify patch",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusPending,
			DependsOn:      []int{2},
			CreatedAt:      now.Add(2 * time.Second),
			UpdatedAt:      now.Add(2 * time.Second),
		}, {
			ID:             "step_4",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          4,
			Title:          "Publish evidence",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusPending,
			DependsOn:      []int{3},
			CreatedAt:      now.Add(3 * time.Second),
			UpdatedAt:      now.Add(3 * time.Second),
		}},
	}
	service := NewService(store, &fakeGateway{})

	steps, err := service.MovePlanStep(context.Background(), session, "step_3", MovePlanStepDirectionUp)
	if err != nil {
		t.Fatalf("MovePlanStep returned error: %v", err)
	}

	if steps[1].ID != "step_3" || steps[1].Index != 2 || !reflect.DeepEqual(steps[1].DependsOn, []int{3}) {
		t.Fatalf("expected moved step_3 to depend on logical step_2 at new index 3, got %+v", steps[1])
	}
	if steps[2].ID != "step_2" || steps[2].Index != 3 || !reflect.DeepEqual(steps[2].DependsOn, []int{1}) {
		t.Fatalf("expected step_2 to keep its logical dependency, got %+v", steps[2])
	}
	if steps[3].ID != "step_4" || steps[3].Index != 4 || !reflect.DeepEqual(steps[3].DependsOn, []int{2}) {
		t.Fatalf("expected step_4 to keep depending on logical step_3 at new index 2, got %+v", steps[3])
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
			Mode:           ExecutionModePlanning,
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
			Mode:           ExecutionModePlanning,
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

func TestServiceCreatePlanStepDraftRewritesShiftedDependencies(t *testing.T) {
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
			Mode:           ExecutionModePlanning,
			Status:         RunStatusPendingApproval,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          1,
			Title:          "Collect evidence",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusPending,
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
			DependsOn:      []int{1},
			CreatedAt:      now.Add(time.Second),
			UpdatedAt:      now.Add(time.Second),
		}, {
			ID:             "step_3",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          3,
			Title:          "Verify patch",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusPending,
			DependsOn:      []int{2},
			CreatedAt:      now.Add(2 * time.Second),
			UpdatedAt:      now.Add(2 * time.Second),
		}},
	}
	service := NewService(store, &fakeGateway{})
	afterID := "step_1"

	steps, err := service.CreatePlanStepDraft(context.Background(), session, "run_1", CreatePlanStepDraftRequest{
		AfterPlanStepID: &afterID,
		Title:           "Review old draft before verification",
		DependsOn:       []int{2},
	})
	if err != nil {
		t.Fatalf("CreatePlanStepDraft returned error: %v", err)
	}

	if len(steps) != 4 {
		t.Fatalf("expected four steps, got %+v", steps)
	}
	if steps[1].Title != "Review old draft before verification" || steps[1].Index != 2 || !reflect.DeepEqual(steps[1].DependsOn, []int{3}) {
		t.Fatalf("expected inserted step dependency on logical old step_2 at new index 3, got %+v", steps[1])
	}
	if steps[2].ID != "step_2" || steps[2].Index != 3 || !reflect.DeepEqual(steps[2].DependsOn, []int{1}) {
		t.Fatalf("expected old step_2 to keep dependency on step_1, got %+v", steps[2])
	}
	if steps[3].ID != "step_3" || steps[3].Index != 4 || !reflect.DeepEqual(steps[3].DependsOn, []int{3}) {
		t.Fatalf("expected old step_3 to keep depending on logical step_2 at new index 3, got %+v", steps[3])
	}
}

func TestServiceCreatePlanStepDraftAppliesApprovalPolicy(t *testing.T) {
	session := auth.Session{
		User:           auth.User{ID: "user_1"},
		OrganizationID: "org_1",
	}
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Config:         Config{ApprovalMode: ApprovalModeTiered},
			Tools: []Tool{{
				Name:      "read_file",
				Type:      "builtin",
				Enabled:   true,
				RiskLevel: ToolRiskSafe,
			}, {
				Name:      "write_file",
				Type:      "builtin",
				Enabled:   true,
				RiskLevel: ToolRiskMedium,
			}},
		},
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Mode:           ExecutionModePlanning,
			Status:         RunStatusPendingApproval,
		}},
	}
	service := NewService(store, &fakeGateway{})

	steps, err := service.CreatePlanStepDraft(context.Background(), session, "run_1", CreatePlanStepDraftRequest{
		Title:    "Read config",
		ToolName: "read_file",
		Input:    map[string]any{"path": "config.yaml"},
	})
	if err != nil {
		t.Fatalf("CreatePlanStepDraft safe tool returned error: %v", err)
	}
	if len(steps) != 1 || steps[0].ApprovalStatus != ApprovalStatusNotRequired {
		t.Fatalf("safe manual step should not require approval, got %+v", steps)
	}

	steps, err = service.CreatePlanStepDraft(context.Background(), session, "run_1", CreatePlanStepDraftRequest{
		Title:    "Write config",
		ToolName: "write_file",
		Input:    map[string]any{"path": "config.yaml", "content": "new"},
	})
	if err != nil {
		t.Fatalf("CreatePlanStepDraft write tool returned error: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected two plan steps, got %+v", steps)
	}
	if steps[0].ApprovalStatus != ApprovalStatusNotRequired || steps[1].ApprovalStatus != ApprovalStatusPending {
		t.Fatalf("manual plan-step approval statuses did not follow policy: %+v", steps)
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
			Mode:           ExecutionModePlanning,
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
			Mode:           ExecutionModePlanning,
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

func TestServiceDeletePlanStepDraftRewritesShiftedDependencies(t *testing.T) {
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
			Mode:           ExecutionModePlanning,
			Status:         RunStatusPendingApproval,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          1,
			Title:          "Collect evidence",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusPending,
			CreatedAt:      now,
			UpdatedAt:      now,
		}, {
			ID:             "step_2",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          2,
			Title:          "Optional cleanup",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusPending,
			CreatedAt:      now.Add(time.Second),
			UpdatedAt:      now.Add(time.Second),
		}, {
			ID:             "step_3",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          3,
			Title:          "Draft patch",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusPending,
			DependsOn:      []int{1},
			CreatedAt:      now.Add(2 * time.Second),
			UpdatedAt:      now.Add(2 * time.Second),
		}, {
			ID:             "step_4",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          4,
			Title:          "Verify patch",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusPending,
			DependsOn:      []int{3},
			CreatedAt:      now.Add(3 * time.Second),
			UpdatedAt:      now.Add(3 * time.Second),
		}},
	}
	service := NewService(store, &fakeGateway{})

	steps, err := service.DeletePlanStepDraft(context.Background(), session, "step_2")
	if err != nil {
		t.Fatalf("DeletePlanStepDraft returned error: %v", err)
	}

	if len(steps) != 3 {
		t.Fatalf("expected three remaining steps, got %+v", steps)
	}
	if steps[1].ID != "step_3" || steps[1].Index != 2 || !reflect.DeepEqual(steps[1].DependsOn, []int{1}) {
		t.Fatalf("expected old step_3 dependency to remain on step_1, got %+v", steps[1])
	}
	if steps[2].ID != "step_4" || steps[2].Index != 3 || !reflect.DeepEqual(steps[2].DependsOn, []int{2}) {
		t.Fatalf("expected old step_4 to keep depending on logical step_3 at new index 2, got %+v", steps[2])
	}
}

func TestServiceDeletePlanStepDraftRejectsDeletingDependencyTarget(t *testing.T) {
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
			Mode:           ExecutionModePlanning,
			Status:         RunStatusPendingApproval,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          1,
			Title:          "Collect evidence",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusPending,
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
			CreatedAt:      now.Add(time.Second),
			UpdatedAt:      now.Add(time.Second),
		}, {
			ID:             "step_3",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          3,
			Title:          "Verify patch",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusPending,
			DependsOn:      []int{2},
			CreatedAt:      now.Add(2 * time.Second),
			UpdatedAt:      now.Add(2 * time.Second),
		}},
	}
	service := NewService(store, &fakeGateway{})

	_, err := service.DeletePlanStepDraft(context.Background(), session, "step_2")
	if err == nil || !strings.Contains(err.Error(), "step 3 depends on it") {
		t.Fatalf("expected dependency-target delete rejection, got %v", err)
	}
	if len(store.planSteps) != 3 || store.planSteps[1].Index != 2 || store.planSteps[2].Index != 3 || !reflect.DeepEqual(store.planSteps[2].DependsOn, []int{2}) {
		t.Fatalf("delete rejection should leave plan steps unchanged, got %+v", store.planSteps)
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
			Mode:           ExecutionModePlanning,
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
	s.listMemoryTypes = append(s.listMemoryTypes, req.Type)
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
	s.searchMemoryTypes = append(s.searchMemoryTypes, req.Type)
	if s.searchMemoryResultsByType != nil {
		return s.searchMemoryResultsByType[req.Type], nil
	}
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
			Mode:           ExecutionModePlanning,
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

func TestServiceExecutePlanStepRunsCustomToolFromAgentConfig(t *testing.T) {
	now := time.Now().UTC()
	var gotPayload map[string]any
	customToolServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("custom tool method = %s, want POST", r.Method)
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("decode custom tool payload: %v", err)
		}
		_, _ = w.Write([]byte("customer:Kiana"))
	}))
	defer customToolServer.Close()

	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Config:         Config{ApprovalMode: ApprovalModeNone},
			Tools: []Tool{{
				Name:      "crm_lookup",
				Type:      "custom",
				ServerID:  customToolServer.URL,
				Enabled:   true,
				RiskLevel: ToolRiskDangerous,
			}},
		},
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Mode:           ExecutionModePlanning,
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
			Title:          "Lookup CRM record",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusNotRequired,
			ToolName:       "crm_lookup",
			Input:          map[string]any{"customer_id": "cust_1"},
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	service := newAuthorizedServiceForTest(t, store, &fakeGateway{})
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}

	completed, err := service.ExecutePlanStep(context.Background(), session, "step_1")
	if err != nil {
		t.Fatalf("ExecutePlanStep returned error: %v", err)
	}
	if completed.Status != PlanStepStatusCompleted || completed.ResultContent != "customer:Kiana" {
		t.Fatalf("expected custom tool step to complete with API result, got %+v", completed)
	}
	if gotPayload["customer_id"] != "cust_1" {
		t.Fatalf("custom tool received payload %+v, want customer_id=cust_1", gotPayload)
	}
	if len(store.toolRuns) != 1 {
		t.Fatalf("expected one persisted tool run, got %+v", store.toolRuns)
	}
	toolRun := store.toolRuns[0]
	if toolRun.ToolType != "custom" || toolRun.ServerID != customToolServer.URL || toolRun.RiskLevel != ToolRiskDangerous {
		t.Fatalf("plan-step tool run should preserve custom tool metadata, got %+v", toolRun)
	}
	if toolRun.Status != ToolRunStatusCompleted || toolRun.ResultContent != "customer:Kiana" || toolRun.Error != "" {
		t.Fatalf("expected completed custom tool run evidence, got %+v", toolRun)
	}
}

func TestServiceExecutePlanStepRejectsPendingApprovalStep(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Mode:           ExecutionModePlanning,
			Status:         RunStatusPendingApproval,
			StartedAt:      now,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          1,
			Title:          "Write code",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusPending,
			ToolName:       "write_file",
		}},
	}
	service := NewService(store, &fakeGateway{})

	_, err := service.ExecutePlanStep(
		context.Background(),
		auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1", User: auth.User{ID: "user_1"}},
		"step_1",
	)
	if err == nil {
		t.Fatal("expected error when executing pending-approval plan step")
	}
	if !strings.Contains(err.Error(), "not approved") {
		t.Fatalf("expected 'not approved' error, got: %v", err)
	}
	// Verify step was not mutated
	if store.planSteps[0].Status != PlanStepStatusPending {
		t.Fatalf("pending-approval step should remain pending: %+v", store.planSteps[0])
	}
}

func TestServiceExecutePlanStepRejectsStaleNotRequiredApproval(t *testing.T) {
	session := auth.Session{
		User:           auth.User{ID: "user_1"},
		OrganizationID: "org_1",
	}
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Config:         Config{ApprovalMode: ApprovalModeTiered},
			Tools: []Tool{{
				Name:      "write_file",
				Type:      "builtin",
				Enabled:   true,
				RiskLevel: ToolRiskMedium,
			}},
		},
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Mode:           ExecutionModePlanning,
			Status:         RunStatusPendingApproval,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			OrganizationID: "org_1",
			RunID:          "run_1",
			Index:          1,
			Title:          "Write config",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusNotRequired,
			ToolName:       "write_file",
			Input:          map[string]any{"path": "config.yaml", "content": "new"},
		}},
	}
	executor := &fakePlanStepExecutor{resultContent: "should not execute stale approval"}
	service := NewService(store, &fakeGateway{})
	service.SetPlanStepExecutor(executor)

	_, err := service.ExecutePlanStep(context.Background(), session, "step_1")
	if err == nil || !strings.Contains(err.Error(), "requires approval") {
		t.Fatalf("expected stale not_required approval rejection, got %v", err)
	}
	if executor.calls != 0 {
		t.Fatalf("stale not_required step should not execute, got %d calls", executor.calls)
	}
	step := store.planSteps[0]
	if step.Status != PlanStepStatusPending || step.ApprovalStatus != ApprovalStatusPending {
		t.Fatalf("stale not_required step should reopen for approval, got %+v", step)
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
			Mode:           ExecutionModePlanning,
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
			Mode:           ExecutionModePlanning,
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
			Mode:           ExecutionModePlanning,
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

func TestServiceRetryPlanStepRejectsOutOfOrderRetryWithoutClearingFailure(t *testing.T) {
	now := time.Now().UTC()
	completedAt := now.Add(time.Minute)
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Mode:           ExecutionModePlanning,
			Status:         RunStatusFailed,
			Error:          "step 2 failed",
			StartedAt:      now,
			CompletedAt:    &completedAt,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		planSteps: []*PlanStep{
			{
				ID:             "step_1",
				RunID:          "run_1",
				OrganizationID: "org_1",
				Index:          1,
				Title:          "Still pending",
				Status:         PlanStepStatusPending,
				ApprovalStatus: ApprovalStatusNotRequired,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
			{
				ID:             "step_2",
				RunID:          "run_1",
				OrganizationID: "org_1",
				Index:          2,
				Title:          "Failed after being approved",
				Status:         PlanStepStatusFailed,
				ApprovalStatus: ApprovalStatusApproved,
				ResultContent:  "partial output",
				Error:          "old failure",
				StartedAt:      &now,
				CompletedAt:    &completedAt,
				CreatedAt:      now,
				UpdatedAt:      now,
			},
		},
	}
	executor := &fakePlanStepExecutor{resultContent: "should not run out of order"}
	service := NewService(store, &fakeGateway{})
	service.SetPlanStepExecutor(executor)
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}

	reopened, err := service.RetryPlanStep(context.Background(), session, "step_2")
	if err == nil || !strings.Contains(err.Error(), "prior plan step 1 must be completed or skipped before executing step 2") {
		t.Fatalf("expected out-of-order retry rejection, got step=%+v err=%v", reopened, err)
	}
	if executor.calls != 0 {
		t.Fatalf("expected executor not to run after out-of-order retry, got %d calls", executor.calls)
	}
	run, err := store.GetRun(context.Background(), "org_1", "run_1")
	if err != nil {
		t.Fatalf("GetRun returned error: %v", err)
	}
	if run.Status != RunStatusFailed || run.Error != "step 2 failed" || run.CompletedAt != &completedAt {
		t.Fatalf("expected out-of-order retry to preserve run failure evidence, got %+v", run)
	}
	step, err := store.GetPlanStep(context.Background(), "org_1", "step_2")
	if err != nil {
		t.Fatalf("GetPlanStep returned error: %v", err)
	}
	if step.Status != PlanStepStatusFailed || step.ApprovalStatus != ApprovalStatusApproved ||
		step.ResultContent != "partial output" || step.Error != "old failure" || step.CompletedAt != &completedAt {
		t.Fatalf("expected out-of-order retry to preserve failed step evidence, got %+v", step)
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
			Mode:           ExecutionModePlanning,
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
			Mode:           ExecutionModePlanning,
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
			Title:          "Calculate answer",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusNotRequired,
			ToolName:       "calculator",
			Input:          map[string]any{"expression": "2 + 3"},
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	service := newAuthorizedServiceForTest(t, store, &fakeGateway{})

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
			Mode:           ExecutionModePlanning,
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
			Mode:           ExecutionModePlanning,
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
	service := newAuthorizedServiceForTest(t, store, gateway)

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
	service := newAuthorizedServiceForTest(t, store, gateway)

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

	service := newAuthorizedServiceForTest(t, store, gateway)
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
	service := newAuthorizedServiceForTest(t, store, gateway)

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
	service := newAuthorizedServiceForTest(t, store, gateway)

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

func TestServiceStartRunUsesAgentConfigModelRoutingRules(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_routing",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config: Config{
				ModelRoutingRules: []ModelRoutingRule{{
					TargetModel:        "gpt-4o",
					MinIteration:       2,
					RequiresToolResult: true,
				}},
				MaxIterations: 3,
			},
			Tools: []Tool{{
				Name:      "datetime",
				Type:      "builtin",
				Enabled:   true,
				RiskLevel: ToolRiskSafe,
			}},
		},
		conversation: &Conversation{
			ID:             "conv_routing",
			AgentID:        "agent_routing",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
	}
	gateway := &mockStructuredGateway{
		replies: []*chat.CompletionResponse{{
			Content: "checking time",
			ToolCalls: []chat.ToolCall{{
				ID:   "call_datetime",
				Type: "function",
				Function: chat.ToolFunction{
					Name:      "datetime",
					Arguments: `{}`,
				},
			}},
			Usage: &chat.CompletionUsage{TotalTokens: 25},
		}, {
			Content: "routed final answer",
			Usage:   &chat.CompletionUsage{TotalTokens: 25},
		}},
	}
	service := newAuthorizedServiceForTest(t, store, gateway)

	result, err := service.StartRun(context.Background(), auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
		User:           auth.User{ID: "user_1"},
	}, StartRunRequest{
		AgentID:        "agent_routing",
		ConversationID: "conv_routing",
		Input:          "use the tool then answer",
	})
	if err != nil {
		t.Fatalf("StartRun returned error: %v", err)
	}
	if result == nil || result.Run == nil || result.Run.Status != RunStatusCompleted {
		t.Fatalf("expected completed routed run, got %+v", result)
	}
	if len(gateway.models) < 2 {
		t.Fatalf("expected two structured calls, got models=%v", gateway.models)
	}
	if gateway.models[0] != "gpt-4o-mini" || gateway.models[1] != "gpt-4o" {
		t.Fatalf("expected config routing to switch second iteration model, got %v", gateway.models)
	}
	if len(store.toolRuns) != 1 || store.toolRuns[0].Status != ToolRunStatusCompleted {
		t.Fatalf("expected tool run before routed second iteration, got %+v", store.toolRuns)
	}
}

func TestServiceStartRunUsesAgentConfigSkillsAndMaxSkills(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_skills",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			SystemPrompt:   "Base prompt",
			Config: Config{
				Skills: []Skill{{
					Name:         "Weather",
					Instructions: "Provide weather-specific checks",
					Triggers:     []string{"weather"},
					ToolNames:    []string{"datetime"},
				}, {
					Name:         "Calculator",
					Instructions: "Perform math checks",
					Triggers:     []string{"calculate"},
					ToolNames:    []string{"calculator"},
				}},
				MaxSkills: 1,
			},
			Tools: []Tool{{
				Name:      "datetime",
				Type:      "builtin",
				Enabled:   true,
				RiskLevel: ToolRiskSafe,
			}, {
				Name:      "calculator",
				Type:      "builtin",
				Enabled:   true,
				RiskLevel: ToolRiskSafe,
			}},
		},
		conversation: &Conversation{
			ID:             "conv_skills",
			AgentID:        "agent_skills",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
	}
	gateway := &skillMockStructuredGateway{
		reply: &chat.CompletionResponse{
			Content: "skill-aware answer",
			Usage:   &chat.CompletionUsage{TotalTokens: 20},
		},
	}
	service := NewService(store, gateway)

	result, err := service.StartRun(context.Background(), auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
		User:           auth.User{ID: "user_1"},
	}, StartRunRequest{
		AgentID:        "agent_skills",
		ConversationID: "conv_skills",
		Input:          "weather forecast please calculate later",
	})
	if err != nil {
		t.Fatalf("StartRun returned error: %v", err)
	}
	if result == nil || result.Run == nil || result.Run.Status != RunStatusCompleted {
		t.Fatalf("expected completed skill run, got %+v", result)
	}
	if !strings.Contains(gateway.lastConfig.SystemPromptOverride, "Weather: Provide weather-specific checks") {
		t.Fatalf("expected selected skill instructions in prompt, got %q", gateway.lastConfig.SystemPromptOverride)
	}
	if strings.Contains(gateway.lastConfig.SystemPromptOverride, "Calculator: Perform math checks") {
		t.Fatalf("expected maxSkills=1 to keep unselected skill out of prompt, got %q", gateway.lastConfig.SystemPromptOverride)
	}
	if len(gateway.lastTools) != 1 {
		t.Fatalf("expected selected skill to filter tools to one entry, got %+v", gateway.lastTools)
	}
	fn, _ := gateway.lastTools[0]["function"].(map[string]any)
	if fn["name"] != "datetime" {
		t.Fatalf("expected weather skill to keep datetime tool, got %+v", gateway.lastTools)
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

func TestServiceStartPlanningRunStopsBeforePlanningReplyWhenTokenBudgetExceeded(t *testing.T) {
	largeInput := strings.Repeat("planning risk evidence ", 900)
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
	}
	gateway := &fakeGateway{plainReply: "should not be called"}
	service := NewService(store, gateway)

	result, err := service.StartPlanningRun(
		context.Background(),
		auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1", User: auth.User{ID: "user_1"}},
		StartRunRequest{AgentID: "agent_1", ConversationID: "conv_1", Input: largeInput},
	)
	if !errors.Is(err, ErrTokenBudgetExceeded) {
		t.Fatalf("expected ErrTokenBudgetExceeded, got result=%+v err=%v", result, err)
	}
	if gateway.plainCalls != 0 || gateway.streamCalls != 0 || gateway.structuredCalls != 0 {
		t.Fatalf("expected planning token budget guard to stop before gateway call, got plain=%d stream=%d structured=%d", gateway.plainCalls, gateway.streamCalls, gateway.structuredCalls)
	}
	if len(store.messages) != 1 || store.messages[0].Role != "user" {
		t.Fatalf("expected only the user planning request to be persisted, got %+v", store.messages)
	}
	if len(store.planSteps) != 0 {
		t.Fatalf("expected no plan steps when initial planning prompt exceeds budget, got %+v", store.planSteps)
	}
	if len(store.runs) != 1 {
		t.Fatalf("expected one durable token-budget run, got %+v", store.runs)
	}
	run := store.runs[0]
	if run.Status != RunStatusTokenBudgetExceeded || !strings.Contains(run.Error, "token_budget_exceeded") || run.CompletedAt == nil {
		t.Fatalf("expected token-budget-exceeded planning run evidence, got %+v", run)
	}
	if run.Mode != ExecutionModePlanning || run.IterationCount != 1 || run.ToolCallCount != 0 || run.FinalMessageID != "" {
		t.Fatalf("expected planning budget stop before assistant message, got %+v", run)
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

func TestServiceStartPlanningRunAppliesToolApprovalToStructuredSteps(t *testing.T) {
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
	gateway := &fakeGateway{plainReply: `[
		{"title":"Read config","toolName":"read_file","input":{"path":"config.yaml"}},
		{"title":"Update config","toolName":"write_file","input":{"path":"config.yaml"}},
		{"title":"Delete cache","toolName":"delete_file","input":{"path":"cache/"}},
		{"title":"Summarize results"}
	]`}
	service := NewService(store, gateway)

	result, err := service.StartPlanningRun(
		context.Background(),
		auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1", User: auth.User{ID: "user_1"}},
		StartRunRequest{AgentID: "agent_1", ConversationID: "conv_1", Input: "refactor config"},
	)
	if err != nil {
		t.Fatalf("StartPlanningRun returned error: %v", err)
	}
	if len(store.planSteps) != 4 {
		t.Fatalf("expected 4 persisted plan steps, got %+v", store.planSteps)
	}

	// Safe tool (read) → not required
	if store.planSteps[0].ApprovalStatus != ApprovalStatusNotRequired {
		t.Fatalf("safe tool step should not require approval: %+v", store.planSteps[0])
	}
	// Medium tool (write) → pending
	if store.planSteps[1].ApprovalStatus != ApprovalStatusPending {
		t.Fatalf("medium tool step should require approval: %+v", store.planSteps[1])
	}
	// Dangerous tool (delete) → pending
	if store.planSteps[2].ApprovalStatus != ApprovalStatusPending {
		t.Fatalf("dangerous tool step should require approval: %+v", store.planSteps[2])
	}
	// No tool → not required
	if store.planSteps[3].ApprovalStatus != ApprovalStatusNotRequired {
		t.Fatalf("no-tool step should not require approval: %+v", store.planSteps[3])
	}

	if len(result.PlanSteps) != 4 {
		t.Fatalf("expected returned RunWithMessages to include 4 plan steps, got %+v", result.PlanSteps)
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
			Mode:           ExecutionModePlanning,
			Status:         RunStatusPendingApproval,
			IterationCount: 1,
			FinalMessageID: "msg_plan",
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
			ToolName:       "read_file",
			Input:          map[string]any{"path": "config.yaml"},
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
	if run.FinalMessageID != "msg_plan" {
		t.Fatalf("expected planning run completion to preserve final message id, got %+v", run)
	}
	if run.IterationCount != 2 || run.ToolCallCount != 1 {
		t.Fatalf("expected planning run completion evidence to converge counters, got iterations=%d toolCalls=%d", run.IterationCount, run.ToolCallCount)
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
			Mode:           ExecutionModePlanning,
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

func TestServiceExecutePlanStepHonorsExplicitDependencies(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Mode:           ExecutionModePlanning,
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
			Title:          "Collect migration evidence",
			Status:         PlanStepStatusCompleted,
			ApprovalStatus: ApprovalStatusNotRequired,
			ResultContent:  "schema evidence",
			CreatedAt:      now,
			UpdatedAt:      now,
		}, {
			ID:             "step_2",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          2,
			Title:          "Optional docs cleanup",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusNotRequired,
			CreatedAt:      now,
			UpdatedAt:      now,
		}, {
			ID:             "step_3",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          3,
			Title:          "Run dependent verification",
			Description:    "This verification only needs migration evidence, not docs cleanup.",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusNotRequired,
			DependsOn:      []int{1},
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	executor := &fakePlanStepExecutor{resultContent: "verification passed"}
	service := NewService(store, &fakeGateway{})
	service.SetPlanStepExecutor(executor)
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}

	completed, err := service.ExecutePlanStep(context.Background(), session, "step_3")
	if err != nil {
		t.Fatalf("ExecutePlanStep with satisfied explicit dependency returned error: %v", err)
	}
	if completed.Status != PlanStepStatusCompleted || completed.ResultContent != "verification passed" {
		t.Fatalf("expected dependent step to complete, got %+v", completed)
	}
	if executor.calls != 1 || executor.seenStep == nil || executor.seenStep.Description == "" || len(executor.seenStep.DependsOn) != 1 {
		t.Fatalf("expected executor to receive structured step metadata, calls=%d step=%+v", executor.calls, executor.seenStep)
	}
	if store.planSteps[1].Status != PlanStepStatusPending {
		t.Fatalf("explicit dependencies should not require unrelated lower-index steps, got %+v", store.planSteps[1])
	}
}

func TestServiceExecutePlanStepRejectsUnmetExplicitDependency(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Mode:           ExecutionModePlanning,
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
			Title:          "Collect migration evidence",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusNotRequired,
			CreatedAt:      now,
			UpdatedAt:      now,
		}, {
			ID:             "step_2",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          2,
			Title:          "Run dependent verification",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusNotRequired,
			DependsOn:      []int{1},
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	executor := &fakePlanStepExecutor{resultContent: "should not run"}
	service := NewService(store, &fakeGateway{})
	service.SetPlanStepExecutor(executor)
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}

	updated, err := service.ExecutePlanStep(context.Background(), session, "step_2")
	if err == nil {
		t.Fatal("expected ExecutePlanStep to reject unmet explicit dependency")
	}
	if updated != nil {
		t.Fatalf("expected no updated step when dependency is unmet, got %+v", updated)
	}
	if !strings.Contains(err.Error(), "dependency plan step 1 must be completed or skipped before executing step 2") {
		t.Fatalf("expected dependency validation error, got %v", err)
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
			Mode:           ExecutionModePlanning,
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

func TestServiceSkipPlanStepRejectsOutOfOrderSkip(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Mode:           ExecutionModePlanning,
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
			Title:          "Still pending",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusNotRequired,
			CreatedAt:      now,
			UpdatedAt:      now,
		}, {
			ID:             "step_2",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          2,
			Title:          "Future optional cleanup",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusNotRequired,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	service := NewService(store, &fakeGateway{})
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}

	skipped, err := service.SkipPlanStep(context.Background(), session, "step_2", "not needed")
	if err == nil || !strings.Contains(err.Error(), "prior plan step 1 must be completed or skipped before executing step 2") {
		t.Fatalf("expected out-of-order skip rejection, got step=%+v err=%v", skipped, err)
	}
	if skipped != nil {
		t.Fatalf("expected rejected skip not to return an updated step, got %+v", skipped)
	}
	if store.planSteps[1].Status != PlanStepStatusPending || store.planSteps[1].Error != "" || store.planSteps[1].CompletedAt != nil {
		t.Fatalf("expected rejected skip to preserve future step evidence, got %+v", store.planSteps[1])
	}
	run, err := store.GetRun(context.Background(), "org_1", "run_1")
	if err != nil {
		t.Fatalf("GetRun returned error: %v", err)
	}
	if run.Status != RunStatusPendingApproval || run.CompletedAt != nil {
		t.Fatalf("expected rejected skip to preserve run state, got %+v", run)
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
			Mode:           ExecutionModePlanning,
			Status:         RunStatusPendingApproval,
			IterationCount: 3,
			ToolCallCount:  1,
			FinalMessageID: "msg_plan",
			StartedAt:      now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		toolRuns: []*ToolRun{{
			ID:             "toolrun_existing",
			OrganizationID: "org_1",
			RunID:          "run_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			ToolCallID:     "plan_step_step_1",
			ToolName:       "read_file",
			Status:         ToolRunStatusCompleted,
			ApprovalStatus: ApprovalStatusNotRequired,
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
	if run.FinalMessageID != "msg_plan" {
		t.Fatalf("expected skip completion to preserve final message id, got %+v", run)
	}
	if run.IterationCount != 3 || run.ToolCallCount != 1 {
		t.Fatalf("expected skip completion to preserve higher iteration count and converge tool calls, got iterations=%d toolCalls=%d", run.IterationCount, run.ToolCallCount)
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
					Mode:           ExecutionModePlanning,
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
	gateway := &fakeGateway{plainReply: `[
		{"title":"准备输入","description":"Gather the expression to calculate."},
		{"title":"计算","description":"Use the calculator tool for exact arithmetic.","toolName":"calculator","input":{"expression":"2+3"},"dependsOn":[1,0,1]}
	]`}
	service := NewService(store, gateway)

	result, err := service.StartPlanningRun(
		context.Background(),
		auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1", User: auth.User{ID: "user_1"}},
		StartRunRequest{AgentID: "agent_1", ConversationID: "conv_1", Input: "make a plan"},
	)
	if err != nil {
		t.Fatalf("StartPlanningRun returned error: %v", err)
	}
	if len(store.planSteps) != 2 {
		t.Fatalf("expected 2 structured plan steps, got %+v", store.planSteps)
	}
	step := store.planSteps[1]
	if step.RunID != result.Run.ID || step.Index != 2 || step.Title != "计算" {
		t.Fatalf("structured step scope/index/title mismatch: %+v", step)
	}
	if step.ToolName != "calculator" {
		t.Fatalf("expected calculator tool name to persist, got %+v", step)
	}
	if step.Description != "Use the calculator tool for exact arithmetic." || len(step.DependsOn) != 1 || step.DependsOn[0] != 1 {
		t.Fatalf("expected structured description/dependencies to persist, got %+v", step)
	}
	if step.Input["expression"] != "2+3" {
		t.Fatalf("expected structured input to persist, got %+v", step.Input)
	}
	if len(result.PlanSteps) != 2 || result.PlanSteps[1].ToolName != "calculator" || result.PlanSteps[1].Description == "" || len(result.PlanSteps[1].DependsOn) != 1 {
		t.Fatalf("expected returned plan steps to include structured tool metadata, got %+v", result.PlanSteps)
	}
}

func TestServiceAdjustPlanStepsReplacesRemainingSuffix(t *testing.T) {
	completedAt := time.Now().UTC()
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			SystemPrompt:   "Follow project constraints.",
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
			Mode:           ExecutionModePlanning,
			Status:         RunStatusPendingApproval,
			Error:          "old suffix failed",
		}},
		messages: []*Message{{
			ID:             "msg_1",
			ConversationID: "conv_1",
			OrganizationID: "org_1",
			Role:           "user",
			Content:        "ship the feature",
			CreatedAt:      completedAt.Add(-time.Minute),
		}},
		planSteps: []*PlanStep{
			{
				ID:             "step_done",
				RunID:          "run_1",
				OrganizationID: "org_1",
				Index:          1,
				Title:          "Collect current evidence",
				Status:         PlanStepStatusCompleted,
				ApprovalStatus: ApprovalStatusNotRequired,
				ResultContent:  "matrix says row 24 is partial",
				CompletedAt:    &completedAt,
				CreatedAt:      completedAt.Add(-3 * time.Minute),
			},
			{
				ID:             "step_pending",
				RunID:          "run_1",
				OrganizationID: "org_1",
				Index:          2,
				Title:          "Implement stale next step",
				Status:         PlanStepStatusPending,
				ApprovalStatus: ApprovalStatusPending,
				CreatedAt:      completedAt.Add(-2 * time.Minute),
			},
			{
				ID:             "step_failed",
				RunID:          "run_1",
				OrganizationID: "org_1",
				Index:          3,
				Title:          "Retry old failing check",
				Status:         PlanStepStatusFailed,
				ApprovalStatus: ApprovalStatusApproved,
				Error:          "old failure",
				CompletedAt:    &completedAt,
				CreatedAt:      completedAt.Add(-time.Minute),
			},
		},
	}
	gateway := &fakeGateway{plainReply: `[
		{"title":"Implement adjusted backend contract","toolName":"write_file","input":{"path":"src/server/internal/agent/service.go"}},
		{"title":"Verify adjusted planning flow","input":{"command":"go test ./internal/agent"}}
	]`}
	service := NewService(store, gateway)

	result, err := service.AdjustPlanSteps(
		context.Background(),
		auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1", User: auth.User{ID: "user_1"}},
		"run_1",
		"scope changed after completed evidence",
	)
	if err != nil {
		t.Fatalf("AdjustPlanSteps returned error: %v", err)
	}
	if result.Run == nil || result.Run.Status != RunStatusPendingApproval || result.Run.Error != "" || result.Run.CompletedAt != nil {
		t.Fatalf("expected run reopened for pending approval, got %+v", result.Run)
	}
	if result.Run.FinalMessageID == "" {
		t.Fatalf("expected adjusted planning reply to become final message, got %+v", result.Run)
	}
	if len(result.Messages) != 2 {
		t.Fatalf("expected user request plus adjusted assistant reply, got %+v", result.Messages)
	}
	adjustedReply := result.Messages[1]
	if adjustedReply.ID != result.Run.FinalMessageID || adjustedReply.Role != "assistant" || adjustedReply.Content != gateway.plainReply {
		t.Fatalf("expected adjusted assistant reply to be persisted and referenced, run=%+v messages=%+v", result.Run, result.Messages)
	}
	if gateway.plainCalls != 1 {
		t.Fatalf("expected one model call, got %d", gateway.plainCalls)
	}
	prompt := chatMessagesContent(gateway.lastPlainMessages)
	for _, want := range []string{"scope changed after completed evidence", "Collect current evidence", "matrix says row 24 is partial", "Implement stale next step", "Retry old failing check"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("adjust prompt missing %q:\n%s", want, prompt)
		}
	}

	steps := store.planSteps
	sortPlanSteps(steps)
	if len(steps) != 3 {
		t.Fatalf("expected completed prefix plus two adjusted steps, got %+v", steps)
	}
	if steps[0].ID != "step_done" || steps[0].Index != 1 || steps[0].ResultContent != "matrix says row 24 is partial" {
		t.Fatalf("completed prefix was not preserved: %+v", steps[0])
	}
	if steps[1].ID == "step_pending" || steps[2].ID == "step_failed" {
		t.Fatalf("old suffix steps should have been replaced, got %+v", steps)
	}
	if steps[1].Index != 2 || steps[1].Title != "Implement adjusted backend contract" || steps[1].Status != PlanStepStatusPending || steps[1].ApprovalStatus != ApprovalStatusPending {
		t.Fatalf("first adjusted step mismatch: %+v", steps[1])
	}
	if steps[1].ToolName != "write_file" || steps[1].Input["path"] != "src/server/internal/agent/service.go" {
		t.Fatalf("structured tool metadata was not preserved: %+v", steps[1])
	}
	if steps[2].Index != 3 || steps[2].Title != "Verify adjusted planning flow" || steps[2].Status != PlanStepStatusPending || steps[2].ApprovalStatus != ApprovalStatusNotRequired {
		t.Fatalf("second adjusted step mismatch: %+v", steps[2])
	}
	if len(result.PlanSteps) != 3 || result.PlanSteps[1].Title != "Implement adjusted backend contract" {
		t.Fatalf("expected refreshed run detail with adjusted steps, got %+v", result.PlanSteps)
	}
}

func TestServiceAdjustPlanStepsRejectsNonAdjustableRunStatuses(t *testing.T) {
	now := time.Now().UTC()

	for _, status := range []string{
		RunStatusRunning,
		RunStatusCompleted,
		RunStatusFailed,
		RunStatusMaxIterationsReached,
		RunStatusTokenBudgetExceeded,
	} {
		t.Run(status, func(t *testing.T) {
			store := &fakeStore{
				agent: &Agent{
					ID:             "agent_1",
					OrganizationID: "org_1",
					UserID:         "user_1",
					Model:          "gpt-4o-mini",
				},
				runs: []*Run{{
					ID:             "run_1",
					OrganizationID: "org_1",
					ConversationID: "conv_1",
					AgentID:        "agent_1",
					UserID:         "user_1",
					Mode:           ExecutionModePlanning,
					Status:         status,
					Error:          "terminal evidence must stay",
					StartedAt:      now,
					CompletedAt:    &now,
					CreatedAt:      now,
					UpdatedAt:      now,
				}},
				messages: []*Message{{
					ID:             "msg_1",
					ConversationID: "conv_1",
					OrganizationID: "org_1",
					Role:           "user",
					Content:        "adjust this plan",
					CreatedAt:      now,
				}},
				planSteps: []*PlanStep{{
					ID:             "step_1",
					RunID:          "run_1",
					OrganizationID: "org_1",
					Index:          1,
					Title:          "Do not replace",
					Status:         PlanStepStatusPending,
					ApprovalStatus: ApprovalStatusNotRequired,
					CreatedAt:      now,
				}},
			}
			gateway := &fakeGateway{plainReply: `[{"title":"Unexpected adjusted step"}]`}
			service := NewService(store, gateway)

			result, err := service.AdjustPlanSteps(
				context.Background(),
				auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1", User: auth.User{ID: "user_1"}},
				"run_1",
				"operator wants to adjust terminal run",
			)
			if err == nil || !strings.Contains(err.Error(), "planning run cannot be adjusted from status "+status) {
				t.Fatalf("expected invalid adjustment error for status %s, got result=%+v err=%v", status, result, err)
			}
			if gateway.plainCalls != 0 {
				t.Fatalf("adjust-plan should fail before model call for status %s, got %d calls", status, gateway.plainCalls)
			}
			if len(store.planSteps) != 1 || store.planSteps[0].ID != "step_1" || store.planSteps[0].Title != "Do not replace" {
				t.Fatalf("adjust-plan rejection mutated plan steps for status %s: %+v", status, store.planSteps)
			}
			if len(store.messages) != 1 {
				t.Fatalf("adjust-plan rejection should not append assistant messages for status %s: %+v", status, store.messages)
			}
			if store.runs[0].Status != status || store.runs[0].Error != "terminal evidence must stay" || store.runs[0].CompletedAt == nil {
				t.Fatalf("adjust-plan rejection mutated run evidence for status %s: %+v", status, store.runs[0])
			}
		})
	}
}

func TestServicePlanningOnlyActionsRejectReactRunWithoutMutation(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
		},
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Mode:           ExecutionModeReact,
			Status:         RunStatusPendingApproval,
			Error:          "react approval evidence must stay",
			StartedAt:      now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		messages: []*Message{{
			ID:             "msg_1",
			ConversationID: "conv_1",
			OrganizationID: "org_1",
			Role:           "assistant",
			Content:        "react tool approval pending",
			CreatedAt:      now,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          1,
			Title:          "Synthetic stale plan step",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusNotRequired,
			CreatedAt:      now,
		}},
	}
	gateway := &fakeGateway{plainReply: `[{"title":"Unexpected plan"}]`}
	executor := &fakePlanStepExecutor{resultContent: "unexpected execution"}
	service := NewService(store, gateway)
	service.SetPlanStepExecutor(executor)
	session := auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1", User: auth.User{ID: "user_1"}}

	adjusted, adjustErr := service.AdjustPlanSteps(context.Background(), session, "run_1", "operator tries planning action")
	if adjustErr == nil || adjustErr.Error() != "agent run is not in planning mode" {
		t.Fatalf("expected planning-mode adjustment rejection, got result=%+v err=%v", adjusted, adjustErr)
	}
	continued, continueErr := service.ContinuePlanningRun(context.Background(), session, "run_1")
	if continueErr == nil || continueErr.Error() != "agent run is not in planning mode" {
		t.Fatalf("expected planning-mode continuation rejection, got result=%+v err=%v", continued, continueErr)
	}
	if gateway.plainCalls != 0 {
		t.Fatalf("react run planning-only rejection should not call model, got %d calls", gateway.plainCalls)
	}
	if executor.calls != 0 {
		t.Fatalf("react run planning-only rejection should not execute plan steps, got %d calls", executor.calls)
	}
	if len(store.messages) != 1 {
		t.Fatalf("react run planning-only rejection should not append messages, got %+v", store.messages)
	}
	if len(store.planSteps) != 1 || store.planSteps[0].ID != "step_1" || store.planSteps[0].Title != "Synthetic stale plan step" {
		t.Fatalf("react run planning-only rejection should not replace plan steps, got %+v", store.planSteps)
	}
	if len(store.updateRunRequests) != 0 {
		t.Fatalf("react run planning-only rejection should not update run, got %+v", store.updateRunRequests)
	}
	if store.runs[0].Mode != ExecutionModeReact || store.runs[0].Status != RunStatusPendingApproval || store.runs[0].Error != "react approval evidence must stay" {
		t.Fatalf("react run evidence changed after planning-only rejection: %+v", store.runs[0])
	}
}

func TestServicePlanStepActionsRejectReactRunWithoutMutation(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*fakeStore, time.Time)
		act       func(*Service, auth.Session) error
	}{
		{
			name: "approve",
			configure: func(store *fakeStore, now time.Time) {
				store.planSteps[0].ApprovalStatus = ApprovalStatusPending
			},
			act: func(service *Service, session auth.Session) error {
				_, err := service.ApprovePlanStep(context.Background(), session, "step_1", "ready")
				return err
			},
		},
		{
			name: "update",
			act: func(service *Service, session auth.Session) error {
				_, err := service.UpdatePlanStepDraft(context.Background(), session, "step_1", UpdatePlanStepDraftRequest{
					Title: stringPointer("mutated title"),
				})
				return err
			},
		},
		{
			name: "move",
			act: func(service *Service, session auth.Session) error {
				_, err := service.MovePlanStep(context.Background(), session, "step_2", MovePlanStepDirectionUp)
				return err
			},
		},
		{
			name: "delete",
			act: func(service *Service, session auth.Session) error {
				_, err := service.DeletePlanStepDraft(context.Background(), session, "step_1")
				return err
			},
		},
		{
			name: "execute",
			act: func(service *Service, session auth.Session) error {
				_, err := service.ExecutePlanStep(context.Background(), session, "step_1")
				return err
			},
		},
		{
			name: "skip",
			act: func(service *Service, session auth.Session) error {
				_, err := service.SkipPlanStep(context.Background(), session, "step_1", "not needed")
				return err
			},
		},
		{
			name: "retry",
			configure: func(store *fakeStore, now time.Time) {
				completedAt := now.Add(time.Minute)
				store.planSteps[0].Status = PlanStepStatusFailed
				store.planSteps[0].Error = "old failure"
				store.planSteps[0].CompletedAt = &completedAt
			},
			act: func(service *Service, session auth.Session) error {
				_, err := service.RetryPlanStep(context.Background(), session, "step_1")
				return err
			},
		},
		{
			name: "create",
			act: func(service *Service, session auth.Session) error {
				_, err := service.CreatePlanStepDraft(context.Background(), session, "run_1", CreatePlanStepDraftRequest{
					Title: "Unexpected new planning step",
				})
				return err
			},
		},
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
					Mode:           ExecutionModeReact,
					Status:         RunStatusPendingApproval,
					Error:          "react approval evidence must stay",
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
						Title:          "Synthetic stale plan step",
						Status:         PlanStepStatusPending,
						ApprovalStatus: ApprovalStatusNotRequired,
						ToolName:       "calculator",
						Input:          map[string]any{"expression": "2 + 3"},
						CreatedAt:      now,
						UpdatedAt:      now,
					},
					{
						ID:             "step_2",
						RunID:          "run_1",
						OrganizationID: "org_1",
						Index:          2,
						Title:          "Second stale plan step",
						Status:         PlanStepStatusPending,
						ApprovalStatus: ApprovalStatusNotRequired,
						CreatedAt:      now,
						UpdatedAt:      now,
					},
				},
			}
			if tt.configure != nil {
				tt.configure(store, now)
			}
			originalSteps := clonePlanStepSnapshots(store.planSteps)
			executor := &fakePlanStepExecutor{resultContent: "unexpected execution"}
			service := NewService(store, &fakeGateway{plainReply: `[{"title":"unexpected"}]`})
			service.SetPlanStepExecutor(executor)
			session := auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1", User: auth.User{ID: "user_1"}}

			err := tt.act(service, session)
			if err == nil || err.Error() != "agent run is not in planning mode" {
				t.Fatalf("expected non-planning plan-step rejection, got %v", err)
			}
			if executor.calls != 0 {
				t.Fatalf("plan-step rejection should not execute tools/model work, got %d calls", executor.calls)
			}
			if len(store.toolRuns) != 0 {
				t.Fatalf("plan-step rejection should not create tool runs, got %+v", store.toolRuns)
			}
			if len(store.updateRunRequests) != 0 {
				t.Fatalf("plan-step rejection should not update run, got %+v", store.updateRunRequests)
			}
			if len(store.runs) != 1 || store.runs[0].Mode != ExecutionModeReact || store.runs[0].Status != RunStatusPendingApproval || store.runs[0].Error != "react approval evidence must stay" {
				t.Fatalf("plan-step rejection mutated React run evidence: %+v", store.runs)
			}
			if !reflect.DeepEqual(originalSteps, clonePlanStepSnapshots(store.planSteps)) {
				t.Fatalf("plan-step rejection mutated stale React plan steps: before=%+v after=%+v", originalSteps, store.planSteps)
			}
		})
	}
}

func TestServicePlanStepActionsRejectNonRecoverableParentRunStatesWithoutMutation(t *testing.T) {
	terminalOrBusyStatuses := []string{
		RunStatusRunning,
		RunStatusCompleted,
		RunStatusFailed,
		RunStatusMaxIterationsReached,
		RunStatusTokenBudgetExceeded,
	}
	tests := []struct {
		name      string
		action    string
		statuses  []string
		configure func(*fakeStore, time.Time)
		act       func(*Service, auth.Session) error
	}{
		{
			name:     "approve",
			action:   "approve",
			statuses: terminalOrBusyStatuses,
			configure: func(store *fakeStore, now time.Time) {
				store.planSteps[0].Status = PlanStepStatusPending
				store.planSteps[0].ApprovalStatus = ApprovalStatusPending
			},
			act: func(service *Service, session auth.Session) error {
				_, err := service.ApprovePlanStep(context.Background(), session, "step_1", "late approval")
				return err
			},
		},
		{
			name:     "create",
			action:   "create",
			statuses: terminalOrBusyStatuses,
			act: func(service *Service, session auth.Session) error {
				_, err := service.CreatePlanStepDraft(context.Background(), session, "run_1", CreatePlanStepDraftRequest{
					Title: "Unexpected new step",
				})
				return err
			},
		},
		{
			name:     "update",
			action:   "update",
			statuses: terminalOrBusyStatuses,
			act: func(service *Service, session auth.Session) error {
				_, err := service.UpdatePlanStepDraft(context.Background(), session, "step_1", UpdatePlanStepDraftRequest{
					Title: stringPointer("Unexpected edited step"),
				})
				return err
			},
		},
		{
			name:     "move",
			action:   "move",
			statuses: terminalOrBusyStatuses,
			act: func(service *Service, session auth.Session) error {
				_, err := service.MovePlanStep(context.Background(), session, "step_2", MovePlanStepDirectionUp)
				return err
			},
		},
		{
			name:     "delete",
			action:   "delete",
			statuses: terminalOrBusyStatuses,
			act: func(service *Service, session auth.Session) error {
				_, err := service.DeletePlanStepDraft(context.Background(), session, "step_1")
				return err
			},
		},
		{
			name:     "execute",
			action:   "execute",
			statuses: terminalOrBusyStatuses,
			configure: func(store *fakeStore, now time.Time) {
				store.planSteps[0].Status = PlanStepStatusApproved
				store.planSteps[0].ApprovalStatus = ApprovalStatusApproved
			},
			act: func(service *Service, session auth.Session) error {
				_, err := service.ExecutePlanStep(context.Background(), session, "step_1")
				return err
			},
		},
		{
			name:     "skip",
			action:   "skip",
			statuses: terminalOrBusyStatuses,
			act: func(service *Service, session auth.Session) error {
				_, err := service.SkipPlanStep(context.Background(), session, "step_1", "late skip")
				return err
			},
		},
		{
			name:     "retry",
			action:   "retry",
			statuses: []string{RunStatusRunning, RunStatusCompleted, RunStatusMaxIterationsReached},
			configure: func(store *fakeStore, now time.Time) {
				completedAt := now.Add(time.Minute)
				store.planSteps[0].Status = PlanStepStatusFailed
				store.planSteps[0].ApprovalStatus = ApprovalStatusApproved
				store.planSteps[0].Error = "failed evidence must stay"
				store.planSteps[0].ResultContent = "partial evidence"
				store.planSteps[0].CompletedAt = &completedAt
			},
			act: func(service *Service, session auth.Session) error {
				_, err := service.RetryPlanStep(context.Background(), session, "step_1")
				return err
			},
		},
	}

	for _, tt := range tests {
		for _, runStatus := range tt.statuses {
			t.Run(tt.name+"_"+runStatus, func(t *testing.T) {
				now := time.Now().UTC()
				completedAt := now.Add(time.Minute)
				store := &fakeStore{
					runs: []*Run{{
						ID:             "run_1",
						OrganizationID: "org_1",
						ConversationID: "conv_1",
						AgentID:        "agent_1",
						UserID:         "user_1",
						Mode:           ExecutionModePlanning,
						Status:         runStatus,
						Error:          "parent run evidence must stay",
						CompletedAt:    &completedAt,
						CreatedAt:      now,
						UpdatedAt:      now,
					}},
					planSteps: []*PlanStep{{
						ID:             "step_1",
						RunID:          "run_1",
						OrganizationID: "org_1",
						Index:          1,
						Title:          "Guarded step",
						Status:         PlanStepStatusPending,
						ApprovalStatus: ApprovalStatusNotRequired,
						ToolName:       "calculator",
						Input:          map[string]any{"expression": "2 + 3"},
						CreatedAt:      now,
						UpdatedAt:      now,
					}, {
						ID:             "step_2",
						RunID:          "run_1",
						OrganizationID: "org_1",
						Index:          2,
						Title:          "Second guarded step",
						Status:         PlanStepStatusApproved,
						ApprovalStatus: ApprovalStatusApproved,
						CreatedAt:      now,
						UpdatedAt:      now,
					}},
				}
				if tt.configure != nil {
					tt.configure(store, now)
				}
				originalSteps := clonePlanStepSnapshots(store.planSteps)
				executor := &fakePlanStepExecutor{resultContent: "unexpected execution"}
				service := NewService(store, &fakeGateway{})
				service.SetPlanStepExecutor(executor)
				session := auth.Session{OrganizationID: "org_1", WorkspaceID: "workspace_1", User: auth.User{ID: "user_1"}}

				err := tt.act(service, session)
				if err == nil || !strings.Contains(err.Error(), "planning run cannot "+tt.action+" plan step from status "+runStatus) {
					t.Fatalf("expected parent status rejection for %s from %s, got %v", tt.action, runStatus, err)
				}
				if executor.calls != 0 {
					t.Fatalf("parent status rejection should not execute plan steps, got %d calls", executor.calls)
				}
				if len(store.toolRuns) != 0 {
					t.Fatalf("parent status rejection should not create tool runs, got %+v", store.toolRuns)
				}
				if len(store.updateRunRequests) != 0 {
					t.Fatalf("parent status rejection should not update run, got %+v", store.updateRunRequests)
				}
				if store.runs[0].Status != runStatus || store.runs[0].Error != "parent run evidence must stay" || store.runs[0].CompletedAt != &completedAt {
					t.Fatalf("parent status rejection mutated run evidence: %+v", store.runs[0])
				}
				if !reflect.DeepEqual(originalSteps, clonePlanStepSnapshots(store.planSteps)) {
					t.Fatalf("parent status rejection mutated plan steps: before=%+v after=%+v", originalSteps, store.planSteps)
				}
			})
		}
	}
}

func TestServiceContinuePlanningRunExecutesUntilNextApproval(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Mode:           ExecutionModePlanning,
			Status:         RunStatusPendingApproval,
			StartedAt:      now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		messages: []*Message{{
			ID:             "msg_1",
			ConversationID: "conv_1",
			OrganizationID: "org_1",
			Role:           "user",
			Content:        "finish the plan",
			CreatedAt:      now,
		}},
		planSteps: []*PlanStep{
			{
				ID:             "step_1",
				RunID:          "run_1",
				OrganizationID: "org_1",
				Index:          1,
				Title:          "Already done",
				Status:         PlanStepStatusCompleted,
				ApprovalStatus: ApprovalStatusNotRequired,
				ResultContent:  "done",
				CompletedAt:    &now,
				CreatedAt:      now,
			},
			{
				ID:             "step_2",
				RunID:          "run_1",
				OrganizationID: "org_1",
				Index:          2,
				Title:          "Run without approval",
				Status:         PlanStepStatusPending,
				ApprovalStatus: ApprovalStatusNotRequired,
				CreatedAt:      now,
			},
			{
				ID:             "step_3",
				RunID:          "run_1",
				OrganizationID: "org_1",
				Index:          3,
				Title:          "Run approved step",
				Status:         PlanStepStatusApproved,
				ApprovalStatus: ApprovalStatusApproved,
				CreatedAt:      now,
			},
			{
				ID:             "step_4",
				RunID:          "run_1",
				OrganizationID: "org_1",
				Index:          4,
				Title:          "Wait for approval",
				Status:         PlanStepStatusPending,
				ApprovalStatus: ApprovalStatusPending,
				CreatedAt:      now,
			},
		},
	}
	executor := &fakePlanStepExecutor{resultContent: "continued step result"}
	service := NewService(store, &fakeGateway{})
	service.SetPlanStepExecutor(executor)

	result, err := service.ContinuePlanningRun(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		"run_1",
	)
	if err != nil {
		t.Fatalf("ContinuePlanningRun returned error: %v", err)
	}
	if executor.calls != 2 {
		t.Fatalf("expected two executable steps to run, got %d calls", executor.calls)
	}
	if result.Run == nil || result.Run.Status != RunStatusPendingApproval || result.Run.CompletedAt != nil {
		t.Fatalf("expected run to stop at next approval, got %+v", result.Run)
	}
	steps := result.PlanSteps
	sortPlanSteps(steps)
	if steps[1].Status != PlanStepStatusCompleted || steps[1].ResultContent != "continued step result" {
		t.Fatalf("expected step 2 completed, got %+v", steps[1])
	}
	if steps[2].Status != PlanStepStatusCompleted || steps[2].ResultContent != "continued step result" {
		t.Fatalf("expected step 3 completed, got %+v", steps[2])
	}
	if steps[3].Status != PlanStepStatusPending || steps[3].ApprovalStatus != ApprovalStatusPending || steps[3].StartedAt != nil {
		t.Fatalf("expected step 4 untouched as next approval gate, got %+v", steps[3])
	}
}

func TestServiceContinuePlanningRunExecutesExplicitDependencyPastPendingApprovalGate(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Mode:           ExecutionModePlanning,
			Status:         RunStatusPendingApproval,
			StartedAt:      now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		messages: []*Message{{
			ID:             "msg_1",
			ConversationID: "conv_1",
			OrganizationID: "org_1",
			Role:           "user",
			Content:        "continue ready branches",
			CreatedAt:      now,
		}},
		planSteps: []*PlanStep{
			{
				ID:             "step_1",
				RunID:          "run_1",
				OrganizationID: "org_1",
				Index:          1,
				Title:          "Collect migration evidence",
				Status:         PlanStepStatusCompleted,
				ApprovalStatus: ApprovalStatusNotRequired,
				ResultContent:  "schema evidence",
				CompletedAt:    &now,
				CreatedAt:      now,
			},
			{
				ID:             "step_2",
				RunID:          "run_1",
				OrganizationID: "org_1",
				Index:          2,
				Title:          "Review destructive cleanup",
				Status:         PlanStepStatusPending,
				ApprovalStatus: ApprovalStatusPending,
				ToolName:       "write_file",
				CreatedAt:      now,
			},
			{
				ID:             "step_3",
				RunID:          "run_1",
				OrganizationID: "org_1",
				Index:          3,
				Title:          "Run dependent verification",
				Status:         PlanStepStatusPending,
				ApprovalStatus: ApprovalStatusNotRequired,
				DependsOn:      []int{1},
				CreatedAt:      now,
			},
		},
	}
	executor := &fakePlanStepExecutor{resultContent: "verification passed"}
	service := NewService(store, &fakeGateway{})
	service.SetPlanStepExecutor(executor)

	result, err := service.ContinuePlanningRun(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		"run_1",
	)
	if err != nil {
		t.Fatalf("ContinuePlanningRun returned error: %v", err)
	}
	if executor.calls != 1 || executor.seenStep == nil || executor.seenStep.ID != "step_3" {
		t.Fatalf("expected continuation to execute only dependency-ready step 3, calls=%d step=%+v", executor.calls, executor.seenStep)
	}
	if result.Run == nil || result.Run.Status != RunStatusPendingApproval || result.Run.CompletedAt != nil {
		t.Fatalf("expected run to remain pending approval, got %+v", result.Run)
	}
	steps := result.PlanSteps
	sortPlanSteps(steps)
	if steps[1].Status != PlanStepStatusPending || steps[1].ApprovalStatus != ApprovalStatusPending || steps[1].StartedAt != nil {
		t.Fatalf("expected pending approval gate to remain untouched, got %+v", steps[1])
	}
	if steps[2].Status != PlanStepStatusCompleted || steps[2].ResultContent != "verification passed" || steps[2].StartedAt == nil {
		t.Fatalf("expected explicit-dependency step to complete, got %+v", steps[2])
	}
}

func TestServiceContinuePlanningRunCompletesRunWhenAllExecutableStepsDone(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Mode:           ExecutionModePlanning,
			Status:         RunStatusPendingApproval,
			IterationCount: 1,
			FinalMessageID: "msg_plan",
			StartedAt:      now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          1,
			Title:          "Implement",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusNotRequired,
			CreatedAt:      now,
		}, {
			ID:             "step_2",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          2,
			Title:          "Verify",
			Status:         PlanStepStatusApproved,
			ApprovalStatus: ApprovalStatusApproved,
			ToolName:       "write_file",
			Input:          map[string]any{"path": "result.md"},
			CreatedAt:      now,
		}},
	}
	executor := &fakePlanStepExecutor{resultContent: "done"}
	service := NewService(store, &fakeGateway{})
	service.SetPlanStepExecutor(executor)

	result, err := service.ContinuePlanningRun(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		"run_1",
	)
	if err != nil {
		t.Fatalf("ContinuePlanningRun returned error: %v", err)
	}
	if executor.calls != 2 {
		t.Fatalf("expected both steps to execute, got %d calls", executor.calls)
	}
	if result.Run == nil || result.Run.Status != RunStatusCompleted || result.Run.CompletedAt == nil {
		t.Fatalf("expected planning run to complete, got %+v", result.Run)
	}
	if result.Run.FinalMessageID != "msg_plan" {
		t.Fatalf("expected continue completion to preserve final message id, got %+v", result.Run)
	}
	if result.Run.IterationCount != 2 || result.Run.ToolCallCount != 1 {
		t.Fatalf("expected continue completion evidence to converge counters, got iterations=%d toolCalls=%d", result.Run.IterationCount, result.Run.ToolCallCount)
	}
	for _, step := range result.PlanSteps {
		if step.Status != PlanStepStatusCompleted || step.ResultContent != "done" {
			t.Fatalf("expected every step completed with executor result, got %+v", result.PlanSteps)
		}
	}
}

func TestServiceContinuePlanningRunReturnsDetailWhenPlanStepFails(t *testing.T) {
	now := time.Now().UTC()
	store := &fakeStore{
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Mode:           ExecutionModePlanning,
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
			Title:          "Failing step",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusNotRequired,
			CreatedAt:      now,
		}, {
			ID:             "step_2",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          2,
			Title:          "Future step",
			Status:         PlanStepStatusPending,
			ApprovalStatus: ApprovalStatusNotRequired,
			CreatedAt:      now,
		}},
	}
	executor := &fakePlanStepExecutor{err: errors.New("executor failed")}
	service := NewService(store, &fakeGateway{})
	service.SetPlanStepExecutor(executor)

	result, err := service.ContinuePlanningRun(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		"run_1",
	)
	if err != nil {
		t.Fatalf("ContinuePlanningRun should return refreshed failure detail instead of surfacing executor error, got %v", err)
	}
	if executor.calls != 1 {
		t.Fatalf("expected execution to stop after first failed step, got %d calls", executor.calls)
	}
	steps := result.PlanSteps
	sortPlanSteps(steps)
	if steps[0].Status != PlanStepStatusFailed || !strings.Contains(steps[0].Error, "executor failed") || steps[0].CompletedAt == nil {
		t.Fatalf("expected first step failure evidence, got %+v", steps[0])
	}
	if steps[1].Status != PlanStepStatusPending || steps[1].StartedAt != nil {
		t.Fatalf("expected future step untouched after failure, got %+v", steps[1])
	}
}

func TestServiceContinuePlanningRunRejectsNonContinuableStatuses(t *testing.T) {
	now := time.Now().UTC()

	for _, status := range []string{
		RunStatusCompleted,
		RunStatusFailed,
		RunStatusMaxIterationsReached,
		RunStatusTokenBudgetExceeded,
	} {
		t.Run(status, func(t *testing.T) {
			store := &fakeStore{
				runs: []*Run{{
					ID:             "run_1",
					OrganizationID: "org_1",
					ConversationID: "conv_1",
					AgentID:        "agent_1",
					UserID:         "user_1",
					Mode:           ExecutionModePlanning,
					Status:         status,
					StartedAt:      now,
					CompletedAt:    &now,
					CreatedAt:      now,
					UpdatedAt:      now,
				}},
				planSteps: []*PlanStep{{
					ID:             "step_1",
					RunID:          "run_1",
					OrganizationID: "org_1",
					Index:          1,
					Title:          "Should not execute",
					Status:         PlanStepStatusPending,
					ApprovalStatus: ApprovalStatusNotRequired,
					CreatedAt:      now,
				}},
			}
			executor := &fakePlanStepExecutor{resultContent: "unexpected execution"}
			service := NewService(store, &fakeGateway{})
			service.SetPlanStepExecutor(executor)

			result, err := service.ContinuePlanningRun(
				context.Background(),
				auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
				"run_1",
			)
			if err == nil || !strings.Contains(err.Error(), "planning run cannot be continued from status "+status) {
				t.Fatalf("expected invalid continuation error for status %s, got result=%+v err=%v", status, result, err)
			}
			if executor.calls != 0 {
				t.Fatalf("non-continuable status %s should not execute plan steps, got %d calls", status, executor.calls)
			}
			if store.planSteps[0].Status != PlanStepStatusPending || store.planSteps[0].StartedAt != nil || store.planSteps[0].CompletedAt != nil {
				t.Fatalf("non-continuable status %s mutated plan step: %+v", status, store.planSteps[0])
			}
		})
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
	service := newAuthorizedServiceForTest(t, store, &fakeGateway{plainReply: "unused"})

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
	service := newAuthorizedServiceForTest(t, store, gateway)

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
			Error:          "stale terminal evidence",
			IterationCount: 1,
			ToolCallCount:  1,
			StartedAt:      now,
			CompletedAt:    &now,
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
	service := newAuthorizedServiceForTest(t, store, gateway)

	updated, err := service.ApproveToolRun(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, "tool_run_datetime", "operator approved")
	if err != nil {
		t.Fatalf("ApproveToolRun returned error: %v", err)
	}
	if updated.Status != ToolRunStatusCompleted || updated.ToolName != "datetime" {
		t.Fatalf("expected first approved tool to complete before next pause, got %+v", updated)
	}
	foundReopen := false
	for _, req := range store.updateRunRequests {
		if req.Status != nil && *req.Status == RunStatusRunning {
			foundReopen = true
			if !req.ClearCompletedAt || req.Error == nil || *req.Error != "" {
				t.Fatalf("expected successful tool execution to reopen run with stale terminal evidence cleared, got %+v", req)
			}
		}
	}
	if !foundReopen {
		t.Fatalf("expected successful tool execution to reopen the run before resume, got %+v", store.updateRunRequests)
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
	service := newAuthorizedServiceForTest(t, store, gateway)

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

func TestServiceContinueRunWithTokenBudgetReturnsPendingApprovalWhenResumeNeedsToolApproval(t *testing.T) {
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
				Content:        "write the summary after continuing",
				CreatedAt:      now,
			},
			{
				ID:             "tool_1",
				ConversationID: "conv_1",
				OrganizationID: "org_1",
				Role:           "tool",
				Content:        "Summary ready.",
				ToolCallID:     "call_summary",
				CreatedAt:      now,
			},
		},
	}
	gateway := &fakeGateway{
		structured: []*chat.CompletionResponse{
			{
				ToolCalls: []chat.ToolCall{
					{ID: "call_write_file", Type: "function", Function: chat.ToolFunction{Name: "write_file", Arguments: `{"path":"summary.md","content":"ready"}`}},
				},
				FinishReason: "tool_calls",
				Usage:        &chat.CompletionUsage{TotalTokens: 300},
			},
		},
	}
	service := NewService(store, gateway)

	result, err := service.ContinueRunWithTokenBudget(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, "run_1", 2500)
	if err != nil {
		t.Fatalf("ContinueRunWithTokenBudget returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result for pending approval resume")
	}
	if gateway.structuredCalls != 1 {
		t.Fatalf("expected one resumed structured call, got %d", gateway.structuredCalls)
	}
	if len(store.toolRuns) != 1 {
		t.Fatalf("expected pending approval tool run to be persisted, got %+v", store.toolRuns)
	}
	toolRun := store.toolRuns[0]
	if toolRun.ToolName != "write_file" || toolRun.Status != ToolRunStatusPendingApproval || toolRun.ApprovalStatus != ApprovalStatusPending || toolRun.AttemptCount != 0 {
		t.Fatalf("expected resumed budget run to pause on write_file approval, got %+v", toolRun)
	}
	run := store.runs[0]
	if run.Status != RunStatusPendingApproval || run.Error != "" || run.CompletedAt != nil || run.IterationCount != 2 || run.ToolCallCount != 2 {
		t.Fatalf("expected continued run to pause cleanly for approval, got %+v", run)
	}
	if store.messages[len(store.messages)-1].Role != "assistant" || len(store.messages[len(store.messages)-1].ToolCalls) != 1 {
		t.Fatalf("expected assistant tool-call message to be persisted, got %+v", store.messages)
	}
}

func TestServiceContinueRunWithTokenBudgetRetriesPlanningStepWithTemporaryBudget(t *testing.T) {
	now := time.Now().UTC()
	completedAt := now.Add(time.Minute)
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
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Mode:           ExecutionModePlanning,
			Status:         RunStatusTokenBudgetExceeded,
			IterationCount: 1,
			Error:          "token_budget_exceeded: estimated 1800 prompt tokens exceeds budget 1000",
			CompletedAt:    &completedAt,
			StartedAt:      now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		messages: []*Message{{
			ID:             "msg_user",
			ConversationID: "conv_1",
			OrganizationID: "org_1",
			Role:           "user",
			Content:        largeContext,
			CreatedAt:      now,
		}},
		planSteps: []*PlanStep{{
			ID:             "step_1",
			RunID:          "run_1",
			OrganizationID: "org_1",
			Index:          1,
			Title:          "Summarize oversized context",
			Status:         PlanStepStatusFailed,
			ApprovalStatus: ApprovalStatusApproved,
			ResultContent:  "stale oversized result",
			Error:          "token_budget_exceeded: estimated 1800 prompt tokens exceeds budget 1000",
			StartedAt:      &now,
			CompletedAt:    &completedAt,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	gateway := &fakeGateway{plainReply: "Completed after budget increase."}
	service := NewService(store, gateway)

	result, err := service.ContinueRunWithTokenBudget(context.Background(), auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, "run_1", 100000)
	if err != nil {
		t.Fatalf("ContinueRunWithTokenBudget returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil planning continuation result")
	}
	if gateway.plainCalls != 1 || gateway.structuredCalls != 0 {
		t.Fatalf("expected planning continuation to retry the plain plan-step executor, got plain=%d structured=%d", gateway.plainCalls, gateway.structuredCalls)
	}
	if store.agent.Config.TokenBudget != 1000 {
		t.Fatalf("continue budget override should not mutate agent config, got %d", store.agent.Config.TokenBudget)
	}
	step := store.planSteps[0]
	if step.Status != PlanStepStatusCompleted || step.ResultContent != "Completed after budget increase." || step.Error != "" || step.CompletedAt == nil {
		t.Fatalf("expected planning step to complete with fresh result, got %+v", step)
	}
	run := store.runs[0]
	if run.Status != RunStatusCompleted || run.Error != "" || run.CompletedAt == nil {
		t.Fatalf("expected planning run to complete cleanly after budget continuation, got %+v", run)
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
	service := newAuthorizedServiceForTest(t, store, gateway)

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
	service := newAuthorizedServiceForTest(t, store, &fakeGateway{})
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}

	updated, err := service.RetryToolRun(context.Background(), session, "tool_run_failed")
	if err != nil {
		t.Fatalf("RetryToolRun returned error: %v", err)
	}
	if updated.Status != ToolRunStatusCompleted || updated.AttemptCount != 2 || updated.Error != "" || updated.ResultContent == "" {
		t.Fatalf("expected failed tool to be re-executed successfully, got %+v", updated)
	}
	foundReopen := false
	for _, req := range store.updateRunRequests {
		if req.Status != nil && *req.Status == RunStatusRunning {
			foundReopen = true
			if !req.ClearCompletedAt || req.Error == nil || *req.Error != "" {
				t.Fatalf("expected retry to reopen run with stale terminal evidence cleared, got %+v", req)
			}
		}
	}
	if !foundReopen {
		t.Fatalf("expected retry to reopen the failed run before resume, got %+v", store.updateRunRequests)
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

func TestServiceRetryToolRunReopensPendingApprovalWithoutExecuting(t *testing.T) {
	now := time.Now().UTC()
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
		runs: []*Run{{
			ID:             "run_1",
			OrganizationID: "org_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			UserID:         "user_1",
			Status:         RunStatusFailed,
			Error:          "tool write_file failed: review required",
			IterationCount: 1,
			ToolCallCount:  1,
			StartedAt:      now,
			CompletedAt:    &now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
		toolRuns: []*ToolRun{{
			ID:             "tool_run_failed_pending",
			OrganizationID: "org_1",
			RunID:          "run_1",
			ConversationID: "conv_1",
			AgentID:        "agent_1",
			ToolCallID:     "call_write",
			ToolName:       "write_file",
			ToolType:       "builtin",
			RiskLevel:      ToolRiskDangerous,
			Arguments:      map[string]any{"path": "/tmp/forbidden", "content": "no"},
			Status:         ToolRunStatusFailed,
			ApprovalStatus: ApprovalStatusPending,
			AttemptCount:   0,
			Error:          "review required",
			StartedAt:      &now,
			CompletedAt:    &now,
			CreatedAt:      now,
			UpdatedAt:      now,
		}},
	}
	service := NewService(store, &fakeGateway{})
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}

	reopened, err := service.RetryToolRun(context.Background(), session, "tool_run_failed_pending")
	if err != nil {
		t.Fatalf("RetryToolRun returned error: %v", err)
	}
	if reopened.Status != ToolRunStatusPendingApproval || reopened.ApprovalStatus != ApprovalStatusPending {
		t.Fatalf("expected retry to reopen pending approval, got %+v", reopened)
	}
	if reopened.AttemptCount != 0 || reopened.ResultContent != "" || reopened.Error != "" || reopened.CompletedAt != nil {
		t.Fatalf("pending-approval retry should clear stale evidence without executing, got %+v", reopened)
	}
	if len(store.messages) != 0 {
		t.Fatalf("pending-approval retry should not create tool messages before approval, got %+v", store.messages)
	}
	if store.runs[0].Status != RunStatusPendingApproval || store.runs[0].Error != "" || store.runs[0].CompletedAt != nil {
		t.Fatalf("expected run to reopen pending approval, got %+v", store.runs[0])
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
	provider := &liveWebSearchProvider{}
	service := newAuthorizedServiceForTest(t, store, gateway, func(options *ToolRuntimeOptions) {
		options.WebSearchProvider = provider
	})
	service.SetWebSearchProvider(nil)

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

func TestServiceToolRunActionsRejectNonRecoverableParentRunStates(t *testing.T) {
	invalidRunStatuses := []string{
		RunStatusRunning,
		RunStatusCompleted,
		RunStatusMaxIterationsReached,
		RunStatusTokenBudgetExceeded,
	}
	tests := []struct {
		name           string
		action         string
		toolStatus     string
		approvalStatus string
		call           func(*Service, auth.Session, string) (*ToolRun, error)
	}{
		{
			name:           "approve",
			action:         "approve",
			toolStatus:     ToolRunStatusPendingApproval,
			approvalStatus: ApprovalStatusPending,
			call: func(service *Service, session auth.Session, toolRunID string) (*ToolRun, error) {
				return service.ApproveToolRun(context.Background(), session, toolRunID, "late approval")
			},
		},
		{
			name:           "reject",
			action:         "reject",
			toolStatus:     ToolRunStatusPendingApproval,
			approvalStatus: ApprovalStatusPending,
			call: func(service *Service, session auth.Session, toolRunID string) (*ToolRun, error) {
				return service.RejectToolRun(context.Background(), session, toolRunID, "late rejection")
			},
		},
		{
			name:           "retry",
			action:         "retry",
			toolStatus:     ToolRunStatusFailed,
			approvalStatus: ApprovalStatusNotRequired,
			call: func(service *Service, session auth.Session, toolRunID string) (*ToolRun, error) {
				return service.RetryToolRun(context.Background(), session, toolRunID)
			},
		},
	}

	for _, runStatus := range invalidRunStatuses {
		for _, tt := range tests {
			t.Run(tt.name+"_"+runStatus, func(t *testing.T) {
				now := time.Now().UTC()
				completedAt := now.Add(time.Minute)
				store := &fakeStore{
					runs: []*Run{{
						ID:             "run_1",
						OrganizationID: "org_1",
						ConversationID: "conv_1",
						AgentID:        "agent_1",
						UserID:         "user_1",
						Status:         runStatus,
						Error:          "parent run evidence must stay",
						CompletedAt:    &completedAt,
						CreatedAt:      now,
						UpdatedAt:      now,
					}},
					toolRuns: []*ToolRun{{
						ID:                     "tool_run_guarded",
						OrganizationID:         "org_1",
						RunID:                  "run_1",
						ConversationID:         "conv_1",
						AgentID:                "agent_1",
						ToolCallID:             "call_guarded",
						ToolName:               "datetime",
						ToolType:               "builtin",
						Arguments:              map[string]any{},
						Status:                 tt.toolStatus,
						ApprovalStatus:         tt.approvalStatus,
						ApprovedByUserID:       "reviewer_1",
						ApprovalDecisionReason: "original decision",
						AttemptCount:           3,
						ResultContent:          "old result",
						Error:                  "old error",
						CompletedAt:            &completedAt,
						CreatedAt:              now,
						UpdatedAt:              now,
					}},
				}
				service := NewService(store, &fakeGateway{})

				result, err := tt.call(service, auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}, "tool_run_guarded")
				if err == nil || !strings.Contains(err.Error(), "agent run cannot "+tt.action+" tool run from status "+runStatus) {
					t.Fatalf("expected parent run status rejection, got result=%+v err=%v", result, err)
				}
				run := store.runs[0]
				if run.Status != runStatus || run.Error != "parent run evidence must stay" || run.CompletedAt != &completedAt {
					t.Fatalf("rejected %s mutated parent run evidence: %+v", tt.action, run)
				}
				toolRun := store.toolRuns[0]
				if toolRun.Status != tt.toolStatus || toolRun.ApprovalStatus != tt.approvalStatus ||
					toolRun.ApprovedByUserID != "reviewer_1" || toolRun.ApprovalDecisionReason != "original decision" ||
					toolRun.AttemptCount != 3 || toolRun.ResultContent != "old result" || toolRun.Error != "old error" ||
					toolRun.CompletedAt != &completedAt {
					t.Fatalf("rejected %s mutated guarded tool run: %+v", tt.action, toolRun)
				}
				if len(store.updateRunRequests) != 0 || len(store.messages) != 0 {
					t.Fatalf("rejected %s should not write run/messages, updates=%+v messages=%+v", tt.action, store.updateRunRequests, store.messages)
				}
			})
		}
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
			executor := newAuthorizedToolExecutorForTest(t, nil, func(options *ToolRuntimeOptions) {
				options.BuiltinTools = map[string]mcp.BuiltinTool{
					tt.toolCallName: &recordingBuiltinTool{name: tt.toolCallName},
				}
			})
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
	runner := NewRunner(store, gateway, newAuthorizedToolExecutorForTest(t, nil), nil, DefaultRunnerConfig())

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
	runner := NewRunner(store, gateway, newAuthorizedToolExecutorForTest(t, nil), nil, DefaultRunnerConfig())
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

func TestRunWithToolsDerivesPreferenceAndFactLongTermMemories(t *testing.T) {
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
			{Content: "I will remember that.", FinishReason: "stop"},
		},
	}
	embedder := &fakeAgentMemoryEmbedder{
		embeddings: map[string][]float32{
			"User: I prefer concise answers. My company is Acme Labs.\nAssistant: I will remember that.": {0.1, 0.2},
			"User preference: I prefer concise answers":                                                  {0.3, 0.4},
			"Important fact: My company is Acme Labs":                                                    {0.5, 0.6},
		},
	}
	runner := NewRunner(store, gateway, newAuthorizedToolExecutorForTest(t, nil), nil, DefaultRunnerConfig())
	runner.SetMemoryEmbedder(embedder)

	_, err := runner.RunWithTools(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		store.agent,
		store.conversation.ID,
		"I prefer concise answers. My company is Acme Labs.",
		nil,
	)
	if err != nil {
		t.Fatalf("RunWithTools returned error: %v", err)
	}
	if len(store.memories) != 3 {
		t.Fatalf("expected interaction, preference, and fact memories, got %+v", store.memories)
	}
	byCategory := map[string]*Memory{}
	for _, memory := range store.memories {
		category, _ := memory.Metadata["memory_category"].(string)
		byCategory[category] = memory
		if memory.Type != MemoryTypeLongTerm || memory.AgentID != "agent_1" || memory.UserID != "user_1" || memory.OrganizationID != "org_1" {
			t.Fatalf("expected scoped long-term memory, got %+v", memory)
		}
		if memory.Metadata["conversation_id"] != "conv_1" {
			t.Fatalf("expected conversation metadata, got %+v", memory.Metadata)
		}
	}
	if byCategory["interaction"] == nil || byCategory["interaction"].Metadata["source"] != "agent_run" {
		t.Fatalf("expected historical interaction memory, got %+v", byCategory)
	}
	if byCategory["preference"] == nil || byCategory["preference"].Content != "User preference: I prefer concise answers" || byCategory["preference"].Importance != 4 || byCategory["preference"].Metadata["source"] != "agent_memory_policy" {
		t.Fatalf("expected derived preference memory, got %+v", byCategory["preference"])
	}
	if byCategory["fact"] == nil || byCategory["fact"].Content != "Important fact: My company is Acme Labs" || byCategory["fact"].Importance != 4 || byCategory["fact"].Metadata["source"] != "agent_memory_policy" {
		t.Fatalf("expected derived fact memory, got %+v", byCategory["fact"])
	}
	if !containsString(embedder.texts, byCategory["interaction"].Content) ||
		!containsString(embedder.texts, byCategory["preference"].Content) ||
		!containsString(embedder.texts, byCategory["fact"].Content) {
		t.Fatalf("expected all long-term memories to be embedded, got %+v", embedder.texts)
	}
}

func TestRunWithToolsLLMAssistedLongTermMemoryExtractionStoresModelCandidates(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config: Config{
				EnableMemory:                   true,
				LongTermMemoryExtractionPolicy: LongTermMemoryExtractionPolicyLLMAssisted,
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
		plainReply: `{"memories":[{"category":"fact","content":"Important fact: Launch checklist requires legal review","importance":5}]}`,
		structured: []*chat.CompletionResponse{
			{Content: "I will keep the launch checklist in mind.", FinishReason: "stop"},
		},
	}
	runner := NewRunner(store, gateway, newAuthorizedToolExecutorForTest(t, nil), nil, DefaultRunnerConfig())

	_, err := runner.RunWithTools(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		store.agent,
		store.conversation.ID,
		"For launch readiness, legal review is part of the checklist.",
		nil,
	)
	if err != nil {
		t.Fatalf("RunWithTools returned error: %v", err)
	}
	if gateway.plainCalls != 1 {
		t.Fatalf("expected one LLM-assisted extraction call, got %d", gateway.plainCalls)
	}
	if len(gateway.lastPlainMessages) != 2 || !strings.Contains(gateway.lastPlainMessages[0].Content, "Extract durable long-term memories") {
		t.Fatalf("expected LLM-assisted extraction prompt, got %+v", gateway.lastPlainMessages)
	}
	if len(store.memories) != 2 {
		t.Fatalf("expected interaction plus LLM-assisted memory, got %+v", store.memories)
	}
	byCategory := map[string]*Memory{}
	for _, memory := range store.memories {
		category, _ := memory.Metadata["memory_category"].(string)
		byCategory[category] = memory
	}
	if byCategory["interaction"] == nil {
		t.Fatalf("expected interaction memory, got %+v", store.memories)
	}
	assisted := byCategory["fact"]
	if assisted == nil || assisted.Content != "Important fact: Launch checklist requires legal review" || assisted.Importance != 5 {
		t.Fatalf("expected LLM-assisted fact memory, got %+v", assisted)
	}
	if assisted.Metadata["source"] != "agent_memory_llm_assisted" || assisted.Metadata["extraction_policy"] != LongTermMemoryExtractionPolicyLLMAssisted || assisted.Metadata["conversation_id"] != "conv_1" {
		t.Fatalf("expected LLM-assisted metadata, got %+v", assisted.Metadata)
	}
}

func TestRunWithToolsLLMAssistedLongTermMemoryExtractionIgnoresInvalidJSON(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config: Config{
				EnableMemory:                   true,
				LongTermMemoryExtractionPolicy: LongTermMemoryExtractionPolicyLLMAssisted,
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
		plainReply: "not json",
		structured: []*chat.CompletionResponse{
			{Content: "Done.", FinishReason: "stop"},
		},
	}
	runner := NewRunner(store, gateway, newAuthorizedToolExecutorForTest(t, nil), nil, DefaultRunnerConfig())

	_, err := runner.RunWithTools(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		store.agent,
		store.conversation.ID,
		"Please answer the launch question.",
		nil,
	)
	if err != nil {
		t.Fatalf("RunWithTools returned error: %v", err)
	}
	if gateway.plainCalls != 1 {
		t.Fatalf("expected one LLM-assisted extraction call, got %d", gateway.plainCalls)
	}
	if len(store.memories) != 1 || store.memories[0].Metadata["memory_category"] != "interaction" {
		t.Fatalf("invalid LLM memory JSON should only leave interaction memory, got %+v", store.memories)
	}
}

func TestRunWithToolsLLMAssistedLongTermMemoryUpdatePolicyConsolidatesByMemoryKey(t *testing.T) {
	oldUpdatedAt := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config: Config{
				EnableMemory:                   true,
				LongTermMemoryWritePolicy:      LongTermMemoryWritePolicyExplicitOnly,
				LongTermMemoryExtractionPolicy: LongTermMemoryExtractionPolicyLLMAssisted,
				LongTermMemoryUpdatePolicy:     LongTermMemoryUpdatePolicyMemoryKeyConsolidate,
			},
		},
		conversation: &Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
		memories: []*Memory{{
			ID:             "memory_company",
			OrganizationID: "org_1",
			UserID:         "user_1",
			AgentID:        "agent_1",
			Type:           MemoryTypeLongTerm,
			Content:        "Important fact: My company is OldCo",
			Importance:     3,
			Metadata: map[string]any{
				"source":          "agent_memory_policy",
				"memory_category": "fact",
				"memory_key":      "fact:company",
				"conversation_id": "conv_old",
			},
			CreatedAt: oldUpdatedAt,
			UpdatedAt: oldUpdatedAt,
		}},
	}
	gateway := &fakeGateway{
		plainReply: `{"memories":[{"category":"fact","content":"Important fact: My company is NewCo","importance":5}]}`,
		structured: []*chat.CompletionResponse{
			{Content: "I will use the updated company context.", FinishReason: "stop"},
		},
	}
	runner := NewRunner(store, gateway, newAuthorizedToolExecutorForTest(t, nil), nil, DefaultRunnerConfig())

	_, err := runner.RunWithTools(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		store.agent,
		store.conversation.ID,
		"Use the updated launch context from the latest onboarding packet.",
		nil,
	)
	if err != nil {
		t.Fatalf("RunWithTools returned error: %v", err)
	}
	if gateway.plainCalls != 1 {
		t.Fatalf("expected one LLM-assisted extraction call, got %d", gateway.plainCalls)
	}
	if len(store.memories) != 1 {
		t.Fatalf("expected LLM-assisted keyed fact to update existing memory, got %+v", store.memories)
	}
	updated := store.memories[0]
	if updated.ID != "memory_company" || updated.Content != "Important fact: My company is NewCo" || updated.Importance != 5 {
		t.Fatalf("expected LLM-assisted company fact memory to update in place, got %+v", updated)
	}
	if updated.UpdatedAt.Equal(oldUpdatedAt) {
		t.Fatalf("expected keyed memory update timestamp to refresh, got %+v", updated)
	}
	if updated.Metadata["source"] != "agent_memory_llm_assisted" ||
		updated.Metadata["extraction_policy"] != LongTermMemoryExtractionPolicyLLMAssisted ||
		updated.Metadata["memory_key"] != "fact:company" ||
		updated.Metadata["update_policy"] != LongTermMemoryUpdatePolicyMemoryKeyConsolidate ||
		updated.Metadata["consolidated_from_memory_id"] != "memory_company" ||
		updated.Metadata["conversation_id"] != "conv_1" {
		t.Fatalf("expected LLM-assisted consolidation metadata, got %+v", updated.Metadata)
	}
}

func TestRunWithToolsLongTermMemoryUpdatePolicyConsolidatesByMemoryKey(t *testing.T) {
	oldUpdatedAt := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config: Config{
				EnableMemory:               true,
				LongTermMemoryWritePolicy:  LongTermMemoryWritePolicyExplicitOnly,
				LongTermMemoryUpdatePolicy: LongTermMemoryUpdatePolicyMemoryKeyConsolidate,
			},
		},
		conversation: &Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
		memories: []*Memory{{
			ID:             "memory_company",
			OrganizationID: "org_1",
			UserID:         "user_1",
			AgentID:        "agent_1",
			Type:           MemoryTypeLongTerm,
			Content:        "Important fact: My company is OldCo",
			Importance:     3,
			Metadata: map[string]any{
				"source":          "agent_memory_policy",
				"memory_category": "fact",
				"memory_key":      "fact:company",
				"conversation_id": "conv_old",
			},
			CreatedAt: oldUpdatedAt,
			UpdatedAt: oldUpdatedAt,
		}},
	}
	gateway := &fakeGateway{
		structured: []*chat.CompletionResponse{
			{Content: "Updated.", FinishReason: "stop"},
		},
	}
	runner := NewRunner(store, gateway, newAuthorizedToolExecutorForTest(t, nil), nil, DefaultRunnerConfig())

	_, err := runner.RunWithTools(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		store.agent,
		store.conversation.ID,
		"My company is NewCo.",
		nil,
	)
	if err != nil {
		t.Fatalf("RunWithTools returned error: %v", err)
	}
	if len(store.memories) != 1 {
		t.Fatalf("expected existing keyed memory to be consolidated, got %+v", store.memories)
	}
	updated := store.memories[0]
	if updated.ID != "memory_company" || updated.Content != "Important fact: My company is NewCo" || updated.Importance != 4 {
		t.Fatalf("expected keyed company fact memory to update, got %+v", updated)
	}
	if updated.UpdatedAt.Equal(oldUpdatedAt) {
		t.Fatalf("expected keyed memory update timestamp to refresh, got %+v", updated)
	}
	if updated.Metadata["memory_key"] != "fact:company" || updated.Metadata["update_policy"] != LongTermMemoryUpdatePolicyMemoryKeyConsolidate || updated.Metadata["consolidated_from_memory_id"] != "memory_company" || updated.Metadata["conversation_id"] != "conv_1" {
		t.Fatalf("expected consolidation metadata, got %+v", updated.Metadata)
	}
}

func TestRunWithToolsLongTermMemoryDefaultUpdatePolicyDoesNotConsolidateByMemoryKey(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config: Config{
				EnableMemory:              true,
				LongTermMemoryWritePolicy: LongTermMemoryWritePolicyExplicitOnly,
			},
		},
		conversation: &Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
		memories: []*Memory{{
			ID:             "memory_company",
			OrganizationID: "org_1",
			UserID:         "user_1",
			AgentID:        "agent_1",
			Type:           MemoryTypeLongTerm,
			Content:        "Important fact: My company is OldCo",
			Importance:     3,
			Metadata: map[string]any{
				"source":          "agent_memory_policy",
				"memory_category": "fact",
				"memory_key":      "fact:company",
				"conversation_id": "conv_old",
			},
		}},
	}
	gateway := &fakeGateway{
		structured: []*chat.CompletionResponse{
			{Content: "Updated.", FinishReason: "stop"},
		},
	}
	runner := NewRunner(store, gateway, newAuthorizedToolExecutorForTest(t, nil), nil, DefaultRunnerConfig())

	_, err := runner.RunWithTools(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		store.agent,
		store.conversation.ID,
		"My company is NewCo.",
		nil,
	)
	if err != nil {
		t.Fatalf("RunWithTools returned error: %v", err)
	}
	if len(store.memories) != 2 {
		t.Fatalf("default update policy should preserve existing keyed memory and create a new one, got %+v", store.memories)
	}
	if store.memories[0].Content != "Important fact: My company is OldCo" || store.memories[1].Content != "Important fact: My company is NewCo" {
		t.Fatalf("unexpected memories after default update policy: %+v", store.memories)
	}
}

func TestRunWithToolsLongTermMemoryUpdatePolicyDoesNotMergeDifferentPreferences(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config: Config{
				EnableMemory:               true,
				LongTermMemoryWritePolicy:  LongTermMemoryWritePolicyExplicitOnly,
				LongTermMemoryUpdatePolicy: LongTermMemoryUpdatePolicyMemoryKeyConsolidate,
			},
		},
		conversation: &Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
		memories: []*Memory{{
			ID:             "memory_preference",
			OrganizationID: "org_1",
			UserID:         "user_1",
			AgentID:        "agent_1",
			Type:           MemoryTypeLongTerm,
			Content:        "User preference: I prefer concise answers",
			Importance:     4,
			Metadata: map[string]any{
				"source":          "agent_memory_policy",
				"memory_category": "preference",
				"memory_key":      "preference:i prefer concise answers",
				"conversation_id": "conv_old",
			},
		}},
	}
	gateway := &fakeGateway{
		structured: []*chat.CompletionResponse{
			{Content: "Noted.", FinishReason: "stop"},
		},
	}
	runner := NewRunner(store, gateway, newAuthorizedToolExecutorForTest(t, nil), nil, DefaultRunnerConfig())

	_, err := runner.RunWithTools(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		store.agent,
		store.conversation.ID,
		"I prefer detailed answers.",
		nil,
	)
	if err != nil {
		t.Fatalf("RunWithTools returned error: %v", err)
	}
	if len(store.memories) != 2 {
		t.Fatalf("different preferences should not share a consolidation key, got %+v", store.memories)
	}
	if store.memories[0].Content != "User preference: I prefer concise answers" || store.memories[1].Content != "User preference: I prefer detailed answers" {
		t.Fatalf("unexpected preferences after keyed update policy: %+v", store.memories)
	}
	if store.memories[1].Metadata["memory_key"] != "preference:i prefer detailed answers" {
		t.Fatalf("expected content-derived preference memory key, got %+v", store.memories[1].Metadata)
	}
}

func TestRunWithToolsLongTermMemoryUpdatePolicyConsolidatesResponseLanguagePreference(t *testing.T) {
	oldUpdatedAt := time.Date(2026, 6, 8, 10, 0, 0, 0, time.UTC)
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config: Config{
				EnableMemory:               true,
				LongTermMemoryWritePolicy:  LongTermMemoryWritePolicyExplicitOnly,
				LongTermMemoryUpdatePolicy: LongTermMemoryUpdatePolicyMemoryKeyConsolidate,
			},
		},
		conversation: &Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
		memories: []*Memory{{
			ID:             "memory_language",
			OrganizationID: "org_1",
			UserID:         "user_1",
			AgentID:        "agent_1",
			Type:           MemoryTypeLongTerm,
			Content:        "User preference: Please always answer in English",
			Importance:     4,
			Metadata: map[string]any{
				"source":          "agent_memory_policy",
				"memory_category": "preference",
				"memory_key":      "preference:please always answer in english",
				"conversation_id": "conv_old",
			},
			CreatedAt: oldUpdatedAt,
			UpdatedAt: oldUpdatedAt,
		}},
	}
	gateway := &fakeGateway{
		structured: []*chat.CompletionResponse{
			{Content: "我会用中文回复。", FinishReason: "stop"},
		},
	}
	runner := NewRunner(store, gateway, newAuthorizedToolExecutorForTest(t, nil), nil, DefaultRunnerConfig())

	_, err := runner.RunWithTools(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		store.agent,
		store.conversation.ID,
		"请始终用中文回复。",
		nil,
	)
	if err != nil {
		t.Fatalf("RunWithTools returned error: %v", err)
	}
	if len(store.memories) != 1 {
		t.Fatalf("expected response-language preference to consolidate, got %+v", store.memories)
	}
	updated := store.memories[0]
	if updated.ID != "memory_language" || updated.Content != "User preference: 请始终用中文回复" || updated.Importance != 4 {
		t.Fatalf("expected language preference memory to update in place, got %+v", updated)
	}
	if updated.UpdatedAt.Equal(oldUpdatedAt) {
		t.Fatalf("expected language preference timestamp to refresh, got %+v", updated)
	}
	if updated.Metadata["memory_key"] != "preference:response_language" || updated.Metadata["update_policy"] != LongTermMemoryUpdatePolicyMemoryKeyConsolidate || updated.Metadata["consolidated_from_memory_id"] != "memory_language" || updated.Metadata["conversation_id"] != "conv_1" {
		t.Fatalf("expected response-language consolidation metadata, got %+v", updated.Metadata)
	}
}

func TestRunWithToolsLongTermMemoryExplicitOnlySkipsInteractionMemory(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config: Config{
				EnableMemory:              true,
				LongTermMemoryWritePolicy: LongTermMemoryWritePolicyExplicitOnly,
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
			{Content: "I will remember only explicit details.", FinishReason: "stop"},
		},
	}
	runner := NewRunner(store, gateway, newAuthorizedToolExecutorForTest(t, nil), nil, DefaultRunnerConfig())

	_, err := runner.RunWithTools(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		store.agent,
		store.conversation.ID,
		"I prefer concise answers. My company is Acme Labs.",
		nil,
	)
	if err != nil {
		t.Fatalf("RunWithTools returned error: %v", err)
	}
	if len(store.memories) != 2 {
		t.Fatalf("expected only explicit preference and fact memories, got %+v", store.memories)
	}
	for _, memory := range store.memories {
		if memory.Metadata["memory_category"] == "interaction" {
			t.Fatalf("explicit_only policy should not write interaction memory, got %+v", memory)
		}
	}
}

func TestRunWithToolsLongTermMemoryInteractionOnlySkipsExplicitMemories(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config: Config{
				EnableMemory:              true,
				LongTermMemoryWritePolicy: LongTermMemoryWritePolicyInteractionOnly,
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
			{Content: "I will keep the conversation as one interaction.", FinishReason: "stop"},
		},
	}
	runner := NewRunner(store, gateway, newAuthorizedToolExecutorForTest(t, nil), nil, DefaultRunnerConfig())

	_, err := runner.RunWithTools(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		store.agent,
		store.conversation.ID,
		"I prefer concise answers. My company is Acme Labs.",
		nil,
	)
	if err != nil {
		t.Fatalf("RunWithTools returned error: %v", err)
	}
	if len(store.memories) != 1 {
		t.Fatalf("expected only one interaction memory, got %+v", store.memories)
	}
	if store.memories[0].Metadata["memory_category"] != "interaction" {
		t.Fatalf("interaction_only policy should write only interaction memory, got %+v", store.memories[0])
	}
}

func TestRunWithToolsLongTermMemoryManualOnlySkipsAutomaticWrites(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config: Config{
				EnableMemory:              true,
				LongTermMemoryWritePolicy: LongTermMemoryWritePolicyManualOnly,
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
			{Content: "No automatic memory should be written.", FinishReason: "stop"},
		},
	}
	runner := NewRunner(store, gateway, newAuthorizedToolExecutorForTest(t, nil), nil, DefaultRunnerConfig())

	_, err := runner.RunWithTools(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		store.agent,
		store.conversation.ID,
		"I prefer concise answers. My company is Acme Labs.",
		nil,
	)
	if err != nil {
		t.Fatalf("RunWithTools returned error: %v", err)
	}
	if len(store.memories) != 0 {
		t.Fatalf("manual_only policy should not write automatic long-term memories, got %+v", store.memories)
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
	runner := NewRunner(store, gateway, newAuthorizedToolExecutorForTest(t, nil), nil, DefaultRunnerConfig())
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

func TestRunWithToolsRefreshesDuplicateDerivedPreferenceMemory(t *testing.T) {
	preferenceContent := "User preference: I prefer concise answers"
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
			ID:             "memory_preference",
			OrganizationID: "org_1",
			UserID:         "user_1",
			AgentID:        "agent_1",
			Type:           MemoryTypeLongTerm,
			Content:        preferenceContent,
			Importance:     4,
			Metadata: map[string]any{
				"source":          "agent_memory_policy",
				"memory_category": "preference",
				"conversation_id": "conv_1",
			},
			CreatedAt: oldUpdatedAt,
			UpdatedAt: oldUpdatedAt,
		}},
	}
	gateway := &fakeGateway{
		structured: []*chat.CompletionResponse{
			{Content: "Noted.", FinishReason: "stop"},
		},
	}
	embedder := &fakeAgentMemoryEmbedder{
		embeddings: map[string][]float32{
			"User: I prefer concise answers.\nAssistant: Noted.": {0.1, 0.2},
			preferenceContent: {0.3, 0.4},
		},
	}
	runner := NewRunner(store, gateway, newAuthorizedToolExecutorForTest(t, nil), nil, DefaultRunnerConfig())
	runner.SetMemoryEmbedder(embedder)

	_, err := runner.RunWithTools(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		store.agent,
		store.conversation.ID,
		"I prefer concise answers.",
		nil,
	)
	if err != nil {
		t.Fatalf("RunWithTools returned error: %v", err)
	}
	if len(store.memories) != 2 {
		t.Fatalf("expected one refreshed preference plus one new interaction memory, got %+v", store.memories)
	}
	preference := store.memories[0]
	if preference.ID != "memory_preference" || preference.UpdatedAt.Equal(oldUpdatedAt) || preference.Importance != 4 {
		t.Fatalf("expected duplicate preference memory to be refreshed, got %+v", preference)
	}
	if !reflect.DeepEqual(store.updateMemoryEmbedding, []float32{0.3, 0.4}) {
		t.Fatalf("expected duplicate preference refresh to update embedding, got %+v", store.updateMemoryEmbedding)
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
	runner := NewRunner(store, gateway, newAuthorizedToolExecutorForTest(t, nil), nil, DefaultRunnerConfig())

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
	runner := NewRunner(store, gateway, newAuthorizedToolExecutorForTest(t, nil), nil, DefaultRunnerConfig())

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
	runner := NewRunner(store, gateway, newAuthorizedToolExecutorForTest(t, nil), nil, DefaultRunnerConfig())

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

func TestRunnerRunRejectsToolEnabledAgent(t *testing.T) {
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
	gateway := &fakeGateway{plainReply: "plain fallback"}
	runner := NewRunner(store, gateway, newAuthorizedToolExecutorForTest(t, nil), nil, DefaultRunnerConfig())

	result, err := runner.Run(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		store.agent,
		store.conversation.ID,
		"what time is it?",
	)

	if !errors.Is(err, ErrStructuredGatewayRequired) {
		t.Fatalf("expected ErrStructuredGatewayRequired, got result=%+v err=%v", result, err)
	}
	if gateway.plainCalls != 0 {
		t.Fatalf("expected plain gateway not to be called for tool-enabled agent, got %d calls", gateway.plainCalls)
	}
	if len(store.messages) != 0 {
		t.Fatalf("expected no messages persisted before fail-closed guard, got %+v", store.messages)
	}
}

func TestRunnerInjectsLongTermAgentMemoriesFromTextFallback(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config:         Config{EnableMemory: true},
		},
		memories: []*Memory{
			{
				ID:             "memory_preference",
				OrganizationID: "org_1",
				UserID:         "user_1",
				AgentID:        "agent_1",
				Type:           MemoryTypeUserManaged,
				Content:        "When asked about migration guard, prefer concise release-note bullets.",
				Importance:     4,
			},
			{
				ID:             "memory_interaction",
				OrganizationID: "org_1",
				UserID:         "user_1",
				AgentID:        "agent_1",
				Type:           MemoryTypeLongTerm,
				Content:        "Previous migration guard answer: use the tenant-safe migration guard.",
				Importance:     5,
			},
			{
				ID:             "memory_short_term",
				OrganizationID: "org_1",
				UserID:         "user_1",
				AgentID:        "agent_1",
				Type:           MemoryTypeShortTerm,
				Content:        "Short-term migration guard note should stay outside long-term retrieval.",
				Importance:     5,
			},
		},
	}
	runner := NewRunner(store, &fakeGateway{}, newAuthorizedToolExecutorForTest(t, nil), nil, DefaultRunnerConfig())

	messages, evidence := runner.buildChatMessagesWithEvidence(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		store.agent,
		[]*Message{{Role: "user", Content: "migration guard"}},
		"migration guard",
	)

	if !evidence.enabled || !evidence.searched || evidence.resultCount != 2 {
		t.Fatalf("expected two layered memory results, got %+v", evidence)
	}
	if !reflect.DeepEqual(store.listMemoryTypes, []string{MemoryTypeUserManaged, MemoryTypeLongTerm}) {
		t.Fatalf("expected text fallback to search user-managed and long-term memories, got %+v", store.listMemoryTypes)
	}
	prompt := chatMessagesContent(messages)
	if !strings.Contains(prompt, "prefer concise release-note bullets") || !strings.Contains(prompt, "tenant-safe migration guard") {
		t.Fatalf("expected user-managed and long-term memories in prompt, got %q", prompt)
	}
	if strings.Contains(prompt, "Short-term migration guard note") {
		t.Fatalf("expected short-term memory to stay out of layered retrieval, got %q", prompt)
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
	runner := NewRunner(store, gateway, newAuthorizedToolExecutorForTest(t, nil), nil, DefaultRunnerConfig())
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

func TestRunnerVectorSearchCoversLongTermAgentMemories(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config:         Config{EnableMemory: true},
		},
		searchMemoryResultsByType: map[string][]*MemorySearchResult{
			MemoryTypeUserManaged: {
				{
					Memory: Memory{
						ID:             "memory_preference",
						OrganizationID: "org_1",
						UserID:         "user_1",
						AgentID:        "agent_1",
						Type:           MemoryTypeUserManaged,
						Content:        "User preference: keep migration guard answers concise.",
						Importance:     5,
					},
					Score: 0.82,
				},
			},
			MemoryTypeLongTerm: {
				{
					Memory: Memory{
						ID:             "memory_interaction",
						OrganizationID: "org_1",
						UserID:         "user_1",
						AgentID:        "agent_1",
						Type:           MemoryTypeLongTerm,
						Content:        "Historical interaction: tenant-safe migration guard was accepted.",
						Importance:     3,
					},
					Score: 0.94,
				},
			},
		},
	}
	runner := NewRunner(store, &fakeGateway{}, newAuthorizedToolExecutorForTest(t, nil), nil, DefaultRunnerConfig())
	runner.SetMemoryEmbedder(&fakeAgentMemoryEmbedder{
		embeddings: map[string][]float32{"migration guard": {0.25, 0.75}},
	})

	messages, evidence := runner.buildChatMessagesWithEvidence(
		context.Background(),
		auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}},
		store.agent,
		[]*Message{{Role: "user", Content: "migration guard"}},
		"migration guard",
	)

	if !evidence.enabled || !evidence.searched || evidence.resultCount != 2 {
		t.Fatalf("expected two vector memory results, got %+v", evidence)
	}
	if !reflect.DeepEqual(store.searchMemoryTypes, []string{MemoryTypeUserManaged, MemoryTypeLongTerm}) {
		t.Fatalf("expected vector search to cover user-managed and long-term memories, got %+v", store.searchMemoryTypes)
	}
	if len(store.listMemoryTypes) != 0 {
		t.Fatalf("expected vector results to avoid text fallback, got text lookups %+v", store.listMemoryTypes)
	}
	prompt := chatMessagesContent(messages)
	longTermIndex := strings.Index(prompt, "tenant-safe migration guard was accepted")
	preferenceIndex := strings.Index(prompt, "keep migration guard answers concise")
	if longTermIndex < 0 || preferenceIndex < 0 {
		t.Fatalf("expected both vector memory types in prompt, got %q", prompt)
	}
	if longTermIndex > preferenceIndex {
		t.Fatalf("expected higher-scored long-term memory to rank before preference, got %q", prompt)
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

func containsString(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
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

	service := newAuthorizedServiceForTest(t, store, gateway)
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

	service := newAuthorizedServiceForTest(t, store, gateway)
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

// TestRunWithToolsRejectsPlainGateway verifies that tool-enabled runs fail
// closed instead of silently completing through a non-structured gateway.
func TestRunWithToolsRejectsPlainGateway(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:           "agent_1",
			UserID:       "user_1",
			Model:        "gpt-4o-mini",
			SystemPrompt: "Base prompt",
			Config: Config{
				Skills: []Skill{{
					Name:         "Weather",
					Instructions: "Provide weather-specific checks",
					Triggers:     []string{"weather"},
				}, {
					Name:         "Calculator",
					Instructions: "Perform math checks",
					Triggers:     []string{"calculate"},
				}},
				MaxSkills: 1,
			},
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

	gateway := &plainOnlyGateway{reply: "fallback reply"}

	service := NewService(store, gateway)
	service.runner.config.MaxIterations = 4

	var chunks []string
	err := service.SendMessageStream(context.Background(), auth.Session{
		User: auth.User{ID: "user_1"},
	}, "conv_1", "weather forecast please calculate later", func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if !errors.Is(err, ErrStructuredGatewayRequired) {
		t.Fatalf("expected ErrStructuredGatewayRequired, got %v", err)
	}

	if len(chunks) != 0 {
		t.Fatalf("expected no streamed chunks on structured gateway failure, got %v", chunks)
	}
	if gateway.lastConfig.ModelID != "" {
		t.Fatalf("expected plain gateway not to be called, got config %+v", gateway.lastConfig)
	}
	if len(store.runs) != 1 {
		t.Fatalf("expected one failed run, got %d", len(store.runs))
	}
	run := store.runs[0]
	if run.Status != RunStatusFailed || run.CompletedAt == nil {
		t.Fatalf("expected failed completed run, got %+v", run)
	}
	if run.Error != ErrStructuredGatewayRequired.Error() {
		t.Fatalf("expected structured gateway error on run, got %q", run.Error)
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

func TestListAvailableToolsIncludesDefaultCommercialBuiltinCatalog(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:     "agent_catalog",
			UserID: "user_1",
			Tools:  []Tool{},
		},
	}
	service := NewService(store, &fakeGateway{})

	definitions, err := service.ListAvailableTools(context.Background(), auth.Session{
		User: auth.User{ID: "user_1"},
	}, "agent_catalog")
	if err != nil {
		t.Fatalf("ListAvailableTools returned error: %v", err)
	}

	byName := make(map[string]ToolDefinition, len(definitions))
	for _, definition := range definitions {
		byName[definition.Name] = definition
	}
	for _, builtin := range []string{"calculator", "datetime", "json_formatter", "text_transform"} {
		definition, ok := byName[builtin]
		if !ok {
			t.Fatalf("expected default builtin catalog tool %s, got %+v", builtin, definitions)
		}
		if definition.ToolType != "builtin" || definition.InputSchema == nil || definition.RiskLevel == "" {
			t.Fatalf("default builtin catalog definition missing metadata for %s: %+v", builtin, definition)
		}
	}
	for _, disabled := range []string{"web_search", "http_request"} {
		if _, ok := byName[disabled]; ok {
			t.Fatalf("disabled builtin %s should not be advertised without provider/policy, got %+v", disabled, definitions)
		}
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
	byName := make(map[string]ToolDefinition, len(definitions))
	for _, definition := range definitions {
		byName[definition.Name] = definition
	}
	customDefinition, ok := byName["crm_lookup"]
	if !ok {
		t.Fatalf("expected custom crm_lookup tool definition, got %+v", definitions)
	}
	if customDefinition.ToolType != "custom" || customDefinition.InputSchema == nil {
		t.Fatalf("custom available tool missing type/schema: %+v", customDefinition)
	}
	properties, ok := customDefinition.InputSchema.(map[string]any)["properties"].(map[string]any)
	if !ok || properties["customer_id"] == nil {
		t.Fatalf("custom input schema was not preserved: %+v", customDefinition.InputSchema)
	}
	if _, ok := byName["calculator"]; !ok {
		t.Fatalf("expected default builtin catalog to remain available with custom tools, got %+v", definitions)
	}
}

func TestListAvailableToolsAllowsWebSearchWhenProviderConfigured(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:     "agent_policy",
			UserID: "user_1",
			Tools: []Tool{
				{Name: "web_search", Type: "builtin", Enabled: true},
				{Name: "calculator", Type: "builtin", Enabled: true},
			},
		},
	}
	provider := fakeAgentWebSearchProvider{
		results: []mcp.WebSearchResult{{Title: "Search ready", URL: "https://search.example.test", Snippet: "configured"}},
	}
	service := newAuthorizedServiceForTest(t, store, &fakeGateway{}, func(options *ToolRuntimeOptions) {
		options.WebSearchProvider = provider
	})

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
	if _, ok := byName["web_search"]; !ok {
		t.Fatalf("expected web_search to be exposed with provider configured, got %+v", definitions)
	}
	if _, ok := byName["calculator"]; !ok {
		t.Fatalf("expected default builtin catalog to remain available with web_search provider, got %+v", definitions)
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

func TestServiceRuntimeOptionsPreserveWebSearchProviderContract(t *testing.T) {
	guard := &liveExecutorGuard{}
	guard.allow.Store(true)
	contract, profile := loadLiveExecutorAuthority(t)
	authorities, err := releasecontract.NewRuntimeAuthorities(contract, profile, guard)
	if err != nil {
		t.Fatalf("build runtime authorities: %v", err)
	}
	provider := &liveWebSearchProvider{}
	store := &fakeStore{agent: &Agent{
		ID: "agent_runtime_web_search", OrganizationID: "org_1", UserID: "user_1",
		Tools: []Tool{{Name: "web_search", Type: "builtin", Enabled: true}},
	}}
	service, err := NewServiceWithRuntimeOptions(store, &fakeGateway{}, nil, ToolRuntimeOptions{
		Authorities:       authorities,
		Guard:             guard,
		Effects:           &liveExecutorRegistrar{descriptors: make(map[string]releasecontract.EffectDescriptor)},
		HTTPClient:        http.DefaultClient,
		WebSearchProvider: provider,
	})
	if err != nil {
		t.Fatalf("construct runtime service: %v", err)
	}
	session := auth.Session{OrganizationID: "org_1", User: auth.User{ID: "user_1"}}

	guard.allow.Store(false)
	if _, err := service.ExecuteTool(t.Context(), session, store.agent.ID, "web_search", map[string]any{"query": "denied"}); err == nil {
		t.Fatal("denied runtime service web search unexpectedly succeeded")
	}
	if got := provider.calls.Load(); got != 0 {
		t.Fatalf("denied runtime service provider calls = %d, want 0", got)
	}

	guard.allow.Store(true)
	result, err := service.ExecuteTool(t.Context(), session, store.agent.ID, "web_search", map[string]any{"query": "current"})
	if err != nil {
		t.Fatalf("current runtime service web search: %v", err)
	}
	if result == nil || result.IsError {
		t.Fatalf("current runtime service result = %+v, want success", result)
	}
	if got := provider.calls.Load(); got != 1 {
		t.Fatalf("current runtime service provider calls = %d, want 1", got)
	}

	definitions, err := service.ListAvailableTools(t.Context(), session, store.agent.ID)
	if err != nil {
		t.Fatalf("list runtime service tools: %v", err)
	}
	for _, definition := range definitions {
		if definition.Name == "web_search" {
			return
		}
	}
	t.Fatalf("runtime service omitted web_search definition: %+v", definitions)
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
	provider := &liveWebSearchProvider{}
	executor := newAuthorizedToolExecutorForTest(t, nil, func(options *ToolRuntimeOptions) {
		options.WebSearchProvider = provider
	})
	executor.SetWebSearchProvider(nil)
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
	if provider.calls.Load() != 0 {
		t.Fatal("disabled commercial builtin was called before executor rejected it")
	}
	if result == nil || !result.IsError {
		t.Fatalf("Execute result = %+v, want disabled tool error result", result)
	}
	if !strings.Contains(strings.ToLower(result.Content), "disabled") {
		t.Fatalf("Execute result content = %q, want disabled message", result.Content)
	}
}

func TestExecuteToolRejectsLoremIpsumDemoBuiltinBeforeCallingTool(t *testing.T) {
	recording := &recordingBuiltinTool{name: "lorem_ipsum"}
	executor := newAuthorizedToolExecutorForTest(t, nil, func(options *ToolRuntimeOptions) {
		options.BuiltinTools = map[string]mcp.BuiltinTool{
			"lorem_ipsum": recording,
		}
	})
	agent := &Agent{
		ID: "agent_policy",
		Tools: []Tool{
			{Name: "lorem_ipsum", Type: "builtin", Enabled: true},
		},
	}

	result, err := executor.Execute(context.Background(), agent, &ToolCall{
		ID:        "call_lorem",
		Name:      "lorem_ipsum",
		Arguments: map[string]any{"paragraphs": float64(1)},
	})
	if !releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
		t.Fatalf("Execute error = %v, want %s", err, releasecontract.CodeCapabilityUnknown)
	}
	if recording.called {
		t.Fatal("demo-only lorem_ipsum builtin was called before executor rejected it")
	}
	if result != nil {
		t.Fatalf("Execute result = %+v, want no result before catalog authorization", result)
	}
}

func TestExecuteToolAllowsEnabledCommercialBuiltin(t *testing.T) {
	executor := newAuthorizedToolExecutorForTest(t, nil)
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
func TestServiceSendMessageStreamUsesDefaultPlanningMode(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_stream_plan",
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
			ID:             "conv_stream_plan",
			AgentID:        "agent_stream_plan",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
	}
	gateway := &fakeGateway{
		plainReply: "Plan:\n1. Inspect stream behavior\n2. Verify planning evidence",
		structured: []*chat.CompletionResponse{{
			Content:      "react path should not run",
			FinishReason: "stop",
		}},
	}
	service := NewService(store, gateway)

	var chunks []string
	err := service.SendMessageStream(context.Background(), auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
		User:           auth.User{ID: "user_1"},
	}, "conv_stream_plan", "make a streamed plan", func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("SendMessageStream returned error: %v", err)
	}
	if streamed := strings.Join(chunks, ""); !strings.Contains(streamed, "Verify planning evidence") {
		t.Fatalf("expected planning reply to be streamed, got chunks=%v", chunks)
	}
	if gateway.plainCalls != 1 || gateway.streamCalls != 0 || gateway.structuredCalls != 0 {
		t.Fatalf("expected stream default planning to use one planning reply and no ReAct/tool stream, got plain=%d stream=%d structured=%d", gateway.plainCalls, gateway.streamCalls, gateway.structuredCalls)
	}
	if len(store.runs) != 1 {
		t.Fatalf("expected one durable planning run, got %+v", store.runs)
	}
	run := store.runs[0]
	if run.Mode != ExecutionModePlanning || run.Status != RunStatusPendingApproval || run.FinalMessageID == "" || run.CompletedAt != nil {
		t.Fatalf("expected open planning run evidence, got %+v", run)
	}
	if len(store.toolRuns) != 0 {
		t.Fatalf("planning stream should not execute tools before plan-step approval, got %+v", store.toolRuns)
	}
	if len(store.planSteps) != 2 || store.planSteps[1].Title != "Verify planning evidence" {
		t.Fatalf("expected parsed planning steps from streamed planning reply, got %+v", store.planSteps)
	}
}

func TestServiceSendMessageStreamHonorsExplicitReactModeOverride(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_stream_react_override",
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
			ID:             "conv_stream_react_override",
			AgentID:        "agent_stream_react_override",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
	}
	gateway := &fakeGateway{
		plainReply: "planning path should not run",
		structured: []*chat.CompletionResponse{{
			Content:      "react streamed answer",
			FinishReason: "stop",
		}},
	}
	service := NewService(store, gateway)

	var chunks []string
	err := service.SendMessageStream(context.Background(), auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
		User:           auth.User{ID: "user_1"},
	}, "conv_stream_react_override", "use react now", func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	}, SendMessageOptions{Mode: ExecutionModeReact})
	if err != nil {
		t.Fatalf("SendMessageStream returned error: %v", err)
	}
	if streamed := strings.Join(chunks, ""); streamed != "react streamed answer" {
		t.Fatalf("expected ReAct streamed answer, got %q from chunks=%v", streamed, chunks)
	}
	if gateway.plainCalls != 0 || gateway.streamCalls != 0 || gateway.structuredCalls != 1 {
		t.Fatalf("expected explicit ReAct override to use structured ReAct path only, got plain=%d stream=%d structured=%d", gateway.plainCalls, gateway.streamCalls, gateway.structuredCalls)
	}
	if len(store.planSteps) != 0 {
		t.Fatalf("explicit ReAct override should not create planning steps, got %+v", store.planSteps)
	}
	if len(store.runs) != 1 {
		t.Fatalf("expected one ReAct run, got %+v", store.runs)
	}
	run := store.runs[0]
	if run.Mode != ExecutionModeReact || run.Status != RunStatusCompleted || run.FinalMessageID == "" || run.CompletedAt == nil {
		t.Fatalf("expected completed ReAct run evidence, got %+v", run)
	}
}

func TestServiceSendMessageStreamHonorsExplicitPlanningModeOverride(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_stream_planning_override",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
			Config: Config{
				DefaultExecutionMode: ExecutionModeReact,
			},
			Tools: []Tool{
				{Name: "datetime", Type: "builtin", Enabled: true},
			},
		},
		conversation: &Conversation{
			ID:             "conv_stream_planning_override",
			AgentID:        "agent_stream_planning_override",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
	}
	gateway := &fakeGateway{
		plainReply: "Plan:\n1. Draft streaming plan\n2. Wait for approval",
		structured: []*chat.CompletionResponse{{
			Content:      "react path should not run",
			FinishReason: "stop",
		}},
	}
	service := NewService(store, gateway)

	var chunks []string
	err := service.SendMessageStream(context.Background(), auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
		User:           auth.User{ID: "user_1"},
	}, "conv_stream_planning_override", "use planning now", func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	}, SendMessageOptions{Mode: ExecutionModePlanning})
	if err != nil {
		t.Fatalf("SendMessageStream returned error: %v", err)
	}
	if streamed := strings.Join(chunks, ""); !strings.Contains(streamed, "Wait for approval") {
		t.Fatalf("expected planning reply to be streamed, got %q from chunks=%v", streamed, chunks)
	}
	if gateway.plainCalls != 1 || gateway.streamCalls != 0 || gateway.structuredCalls != 0 {
		t.Fatalf("expected explicit planning override to use planning path only, got plain=%d stream=%d structured=%d", gateway.plainCalls, gateway.streamCalls, gateway.structuredCalls)
	}
	if len(store.toolRuns) != 0 {
		t.Fatalf("planning override should not execute tools before approval, got %+v", store.toolRuns)
	}
	if len(store.runs) != 1 {
		t.Fatalf("expected one planning run, got %+v", store.runs)
	}
	run := store.runs[0]
	if run.Mode != ExecutionModePlanning || run.Status != RunStatusPendingApproval || run.FinalMessageID == "" || run.CompletedAt != nil {
		t.Fatalf("expected open planning run evidence, got %+v", run)
	}
	if len(store.planSteps) != 2 || store.planSteps[1].Title != "Wait for approval" {
		t.Fatalf("expected parsed planning steps from explicit override, got %+v", store.planSteps)
	}
}

func TestServiceSendMessageStreamRejectsInvalidMode(t *testing.T) {
	store := &fakeStore{
		agent: &Agent{
			ID:             "agent_stream_invalid_mode",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
		},
		conversation: &Conversation{
			ID:             "conv_stream_invalid_mode",
			AgentID:        "agent_stream_invalid_mode",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
	}
	gateway := &fakeGateway{plainReply: "should not run"}
	service := NewService(store, gateway)

	err := service.SendMessageStream(context.Background(), auth.Session{
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
		User:           auth.User{ID: "user_1"},
	}, "conv_stream_invalid_mode", "hello", func(chunk string) error {
		t.Fatalf("unexpected streamed chunk for invalid mode: %q", chunk)
		return nil
	}, SendMessageOptions{Mode: "manual"})
	if err == nil || err.Error() != "mode must be react or planning" {
		t.Fatalf("expected invalid mode error, got %v", err)
	}
	if gateway.plainCalls != 0 || gateway.streamCalls != 0 || gateway.structuredCalls != 0 {
		t.Fatalf("invalid mode should not call gateway, got plain=%d stream=%d structured=%d", gateway.plainCalls, gateway.streamCalls, gateway.structuredCalls)
	}
	if len(store.messages) != 0 || len(store.runs) != 0 || len(store.planSteps) != 0 || len(store.toolRuns) != 0 {
		t.Fatalf("invalid mode should not persist run evidence, messages=%+v runs=%+v planSteps=%+v toolRuns=%+v", store.messages, store.runs, store.planSteps, store.toolRuns)
	}
}

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
	reply      string
	lastConfig chat.ConversationConfig
}

func (g *plainOnlyGateway) GenerateReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig) (string, error) {
	g.lastConfig = config
	return g.reply, nil
}

func (g *plainOnlyGateway) GenerateReplyStream(ctx context.Context, messages []chat.Message, config chat.ConversationConfig, onChunk func(string) error) error {
	g.lastConfig = config
	return onChunk(g.reply)
}
