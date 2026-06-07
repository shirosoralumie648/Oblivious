package schedule

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

func testScheduleSQLStore(t *testing.T) (*SQLStore, context.Context) {
	t.Helper()

	databaseURL := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL"))
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is required for DB-backed schedule tests")
	}

	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.Ping(); err != nil {
		t.Fatalf("ping database: %v", err)
	}
	t.Cleanup(func() {
		database.Close()
	})

	if _, err := database.Exec(`SELECT pg_advisory_lock(104210)`); err != nil {
		t.Fatalf("lock schedule test database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := database.Exec(`SELECT pg_advisory_unlock(104210)`); err != nil {
			t.Fatalf("unlock schedule test database: %v", err)
		}
	})

	statements := []string{
		`DROP TABLE IF EXISTS scheduled_task_runs CASCADE`,
		`DROP TABLE IF EXISTS scheduled_tasks CASCADE`,
		`DROP TABLE IF EXISTS organizations CASCADE`,
		`CREATE TABLE organizations (id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, name TEXT NOT NULL, status TEXT NOT NULL DEFAULT 'active', created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW())`,
		`INSERT INTO organizations (id, slug, name) VALUES ('org_1', 'schedule-org-1', 'Schedule Org 1'), ('org_2', 'schedule-org-2', 'Schedule Org 2')`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			t.Fatalf("prepare schedule database: %v\nstatement: %s", err, statement)
		}
	}

	for _, migrationPath := range []string{
		"../../migrations/0045_scheduled_tasks.sql",
		"../../migrations/0063_scheduled_task_workflow_trigger_id.sql",
	} {
		migration, err := os.ReadFile(migrationPath)
		if err != nil {
			t.Fatalf("read scheduled task migration %s: %v", migrationPath, err)
		}
		if _, err := database.Exec(string(migration)); err != nil {
			t.Fatalf("apply scheduled task migration %s: %v", migrationPath, err)
		}
	}

	return NewSQLStore(database), context.Background()
}

func TestSQLStoreCreatesAndListsScheduledTasksByOrganization(t *testing.T) {
	store, ctx := testScheduleSQLStore(t)
	nextRunAt := time.Date(2026, time.June, 5, 9, 0, 0, 0, time.UTC)

	workflowTask, err := store.CreateScheduledTask(ctx, CreateScheduledTaskInput{
		OrganizationID: "org_1",
		Name:           "Hourly workflow",
		TargetType:     TargetTypeWorkflow,
		TargetID:       "workflow_1",
		CronExpression: "0 * * * *",
		Enabled:        true,
		NextRunAt:      &nextRunAt,
	})
	if err != nil {
		t.Fatalf("CreateScheduledTask workflow returned error: %v", err)
	}
	if workflowTask.ID == "" || workflowTask.OrganizationID != "org_1" || workflowTask.TargetType != TargetTypeWorkflow {
		t.Fatalf("unexpected workflow scheduled task: %+v", workflowTask)
	}
	if workflowTask.Name != "Hourly workflow" {
		t.Fatalf("expected workflow scheduled task name to persist, got %+v", workflowTask)
	}
	if workflowTask.NextRunAt == nil || !workflowTask.NextRunAt.Equal(nextRunAt) {
		t.Fatalf("expected persisted next run at %v, got %v", nextRunAt, workflowTask.NextRunAt)
	}
	if !workflowTask.Enabled {
		t.Fatalf("expected workflow task to be enabled: %+v", workflowTask)
	}

	agentTask, err := store.CreateScheduledTask(ctx, CreateScheduledTaskInput{
		OrganizationID: "org_1",
		Name:           "Weekly agent",
		TargetType:     TargetTypeAgent,
		TargetID:       "agent_1",
		CronExpression: "30 9 * * 1",
		Enabled:        false,
	})
	if err != nil {
		t.Fatalf("CreateScheduledTask agent returned error: %v", err)
	}
	if agentTask.Enabled {
		t.Fatalf("expected agent task to be disabled: %+v", agentTask)
	}
	if agentTask.NextRunAt != nil {
		t.Fatalf("expected disabled task to have no next run: %+v", agentTask)
	}

	if _, err := store.CreateScheduledTask(ctx, CreateScheduledTaskInput{
		OrganizationID: "org_2",
		Name:           "Other org workflow",
		TargetType:     TargetTypeWorkflow,
		TargetID:       "workflow_other",
		CronExpression: "0 0 * * *",
		Enabled:        true,
	}); err != nil {
		t.Fatalf("CreateScheduledTask other org returned error: %v", err)
	}

	tasks, err := store.ListScheduledTasks(ctx, "org_1")
	if err != nil {
		t.Fatalf("ListScheduledTasks returned error: %v", err)
	}
	if len(tasks) != 2 {
		t.Fatalf("expected 2 org_1 tasks, got %+v", tasks)
	}
	for _, task := range tasks {
		if task.OrganizationID != "org_1" {
			t.Fatalf("list leaked cross-org task: %+v", task)
		}
	}
	if tasks[0].ID != workflowTask.ID || tasks[0].Name != "Hourly workflow" || tasks[0].NextRunAt == nil || !tasks[0].NextRunAt.Equal(nextRunAt) {
		t.Fatalf("expected enabled task with next run first, got %+v", tasks)
	}
}

