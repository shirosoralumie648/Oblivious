package workflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServiceRunReadyNodeExecutesHTTPNode(t *testing.T) {
	var receivedMethod string
	var receivedPath string
	var receivedBody map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		receivedPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&receivedBody); err != nil {
			t.Fatalf("decode upstream request body: %v", err)
		}
		w.Header().Set("X-Workflow-Test", "ok")
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"accepted":true,"ticket":"INC-9"}`))
	}))
	t.Cleanup(upstream.Close)

	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "HTTP Flow",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG(
			[]map[string]any{
				{"id": "start", "type": "start"},
				{"id": "call_api", "type": "http", "input": map[string]any{
					"method": "POST",
					"url":    upstream.URL + "/tickets",
					"headers": map[string]any{
						"X-Trace": "{{input.trace}}",
					},
					"body": map[string]any{
						"ticket": "{{input.ticket}}",
					},
				}},
			},
			[]map[string]any{{"from": "start", "to": "call_api"}},
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		Input:          map[string]any{"ticket": "INC-9", "trace": "trace-1"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}
	if err := service.RunReadyNode(ctx, "org_1", execution.ID, "start"); err != nil {
		t.Fatalf("RunReadyNode start returned error: %v", err)
	}

	if err := service.RunReadyNode(ctx, "org_1", execution.ID, "call_api"); err != nil {
		t.Fatalf("RunReadyNode http returned error: %v", err)
	}

	if receivedMethod != http.MethodPost || receivedPath != "/tickets" {
		t.Fatalf("expected POST /tickets upstream request, got %s %s", receivedMethod, receivedPath)
	}
	if receivedBody["ticket"] != "INC-9" {
		t.Fatalf("expected interpolated request body, got %+v", receivedBody)
	}

	updated, err := service.GetExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution returned error: %v", err)
	}
	httpNodes := workflowNodeExecutionsByID(updated.NodeExecutions, "call_api")
	if len(httpNodes) != 2 {
		t.Fatalf("expected seeded and completed http node, got %+v", httpNodes)
	}
	completed := httpNodes[len(httpNodes)-1]
	if completed.Status != NodeStatusSucceeded {
		t.Fatalf("expected http node success, got %+v", completed)
	}
	if completed.Output["statusCode"] != float64(http.StatusAccepted) && completed.Output["statusCode"] != http.StatusAccepted {
		t.Fatalf("expected statusCode 202 in output, got %#v", completed.Output)
	}
	if completed.Output["body"] != `{"accepted":true,"ticket":"INC-9"}` {
		t.Fatalf("expected raw response body, got %#v", completed.Output["body"])
	}
	headers, ok := completed.Output["headers"].(map[string]any)
	if !ok || headers["X-Workflow-Test"] == nil {
		t.Fatalf("expected response headers in output, got %#v", completed.Output["headers"])
	}
}

func TestServiceRunReadyNodeRecordsHTTPNodeFailure(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"upstream unavailable"}`))
	}))
	t.Cleanup(upstream.Close)

	store := newMemoryWorkflowStore()
	service := NewService(store)
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "HTTP Failure Flow",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG(
			[]map[string]any{{"id": "call_api", "type": "http", "input": map[string]any{
				"method": "GET",
				"url":    upstream.URL + "/fail",
			}}},
			nil,
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, StartExecutionRequest{OrganizationID: "org_1", WorkflowID: workflow.ID})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}

	err = service.RunReadyNode(ctx, "org_1", execution.ID, "call_api")
	if err == nil || !strings.Contains(err.Error(), "502") {
		t.Fatalf("RunReadyNode http failure err=%v, want status 502 error", err)
	}

	updated, err := service.GetExecution(ctx, "org_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution returned error: %v", err)
	}
	httpNodes := workflowNodeExecutionsByID(updated.NodeExecutions, "call_api")
	if len(httpNodes) != 2 || httpNodes[len(httpNodes)-1].Status != NodeStatusFailed {
		t.Fatalf("expected failed http node to be recorded, got %+v", httpNodes)
	}
	if updated.Status != ExecutionStatusPaused {
		t.Fatalf("expected failed http node to pause execution, got %s", updated.Status)
	}
	if !strings.Contains(errorMessage(httpNodes[len(httpNodes)-1].Error), "502") {
		t.Fatalf("expected failure status in node error, got %#v", httpNodes[len(httpNodes)-1].Error)
	}
}

func TestServiceTestNodeExecutesHTTPNode(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"dryRun":true}`))
	}))
	t.Cleanup(upstream.Close)

	service := NewService(newMemoryWorkflowStore())
	ctx := context.Background()
	workflow, err := service.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "HTTP Test Node Flow",
		Status:         WorkflowStatusPublished,
		Definition: workflowDefinitionDAG(
			[]map[string]any{{"id": "call_api", "type": "http", "input": map[string]any{
				"method": "POST",
				"url":    upstream.URL + "/dry-run",
				"body":   map[string]any{"ticket": "{{input.ticket}}"},
			}}},
			nil,
		),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	result, err := service.TestNode(ctx, TestNodeRequest{
		OrganizationID: "org_1",
		WorkflowID:     workflow.ID,
		NodeID:         "call_api",
		Input:          map[string]any{"ticket": "INC-10"},
	})
	if err != nil {
		t.Fatalf("TestNode returned error: %v", err)
	}
	if result.Status != ExecutionStatusSucceeded {
		t.Fatalf("expected test node success, got %+v", result)
	}
	if result.Output["statusCode"] != http.StatusOK {
		t.Fatalf("expected HTTP test node status output, got %+v", result.Output)
	}
	if result.Output["body"] != `{"dryRun":true}` {
		t.Fatalf("expected HTTP test node response body, got %+v", result.Output)
	}
}
