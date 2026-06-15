package workflow

import (
	"context"
	"errors"
	"strings"
	"testing"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/knowledge"
	"oblivious/server/internal/metrics"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestServiceRunReadyNodeInterpolatesWorkflowInputAndPriorNodeOutput(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		StaticNodeExecutor("start", map[string]any{"ticket": "INC-7"}),
		EchoNodeExecutor("http"),
	)))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Escalation Flow",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{"id": "start", "type": "start"},
				{"id": "notify", "type": "http", "input": map[string]any{
					"url":  "https://tickets.example/{{nodes.start.output.ticket}}",
					"body": "{{workflow.name}}: {{input.customer}}",
				}},
			},
			[]map[string]any{{"from": "start", "to": "notify"}},
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		Input:          map[string]any{"customer": "Acme"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	if err := service.RunReadyNode(ctx, "org_1", execution.ID, "start"); err != nil {
		t.Fatalf("RunReadyNode start returned error: %v", err)
	}
	if err := service.RunReadyNode(ctx, "org_1", execution.ID, "notify"); err != nil {
		t.Fatalf("RunReadyNode notify returned error: %v", err)
	}

	updated, err := service.GetExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution returned error: %v", err)
	}
	notifyNodes := workflowNodeExecutionsByID(updated.NodeExecutions, "notify")
	if len(notifyNodes) != 2 {
		t.Fatalf("expected seeded and completed notify nodes, got %+v", notifyNodes)
	}
	completed := notifyNodes[len(notifyNodes)-1]
	if completed.Status != NodeStatusSucceeded {
		t.Fatalf("expected notify node to succeed, got %+v", completed)
	}
	if completed.Input["url"] != "https://tickets.example/INC-7" {
		t.Fatalf("expected interpolated node output in URL, got %#v", completed.Input)
	}
	if completed.Input["body"] != "Escalation Flow: Acme" {
		t.Fatalf("expected workflow/input interpolation in body, got %#v", completed.Input)
	}
	if completed.Output["url"] != "https://tickets.example/INC-7" || completed.Output["body"] != "Escalation Flow: Acme" {
		t.Fatalf("expected echo executor output to receive interpolated input, got %#v", completed.Output)
	}
}

func TestServiceRunReadyNodeInterpolatesSystemExecutionAndOrganizationVariables(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		EchoNodeExecutor("agent"),
	)))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "System Variable Flow",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{"id": "notify", "type": "agent", "input": map[string]any{
					"executionId": "{{execution.id}}",
					"startedAt":   "{{execution.started_at}}",
					"workflow":    "{{workflow.name}}",
					"orgId":       "{{org.id}}",
				}},
			},
			[]map[string]any{},
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	if err := service.RunReadyNode(ctx, "org_1", execution.ID, "notify"); err != nil {
		t.Fatalf("RunReadyNode notify returned error: %v", err)
	}

	updated, err := service.GetExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution returned error: %v", err)
	}
	nodes := workflowNodeExecutionsByID(updated.NodeExecutions, "notify")
	if len(nodes) != 2 {
		t.Fatalf("expected seeded and completed notify nodes, got %+v", nodes)
	}
	completed := nodes[len(nodes)-1]
	if completed.Input["executionId"] != execution.ID {
		t.Fatalf("expected execution.id interpolation %q, got %#v", execution.ID, completed.Input)
	}
	if completed.Input["startedAt"] == "" {
		t.Fatalf("expected execution.started_at interpolation, got %#v", completed.Input)
	}
	if completed.Input["workflow"] != "System Variable Flow" || completed.Input["orgId"] != "org_1" {
		t.Fatalf("expected workflow/org system variables, got %#v", completed.Input)
	}
}

func TestServiceRunReadyNodeInterpolatesUserVariableFromExecutionContextAndTriggerPayload(t *testing.T) {
	tests := []struct {
		name           string
		contextValue   map[string]any
		triggerPayload map[string]any
		wantUserID     string
	}{
		{
			name:         "top level execution context userId",
			contextValue: map[string]any{"userId": "user_context"},
			wantUserID:   "user_context",
		},
		{
			name:           "nested trigger payload user id",
			triggerPayload: map[string]any{"user": map[string]any{"id": "user_trigger"}},
			wantUserID:     "user_trigger",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMemoryWorkflowStore()
			service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
				EchoNodeExecutor("agent"),
			)))
			ctx := context.Background()
			workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
				OrganizationID: "org_1",
				Name:           "User Variable Flow",
				Status:         WorkflowStatusPublished,
				Definition: workflowDefinitionDAG(
					[]map[string]any{
						{"id": "notify", "type": "agent", "input": map[string]any{
							"userId": "{{user.id}}",
						}},
					},
					[]map[string]any{},
				),
			})
			if err != nil {
				t.Fatalf("CreateWorkflow returned error: %v", err)
			}
			execution, err := service.StartExecution(ctx, StartExecutionRequest{
				OrganizationID: "org_1",
				WorkflowID:     workflow.ID,
				TriggerType:    WorkflowTriggerConversation,
				TriggerPayload: tt.triggerPayload,
				Context:        tt.contextValue,
			})
			if err != nil {
				t.Fatalf("StartExecution returned error: %v", err)
			}

			if err := service.RunReadyNode(ctx, "org_1", execution.ID, "notify"); err != nil {
				t.Fatalf("RunReadyNode notify returned error: %v", err)
			}

			updated, err := service.GetExecution(ctx, "org_1", execution.ID)
			if err != nil {
				t.Fatalf("GetExecution returned error: %v", err)
			}
			nodes := workflowNodeExecutionsByID(updated.NodeExecutions, "notify")
			if len(nodes) != 2 {
				t.Fatalf("expected seeded and completed notify nodes, got %+v", nodes)
			}
			completed := nodes[len(nodes)-1]
			if completed.Input["userId"] != tt.wantUserID {
				t.Fatalf("expected user.id interpolation %q, got %#v", tt.wantUserID, completed.Input)
			}
		})
	}
}

func TestServiceRunReadyNodeInterpolatesCurrentNodeLocalVariablesOnly(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		EchoNodeExecutor("agent"),
	)))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Node Local Variable Flow",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{"id": "prepare", "type": "agent", "variables": map[string]any{
					"summary": "{{input.ticket}}:{{workflow.name}}",
					"payload": map[string]any{"ticket": "{{input.ticket}}"},
				}, "input": map[string]any{
					"summary": "{{node.prepare.summary}}",
					"ticket":  "{{node.prepare.payload.ticket}}",
				}},
				{"id": "notify", "type": "agent", "input": map[string]any{
					"leaked": "{{node.prepare.summary}}",
				}},
			},
			[]map[string]any{{"from": "prepare", "to": "notify"}},
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		Input:          map[string]any{"ticket": "INC-42"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	if err := service.RunReadyNode(ctx, "org_1", execution.ID, "prepare"); err != nil {
		t.Fatalf("RunReadyNode prepare returned error: %v", err)
	}

	updated, err := service.GetExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution returned error: %v", err)
	}
	prepareNodes := workflowNodeExecutionsByID(updated.NodeExecutions, "prepare")
	if len(prepareNodes) != 2 {
		t.Fatalf("expected seeded and completed prepare nodes, got %+v", prepareNodes)
	}
	completed := prepareNodes[len(prepareNodes)-1]
	if completed.Input["summary"] != "INC-42:Node Local Variable Flow" || completed.Input["ticket"] != "INC-42" {
		t.Fatalf("expected current node-local variables to interpolate, got %#v", completed.Input)
	}

	err = service.RunReadyNode(ctx, "org_1", execution.ID, "notify")
	if err == nil {
		t.Fatalf("expected downstream node-local variable reference to fail")
	}
	if !errors.Is(err, ErrWorkflowVariableNotFound) || !strings.Contains(err.Error(), "node.prepare.summary") {
		t.Fatalf("expected node-local scope error, got %v", err)
	}
}

func TestServiceRunExecutionUntilBlockedAdvancesReadyDAGNodesToSuccess(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		StaticNodeExecutor("start", map[string]any{"ticket": "INC-11"}),
		EchoNodeExecutor("agent"),
		EchoNodeExecutor("end"),
	)))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Auto DAG",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{"id": "start", "type": "start"},
				{"id": "enrich", "type": "agent", "input": map[string]any{"ticket": "{{nodes.start.output.ticket}}"}},
				{"id": "notify", "type": "end", "input": map[string]any{"ticket": "{{nodes.enrich.output.ticket}}"}},
			},
			[]map[string]any{
				{"from": "start", "to": "enrich"},
				{"from": "enrich", "to": "notify"},
			},
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{OrganizationID: "org_1", WorkflowID: workflow.ID})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	completed, err := service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("RunExecutionUntilBlocked returned error: %v", err)
	}
	if completed.Status != ExecutionStatusSucceeded || completed.CompletedAt == nil {
		t.Fatalf("expected completed execution to be succeeded with completion time, got %+v", completed)
	}
	for _, nodeID := range []string{"start", "enrich", "notify"} {
		nodes := workflowNodeExecutionsByID(completed.NodeExecutions, nodeID)
		if len(nodes) != 2 || nodes[len(nodes)-1].Status != NodeStatusSucceeded {
			t.Fatalf("expected seeded and succeeded node %s, got %+v", nodeID, nodes)
		}
	}
	notifyNodes := workflowNodeExecutionsByID(completed.NodeExecutions, "notify")
	if got := notifyNodes[len(notifyNodes)-1].Output["ticket"]; got != "INC-11" {
		t.Fatalf("expected downstream interpolation to reach notify output, got %#v", got)
	}
}