func TestSQLStoreSyncsWorkflowTriggerBackedScheduledTasksWithoutTouchingManualTasks(t *testing.T) {
	store, ctx := testScheduleSQLStore(t)
	now := time.Date(2026, time.June, 5, 8, 0, 0, 0, time.UTC)
	firstNextRun := time.Date(2026, time.June, 5, 9, 0, 0, 0, time.UTC)
	secondNextRun := time.Date(2026, time.June, 6, 10, 0, 0, 0, time.UTC)

	manualTask, err := store.CreateScheduledTask(ctx, CreateScheduledTaskInput{
		OrganizationID: "org_1",
		Name:           "Manual workflow",
		TargetType:     TargetTypeWorkflow,
		TargetID:       "workflow_1",
		CronExpression: "15 * * * *",
		Enabled:        true,
		NextRunAt:      &firstNextRun,
	})
	if err != nil {
		t.Fatalf("CreateScheduledTask manual returned error: %v", err)
	}

	synced, err := store.SyncWorkflowScheduledTasks(ctx, SyncWorkflowScheduledTasksInput{
		OrganizationID: "org_1",
		WorkflowID:     "workflow_1",
		Triggers: []SyncWorkflowScheduledTaskTriggerInput{
			{
				TriggerID:      "hourly",
				Name:           "Hourly trigger",
				CronExpression: "0 * * * *",
				Enabled:        true,
				NextRunAt:      &firstNextRun,
			},
			{
				TriggerID:      "weekday",
				Name:           "Weekday trigger",
				CronExpression: "0 9 * * 1",
				Enabled:        false,
			},
		},
		Now: now,
	})
	if err != nil {
		t.Fatalf("SyncWorkflowScheduledTasks first returned error: %v", err)
	}
	if len(synced) != 2 {
		t.Fatalf("expected two synced tasks, got %+v", synced)
	}
	firstIDByTrigger := map[string]string{}
	for _, task := range synced {
		firstIDByTrigger[task.WorkflowTriggerID] = task.ID
		if task.TargetType != TargetTypeWorkflow || task.TargetID != "workflow_1" {
			t.Fatalf("unexpected synced task target: %+v", task)
		}
	}
	if firstIDByTrigger["hourly"] == "" || firstIDByTrigger["weekday"] == "" {
		t.Fatalf("expected trigger ids to be persisted, got %+v", synced)
	}

	resynced, err := store.SyncWorkflowScheduledTasks(ctx, SyncWorkflowScheduledTasksInput{
		OrganizationID: "org_1",
		WorkflowID:     "workflow_1",
		Triggers: []SyncWorkflowScheduledTaskTriggerInput{
			{
				TriggerID:      "hourly",
				Name:           "Updated hourly trigger",
				CronExpression: "30 * * * *",
				Enabled:        true,
				NextRunAt:      &secondNextRun,
			},
		},
		Now: now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("SyncWorkflowScheduledTasks second returned error: %v", err)
	}
	if len(resynced) != 1 {
		t.Fatalf("expected one remaining synced task, got %+v", resynced)
	}
	if resynced[0].ID != firstIDByTrigger["hourly"] {
		t.Fatalf("expected hourly trigger to update existing task %q, got %+v", firstIDByTrigger["hourly"], resynced[0])
	}
	if resynced[0].Name != "Updated hourly trigger" || resynced[0].CronExpression != "30 * * * *" || resynced[0].NextRunAt == nil || !resynced[0].NextRunAt.Equal(secondNextRun) {
		t.Fatalf("expected hourly task cron/next run to update, got %+v", resynced[0])
	}

	tasks, err := store.ListScheduledTasks(ctx, "org_1")
	if err != nil {
		t.Fatalf("ListScheduledTasks returned error: %v", err)
	}
	taskIDs := map[string]ScheduledTask{}
	for _, task := range tasks {
		taskIDs[task.ID] = task
	}
	if _, ok := taskIDs[manualTask.ID]; !ok {
		t.Fatalf("manual task should not be deleted by workflow trigger sync, tasks=%+v", tasks)
	}
	if _, ok := taskIDs[firstIDByTrigger["weekday"]]; ok {
		t.Fatalf("stale workflow trigger task should be deleted, tasks=%+v", tasks)
	}
}

func TestSQLStoreGetsAndUpdatesScheduledTaskEnabledState(t *testing.T) {
	store, ctx := testScheduleSQLStore(t)
	nextRunAt := time.Date(2026, time.June, 5, 10, 0, 0, 0, time.UTC)

	task, err := store.CreateScheduledTask(ctx, CreateScheduledTaskInput{
		OrganizationID: "org_1",
		Name:           "Toggle workflow",
		TargetType:     TargetTypeWorkflow,
		TargetID:       "workflow_1",
		CronExpression: "0 * * * *",
		Enabled:        false,
	})
	if err != nil {
		t.Fatalf("CreateScheduledTask returned error: %v", err)
	}

	got, err := store.GetScheduledTask(ctx, "org_1", task.ID)
	if err != nil {
		t.Fatalf("GetScheduledTask returned error: %v", err)
	}
	if got.ID != task.ID || got.OrganizationID != "org_1" || got.Name != "Toggle workflow" || got.Enabled {
		t.Fatalf("unexpected fetched task: %+v", got)
	}

	enabledTask, err := store.UpdateScheduledTaskEnabled(ctx, "org_1", task.ID, UpdateScheduledTaskEnabledInput{
		Enabled:   true,
		NextRunAt: &nextRunAt,
	})
	if err != nil {
		t.Fatalf("UpdateScheduledTaskEnabled enable returned error: %v", err)
	}
	if !enabledTask.Enabled || enabledTask.NextRunAt == nil || !enabledTask.NextRunAt.Equal(nextRunAt) {
		t.Fatalf("expected task enabled with next run %v, got %+v", nextRunAt, enabledTask)
	}
	if enabledTask.Name != "Toggle workflow" {
		t.Fatalf("expected update response to retain scheduled task name, got %+v", enabledTask)
	}

	disabledTask, err := store.UpdateScheduledTaskEnabled(ctx, "org_1", task.ID, UpdateScheduledTaskEnabledInput{
		Enabled:   false,
		NextRunAt: nil,
	})
	if err != nil {
		t.Fatalf("UpdateScheduledTaskEnabled disable returned error: %v", err)
	}
	if disabledTask.Enabled || disabledTask.NextRunAt != nil {
		t.Fatalf("expected disabled task with no next run, got %+v", disabledTask)
	}

	if _, err := store.GetScheduledTask(ctx, "org_2", task.ID); err == nil {
		t.Fatalf("expected cross-org GetScheduledTask to fail")
	}
}

func TestSQLStoreRecordsAndListsScheduledTaskRunsByOrganizationAndTask(t *testing.T) {
	store, ctx := testScheduleSQLStore(t)
	startedAt := time.Date(2026, time.June, 5, 9, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(3 * time.Minute)

	orgTask, err := store.CreateScheduledTask(ctx, CreateScheduledTaskInput{
		OrganizationID: "org_1",
		Name:           "Run history workflow",
		TargetType:     TargetTypeWorkflow,
		TargetID:       "workflow_1",
		CronExpression: "0 * * * *",
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("CreateScheduledTask org task returned error: %v", err)
	}
	otherOrgTask, err := store.CreateScheduledTask(ctx, CreateScheduledTaskInput{
		OrganizationID: "org_2",
		Name:           "Other org workflow",
		TargetType:     TargetTypeWorkflow,
		TargetID:       "workflow_other",
		CronExpression: "0 * * * *",
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("CreateScheduledTask other org task returned error: %v", err)
	}

	firstRun, err := store.RecordScheduledTaskRun(ctx, RecordScheduledTaskRunInput{
		OrganizationID:  "org_1",
		ScheduledTaskID: orgTask.ID,
		Status:          RunStatusRunning,
		StartedAt:       &startedAt,
	})
	if err != nil {
		t.Fatalf("RecordScheduledTaskRun first returned error: %v", err)
	}
	if firstRun.ID == "" || firstRun.OrganizationID != "org_1" || firstRun.ScheduledTaskID != orgTask.ID || firstRun.Status != RunStatusRunning {
		t.Fatalf("unexpected first run: %+v", firstRun)
	}
	if firstRun.StartedAt == nil || !firstRun.StartedAt.Equal(startedAt) {
		t.Fatalf("expected started at %v, got %v", startedAt, firstRun.StartedAt)
	}

	secondRun, err := store.RecordScheduledTaskRun(ctx, RecordScheduledTaskRunInput{
		OrganizationID:  "org_1",
		ScheduledTaskID: orgTask.ID,
		Status:          RunStatusFailed,
		StartedAt:       &startedAt,
		FinishedAt:      &finishedAt,
		Error:           "workflow failed",
	})
	if err != nil {
		t.Fatalf("RecordScheduledTaskRun second returned error: %v", err)
	}
	if secondRun.FinishedAt == nil || !secondRun.FinishedAt.Equal(finishedAt) || secondRun.Error != "workflow failed" {
		t.Fatalf("unexpected second run fields: %+v", secondRun)
	}

	if _, err := store.RecordScheduledTaskRun(ctx, RecordScheduledTaskRunInput{
		OrganizationID:  "org_2",
		ScheduledTaskID: otherOrgTask.ID,
		Status:          RunStatusCompleted,
		StartedAt:       &startedAt,
		FinishedAt:      &finishedAt,
	}); err != nil {
		t.Fatalf("RecordScheduledTaskRun other org returned error: %v", err)
	}

	runs, err := store.ListScheduledTaskRuns(ctx, "org_1", orgTask.ID)
	if err != nil {
		t.Fatalf("ListScheduledTaskRuns returned error: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("expected 2 org_1 task runs, got %+v", runs)
	}
	for _, run := range runs {
		if run.OrganizationID != "org_1" || run.ScheduledTaskID != orgTask.ID {
			t.Fatalf("list leaked cross-org or cross-task run: %+v", run)
		}
	}
	if runs[0].ID != secondRun.ID || runs[1].ID != firstRun.ID {
		t.Fatalf("expected newest run first, got %+v", runs)
	}
}

func TestSQLStoreClaimsDueScheduledTaskRunsOnceAndRecordsRunningRuns(t *testing.T) {
	store, ctx := testScheduleSQLStore(t)
	now := time.Date(2026, time.June, 5, 9, 0, 0, 0, time.UTC)
	dueAt := now.Add(-time.Minute)
	futureAt := now.Add(time.Hour)

	dueTask, err := store.CreateScheduledTask(ctx, CreateScheduledTaskInput{
		OrganizationID: "org_1",
		Name:           "Due workflow",
		TargetType:     TargetTypeWorkflow,
		TargetID:       "workflow_due",
		CronExpression: "0 * * * *",
		Enabled:        true,
		NextRunAt:      &dueAt,
	})
	if err != nil {
		t.Fatalf("CreateScheduledTask due returned error: %v", err)
	}
	if _, err := store.CreateScheduledTask(ctx, CreateScheduledTaskInput{
		OrganizationID: "org_1",
		Name:           "Future workflow",
		TargetType:     TargetTypeWorkflow,
		TargetID:       "workflow_future",
		CronExpression: "0 * * * *",
		Enabled:        true,
		NextRunAt:      &futureAt,
	}); err != nil {
		t.Fatalf("CreateScheduledTask future returned error: %v", err)
	}
	if _, err := store.CreateScheduledTask(ctx, CreateScheduledTaskInput{
		OrganizationID: "org_1",
		Name:           "Disabled workflow",
		TargetType:     TargetTypeWorkflow,
		TargetID:       "workflow_disabled",
		CronExpression: "0 * * * *",
		Enabled:        false,
		NextRunAt:      &dueAt,
	}); err != nil {
		t.Fatalf("CreateScheduledTask disabled returned error: %v", err)
	}

	claimed, err := store.ClaimDueScheduledTaskRuns(ctx, ClaimDueScheduledTaskRunsInput{Now: now, Limit: 10})
	if err != nil {
		t.Fatalf("ClaimDueScheduledTaskRuns returned error: %v", err)
	}
	if len(claimed) != 1 {
		t.Fatalf("expected only due enabled task to be claimed, got %+v", claimed)
	}
	if claimed[0].Task.ID != dueTask.ID || claimed[0].Task.Name != "Due workflow" || claimed[0].Run.ScheduledTaskID != dueTask.ID || claimed[0].Run.Status != RunStatusRunning {
		t.Fatalf("unexpected claimed task/run: %+v", claimed[0])
	}
	if claimed[0].Run.StartedAt == nil || !claimed[0].Run.StartedAt.Equal(now) {
		t.Fatalf("expected claimed run started at %v, got %v", now, claimed[0].Run.StartedAt)
	}

	runs, err := store.ListScheduledTaskRuns(ctx, "org_1", dueTask.ID)
	if err != nil {
		t.Fatalf("ListScheduledTaskRuns returned error: %v", err)
	}
	if len(runs) != 1 || runs[0].Status != RunStatusRunning {
		t.Fatalf("expected running run to be recorded, got %+v", runs)
	}

	claimedAgain, err := store.ClaimDueScheduledTaskRuns(ctx, ClaimDueScheduledTaskRunsInput{Now: now, Limit: 10})
	if err != nil {
		t.Fatalf("ClaimDueScheduledTaskRuns second call returned error: %v", err)
	}
	if len(claimedAgain) != 0 {
		t.Fatalf("expected running due task not to be claimed twice, got %+v", claimedAgain)
	}
}

func TestSQLStoreCompletesManualScheduledTaskRunWithoutAdvancingNextRun(t *testing.T) {
	store, ctx := testScheduleSQLStore(t)
	nextRunAt := time.Date(2026, time.June, 5, 10, 0, 0, 0, time.UTC)
	startedAt := time.Date(2026, time.June, 5, 9, 10, 0, 0, time.UTC)
	finishedAt := startedAt.Add(2 * time.Minute)

	task, err := store.CreateScheduledTask(ctx, CreateScheduledTaskInput{
		OrganizationID: "org_1",
		Name:           "Manual completion workflow",
		TargetType:     TargetTypeWorkflow,
		TargetID:       "workflow_due",
		CronExpression: "0 * * * *",
		Enabled:        true,
		NextRunAt:      &nextRunAt,
	})
	if err != nil {
		t.Fatalf("CreateScheduledTask returned error: %v", err)
	}
	run, err := store.RecordScheduledTaskRun(ctx, RecordScheduledTaskRunInput{
		OrganizationID:  "org_1",
		ScheduledTaskID: task.ID,
		Status:          RunStatusRunning,
		StartedAt:       &startedAt,
	})
	if err != nil {
		t.Fatalf("RecordScheduledTaskRun returned error: %v", err)
	}

	completed, err := store.CompleteManualScheduledTaskRun(ctx, "org_1", task.ID, run.ID, finishedAt)
	if err != nil {
		t.Fatalf("CompleteManualScheduledTaskRun returned error: %v", err)
	}
	if completed.Status != RunStatusCompleted || completed.FinishedAt == nil || !completed.FinishedAt.Equal(finishedAt) {
		t.Fatalf("unexpected completed manual run: %+v", completed)
	}

	tasks, err := store.ListScheduledTasks(ctx, "org_1")
	if err != nil {
		t.Fatalf("ListScheduledTasks returned error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected one task, got %+v", tasks)
	}
	if tasks[0].LastRunAt == nil || !tasks[0].LastRunAt.Equal(finishedAt) {
		t.Fatalf("expected last_run_at %v, got %v", finishedAt, tasks[0].LastRunAt)
	}
	if tasks[0].NextRunAt == nil || !tasks[0].NextRunAt.Equal(nextRunAt) {
		t.Fatalf("expected manual run not to advance next_run_at %v, got %v", nextRunAt, tasks[0].NextRunAt)
	}
}

func TestSQLStoreCompletesScheduledTaskRunAndAdvancesTask(t *testing.T) {
	store, ctx := testScheduleSQLStore(t)
	now := time.Date(2026, time.June, 5, 9, 0, 0, 0, time.UTC)
	nextRunAt := time.Date(2026, time.June, 5, 10, 0, 0, 0, time.UTC)

	task, err := store.CreateScheduledTask(ctx, CreateScheduledTaskInput{
		OrganizationID: "org_1",
		Name:           "Scheduled completion workflow",
		TargetType:     TargetTypeWorkflow,
		TargetID:       "workflow_due",
		CronExpression: "0 * * * *",
		Enabled:        true,
		NextRunAt:      &now,
	})
	if err != nil {
		t.Fatalf("CreateScheduledTask returned error: %v", err)
	}
	claimed, err := store.ClaimDueScheduledTaskRuns(ctx, ClaimDueScheduledTaskRunsInput{Now: now, Limit: 1})
	if err != nil {
		t.Fatalf("ClaimDueScheduledTaskRuns returned error: %v", err)
	}
	if len(claimed) != 1 {
		t.Fatalf("expected one claimed run, got %+v", claimed)
	}

	completed, err := store.CompleteScheduledTaskRun(ctx, "org_1", task.ID, claimed[0].Run.ID, CompleteScheduledTaskRunInput{
		FinishedAt: now,
		NextRunAt:  nextRunAt,
	})
	if err != nil {
		t.Fatalf("CompleteScheduledTaskRun returned error: %v", err)
	}
	if completed.Status != RunStatusCompleted || completed.FinishedAt == nil || !completed.FinishedAt.Equal(now) {
		t.Fatalf("unexpected completed run: %+v", completed)
	}

	tasks, err := store.ListScheduledTasks(ctx, "org_1")
	if err != nil {
		t.Fatalf("ListScheduledTasks returned error: %v", err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected one task, got %+v", tasks)
	}
	if tasks[0].LastRunAt == nil || !tasks[0].LastRunAt.Equal(now) {
		t.Fatalf("expected last_run_at %v, got %v", now, tasks[0].LastRunAt)
	}
	if tasks[0].NextRunAt == nil || !tasks[0].NextRunAt.Equal(nextRunAt) {
		t.Fatalf("expected next_run_at %v, got %v", nextRunAt, tasks[0].NextRunAt)
	}
}
