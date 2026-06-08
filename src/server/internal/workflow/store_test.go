package workflow

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func testWorkflowSQLStore(t *testing.T) (*SQLStore, *sql.DB, context.Context) {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		if strings.EqualFold(os.Getenv("OBLIVIOUS_REQUIRE_TEST_DATABASE"), "true") {
			t.Fatal("TEST_DATABASE_URL is required for DB-backed workflow tests")
		}
		t.Skip("TEST_DATABASE_URL is required for DB-backed workflow tests")
	}

	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	// Pin to a single connection so the advisory lock is held for the
	// lifetime of the test and cannot be bypassed by the connection pool.
	database.SetMaxOpenConns(1)
	database.SetMaxIdleConns(1)
	if err := database.Ping(); err != nil {
		t.Fatalf("ping database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})

	if _, err := database.Exec(`SELECT pg_advisory_lock(104242)`); err != nil {
		t.Fatalf("lock workflow test database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := database.Exec(`SELECT pg_advisory_unlock(104242)`); err != nil {
			t.Fatalf("unlock workflow test database: %v", err)
		}
	})

	statements := []string{
		`DROP TABLE IF EXISTS workflow_node_executions CASCADE`,
		`DROP TABLE IF EXISTS workflow_executions CASCADE`,
		`DROP TABLE IF EXISTS workflows CASCADE`,
		`DROP TABLE IF EXISTS organizations CASCADE`,
		`CREATE TABLE organizations (id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'active', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`INSERT INTO organizations (id, slug, name) VALUES ('org_workflow_1', 'workflow-org-1', 'Workflow Org 1'), ('org_workflow_2', 'workflow-org-2', 'Workflow Org 2')`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("prepare workflow database: %v\nstatement: %s", err, statement)
		}
	}

	migration, err := os.ReadFile("../../migrations/0042_workflows.sql")
	if err != nil {
		t.Fatalf("read workflow migration: %v", err)
	}
	if _, err := database.Exec(string(migration)); err != nil {
		t.Fatalf("apply workflow migration: %v", err)
	}

	return NewSQLStore(database), database, context.Background()
}

func TestWorkflowStorePersistsDefinitionsAndExecutions(t *testing.T) {
	store, database, ctx := testWorkflowSQLStore(t)

	created, err := store.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_workflow_1",
		Name:           "Customer Onboarding",
		Description:    "Provision account and notify owner",
		Status:         WorkflowStatusPublished,
		Version:        2,
		Definition: map[string]any{
			"nodes": []any{
				map[string]any{"id": "start", "type": "trigger"},
				map[string]any{"id": "notify", "type": "agent"},
			},
			"edges": []any{
				map[string]any{"from": "start", "to": "notify"},
			},
		},
		Variables: map[string]any{"priority": "high", "retry_count": float64(3)},
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	if !strings.HasPrefix(created.ID, "workflow_") {
		t.Fatalf("expected workflow_ id, got %q", created.ID)
	}
	if created.Status != WorkflowStatusPublished || created.Version != 2 {
		t.Fatalf("unexpected workflow status/version: %+v", created)
	}
	if got := created.Definition["nodes"].([]any)[1].(map[string]any)["type"]; got != "agent" {
		t.Fatalf("expected definition JSON to round trip, got %#v", created.Definition)
	}
	if created.Variables["priority"] != "high" || created.Variables["retry_count"] != float64(3) {
		t.Fatalf("expected variables JSON to round trip, got %#v", created.Variables)
	}

	if _, err := store.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_workflow_2",
		Name:           "Other Tenant Workflow",
		Definition:     map[string]any{"nodes": []any{}},
	}); err != nil {
		t.Fatalf("CreateWorkflow for other tenant returned error: %v", err)
	}

	got, err := store.GetWorkflow(ctx, "org_workflow_1", created.ID)
	if err != nil {
		t.Fatalf("GetWorkflow returned error: %v", err)
	}
	if got == nil || got.ID != created.ID || got.OrganizationID != "org_workflow_1" {
		t.Fatalf("unexpected workflow from GetWorkflow: %+v", got)
	}
	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("expected workflow timestamps: %+v", got)
	}
	if crossTenant, err := store.GetWorkflow(ctx, "org_workflow_2", created.ID); err != nil || crossTenant != nil {
		t.Fatalf("cross-tenant GetWorkflow got workflow=%+v err=%v, want nil nil", crossTenant, err)
	}

	workflows, err := store.ListWorkflows(ctx, "org_workflow_1")
	if err != nil {
		t.Fatalf("ListWorkflows returned error: %v", err)
	}
	if len(workflows) != 1 || workflows[0].ID != created.ID {
		t.Fatalf("expected one tenant-scoped workflow, got %+v", workflows)
	}

	startedAt := time.Date(2026, 6, 4, 8, 0, 0, 0, time.UTC)
	completedAt := startedAt.Add(3 * time.Second)
	execution, err := store.CreateExecution(ctx, CreateExecutionRequest{
		OrganizationID: "org_workflow_1",
		WorkflowID:     created.ID,
		Status:         ExecutionStatusFailed,
		Input:          map[string]any{"customer_id": "cus_123"},
		Output:         map[string]any{"notified": false},
		Error:          map[string]any{"message": "provider unavailable"},
		Context:        map[string]any{"trace_id": "trace_123"},
		StartedAt:      startedAt,
		CompletedAt:    &completedAt,
		NodeExecutions: []CreateNodeExecutionRequest{{
			NodeID:      "notify",
			NodeType:    "agent",
			Status:      NodeStatusFailed,
			Attempt:     2,
			Input:       map[string]any{"template": "welcome"},
			Output:      map[string]any{"sent": false},
			Error:       map[string]any{"message": "provider unavailable"},
			StartedAt:   startedAt.Add(time.Second),
			CompletedAt: &completedAt,
		}},
	})
	if err != nil {
		t.Fatalf("CreateExecution returned error: %v", err)
	}
	if !strings.HasPrefix(execution.ID, "wexec_") {
		t.Fatalf("expected wexec_ id, got %q", execution.ID)
	}
	if len(execution.NodeExecutions) != 1 || !strings.HasPrefix(execution.NodeExecutions[0].ID, "wnode_") {
		t.Fatalf("expected persisted node execution with wnode_ id, got %+v", execution.NodeExecutions)
	}
	if execution.Input["customer_id"] != "cus_123" || execution.Context["trace_id"] != "trace_123" {
		t.Fatalf("expected execution JSON to round trip, got %+v", execution)
	}
	if execution.Error["message"] != "provider unavailable" {
		t.Fatalf("expected JSON error to round trip, got %#v", execution.Error)
	}

	var nodeRows int
	if err := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM workflow_node_executions WHERE organization_id = $1 AND execution_id = $2`, "org_workflow_1", execution.ID).Scan(&nodeRows); err != nil {
		t.Fatalf("count workflow node executions: %v", err)
	}
	if nodeRows != 1 {
		t.Fatalf("expected one node execution row, got %d", nodeRows)
	}

	gotExecution, err := store.GetExecution(ctx, "org_workflow_1", execution.ID)
	if err != nil {
		t.Fatalf("GetExecution returned error: %v", err)
	}
	if gotExecution == nil || gotExecution.ID != execution.ID || gotExecution.WorkflowID != created.ID {
		t.Fatalf("unexpected execution from GetExecution: %+v", gotExecution)
	}
	if len(gotExecution.NodeExecutions) != 1 || gotExecution.NodeExecutions[0].NodeID != "notify" {
		t.Fatalf("expected node executions to load with execution, got %+v", gotExecution.NodeExecutions)
	}
	if gotExecution.NodeExecutions[0].Error["message"] != "provider unavailable" {
		t.Fatalf("expected node error JSON to round trip, got %#v", gotExecution.NodeExecutions[0].Error)
	}

	executions, err := store.ListExecutions(ctx, "org_workflow_1", created.ID)
	if err != nil {
		t.Fatalf("ListExecutions returned error: %v", err)
	}
	if len(executions) != 1 || executions[0].ID != execution.ID {
		t.Fatalf("expected one workflow execution, got %+v", executions)
	}
	if crossTenant, err := store.GetExecution(ctx, "org_workflow_2", execution.ID); err != nil || crossTenant != nil {
		t.Fatalf("cross-tenant GetExecution got execution=%+v err=%v, want nil nil", crossTenant, err)
	}
}