func TestServiceWorkflowSuccessRateEvidenceGate(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		StaticNodeExecutor("start", map[string]any{"ticket": "INC-SLO"}),
		EchoNodeExecutor("agent"),
		EchoNodeExecutor("end"),
	)))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Success Rate Evidence Flow",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{"id": "start", "type": "start"},
				{"id": "enrich", "type": "agent", "input": map[string]any{
					"ticket": "{{nodes.start.output.ticket}}",
					"run":    "{{input.run}}",
				}},
				{"id": "finish", "type": "end", "input": map[string]any{
					"ticket": "{{nodes.enrich.output.ticket}}",
					"run":    "{{nodes.enrich.output.run}}",
				}},
			},
			[]map[string]any{
				{"from": "start", "to": "enrich"},
				{"from": "enrich", "to": "finish"},
			},
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	const executions = 100
	const threshold = 0.99
	succeededBefore := testutil.ToFloat64(metrics.WorkflowExecutionTotal.WithLabelValues(string(ExecutionStatusSucceeded)))
	succeeded := 0
	failed := 0
	for i := range executions {
		execution, err := service.StartExecution(ctx, StartExecutionRequest{
			OrganizationID: "org_1",
			WorkflowID:     workflow.ID,
			Input:          map[string]any{"run": i + 1},
		})
		if err != nil {
			failed++
			t.Logf("workflow_success_rate_start_error run=%d err=%v", i+1, err)
			continue
		}
		completed, err := service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID)
		if err != nil {
			failed++
			t.Logf("workflow_success_rate_run_error run=%d execution=%s err=%v", i+1, execution.ID, err)
			continue
		}
		if completed.Status == ExecutionStatusSucceeded {
			succeeded++
			continue
		}
		failed++
		t.Logf("workflow_success_rate_terminal_miss run=%d execution=%s status=%s", i+1, execution.ID, completed.Status)
	}

	successRate := float64(succeeded) / float64(executions)
	t.Logf("workflow_success_rate_evidence executions=%d succeeded=%d failed=%d success_rate=%.4f threshold=%.4f", executions, succeeded, failed, successRate, threshold)
	if successRate < threshold {
		t.Fatalf("workflow success rate %.4f below %.4f", successRate, threshold)
	}
	succeededAfter := testutil.ToFloat64(metrics.WorkflowExecutionTotal.WithLabelValues(string(ExecutionStatusSucceeded)))
	if got := succeededAfter - succeededBefore; got != executions {
		t.Fatalf("expected %d succeeded workflow metric increments, got %v", executions, got)
	}
}

func TestServiceRunExecutionUntilBlockedPausesAtUserInputAndResumesWithSubmittedInput(t *testing.T) {
	store := newMemoryWorkflowStore()
	var notifyInput NodeExecutorInput
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		StaticNodeExecutor("start", map[string]any{"ticket": "INC-21"}),
		UserInputNodeExecutor(),
		functionNodeExecutor{
			nodeType: "agent",
			execute: func(_ context.Context, input NodeExecutorInput) (map[string]any, error) {
				notifyInput = input
				return map[string]any{"sent": true, "approver": input.Input["approver"]}, nil
			},
		},
	)))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Human Approval Flow",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{"id": "start", "type": "start"},
				{"id": "approval", "type": "user_input", "input": map[string]any{
					"prompt":   "Approve {{nodes.start.output.ticket}}?",
					"required": []any{"approved", "approver"},
				}},
				{"id": "notify", "type": "agent", "input": map[string]any{
					"approved": "{{nodes.approval.output.approved}}",
					"approver": "{{nodes.approval.output.approver}}",
				}},
			},
			[]map[string]any{
				{"from": "start", "to": "approval"},
				{"from": "approval", "to": "notify"},
			},
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{OrganizationID: "org_1", WorkflowID: workflow.ID})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	blocked, err := service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID)
	if !errors.Is(err, ErrWorkflowUserInputRequired) {
		t.Fatalf("RunExecutionUntilBlocked err=%v, want ErrWorkflowUserInputRequired", err)
	}
	if blocked.Status != ExecutionStatusPaused || blocked.CompletedAt != nil {
		t.Fatalf("expected user input node to pause without completing execution, got %+v", blocked)
	}
	approvalNodes := workflowNodeExecutionsByID(blocked.NodeExecutions, "approval")
	if len(approvalNodes) != 2 || approvalNodes[len(approvalNodes)-1].Status != NodeStatusPending {
		t.Fatalf("expected pending user input node after pause, got %+v", approvalNodes)
	}
	pendingApproval := approvalNodes[len(approvalNodes)-1]
	if pendingApproval.Input["prompt"] != "Approve INC-21?" {
		t.Fatalf("expected resolved prompt on pending user input node, got %#v", pendingApproval.Input)
	}
	if pendingApproval.Context["waitReason"] != "user_input_required" {
		t.Fatalf("expected user input wait context, got %+v", pendingApproval.Context)
	}
	if nodes := workflowNodeExecutionsByID(blocked.NodeExecutions, "notify"); len(nodes) != 0 {
		t.Fatalf("notify should not be seeded before user input is submitted, got %+v", nodes)
	}

	resumed, err := service.ResumeExecution(ctx, "org_1", execution.ID, ResumeExecutionRequest{
		NodeID: "approval",
		Input:  map[string]any{"approved": true, "approver": "ops"},
	})
	if err != nil {
		t.Fatalf("ResumeExecution with user input returned error: %v", err)
	}
	if resumed.Status != ExecutionStatusRunning {
		t.Fatalf("expected submitted input to resume execution, got %s", resumed.Status)
	}
	approvalNodes = workflowNodeExecutionsByID(resumed.NodeExecutions, "approval")
	if len(approvalNodes) != 3 || approvalNodes[len(approvalNodes)-1].Status != NodeStatusSucceeded {
		t.Fatalf("expected user input node to be completed with submitted input, got %+v", approvalNodes)
	}
	if approvalNodes[len(approvalNodes)-1].Output["approver"] != "ops" {
		t.Fatalf("expected submitted user input in node output, got %#v", approvalNodes[len(approvalNodes)-1].Output)
	}

	completed, err := service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("RunExecutionUntilBlocked after user input returned error: %v", err)
	}
	if completed.Status != ExecutionStatusSucceeded {
		t.Fatalf("expected workflow to complete after user input, got %+v", completed)
	}
	if notifyInput.Input["approver"] != "ops" || notifyInput.Input["approved"] != true {
		t.Fatalf("expected submitted approval to reach downstream node, got %#v", notifyInput.Input)
	}
}

func TestServiceRunExecutionUntilBlockedPausesAtAgentToolApproval(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(NewAgentNodeExecutor(&recordingWorkflowAgentRunner{
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
	}))))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Agent Approval Flow",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG([]map[string]any{{
			"id":   "call_agent",
			"type": "agent",
			"input": map[string]any{
				"agentId":        "agent_1",
				"conversationId": "conv_1",
				"input":          "delete stale file",
			},
		}}, nil),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		Context:        map[string]any{"userId": "user_1"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	blocked, err := service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID)
	if !errors.Is(err, ErrWorkflowUserInputRequired) {
		t.Fatalf("RunExecutionUntilBlocked err=%v, want ErrWorkflowUserInputRequired", err)
	}
	if blocked.Status != ExecutionStatusPaused {
		t.Fatalf("expected pending agent approval to pause workflow, got %+v", blocked)
	}
	nodes := workflowNodeExecutionsByID(blocked.NodeExecutions, "call_agent")
	if len(nodes) != 2 || nodes[len(nodes)-1].Status != NodeStatusPending {
		t.Fatalf("expected pending agent node after approval pause, got %+v", nodes)
	}
	pending := nodes[len(nodes)-1]
	if pending.Context["waitReason"] != "agent_approval_required" {
		t.Fatalf("expected agent approval wait reason, got %+v", pending.Context)
	}
	if pending.Output["runId"] != "run_pending" || pending.Output["status"] != "pending_approval" {
		t.Fatalf("expected pending agent run output on wait node, got %#v", pending.Output)
	}
}

