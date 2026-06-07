package workflow

import (
	"context"
	"errors"
	"testing"
)

func TestCodeNodeExecutorRunsCodeWithScopedRequest(t *testing.T) {
	runner := &recordingWorkflowCodeRunner{
		result: &WorkflowCodeResult{
			Output: map[string]any{"total": float64(42)},
			Logs:   []string{"calculated total"},
			Raw:    map[string]any{"durationMs": float64(8)},
		},
	}
	executor := NewCodeNodeExecutor(WithCodeRunner(runner))

	output, err := executor.Execute(context.Background(), NodeExecutorInput{
		Execution: &WorkflowExecution{
			OrganizationID: "org_1",
			Context: map[string]any{
				"userId":      "user_1",
				"workspaceId": "workspace_1",
			},
		},
		Input: map[string]any{
			"language":  "javascript",
			"code":      "return { total: inputs.price * inputs.count }",
			"timeoutMs": float64(1500),
			"inputs": map[string]any{
				"price": float64(6),
				"count": float64(7),
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if runner.request.OrganizationID != "org_1" || runner.request.UserID != "user_1" || runner.request.WorkspaceID != "workspace_1" {
		t.Fatalf("unexpected code request scope: %+v", runner.request)
	}
	if runner.request.Language != "javascript" || runner.request.Code == "" || runner.request.TimeoutMS != 1500 {
		t.Fatalf("unexpected code request controls: %+v", runner.request)
	}
	if runner.request.Inputs["count"] != float64(7) {
		t.Fatalf("unexpected code inputs: %+v", runner.request.Inputs)
	}
	if output["total"] != float64(42) {
		t.Fatalf("expected runner output to be flattened, got %#v", output)
	}
	if logs, ok := output["logs"].([]string); !ok || len(logs) != 1 || logs[0] != "calculated total" {
		t.Fatalf("expected logs in output, got %#v", output["logs"])
	}
}

func TestCodeNodeExecutorRequiresRunnerAndCodeWhenExecuting(t *testing.T) {
	tests := []struct {
		name     string
		executor NodeExecutor
		input    map[string]any
	}{
		{
			name:     "missing runner",
			executor: NewCodeNodeExecutor(),
			input:    map[string]any{"language": "javascript", "code": "return {}"},
		},
		{
			name:     "missing code",
			executor: NewCodeNodeExecutor(WithCodeRunner(&recordingWorkflowCodeRunner{})),
			input:    map[string]any{"language": "javascript"},
		},
		{
			name:     "invalid inputs",
			executor: NewCodeNodeExecutor(WithCodeRunner(&recordingWorkflowCodeRunner{})),
			input:    map[string]any{"language": "javascript", "code": "return {}", "inputs": "not object"},
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

func TestCodeNodeExecutorKeepsStaticOutputsCompatibility(t *testing.T) {
	output, err := NewCodeNodeExecutor().Execute(context.Background(), NodeExecutorInput{
		Input: map[string]any{
			"outputs": map[string]any{"summary": "INC-42/high"},
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if output["summary"] != "INC-42/high" {
		t.Fatalf("expected static outputs compatibility, got %#v", output)
	}
}

func TestServiceRunReadyNodeExecutesCodeRunnerNode(t *testing.T) {
	store := newMemoryWorkflowStore()
	runner := &recordingWorkflowCodeRunner{
		result: &WorkflowCodeResult{
			Output: map[string]any{"summary": "INC-42/high"},
		},
	}
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		NewCodeNodeExecutor(WithCodeRunner(runner)),
	)))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Code Runner Flow",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG([]map[string]any{{
			"id":   "shape_payload",
			"type": "code",
			"input": map[string]any{
				"language": "javascript",
				"code":     "return { summary: inputs.ticket + '/' + inputs.priority }",
				"inputs": map[string]any{
					"ticket":   "{{input.ticket}}",
					"priority": "{{input.priority}}",
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
		Input:          map[string]any{"ticket": "INC-42", "priority": "high"},
		Context:        map[string]any{"userId": "user_1", "workspaceId": "workspace_1"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	if err := service.RunReadyNode(ctx, "org_1", execution.ID, "shape_payload"); err != nil {
		t.Fatalf("RunReadyNode code returned error: %v", err)
	}

	if runner.request.Inputs["ticket"] != "INC-42" || runner.request.Inputs["priority"] != "high" {
		t.Fatalf("expected interpolated code inputs, got %+v", runner.request.Inputs)
	}
	updated, err := service.GetExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution returned error: %v", err)
	}
	nodes := workflowNodeExecutionsByID(updated.NodeExecutions, "shape_payload")
	if len(nodes) != 2 || nodes[len(nodes)-1].Status != NodeStatusSucceeded {
		t.Fatalf("expected succeeded code node, got %+v", nodes)
	}
	if nodes[len(nodes)-1].Output["summary"] != "INC-42/high" {
		t.Fatalf("expected code summary in node output, got %#v", nodes[len(nodes)-1].Output)
	}
}

type recordingWorkflowCodeRunner struct {
	request WorkflowCodeRequest
	result  *WorkflowCodeResult
	err     error
}

func (r *recordingWorkflowCodeRunner) RunWorkflowCode(ctx context.Context, req WorkflowCodeRequest) (*WorkflowCodeResult, error) {
	_ = ctx
	r.request = req
	if r.err != nil {
		return nil, r.err
	}
	return r.result, nil
}
