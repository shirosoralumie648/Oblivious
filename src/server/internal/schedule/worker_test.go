package schedule

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/agent"
	"oblivious/server/internal/workflow"
)

func TestRunDueTasksClaimsDueWorkflowRunAndAdvancesScheduleOnSuccess(t *testing.T) {
	now := time.Date(2026, time.June, 5, 9, 0, 0, 0, time.UTC)
	store := &fakeStore{
		claimedRuns: []ClaimedScheduledTaskRun{
			{
				Task: ScheduledTask{
					ID:             "sched_1",
					OrganizationID: "org_1",
					TargetType:     TargetTypeWorkflow,
					TargetID:       "workflow_1",
					CronExpression: "0 * * * *",
					Enabled:        true,
					NextRunAt:      &now,
				},
				Run: ScheduledTaskRun{
					ID:              "schedrun_1",
					OrganizationID:  "org_1",
					ScheduledTaskID: "sched_1",
					Status:          RunStatusRunning,
					StartedAt:       &now,
				},
			},
		},
	}
	starter := &fakeWorkflowStarter{
		result:    &workflow.WorkflowExecution{ID: "wexec_1", OrganizationID: "org_1", WorkflowID: "workflow_1", Status: workflow.ExecutionStatusRunning},
		runResult: &workflow.WorkflowExecution{ID: "wexec_1", OrganizationID: "org_1", WorkflowID: "workflow_1", Status: workflow.ExecutionStatusSucceeded},
	}
	service := NewService(store, WithWorkflowStarter(starter))

	results, err := service.RunDueTasks(context.Background(), now, 5)
	if err != nil {
		t.Fatalf("RunDueTasks returned error: %v", err)
	}

	if store.claimDueCalls != 1 || !store.claimedInput.Now.Equal(now) || store.claimedInput.Limit != 5 {
		t.Fatalf("expected due task claim with now/limit, calls=%d input=%+v", store.claimDueCalls, store.claimedInput)
	}
	if store.recordRunCalls != 0 {
		t.Fatalf("expected worker to use claimed run instead of recording another run, got %d record calls", store.recordRunCalls)
	}
	if starter.calls != 1 || starter.request.OrganizationID != "org_1" || starter.request.WorkflowID != "workflow_1" {
		t.Fatalf("unexpected workflow starter call count=%d request=%+v", starter.calls, starter.request)
	}
	if starter.request.TriggerType != workflow.WorkflowTriggerSchedule {
		t.Fatalf("expected schedule trigger, got %q", starter.request.TriggerType)
	}
	if starter.request.TriggerPayload["scheduledTaskId"] != "sched_1" || starter.request.TriggerPayload["scheduledTaskRunId"] != "schedrun_1" {
		t.Fatalf("expected claimed schedule ids in trigger payload, got %+v", starter.request.TriggerPayload)
	}
	if store.completeRunCalls != 1 || store.completedTaskID != "sched_1" || store.completedRunID != "schedrun_1" {
		t.Fatalf("expected claimed run completion, calls=%d task=%q run=%q", store.completeRunCalls, store.completedTaskID, store.completedRunID)
	}
	expectedNextRun := time.Date(2026, time.June, 5, 10, 0, 0, 0, time.UTC)
	if !store.completedRunInput.NextRunAt.Equal(expectedNextRun) {
		t.Fatalf("expected next run %v, got %v", expectedNextRun, store.completedRunInput.NextRunAt)
	}
	if len(results) != 1 || results[0].Run.ID != "schedrun_1" || results[0].Run.Status != RunStatusCompleted || results[0].Execution.ID != "wexec_1" || results[0].Err != nil {
		t.Fatalf("unexpected due task result: %+v", results)
	}
}

func TestRunDueTasksAdvancesScheduleOnPartialSuccessWorkflow(t *testing.T) {
	now := time.Date(2026, time.June, 5, 9, 0, 0, 0, time.UTC)
	store := &fakeStore{
		claimedRuns: []ClaimedScheduledTaskRun{
			{
				Task: ScheduledTask{
					ID:             "sched_1",
					OrganizationID: "org_1",
					TargetType:     TargetTypeWorkflow,
					TargetID:       "workflow_1",
					CronExpression: "0 * * * *",
					Enabled:        true,
					NextRunAt:      &now,
				},
				Run: ScheduledTaskRun{
					ID:              "schedrun_1",
					OrganizationID:  "org_1",
					ScheduledTaskID: "sched_1",
					Status:          RunStatusRunning,
					StartedAt:       &now,
				},
			},
		},
	}
	starter := &fakeWorkflowStarter{
		result:    &workflow.WorkflowExecution{ID: "wexec_1", OrganizationID: "org_1", WorkflowID: "workflow_1", Status: workflow.ExecutionStatusRunning},
		runResult: &workflow.WorkflowExecution{ID: "wexec_1", OrganizationID: "org_1", WorkflowID: "workflow_1", Status: workflow.ExecutionStatusPartialSuccess},
	}
	service := NewService(store, WithWorkflowStarter(starter))

	results, err := service.RunDueTasks(context.Background(), now, 5)
	if err != nil {
		t.Fatalf("RunDueTasks returned error: %v", err)
	}

	if store.completeRunCalls != 1 || store.completedTaskID != "sched_1" || store.completedRunID != "schedrun_1" {
		t.Fatalf("expected partial success workflow to complete claimed run, calls=%d task=%q run=%q", store.completeRunCalls, store.completedTaskID, store.completedRunID)
	}
	if store.updateRunCalls != 0 {
		t.Fatalf("expected partial success workflow not to fail claimed run, got update calls=%d input=%+v", store.updateRunCalls, store.updatedRunInput)
	}
	if len(results) != 1 || results[0].Run.Status != RunStatusCompleted || results[0].Execution.Status != workflow.ExecutionStatusPartialSuccess || results[0].Err != nil {
		t.Fatalf("unexpected partial success due task result: %+v", results)
	}
}