func TestServiceResumeExecutionCompletesPendingAgentNodeWithApprovedRunResult(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		NewAgentNodeExecutor(&recordingWorkflowAgentRunner{
			response: &WorkflowAgentRunResult{
				RunID:  "run_pending",
				Status: "pending_approval",
			},
		}),
		EchoNodeExecutor("end"),
	)))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Agent Approval Resume Flow",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{
					"id":   "call_agent",
					"type": "agent",
					"input": map[string]any{
						"agentId":        "agent_1",
						"conversationId": "conv_1",
						"input":          "delete stale file",
					},
				},
				{"id": "done", "type": "end", "input": map[string]any{"agentText": "{{nodes.call_agent.output.text}}"}},
			},
			[]map[string]any{{"from": "call_agent", "to": "done"}},
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		Context:        map[string]any{"userId": "user_1"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}
	if _, err := service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID); !errors.Is(err, ErrWorkflowUserInputRequired) {
		t.Fatalf("RunExecutionUntilBlocked err=%v, want ErrWorkflowUserInputRequired", err)
	}

	resumed, err := service.ResumeExecution(ctx, "org_1", execution.ID, ResumeExecutionRequest{
		NodeID: "call_agent",
		Input: map[string]any{
			"runId":          "run_pending",
			"status":         "completed",
			"finalMessageId": "msg_final",
			"text":           "Approved deletion completed",
			"content":        "Approved deletion completed",
		},
	})
	if err != nil {
		t.Fatalf("ResumeExecution returned error: %v", err)
	}
	if resumed.Status != ExecutionStatusRunning {
		t.Fatalf("expected execution to resume running, got %+v", resumed)
	}
	agentNodes := workflowNodeExecutionsByID(resumed.NodeExecutions, "call_agent")
	if len(agentNodes) != 3 || agentNodes[len(agentNodes)-1].Status != NodeStatusSucceeded {
		t.Fatalf("expected agent node to complete with approved result, got %+v", agentNodes)
	}
	if agentNodes[len(agentNodes)-1].Output["text"] != "Approved deletion completed" {
		t.Fatalf("expected approved agent output, got %#v", agentNodes[len(agentNodes)-1].Output)
	}

	completed, err := service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("RunExecutionUntilBlocked after resume returned error: %v", err)
	}
	if completed.Status != ExecutionStatusSucceeded {
		t.Fatalf("expected workflow to complete after agent approval, got %+v", completed)
	}
	doneNodes := workflowNodeExecutionsByID(completed.NodeExecutions, "done")
	if len(doneNodes) != 2 || doneNodes[len(doneNodes)-1].Input["agentText"] != "Approved deletion completed" {
		t.Fatalf("expected downstream done node to receive approved agent text, got %+v", doneNodes)
	}
}

func TestServiceResumeExecutionApprovesPendingAgentToolRun(t *testing.T) {
	store := newMemoryWorkflowStore()
	agentRunner := &recordingWorkflowAgentRunner{
		response: &WorkflowAgentRunResult{
			RunID:  "run_pending",
			Status: "pending_approval",
		},
		approvalResponse: &WorkflowAgentRunResult{
			RunID:          "run_pending",
			Status:         "completed",
			FinalMessageID: "msg_final",
			FinalMessage:   "Approved tool result completed",
		},
	}
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		NewAgentNodeExecutor(agentRunner),
		EchoNodeExecutor("end"),
	)))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Agent Tool Approval Flow",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{
					"id":   "call_agent",
					"type": "agent",
					"input": map[string]any{
						"agentId":        "agent_1",
						"conversationId": "conv_1",
						"input":          "delete stale file",
					},
				},
				{"id": "done", "type": "end", "input": map[string]any{"agentText": "{{nodes.call_agent.output.text}}"}},
			},
			[]map[string]any{{"from": "call_agent", "to": "done"}},
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		Context:        map[string]any{"userId": "user_1", "workspaceId": "workspace_1"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}
	if _, err := service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID); !errors.Is(err, ErrWorkflowUserInputRequired) {
		t.Fatalf("RunExecutionUntilBlocked err=%v, want ErrWorkflowUserInputRequired", err)
	}

	resumed, err := service.ResumeExecution(ctx, "org_1", execution.ID, ResumeExecutionRequest{
		NodeID: "call_agent",
		Input: map[string]any{
			"toolRunId":      "tool_run_pending",
			"approvalReason": "operator approved",
		},
	})
	if err != nil {
		t.Fatalf("ResumeExecution returned error: %v", err)
	}
	if agentRunner.approvalRequest.RunID != "run_pending" || agentRunner.approvalRequest.ToolRunID != "tool_run_pending" || agentRunner.approvalRequest.Reason != "operator approved" {
		t.Fatalf("unexpected approval request: %+v", agentRunner.approvalRequest)
	}
	if agentRunner.approvalRequest.UserID != "user_1" || agentRunner.approvalRequest.WorkspaceID != "workspace_1" {
		t.Fatalf("unexpected approval scope: %+v", agentRunner.approvalRequest)
	}
	agentNodes := workflowNodeExecutionsByID(resumed.NodeExecutions, "call_agent")
	if len(agentNodes) != 3 || agentNodes[len(agentNodes)-1].Status != NodeStatusSucceeded {
		t.Fatalf("expected completed agent node after approval, got %+v", agentNodes)
	}
	if agentNodes[len(agentNodes)-1].Output["text"] != "Approved tool result completed" {
		t.Fatalf("expected approved run output, got %#v", agentNodes[len(agentNodes)-1].Output)
	}

	completed, err := service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("RunExecutionUntilBlocked after approval returned error: %v", err)
	}
	doneNodes := workflowNodeExecutionsByID(completed.NodeExecutions, "done")
	if len(doneNodes) != 2 || doneNodes[len(doneNodes)-1].Input["agentText"] != "Approved tool result completed" {
		t.Fatalf("expected downstream node to receive approved run text, got %+v", doneNodes)
	}
}

func TestServiceResumeExecutionExecutesPendingAgentPlanStep(t *testing.T) {
	store := newMemoryWorkflowStore()
	agentRunner := &recordingWorkflowAgentRunner{
		response: &WorkflowAgentRunResult{
			RunID:  "run_plan",
			Status: "pending_approval",
			PlanSteps: []WorkflowAgentPlanStep{{
				ID:             "step_pending",
				Title:          "Run verification",
				Status:         "pending",
				ApprovalStatus: "approved",
				ToolName:       "shell",
			}},
		},
		controlResponse: &WorkflowAgentRunResult{
			RunID:          "run_plan",
			Status:         "completed",
			FinalMessageID: "msg_final",
			FinalMessage:   "Plan step completed",
			PlanSteps: []WorkflowAgentPlanStep{{
				ID:             "step_pending",
				Title:          "Run verification",
				Status:         "completed",
				ApprovalStatus: "approved",
				ToolName:       "shell",
				ResultContent:  "go test passed",
			}},
		},
	}
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		NewAgentNodeExecutor(agentRunner),
		EchoNodeExecutor("end"),
	)))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Agent Plan Step Resume Flow",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{
					"id":   "call_agent",
					"type": "agent",
					"input": map[string]any{
						"agentId":        "agent_1",
						"conversationId": "conv_1",
						"input":          "verify change",
						"mode":           "planning",
					},
				},
				{"id": "done", "type": "end", "input": map[string]any{"agentText": "{{nodes.call_agent.output.text}}"}},
			},
			[]map[string]any{{"from": "call_agent", "to": "done"}},
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		Context:        map[string]any{"userId": "user_1", "workspaceId": "workspace_1"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}
	if _, err := service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID); !errors.Is(err, ErrWorkflowUserInputRequired) {
		t.Fatalf("RunExecutionUntilBlocked err=%v, want ErrWorkflowUserInputRequired", err)
	}

	resumed, err := service.ResumeExecution(ctx, "org_1", execution.ID, ResumeExecutionRequest{
		NodeID: "call_agent",
		Input: map[string]any{
			"action":     "execute_plan_step",
			"planStepId": "step_pending",
		},
	})
	if err != nil {
		t.Fatalf("ResumeExecution returned error: %v", err)
	}
	if agentRunner.controlAction != "execute_plan_step" ||
		agentRunner.controlRequest.RunID != "run_plan" ||
		agentRunner.controlRequest.PlanStepID != "step_pending" ||
		agentRunner.controlRequest.UserID != "user_1" ||
		agentRunner.controlRequest.WorkspaceID != "workspace_1" {
		t.Fatalf("unexpected agent plan-step control request: action=%q req=%+v", agentRunner.controlAction, agentRunner.controlRequest)
	}
	agentNodes := workflowNodeExecutionsByID(resumed.NodeExecutions, "call_agent")
	if len(agentNodes) != 3 || agentNodes[len(agentNodes)-1].Status != NodeStatusSucceeded {
		t.Fatalf("expected completed agent node after plan-step execution, got %+v", agentNodes)
	}
	if agentNodes[len(agentNodes)-1].Output["text"] != "Plan step completed" {
		t.Fatalf("expected plan-step run output, got %#v", agentNodes[len(agentNodes)-1].Output)
	}
	steps, ok := agentNodes[len(agentNodes)-1].Output["planSteps"].([]map[string]any)
	if !ok || len(steps) != 1 || steps[0]["resultContent"] != "go test passed" {
		t.Fatalf("expected plan-step result evidence in workflow output, got %#v", agentNodes[len(agentNodes)-1].Output["planSteps"])
	}

	completed, err := service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("RunExecutionUntilBlocked after plan step returned error: %v", err)
	}
	doneNodes := workflowNodeExecutionsByID(completed.NodeExecutions, "done")
	if len(doneNodes) != 2 || doneNodes[len(doneNodes)-1].Input["agentText"] != "Plan step completed" {
		t.Fatalf("expected downstream node to receive plan-step result, got %+v", doneNodes)
	}
}

