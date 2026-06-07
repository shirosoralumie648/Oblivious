package workflow

import (
	"context"
	"errors"
	"testing"
)

func TestRPANodeExecutorRunsBrowserAutomationWithScopedRequest(t *testing.T) {
	runner := &recordingWorkflowRPARunner{
		result: &WorkflowRPAResult{
			FinalURL: "https://example.test/tickets/INC-42",
			Screenshot: &WorkflowRPAArtifact{
				ContentType: "image/png",
				URL:         "artifact://screenshots/rpa_1.png",
			},
			Steps: []WorkflowRPAStepResult{{
				Action: "goto",
				Status: "succeeded",
			}},
			Output: map[string]any{"ticketStatus": "open"},
		},
	}
	executor := NewRPANodeExecutor(runner)

	output, err := executor.Execute(context.Background(), NodeExecutorInput{
		Execution: &WorkflowExecution{
			OrganizationID: "org_1",
			Context: map[string]any{
				"userId":      "user_1",
				"workspaceId": "workspace_1",
			},
		},
		Input: map[string]any{
			"targetUrl":   "https://example.test/tickets/INC-42",
			"timeoutMs":   float64(45000),
			"screenshot":  true,
			"browserMode": "headless",
			"steps": []any{
				map[string]any{"action": "goto", "value": "https://example.test/tickets/INC-42"},
				map[string]any{"action": "click", "selector": "#refresh"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if runner.request.OrganizationID != "org_1" || runner.request.UserID != "user_1" || runner.request.WorkspaceID != "workspace_1" {
		t.Fatalf("unexpected RPA request scope: %+v", runner.request)
	}
	if runner.request.TargetURL != "https://example.test/tickets/INC-42" || runner.request.TimeoutMS != 45000 {
		t.Fatalf("unexpected RPA request target/timeout: %+v", runner.request)
	}
	if !runner.request.Screenshot || runner.request.BrowserMode != "headless" {
		t.Fatalf("unexpected RPA browser controls: %+v", runner.request)
	}
	if len(runner.request.Steps) != 2 || runner.request.Steps[1].Action != "click" || runner.request.Steps[1].Selector != "#refresh" {
		t.Fatalf("unexpected RPA request steps: %+v", runner.request.Steps)
	}
	if output["finalUrl"] != "https://example.test/tickets/INC-42" {
		t.Fatalf("expected final URL in output, got %#v", output)
	}
	if screenshot, ok := output["screenshot"].(map[string]any); !ok || screenshot["url"] != "artifact://screenshots/rpa_1.png" {
		t.Fatalf("expected screenshot artifact in output, got %#v", output["screenshot"])
	}
	if resultOutput, ok := output["output"].(map[string]any); !ok || resultOutput["ticketStatus"] != "open" {
		t.Fatalf("expected runner output in RPA output, got %#v", output["output"])
	}
}

func TestRPANodeExecutorRequiresRunnerAndTarget(t *testing.T) {
	tests := []struct {
		name     string
		executor NodeExecutor
		input    map[string]any
	}{
		{
			name:     "missing runner",
			executor: NewRPANodeExecutor(nil),
			input:    map[string]any{"targetUrl": "https://example.test"},
		},
		{
			name:     "missing target",
			executor: NewRPANodeExecutor(&recordingWorkflowRPARunner{}),
			input:    map[string]any{"steps": []any{map[string]any{"action": "goto"}}},
		},
		{
			name:     "invalid steps",
			executor: NewRPANodeExecutor(&recordingWorkflowRPARunner{}),
			input:    map[string]any{"targetUrl": "https://example.test", "steps": "goto"},
		},
		{
			name:     "invalid target url",
			executor: NewRPANodeExecutor(&recordingWorkflowRPARunner{}),
			input:    map[string]any{"targetUrl": "javascript:alert(1)"},
		},
		{
			name:     "unsupported step action",
			executor: NewRPANodeExecutor(&recordingWorkflowRPARunner{}),
			input: map[string]any{
				"targetUrl": "https://example.test",
				"steps":     []any{map[string]any{"action": "dance"}},
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

func TestServiceRunReadyNodeExecutesRPANode(t *testing.T) {
	store := newMemoryWorkflowStore()
	runner := &recordingWorkflowRPARunner{
		result: &WorkflowRPAResult{
			FinalURL: "https://example.test/tickets/INC-42",
			Output:   map[string]any{"refreshed": true},
			Steps:    []WorkflowRPAStepResult{{Action: "goto", Status: "succeeded"}},
		},
	}
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		NewRPANodeExecutor(runner),
	)))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "RPA Ticket Flow",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG([]map[string]any{{
			"id":   "refresh_ticket",
			"type": "rpa",
			"input": map[string]any{
				"targetUrl": "https://example.test/tickets/{{input.ticketId}}",
				"steps": []any{
					map[string]any{"action": "goto", "value": "https://example.test/tickets/{{input.ticketId}}"},
				},
				"timeoutMs": 30000,
			},
		}}, nil),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		Input:          map[string]any{"ticketId": "INC-42"},
		Context:        map[string]any{"userId": "user_1", "workspaceId": "workspace_1"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	if err := service.RunReadyNode(ctx, "org_1", execution.ID, "refresh_ticket"); err != nil {
		t.Fatalf("RunReadyNode rpa returned error: %v", err)
	}

	if runner.request.TargetURL != "https://example.test/tickets/INC-42" || len(runner.request.Steps) != 1 || runner.request.Steps[0].Value != "https://example.test/tickets/INC-42" {
		t.Fatalf("expected interpolated RPA target and step value, got %+v", runner.request)
	}
	updated, err := service.GetExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution returned error: %v", err)
	}
	nodes := workflowNodeExecutionsByID(updated.NodeExecutions, "refresh_ticket")
	if len(nodes) != 2 || nodes[len(nodes)-1].Status != NodeStatusSucceeded {
		t.Fatalf("expected succeeded RPA node, got %+v", nodes)
	}
	if nodes[len(nodes)-1].Output["finalUrl"] != "https://example.test/tickets/INC-42" {
		t.Fatalf("expected final URL in node output, got %#v", nodes[len(nodes)-1].Output)
	}
}

type recordingWorkflowRPARunner struct {
	request WorkflowRPARequest
	result  *WorkflowRPAResult
	err     error
}

func (r *recordingWorkflowRPARunner) RunRPA(ctx context.Context, req WorkflowRPARequest) (*WorkflowRPAResult, error) {
	_ = ctx
	r.request = req
	if r.err != nil {
		return nil, r.err
	}
	return r.result, nil
}