func TestRunDueTasksRunsWorkflowUntilBlockedBeforeCompletingRun(t *testing.T) {
	now := time.Date(2026, time.June, 5, 9, 0, 0, 0, time.UTC)
	store := &fakeStore{
		claimedRuns: []ClaimedScheduledTaskRun{
			{
				Task: ScheduledTask{
					ID:             "sched_1",
					OrganizationID: "org_1",
					TargetType:     TargetTypeWorkflow,
					TargetID:       "workflow_1",
					CronExpression: "0 * * * *",
					Enabled:        true,
					NextRunAt:      &now,
				},
				Run: ScheduledTaskRun{
					ID:              "schedrun_1",
					OrganizationID:  "org_1",
					ScheduledTaskID: "sched_1",
					Status:          RunStatusRunning,
					StartedAt:       &now,
				},
			},
		},
	}
	starter := &fakeWorkflowStarter{
		result:    &workflow.WorkflowExecution{ID: "wexec_1", OrganizationID: "org_1", WorkflowID: "workflow_1", Status: workflow.ExecutionStatusRunning},
		runResult: &workflow.WorkflowExecution{ID: "wexec_1", OrganizationID: "org_1", WorkflowID: "workflow_1", Status: workflow.ExecutionStatusRunning},
	}
	service := NewService(store, WithWorkflowStarter(starter))

	results, err := service.RunDueTasks(context.Background(), now, 5)
	if err != nil {
		t.Fatalf("RunDueTasks returned error: %v", err)
	}

	if starter.runCalls != 1 || starter.runExecutionID != "wexec_1" {
		t.Fatalf("expected claimed workflow execution to run until blocked, calls=%d execution=%q", starter.runCalls, starter.runExecutionID)
	}
	if store.completeRunCalls != 0 {
		t.Fatalf("expected still-running workflow not to complete scheduled run, got %d complete calls", store.completeRunCalls)
	}
	if store.updateRunCalls != 0 {
		t.Fatalf("expected still-running workflow run to remain running, got update calls=%d input=%+v", store.updateRunCalls, store.updatedRunInput)
	}
	if len(results) != 1 || results[0].Run.Status != RunStatusRunning || results[0].Execution.Status != workflow.ExecutionStatusRunning || results[0].Err != nil {
		t.Fatalf("unexpected blocked workflow result: %+v", results)
	}
}

func TestRunDueTasksStartsDueAgentTargetAndAdvancesScheduleOnSuccess(t *testing.T) {
	now := time.Date(2026, time.June, 5, 9, 0, 0, 0, time.UTC)
	store := &fakeStore{
		claimedRuns: []ClaimedScheduledTaskRun{
			{
				Task: ScheduledTask{
					ID:             "sched_agent_1",
					OrganizationID: "org_1",
					TargetType:     TargetTypeAgent,
					TargetID:       "agent_1",
					CronExpression: "0 * * * *",
					Enabled:        true,
					NextRunAt:      &now,
				},
				Run: ScheduledTaskRun{
					ID:              "schedrun_agent_1",
					OrganizationID:  "org_1",
					ScheduledTaskID: "sched_agent_1",
					Status:          RunStatusRunning,
					StartedAt:       &now,
				},
			},
		},
	}
	agentStarter := &fakeAgentStarter{
		result: &agent.RunWithMessages{Run: &agent.Run{ID: "run_agent_1", Status: agent.RunStatusCompleted}},
	}
	service := NewService(store, WithAgentStarter(agentStarter))

	results, err := service.RunDueTasks(context.Background(), now, 5)
	if err != nil {
		t.Fatalf("RunDueTasks returned error: %v", err)
	}

	if agentStarter.calls != 1 {
		t.Fatalf("expected agent starter once, got %d", agentStarter.calls)
	}
	if agentStarter.session.OrganizationID != "org_1" || agentStarter.session.User.ID != "scheduled_task:sched_agent_1" {
		t.Fatalf("unexpected agent starter session: %+v", agentStarter.session)
	}
	if agentStarter.request.AgentID != "agent_1" || agentStarter.request.ConversationID != "schedrun_agent_1" {
		t.Fatalf("unexpected agent start request: %+v", agentStarter.request)
	}
	if agentStarter.request.Input == "" || !strings.Contains(agentStarter.request.Input, "sched_agent_1") || !strings.Contains(agentStarter.request.Input, "schedrun_agent_1") {
		t.Fatalf("expected scheduled task context in agent input, got %q", agentStarter.request.Input)
	}
	if store.completeRunCalls != 1 || store.completedTaskID != "sched_agent_1" || store.completedRunID != "schedrun_agent_1" {
		t.Fatalf("expected successful agent task to complete claimed run, calls=%d task=%q run=%q", store.completeRunCalls, store.completedTaskID, store.completedRunID)
	}
	expectedNextRun := time.Date(2026, time.June, 5, 10, 0, 0, 0, time.UTC)
	if !store.completedRunInput.NextRunAt.Equal(expectedNextRun) {
		t.Fatalf("expected next run %v, got %v", expectedNextRun, store.completedRunInput.NextRunAt)
	}
	if len(results) != 1 || results[0].Run.Status != RunStatusCompleted || results[0].Err != nil {
		t.Fatalf("unexpected due agent result: %+v", results)
	}
}

