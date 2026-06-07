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

type recordingWorkflowAgentRunner struct {
	request          WorkflowAgentRunRequest
	approvalRequest  WorkflowAgentApprovalRequest
	response         *WorkflowAgentRunResult
	approvalResponse *WorkflowAgentRunResult
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
