package workflow

import (
	"context"
	"errors"
	"testing"
)

func TestDatabaseNodeExecutorRunsQueryWithScopedRequest(t *testing.T) {
	runner := &recordingWorkflowDatabaseRunner{
		result: &WorkflowDatabaseResult{
			Rows: []map[string]any{{
				"id":       "ticket_1",
				"priority": "high",
			}},
			RowsAffected: 1,
		},
	}
	executor := NewDatabaseNodeExecutor(runner)

	output, err := executor.Execute(context.Background(), NodeExecutorInput{
		Execution: &WorkflowExecution{
			OrganizationID: "org_1",
			Context: map[string]any{
				"userId":      "user_1",
				"workspaceId": "workspace_1",
			},
		},
		Input: map[string]any{
			"connectionId": "primary_reporting",
			"query":        "select id, priority from tickets where id = $1",
			"parameters":   []any{"ticket_1"},
			"limit":        float64(20),
			"readOnly":     true,
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	if runner.request.OrganizationID != "org_1" || runner.request.UserID != "user_1" || runner.request.WorkspaceID != "workspace_1" {
		t.Fatalf("unexpected database request scope: %+v", runner.request)
	}
	if runner.request.ConnectionID != "primary_reporting" || runner.request.Query != "select id, priority from tickets where id = $1" {
		t.Fatalf("unexpected database request query: %+v", runner.request)
	}
	if len(runner.request.Parameters) != 1 || runner.request.Parameters[0] != "ticket_1" {
		t.Fatalf("unexpected database request parameters: %+v", runner.request.Parameters)
	}
	if runner.request.Limit != 20 || !runner.request.ReadOnly {
		t.Fatalf("unexpected database request controls: %+v", runner.request)
	}
	if output["rowCount"] != 1 || output["rowsAffected"] != int64(1) {
		t.Fatalf("unexpected database output counts: %#v", output)
	}
	rows, ok := output["rows"].([]map[string]any)
	if !ok || len(rows) != 1 || rows[0]["priority"] != "high" {
		t.Fatalf("expected query rows in output, got %#v", output["rows"])
	}
}

func TestDatabaseNodeExecutorRequiresRunnerAndQuery(t *testing.T) {
	tests := []struct {
		name     string
		executor NodeExecutor
		input    map[string]any
	}{
		{
			name:     "missing runner",
			executor: NewDatabaseNodeExecutor(nil),
			input:    map[string]any{"query": "select 1"},
		},
		{
			name:     "missing query",
			executor: NewDatabaseNodeExecutor(&recordingWorkflowDatabaseRunner{}),
			input:    map[string]any{"connectionId": "primary_reporting"},
		},
		{
			name:     "blank query",
			executor: NewDatabaseNodeExecutor(&recordingWorkflowDatabaseRunner{}),
			input:    map[string]any{"query": "   "},
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

func TestServiceRunReadyNodeExecutesDatabaseNode(t *testing.T) {
	store := newMemoryWorkflowStore()
	runner := &recordingWorkflowDatabaseRunner{
		result: &WorkflowDatabaseResult{
			Rows:         []map[string]any{{"id": "INC-42", "status": "open"}},
			RowsAffected: 1,
		},
	}
	service := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(
		NewDatabaseNodeExecutor(runner),
	)))
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Database Lookup Flow",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG([]map[string]any{{
			"id":   "lookup_ticket",
			"type": "database",
			"input": map[string]any{
				"query":      "select id, status from tickets where id = $1",
				"parameters": []any{"{{input.ticketId}}"},
				"limit":      5,
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

	if err := service.RunReadyNode(ctx, "org_1", execution.ID, "lookup_ticket"); err != nil {
		t.Fatalf("RunReadyNode database returned error: %v", err)
	}

	if len(runner.request.Parameters) != 1 || runner.request.Parameters[0] != "INC-42" {
		t.Fatalf("expected interpolated query parameter, got %+v", runner.request.Parameters)
	}
	updated, err := service.GetExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution returned error: %v", err)
	}
	nodes := workflowNodeExecutionsByID(updated.NodeExecutions, "lookup_ticket")
	if len(nodes) != 2 || nodes[len(nodes)-1].Status != NodeStatusSucceeded {
		t.Fatalf("expected succeeded database node, got %+v", nodes)
	}
	completed := nodes[len(nodes)-1]
	if completed.Output["rowCount"] != 1 {
		t.Fatalf("expected row count in database output, got %#v", completed.Output)
	}
}

type recordingWorkflowDatabaseRunner struct {
	request WorkflowDatabaseRequest
	result  *WorkflowDatabaseResult
	err     error
}

func (r *recordingWorkflowDatabaseRunner) RunDatabaseQuery(ctx context.Context, req WorkflowDatabaseRequest) (*WorkflowDatabaseResult, error) {
	_ = ctx
	r.request = req
	if r.err != nil {
		return nil, r.err
	}
	return r.result, nil
}