func TestServiceResumeExecutionContinuesPendingAgentPlan(t *testing.T) {
	store := newMemoryWorkflowStore()
	agentRunner := &recordingWorkflowAgentRunner{
		response: &WorkflowAgentRunResult{
			RunID:  "run_continue",
			Status: "pending_approval",
			PlanSteps: []WorkflowAgentPlanStep{{
				ID:             "step_pending",
				Title:          "Continue verification",
				Status:         "pending",
				ApprovalStatus: "not_required",
			}},
		},
		controlResponse: &WorkflowAgentRunResult{
			RunID:          "run_continue",
			Status:         "completed",
			FinalMessageID: "msg_final",
			FinalMessage:   "Continued plan completed",
			PlanSteps: []WorkflowAgentPlanStep{{
				ID:            "step_pending",
				Title:         "Continue verification",
				Status:        "completed",
				ResultContent: "continued safely",
			}},
		},
	}
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		NewAgentNodeExecutor(agentRunner),
		EchoNodeExecutor("end"),
	)))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Agent Plan Continue Resume Flow",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{
					"id":   "call_agent",
					"type": "agent",
					"input": map[string]any{
						"agentId":        "agent_1",
						"conversationId": "conv_1",
						"input":          "continue plan",
						"mode":           "planning",
					},
				},
				{"id": "done", "type": "end", "input": map[string]any{"agentText": "{{nodes.call_agent.output.text}}"}},
			},
			[]map[string]any{{"from": "call_agent", "to": "done"}},
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		Context:        map[string]any{"userId": "user_1", "workspaceId": "workspace_1"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}
	if _, err := service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID); !errors.Is(err, ErrWorkflowUserInputRequired) {
		t.Fatalf("RunExecutionUntilBlocked err=%v, want ErrWorkflowUserInputRequired", err)
	}

	resumed, err := service.ResumeExecution(ctx, "org_1", execution.ID, ResumeExecutionRequest{
		NodeID: "call_agent",
		Input:  map[string]any{"action": "continue_plan"},
	})
	if err != nil {
		t.Fatalf("ResumeExecution returned error: %v", err)
	}
	if agentRunner.controlAction != "continue_plan" ||
		agentRunner.controlRequest.RunID != "run_continue" ||
		agentRunner.controlRequest.UserID != "user_1" ||
		agentRunner.controlRequest.WorkspaceID != "workspace_1" {
		t.Fatalf("unexpected continue-plan control request: action=%q req=%+v", agentRunner.controlAction, agentRunner.controlRequest)
	}
	agentNodes := workflowNodeExecutionsByID(resumed.NodeExecutions, "call_agent")
	if len(agentNodes) != 3 || agentNodes[len(agentNodes)-1].Status != NodeStatusSucceeded {
		t.Fatalf("expected completed agent node after continue plan, got %+v", agentNodes)
	}
	if agentNodes[len(agentNodes)-1].Output["text"] != "Continued plan completed" {
		t.Fatalf("expected continue-plan output, got %#v", agentNodes[len(agentNodes)-1].Output)
	}

	completed, err := service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("RunExecutionUntilBlocked after continue plan returned error: %v", err)
	}
	doneNodes := workflowNodeExecutionsByID(completed.NodeExecutions, "done")
	if len(doneNodes) != 2 || doneNodes[len(doneNodes)-1].Input["agentText"] != "Continued plan completed" {
		t.Fatalf("expected downstream node to receive continue-plan result, got %+v", doneNodes)
	}
}

func TestServiceResumeExecutionAdjustsPendingAgentPlan(t *testing.T) {
	store := newMemoryWorkflowStore()
	agentRunner := &recordingWorkflowAgentRunner{
		response: &WorkflowAgentRunResult{
			RunID:  "run_adjust",
			Status: "pending_approval",
			PlanSteps: []WorkflowAgentPlanStep{{
				ID:             "step_risky",
				Title:          "Run risky command",
				Status:         "pending",
				ApprovalStatus: "pending",
				ToolName:       "shell",
			}},
		},
		controlResponse: &WorkflowAgentRunResult{
			RunID:          "run_adjust",
			Status:         "completed",
			FinalMessageID: "msg_final",
			FinalMessage:   "Adjusted plan completed",
			PlanSteps: []WorkflowAgentPlanStep{{
				ID:            "step_safe",
				Title:         "Use safe command",
				Status:        "completed",
				ToolName:      "shell",
				ResultContent: "used safe command",
			}},
		},
	}
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		NewAgentNodeExecutor(agentRunner),
		EchoNodeExecutor("end"),
	)))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Agent Plan Adjust Resume Flow",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{
					"id":   "call_agent",
					"type": "agent",
					"input": map[string]any{
						"agentId":        "agent_1",
						"conversationId": "conv_1",
						"input":          "adjust plan",
						"mode":           "planning",
					},
				},
				{"id": "done", "type": "end", "input": map[string]any{"agentText": "{{nodes.call_agent.output.text}}"}},
			},
			[]map[string]any{{"from": "call_agent", "to": "done"}},
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		Context:        map[string]any{"userId": "user_1", "workspaceId": "workspace_1"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}
	if _, err := service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID); !errors.Is(err, ErrWorkflowUserInputRequired) {
		t.Fatalf("RunExecutionUntilBlocked err=%v, want ErrWorkflowUserInputRequired", err)
	}

	resumed, err := service.ResumeExecution(ctx, "org_1", execution.ID, ResumeExecutionRequest{
		NodeID: "call_agent",
		Input: map[string]any{
			"action": "adjust_plan",
			"reason": "avoid risky command",
		},
	})
	if err != nil {
		t.Fatalf("ResumeExecution returned error: %v", err)
	}
	if agentRunner.controlAction != "adjust_plan" ||
		agentRunner.controlRequest.RunID != "run_adjust" ||
		agentRunner.controlRequest.UserID != "user_1" ||
		agentRunner.controlRequest.WorkspaceID != "workspace_1" ||
		agentRunner.controlRequest.Reason != "avoid risky command" {
		t.Fatalf("unexpected adjust-plan control request: action=%q req=%+v", agentRunner.controlAction, agentRunner.controlRequest)
	}
	agentNodes := workflowNodeExecutionsByID(resumed.NodeExecutions, "call_agent")
	if len(agentNodes) != 3 || agentNodes[len(agentNodes)-1].Status != NodeStatusSucceeded {
		t.Fatalf("expected completed agent node after adjust plan, got %+v", agentNodes)
	}
	if agentNodes[len(agentNodes)-1].Output["text"] != "Adjusted plan completed" {
		t.Fatalf("expected adjust-plan output, got %#v", agentNodes[len(agentNodes)-1].Output)
	}

	completed, err := service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("RunExecutionUntilBlocked after adjust plan returned error: %v", err)
	}
	doneNodes := workflowNodeExecutionsByID(completed.NodeExecutions, "done")
	if len(doneNodes) != 2 || doneNodes[len(doneNodes)-1].Input["agentText"] != "Adjusted plan completed" {
		t.Fatalf("expected downstream node to receive adjust-plan result, got %+v", doneNodes)
	}
}

func TestServiceResumeExecutionContinuesAgentRunWithTokenBudget(t *testing.T) {
	store := newMemoryWorkflowStore()
	agentRunner := &recordingWorkflowAgentRunner{
		response: &WorkflowAgentRunResult{
			RunID:  "run_budget",
			Status: "token_budget_exceeded",
			PlanSteps: []WorkflowAgentPlanStep{{
				ID:     "step_budget",
				Title:  "Retry with larger budget",
				Status: "failed",
				Error:  "token_budget_exceeded: plan step exceeded budget",
			}},
		},
		controlResponse: &WorkflowAgentRunResult{
			RunID:          "run_budget",
			Status:         "completed",
			FinalMessageID: "msg_final",
			FinalMessage:   "Budget continuation completed",
			PlanSteps: []WorkflowAgentPlanStep{{
				ID:            "step_budget",
				Title:         "Retry with larger budget",
				Status:        "completed",
				ResultContent: "retried with larger budget",
			}},
		},
	}
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		NewAgentNodeExecutor(agentRunner),
		EchoNodeExecutor("end"),
	)))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Agent Budget Resume Flow",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{
					"id":   "call_agent",
					"type": "agent",
					"input": map[string]any{
						"agentId":        "agent_1",
						"conversationId": "conv_1",
						"input":          "large investigation",
						"mode":           "planning",
						"tokenBudget":    1000,
					},
				},
				{"id": "done", "type": "end", "input": map[string]any{"agentText": "{{nodes.call_agent.output.text}}"}},
			},
			[]map[string]any{{"from": "call_agent", "to": "done"}},
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		Context:        map[string]any{"userId": "user_1", "workspaceId": "workspace_1"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}
	blocked, err := service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID)
	if !errors.Is(err, ErrWorkflowUserInputRequired) {
		t.Fatalf("RunExecutionUntilBlocked err=%v, want ErrWorkflowUserInputRequired", err)
	}
	agentNodes := workflowNodeExecutionsByID(blocked.NodeExecutions, "call_agent")
	if len(agentNodes) != 2 || agentNodes[len(agentNodes)-1].Status != NodeStatusPending || agentNodes[len(agentNodes)-1].Output["status"] != "token_budget_exceeded" {
		t.Fatalf("expected pending token-budget agent node, got %+v", agentNodes)
	}

	resumed, err := service.ResumeExecution(ctx, "org_1", execution.ID, ResumeExecutionRequest{
		NodeID: "call_agent",
		Input: map[string]any{
			"tokenBudget": 5000,
		},
	})
	if err != nil {
		t.Fatalf("ResumeExecution returned error: %v", err)
	}
	if agentRunner.controlAction != "continue_budget" ||
		agentRunner.controlRequest.RunID != "run_budget" ||
		agentRunner.controlRequest.TokenBudget != 5000 ||
		agentRunner.controlRequest.UserID != "user_1" {
		t.Fatalf("unexpected agent budget control request: action=%q req=%+v", agentRunner.controlAction, agentRunner.controlRequest)
	}
	agentNodes = workflowNodeExecutionsByID(resumed.NodeExecutions, "call_agent")
	if len(agentNodes) != 3 || agentNodes[len(agentNodes)-1].Status != NodeStatusSucceeded {
		t.Fatalf("expected completed agent node after budget continuation, got %+v", agentNodes)
	}
	if agentNodes[len(agentNodes)-1].Output["text"] != "Budget continuation completed" {
		t.Fatalf("expected budget continuation output, got %#v", agentNodes[len(agentNodes)-1].Output)
	}

	completed, err := service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("RunExecutionUntilBlocked after budget continuation returned error: %v", err)
	}
	doneNodes := workflowNodeExecutionsByID(completed.NodeExecutions, "done")
	if len(doneNodes) != 2 || doneNodes[len(doneNodes)-1].Input["agentText"] != "Budget continuation completed" {
		t.Fatalf("expected downstream node to receive budget continuation result, got %+v", doneNodes)
	}
}

