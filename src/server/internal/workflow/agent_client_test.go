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

func TestAgentClientPlanningControlsUseWorkspaceSessionAndReloadRun(t *testing.T) {
	result := &agent.RunWithMessages{
		Run: &agent.Run{
			ID:             "run_1",
			Status:         agent.RunStatusPendingApproval,
			FinalMessageID: "msg_plan",
		},
		Messages: []*agent.Message{{
			ID:      "msg_plan",
			Role:    "assistant",
			Content: "plan state refreshed",
		}},
		PlanSteps: []*agent.PlanStep{{
			ID:             "step_1",
			RunID:          "run_1",
			Title:          "Verify boundary",
			Status:         agent.PlanStepStatusCompleted,
			ApprovalStatus: agent.ApprovalStatusApproved,
		}},
	}
	req := WorkflowAgentControlRequest{
		OrganizationID: "org_1",
		UserID:         "user_1",
		WorkspaceID:    "workspace_1",
		RunID:          "run_1",
		PlanStepID:     "step_1",
		Reason:         "operator reason",
		TokenBudget:    4096,
	}

	tests := []struct {
		name                  string
		call                  func(*AgentClient, context.Context, WorkflowAgentControlRequest) (*WorkflowAgentRunResult, error)
		wantControlAction     string
		wantPlanStepAction    string
		wantReloadAfterCall   bool
		wantReasonForwarded   bool
		wantBudgetForwarded   bool
		wantPlanStepForwarded bool
	}{
		{
			name:              "continue plan",
			call:              (*AgentClient).ContinueAgentPlan,
			wantControlAction: "continue-plan",
		},
		{
			name:                "adjust plan",
			call:                (*AgentClient).AdjustAgentPlan,
			wantControlAction:   "adjust-plan",
			wantReasonForwarded: true,
		},
		{
			name:                "continue token budget",
			call:                (*AgentClient).ContinueAgentRunWithTokenBudget,
			wantControlAction:   "continue-budget",
			wantReloadAfterCall: true,
			wantBudgetForwarded: true,
		},
		{
			name:                  "approve plan step",
			call:                  (*AgentClient).ApproveAgentPlanStep,
			wantPlanStepAction:    "approve",
			wantReloadAfterCall:   true,
			wantReasonForwarded:   true,
			wantPlanStepForwarded: true,
		},
		{
			name:                  "execute plan step",
			call:                  (*AgentClient).ExecuteAgentPlanStep,
			wantPlanStepAction:    "execute",
			wantReloadAfterCall:   true,
			wantPlanStepForwarded: true,
		},
		{
			name:                  "skip plan step",
			call:                  (*AgentClient).SkipAgentPlanStep,
			wantPlanStepAction:    "skip",
			wantReloadAfterCall:   true,
			wantReasonForwarded:   true,
			wantPlanStepForwarded: true,
		},
		{
			name:                  "retry plan step",
			call:                  (*AgentClient).RetryAgentPlanStep,
			wantPlanStepAction:    "retry",
			wantReloadAfterCall:   true,
			wantPlanStepForwarded: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &recordingAgentClientService{
				result:        result,
				planStepRunID: "run_1",
			}
			client := &AgentClient{agentService: service}

			got, err := tt.call(client, context.Background(), req)
			if err != nil {
				t.Fatalf("%s returned error: %v", tt.name, err)
			}
			if got.RunID != "run_1" || got.FinalMessage != "plan state refreshed" || len(got.PlanSteps) != 1 || got.PlanSteps[0].ID != "step_1" {
				t.Fatalf("expected refreshed run detail with plan step evidence, got %+v", got)
			}
			if tt.wantControlAction != "" {
				if service.controlAction != tt.wantControlAction {
					t.Fatalf("control action = %q, want %q", service.controlAction, tt.wantControlAction)
				}
				if service.controlRunID != "run_1" {
					t.Fatalf("control run ID = %q, want run_1", service.controlRunID)
				}
				if service.controlSession.OrganizationID != "org_1" || service.controlSession.WorkspaceID != "workspace_1" || service.controlSession.User.ID != "user_1" {
					t.Fatalf("unexpected control session: %+v", service.controlSession)
				}
			}
			if tt.wantPlanStepAction != "" {
				if service.planStepAction != tt.wantPlanStepAction {
					t.Fatalf("plan-step action = %q, want %q", service.planStepAction, tt.wantPlanStepAction)
				}
				if service.planStepSession.OrganizationID != "org_1" || service.planStepSession.WorkspaceID != "workspace_1" || service.planStepSession.User.ID != "user_1" {
					t.Fatalf("unexpected plan-step session: %+v", service.planStepSession)
				}
			}
			if tt.wantReasonForwarded && service.controlReason != "operator reason" && service.planStepReason != "operator reason" {
				t.Fatalf("expected reason to be forwarded, got control=%q planStep=%q", service.controlReason, service.planStepReason)
			}
			if tt.wantBudgetForwarded && service.controlTokenBudget != 4096 {
				t.Fatalf("token budget = %d, want 4096", service.controlTokenBudget)
			}
			if tt.wantPlanStepForwarded && service.planStepID != "step_1" {
				t.Fatalf("plan step ID = %q, want step_1", service.planStepID)
			}
			if tt.wantReloadAfterCall {
				if service.fetchedRunID != "run_1" {
					t.Fatalf("expected run reload for run_1, got %q", service.fetchedRunID)
				}
				if service.fetchSession.OrganizationID != "org_1" || service.fetchSession.WorkspaceID != "workspace_1" || service.fetchSession.User.ID != "user_1" {
					t.Fatalf("unexpected reload session: %+v", service.fetchSession)
				}
			}
		})
	}
}

