package workflow

import (
	"context"
	"database/sql"
	"errors"
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
		`DROP TABLE IF EXISTS workflow_debug_variable_snapshots CASCADE`,
		`DROP TABLE IF EXISTS workflow_debug_trace_entries CASCADE`,
		`DROP TABLE IF EXISTS workflow_execution_events CASCADE`,
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
	eventMigration, err := os.ReadFile("../../migrations/0087_workflow_execution_events.sql")
	if err != nil {
		t.Fatalf("read workflow execution events migration: %v", err)
	}
	if _, err := database.Exec(string(eventMigration)); err != nil {
		t.Fatalf("apply workflow execution events migration: %v", err)
	}
	debugTraceMigration, err := os.ReadFile("../../migrations/0091_workflow_debug_trace.sql")
	if err != nil {
		t.Fatalf("read workflow debug trace migration: %v", err)
	}
	if _, err := database.Exec(string(debugTraceMigration)); err != nil {
		t.Fatalf("apply workflow debug trace migration: %v", err)
	}
	scheduleIdempotencyMigration, err := os.ReadFile("../../migrations/0099_workflow_schedule_run_idempotency.sql")
	if err != nil {
		t.Fatalf("read workflow schedule run idempotency migration: %v", err)
	}
	if _, err := database.Exec(string(scheduleIdempotencyMigration)); err != nil {
		t.Fatalf("apply workflow schedule run idempotency migration: %v", err)
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
	events, err := store.ListExecutionEvents(ctx, "org_workflow_1", execution.ID)
	if err != nil {
		t.Fatalf("ListExecutionEvents returned error: %v", err)
	}
	if len(events) != 1 || events[0].EventType != "created" || events[0].ToStatus != ExecutionStatusFailed {
		t.Fatalf("expected created execution event, got %+v", events)
	}
	trace, err := store.ListExecutionDebugTraceEntries(ctx, "org_workflow_1", execution.ID)
	if err != nil {
		t.Fatalf("ListExecutionDebugTraceEntries returned error: %v", err)
	}
	if len(trace) != 1 || trace[0].NodeID != "notify" || trace[0].Error["message"] != "provider unavailable" {
		t.Fatalf("expected automatic node execution debug trace, got %+v", trace)
	}
	traceStartedAt := startedAt.Add(1500 * time.Millisecond)
	traceCompletedAt := startedAt.Add(1700 * time.Millisecond)
	if err := store.AppendExecutionDebugTraceEntry(ctx, AppendExecutionDebugTraceEntryRequest{
		OrganizationID: "org_workflow_1",
		ExecutionID:    execution.ID,
		CreatedAt:      traceStartedAt,
		Entry: ExecutionDebugTraceEntry{
			NodeID:      "notify",
			NodeType:    "agent",
			Status:      NodeStatusFailed,
			Attempt:     2,
			Input:       map[string]any{"template": "welcome"},
			Output:      map[string]any{"sent": false},
			Error:       map[string]any{"message": "provider unavailable"},
			Context:     map[string]any{"trace_id": "trace_123"},
			StartedAt:   traceStartedAt,
			CompletedAt: &traceCompletedAt,
			DurationMS:  200,
		},
	}); err != nil {
		t.Fatalf("AppendExecutionDebugTraceEntry returned error: %v", err)
	}
	trace, err = store.ListExecutionDebugTraceEntries(ctx, "org_workflow_1", execution.ID)
	if err != nil {
		t.Fatalf("ListExecutionDebugTraceEntries returned error: %v", err)
	}
	if len(trace) != 2 || trace[1].NodeID != "notify" || trace[1].Error["message"] != "provider unavailable" || trace[1].DurationMS != 200 {
		t.Fatalf("expected debug trace JSON to round trip, got %+v", trace)
	}
	if !trace[1].StartedAt.Equal(traceStartedAt) || trace[1].CompletedAt == nil || !trace[1].CompletedAt.Equal(traceCompletedAt) {
		t.Fatalf("expected debug trace timestamps to round trip, got %+v", trace[1])
	}
	if !trace[1].CreatedAt.Equal(traceStartedAt) {
		t.Fatalf("expected debug trace created_at to round trip, got %+v", trace[1])
	}
	crossTenantTrace, err := store.ListExecutionDebugTraceEntries(ctx, "org_workflow_2", execution.ID)
	if err != nil {
		t.Fatalf("ListExecutionDebugTraceEntries cross tenant returned error: %v", err)
	}
	if len(crossTenantTrace) != 0 {
		t.Fatalf("expected tenant-scoped debug trace, got %+v", crossTenantTrace)
	}
	if err := store.AppendExecutionVariableSnapshot(ctx, AppendExecutionVariableSnapshotRequest{
		OrganizationID: "org_workflow_1",
		ExecutionID:    execution.ID,
		CreatedAt:      traceCompletedAt,
		Snapshot: ExecutionVariableSnapshot{
			Input:   map[string]any{"customer_id": "cus_123"},
			Context: map[string]any{"trace_id": "trace_123"},
			NodeOutputs: map[string]map[string]any{
				"notify": map[string]any{"sent": false},
			},
		},
	}); err != nil {
		t.Fatalf("AppendExecutionVariableSnapshot returned error: %v", err)
	}
	variableSnapshot, err := store.LatestExecutionVariableSnapshot(ctx, "org_workflow_1", execution.ID)
	if err != nil {
		t.Fatalf("LatestExecutionVariableSnapshot returned error: %v", err)
	}
	if variableSnapshot == nil || variableSnapshot.Input["customer_id"] != "cus_123" || variableSnapshot.Context["trace_id"] != "trace_123" || variableSnapshot.NodeOutputs["notify"]["sent"] != false {
		t.Fatalf("expected variable snapshot JSON to round trip, got %+v", variableSnapshot)
	}
	crossTenantSnapshot, err := store.LatestExecutionVariableSnapshot(ctx, "org_workflow_2", execution.ID)
	if err != nil {
		t.Fatalf("LatestExecutionVariableSnapshot cross tenant returned error: %v", err)
	}
	if crossTenantSnapshot != nil {
		t.Fatalf("expected tenant-scoped variable snapshot, got %+v", crossTenantSnapshot)
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

	updatedAt := time.Date(2026, 6, 4, 8, 5, 0, 0, time.UTC)
	if _, err := store.UpdateExecutionStatus(ctx, "org_workflow_1", execution.ID, ExecutionStatusCancelled, &updatedAt); err != nil {
		t.Fatalf("UpdateExecutionStatus returned error: %v", err)
	}
	events, err = store.ListExecutionEvents(ctx, "org_workflow_1", execution.ID)
	if err != nil {
		t.Fatalf("ListExecutionEvents after status update returned error: %v", err)
	}
	if len(events) != 2 || events[1].EventType != "status_changed" || events[1].FromStatus != ExecutionStatusFailed || events[1].ToStatus != ExecutionStatusCancelled {
		t.Fatalf("expected status_changed execution event, got %+v", events)
	}
}

func TestWorkflowExecutionEventsMigrationDeclaresTransitionAudit(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/0087_workflow_execution_events.sql")
	if err != nil {
		t.Fatalf("read workflow execution events migration: %v", err)
	}
	migration := string(raw)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS workflow_execution_events",
		"execution_id TEXT NOT NULL REFERENCES workflow_executions(id) ON DELETE CASCADE",
		"event_type TEXT NOT NULL",
		"from_status TEXT NOT NULL DEFAULT ''",
		"to_status TEXT NOT NULL",
		"CHECK (event_type IN ('created', 'status_changed'))",
		"ON workflow_execution_events (organization_id, execution_id, created_at ASC, id ASC)",
		"FROM workflow_executions e",
	} {
		if !strings.Contains(migration, want) {
			t.Fatalf("expected workflow execution events migration to contain %q, got:\n%s", want, migration)
		}
	}
}

func TestWorkflowSQLStoreReplaysExecutionStateAfterServiceRebuild(t *testing.T) {
	store, database, ctx := testWorkflowSQLStore(t)
	workflowDef, err := store.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_workflow_1",
		Name:           "Restart Replay Flow",
		Status:         WorkflowStatusPublished,
		Definition: map[string]any{
			"nodes": []any{
				map[string]any{"id": "start", "type": "manual"},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := store.CreateExecution(ctx, CreateExecutionRequest{
		OrganizationID: "org_workflow_1",
		WorkflowID:     workflowDef.ID,
		Status:         ExecutionStatusRunning,
		StartedAt:      time.Date(2026, 7, 4, 10, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateExecution returned error: %v", err)
	}

	beforeRestartService := NewService(store)
	if _, err := beforeRestartService.PauseExecution(ctx, "org_workflow_1", execution.ID); err != nil {
		t.Fatalf("PauseExecution returned error: %v", err)
	}
	if _, err := beforeRestartService.ResumeExecution(ctx, "org_workflow_1", execution.ID); err != nil {
		t.Fatalf("ResumeExecution returned error: %v", err)
	}
	if _, err := beforeRestartService.CancelExecution(ctx, "org_workflow_1", execution.ID); err != nil {
		t.Fatalf("CancelExecution returned error: %v", err)
	}

	afterRestartService := NewService(NewSQLStore(database))
	snapshot, err := afterRestartService.BuildExecutionDebugSnapshot(ctx, "org_workflow_1", execution.ID)
	if err != nil {
		t.Fatalf("BuildExecutionDebugSnapshot after service rebuild returned error: %v", err)
	}
	if snapshot.Status != ExecutionStatusCancelled {
		t.Fatalf("snapshot status=%s, want cancelled", snapshot.Status)
	}
	if !snapshot.StateReplay.Valid {
		t.Fatalf("expected valid state replay after service rebuild, got %+v", snapshot.StateReplay)
	}
	if snapshot.StateReplay.InitialStatus != ExecutionStatusRunning || snapshot.StateReplay.FinalStatus != ExecutionStatusCancelled {
		t.Fatalf("unexpected state replay endpoints after rebuild: %+v", snapshot.StateReplay)
	}
	wantTransitions := []struct {
		from  ExecutionStatus
		to    ExecutionStatus
		event WorkflowStateMachineEvent
	}{
		{ExecutionStatusRunning, ExecutionStatusPaused, StateEventPause},
		{ExecutionStatusPaused, ExecutionStatusRunning, StateEventResume},
		{ExecutionStatusRunning, ExecutionStatusCancelled, StateEventCancel},
	}
	if len(snapshot.StateReplay.Transitions) != len(wantTransitions) {
		t.Fatalf("state replay transitions=%+v, want %d transitions", snapshot.StateReplay.Transitions, len(wantTransitions))
	}
	for i, want := range wantTransitions {
		got := snapshot.StateReplay.Transitions[i]
		if got.FromStatus != want.from || got.ToStatus != want.to || got.Event != want.event {
			t.Fatalf("transition[%d]=%+v, want from=%s to=%s event=%s", i, got, want.from, want.to, want.event)
		}
		if got.EventID == "" || got.CreatedAt.IsZero() {
			t.Fatalf("transition[%d] missing durable event metadata: %+v", i, got)
		}
	}
}

func TestWorkflowSQLStoreRejectsDuplicateScheduleRunAfterServiceRebuild(t *testing.T) {
	store, database, ctx := testWorkflowSQLStore(t)
	workflowDef, err := store.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_workflow_1",
		Name:           "Scheduled Idempotent Flow",
		Status:         WorkflowStatusPublished,
		Definition: map[string]any{
			"nodes": []any{
				map[string]any{"id": "start", "type": "schedule"},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	firstService := NewService(store)
	first, err := firstService.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_workflow_1",
		WorkflowID:     workflowDef.ID,
		TriggerType:    WorkflowTriggerSchedule,
		TriggerPayload: map[string]any{
			"scheduledTaskId":    "sched_1",
			"scheduledTaskRunId": "schedrun_1",
		},
	})
	if err != nil {
		t.Fatalf("first StartExecution returned error: %v", err)
	}

	rebuiltService := NewService(NewSQLStore(database))
	second, err := rebuiltService.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_workflow_1",
		WorkflowID:     workflowDef.ID,
		TriggerType:    WorkflowTriggerSchedule,
		TriggerPayload: map[string]any{
			"scheduledTaskId":    "sched_1",
			"scheduledTaskRunId": "schedrun_1",
		},
	})
	if err != nil {
		t.Fatalf("duplicate StartExecution after service rebuild returned error: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("duplicate scheduled run created execution %s, want existing execution %s", second.ID, first.ID)
	}
	executions, err := rebuiltService.ListExecutions(ctx, "org_workflow_1", workflowDef.ID)
	if err != nil {
		t.Fatalf("ListExecutions returned error: %v", err)
	}
	if len(executions) != 1 {
		t.Fatalf("expected one persisted execution for schedule run replay, got %d: %+v", len(executions), executions)
	}
}

func TestWorkflowSQLStoreReturnsExistingScheduleRunOnUniqueConflict(t *testing.T) {
	store, _, ctx := testWorkflowSQLStore(t)
	workflowDef, err := store.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_workflow_1",
		Name:           "Schedule Conflict Flow",
		Status:         WorkflowStatusPublished,
		Definition: map[string]any{
			"nodes": []any{
				map[string]any{"id": "start", "type": "schedule"},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	contextValue := map[string]any{
		"trigger": map[string]any{
			"type":               "schedule",
			"scheduledTaskId":    "sched_1",
			"scheduledTaskRunId": "schedrun_conflict",
		},
	}

	first, err := store.CreateExecution(ctx, CreateExecutionRequest{
		OrganizationID: "org_workflow_1",
		WorkflowID:     workflowDef.ID,
		Status:         ExecutionStatusRunning,
		Context:        contextValue,
	})
	if err != nil {
		t.Fatalf("first CreateExecution returned error: %v", err)
	}
	second, err := store.CreateExecution(ctx, CreateExecutionRequest{
		OrganizationID: "org_workflow_1",
		WorkflowID:     workflowDef.ID,
		Status:         ExecutionStatusRunning,
		Context:        contextValue,
	})
	if err != nil {
		t.Fatalf("duplicate CreateExecution returned error: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("duplicate CreateExecution returned execution %s, want existing execution %s", second.ID, first.ID)
	}
}

func TestWorkflowSQLStorePrunesDebugRetentionAfterServiceRebuild(t *testing.T) {
	store, database, ctx := testWorkflowSQLStore(t)
	workflowDef, err := store.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_workflow_1",
		Name:           "Retention Prune Flow",
		Status:         WorkflowStatusPublished,
		Definition: map[string]any{
			"nodes": []any{
				map[string]any{"id": "start", "type": "manual"},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := store.CreateExecution(ctx, CreateExecutionRequest{
		OrganizationID: "org_workflow_1",
		WorkflowID:     workflowDef.ID,
		Status:         ExecutionStatusRunning,
		StartedAt:      time.Date(2026, 7, 5, 8, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateExecution returned error: %v", err)
	}
	otherWorkflow, err := store.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_workflow_2",
		Name:           "Other Retention Flow",
		Status:         WorkflowStatusPublished,
		Definition:     map[string]any{"nodes": []any{map[string]any{"id": "start", "type": "manual"}}},
	})
	if err != nil {
		t.Fatalf("CreateWorkflow other tenant returned error: %v", err)
	}
	otherExecution, err := store.CreateExecution(ctx, CreateExecutionRequest{
		OrganizationID: "org_workflow_2",
		WorkflowID:     otherWorkflow.ID,
		Status:         ExecutionStatusRunning,
		StartedAt:      time.Date(2026, 7, 5, 8, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("CreateExecution other tenant returned error: %v", err)
	}

	expiredAt := time.Date(2026, 7, 5, 8, 1, 0, 0, time.UTC)
	cutoff := time.Date(2026, 7, 5, 9, 0, 0, 0, time.UTC)
	retainedAt := time.Date(2026, 7, 5, 9, 1, 0, 0, time.UTC)
	for _, entry := range []struct {
		organizationID string
		executionID    string
		nodeID         string
		createdAt      time.Time
	}{
		{"org_workflow_1", execution.ID, "expired", expiredAt},
		{"org_workflow_1", execution.ID, "retained", retainedAt},
		{"org_workflow_2", otherExecution.ID, "other-expired", expiredAt},
	} {
		if err := store.AppendExecutionDebugTraceEntry(ctx, AppendExecutionDebugTraceEntryRequest{
			OrganizationID: entry.organizationID,
			ExecutionID:    entry.executionID,
			CreatedAt:      entry.createdAt,
			Entry: ExecutionDebugTraceEntry{
				NodeID:     entry.nodeID,
				NodeType:   "manual",
				Status:     NodeStatusSucceeded,
				Attempt:    1,
				Input:      map[string]any{"node": entry.nodeID},
				Output:     map[string]any{"ok": true},
				Context:    map[string]any{"trace": entry.nodeID},
				StartedAt:  entry.createdAt,
				DurationMS: 1,
			},
		}); err != nil {
			t.Fatalf("AppendExecutionDebugTraceEntry %s returned error: %v", entry.nodeID, err)
		}
		if err := store.AppendExecutionVariableSnapshot(ctx, AppendExecutionVariableSnapshotRequest{
			OrganizationID: entry.organizationID,
			ExecutionID:    entry.executionID,
			CreatedAt:      entry.createdAt,
			Snapshot: ExecutionVariableSnapshot{
				Input:       map[string]any{"node": entry.nodeID},
				Context:     map[string]any{"trace": entry.nodeID},
				NodeOutputs: map[string]map[string]any{entry.nodeID: {"ok": true}},
			},
		}); err != nil {
			t.Fatalf("AppendExecutionVariableSnapshot %s returned error: %v", entry.nodeID, err)
		}
	}

	afterRestartService := NewService(NewSQLStore(database))
	result, err := afterRestartService.PruneExecutionDebugData(ctx, "org_workflow_1", cutoff)
	if err != nil {
		t.Fatalf("PruneExecutionDebugData after service rebuild returned error: %v", err)
	}
	if result.TraceEntriesDeleted != 1 || result.VariableSnapshotsDeleted != 1 {
		t.Fatalf("unexpected prune result after service rebuild: %+v", result)
	}

	trace, err := store.ListExecutionDebugTraceEntries(ctx, "org_workflow_1", execution.ID)
	if err != nil {
		t.Fatalf("ListExecutionDebugTraceEntries returned error: %v", err)
	}
	if len(trace) != 1 || trace[0].NodeID != "retained" {
		t.Fatalf("expected only retained tenant trace row after prune, got %+v", trace)
	}
	snapshot, err := store.LatestExecutionVariableSnapshot(ctx, "org_workflow_1", execution.ID)
	if err != nil {
		t.Fatalf("LatestExecutionVariableSnapshot returned error: %v", err)
	}
	if snapshot == nil || snapshot.Input["node"] != "retained" {
		t.Fatalf("expected retained tenant variable snapshot after prune, got %+v", snapshot)
	}
	otherTrace, err := store.ListExecutionDebugTraceEntries(ctx, "org_workflow_2", otherExecution.ID)
	if err != nil {
		t.Fatalf("ListExecutionDebugTraceEntries other tenant returned error: %v", err)
	}
	if len(otherTrace) != 1 || otherTrace[0].NodeID != "other-expired" {
		t.Fatalf("expected other tenant expired trace to remain, got %+v", otherTrace)
	}
	otherSnapshot, err := store.LatestExecutionVariableSnapshot(ctx, "org_workflow_2", otherExecution.ID)
	if err != nil {
		t.Fatalf("LatestExecutionVariableSnapshot other tenant returned error: %v", err)
	}
	if otherSnapshot == nil || otherSnapshot.Input["node"] != "other-expired" {
		t.Fatalf("expected other tenant variable snapshot to remain, got %+v", otherSnapshot)
	}
}

func TestWorkflowSQLStoreResolvesPausedFailureAfterServiceRebuild(t *testing.T) {
	store, database, ctx := testWorkflowSQLStore(t)
	workflowDef, err := store.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_workflow_1",
		Name:           "Failure Recovery Flow",
		Status:         WorkflowStatusPublished,
		Definition: map[string]any{
			"nodes": []any{
				map[string]any{"id": "must_review", "type": "manual"},
				map[string]any{"id": "notify", "type": "manual"},
			},
			"edges": []any{
				map[string]any{"from": "must_review", "to": "notify"},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	beforeRestartService := NewService(store)
	execution, err := beforeRestartService.StartExecution(ctx, StartExecutionRequest{
		OrganizationID: "org_workflow_1",
		WorkflowID:     workflowDef.ID,
		Input:          map[string]any{"ticket": "INC-42"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}
	if _, err := beforeRestartService.RecordNodeStatus(ctx, "org_workflow_1", execution.ID, RecordNodeStatusRequest{
		NodeID: "must_review",
		Status: NodeStatusFailed,
		Input:  map[string]any{"ticket": "INC-42"},
		Error:  map[string]any{"message": "manual review required"},
	}); err != nil {
		t.Fatalf("RecordNodeStatus pause policy returned error: %v", err)
	}

	afterRestartService := NewService(NewSQLStore(database))
	resolved, err := afterRestartService.ResolvePausedFailure(ctx, "org_workflow_1", execution.ID, ResolveFailureDecisionRequest{
		Action: FailureActionContinue,
		NodeID: "must_review",
	})
	if err != nil {
		t.Fatalf("ResolvePausedFailure after service rebuild returned error: %v", err)
	}
	if resolved.Status != ExecutionStatusRunning {
		t.Fatalf("expected skip decision to resume execution after rebuild, got %s", resolved.Status)
	}
	failedNodes := workflowNodeExecutionsByID(resolved.NodeExecutions, "must_review")
	if len(failedNodes) < 2 || failedNodes[len(failedNodes)-1].Status != NodeStatusSkipped {
		t.Fatalf("expected skipped decision node after rebuilt-service failure decision, got %+v", failedNodes)
	}
	notifyNodes := workflowNodeExecutionsByID(resolved.NodeExecutions, "notify")
	if len(notifyNodes) != 1 || notifyNodes[0].Status != NodeStatusPending {
		t.Fatalf("expected downstream notify node to be seeded after rebuilt-service failure decision, got %+v", notifyNodes)
	}

	snapshot, err := afterRestartService.BuildExecutionDebugSnapshot(ctx, "org_workflow_1", execution.ID)
	if err != nil {
		t.Fatalf("BuildExecutionDebugSnapshot after failure decision returned error: %v", err)
	}
	if !snapshot.StateReplay.Valid || snapshot.StateReplay.FinalStatus != ExecutionStatusRunning {
		t.Fatalf("expected valid state replay ending running after failure decision, got %+v", snapshot.StateReplay)
	}
	if len(snapshot.StateReplay.Transitions) != 2 {
		t.Fatalf("expected pause and resume transitions after rebuilt-service failure decision, got %+v", snapshot.StateReplay.Transitions)
	}
	if snapshot.StateReplay.Transitions[0].Event != StateEventPause || snapshot.StateReplay.Transitions[1].Event != StateEventResume {
		t.Fatalf("expected pause then resume transitions, got %+v", snapshot.StateReplay.Transitions)
	}
}

func TestWorkflowSQLStoreRunsDueAutoRetryAfterServiceRebuild(t *testing.T) {
	store, database, ctx := testWorkflowSQLStore(t)
	attempts := 0
	beforeRestartService := NewService(store, WithNodeExecutors(NewNodeExecutorRegistry(functionNodeExecutor{
		nodeType: "http",
		execute: func(context.Context, NodeExecutorInput) (map[string]any, error) {
			attempts++
			return nil, errors.New("temporary outage")
		},
	})))
	workflowDef, err := beforeRestartService.CreateWorkflow(ctx, CreateWorkflowRequest{
		OrganizationID: "org_workflow_1",
		Name:           "Retry Recovery Flow",
		Status:         WorkflowStatusPublished,
		Definition: map[string]any{
			"nodes": []any{
				map[string]any{
					"id":   "call_api",
					"type": "http",
					"failurePolicy": map[string]any{
						"strategy":    "auto_retry",
						"maxRetries":  float64(2),
						"retryDelays": []any{"1h"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := beforeRestartService.StartExecution(ctx, StartExecutionRequest{OrganizationID: "org_workflow_1", WorkflowID: workflowDef.ID})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}
	blocked, err := beforeRestartService.RunExecutionUntilBlocked(ctx, "org_workflow_1", execution.ID)
	if err != nil {
		t.Fatalf("RunExecutionUntilBlocked before rebuild returned error: %v", err)
	}
	if attempts != 1 {
		t.Fatalf("expected one failed attempt before rebuild, got %d", attempts)
	}
	retryingNodes := workflowNodeExecutionsByID(blocked.NodeExecutions, "call_api")
	if len(retryingNodes) != 2 || retryingNodes[len(retryingNodes)-1].Status != NodeStatusRetrying {
		t.Fatalf("expected retrying node before rebuild, got %+v", retryingNodes)
	}

	retryAt := time.Now().UTC().Add(-time.Second).Format(time.RFC3339)
	if _, err := database.ExecContext(ctx, `
		UPDATE workflow_node_executions
		SET context = jsonb_set(context, '{retryAt}', to_jsonb($1::text), true)
		WHERE organization_id = $2 AND execution_id = $3 AND node_id = $4 AND status = $5
	`, retryAt, "org_workflow_1", execution.ID, "call_api", string(NodeStatusRetrying)); err != nil {
		t.Fatalf("force retry due after rebuild: %v", err)
	}

	afterRestartAttempts := 0
	afterRestartService := NewService(NewSQLStore(database), WithNodeExecutors(NewNodeExecutorRegistry(functionNodeExecutor{
		nodeType: "http",
		execute: func(context.Context, NodeExecutorInput) (map[string]any, error) {
			afterRestartAttempts++
			return map[string]any{"attempts": afterRestartAttempts + attempts}, nil
		},
	})))
	completed, err := afterRestartService.RunExecutionUntilBlocked(ctx, "org_workflow_1", execution.ID)
	if err != nil {
		t.Fatalf("RunExecutionUntilBlocked after rebuild returned error: %v", err)
	}
	if afterRestartAttempts != 1 {
		t.Fatalf("expected one retry attempt after rebuild, got %d", afterRestartAttempts)
	}
	if completed.Status != ExecutionStatusSucceeded {
		t.Fatalf("expected execution to succeed after rebuilt-service retry, got %+v", completed)
	}
	nodes := workflowNodeExecutionsByID(completed.NodeExecutions, "call_api")
	if len(nodes) != 3 || nodes[len(nodes)-1].Status != NodeStatusSucceeded || nodes[len(nodes)-1].Attempt != 2 {
		t.Fatalf("expected rebuilt-service retry attempt 2 to succeed, got %+v", nodes)
	}
}

func TestWorkflowDebugTraceMigrationDeclaresDurableTraceTables(t *testing.T) {
	raw, err := os.ReadFile("../../migrations/0091_workflow_debug_trace.sql")
	if err != nil {
		t.Fatalf("read workflow debug trace migration: %v", err)
	}
	migration := string(raw)
	for _, want := range []string{
		"CREATE TABLE IF NOT EXISTS workflow_debug_trace_entries",
		"execution_id TEXT NOT NULL REFERENCES workflow_executions(id) ON DELETE CASCADE",
		"input JSONB NOT NULL DEFAULT '{}'",
		"output JSONB NOT NULL DEFAULT '{}'",
		"context JSONB NOT NULL DEFAULT '{}'",
		"duration_ms INTEGER NOT NULL DEFAULT 0",
		"ON workflow_debug_trace_entries (organization_id, execution_id, created_at ASC, id ASC)",
		"CREATE TABLE IF NOT EXISTS workflow_debug_variable_snapshots",
		"node_outputs JSONB NOT NULL DEFAULT '{}'",
		"ON workflow_debug_variable_snapshots (organization_id, execution_id, created_at DESC, id DESC)",
	} {
		if !strings.Contains(migration, want) {
			t.Fatalf("expected workflow debug trace migration to contain %q, got:\n%s", want, migration)
		}
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
