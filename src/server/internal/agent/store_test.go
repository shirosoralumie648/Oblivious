package agent

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"oblivious/server/internal/chat"
)

func testAgentRunSQLStore(t *testing.T) (*SQLStore, context.Context) {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		if strings.EqualFold(os.Getenv("OBLIVIOUS_REQUIRE_TEST_DATABASE"), "true") {
			t.Fatal("TEST_DATABASE_URL is required for DB-backed agent run tests")
		}
		t.Skip("TEST_DATABASE_URL is required for DB-backed agent run tests")
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

	if _, err := database.Exec(`SELECT pg_advisory_lock(104210)`); err != nil {
		t.Fatalf("lock agent run test database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := database.Exec(`SELECT pg_advisory_unlock(104210)`); err != nil {
			t.Fatalf("unlock agent run test database: %v", err)
		}
	})

	statements := []string{
		`DROP TABLE IF EXISTS agent_plan_steps CASCADE`,
		`DROP TABLE IF EXISTS agent_tool_runs CASCADE`,
		`DROP TABLE IF EXISTS agent_runs CASCADE`,
		`DROP TABLE IF EXISTS agent_memories CASCADE`,
		`DROP TABLE IF EXISTS agent_messages CASCADE`,
		`DROP TABLE IF EXISTS agent_conversations CASCADE`,
		`DROP TABLE IF EXISTS agents CASCADE`,
		`DROP TABLE IF EXISTS organizations CASCADE`,
		`DROP TABLE IF EXISTS users CASCADE`,
		`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, role TEXT NOT NULL DEFAULT 'user', name TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE organizations (id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'active', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE agents (id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, name TEXT NOT NULL, description TEXT, model TEXT DEFAULT 'gpt-4o-mini', system_prompt TEXT, tools JSONB DEFAULT '[]', config JSONB DEFAULT '{}', is_public BOOLEAN DEFAULT false, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE agent_conversations (id TEXT PRIMARY KEY, agent_id TEXT NOT NULL REFERENCES agents(id) ON DELETE CASCADE, user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, title TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`CREATE TABLE agent_messages (id TEXT PRIMARY KEY, conversation_id TEXT NOT NULL REFERENCES agent_conversations(id) ON DELETE CASCADE, organization_id TEXT REFERENCES organizations(id) ON DELETE SET NULL, role TEXT NOT NULL, content TEXT NOT NULL, tool_calls JSONB DEFAULT '[]', tool_call_id TEXT, created_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`INSERT INTO users (id, email, password_hash, name) VALUES ('user_1', 'agent-run-user@example.com', 'hash', 'Agent Run User'), ('user_2', 'agent-run-other@example.com', 'hash', 'Other User')`,
		`INSERT INTO organizations (id, slug, name) VALUES ('org_1', 'agent-run-org-1', 'Agent Run Org 1'), ('org_2', 'agent-run-org-2', 'Agent Run Org 2')`,
		`INSERT INTO agents (id, user_id, organization_id, name, tools) VALUES ('agent_1', 'user_1', 'org_1', 'Durable Agent', '[{"name":"datetime","type":"builtin","enabled":true}]'::jsonb), ('agent_2', 'user_2', 'org_2', 'Other Durable Agent', '[]'::jsonb)`,
		`INSERT INTO agent_conversations (id, agent_id, user_id, organization_id, title) VALUES ('conv_1', 'agent_1', 'user_1', 'org_1', 'Durable Conversation'), ('conv_2', 'agent_2', 'user_2', 'org_2', 'Other Conversation')`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("prepare agent run database: %v\nstatement: %s", err, statement)
		}
	}

	migration, err := os.ReadFile("../../migrations/0031_agent_workflow_runs.sql")
	if err != nil {
		t.Fatalf("read agent workflow migration: %v", err)
	}
	if _, err := database.Exec(string(migration)); err != nil {
		t.Fatalf("apply agent workflow migration: %v", err)
	}
	runModeMigration, err := os.ReadFile("../../migrations/0070_agent_run_mode.sql")
	if err != nil {
		t.Fatalf("read agent run mode migration: %v", err)
	}
	if _, err := database.Exec(string(runModeMigration)); err != nil {
		t.Fatalf("apply agent run mode migration: %v", err)
	}
	toolRiskMigration, err := os.ReadFile("../../migrations/0050_agent_tool_risk_level.sql")
	if err != nil {
		t.Fatalf("read agent tool risk migration: %v", err)
	}
	if _, err := database.Exec(string(toolRiskMigration)); err != nil {
		t.Fatalf("apply agent tool risk migration: %v", err)
	}
	planStepMigration, err := os.ReadFile("../../migrations/0051_agent_plan_steps.sql")
	if err != nil {
		t.Fatalf("read agent plan steps migration: %v", err)
	}
	if _, err := database.Exec(string(planStepMigration)); err != nil {
		t.Fatalf("apply agent plan steps migration: %v", err)
	}
	planStepExecutionMigration, err := os.ReadFile("../../migrations/0052_agent_plan_step_execution.sql")
	if err != nil {
		t.Fatalf("read agent plan step execution migration: %v", err)
	}
	if _, err := database.Exec(string(planStepExecutionMigration)); err != nil {
		t.Fatalf("apply agent plan step execution migration: %v", err)
	}
	agentMemoriesMigration, err := os.ReadFile("../../migrations/0044_agent_memories.sql")
	if err != nil {
		t.Fatalf("read agent memories migration: %v", err)
	}
	if _, err := database.Exec(string(agentMemoriesMigration)); err != nil {
		t.Fatalf("apply agent memories migration: %v", err)
	}

	return NewSQLStore(database), context.Background()
}