func TestRunDueTasksMarksClaimedRunFailedWhenWorkflowStartFails(t *testing.T) {
	now := time.Date(2026, time.June, 5, 9, 0, 0, 0, time.UTC)
	store := &fakeStore{
		claimedRuns: []ClaimedScheduledTaskRun{
			{
				Task: ScheduledTask{
					ID:             "sched_1",
					OrganizationID: "org_1",
					TargetType:     TargetTypeWorkflow,
					TargetID:       "workflow_1",
					CronExpression: "0 * * * *",
					Enabled:        true,
					NextRunAt:      &now,
				},
				Run: ScheduledTaskRun{
					ID:              "schedrun_1",
					OrganizationID:  "org_1",
					ScheduledTaskID: "sched_1",
					Status:          RunStatusRunning,
					StartedAt:       &now,
				},
			},
		},
	}
	starter := &fakeWorkflowStarter{err: errors.New("workflow unavailable")}
	service := NewService(store, WithWorkflowStarter(starter))

	results, err := service.RunDueTasks(context.Background(), now, 5)
	if err != nil {
		t.Fatalf("RunDueTasks returned error: %v", err)
	}

	if starter.calls != 1 {
		t.Fatalf("expected workflow starter once, got %d", starter.calls)
	}
	if store.completeRunCalls != 0 {
		t.Fatalf("expected failed workflow not to use success completion, got %d complete calls", store.completeRunCalls)
	}
	if store.failRunCalls != 1 || store.failedTaskID != "sched_1" || store.failedRunID != "schedrun_1" || store.failedRunInput.Error != "workflow unavailable" || store.failedRunInput.FinishedAt.IsZero() {
		t.Fatalf("expected claimed run marked failed with task advancement, calls=%d task=%q run=%q input=%+v", store.failRunCalls, store.failedTaskID, store.failedRunID, store.failedRunInput)
	}
	expectedNextRun := time.Date(2026, time.June, 5, 10, 0, 0, 0, time.UTC)
	if !store.failedRunInput.NextRunAt.Equal(expectedNextRun) {
		t.Fatalf("expected failed run to advance next run %v, got %v", expectedNextRun, store.failedRunInput.NextRunAt)
	}
	if store.updateRunCalls != 0 {
		t.Fatalf("expected failed claimed run to use atomic fail path, got update calls=%d input=%+v", store.updateRunCalls, store.updatedRunInput)
	}
	if len(results) != 1 || results[0].Run.ID != "schedrun_1" || results[0].Run.Status != RunStatusFailed || results[0].Err == nil {
		t.Fatalf("unexpected due task failure result: %+v", results)
	}
}

func TestWorkerPollsDueTasksUntilContextCancelled(t *testing.T) {
	now := time.Date(2026, time.June, 5, 9, 0, 0, 0, time.UTC)
	store := &fakeStore{}
	service := NewService(store, WithWorkflowStarter(&fakeWorkflowStarter{}))
	worker := NewWorker(service, WorkerConfig{
		Interval: 5 * time.Millisecond,
		Limit:    7,
		Now:      func() time.Time { return now },
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		worker.Run(ctx)
		close(done)
	}()

	deadline := time.After(250 * time.Millisecond)
	for {
		if store.claimDueCalls > 0 {
			break
		}
		select {
		case <-deadline:
			cancel()
			t.Fatalf("expected worker to poll due tasks before deadline")
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("expected worker to stop after context cancellation")
	}
	if store.claimedInput.Limit != 7 || !store.claimedInput.Now.Equal(now) {
		t.Fatalf("expected worker to pass configured now/limit, got %+v", store.claimedInput)
	}
}
