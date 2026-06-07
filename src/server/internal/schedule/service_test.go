package schedule

import (
	"context"
	"errors"
	"testing"
	"time"

	"oblivious/server/internal/agent"
	"oblivious/server/internal/auth"
	"oblivious/server/internal/workflow"
)

type fakeStore struct {
	createdInput          CreateScheduledTaskInput
	createdTask           ScheduledTask
	gotTaskOrgID          string
	gotTaskID             string
	gotTask               ScheduledTask
	updateEnabledOrgID    string
	updateEnabledTaskID   string
	updateEnabledInput    UpdateScheduledTaskEnabledInput
	updatedEnabledTask    ScheduledTask
	claimedInput          ClaimDueScheduledTaskRunsInput
	claimedRuns           []ClaimedScheduledTaskRun
	completedRunInput     CompleteScheduledTaskRunInput
	completedRunID        string
	completedTaskID       string
	completedManualOrgID  string
	completedManualTaskID string
	completedManualRunID  string
	completedManualAt     time.Time
	completedManualRun    ScheduledTaskRun
	recordedRunInput      RecordScheduledTaskRunInput
	recordedRun           ScheduledTaskRun
	syncedWorkflowInput   SyncWorkflowScheduledTasksInput
	syncedWorkflowTasks   []ScheduledTask
	syncWorkflowErr       error
	runningRunCount       int
	listedOrgID           string
	listedTasks           []ScheduledTask
	listedRunsOrgID       string
	listedRunsTaskID      string
	listedRuns            []ScheduledTaskRun
	createCalls           int
	syncWorkflowCalls     int
	recordRunCalls        int
	listTaskCalls         int
	listScheduledRunCalls int
	getTaskCalls          int
	updateEnabledCalls    int
	claimDueCalls         int
	completeRunCalls      int
	completeManualCalls   int
	updatedRunID          string
	updatedRunInput       UpdateScheduledTaskRunInput
	updateRunCalls        int
}

type fakeWorkflowStarter struct {
	request        workflow.StartExecutionRequest
	runExecutionID string
	result         *workflow.WorkflowExecution
	runResult      *workflow.WorkflowExecution
	err            error
	runErr         error
	calls          int
	runCalls       int
}

type fakeAgentStarter struct {
	request agent.StartRunRequest
	session auth.Session
	result  *agent.RunWithMessages
	err     error
	calls   int
}

func (f *fakeWorkflowStarter) StartExecution(ctx context.Context, request workflow.StartExecutionRequest) (*workflow.WorkflowExecution, error) {
	f.calls++
	f.request = request
	return f.result, f.err
}

func (f *fakeWorkflowStarter) RunExecutionUntilBlocked(ctx context.Context, organizationID, executionID string) (*workflow.WorkflowExecution, error) {
	f.runCalls++
	f.runExecutionID = executionID
	if f.runResult != nil || f.runErr != nil {
		return f.runResult, f.runErr
	}
	return f.result, nil
}

func (f *fakeAgentStarter) StartRun(ctx context.Context, session auth.Session, request agent.StartRunRequest) (*agent.RunWithMessages, error) {
	f.calls++
	f.session = session
	f.request = request
	return f.result, f.err
}

func (f *fakeStore) CreateScheduledTask(ctx context.Context, input CreateScheduledTaskInput) (ScheduledTask, error) {
	f.createCalls++
	f.createdInput = input
	return f.createdTask, nil
}

func (f *fakeStore) SyncWorkflowScheduledTasks(ctx context.Context, input SyncWorkflowScheduledTasksInput) ([]ScheduledTask, error) {
	f.syncWorkflowCalls++
	f.syncedWorkflowInput = input
	return f.syncedWorkflowTasks, f.syncWorkflowErr
}

func (f *fakeStore) ListScheduledTasks(ctx context.Context, organizationID string) ([]ScheduledTask, error) {
	f.listTaskCalls++
	f.listedOrgID = organizationID
	return f.listedTasks, nil
}

func (f *fakeStore) GetScheduledTask(ctx context.Context, organizationID string, scheduledTaskID string) (ScheduledTask, error) {
	f.getTaskCalls++
	f.gotTaskOrgID = organizationID
	f.gotTaskID = scheduledTaskID
	return f.gotTask, nil
}

func (f *fakeStore) UpdateScheduledTaskEnabled(ctx context.Context, organizationID string, scheduledTaskID string, input UpdateScheduledTaskEnabledInput) (ScheduledTask, error) {
	f.updateEnabledCalls++
	f.updateEnabledOrgID = organizationID
	f.updateEnabledTaskID = scheduledTaskID
	f.updateEnabledInput = input
	return f.updatedEnabledTask, nil
}

func (f *fakeStore) ClaimDueScheduledTaskRuns(ctx context.Context, input ClaimDueScheduledTaskRunsInput) ([]ClaimedScheduledTaskRun, error) {
	f.claimDueCalls++
	f.claimedInput = input
	return f.claimedRuns, nil
}