func TestAgentRunStorePersistsRunLifecycle(t *testing.T) {
	store, ctx := testAgentRunSQLStore(t)

	run, err := store.CreateRun(ctx, &CreateRunRequest{
		OrganizationID:    "org_1",
		ConversationID:    "conv_1",
		AgentID:           "agent_1",
		UserID:            "user_1",
		RequestID:         "req_agent_run_lifecycle",
		Mode:              ExecutionModePlanning,
		Status:            RunStatusRunning,
		MemoryEnabled:     true,
		MemorySearched:    true,
		MemoryResultCount: 2,
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	if run.ID == "" || run.OrganizationID != "org_1" || run.Status != RunStatusRunning {
		t.Fatalf("unexpected created run: %+v", run)
	}
	if run.Mode != ExecutionModePlanning {
		t.Fatalf("expected persisted run mode %q, got %q", ExecutionModePlanning, run.Mode)
	}
	if !run.MemoryEnabled || !run.MemorySearched || run.MemoryResultCount != 2 {
		t.Fatalf("memory evidence was not persisted on run: %+v", run)
	}

	completedAt := time.Now().UTC()
	updated, err := store.UpdateRun(ctx, "org_1", run.ID, UpdateRunRequest{
		Status:         stringPtr(RunStatusCompleted),
		IterationCount: intPtr(2),
		ToolCallCount:  intPtr(1),
		FinalMessageID: stringPtr("msg_final"),
		CompletedAt:    &completedAt,
	})
	if err != nil {
		t.Fatalf("UpdateRun returned error: %v", err)
	}
	if updated.Status != RunStatusCompleted || updated.FinalMessageID != "msg_final" {
		t.Fatalf("expected completed run with final message, got %+v", updated)
	}
	if updated.IterationCount != 2 || updated.ToolCallCount != 1 {
		t.Fatalf("expected iteration/tool counts to persist, got %+v", updated)
	}
	if updated.CompletedAt == nil {
		t.Fatalf("expected completed_at to be persisted: %+v", updated)
	}
}

func TestAgentSQLStorePersistsApprovalConfigAndToolRiskLevels(t *testing.T) {
	store, ctx := testAgentRunSQLStore(t)

	requiresApproval := true
	created, err := store.CreateAgent(ctx, "user_1", "org_1", &CreateAgentRequest{
		Name:  "Risk Managed Agent",
		Model: "gpt-4o-mini",
		Tools: []Tool{
			{Name: "datetime", Type: "builtin", Enabled: true, RiskLevel: ToolRiskSafe},
			{Name: "execute_code", Type: "builtin", Enabled: true, RiskLevel: ToolRiskDangerous},
		},
		Config: Config{
			ApprovalMode: ApprovalModeCustom,
			ToolApprovalOverrides: map[string]ToolApprovalOverride{
				"execute_code": {
					RiskLevel:        ToolRiskDangerous,
					RequiresApproval: &requiresApproval,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateAgent returned error: %v", err)
	}

	got, err := store.GetAgent(ctx, created.ID, "org_1")
	if err != nil {
		t.Fatalf("GetAgent returned error: %v", err)
	}
	assertAgentApprovalAndToolRisk(t, got, ApprovalModeCustom, ToolRiskDangerous, true)

	requiresApproval = false
	updated, err := store.UpdateAgent(ctx, created.ID, "org_1", &UpdateAgentRequest{
		Tools: []Tool{
			{Name: "write_note", Type: "builtin", Enabled: true, RiskLevel: ToolRiskMedium},
		},
		Config: &Config{
			ApprovalMode: ApprovalModeNone,
			ToolApprovalOverrides: map[string]ToolApprovalOverride{
				"write_note": {
					RiskLevel:        ToolRiskMedium,
					RequiresApproval: &requiresApproval,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("UpdateAgent returned error: %v", err)
	}
	assertAgentApprovalAndToolRisk(t, updated, ApprovalModeNone, ToolRiskMedium, false)

	got, err = store.GetAgent(ctx, created.ID, "org_1")
	if err != nil {
		t.Fatalf("GetAgent after update returned error: %v", err)
	}
	assertAgentApprovalAndToolRisk(t, got, ApprovalModeNone, ToolRiskMedium, false)
}

func TestAgentSQLStorePersistsDefaultExecutionModeConfig(t *testing.T) {
	store, ctx := testAgentRunSQLStore(t)

	created, err := store.CreateAgent(ctx, "user_1", "org_1", &CreateAgentRequest{
		Name:  "Planning Default Agent",
		Model: "gpt-4o-mini",
		Config: Config{
			DefaultExecutionMode: ExecutionModePlanning,
		},
	})
	if err != nil {
		t.Fatalf("CreateAgent returned error: %v", err)
	}

	got, err := store.GetAgent(ctx, created.ID, "org_1")
	if err != nil {
		t.Fatalf("GetAgent returned error: %v", err)
	}
	if got.Config.DefaultExecutionMode != ExecutionModePlanning {
		t.Fatalf("default execution mode = %q, want %q", got.Config.DefaultExecutionMode, ExecutionModePlanning)
	}

	updated, err := store.UpdateAgent(ctx, created.ID, "org_1", &UpdateAgentRequest{
		Config: &Config{
			DefaultExecutionMode: ExecutionModeReact,
		},
	})
	if err != nil {
		t.Fatalf("UpdateAgent returned error: %v", err)
	}
	if updated.Config.DefaultExecutionMode != ExecutionModeReact {
		t.Fatalf("updated default execution mode = %q, want %q", updated.Config.DefaultExecutionMode, ExecutionModeReact)
	}

	got, err = store.GetAgent(ctx, created.ID, "org_1")
	if err != nil {
		t.Fatalf("GetAgent after update returned error: %v", err)
	}
	if got.Config.DefaultExecutionMode != ExecutionModeReact {
		t.Fatalf("persisted default execution mode after update = %q, want %q", got.Config.DefaultExecutionMode, ExecutionModeReact)
	}
}

func TestAgentToolRunStorePersistsToolLifecycle(t *testing.T) {
	store, ctx := testAgentRunSQLStore(t)

	run, err := store.CreateRun(ctx, &CreateRunRequest{
		OrganizationID: "org_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		RequestID:      "req_tool_run_lifecycle",
		Status:         RunStatusRunning,
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}

	toolRun, err := store.CreateToolRun(ctx, &CreateToolRunRequest{
		OrganizationID: "org_1",
		RunID:          run.ID,
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		ToolCallID:     "call_datetime_1",
		ToolName:       "datetime",
		ToolType:       "builtin",
		Arguments:      map[string]any{"format": "rfc3339"},
		Status:         ToolRunStatusPendingApproval,
		ApprovalStatus: ApprovalStatusPending,
		AttemptCount:   0,
	})
	if err != nil {
		t.Fatalf("CreateToolRun returned error: %v", err)
	}
	if toolRun.ID == "" || toolRun.RunID != run.ID || toolRun.Status != ToolRunStatusPendingApproval {
		t.Fatalf("unexpected created tool run: %+v", toolRun)
	}
	if toolRun.Arguments["format"] != "rfc3339" {
		t.Fatalf("expected arguments to round trip, got %+v", toolRun.Arguments)
	}

	startedAt := time.Now().UTC()
	running, err := store.UpdateToolRun(ctx, "org_1", toolRun.ID, UpdateToolRunRequest{
		Status:         stringPtr(ToolRunStatusRunning),
		ApprovalStatus: stringPtr(ApprovalStatusNotRequired),
		AttemptCount:   intPtr(1),
		StartedAt:      &startedAt,
	})
	if err != nil {
		t.Fatalf("UpdateToolRun running returned error: %v", err)
	}
	if running.Status != ToolRunStatusRunning || running.AttemptCount != 1 || running.StartedAt == nil {
		t.Fatalf("expected running tool run with attempt evidence, got %+v", running)
	}

	completedAt := time.Now().UTC()
	completed, err := store.UpdateToolRun(ctx, "org_1", toolRun.ID, UpdateToolRunRequest{
		Status:        stringPtr(ToolRunStatusCompleted),
		ResultContent: stringPtr("2026-05-28T00:00:00Z"),
		CompletedAt:   &completedAt,
	})
	if err != nil {
		t.Fatalf("UpdateToolRun completed returned error: %v", err)
	}
	if completed.Status != ToolRunStatusCompleted || completed.ResultContent != "2026-05-28T00:00:00Z" {
		t.Fatalf("expected completed tool run result, got %+v", completed)
	}
	if completed.CompletedAt == nil {
		t.Fatalf("expected completed_at to be persisted: %+v", completed)
	}
}

func TestAgentPlanStepStoreRoundTripsStepsInOrder(t *testing.T) {
	store, ctx := testAgentRunSQLStore(t)

	run, err := store.CreateRun(ctx, &CreateRunRequest{
		OrganizationID: "org_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		RequestID:      "req_plan_step_round_trip",
		Status:         RunStatusRunning,
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}

	first, err := store.CreatePlanStep(ctx, &CreatePlanStepRequest{
		OrganizationID: "org_1",
		RunID:          run.ID,
		Index:          1,
		Title:          "Gather requirements",
		Status:         PlanStepStatusPending,
		ApprovalStatus: ApprovalStatusNotRequired,
	})
	if err != nil {
		t.Fatalf("CreatePlanStep first returned error: %v", err)
	}
	second, err := store.CreatePlanStep(ctx, &CreatePlanStepRequest{
		OrganizationID: "org_1",
		RunID:          run.ID,
		Index:          2,
		Title:          "Draft implementation",
		Status:         PlanStepStatusPending,
		ApprovalStatus: ApprovalStatusNotRequired,
		ToolName:       "write_file",
		Input:          map[string]any{"path": "src/server/internal/agent/store.go"},
	})
	if err != nil {
		t.Fatalf("CreatePlanStep second returned error: %v", err)
	}

	steps, err := store.ListPlanSteps(ctx, "org_1", run.ID)
	if err != nil {
		t.Fatalf("ListPlanSteps returned error: %v", err)
	}
	if len(steps) != 2 {
		t.Fatalf("expected 2 plan steps, got %+v", steps)
	}
	if steps[0].ID != first.ID || steps[1].ID != second.ID {
		t.Fatalf("expected insertion/index order, got %+v", steps)
	}
	if steps[0].Index != 1 || steps[0].Title != "Gather requirements" || steps[0].Status != PlanStepStatusPending {
		t.Fatalf("unexpected first step: %+v", steps[0])
	}
	if steps[1].Index != 2 || steps[1].Title != "Draft implementation" || steps[1].ToolName != "write_file" {
		t.Fatalf("unexpected second step: %+v", steps[1])
	}
	if steps[1].Input["path"] != "src/server/internal/agent/store.go" {
		t.Fatalf("expected step input to round trip, got %+v", steps[1].Input)
	}

	crossTenantSteps, err := store.ListPlanSteps(ctx, "org_2", run.ID)
	if err != nil {
		t.Fatalf("cross-tenant ListPlanSteps returned error: %v", err)
	}
	if len(crossTenantSteps) != 0 {
		t.Fatalf("expected no cross-tenant plan steps, got %+v", crossTenantSteps)
	}
}

func TestAgentPlanStepStoreUpdatesStatusAndExecutionResult(t *testing.T) {
	store, ctx := testAgentRunSQLStore(t)

	run, err := store.CreateRun(ctx, &CreateRunRequest{
		OrganizationID: "org_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		RequestID:      "req_plan_step_update",
		Status:         RunStatusRunning,
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	step, err := store.CreatePlanStep(ctx, &CreatePlanStepRequest{
		OrganizationID: "org_1",
		RunID:          run.ID,
		Index:          1,
		Title:          "Execute a minimal step",
		Status:         PlanStepStatusPending,
		ApprovalStatus: ApprovalStatusPending,
		ToolName:       "agent_step",
		Input:          map[string]any{"kind": "test"},
	})
	if err != nil {
		t.Fatalf("CreatePlanStep returned error: %v", err)
	}

	startedAt := time.Now().UTC()
	running, err := store.UpdatePlanStep(ctx, "org_1", step.ID, UpdatePlanStepRequest{
		Status:         stringPtr(PlanStepStatusRunning),
		ApprovalStatus: stringPtr(ApprovalStatusApproved),
		StartedAt:      &startedAt,
	})
	if err != nil {
		t.Fatalf("UpdatePlanStep running returned error: %v", err)
	}
	if running.Status != PlanStepStatusRunning || running.ApprovalStatus != ApprovalStatusApproved {
		t.Fatalf("expected running approved step, got %+v", running)
	}
	if running.StartedAt == nil || running.CompletedAt != nil {
		t.Fatalf("expected started step without completion time, got %+v", running)
	}
	if running.Title != "Execute a minimal step" || running.Input["kind"] != "test" {
		t.Fatalf("UpdatePlanStep should preserve step fields, got %+v", running)
	}

	updatedTitle := "Execute adjusted step"
	updatedToolName := "read_file"
	adjusted, err := store.UpdatePlanStep(ctx, "org_1", step.ID, UpdatePlanStepRequest{
		Title:        &updatedTitle,
		ToolName:     &updatedToolName,
		Input:        map[string]any{"path": "new.go"},
		ReplaceInput: true,
	})
	if err != nil {
		t.Fatalf("UpdatePlanStep adjusted fields returned error: %v", err)
	}
	if adjusted.Title != "Execute adjusted step" || adjusted.ToolName != "read_file" || adjusted.Input["path"] != "new.go" {
		t.Fatalf("expected updated plan step draft fields, got %+v", adjusted)
	}

	completedAt := time.Now().UTC()
	completed, err := store.UpdatePlanStep(ctx, "org_1", step.ID, UpdatePlanStepRequest{
		Status:        stringPtr(PlanStepStatusCompleted),
		ResultContent: stringPtr("done"),
		Error:         stringPtr(""),
		CompletedAt:   &completedAt,
	})
	if err != nil {
		t.Fatalf("UpdatePlanStep completed returned error: %v", err)
	}
	if completed.Status != PlanStepStatusCompleted || completed.ResultContent != "done" || completed.Error != "" {
		t.Fatalf("expected completed step result, got %+v", completed)
	}
	if completed.CompletedAt == nil {
		t.Fatalf("expected completed_at to be persisted, got %+v", completed)
	}

	listed, err := store.ListPlanSteps(ctx, "org_1", run.ID)
	if err != nil {
		t.Fatalf("ListPlanSteps returned error: %v", err)
	}
	if len(listed) != 1 || listed[0].Status != PlanStepStatusCompleted || listed[0].ResultContent != "done" {
		t.Fatalf("expected listed completed step with result, got %+v", listed)
	}
	if _, err := store.UpdatePlanStep(ctx, "org_2", step.ID, UpdatePlanStepRequest{Status: stringPtr(PlanStepStatusFailed)}); err == nil {
		t.Fatal("cross-tenant UpdatePlanStep should fail")
	}
}

func TestAgentPlanStepStoreUpdatesIndexUnderUniqueRunOrder(t *testing.T) {
	store, ctx := testAgentRunSQLStore(t)

	run, err := store.CreateRun(ctx, &CreateRunRequest{
		OrganizationID: "org_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		RequestID:      "req_plan_step_reorder",
		Status:         RunStatusPendingApproval,
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	first, err := store.CreatePlanStep(ctx, &CreatePlanStepRequest{
		OrganizationID: "org_1",
		RunID:          run.ID,
		Index:          1,
		Title:          "Draft patch",
		Status:         PlanStepStatusPending,
		ApprovalStatus: ApprovalStatusPending,
	})
	if err != nil {
		t.Fatalf("CreatePlanStep first returned error: %v", err)
	}
	second, err := store.CreatePlanStep(ctx, &CreatePlanStepRequest{
		OrganizationID: "org_1",
		RunID:          run.ID,
		Index:          2,
		Title:          "Verify patch",
		Status:         PlanStepStatusPending,
		ApprovalStatus: ApprovalStatusPending,
	})
	if err != nil {
		t.Fatalf("CreatePlanStep second returned error: %v", err)
	}

	bufferIndex := 3
	if _, err := store.UpdatePlanStep(ctx, "org_1", second.ID, UpdatePlanStepRequest{Index: &bufferIndex}); err != nil {
		t.Fatalf("UpdatePlanStep buffer index returned error: %v", err)
	}
	secondIndex := 2
	if _, err := store.UpdatePlanStep(ctx, "org_1", first.ID, UpdatePlanStepRequest{Index: &secondIndex}); err != nil {
		t.Fatalf("UpdatePlanStep first swap returned error: %v", err)
	}
	firstIndex := 1
	if _, err := store.UpdatePlanStep(ctx, "org_1", second.ID, UpdatePlanStepRequest{Index: &firstIndex}); err != nil {
		t.Fatalf("UpdatePlanStep second swap returned error: %v", err)
	}

	steps, err := store.ListPlanSteps(ctx, "org_1", run.ID)
	if err != nil {
		t.Fatalf("ListPlanSteps returned error: %v", err)
	}
	if len(steps) != 2 || steps[0].ID != second.ID || steps[0].Index != 1 || steps[1].ID != first.ID || steps[1].Index != 2 {
		t.Fatalf("expected reordered plan steps, got %+v", steps)
	}
}

func TestAgentToolRunStorePersistsRiskLevel(t *testing.T) {
	store, ctx := testAgentRunSQLStore(t)

	run, err := store.CreateRun(ctx, &CreateRunRequest{
		OrganizationID: "org_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		RequestID:      "req_tool_run_risk",
		Status:         RunStatusRunning,
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}

	toolRun, err := store.CreateToolRun(ctx, &CreateToolRunRequest{
		OrganizationID: "org_1",
		RunID:          run.ID,
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		ToolCallID:     "call_execute_code_1",
		ToolName:       "execute_code",
		ToolType:       "builtin",
		RiskLevel:      ToolRiskDangerous,
		Arguments:      map[string]any{"language": "python"},
		Status:         ToolRunStatusPendingApproval,
		ApprovalStatus: ApprovalStatusPending,
	})
	if err != nil {
		t.Fatalf("CreateToolRun returned error: %v", err)
	}
	if toolRun.RiskLevel != ToolRiskDangerous {
		t.Fatalf("created tool run risk level = %q, want %q", toolRun.RiskLevel, ToolRiskDangerous)
	}

	got, err := store.GetToolRun(ctx, "org_1", toolRun.ID)
	if err != nil {
		t.Fatalf("GetToolRun returned error: %v", err)
	}
	if got.RiskLevel != ToolRiskDangerous {
		t.Fatalf("GetToolRun risk level = %q, want %q", got.RiskLevel, ToolRiskDangerous)
	}

	listed, err := store.ListToolRuns(ctx, "org_1", run.ID)
	if err != nil {
		t.Fatalf("ListToolRuns returned error: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected 1 listed tool run, got %d", len(listed))
	}
	if listed[0].RiskLevel != ToolRiskDangerous {
		t.Fatalf("ListToolRuns risk level = %q, want %q", listed[0].RiskLevel, ToolRiskDangerous)
	}
}

func TestAgentRunStoreRejectsCrossTenantAccess(t *testing.T) {
	store, ctx := testAgentRunSQLStore(t)

	run, err := store.CreateRun(ctx, &CreateRunRequest{
		OrganizationID: "org_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		RequestID:      "req_cross_tenant",
		Status:         RunStatusRunning,
	})
	if err != nil {
		t.Fatalf("CreateRun returned error: %v", err)
	}
	toolRun, err := store.CreateToolRun(ctx, &CreateToolRunRequest{
		OrganizationID: "org_1",
		RunID:          run.ID,
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		ToolCallID:     "call_cross_tenant",
		ToolName:       "datetime",
		ToolType:       "builtin",
		Arguments:      map[string]any{},
		Status:         ToolRunStatusFailed,
		ApprovalStatus: ApprovalStatusNotRequired,
		AttemptCount:   1,
		Error:          "forced failure",
	})
	if err != nil {
		t.Fatalf("CreateToolRun returned error: %v", err)
	}

	if got, err := store.GetRun(ctx, "org_2", run.ID); err != nil || got != nil {
		t.Fatalf("cross-tenant GetRun got run=%+v err=%v, want nil nil", got, err)
	}
	if got, err := store.ListRuns(ctx, "org_2", "conv_1"); err != nil || len(got) != 0 {
		t.Fatalf("cross-tenant ListRuns got runs=%+v err=%v, want empty nil", got, err)
	}
	if _, err := store.UpdateRun(ctx, "org_2", run.ID, UpdateRunRequest{Status: stringPtr(RunStatusCompleted)}); err == nil {
		t.Fatal("cross-tenant UpdateRun should fail")
	}
	if got, err := store.GetToolRun(ctx, "org_2", toolRun.ID); err != nil || got != nil {
		t.Fatalf("cross-tenant GetToolRun got toolRun=%+v err=%v, want nil nil", got, err)
	}
	if got, err := store.ListToolRuns(ctx, "org_2", run.ID); err != nil || len(got) != 0 {
		t.Fatalf("cross-tenant ListToolRuns got toolRuns=%+v err=%v, want empty nil", got, err)
	}
	if _, err := store.UpdateToolRun(ctx, "org_2", toolRun.ID, UpdateToolRunRequest{Status: stringPtr(ToolRunStatusCompleted)}); err == nil {
		t.Fatal("cross-tenant UpdateToolRun should fail")
	}
}

func TestAgentRunStoreRejectsCrossTenantCreate(t *testing.T) {
	store, ctx := testAgentRunSQLStore(t)

	run, err := store.CreateRun(ctx, &CreateRunRequest{
		OrganizationID: "org_2",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		RequestID:      "req_wrong_tenant",
		Status:         RunStatusRunning,
	})
	if err == nil || run != nil {
		t.Fatalf("CreateRun with mismatched tenant got run=%+v err=%v, want nil error", run, err)
	}

	validRun, err := store.CreateRun(ctx, &CreateRunRequest{
		OrganizationID: "org_1",
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		UserID:         "user_1",
		RequestID:      "req_valid_tenant",
		Status:         RunStatusRunning,
	})
	if err != nil {
		t.Fatalf("CreateRun valid tenant returned error: %v", err)
	}
	toolRun, err := store.CreateToolRun(ctx, &CreateToolRunRequest{
		OrganizationID: "org_2",
		RunID:          validRun.ID,
		ConversationID: "conv_1",
		AgentID:        "agent_1",
		ToolCallID:     "call_wrong_tenant",
		ToolName:       "datetime",
		ToolType:       "builtin",
		Arguments:      map[string]any{},
		Status:         ToolRunStatusRunning,
		ApprovalStatus: ApprovalStatusNotRequired,
		AttemptCount:   1,
	})
	if err == nil || toolRun != nil {
		t.Fatalf("CreateToolRun with mismatched tenant got toolRun=%+v err=%v, want nil error", toolRun, err)
	}
}

func TestAgentMemoryStorePersistsAndFiltersMemories(t *testing.T) {
	store, ctx := testAgentRunSQLStore(t)

	created, err := store.CreateMemory(ctx, &CreateMemoryStoreRequest{
		OrganizationID: "org_1",
		UserID:         "user_1",
		AgentID:        "agent_1",
		Type:           MemoryTypeUserManaged,
		Content:        "I prefer concise answers",
		Metadata:       map[string]any{"topic": "style"},
	})
	if err != nil {
		t.Fatalf("CreateMemory returned error: %v", err)
	}
	if created.ID == "" || created.Type != MemoryTypeUserManaged || created.Metadata["topic"] != "style" {
		t.Fatalf("unexpected created memory: %+v", created)
	}
	_, err = store.CreateMemory(ctx, &CreateMemoryStoreRequest{
		OrganizationID: "org_1",
		UserID:         "user_1",
		AgentID:        "agent_1",
		Type:           MemoryTypeLongTerm,
		Content:        "Use detailed examples",
	})
	if err != nil {
		t.Fatalf("CreateMemory second returned error: %v", err)
	}

	memories, err := store.ListMemories(ctx, "org_1", "user_1", ListMemoriesRequest{
		AgentID: "agent_1",
		Type:    MemoryTypeUserManaged,
		Query:   "concise",
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("ListMemories returned error: %v", err)
	}
	if len(memories) != 1 || memories[0].ID != created.ID {
		t.Fatalf("expected only created user-managed memory, got %+v", memories)
	}

	crossTenant, err := store.ListMemories(ctx, "org_2", "user_1", ListMemoriesRequest{Limit: 10})
	if err != nil {
		t.Fatalf("cross-tenant ListMemories returned error: %v", err)
	}
	if len(crossTenant) != 0 {
		t.Fatalf("expected cross-tenant list to be empty, got %+v", crossTenant)
	}
}

func stringPtr(value string) *string {
	return &value
}

func intPtr(value int) *int {
	return &value
}

func assertAgentApprovalAndToolRisk(t *testing.T, got *Agent, wantApprovalMode, wantRiskLevel string, wantRequiresApproval bool) {
	t.Helper()

	if got == nil {
		t.Fatal("expected agent, got nil")
	}
	if got.Config.ApprovalMode != wantApprovalMode {
		t.Fatalf("approval mode = %q, want %q; agent=%+v", got.Config.ApprovalMode, wantApprovalMode, got)
	}
	if len(got.Tools) != 1 && len(got.Tools) != 2 {
		t.Fatalf("expected persisted tools, got %+v", got.Tools)
	}

	foundRiskLevel := false
	for _, tool := range got.Tools {
		if tool.RiskLevel == wantRiskLevel {
			foundRiskLevel = true
			break
		}
	}
	if !foundRiskLevel {
		t.Fatalf("expected a tool risk level %q, got %+v", wantRiskLevel, got.Tools)
	}

	for _, override := range got.Config.ToolApprovalOverrides {
		if override.RiskLevel != wantRiskLevel {
			continue
		}
		if override.RequiresApproval == nil {
			t.Fatalf("requires approval override was not persisted: %+v", got.Config.ToolApprovalOverrides)
		}
		if *override.RequiresApproval != wantRequiresApproval {
			t.Fatalf("requires approval = %v, want %v", *override.RequiresApproval, wantRequiresApproval)
		}
		return
	}
	t.Fatalf("expected override risk level %q, got %+v", wantRiskLevel, got.Config.ToolApprovalOverrides)
}

func TestMarshalToolCallsRoundTrip(t *testing.T) {
	original := []ToolCall{
		{ID: "call_1", Name: "weather", Arguments: map[string]any{"city": "Beijing"}},
		{ID: "call_2", Name: "datetime", Arguments: map[string]any{}},
	}

	data := MarshalToolCalls(original)
	if len(data) == 0 {
		t.Fatal("MarshalToolCalls should return non-empty JSON for non-empty input")
	}

	result := UnmarshalToolCalls(data)
	if len(result) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(result))
	}
	if result[0].ID != "call_1" || result[0].Name != "weather" {
		t.Fatalf("unexpected first tool call: %+v", result[0])
	}
	if result[1].ID != "call_2" || result[1].Name != "datetime" {
		t.Fatalf("unexpected second tool call: %+v", result[1])
	}
}

func TestMarshalToolCallsEmpty(t *testing.T) {
	if data := MarshalToolCalls(nil); data != nil {
		t.Fatal("MarshalToolCalls should return nil for nil input")
	}
	if data := MarshalToolCalls([]ToolCall{}); data != nil {
		t.Fatal("MarshalToolCalls should return nil for empty slice")
	}
}

func TestUnmarshalToolCallsEmpty(t *testing.T) {
	if result := UnmarshalToolCalls(nil); result != nil {
		t.Fatal("UnmarshalToolCalls should return nil for nil data")
	}
	if result := UnmarshalToolCalls([]byte{}); result != nil {
		t.Fatal("UnmarshalToolCalls should return nil for empty data")
	}
}

func TestHasEnabledTools(t *testing.T) {
	if hasEnabledTools(nil) {
		t.Fatal("nil agent should not have enabled tools")
	}

	agent := &Agent{Tools: []Tool{}}
	if hasEnabledTools(agent) {
		t.Fatal("agent with no tools should not have enabled tools")
	}

	agent = &Agent{Tools: []Tool{
		{Name: "datetime", Type: "builtin", Enabled: false},
	}}
	if hasEnabledTools(agent) {
		t.Fatal("agent with only disabled tools should not be detected")
	}

	agent = &Agent{Tools: []Tool{
		{Name: "disabled", Type: "builtin", Enabled: false},
		{Name: "enabled", Type: "builtin", Enabled: true},
	}}
	if !hasEnabledTools(agent) {
		t.Fatal("agent with at least one enabled tool should be detected")
	}

	agent = &Agent{Tools: []Tool{
		{Name: "mcp_tool", Type: "mcp", Enabled: true},
	}}
	if !hasEnabledTools(agent) {
		t.Fatal("agent with enabled MCP tool should be detected")
	}
}

func TestStreamContentSplitsWords(t *testing.T) {
	var chunks []string
	err := streamContent("hello world", func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 2 {
		t.Fatalf("expected 2 chunks for 'hello world', got %d: %v", len(chunks), chunks)
	}
	if chunks[0] != "hello " || chunks[1] != "world" {
		t.Fatalf("unexpected chunks: %v", chunks)
	}
}

func TestStreamContentSingleWord(t *testing.T) {
	var chunks []string
	err := streamContent("ok", func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for 'ok', got %d", len(chunks))
	}
	if chunks[0] != "ok" {
		t.Fatalf("unexpected chunk: %q", chunks[0])
	}
}

func TestStreamContentEmpty(t *testing.T) {
	err := streamContent("", func(chunk string) error {
		t.Fatal("should not call onChunk for empty content")
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestToolCallsToChatToolCalls(t *testing.T) {
	input := []ToolCall{
		{ID: "call_1", Name: "weather.lookup", Arguments: map[string]any{"city": "Paris"}},
		{ID: "call_2", Name: "datetime", Arguments: map[string]any{}},
	}

	result := toolCallsToChatToolCalls(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 chat tool calls, got %d", len(result))
	}
	if result[0].ID != "call_1" || result[0].Type != "function" {
		t.Fatalf("unexpected first chat tool call: %+v", result[0])
	}
	if result[0].Function.Name != "weather.lookup" {
		t.Fatalf("expected weather.lookup, got %q", result[0].Function.Name)
	}
	if result[0].Function.Arguments == "" {
		t.Fatal("arguments should be serialized JSON string")
	}
}

func TestToolCallsToChatToolCallsEmpty(t *testing.T) {
	if result := toolCallsToChatToolCalls(nil); result != nil {
		t.Fatal("should return nil for nil input")
	}
	if result := toolCallsToChatToolCalls([]ToolCall{}); result != nil {
		t.Fatal("should return nil for empty input")
	}
}

func TestChatToolCallsToAgent(t *testing.T) {
	input := []chat.ToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: chat.ToolFunction{
				Name:      "weather.lookup",
				Arguments: `{"city":"Paris"}`,
			},
		},
		{
			ID:   "call_2",
			Type: "function",
			Function: chat.ToolFunction{
				Name:      "datetime",
				Arguments: "",
			},
		},
	}

	result := chatToolCallsToAgent(input)
	if len(result) != 2 {
		t.Fatalf("expected 2 agent tool calls, got %d", len(result))
	}
	if result[0].ID != "call_1" || result[0].Name != "weather.lookup" {
		t.Fatalf("unexpected first agent tool call: %+v", result[0])
	}
	city, ok := result[0].Arguments["city"].(string)
	if !ok || city != "Paris" {
		t.Fatalf("expected city=Paris in arguments, got %+v", result[0].Arguments)
	}
	if result[1].Arguments == nil {
		t.Fatal("empty arguments should result in empty map, not nil")
	}
}

func TestChatToolCallsToAgentEmpty(t *testing.T) {
	if result := chatToolCallsToAgent(nil); result != nil {
		t.Fatal("should return nil for nil input")
	}
	if result := chatToolCallsToAgent([]chat.ToolCall{}); result != nil {
		t.Fatal("should return nil for empty input")
	}
}

func TestParseToolCallsFromResponse(t *testing.T) {
	// Simulates a raw LLM response map containing tool_calls.
	response := map[string]any{
		"tool_calls": []any{
			map[string]any{
				"id":   "call_abc",
				"type": "function",
				"function": map[string]any{
					"name":      "datetime",
					"arguments": `{}`,
				},
			},
			map[string]any{
				"id":   "call_def",
				"type": "function",
				"function": map[string]any{
					"name":      "web_search",
					"arguments": `{"query":"golang news"}`,
				},
			},
		},
	}

	toolCalls, err := ParseToolCallsFromResponse(response)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(toolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(toolCalls))
	}
	if toolCalls[0].ID != "call_abc" || toolCalls[0].Name != "datetime" {
		t.Fatalf("unexpected first tool call: %+v", toolCalls[0])
	}
	if toolCalls[1].ID != "call_def" || toolCalls[1].Name != "web_search" {
		t.Fatalf("unexpected second tool call: %+v", toolCalls[1])
	}
	if toolCalls[1].Arguments["query"] != "golang news" {
		t.Fatalf("unexpected arguments: %+v", toolCalls[1].Arguments)
	}
}

func TestParseToolCallsFromResponseNoToolCalls(t *testing.T) {
	response := map[string]any{
		"content": "simple text response",
	}

	toolCalls, err := ParseToolCallsFromResponse(response)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if toolCalls != nil {
		t.Fatalf("expected nil tool calls, got %+v", toolCalls)
	}
}

func TestParseToolCallsFromResponseNilResponse(t *testing.T) {
	toolCalls, err := ParseToolCallsFromResponse(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if toolCalls != nil {
		t.Fatal("expected nil tool calls for nil response")
	}
}

func TestParseToolCallsFromResponseMalformedEntries(t *testing.T) {
	// Missing function field should be skipped gracefully.
	response := map[string]any{
		"tool_calls": []any{
			map[string]any{
				"id": "call_skip",
			},
			map[string]any{
				"id":   "call_good",
				"type": "function",
				"function": map[string]any{
					"name":      "datetime",
					"arguments": `{}`,
				},
			},
		},
	}

	toolCalls, err := ParseToolCallsFromResponse(response)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call after skipping malformed, got %d", len(toolCalls))
	}
	if toolCalls[0].ID != "call_good" {
		t.Fatalf("expected call_good, got %q", toolCalls[0].ID)
	}
}
