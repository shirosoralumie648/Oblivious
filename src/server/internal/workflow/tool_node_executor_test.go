package workflow

import (
	"context"
	"errors"
	"testing"
)

func TestToolNodeExecutorRunsToolWithScopedRequest(t *testing.T) {
	runner := &recordingWorkflowToolRunner{
		result: &WorkflowToolResult{
			Content: "Result: 42",
			IsError: false,
			Output:  map[string]any{"value": float64(42)},
			Raw:     map[string]any{"durationMs": float64(12)},
		},
	}
	executor := NewToolNodeExecutor(runner)

	output, err := executor.Execute(context.Background(), NodeExecutorInput{
		Execution: &WorkflowExecution{
			OrganizationID: "org_1",
			Context: map[string]any{
				"userId":      "user_1",
				"workspaceId": "workspace_1",
			},
		},
		Input: map[string]any{
			"toolName": "calculator",
			"toolType": "builtin",
			"arguments": map[string]any{
				"expression": "6 * 7",
			},
			"serverId": "server_1",
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if runner.request.OrganizationID != "org_1" || runner.request.UserID != "user_1" || runner.request.WorkspaceID != "workspace_1" {
		t.Fatalf("unexpected tool request scope: %+v", runner.request)
	}
	if runner.request.ToolName != "calculator" || runner.request.ToolType != "builtin" || runner.request.ServerID != "server_1" {
		t.Fatalf("unexpected tool identity: %+v", runner.request)
	}
	if runner.request.Arguments["expression"] != "6 * 7" {
		t.Fatalf("unexpected tool arguments: %+v", runner.request.Arguments)
	}
	if output["content"] != "Result: 42" || output["isError"] != false {
		t.Fatalf("unexpected tool output: %#v", output)
	}
	if structured, ok := output["output"].(map[string]any); !ok || structured["value"] != float64(42) {
		t.Fatalf("expected structured output, got %#v", output["output"])
	}
}

func TestToolNodeExecutorRequiresRunnerAndToolName(t *testing.T) {
	tests := []struct {
		name     string
		executor NodeExecutor
		input    map[string]any
	}{
		{
			name:     "missing runner",
			executor: NewToolNodeExecutor(nil),
			input:    map[string]any{"toolName": "calculator"},
		},
		{
			name:     "missing tool name",
			executor: NewToolNodeExecutor(&recordingWorkflowToolRunner{}),
			input:    map[string]any{"arguments": map[string]any{"expression": "1+1"}},
		},
		{
			name:     "invalid arguments",
			executor: NewToolNodeExecutor(&recordingWorkflowToolRunner{}),
			input:    map[string]any{"toolName": "calculator", "arguments": "1+1"},
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

func TestServiceRunReadyNodeExecutesToolNode(t *testing.T) {
	store := newMemoryWorkflowStore()
	runner := &recordingWorkflowToolRunner{
		result: &WorkflowToolResult{
			Content: "Result: 42",
			Output:  map[string]any{"value": float64(42)},
		},
	}
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		NewToolNodeExecutor(runner),
	)))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Tool Flow",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG([]map[string]any{{
			"id":   "calculate_total",
			"type": "tool",
			"input": map[string]any{
				"toolName": "calculator",
				"arguments": map[string]any{
					"expression": "{{input.expression}}",
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
		Input:          map[string]any{"expression": "6 * 7"},
		Context:        map[string]any{"userId": "user_1", "workspaceId": "workspace_1"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	if err := service.RunReadyNode(ctx, "org_1", execution.ID, "calculate_total"); err != nil {
		t.Fatalf("RunReadyNode tool returned error: %v", err)
	}

	if runner.request.ToolName != "calculator" || runner.request.Arguments["expression"] != "6 * 7" {
		t.Fatalf("expected interpolated tool request, got %+v", runner.request)
	}
	updated, err := service.GetExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution returned error: %v", err)
	}
	nodes := workflowNodeExecutionsByID(updated.NodeExecutions, "calculate_total")
	if len(nodes) != 2 || nodes[len(nodes)-1].Status != NodeStatusSucceeded {
		t.Fatalf("expected succeeded tool node, got %+v", nodes)
	}
	if nodes[len(nodes)-1].Output["content"] != "Result: 42" {
		t.Fatalf("expected tool content in node output, got %#v", nodes[len(nodes)-1].Output)
	}
}

type recordingWorkflowToolRunner struct {
	request WorkflowToolRequest
	result  *WorkflowToolResult
	err     error
}

func (r *recordingWorkflowToolRunner) RunWorkflowTool(ctx context.Context, req WorkflowToolRequest) (*WorkflowToolResult, error) {
	_ = ctx
	r.request = req
	if r.err != nil {
		return nil, r.err
	}
	return r.result, nil
}