func TestServiceResumeExecutionRejectsUserInputMissingRequiredFields(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		UserInputNodeExecutor(),
	)))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Required Approval Flow",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG([]map[string]any{{
			"id":   "approval",
			"type": "user_input",
			"input": map[string]any{
				"prompt":   "Approve?",
				"required": []any{"approved", "approver"},
			},
		}}, nil),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{OrganizationID: "org_1", WorkflowID: workflow.ID})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}
	blocked, err := service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID)
	if !errors.Is(err, ErrWorkflowUserInputRequired) {
		t.Fatalf("RunExecutionUntilBlocked err=%v, want ErrWorkflowUserInputRequired", err)
	}
	if blocked.Status != ExecutionStatusPaused {
		t.Fatalf("expected paused execution, got %+v", blocked)
	}

	_, err = service.ResumeExecution(ctx, "org_1", execution.ID, ResumeExecutionRequest{
		NodeID: "approval",
		Input:  map[string]any{"approved": true},
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("ResumeExecution missing required field err=%v, want ErrInvalidInput", err)
	}
	stillPaused, err := service.GetExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution returned error: %v", err)
	}
	if stillPaused.Status != ExecutionStatusPaused {
		t.Fatalf("invalid user input must leave execution paused, got %+v", stillPaused)
	}
	approvalNodes := workflowNodeExecutionsByID(stillPaused.NodeExecutions, "approval")
	if len(approvalNodes) != 2 || approvalNodes[len(approvalNodes)-1].Status != NodeStatusPending {
		t.Fatalf("invalid user input must not complete node, got %+v", approvalNodes)
	}
}

func TestServiceRunExecutionUntilBlockedContinuesAfterSkipOnFailure(t *testing.T) {
	store := newMemoryWorkflowStore()
	notifyRuns := 0
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		StaticNodeExecutor("start", map[string]any{"ticket": "INC-12"}),
		functionNodeExecutor{
			nodeType: "knowledge",
			execute: func(context.Context, NodeExecutorInput) (map[string]any, error) {
				return nil, errors.New("knowledge timeout")
			},
		},
		functionNodeExecutor{
			nodeType: "agent",
			execute: func(context.Context, NodeExecutorInput) (map[string]any, error) {
				notifyRuns++
				return map[string]any{"notified": true}, nil
			},
		},
	)))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Optional Lookup Flow",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{"id": "start", "type": "start"},
				{"id": "optional_lookup", "type": "knowledge", "failurePolicy": map[string]any{"strategy": "skip_on_failure"}},
				{"id": "notify", "type": "agent"},
			},
			[]map[string]any{
				{"from": "start", "to": "optional_lookup"},
				{"from": "optional_lookup", "to": "notify"},
			},
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	completed, err := service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("RunExecutionUntilBlocked returned error for skipped optional node: %v", err)
	}
	if completed.Status != ExecutionStatusPartialSuccess || completed.CompletedAt == nil {
		t.Fatalf("expected partial success with completion time, got %+v", completed)
	}
	if notifyRuns != 1 {
		t.Fatalf("expected downstream notify node to execute once, got %d", notifyRuns)
	}
	lookupNodes := workflowNodeExecutionsByID(completed.NodeExecutions, "optional_lookup")
	if len(lookupNodes) == 0 || lookupNodes[len(lookupNodes)-1].Status != NodeStatusSkipped {
		t.Fatalf("expected optional lookup to be skipped, got %+v", lookupNodes)
	}
	notifyNodes := workflowNodeExecutionsByID(completed.NodeExecutions, "notify")
	if len(notifyNodes) == 0 || notifyNodes[len(notifyNodes)-1].Status != NodeStatusSucceeded {
		t.Fatalf("expected notify node to succeed after skipped parent, got %+v", notifyNodes)
	}
}

func TestServiceRunExecutionUntilBlockedWaitsForAutoRetryDelay(t *testing.T) {
	store := newMemoryWorkflowStore()
	attempts := 0
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(functionNodeExecutor{
		nodeType: "http",
		execute: func(context.Context, NodeExecutorInput) (map[string]any, error) {
			attempts++
			return nil, errors.New("upstream unavailable")
		},
	})))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Delayed Auto Retry",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG([]map[string]any{{
			"id":   "call_api",
			"type": "http",
			"failurePolicy": map[string]any{
				"strategy":    "auto_retry",
				"maxRetries":  2,
				"retryDelays": []any{"1h"},
			},
		}}, nil),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{OrganizationID: "org_1", WorkflowID: workflow.ID})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	blocked, err := service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("RunExecutionUntilBlocked returned retry scheduling error: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected one execution attempt before retry delay, got %d", attempts)
	}
	if blocked.Status != ExecutionStatusRunning {
		t.Fatalf("expected execution to remain running while waiting for retry, got %s", blocked.Status)
	}
	nodes := workflowNodeExecutionsByID(blocked.NodeExecutions, "call_api")
	if len(nodes) != 2 || nodes[len(nodes)-1].Status != NodeStatusRetrying || nodes[len(nodes)-1].Attempt != 2 {
		t.Fatalf("expected retrying attempt 2 after first failure, got %+v", nodes)
	}
	if nodes[len(nodes)-1].Context["retryAt"] == "" {
		t.Fatalf("expected retryAt context to be recorded, got %+v", nodes[len(nodes)-1].Context)
	}
}

func TestServiceRunExecutionUntilBlockedRunsDueAutoRetry(t *testing.T) {
	store := newMemoryWorkflowStore()
	attempts := 0
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(functionNodeExecutor{
		nodeType: "http",
		execute: func(context.Context, NodeExecutorInput) (map[string]any, error) {
			attempts++
			if attempts == 1 {
				return nil, errors.New("temporary outage")
			}
			return map[string]any{"attempts": attempts}, nil
		},
	})))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Due Auto Retry",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG([]map[string]any{{
			"id":   "call_api",
			"type": "http",
			"failurePolicy": map[string]any{
				"strategy":    "auto_retry",
				"maxRetries":  2,
				"retryDelays": []any{"1ns"},
			},
		}}, nil),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{OrganizationID: "org_1", WorkflowID: workflow.ID})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	completed, err := service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("RunExecutionUntilBlocked returned error after due retry: %v", err)
	}
	if attempts != 2 {
		t.Fatalf("expected original execution plus one retry, got %d attempts", attempts)
	}
	if completed.Status != ExecutionStatusSucceeded {
		t.Fatalf("expected execution to succeed after retry, got %+v", completed)
	}
	nodes := workflowNodeExecutionsByID(completed.NodeExecutions, "call_api")
	if len(nodes) != 3 {
		t.Fatalf("expected pending, retrying, and succeeded records, got %+v", nodes)
	}
	if nodes[len(nodes)-1].Status != NodeStatusSucceeded || nodes[len(nodes)-1].Attempt != 2 {
		t.Fatalf("expected retry attempt 2 to succeed, got %+v", nodes)
	}
}

func TestServiceRunExecutionUntilBlockedAllowsExecutionAtNodeLimit(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		EchoNodeExecutor("start"),
		EchoNodeExecutor("agent"),
		EchoNodeExecutor("end"),
	)))
	ctx := context.Background()
	definition := workflowDefinitionDAG(
		[]map[string]any{
			{"id": "start", "type": "start"},
			{"id": "enrich", "type": "agent"},
			{"id": "notify", "type": "end"},
		},
		[]map[string]any{
			{"from": "start", "to": "enrich"},
			{"from": "enrich", "to": "notify"},
		},
	)
	definition["max_node_executions"] = 3
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Bounded Auto DAG",
		Status:         WorkflowStatusPublished,
		Definition:     definition,
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{OrganizationID: "org_1", WorkflowID: workflow.ID})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	completed, err := service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("RunExecutionUntilBlocked returned error at exact node limit: %v", err)
	}
	if completed.Status != ExecutionStatusSucceeded {
		t.Fatalf("expected exact node limit execution to succeed, got %+v", completed)
	}
}