func (f *fakeStore) RecordScheduledTaskRun(ctx context.Context, input RecordScheduledTaskRunInput) (ScheduledTaskRun, error) {
	f.recordRunCalls++
	f.recordedRunInput = input
	return f.recordedRun, nil
}

func (f *fakeStore) CompleteScheduledTaskRun(ctx context.Context, organizationID string, scheduledTaskID string, scheduledTaskRunID string, input CompleteScheduledTaskRunInput) (ScheduledTaskRun, error) {
	f.completeRunCalls++
	f.completedTaskID = scheduledTaskID
	f.completedRunID = scheduledTaskRunID
	f.completedRunInput = input
	run := f.recordedRun
	if run.ID == "" {
		run = ScheduledTaskRun{ID: scheduledTaskRunID, OrganizationID: organizationID, ScheduledTaskID: scheduledTaskID}
	}
	run.Status = RunStatusCompleted
	run.FinishedAt = &input.FinishedAt
	return run, nil
}

func (f *fakeStore) CompleteManualScheduledTaskRun(ctx context.Context, organizationID string, scheduledTaskID string, scheduledTaskRunID string, finishedAt time.Time) (ScheduledTaskRun, error) {
	f.completeManualCalls++
	f.completedManualOrgID = organizationID
	f.completedManualTaskID = scheduledTaskID
	f.completedManualRunID = scheduledTaskRunID
	f.completedManualAt = finishedAt
	run := f.completedManualRun
	if run.ID == "" {
		run = ScheduledTaskRun{ID: scheduledTaskRunID, OrganizationID: organizationID, ScheduledTaskID: scheduledTaskID}
	}
	run.Status = RunStatusCompleted
	run.FinishedAt = &finishedAt
	return run, nil
}

func (f *fakeStore) UpdateScheduledTaskRun(ctx context.Context, organizationID string, scheduledTaskRunID string, input UpdateScheduledTaskRunInput) (ScheduledTaskRun, error) {
	f.updateRunCalls++
	f.updatedRunID = scheduledTaskRunID
	f.updatedRunInput = input
	run := f.recordedRun
	if run.ID == "" {
		run = ScheduledTaskRun{ID: scheduledTaskRunID, OrganizationID: organizationID}
	}
	run.Status = input.Status
	run.FinishedAt = input.FinishedAt
	run.Error = input.Error
	return run, nil
}

func (f *fakeStore) ListScheduledTaskRuns(ctx context.Context, organizationID string, scheduledTaskID string) ([]ScheduledTaskRun, error) {
	f.listScheduledRunCalls++
	f.listedRunsOrgID = organizationID
	f.listedRunsTaskID = scheduledTaskID
	return f.listedRuns, nil
}

func (f *fakeStore) CountRunningScheduledTaskRuns(ctx context.Context, organizationID string, scheduledTaskID string) (int, error) {
	return f.runningRunCount, nil
}

