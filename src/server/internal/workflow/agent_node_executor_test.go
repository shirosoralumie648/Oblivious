package workflow

import (
	"context"
	"errors"
	"testing"
)

func TestAgentNodeExecutorStartsAgentRunAndReturnsDurableSummary(t *testing.T) {
	runner := &recordingWorkflowAgentRunner{
		response: &WorkflowAgentRunResult{
			RunID:          "run_1",
			Status:         "completed",
			FinalMessageID: "msg_final",
			FinalMessage:   "Escalation drafted",
			Messages: []WorkflowAgentMessage{{
				ID:      "msg_final",
				Role:    "assistant",
				Content: "Escalation drafted",
			}},
			ToolRuns: []WorkflowAgentToolRun{{
				ID:             "tool_run_1",
				ToolName:       "datetime",
				Status:         "completed",
				ApprovalStatus: "not_required",
			}},
			PlanSteps: []WorkflowAgentPlanStep{{
				ID:     "step_1",
				Title:  "Draft response",
				Status: "completed",
			}},
		},
	}
	executor := NewAgentNodeExecutor(runner)
	output, err := executor.Execute(context.Background(), NodeExecutorInput{
		Execution: &WorkflowExecution{
			ID:             "wexec_1",
			OrganizationID: "org_1",
			Context: map[string]any{
				"userId":      "user_1",
				"workspaceId": "workspace_1",
				"requestId":   "req_1",
			},
		},
		Input: map[string]any{
			"agentId":        "agent_1",
			"conversationId": "conv_1",
			"input":          "Handle {{ticket}}",
			"mode":           "react",
			"maxIterations":  float64(4),
			"tokenBudget":    float64(12000),
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if runner.request.OrganizationID != "org_1" || runner.request.UserID != "user_1" || runner.request.WorkspaceID != "workspace_1" {
		t.Fatalf("unexpected runner session scope: %+v", runner.request)
	}
	if runner.request.AgentID != "agent_1" || runner.request.ConversationID != "conv_1" || runner.request.Input != "Handle {{ticket}}" {
		t.Fatalf("unexpected runner request: %+v", runner.request)
	}
	if runner.request.Mode != "react" || runner.request.MaxIterations == nil || *runner.request.MaxIterations != 4 || runner.request.TokenBudget == nil || *runner.request.TokenBudget != 12000 {
		t.Fatalf("unexpected run controls: %+v", runner.request)
	}
	if output["runId"] != "run_1" || output["status"] != "completed" || output["finalMessageId"] != "msg_final" {
		t.Fatalf("unexpected agent node run output: %#v", output)
	}
	if output["text"] != "Escalation drafted" || output["content"] != "Escalation drafted" {
		t.Fatalf("expected final message aliases in output, got %#v", output)
	}
	if toolRuns, ok := output["toolRuns"].([]map[string]any); !ok || len(toolRuns) != 1 || toolRuns[0]["toolName"] != "datetime" {
		t.Fatalf("expected tool run summary in output, got %#v", output["toolRuns"])
	}
	if planSteps, ok := output["planSteps"].([]map[string]any); !ok || len(planSteps) != 1 || planSteps[0]["title"] != "Draft response" {
		t.Fatalf("expected plan step summary in output, got %#v", output["planSteps"])
	}
}

func TestAgentNodeExecutorRequiresRunnerAndCoreInput(t *testing.T) {
	tests := []struct {
		name     string
		executor NodeExecutor
		input    map[string]any
	}{
		{
			name:     "missing runner",
			executor: NewAgentNodeExecutor(nil),
			input: map[string]any{
				"agentId":        "agent_1",
				"conversationId": "conv_1",
				"input":          "hello",
			},
		},
		{
			name:     "missing agent id",
			executor: NewAgentNodeExecutor(&recordingWorkflowAgentRunner{}),
			input: map[string]any{
				"conversationId": "conv_1",
				"input":          "hello",
			},
		},
		{
			name:     "missing conversation id",
			executor: NewAgentNodeExecutor(&recordingWorkflowAgentRunner{}),
			input: map[string]any{
				"agentId": "agent_1",
				"input":   "hello",
			},
		},
		{
			name:     "missing input",
			executor: NewAgentNodeExecutor(&recordingWorkflowAgentRunner{}),
			input: map[string]any{
				"agentId":        "agent_1",
				"conversationId": "conv_1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.executor.Execute(context.Background(), NodeExecutorInput{
				Execution: &WorkflowExecution{OrganizationID: "org_1", Context: map[string]any{"userId": "user_1"}},
				Input:     tt.input,
			})
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("Execute err=%v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestAgentNodeExecutorReturnsApprovalRequiredForPendingRun(t *testing.T) {
	executor := NewAgentNodeExecutor(&recordingWorkflowAgentRunner{
		response: &WorkflowAgentRunResult{
			RunID:  "run_pending",
			Status: "pending_approval",
			ToolRuns: []WorkflowAgentToolRun{{
				ID:             "tool_run_pending",
				ToolName:       "delete_file",
				Status:         "pending_approval",
				ApprovalStatus: "pending",
			}},
		},
	})

	output, err := executor.Execute(context.Background(), NodeExecutorInput{
		Execution: &WorkflowExecution{OrganizationID: "org_1", Context: map[string]any{"userId": "user_1"}},
		Input: map[string]any{
			"agentId":        "agent_1",
			"conversationId": "conv_1",
			"input":          "delete stale file",
		},
	})
	if !errors.Is(err, ErrWorkflowUserInputRequired) {
		t.Fatalf("Execute err=%v, want ErrWorkflowUserInputRequired", err)
	}
	if output["runId"] != "run_pending" || output["status"] != "pending_approval" {
		t.Fatalf("expected pending run output to be returned with approval error, got %#v", output)
	}
}

func TestAgentNodeExecutorReturnsResumeRequiredForTokenBudgetExceededRun(t *testing.T) {
	executor := NewAgentNodeExecutor(&recordingWorkflowAgentRunner{
		response: &WorkflowAgentRunResult{
			RunID:  "run_budget",
			Status: "token_budget_exceeded",
			PlanSteps: []WorkflowAgentPlanStep{{
				ID:     "step_budget",
				Title:  "Retry with larger budget",
				Status: "failed",
				Error:  "token_budget_exceeded: prompt exceeded budget",
			}},
		},
	})

	output, err := executor.Execute(context.Background(), NodeExecutorInput{
		Execution: &WorkflowExecution{OrganizationID: "org_1", Context: map[string]any{"userId": "user_1"}},
		Input: map[string]any{
			"agentId":        "agent_1",
			"conversationId": "conv_1",
			"input":          "continue large plan",
		},
	})
	if !errors.Is(err, ErrWorkflowUserInputRequired) {
		t.Fatalf("Execute err=%v, want ErrWorkflowUserInputRequired", err)
	}
	if output["runId"] != "run_budget" || output["status"] != "token_budget_exceeded" {
		t.Fatalf("expected token-budget run output to be returned with resume error, got %#v", output)
	}
}

func TestAgentNodeExecutorResumesPlanningControlActions(t *testing.T) {
	tests := []struct {
		name           string
		submitted      map[string]any
		wantAction     string
		wantPlanStepID string
		wantBudget     int
	}{
		{
			name:       "continue plan",
			submitted:  map[string]any{"action": "continue_plan"},
			wantAction: "continue_plan",
		},
		{
			name:       "adjust plan",
			submitted:  map[string]any{"action": "adjust_plan", "reason": "avoid risky command"},
			wantAction: "adjust_plan",
		},
		{
			name:       "continue budget",
			submitted:  map[string]any{"tokenBudget": float64(5000)},
			wantAction: "continue_budget",
			wantBudget: 5000,
		},
		{
			name:           "execute plan step",
			submitted:      map[string]any{"action": "execute_plan_step", "planStepId": "step_1"},
			wantAction:     "execute_plan_step",
			wantPlanStepID: "step_1",
		},
		{
			name:           "skip plan step",
			submitted:      map[string]any{"action": "skip", "plan_step_id": "step_2", "reason": "not needed"},
			wantAction:     "skip_plan_step",
			wantPlanStepID: "step_2",
		},
		{
			name:           "retry plan step",
			submitted:      map[string]any{"action": "retry", "planStepID": "step_3"},
			wantAction:     "retry_plan_step",
			wantPlanStepID: "step_3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := &recordingWorkflowAgentRunner{
				controlResponse: &WorkflowAgentRunResult{
					RunID:          "run_plan",
					Status:         "completed",
					FinalMessageID: "msg_final",
					FinalMessage:   "planning control completed",
					PlanSteps: []WorkflowAgentPlanStep{{
						ID:             "step_1",
						Title:          "Execute plan",
						Status:         "completed",
						ApprovalStatus: "approved",
						ToolName:       "shell",
						ResultContent:  "ok",
					}},
				},
			}
			executor := NewAgentNodeExecutor(runner)
			output, err := executor.ApproveToolRun(context.Background(), NodeExecutorInput{
				Execution: &WorkflowExecution{
					OrganizationID: "org_1",
					Context: map[string]any{
						"userId":      "user_1",
						"workspaceId": "workspace_1",
						"requestId":   "req_1",
					},
				},
				Input: map[string]any{
					"agentId":        "agent_1",
					"conversationId": "conv_1",
					"input":          "run plan",
				},
			}, WorkflowNodeExecution{
				Output: map[string]any{"runId": "run_plan"},
			}, tt.submitted)
			if err != nil {
				t.Fatalf("ApproveToolRun returned error: %v", err)
			}
			if runner.controlAction != tt.wantAction {
				t.Fatalf("expected control action %q, got %q", tt.wantAction, runner.controlAction)
			}
			if runner.controlRequest.RunID != "run_plan" || runner.controlRequest.UserID != "user_1" || runner.controlRequest.WorkspaceID != "workspace_1" {
				t.Fatalf("unexpected control request scope: %+v", runner.controlRequest)
			}
			if runner.controlRequest.PlanStepID != tt.wantPlanStepID {
				t.Fatalf("expected plan step %q, got %+v", tt.wantPlanStepID, runner.controlRequest)
			}
			if runner.controlRequest.TokenBudget != tt.wantBudget {
				t.Fatalf("expected budget %d, got %+v", tt.wantBudget, runner.controlRequest)
			}
			if output["text"] != "planning control completed" || output["status"] != "completed" {
				t.Fatalf("unexpected planning control output: %#v", output)
			}
			steps, ok := output["planSteps"].([]map[string]any)
			if !ok || len(steps) != 1 || steps[0]["approvalStatus"] != "approved" || steps[0]["toolName"] != "shell" || steps[0]["resultContent"] != "ok" {
				t.Fatalf("expected enriched plan-step output, got %#v", output["planSteps"])
			}
		})
	}
}

type recordingWorkflowAgentRunner struct {
	request          WorkflowAgentRunRequest
	approvalRequest  WorkflowAgentApprovalRequest
	controlRequest   WorkflowAgentControlRequest
	controlAction    string
	response         *WorkflowAgentRunResult
	approvalResponse *WorkflowAgentRunResult
	controlResponse  *WorkflowAgentRunResult
	err              error
}

func (r *recordingWorkflowAgentRunner) StartAgentRun(ctx context.Context, req WorkflowAgentRunRequest) (*WorkflowAgentRunResult, error) {
	_ = ctx
	r.request = req
	if r.err != nil {
		return nil, r.err
	}
	return r.response, nil
}

func (r *recordingWorkflowAgentRunner) ApproveAgentToolRun(ctx context.Context, req WorkflowAgentApprovalRequest) (*WorkflowAgentRunResult, error) {
	_ = ctx
	r.approvalRequest = req
	if r.err != nil {
		return nil, r.err
	}
	return r.approvalResponse, nil
}

func (r *recordingWorkflowAgentRunner) ContinueAgentPlan(ctx context.Context, req WorkflowAgentControlRequest) (*WorkflowAgentRunResult, error) {
	_ = ctx
	r.controlAction = "continue_plan"
	r.controlRequest = req
	return r.controlResponse, r.err
}

func (r *recordingWorkflowAgentRunner) AdjustAgentPlan(ctx context.Context, req WorkflowAgentControlRequest) (*WorkflowAgentRunResult, error) {
	_ = ctx
	r.controlAction = "adjust_plan"
	r.controlRequest = req
	return r.controlResponse, r.err
}

func (r *recordingWorkflowAgentRunner) ContinueAgentRunWithTokenBudget(ctx context.Context, req WorkflowAgentControlRequest) (*WorkflowAgentRunResult, error) {
	_ = ctx
	r.controlAction = "continue_budget"
	r.controlRequest = req
	return r.controlResponse, r.err
}

func (r *recordingWorkflowAgentRunner) ApproveAgentPlanStep(ctx context.Context, req WorkflowAgentControlRequest) (*WorkflowAgentRunResult, error) {
	_ = ctx
	r.controlAction = "approve_plan_step"
	r.controlRequest = req
	return r.controlResponse, r.err
}

func (r *recordingWorkflowAgentRunner) ExecuteAgentPlanStep(ctx context.Context, req WorkflowAgentControlRequest) (*WorkflowAgentRunResult, error) {
	_ = ctx
	r.controlAction = "execute_plan_step"
	r.controlRequest = req
	return r.controlResponse, r.err
}

func (r *recordingWorkflowAgentRunner) SkipAgentPlanStep(ctx context.Context, req WorkflowAgentControlRequest) (*WorkflowAgentRunResult, error) {
	_ = ctx
	r.controlAction = "skip_plan_step"
	r.controlRequest = req
	return r.controlResponse, r.err
}

func (r *recordingWorkflowAgentRunner) RetryAgentPlanStep(ctx context.Context, req WorkflowAgentControlRequest) (*WorkflowAgentRunResult, error) {
	_ = ctx
	r.controlAction = "retry_plan_step"
	r.controlRequest = req
	return r.controlResponse, r.err
}