func TestServiceRunReadyNodeRecordsUnknownExecutorAndVariableErrors(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(EchoNodeExecutor("http"))))
	ctx := context.Background()

	unknownWorkflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Unknown Executor Flow",
		Status:         WorkflowStatusPublished,
		Definition:     workflowDefinitionDAG([]map[string]any{{"id": "call_agent", "type": "agent"}}, nil),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow unknown returned error: %v", err)
	}
	unknownExecution, err := service.StartExecution(ctx, StartExecutionRequest{OrganizationID: "org_1", WorkflowID: unknownWorkflow.ID})
	if err != nil {
		t.Fatalf("StartExecution unknown returned error: %v", err)
	}

	err = service.RunReadyNode(ctx, "org_1", unknownExecution.ID, "call_agent")
	if !errors.Is(err, ErrNodeExecutorNotFound) {
		t.Fatalf("RunReadyNode unknown executor err=%v, want ErrNodeExecutorNotFound", err)
	}
	unknownExecution, err = service.GetExecution(ctx, "org_1", unknownExecution.ID)
	if err != nil {
		t.Fatalf("GetExecution unknown returned error: %v", err)
	}
	unknownNodes := workflowNodeExecutionsByID(unknownExecution.NodeExecutions, "call_agent")
	if len(unknownNodes) != 2 || unknownNodes[len(unknownNodes)-1].Status != NodeStatusFailed {
		t.Fatalf("expected unknown executor failure node to be recorded, got %+v", unknownNodes)
	}
	if unknownExecution.Status != ExecutionStatusPaused {
		t.Fatalf("expected unknown executor to pause execution, got %s", unknownExecution.Status)
	}
	if !strings.Contains(errorMessage(unknownNodes[len(unknownNodes)-1].Error), "executor") {
		t.Fatalf("expected executor error to be traceable, got %#v", unknownNodes[len(unknownNodes)-1].Error)
	}

	variableWorkflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Missing Variable Flow",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG([]map[string]any{{
			"id":    "notify",
			"type":  "http",
			"input": map[string]any{"body": "{{input.missing}}"},
		}}, nil),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow variable returned error: %v", err)
	}
	variableExecution, err := service.StartExecution(ctx, StartExecutionRequest{OrganizationID: "org_1", WorkflowID: variableWorkflow.ID})
	if err != nil {
		t.Fatalf("StartExecution variable returned error: %v", err)
	}

	err = service.RunReadyNode(ctx, "org_1", variableExecution.ID, "notify")
	if !errors.Is(err, ErrWorkflowVariableNotFound) {
		t.Fatalf("RunReadyNode missing variable err=%v, want ErrWorkflowVariableNotFound", err)
	}
	variableExecution, err = service.GetExecution(ctx, "org_1", variableExecution.ID)
	if err != nil {
		t.Fatalf("GetExecution variable returned error: %v", err)
	}
	variableNodes := workflowNodeExecutionsByID(variableExecution.NodeExecutions, "notify")
	if len(variableNodes) != 2 || variableNodes[len(variableNodes)-1].Status != NodeStatusFailed {
		t.Fatalf("expected missing variable failure node to be recorded, got %+v", variableNodes)
	}
	if !strings.Contains(errorMessage(variableNodes[len(variableNodes)-1].Error), "input.missing") {
		t.Fatalf("expected missing variable path in node error, got %#v", variableNodes[len(variableNodes)-1].Error)
	}
}

func TestServiceRunExecutionFailureBranchRunsWithErrorContext(t *testing.T) {
	store := newMemoryWorkflowStore()
	var branchInput NodeExecutorInput
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		functionNodeExecutor{
			nodeType: "http",
			execute: func(context.Context, NodeExecutorInput) (map[string]any, error) {
				return nil, errors.New("payment declined")
			},
		},
		functionNodeExecutor{
			nodeType: "agent",
			execute: func(_ context.Context, input NodeExecutorInput) (map[string]any, error) {
				branchInput = input
				return map[string]any{"notified": true}, nil
			},
		},
	)))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Payment Failure Branch",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{"id": "charge_card", "type": "http", "failurePolicy": map[string]any{"strategy": "failure_branch", "failureBranchNodeId": "notify_ops"}},
				{"id": "notify_ops", "type": "agent"},
			},
			[]map[string]any{{"from": "charge_card", "to": "notify_ops"}},
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		Input:          map[string]any{"invoiceId": "inv_123"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	_, err = service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID)
	if err == nil || !strings.Contains(err.Error(), "payment declined") {
		t.Fatalf("RunExecutionUntilBlocked err=%v, want payment declined", err)
	}

	updated, err := service.GetExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution returned error: %v", err)
	}
	if updated.Status != ExecutionStatusRunning {
		t.Fatalf("expected failure_branch execution to remain running, got %s", updated.Status)
	}
	branchNodes := workflowNodeExecutionsByID(updated.NodeExecutions, "notify_ops")
	if len(branchNodes) != 2 {
		t.Fatalf("expected pending and running branch node records, got %+v", branchNodes)
	}
	if branchNodes[0].Status != NodeStatusPending || branchNodes[1].Status != NodeStatusSucceeded {
		t.Fatalf("expected branch node to be seeded then executed, got %+v", branchNodes)
	}
	if branchInput.Input["invoiceId"] != "inv_123" {
		t.Fatalf("expected workflow input to reach branch node, got %#v", branchInput.Input)
	}
	assertWorkflowErrorContext(t, branchInput.Input["workflow.error"], "charge_card", "payment declined")
}

func TestServiceRunReadyNodeExecutesConditionNode(t *testing.T) {
	tests := []struct {
		name          string
		priority      string
		wantMatched   bool
		wantBranch    string
		wantInputLeft string
	}{
		{
			name:          "matches high priority",
			priority:      "high",
			wantMatched:   true,
			wantBranch:    "true",
			wantInputLeft: "high",
		},
		{
			name:          "does not match low priority",
			priority:      "low",
			wantMatched:   false,
			wantBranch:    "false",
			wantInputLeft: "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMemoryWorkflowStore()
			service := NewService(store)
			ctx := context.Background()
			workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
				OrganizationID: "org_1",
				Name:           "Priority Branch",
				Status:         WorkflowStatusPublished,
				Definition: workflowDefinitionDAG(
					[]map[string]any{{
						"id":   "check_priority",
						"type": "condition",
						"input": map[string]any{
							"left":     "{{input.priority}}",
							"operator": "equals",
							"right":    "high",
						},
					}},
					nil,
				),
			})
			if err != nil {
				t.Fatalf("CreateWorkflow returned error: %v", err)
			}
			execution, err := service.StartExecution(ctx, StartExecutionRequest{
				OrganizationID: "org_1",
				WorkflowID:     workflow.ID,
				Input:          map[string]any{"priority": tt.priority},
			})
			if err != nil {
				t.Fatalf("StartExecution returned error: %v", err)
			}

			if err := service.RunReadyNode(ctx, "org_1", execution.ID, "check_priority"); err != nil {
				t.Fatalf("RunReadyNode condition returned error: %v", err)
			}

			updated, err := service.GetExecution(ctx, "org_1", execution.ID)
			if err != nil {
				t.Fatalf("GetExecution returned error: %v", err)
			}
			nodes := workflowNodeExecutionsByID(updated.NodeExecutions, "check_priority")
			if len(nodes) != 2 {
				t.Fatalf("expected seeded and completed condition nodes, got %+v", nodes)
			}
			completed := nodes[len(nodes)-1]
			if completed.Status != NodeStatusSucceeded {
				t.Fatalf("expected condition node to succeed, got %+v", completed)
			}
			if completed.Input["left"] != tt.wantInputLeft {
				t.Fatalf("expected interpolated condition input left=%q, got %#v", tt.wantInputLeft, completed.Input)
			}
			if completed.Output["matched"] != tt.wantMatched || completed.Output["branch"] != tt.wantBranch {
				t.Fatalf("expected condition output matched=%v branch=%q, got %#v", tt.wantMatched, tt.wantBranch, completed.Output)
			}
		})
	}
}

