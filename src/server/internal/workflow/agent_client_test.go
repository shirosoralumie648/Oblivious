package workflow

import (
	"context"
	"errors"
	"testing"

	"oblivious/server/internal/agent"
	"oblivious/server/internal/auth"
)

func TestAgentClientStartAgentRunUsesPlanningModeAndWorkspaceSession(t *testing.T) {
	service := &recordingAgentClientService{
		result: &agent.RunWithMessages{
			Run: &agent.Run{
				ID:             "run_planning",
				Status:         agent.RunStatusPendingApproval,
				FinalMessageID: "msg_plan",
			},
			Messages: []*agent.Message{{
				ID:      "msg_plan",
				Role:    "assistant",
				Content: "1. Inspect\n2. Verify",
			}},
			PlanSteps: []*agent.PlanStep{{
				ID:             "step_1",
				Title:          "Inspect",
				Status:         agent.PlanStepStatusPending,
				ApprovalStatus: agent.ApprovalStatusNotRequired,
			}},
		},
	}
	client := &AgentClient{agentService: service}

	result, err := client.StartAgentRun(context.Background(), WorkflowAgentRunRequest{
		OrganizationID: "org_1",
		UserID:         "user_1",
		WorkspaceID:    "workspace_1",
		AgentID:        "agent_1",
		ConversationID: "conv_1",
		Input:          "draft a plan",
		Mode:           " PLANNING ",
	})
	if err != nil {
		t.Fatalf("StartAgentRun returned error: %v", err)
	}
	if service.startAction != "planning" {
		t.Fatalf("expected planning start path, got %q", service.startAction)
	}
	if service.startSession.OrganizationID != "org_1" || service.startSession.WorkspaceID != "workspace_1" || service.startSession.User.ID != "user_1" {
		t.Fatalf("unexpected start session: %+v", service.startSession)
	}
	if service.startRequest.AgentID != "agent_1" || service.startRequest.ConversationID != "conv_1" || service.startRequest.Input != "draft a plan" {
		t.Fatalf("unexpected start request: %+v", service.startRequest)
	}
	if result.RunID != "run_planning" || result.Status != agent.RunStatusPendingApproval || result.FinalMessage != "1. Inspect\n2. Verify" {
		t.Fatalf("unexpected planning result: %+v", result)
	}
	if len(result.PlanSteps) != 1 || result.PlanSteps[0].ID != "step_1" {
		t.Fatalf("expected planning step evidence in result, got %+v", result.PlanSteps)
	}
}

func TestAgentClientStartAgentRunDefaultsToReactPath(t *testing.T) {
	service := &recordingAgentClientService{
		result: &agent.RunWithMessages{
			Run: &agent.Run{
				ID:             "run_react",
				Status:         agent.RunStatusCompleted,
				FinalMessageID: "msg_react",
			},
			Messages: []*agent.Message{{
				ID:      "msg_react",
				Role:    "assistant",
				Content: "done",
			}},
		},
	}
	client := &AgentClient{agentService: service}

	result, err := client.StartAgentRun(context.Background(), WorkflowAgentRunRequest{
		OrganizationID: "org_1",
		UserID:         "user_1",
		WorkspaceID:    "workspace_1",
		AgentID:        "agent_1",
		ConversationID: "conv_1",
		Input:          "run now",
		Mode:           "manual",
	})
	if err != nil {
		t.Fatalf("StartAgentRun returned error: %v", err)
	}
	if service.startAction != "react" {
		t.Fatalf("expected invalid mode to normalize to react path, got %q", service.startAction)
	}
	if result.RunID != "run_react" || result.FinalMessage != "done" {
		t.Fatalf("unexpected react result: %+v", result)
	}
}

func TestAgentClientApproveAgentToolRunUsesWorkspaceSession(t *testing.T) {
	service := &recordingAgentClientService{
		result: &agent.RunWithMessages{
			Run: &agent.Run{ID: "run_1", Status: agent.RunStatusCompleted},
		},
	}
	client := &AgentClient{agentService: service}

	if _, err := client.ApproveAgentToolRun(context.Background(), WorkflowAgentApprovalRequest{
		OrganizationID: "org_1",
		UserID:         "user_1",
		WorkspaceID:    "workspace_1",
		RunID:          "run_1",
		ToolRunID:      "tool_run_1",
		Reason:         "operator approved",
	}); err != nil {
		t.Fatalf("ApproveAgentToolRun returned error: %v", err)
	}
	if service.approveSession.OrganizationID != "org_1" || service.approveSession.WorkspaceID != "workspace_1" || service.approveSession.User.ID != "user_1" {
		t.Fatalf("unexpected approve session: %+v", service.approveSession)
	}
	if service.approvedToolRunID != "tool_run_1" || service.approvalReason != "operator approved" {
		t.Fatalf("unexpected approval call toolRun=%q reason=%q", service.approvedToolRunID, service.approvalReason)
	}
	if service.fetchedRunID != "run_1" {
		t.Fatalf("expected approved run reload, got %q", service.fetchedRunID)
	}
	if service.fetchSession.WorkspaceID != "workspace_1" {
		t.Fatalf("expected reload to preserve workspace session, got %+v", service.fetchSession)
	}
}