func TestAgentClientPlanStepActionRejectsCrossRunResult(t *testing.T) {
	service := &recordingAgentClientService{
		result:        &agent.RunWithMessages{Run: &agent.Run{ID: "run_1", Status: agent.RunStatusPendingApproval}},
		planStepRunID: "run_other",
	}
	client := &AgentClient{agentService: service}

	_, err := client.ExecuteAgentPlanStep(context.Background(), WorkflowAgentControlRequest{
		OrganizationID: "org_1",
		UserID:         "user_1",
		WorkspaceID:    "workspace_1",
		RunID:          "run_1",
		PlanStepID:     "step_1",
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for cross-run plan-step action result, got %v", err)
	}
	if service.fetchedRunID != "" {
		t.Fatalf("cross-run plan-step result should not reload run detail, got %q", service.fetchedRunID)
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
	startAction        string
	startSession       auth.Session
	startRequest       agent.StartRunRequest
	approveSession     auth.Session
	approvedToolRunID  string
	approvalReason     string
	controlAction      string
	controlSession     auth.Session
	controlRunID       string
	controlReason      string
	controlTokenBudget int
	planStepAction     string
	planStepSession    auth.Session
	planStepID         string
	planStepReason     string
	planStepRunID      string
	fetchSession       auth.Session
	fetchedRunID       string
	result             *agent.RunWithMessages
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
	s.controlAction = "continue-plan"
	s.controlSession = session
	s.controlRunID = runID
	return s.result, nil
}

func (s *recordingAgentClientService) AdjustPlanSteps(ctx context.Context, session auth.Session, runID, reason string) (*agent.RunWithMessages, error) {
	_ = ctx
	s.controlAction = "adjust-plan"
	s.controlSession = session
	s.controlRunID = runID
	s.controlReason = reason
	return s.result, nil
}

func (s *recordingAgentClientService) ContinueRunWithTokenBudget(ctx context.Context, session auth.Session, runID string, tokenBudget int) (*agent.RunResult, error) {
	_ = ctx
	s.controlAction = "continue-budget"
	s.controlSession = session
	s.controlRunID = runID
	s.controlTokenBudget = tokenBudget
	return &agent.RunResult{}, nil
}

func (s *recordingAgentClientService) ApprovePlanStep(ctx context.Context, session auth.Session, planStepID, reason string) (*agent.PlanStep, error) {
	_ = ctx
	s.planStepAction = "approve"
	s.planStepSession = session
	s.planStepID = planStepID
	s.planStepReason = reason
	return &agent.PlanStep{ID: planStepID, RunID: s.planStepRunID}, nil
}

func (s *recordingAgentClientService) ExecutePlanStep(ctx context.Context, session auth.Session, planStepID string) (*agent.PlanStep, error) {
	_ = ctx
	s.planStepAction = "execute"
	s.planStepSession = session
	s.planStepID = planStepID
	return &agent.PlanStep{ID: planStepID, RunID: s.planStepRunID}, nil
}

func (s *recordingAgentClientService) SkipPlanStep(ctx context.Context, session auth.Session, planStepID, reason string) (*agent.PlanStep, error) {
	_ = ctx
	s.planStepAction = "skip"
	s.planStepSession = session
	s.planStepID = planStepID
	s.planStepReason = reason
	return &agent.PlanStep{ID: planStepID, RunID: s.planStepRunID}, nil
}

func (s *recordingAgentClientService) RetryPlanStep(ctx context.Context, session auth.Session, planStepID string) (*agent.PlanStep, error) {
	_ = ctx
	s.planStepAction = "retry"
	s.planStepSession = session
	s.planStepID = planStepID
	return &agent.PlanStep{ID: planStepID, RunID: s.planStepRunID}, nil
}