func TestWorkflowStorePersistsVersionHistoryAndExecutionVersion(t *testing.T) {
	store, _, ctx := testWorkflowSQLStore(t)

	created, err := store.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_workflow_1",
		Name:           "Versioned Workflow",
		Status:         WorkflowStatusPublished,
		Definition:     map[string]any{"nodes": []any{map[string]any{"id": "start_v1", "type": "manual"}}},
		Variables:      map[string]any{"revision": "one"},
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	updated, err := store.UpdateWorkflow(ctx, UpdateWorkflowStoreRequest{
		OrganizationID: "org_workflow_1",
		WorkflowID:     created.ID,
		Name:           created.Name,
		Description:    created.Description,
		Status:         WorkflowStatusPublished,
		Definition:     map[string]any{"nodes": []any{map[string]any{"id": "start_v2", "type": "manual"}}},
		Variables:      map[string]any{"revision": "two"},
	})
	if err != nil {
		t.Fatalf("UpdateWorkflow returned error: %v", err)
	}
	if updated.Version != 2 {
		t.Fatalf("UpdateWorkflow version=%d, want 2", updated.Version)
	}

	versions, err := store.ListWorkflowVersions(ctx, "org_workflow_1", created.ID)
	if err != nil {
		t.Fatalf("ListWorkflowVersions returned error: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("expected two workflow versions, got %+v", versions)
	}
	if got := versions[0].Definition["nodes"].([]any)[0].(map[string]any)["id"]; got != "start_v1" {
		t.Fatalf("v1 node=%v, want start_v1", got)
	}
	if got := versions[1].Variables["revision"]; got != "two" {
		t.Fatalf("v2 variables revision=%v, want two", got)
	}

	execution, err := store.CreateExecution(ctx, CreateExecutionRequest{
		OrganizationID:   "org_workflow_1",
		WorkflowID:       created.ID,
		WorkflowVersion:  updated.Version,
		Status:           ExecutionStatusRunning,
		WorkflowSnapshot: updated.Definition,
	})
	if err != nil {
		t.Fatalf("CreateExecution returned error: %v", err)
	}
	if execution.WorkflowVersion != updated.Version {
		t.Fatalf("execution workflow version=%d, want %d", execution.WorkflowVersion, updated.Version)
	}
	if got := execution.WorkflowSnapshot["nodes"].([]any)[0].(map[string]any)["id"]; got != "start_v2" {
		t.Fatalf("execution snapshot node=%v, want start_v2", got)
	}
}