func TestAgentClientRunResultMappingIsNilSafeAndFallsBackToAssistantMessage(t *testing.T) {
	result := toWorkflowAgentRunResult(&agent.RunWithMessages{
		Messages: []*agent.Message{
			nil,
			{ID: "msg_user", Role: "user", Content: "hello"},
			{ID: "msg_assistant_1", Role: "assistant", Content: "first"},
			{ID: "msg_assistant_2", Role: "assistant", Content: "last"},
		},
		ToolRuns: []*agent.ToolRun{
			nil,
			{ID: "tool_run_1", ToolName: "shell", Status: agent.ToolRunStatusCompleted, ApprovalStatus: agent.ApprovalStatusApproved},
		},
		PlanSteps: []*agent.PlanStep{
			nil,
			{ID: "step_1", Title: "Verify", Status: agent.PlanStepStatusCompleted, ApprovalStatus: agent.ApprovalStatusNotRequired, ResultContent: "ok"},
		},
	})
	if result.RunID != "" || result.Status != "" || result.FinalMessageID != "" {
		t.Fatalf("run metadata should stay empty when Agent result has nil run, got %+v", result)
	}
	if result.FinalMessage != "last" {
		t.Fatalf("expected fallback to last assistant message, got %q", result.FinalMessage)
	}
	if len(result.Messages) != 3 || len(result.ToolRuns) != 1 || len(result.PlanSteps) != 1 {
		t.Fatalf("expected nil-safe payload mapping, got messages=%+v toolRuns=%+v planSteps=%+v", result.Messages, result.ToolRuns, result.PlanSteps)
	}

	result = toWorkflowAgentRunResult(&agent.RunWithMessages{
		Run: &agent.Run{ID: "run_1", Status: agent.RunStatusCompleted, FinalMessageID: "msg_assistant_1"},
		Messages: []*agent.Message{
			{ID: "msg_assistant_1", Role: "assistant", Content: "selected"},
			{ID: "msg_assistant_2", Role: "assistant", Content: "newer"},
		},
	})
	if result.FinalMessageID != "msg_assistant_1" || result.FinalMessage != "selected" {
		t.Fatalf("expected explicit final message ID to win, got %+v", result)
	}
}

func TestAgentClientNilServiceReturnsInvalidInput(t *testing.T) {
	_, err := NewAgentClient(nil).StartAgentRun(context.Background(), WorkflowAgentRunRequest{})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for nil agent service, got %v", err)
	}
}

type recordingAgentClientService struct {
	startAction       string
	startSession      auth.Session
	startRequest      agent.StartRunRequest
	approveSession    auth.Session
	approvedToolRunID string
	approvalReason    string
	fetchSession      auth.Session
	fetchedRunID      string
	result            *agent.RunWithMessages
}

func (s *recordingAgentClientService) StartRun(ctx context.Context, session auth.Session, req agent.StartRunRequest) (*agent.RunWithMessages, error) {
	_ = ctx
	s.startAction = "react"
	s.startSession = session
	s.startRequest = req
	return s.result, nil
}

func (s *recordingAgentClientService) StartPlanningRun(ctx context.Context, session auth.Session, req agent.StartRunRequest) (*agent.RunWithMessages, error) {
	_ = ctx
	s.startAction = "planning"
	s.startSession = session
	s.startRequest = req
	return s.result, nil
}

func (s *recordingAgentClientService) ApproveToolRun(ctx context.Context, session auth.Session, toolRunID, reason string) (*agent.ToolRun, error) {
	_ = ctx
	s.approveSession = session
	s.approvedToolRunID = toolRunID
	s.approvalReason = reason
	return &agent.ToolRun{ID: toolRunID, Status: agent.ToolRunStatusCompleted}, nil
}

func (s *recordingAgentClientService) GetRunWithMessages(ctx context.Context, session auth.Session, runID string) (*agent.RunWithMessages, error) {
	_ = ctx
	s.fetchSession = session
	s.fetchedRunID = runID
	return s.result, nil
}

func (s *recordingAgentClientService) ContinuePlanningRun(ctx context.Context, session auth.Session, runID string) (*agent.RunWithMessages, error) {
	_ = ctx
	s.fetchSession = session
	s.fetchedRunID = runID
	return s.result, nil
}

func (s *recordingAgentClientService) AdjustPlanSteps(ctx context.Context, session auth.Session, runID, reason string) (*agent.RunWithMessages, error) {
	_ = ctx
	s.fetchSession = session
	s.fetchedRunID = runID
	s.approvalReason = reason
	return s.result, nil
}

func (s *recordingAgentClientService) ContinueRunWithTokenBudget(ctx context.Context, session auth.Session, runID string, tokenBudget int) (*agent.RunResult, error) {
	_ = ctx
	s.fetchSession = session
	s.fetchedRunID = runID
	return &agent.RunResult{}, nil
}

func (s *recordingAgentClientService) ApprovePlanStep(ctx context.Context, session auth.Session, planStepID, reason string) (*agent.PlanStep, error) {
	_ = ctx
	s.fetchSession = session
	s.approvalReason = reason
	return &agent.PlanStep{ID: planStepID, RunID: s.fetchedRunID}, nil
}

func (s *recordingAgentClientService) ExecutePlanStep(ctx context.Context, session auth.Session, planStepID string) (*agent.PlanStep, error) {
	_ = ctx
	s.fetchSession = session
	return &agent.PlanStep{ID: planStepID, RunID: s.fetchedRunID}, nil
}

func (s *recordingAgentClientService) SkipPlanStep(ctx context.Context, session auth.Session, planStepID, reason string) (*agent.PlanStep, error) {
	_ = ctx
	s.fetchSession = session
	s.approvalReason = reason
	return &agent.PlanStep{ID: planStepID, RunID: s.fetchedRunID}, nil
}

func (s *recordingAgentClientService) RetryPlanStep(ctx context.Context, session auth.Session, planStepID string) (*agent.PlanStep, error) {
	_ = ctx
	s.fetchSession = session
	return &agent.PlanStep{ID: planStepID, RunID: s.fetchedRunID}, nil
}