func TestCreateScheduledTaskUsesOrganizationScopeAndNormalizesInput(t *testing.T) {
	expectedNextRun := time.Date(2026, time.June, 5, 9, 0, 0, 0, time.UTC)
	store := &fakeStore{
		createdTask: ScheduledTask{
			ID:             "sched_1",
			OrganizationID: "org_1",
			TargetType:     TargetTypeWorkflow,
			TargetID:       "workflow_1",
			CronExpression: "*/15 * * * *",
			Enabled:        true,
			NextRunAt:      &expectedNextRun,
		},
	}
	service := NewService(store)

	task, err := service.Create(context.Background(), auth.Session{OrganizationID: "org_1"}, CreateScheduledTaskInput{
		Name:           "  Daily digest  ",
		TargetType:     " workflow ",
		TargetID:       " workflow_1 ",
		CronExpression: " */15 * * * * ",
		Enabled:        true,
		Now:            time.Date(2026, time.June, 5, 8, 46, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}

	if store.createCalls != 1 {
		t.Fatalf("expected store create once, got %d", store.createCalls)
	}
	if store.createdInput.OrganizationID != "org_1" {
		t.Fatalf("expected organization scope org_1, got %q", store.createdInput.OrganizationID)
	}
	if store.createdInput.Name != "Daily digest" {
		t.Fatalf("expected trimmed task name, got %q", store.createdInput.Name)
	}
	if store.createdInput.TargetType != TargetTypeWorkflow || store.createdInput.TargetID != "workflow_1" {
		t.Fatalf("expected normalized workflow target, got %+v", store.createdInput)
	}
	if store.createdInput.CronExpression != "*/15 * * * *" {
		t.Fatalf("expected trimmed cron expression, got %q", store.createdInput.CronExpression)
	}
	if store.createdInput.NextRunAt == nil || !store.createdInput.NextRunAt.Equal(time.Date(2026, time.June, 5, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected next run at 2026-06-05T09:00:00Z, got %v", store.createdInput.NextRunAt)
	}
	if !task.Enabled || task.ID != "sched_1" {
		t.Fatalf("unexpected task: %+v", task)
	}
}

func TestCreateScheduledTaskRejectsBlankName(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)

	_, err := service.Create(context.Background(), auth.Session{OrganizationID: "org_1"}, CreateScheduledTaskInput{
		Name:           "   ",
		TargetType:     TargetTypeAgent,
		TargetID:       "agent_1",
		CronExpression: "0 9 * * *",
		Enabled:        true,
	})
	if !errors.Is(err, ErrInvalidScheduledTaskName) {
		t.Fatalf("expected ErrInvalidScheduledTaskName, got %v", err)
	}
	if store.createCalls != 0 {
		t.Fatalf("expected blank name not to hit store, got %d calls", store.createCalls)
	}
}

func TestSyncWorkflowScheduleTriggersNormalizesAndDelegatesTriggerBackedTasks(t *testing.T) {
	now := time.Date(2026, time.June, 5, 8, 0, 0, 0, time.UTC)
	store := &fakeStore{
		syncedWorkflowTasks: []ScheduledTask{
			{
				ID:                "sched_1",
				OrganizationID:    "org_1",
				TargetType:        TargetTypeWorkflow,
				TargetID:          "workflow_1",
				WorkflowTriggerID: "daily-report",
				CronExpression:    "0 9 * * 1",
				Enabled:           true,
			},
		},
	}
	service := NewService(store)

	err := service.SyncWorkflowScheduleTriggers(context.Background(), workflow.WorkflowScheduleSyncRequest{
		OrganizationID: " org_1 ",
		WorkflowID:     " workflow_1 ",
		Triggers: []workflow.WorkflowScheduleTrigger{
			{
				ID:             " daily-report ",
				Name:           " Daily report ",
				CronExpression: " 0 9 * * 1 ",
				Enabled:        true,
			},
			{
				ID:             " disabled-maintenance ",
				CronExpression: " 30 2 * * * ",
				Enabled:        false,
			},
		},
		Now: now,
	})
	if err != nil {
		t.Fatalf("SyncWorkflowScheduleTriggers returned error: %v", err)
	}

	if store.syncWorkflowCalls != 1 {
		t.Fatalf("expected store sync once, got %d", store.syncWorkflowCalls)
	}
	if store.syncedWorkflowInput.OrganizationID != "org_1" || store.syncedWorkflowInput.WorkflowID != "workflow_1" {
		t.Fatalf("unexpected workflow sync scope: %+v", store.syncedWorkflowInput)
	}
	if len(store.syncedWorkflowInput.Triggers) != 2 {
		t.Fatalf("expected two normalized triggers, got %+v", store.syncedWorkflowInput.Triggers)
	}
	first := store.syncedWorkflowInput.Triggers[0]
	if first.TriggerID != "daily-report" || first.Name != "Daily report" || first.CronExpression != "0 9 * * 1" || !first.Enabled {
		t.Fatalf("unexpected normalized enabled trigger: %+v", first)
	}
	expectedNextRun := time.Date(2026, time.June, 8, 9, 0, 0, 0, time.UTC)
	if first.NextRunAt == nil || !first.NextRunAt.Equal(expectedNextRun) {
		t.Fatalf("expected next run %v, got %v", expectedNextRun, first.NextRunAt)
	}
	second := store.syncedWorkflowInput.Triggers[1]
	if second.TriggerID != "disabled-maintenance" || second.Name != "disabled-maintenance" || second.CronExpression != "30 2 * * *" || second.Enabled || second.NextRunAt != nil {
		t.Fatalf("unexpected normalized disabled trigger: %+v", second)
	}
}

func TestSyncWorkflowScheduleTriggersRejectsInvalidInputsBeforeStore(t *testing.T) {
	tests := []struct {
		name string
		req  workflow.WorkflowScheduleSyncRequest
		err  error
	}{
		{
			name: "missing trigger id",
			req: workflow.WorkflowScheduleSyncRequest{
				OrganizationID: "org_1",
				WorkflowID:     "workflow_1",
				Triggers: []workflow.WorkflowScheduleTrigger{
					{CronExpression: "0 * * * *", Enabled: true},
				},
			},
			err: ErrInvalidScheduledTaskID,
		},
		{
			name: "malformed cron",
			req: workflow.WorkflowScheduleSyncRequest{
				OrganizationID: "org_1",
				WorkflowID:     "workflow_1",
				Triggers: []workflow.WorkflowScheduleTrigger{
					{ID: "bad-cron", CronExpression: "not a cron", Enabled: true},
				},
			},
			err: ErrInvalidCronExpression,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &fakeStore{}
			service := NewService(store)
			if err := service.SyncWorkflowScheduleTriggers(context.Background(), tt.req); !errors.Is(err, tt.err) {
				t.Fatalf("SyncWorkflowScheduleTriggers err=%v, want %v", err, tt.err)
			}
			if store.syncWorkflowCalls != 0 {
				t.Fatalf("expected invalid sync not to call store, got %d calls", store.syncWorkflowCalls)
			}
		})
	}
}

func TestCreateScheduledTaskLeavesDisabledTaskWithoutNextRun(t *testing.T) {
	store := &fakeStore{
		createdTask: ScheduledTask{
			ID:             "sched_2",
			OrganizationID: "org_1",
			TargetType:     TargetTypeAgent,
			TargetID:       "agent_1",
			CronExpression: "0 9 * * *",
			Enabled:        false,
		},
	}
	service := NewService(store)

	_, err := service.Create(context.Background(), auth.Session{OrganizationID: "org_1"}, CreateScheduledTaskInput{
		Name:           "Agent morning run",
		TargetType:     TargetTypeAgent,
		TargetID:       "agent_1",
		CronExpression: "0 9 * * *",
		Enabled:        false,
		Now:            time.Date(2026, time.June, 5, 8, 46, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if store.createdInput.NextRunAt != nil {
		t.Fatalf("expected disabled task not to compute next run, got %v", store.createdInput.NextRunAt)
	}
}

func TestCreateScheduledTaskRejectsEmptyCronExpression(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)

	_, err := service.Create(context.Background(), auth.Session{OrganizationID: "org_1"}, CreateScheduledTaskInput{
		Name:           "Agent morning run",
		TargetType:     TargetTypeAgent,
		TargetID:       "agent_1",
		CronExpression: "   ",
		Enabled:        true,
	})
	if !errors.Is(err, ErrInvalidCronExpression) {
		t.Fatalf("expected ErrInvalidCronExpression, got %v", err)
	}
	if store.createCalls != 0 {
		t.Fatalf("expected invalid cron expression not to hit store, got %d calls", store.createCalls)
	}
}

func TestCreateScheduledTaskRejectsMalformedCronExpression(t *testing.T) {
	tests := []struct {
		name       string
		expression string
	}{
		{name: "not a cron expression", expression: "every 5 minutes"},
		{name: "too many fields", expression: "0 9 * * * *"},
		{name: "minute out of range", expression: "61 * * * *"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeStore{}
			service := NewService(store)

			_, err := service.Create(context.Background(), auth.Session{OrganizationID: "org_1"}, CreateScheduledTaskInput{
				Name:           "Agent morning run",
				TargetType:     TargetTypeAgent,
				TargetID:       "agent_1",
				CronExpression: test.expression,
				Enabled:        true,
			})
			if !errors.Is(err, ErrInvalidCronExpression) {
				t.Fatalf("expected ErrInvalidCronExpression, got %v", err)
			}
			if store.createCalls != 0 {
				t.Fatalf("expected malformed cron expression not to hit store, got %d calls", store.createCalls)
			}
		})
	}
}

func TestCreateScheduledTaskRejectsUnsupportedTargetType(t *testing.T) {
	store := &fakeStore{}
	service := NewService(store)

	_, err := service.Create(context.Background(), auth.Session{OrganizationID: "org_1"}, CreateScheduledTaskInput{
		Name:           "Task run",
		TargetType:     "task",
		TargetID:       "task_1",
		CronExpression: "0 * * * *",
		Enabled:        true,
	})
	if !errors.Is(err, ErrInvalidTargetType) {
		t.Fatalf("expected ErrInvalidTargetType, got %v", err)
	}
	if store.createCalls != 0 {
		t.Fatalf("expected invalid target type not to hit store, got %d calls", store.createCalls)
	}
}

func TestListScheduledTasksUsesOrganizationScope(t *testing.T) {
	store := &fakeStore{
		listedTasks: []ScheduledTask{
			{ID: "sched_1", OrganizationID: "org_1", TargetType: TargetTypeWorkflow, TargetID: "workflow_1", CronExpression: "0 * * * *", Enabled: true},
		},
	}
	service := NewService(store)

	tasks, err := service.List(context.Background(), auth.Session{OrganizationID: "org_1"})
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}

	if store.listedOrgID != "org_1" {
		t.Fatalf("expected organization scope org_1, got %q", store.listedOrgID)
	}
	if len(tasks) != 1 || tasks[0].ID != "sched_1" {
		t.Fatalf("unexpected tasks: %+v", tasks)
	}
}

func TestUpdateScheduledTaskEnabledComputesNextRunWhenEnabling(t *testing.T) {
	nextRunAt := time.Date(2026, time.June, 5, 10, 0, 0, 0, time.UTC)
	store := &fakeStore{
		gotTask: ScheduledTask{
			ID:             "sched_1",
			OrganizationID: "org_1",
			TargetType:     TargetTypeWorkflow,
			TargetID:       "workflow_1",
			CronExpression: "0 * * * *",
			Enabled:        false,
		},
		updatedEnabledTask: ScheduledTask{
			ID:             "sched_1",
			OrganizationID: "org_1",
			TargetType:     TargetTypeWorkflow,
			TargetID:       "workflow_1",
			CronExpression: "0 * * * *",
			Enabled:        true,
			NextRunAt:      &nextRunAt,
		},
	}
	service := NewService(store)

	task, err := service.UpdateEnabled(context.Background(), auth.Session{OrganizationID: "org_1"}, " sched_1 ", true, time.Date(2026, time.June, 5, 9, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("UpdateEnabled returned error: %v", err)
	}

	if store.getTaskCalls != 1 || store.gotTaskOrgID != "org_1" || store.gotTaskID != "sched_1" {
		t.Fatalf("expected scoped task lookup, calls=%d org=%q task=%q", store.getTaskCalls, store.gotTaskOrgID, store.gotTaskID)
	}
	if store.updateEnabledCalls != 1 || store.updateEnabledOrgID != "org_1" || store.updateEnabledTaskID != "sched_1" {
		t.Fatalf("expected scoped enabled update, calls=%d org=%q task=%q", store.updateEnabledCalls, store.updateEnabledOrgID, store.updateEnabledTaskID)
	}
	if !store.updateEnabledInput.Enabled {
		t.Fatalf("expected enabled update input, got %+v", store.updateEnabledInput)
	}
	if store.updateEnabledInput.NextRunAt == nil || !store.updateEnabledInput.NextRunAt.Equal(nextRunAt) {
		t.Fatalf("expected next run at %v, got %v", nextRunAt, store.updateEnabledInput.NextRunAt)
	}
	if !task.Enabled || task.NextRunAt == nil || !task.NextRunAt.Equal(nextRunAt) {
		t.Fatalf("unexpected updated task: %+v", task)
	}
}

func TestUpdateScheduledTaskEnabledClearsNextRunWhenDisabling(t *testing.T) {
	nextRunAt := time.Date(2026, time.June, 5, 10, 0, 0, 0, time.UTC)
	store := &fakeStore{
		gotTask: ScheduledTask{
			ID:             "sched_1",
			OrganizationID: "org_1",
			TargetType:     TargetTypeWorkflow,
			TargetID:       "workflow_1",
			CronExpression: "0 * * * *",
			Enabled:        true,
			NextRunAt:      &nextRunAt,
		},
		updatedEnabledTask: ScheduledTask{
			ID:             "sched_1",
			OrganizationID: "org_1",
			TargetType:     TargetTypeWorkflow,
			TargetID:       "workflow_1",
			CronExpression: "0 * * * *",
			Enabled:        false,
		},
	}
	service := NewService(store)

	task, err := service.UpdateEnabled(context.Background(), auth.Session{OrganizationID: "org_1"}, " sched_1 ", false, time.Date(2026, time.June, 5, 9, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("UpdateEnabled returned error: %v", err)
	}

	if store.updateEnabledCalls != 1 {
		t.Fatalf("expected one enabled update, got %d", store.updateEnabledCalls)
	}
	if store.updateEnabledInput.Enabled {
		t.Fatalf("expected disabled update input, got %+v", store.updateEnabledInput)
	}
	if store.updateEnabledInput.NextRunAt != nil {
		t.Fatalf("expected disabling to clear next run, got %v", store.updateEnabledInput.NextRunAt)
	}
	if task.Enabled || task.NextRunAt != nil {
		t.Fatalf("unexpected disabled task: %+v", task)
	}
}

func TestRecordScheduledTaskRunUsesOrganizationScopeAndNormalizesInput(t *testing.T) {
	startedAt := time.Date(2026, time.June, 5, 9, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(2 * time.Minute)
	store := &fakeStore{
		recordedRun: ScheduledTaskRun{
			ID:              "schedrun_1",
			OrganizationID:  "org_1",
			ScheduledTaskID: "sched_1",
			Status:          RunStatusFailed,
			StartedAt:       &startedAt,
			FinishedAt:      &finishedAt,
			Error:           "workflow failed",
		},
	}
	service := NewService(store)

	run, err := service.RecordRun(context.Background(), auth.Session{OrganizationID: "org_1"}, RecordScheduledTaskRunInput{
		ScheduledTaskID: " sched_1 ",
		Status:          " FAILED ",
		StartedAt:       &startedAt,
		FinishedAt:      &finishedAt,
		Error:           " workflow failed ",
	})
	if err != nil {
		t.Fatalf("RecordRun returned error: %v", err)
	}

	if store.recordRunCalls != 1 {
		t.Fatalf("expected store record run once, got %d", store.recordRunCalls)
	}
	if store.recordedRunInput.OrganizationID != "org_1" {
		t.Fatalf("expected organization scope org_1, got %q", store.recordedRunInput.OrganizationID)
	}
	if store.recordedRunInput.ScheduledTaskID != "sched_1" {
		t.Fatalf("expected trimmed scheduled task id, got %q", store.recordedRunInput.ScheduledTaskID)
	}
	if store.recordedRunInput.Status != RunStatusFailed {
		t.Fatalf("expected normalized failed status, got %q", store.recordedRunInput.Status)
	}
	if store.recordedRunInput.Error != "workflow failed" {
		t.Fatalf("expected trimmed run error, got %q", store.recordedRunInput.Error)
	}
	if store.recordedRunInput.StartedAt == nil || !store.recordedRunInput.StartedAt.Equal(startedAt) {
		t.Fatalf("expected started at to pass through, got %v", store.recordedRunInput.StartedAt)
	}
	if store.recordedRunInput.FinishedAt == nil || !store.recordedRunInput.FinishedAt.Equal(finishedAt) {
		t.Fatalf("expected finished at to pass through, got %v", store.recordedRunInput.FinishedAt)
	}
	if run.ID != "schedrun_1" || run.Status != RunStatusFailed || run.Error != "workflow failed" {
		t.Fatalf("unexpected recorded run: %+v", run)
	}
}

func TestRecordScheduledTaskRunQueuesWhenScheduleAlreadyHasRunningRun(t *testing.T) {
	startedAt := time.Date(2026, time.June, 5, 9, 0, 0, 0, time.UTC)
	store := &fakeStore{
		runningRunCount: 1,
		recordedRun: ScheduledTaskRun{
			ID:              "schedrun_queued",
			OrganizationID:  "org_1",
			ScheduledTaskID: "sched_1",
			Status:          RunStatusQueued,
		},
	}
	service := NewService(store)

	run, err := service.RecordRun(context.Background(), auth.Session{OrganizationID: "org_1"}, RecordScheduledTaskRunInput{
		ScheduledTaskID: "sched_1",
		Status:          RunStatusRunning,
		StartedAt:       &startedAt,
	})
	if err != nil {
		t.Fatalf("RecordRun returned error: %v", err)
	}

	if store.recordRunCalls != 1 {
		t.Fatalf("expected queued run to be recorded once, got %d calls", store.recordRunCalls)
	}
	if store.recordedRunInput.Status != RunStatusQueued {
		t.Fatalf("expected overlapping scheduled run to be queued, got %q", store.recordedRunInput.Status)
	}
	if store.recordedRunInput.StartedAt != nil {
		t.Fatalf("expected queued run not to keep startedAt, got %v", store.recordedRunInput.StartedAt)
	}
	if run.Status != RunStatusQueued {
		t.Fatalf("expected returned run status queued, got %+v", run)
	}
}

func TestTriggerWorkflowScheduleRecordsRunAndStartsWorkflowWithTriggerPayload(t *testing.T) {
	startedAt := time.Date(2026, time.June, 5, 9, 0, 0, 0, time.UTC)
	store := &fakeStore{
		recordedRun: ScheduledTaskRun{
			ID:              "schedrun_1",
			OrganizationID:  "org_1",
			ScheduledTaskID: "sched_1",
			Status:          RunStatusRunning,
			StartedAt:       &startedAt,
		},
	}
	starter := &fakeWorkflowStarter{
		result: &workflow.WorkflowExecution{ID: "wexec_1", OrganizationID: "org_1", WorkflowID: "workflow_1", Status: workflow.ExecutionStatusRunning},
	}
	service := NewService(store, WithWorkflowStarter(starter))

	execution, err := service.TriggerWorkflow(context.Background(), auth.Session{OrganizationID: "org_1"}, ScheduledTask{
		ID:             " sched_1 ",
		OrganizationID: "org_1",
		TargetType:     TargetTypeWorkflow,
		TargetID:       " workflow_1 ",
		CronExpression: "0 9 * * *",
	}, map[string]any{"window": "daily", "attempt": 1})
	if err != nil {
		t.Fatalf("TriggerWorkflow returned error: %v", err)
	}

	if store.recordRunCalls != 1 {
		t.Fatalf("expected scheduled run to be recorded once, got %d calls", store.recordRunCalls)
	}
	if store.recordedRunInput.OrganizationID != "org_1" || store.recordedRunInput.ScheduledTaskID != "sched_1" || store.recordedRunInput.Status != RunStatusRunning {
		t.Fatalf("unexpected scheduled run input: %+v", store.recordedRunInput)
	}
	if store.recordedRunInput.StartedAt == nil {
		t.Fatalf("expected running scheduled run to include startedAt")
	}
	if store.updateRunCalls != 1 || store.updatedRunID != "schedrun_1" || store.updatedRunInput.Status != RunStatusCompleted || store.updatedRunInput.FinishedAt == nil {
		t.Fatalf("expected scheduled run to be completed after workflow start, calls=%d id=%q input=%+v", store.updateRunCalls, store.updatedRunID, store.updatedRunInput)
	}
	if starter.calls != 1 {
		t.Fatalf("expected workflow starter once, got %d", starter.calls)
	}
	if starter.request.OrganizationID != "org_1" || starter.request.WorkflowID != "workflow_1" {
		t.Fatalf("unexpected workflow start identity: %+v", starter.request)
	}
	if starter.request.TriggerType != workflow.WorkflowTriggerSchedule {
		t.Fatalf("expected schedule trigger, got %q", starter.request.TriggerType)
	}
	if starter.request.Input["window"] != "daily" || starter.request.Input["attempt"].(int) != 1 {
		t.Fatalf("expected schedule payload as workflow input, got %+v", starter.request.Input)
	}
	if starter.request.TriggerPayload["scheduledTaskId"] != "sched_1" || starter.request.TriggerPayload["scheduledTaskRunId"] != "schedrun_1" {
		t.Fatalf("expected schedule trigger ids, got %+v", starter.request.TriggerPayload)
	}
	payload := starter.request.TriggerPayload["payload"].(map[string]any)
	if payload["window"] != "daily" || payload["attempt"].(int) != 1 {
		t.Fatalf("expected raw schedule payload in trigger context, got %+v", payload)
	}
	if execution.ID != "wexec_1" {
		t.Fatalf("unexpected workflow execution: %+v", execution)
	}
}

func TestTriggerWorkflowScheduleRejectsCrossOrganizationTask(t *testing.T) {
	store := &fakeStore{}
	starter := &fakeWorkflowStarter{}
	service := NewService(store, WithWorkflowStarter(starter))

	_, err := service.TriggerWorkflow(context.Background(), auth.Session{OrganizationID: "org_1"}, ScheduledTask{
		ID:             "sched_1",
		OrganizationID: "org_2",
		TargetType:     TargetTypeWorkflow,
		TargetID:       "workflow_1",
		CronExpression: "0 9 * * *",
	}, nil)
	if !errors.Is(err, ErrInvalidOrganization) {
		t.Fatalf("expected ErrInvalidOrganization, got %v", err)
	}
	if store.recordRunCalls != 0 || starter.calls != 0 {
		t.Fatalf("expected no side effects for cross-org task, record calls=%d starter calls=%d", store.recordRunCalls, starter.calls)
	}
}

func TestTriggerWorkflowScheduleMarksRunFailedWhenWorkflowStartFails(t *testing.T) {
	startedAt := time.Date(2026, time.June, 5, 9, 0, 0, 0, time.UTC)
	store := &fakeStore{
		recordedRun: ScheduledTaskRun{
			ID:              "schedrun_1",
			OrganizationID:  "org_1",
			ScheduledTaskID: "sched_1",
			Status:          RunStatusRunning,
			StartedAt:       &startedAt,
		},
	}
	starter := &fakeWorkflowStarter{err: errors.New("workflow unavailable")}
	service := NewService(store, WithWorkflowStarter(starter))

	_, err := service.TriggerWorkflow(context.Background(), auth.Session{OrganizationID: "org_1"}, ScheduledTask{
		ID:             "sched_1",
		OrganizationID: "org_1",
		TargetType:     TargetTypeWorkflow,
		TargetID:       "workflow_1",
		CronExpression: "0 9 * * *",
	}, map[string]any{"window": "daily"})
	if err == nil || err.Error() != "workflow unavailable" {
		t.Fatalf("expected workflow start error, got %v", err)
	}
	if store.updateRunCalls != 1 || store.updatedRunID != "schedrun_1" || store.updatedRunInput.Status != RunStatusFailed || store.updatedRunInput.FinishedAt == nil || store.updatedRunInput.Error != "workflow unavailable" {
		t.Fatalf("expected failed scheduled run after workflow start error, calls=%d id=%q input=%+v", store.updateRunCalls, store.updatedRunID, store.updatedRunInput)
	}
}

func TestRunScheduledTaskNowStartsWorkflowAndCompletesManualRun(t *testing.T) {
	startedAt := time.Date(2026, time.June, 5, 9, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(time.Minute)
	store := &fakeStore{
		gotTask: ScheduledTask{
			ID:             "sched_1",
			OrganizationID: "org_1",
			TargetType:     TargetTypeWorkflow,
			TargetID:       "workflow_1",
			CronExpression: "0 * * * *",
			Enabled:        true,
		},
		recordedRun: ScheduledTaskRun{
			ID:              "schedrun_1",
			OrganizationID:  "org_1",
			ScheduledTaskID: "sched_1",
			Status:          RunStatusRunning,
			StartedAt:       &startedAt,
		},
		completedManualRun: ScheduledTaskRun{
			ID:              "schedrun_1",
			OrganizationID:  "org_1",
			ScheduledTaskID: "sched_1",
			Status:          RunStatusCompleted,
			StartedAt:       &startedAt,
			FinishedAt:      &finishedAt,
		},
	}
	starter := &fakeWorkflowStarter{
		result: &workflow.WorkflowExecution{ID: "wexec_1", OrganizationID: "org_1", WorkflowID: "workflow_1", Status: workflow.ExecutionStatusRunning},
	}
	service := NewService(store, WithWorkflowStarter(starter))

	result, err := service.RunNow(context.Background(), auth.Session{OrganizationID: "org_1"}, " sched_1 ")
	if err != nil {
		t.Fatalf("RunNow returned error: %v", err)
	}

	if store.getTaskCalls != 1 || store.gotTaskID != "sched_1" {
		t.Fatalf("expected task lookup for sched_1, calls=%d id=%q", store.getTaskCalls, store.gotTaskID)
	}
	if store.recordRunCalls != 1 || store.recordedRunInput.ScheduledTaskID != "sched_1" || store.recordedRunInput.Status != RunStatusRunning || store.recordedRunInput.StartedAt == nil {
		t.Fatalf("expected manual run to record running run, calls=%d input=%+v", store.recordRunCalls, store.recordedRunInput)
	}
	if starter.calls != 1 || starter.request.WorkflowID != "workflow_1" || starter.request.TriggerType != workflow.WorkflowTriggerSchedule {
		t.Fatalf("unexpected workflow starter call count=%d request=%+v", starter.calls, starter.request)
	}
	if starter.request.TriggerPayload["manual"] != true || starter.request.TriggerPayload["scheduledTaskRunId"] != "schedrun_1" {
		t.Fatalf("expected manual schedule trigger payload, got %+v", starter.request.TriggerPayload)
	}
	if store.completeManualCalls != 1 || store.completedManualTaskID != "sched_1" || store.completedManualRunID != "schedrun_1" || store.completedManualAt.IsZero() {
		t.Fatalf("expected manual run completion, calls=%d task=%q run=%q at=%v", store.completeManualCalls, store.completedManualTaskID, store.completedManualRunID, store.completedManualAt)
	}
	if result.Run.Status != RunStatusCompleted || result.Execution == nil || result.Execution.ID != "wexec_1" {
		t.Fatalf("unexpected run-now result: %+v", result)
	}
}

func TestRunScheduledTaskNowQueuesWhenScheduleAlreadyHasRunningRun(t *testing.T) {
	store := &fakeStore{
		gotTask: ScheduledTask{
			ID:             "sched_1",
			OrganizationID: "org_1",
			TargetType:     TargetTypeWorkflow,
			TargetID:       "workflow_1",
			CronExpression: "0 * * * *",
			Enabled:        true,
		},
		runningRunCount: 1,
		recordedRun: ScheduledTaskRun{
			ID:              "schedrun_queued",
			OrganizationID:  "org_1",
			ScheduledTaskID: "sched_1",
			Status:          RunStatusQueued,
		},
	}
	starter := &fakeWorkflowStarter{}
	service := NewService(store, WithWorkflowStarter(starter))

	result, err := service.RunNow(context.Background(), auth.Session{OrganizationID: "org_1"}, "sched_1")
	if err != nil {
		t.Fatalf("RunNow returned error: %v", err)
	}

	if store.recordRunCalls != 1 || store.recordedRunInput.Status != RunStatusQueued || store.recordedRunInput.StartedAt != nil {
		t.Fatalf("expected overlapping manual run to be queued, calls=%d input=%+v", store.recordRunCalls, store.recordedRunInput)
	}
	if starter.calls != 0 || store.completeManualCalls != 0 {
		t.Fatalf("expected queued manual run not to start target, starter calls=%d complete calls=%d", starter.calls, store.completeManualCalls)
	}
	if result.Run.Status != RunStatusQueued {
		t.Fatalf("expected queued run result, got %+v", result)
	}
}

func TestRecordScheduledTaskRunRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name  string
		input RecordScheduledTaskRunInput
		err   error
	}{
		{name: "missing organization", input: RecordScheduledTaskRunInput{ScheduledTaskID: "sched_1", Status: RunStatusQueued}, err: ErrInvalidOrganization},
		{name: "missing scheduled task id", input: RecordScheduledTaskRunInput{Status: RunStatusQueued}, err: ErrInvalidScheduledTaskID},
		{name: "unsupported status", input: RecordScheduledTaskRunInput{ScheduledTaskID: "sched_1", Status: "blocked"}, err: ErrInvalidRunStatus},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := &fakeStore{}
			service := NewService(store)

			session := auth.Session{OrganizationID: "org_1"}
			if test.err == ErrInvalidOrganization {
				session.OrganizationID = " "
			}
			_, err := service.RecordRun(context.Background(), session, test.input)
			if !errors.Is(err, test.err) {
				t.Fatalf("expected %v, got %v", test.err, err)
			}
			if store.recordRunCalls != 0 {
				t.Fatalf("expected invalid input not to hit store, got %d calls", store.recordRunCalls)
			}
		})
	}
}

func TestListScheduledTaskRunsUsesOrganizationScope(t *testing.T) {
	store := &fakeStore{
		listedRuns: []ScheduledTaskRun{
			{ID: "schedrun_1", OrganizationID: "org_1", ScheduledTaskID: "sched_1", Status: RunStatusQueued},
		},
	}
	service := NewService(store)

	runs, err := service.ListRuns(context.Background(), auth.Session{OrganizationID: "org_1"}, " sched_1 ")
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}

	if store.listedRunsOrgID != "org_1" {
		t.Fatalf("expected organization scope org_1, got %q", store.listedRunsOrgID)
	}
	if store.listedRunsTaskID != "sched_1" {
		t.Fatalf("expected trimmed scheduled task id, got %q", store.listedRunsTaskID)
	}
	if len(runs) != 1 || runs[0].ID != "schedrun_1" {
		t.Fatalf("unexpected scheduled task runs: %+v", runs)
	}
}