func TestServiceRunReadyNodeExecutesLoopNodeAndSeedsMatchedBranch(t *testing.T) {
	tests := []struct {
		name             string
		items            []any
		wantMatched      bool
		wantBranch       string
		wantIterationCnt float64
		wantIteratorNode bool
		wantDoneNode     bool
	}{
		{
			name:             "non empty items continue loop branch",
			items:            []any{"INC-1", "INC-2"},
			wantMatched:      true,
			wantBranch:       "true",
			wantIterationCnt: 2,
			wantIteratorNode: true,
		},
		{
			name:             "empty items continue done branch",
			items:            []any{},
			wantMatched:      false,
			wantBranch:       "false",
			wantIterationCnt: 0,
			wantDoneNode:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := newMemoryWorkflowStore()
			service := NewService(store)
			ctx := context.Background()
			workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
				OrganizationID: "org_1",
				Name:           "Loop Flow",
				Status:         WorkflowStatusPublished,
				Definition: workflowDefinitionDAG(
					[]map[string]any{
						{
							"id":   "for_each_ticket",
							"type": "loop",
							"input": map[string]any{
								"items": "{{input.tickets}}",
							},
						},
						{"id": "process_ticket", "type": "manual"},
						{"id": "done", "type": "end"},
					},
					[]map[string]any{
						{"from": "for_each_ticket", "to": "process_ticket", "branch": "true"},
						{"from": "for_each_ticket", "to": "done", "branch": "false"},
					},
				),
			})
			if err != nil {
				t.Fatalf("CreateWorkflow returned error: %v", err)
			}
			execution, err := service.StartExecution(ctx, StartExecutionRequest{
				OrganizationID: "org_1",
				WorkflowID:     workflow.ID,
				Input:          map[string]any{"tickets": tt.items},
			})
			if err != nil {
				t.Fatalf("StartExecution returned error: %v", err)
			}

			if err := service.RunReadyNode(ctx, "org_1", execution.ID, "for_each_ticket"); err != nil {
				t.Fatalf("RunReadyNode loop returned error: %v", err)
			}

			updated, err := service.GetExecution(ctx, "org_1", execution.ID)
			if err != nil {
				t.Fatalf("GetExecution returned error: %v", err)
			}
			loopNodes := workflowNodeExecutionsByID(updated.NodeExecutions, "for_each_ticket")
			if len(loopNodes) != 2 {
				t.Fatalf("expected seeded and completed loop nodes, got %+v", loopNodes)
			}
			completed := loopNodes[len(loopNodes)-1]
			if completed.Status != NodeStatusSucceeded {
				t.Fatalf("expected loop node to succeed, got %+v", completed)
			}
			if completed.Output["matched"] != tt.wantMatched || completed.Output["branch"] != tt.wantBranch {
				t.Fatalf("expected loop matched=%v branch=%q, got %#v", tt.wantMatched, tt.wantBranch, completed.Output)
			}
			if completed.Output["iterationCount"] != tt.wantIterationCnt {
				t.Fatalf("expected iterationCount=%v, got %#v", tt.wantIterationCnt, completed.Output)
			}
			if tt.wantIteratorNode {
				nodes := workflowNodeExecutionsByID(updated.NodeExecutions, "process_ticket")
				if len(nodes) != 1 || nodes[0].Status != NodeStatusPending {
					t.Fatalf("expected process_ticket branch to be seeded, got %+v", nodes)
				}
				if nodes := workflowNodeExecutionsByID(updated.NodeExecutions, "done"); len(nodes) != 0 {
					t.Fatalf("expected done branch not to be seeded, got %+v", nodes)
				}
			}
			if tt.wantDoneNode {
				nodes := workflowNodeExecutionsByID(updated.NodeExecutions, "done")
				if len(nodes) != 1 || nodes[0].Status != NodeStatusPending {
					t.Fatalf("expected done branch to be seeded, got %+v", nodes)
				}
				if nodes := workflowNodeExecutionsByID(updated.NodeExecutions, "process_ticket"); len(nodes) != 0 {
					t.Fatalf("expected process_ticket branch not to be seeded, got %+v", nodes)
				}
			}
		})
	}
}

func TestServiceRunReadyNodeExecutesCodeNodeOutputsForDownstreamInterpolation(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		StaticNodeExecutor("start", map[string]any{"priority": "high"}),
		NewCodeNodeExecutor(),
		EchoNodeExecutor("agent"),
	)))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Code Transform Flow",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{"id": "start", "type": "start"},
				{"id": "shape_payload", "type": "code", "input": map[string]any{
					"language": "javascript",
					"outputs": map[string]any{
						"summary":  "{{input.ticket}}/{{nodes.start.output.priority}}",
						"assignee": "{{workflow.defaultAssignee}}",
						"payload": map[string]any{
							"ticket":   "{{input.ticket}}",
							"priority": "{{nodes.start.output.priority}}",
						},
					},
				}},
				{"id": "notify", "type": "agent", "input": map[string]any{
					"message":  "{{nodes.shape_payload.output.summary}}",
					"assignee": "{{nodes.shape_payload.output.assignee}}",
				}},
			},
			[]map[string]any{
				{"from": "start", "to": "shape_payload"},
				{"from": "shape_payload", "to": "notify"},
			},
		),
		Variables: map[string]any{"defaultAssignee": "ops"},
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		Input:          map[string]any{"ticket": "INC-77"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	completed, err := service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("RunExecutionUntilBlocked returned error: %v", err)
	}
	if completed.Status != ExecutionStatusSucceeded {
		t.Fatalf("expected execution to succeed, got %+v", completed)
	}
	codeNodes := workflowNodeExecutionsByID(completed.NodeExecutions, "shape_payload")
	if len(codeNodes) != 2 {
		t.Fatalf("expected seeded and completed code node records, got %+v", codeNodes)
	}
	codeNode := codeNodes[len(codeNodes)-1]
	if codeNode.Status != NodeStatusSucceeded {
		t.Fatalf("expected code node to succeed, got %+v", codeNode)
	}
	if codeNode.Output["summary"] != "INC-77/high" || codeNode.Output["assignee"] != "ops" {
		t.Fatalf("expected interpolated code outputs, got %#v", codeNode.Output)
	}
	payload, ok := codeNode.Output["payload"].(map[string]any)
	if !ok || payload["ticket"] != "INC-77" || payload["priority"] != "high" {
		t.Fatalf("expected nested code payload output, got %#v", codeNode.Output["payload"])
	}
	notifyNodes := workflowNodeExecutionsByID(completed.NodeExecutions, "notify")
	if len(notifyNodes) != 2 {
		t.Fatalf("expected seeded and completed notify node records, got %+v", notifyNodes)
	}
	if notifyNodes[len(notifyNodes)-1].Input["message"] != "INC-77/high" || notifyNodes[len(notifyNodes)-1].Output["assignee"] != "ops" {
		t.Fatalf("expected downstream interpolation from code output, got input=%#v output=%#v", notifyNodes[len(notifyNodes)-1].Input, notifyNodes[len(notifyNodes)-1].Output)
	}
}

func TestServiceRunReadyNodeExecutesInjectedKnowledgeRetriever(t *testing.T) {
	store := newMemoryWorkflowStore()
	retriever := &fakeWorkflowKnowledgeRetriever{
		results: []knowledge.KnowledgeRetrievalResult{{
			DocumentID:      "doc_1",
			DocumentTitle:   "Deployment Guide",
			ChunkID:         "chunk_1",
			ChunkIndex:      2,
			RetrievalMethod: knowledge.KnowledgeRetrievalModeHybrid,
			Similarity:      0.82,
			Snippet:         "Restart the service after deployment.",
		}},
	}
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		NewKnowledgeNodeExecutor(retriever),
	)))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Knowledge Lookup",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG([]map[string]any{{
			"id":   "lookup",
			"type": "knowledge",
			"input": map[string]any{
				"knowledgeBaseId": "kb_1",
				"query":           "{{input.question}}",
				"mode":            knowledge.KnowledgeRetrievalModeHybrid,
				"limit":           3,
				"minScore":        0.5,
			},
		}}, nil),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		Input:          map[string]any{"question": "How do we deploy?"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	if err := service.RunReadyNode(ctx, "org_1", execution.ID, "lookup"); err != nil {
		t.Fatalf("RunReadyNode knowledge returned error: %v", err)
	}

	if retriever.session.OrganizationID != "org_1" {
		t.Fatalf("expected workflow organization to be passed in session, got %+v", retriever.session)
	}
	if retriever.knowledgeBaseID != "kb_1" {
		t.Fatalf("expected knowledge base id kb_1, got %q", retriever.knowledgeBaseID)
	}
	if retriever.query != "How do we deploy?" {
		t.Fatalf("expected interpolated query, got %q", retriever.query)
	}
	if retriever.options.Mode != knowledge.KnowledgeRetrievalModeHybrid || retriever.options.Limit != 3 || retriever.options.MinScore != 0.5 {
		t.Fatalf("unexpected retrieval options: %+v", retriever.options)
	}

	updated, err := service.GetExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution returned error: %v", err)
	}
	nodes := workflowNodeExecutionsByID(updated.NodeExecutions, "lookup")
	if len(nodes) != 2 {
		t.Fatalf("expected seeded and completed knowledge nodes, got %+v", nodes)
	}
	completed := nodes[len(nodes)-1]
	if completed.Status != NodeStatusSucceeded {
		t.Fatalf("expected knowledge node to succeed, got %+v", completed)
	}
	if completed.Input["query"] != "How do we deploy?" {
		t.Fatalf("expected interpolated query in recorded input, got %#v", completed.Input)
	}
	if completed.Output["query"] != "How do we deploy?" || completed.Output["knowledgeBaseId"] != "kb_1" {
		t.Fatalf("expected query and knowledge base in output, got %#v", completed.Output)
	}
	if completed.Output["count"] != 1 {
		t.Fatalf("expected output count 1, got %#v", completed.Output["count"])
	}
	results, ok := completed.Output["results"].([]knowledge.KnowledgeRetrievalResult)
	if !ok || len(results) != 1 {
		t.Fatalf("expected typed knowledge results in output, got %#v", completed.Output["results"])
	}
	if results[0].DocumentID != "doc_1" || results[0].Snippet == "" {
		t.Fatalf("unexpected knowledge result: %+v", results[0])
	}
}

func TestServiceRunReadyNodeExecutesInjectedLLMGateway(t *testing.T) {
	store := newMemoryWorkflowStore()
	gateway := &fakeWorkflowLLMGateway{
		response: &LLMChatResponse{
			Text:  "deployment summary",
			Model: "gpt-workflow",
			Usage: map[string]any{
				"promptTokens":     12,
				"completionTokens": 4,
			},
			Raw: map[string]any{
				"id":           "chatcmpl_1",
				"finishReason": "stop",
			},
		},
	}
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		NewLLMNodeExecutor(gateway),
	)))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "LLM Summary",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG([]map[string]any{{
			"id":   "summarize",
			"type": "llm",
			"input": map[string]any{
				"data": map[string]any{
					"model":  "gpt-workflow",
					"prompt": "Summarize {{input.ticket}}",
					"messages": []any{
						map[string]any{"role": "system", "content": "Be concise."},
						map[string]any{"role": "user", "content": "Ticket {{input.ticket}}: {{input.issue}}"},
					},
					"options": map[string]any{
						"temperature": 0.2,
						"maxTokens":   128,
					},
				},
			},
		}}, nil),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		Input:          map[string]any{"ticket": "INC-42", "issue": "database latency"},
		Context:        map[string]any{"userId": "user_1", "workspaceId": "workspace_1", "requestId": "req_workflow_1"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	if err := service.RunReadyNode(ctx, "org_1", execution.ID, "summarize"); err != nil {
		t.Fatalf("RunReadyNode llm returned error: %v", err)
	}

	if gateway.request.Model != "gpt-workflow" {
		t.Fatalf("expected model gpt-workflow, got %q", gateway.request.Model)
	}
	if gateway.request.Prompt != "Summarize INC-42" {
		t.Fatalf("expected interpolated prompt, got %q", gateway.request.Prompt)
	}
	if len(gateway.request.Messages) != 2 {
		t.Fatalf("expected two chat messages, got %+v", gateway.request.Messages)
	}
	if gateway.request.Messages[1].Role != "user" || gateway.request.Messages[1].Content != "Ticket INC-42: database latency" {
		t.Fatalf("expected interpolated user message, got %+v", gateway.request.Messages)
	}
	if gateway.request.Options["temperature"] != 0.2 || gateway.request.Options["maxTokens"] != 128 {
		t.Fatalf("expected options to be passed through, got %+v", gateway.request.Options)
	}
	if gateway.request.OrganizationID != "org_1" || gateway.request.FeatureType != "workflow" {
		t.Fatalf("expected workflow llm request attribution, got %+v", gateway.request)
	}
	if gateway.request.UserID != "user_1" || gateway.request.WorkspaceID != "workspace_1" || gateway.request.RequestID != "req_workflow_1" {
		t.Fatalf("expected workflow llm request identity metadata, got %+v", gateway.request)
	}

	updated, err := service.GetExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution returned error: %v", err)
	}
	nodes := workflowNodeExecutionsByID(updated.NodeExecutions, "summarize")
	if len(nodes) != 2 {
		t.Fatalf("expected seeded and completed llm nodes, got %+v", nodes)
	}
	completed := nodes[len(nodes)-1]
	if completed.Status != NodeStatusSucceeded {
		t.Fatalf("expected llm node to succeed, got %+v", completed)
	}
	data, ok := completed.Input["data"].(map[string]any)
	if !ok || data["prompt"] != "Summarize INC-42" {
		t.Fatalf("expected interpolated prompt in recorded input, got %#v", completed.Input)
	}
	if completed.Output["text"] != "deployment summary" || completed.Output["content"] != "deployment summary" {
		t.Fatalf("expected text/content output, got %#v", completed.Output)
	}
	if completed.Output["model"] != "gpt-workflow" {
		t.Fatalf("expected model in output, got %#v", completed.Output)
	}
	usage, ok := completed.Output["usage"].(map[string]any)
	if !ok || usage["promptTokens"] != 12 || usage["completionTokens"] != 4 {
		t.Fatalf("expected usage output, got %#v", completed.Output["usage"])
	}
	raw, ok := completed.Output["raw"].(map[string]any)
	if !ok || raw["id"] != "chatcmpl_1" || raw["finishReason"] != "stop" {
		t.Fatalf("expected useful raw response fields, got %#v", completed.Output["raw"])
	}
}

func TestServiceRunExecutionUntilBlockedPausesAfterLLMTokenBudgetExceeded(t *testing.T) {
	store := newMemoryWorkflowStore()
	notifyRuns := 0
	gateway := &fakeWorkflowLLMGateway{
		response: &LLMChatResponse{
			Text:  "budget heavy response",
			Model: "gpt-workflow",
			Usage: map[string]any{
				"promptTokens":     65,
				"completionTokens": 50,
			},
		},
	}
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		NewLLMNodeExecutor(gateway),
		functionNodeExecutor{
			nodeType: "agent",
			execute: func(context.Context, NodeExecutorInput) (map[string]any, error) {
				notifyRuns++
				return map[string]any{"notified": true}, nil
			},
		},
	)))
	ctx := context.Background()
	definition := workflowDefinitionDAG(
		[]map[string]any{
			{"id": "summarize", "type": "llm", "input": map[string]any{"prompt": "Summarize {{input.ticket}}"}},
			{"id": "notify", "type": "agent"},
		},
		[]map[string]any{{"from": "summarize", "to": "notify"}},
	)
	definition["max_tokens_budget"] = 100
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Budget Guarded LLM",
		Status:         WorkflowStatusPublished,
		Definition:     definition,
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		Input:          map[string]any{"ticket": "INC-99"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	blocked, err := service.RunExecutionUntilBlocked(ctx, "org_1", execution.ID)
	if !errors.Is(err, ErrWorkflowResourceLimit) {
		t.Fatalf("RunExecutionUntilBlocked err=%v, want ErrWorkflowResourceLimit", err)
	}
	if blocked.Status != ExecutionStatusPaused || blocked.CompletedAt != nil {
		t.Fatalf("expected token budget to pause execution without completion time, got %+v", blocked)
	}
	if notifyRuns != 0 {
		t.Fatalf("downstream notify must not run after token budget pause, got %d runs", notifyRuns)
	}
	notifyNodes := workflowNodeExecutionsByID(blocked.NodeExecutions, "notify")
	for _, node := range notifyNodes {
		if node.Status == NodeStatusRunning || isSuccessfulNodeStatus(node.Status) {
			t.Fatalf("expected notify node not to run after token budget pause, got %+v", notifyNodes)
		}
	}
}

func TestServiceRegisterNodeExecutorsOverridesDefaultLLMExecutor(t *testing.T) {
	store := newMemoryWorkflowStore()
	service := NewService(store)
	gateway := &fakeWorkflowLLMGateway{
		response: &LLMChatResponse{
			Text:  "registered executor response",
			Model: "gpt-4o-mini",
		},
	}
	service.RegisterNodeExecutors(NewLLMNodeExecutor(gateway))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Registered LLM",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG([]map[string]any{{
			"id":   "generate",
			"type": "llm",
			"input": map[string]any{
				"model":  "gpt-4o-mini",
				"prompt": "{{input.question}}",
			},
		}}, nil),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		Input:          map[string]any{"question": "Summarize the ticket"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	if err := service.RunReadyNode(ctx, "org_1", execution.ID, "generate"); err != nil {
		t.Fatalf("RunReadyNode llm returned error: %v", err)
	}

	if gateway.request.Prompt != "Summarize the ticket" {
		t.Fatalf("expected registered llm executor to call gateway, got request %+v", gateway.request)
	}
	updated, err := service.GetExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution returned error: %v", err)
	}
	nodes := workflowNodeExecutionsByID(updated.NodeExecutions, "generate")
	if len(nodes) != 2 {
		t.Fatalf("expected seeded and completed llm nodes, got %+v", nodes)
	}
	if nodes[len(nodes)-1].Output["text"] != "registered executor response" {
		t.Fatalf("expected real llm executor output, got %#v", nodes[len(nodes)-1].Output)
	}
}

func errorMessage(errorValue map[string]any) string {
	if message, ok := errorValue["message"].(string); ok {
		return message
	}
	return ""
}

type fakeWorkflowKnowledgeRetriever struct {
	results         []knowledge.KnowledgeRetrievalResult
	session         auth.Session
	knowledgeBaseID string
	query           string
	options         knowledge.KnowledgeRetrievalOptions
}

func (f *fakeWorkflowKnowledgeRetriever) RetrieveWithOptions(ctx context.Context, session auth.Session, knowledgeBaseID, query string, options knowledge.KnowledgeRetrievalOptions) ([]knowledge.KnowledgeRetrievalResult, error) {
	f.session = session
	f.knowledgeBaseID = knowledgeBaseID
	f.query = query
	f.options = options
	return f.results, nil
}

type fakeWorkflowLLMGateway struct {
	request  LLMChatRequest
	response *LLMChatResponse
}

func (f *fakeWorkflowLLMGateway) Chat(ctx context.Context, request LLMChatRequest) (*LLMChatResponse, error) {
	f.request = request
	return f.response, nil
}
