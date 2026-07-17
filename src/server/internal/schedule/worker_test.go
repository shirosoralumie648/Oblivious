package schedule

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/agent"
	"oblivious/server/internal/releasecontract"
	"oblivious/server/internal/workflow"
)

func TestScheduledTaskReadinessGuardContract(t *testing.T) {
	now := time.Date(2026, time.July, 18, 1, 0, 0, 0, time.UTC)
	denialCodes := []releasecontract.ReadinessCode{
		releasecontract.CodeCapabilityDisabled,
		releasecontract.CodeCapabilityBlocked,
		releasecontract.CodeReadinessStale,
		releasecontract.CodeCapabilityUnknown,
		releasecontract.CodeReadinessUnavailable,
		releasecontract.CodeBuildIdentityMismatch,
	}
	for _, code := range denialCodes {
		t.Run("claim denial "+string(code), func(t *testing.T) {
			store, workflowStarter, agentStarter := scheduledReadinessFixture(now, TargetTypeWorkflow)
			guard := &scheduledGuardSpy{denyAtCall: 1, denial: &releasecontract.ReadinessError{Code: code}}
			worker := newScheduledReadinessWorker(t, store, workflowStarter, agentStarter, guard, now)

			worker.runOnce(context.Background())

			if store.claimDueCalls != 0 || workflowStarter.calls != 0 || workflowStarter.runCalls != 0 || agentStarter.calls != 0 {
				t.Fatalf("denied claim reached downstream effects: claim=%d workflowStart=%d workflowContinue=%d agent=%d", store.claimDueCalls, workflowStarter.calls, workflowStarter.runCalls, agentStarter.calls)
			}
			guard.assertCall(t, 0, "task.scheduled_execution", releasecontract.BoundaryWorkerClaim)
		})
	}

	t.Run("nil guard fails before loop construction", func(t *testing.T) {
		contract, profile := loadScheduledReadinessAuthority(t)
		authorities, err := releasecontract.NewRuntimeAuthorities(contract, profile, &scheduledGuardSpy{})
		if err != nil {
			t.Fatalf("compile runtime authorities: %v", err)
		}
		_, err = NewReadinessWorker(NewService(&fakeStore{}), WorkerConfig{
			Guard: nil, Authorities: authorities, Effects: &scheduledEffectRegistrar{},
		})
		if !releasecontract.IsReadinessCode(err, releasecontract.CodeReadinessUnavailable) {
			t.Fatalf("nil guard error = %v", err)
		}
	})

	t.Run("expiry after claim blocks workflow start and records only failure bookkeeping", func(t *testing.T) {
		store, workflowStarter, agentStarter := scheduledReadinessFixture(now, TargetTypeWorkflow)
		guard := &scheduledGuardSpy{denyAtCall: 2, denial: &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessStale}}
		worker := newScheduledReadinessWorker(t, store, workflowStarter, agentStarter, guard, now)

		worker.runOnce(context.Background())

		if store.claimDueCalls != 1 || workflowStarter.calls != 0 || workflowStarter.runCalls != 0 || agentStarter.calls != 0 {
			t.Fatalf("expiry after claim reached target: claim=%d workflowStart=%d workflowContinue=%d agent=%d", store.claimDueCalls, workflowStarter.calls, workflowStarter.runCalls, agentStarter.calls)
		}
		if store.failRunCalls != 1 || store.failedRunInput.Error != string(releasecontract.CodeReadinessStale) {
			t.Fatalf("expected stable failure bookkeeping, calls=%d input=%+v", store.failRunCalls, store.failedRunInput)
		}
	})

	t.Run("expiry after workflow creation blocks continuation independently", func(t *testing.T) {
		store, workflowStarter, agentStarter := scheduledReadinessFixture(now, TargetTypeWorkflow)
		guard := &scheduledGuardSpy{denyAtCall: 3, denial: &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessStale}}
		worker := newScheduledReadinessWorker(t, store, workflowStarter, agentStarter, guard, now)

		worker.runOnce(context.Background())

		if store.claimDueCalls != 1 || workflowStarter.calls != 1 || workflowStarter.runCalls != 0 || agentStarter.calls != 0 {
			t.Fatalf("expiry before continuation reached target: claim=%d workflowStart=%d workflowContinue=%d agent=%d", store.claimDueCalls, workflowStarter.calls, workflowStarter.runCalls, agentStarter.calls)
		}
		if store.failRunCalls != 1 || store.completeRunCalls != 0 {
			t.Fatalf("expected bounded failure bookkeeping only: failed=%d completed=%d", store.failRunCalls, store.completeRunCalls)
		}
	})

	t.Run("expiry after agent claim blocks agent start independently", func(t *testing.T) {
		store, workflowStarter, agentStarter := scheduledReadinessFixture(now, TargetTypeAgent)
		guard := &scheduledGuardSpy{denyAtCall: 2, denial: &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessStale}}
		worker := newScheduledReadinessWorker(t, store, workflowStarter, agentStarter, guard, now)

		worker.runOnce(context.Background())

		if store.claimDueCalls != 1 || workflowStarter.calls != 0 || workflowStarter.runCalls != 0 || agentStarter.calls != 0 {
			t.Fatalf("expiry before agent start reached target: claim=%d workflowStart=%d workflowContinue=%d agent=%d", store.claimDueCalls, workflowStarter.calls, workflowStarter.runCalls, agentStarter.calls)
		}
		if store.failRunCalls != 1 || store.completeRunCalls != 0 {
			t.Fatalf("expected bounded failure bookkeeping only: failed=%d completed=%d", store.failRunCalls, store.completeRunCalls)
		}
	})

	t.Run("authorizing guard preserves success and registers stable descriptors", func(t *testing.T) {
		store, workflowStarter, agentStarter := scheduledReadinessFixture(now, TargetTypeWorkflow)
		guard := &scheduledGuardSpy{}
		registrar := &scheduledEffectRegistrar{}
		worker := newScheduledReadinessWorkerWithRegistrar(t, store, workflowStarter, agentStarter, guard, registrar, now)

		worker.runOnce(context.Background())

		if store.claimDueCalls != 1 || workflowStarter.calls != 1 || workflowStarter.runCalls != 1 || store.completeRunCalls != 1 {
			t.Fatalf("authorized behavior changed: claim=%d start=%d continue=%d complete=%d", store.claimDueCalls, workflowStarter.calls, workflowStarter.runCalls, store.completeRunCalls)
		}
		if len(registrar.descriptors) != 4 {
			t.Fatalf("registered descriptors = %#v", registrar.descriptors)
		}
	})
}

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

type scheduledGuardCall struct {
	capabilityID string
	boundary     releasecontract.Boundary
}

type scheduledGuardSpy struct {
	denyAtCall int
	denial     error
	calls      []scheduledGuardCall
}

func (g *scheduledGuardSpy) Require(_ context.Context, capabilityID string, boundary releasecontract.Boundary) error {
	g.calls = append(g.calls, scheduledGuardCall{capabilityID: capabilityID, boundary: boundary})
	if g.denyAtCall > 0 && len(g.calls) >= g.denyAtCall {
		return g.denial
	}
	return nil
}

func (g *scheduledGuardSpy) assertCall(t *testing.T, index int, capabilityID string, boundary releasecontract.Boundary) {
	t.Helper()
	if len(g.calls) <= index || g.calls[index].capabilityID != capabilityID || g.calls[index].boundary != boundary {
		t.Fatalf("guard call %d = %#v, all calls=%#v", index, func() any {
			if len(g.calls) <= index {
				return nil
			}
			return g.calls[index]
		}(), g.calls)
	}
}

type scheduledEffectRegistrar struct {
	descriptors []releasecontract.EffectDescriptor
}

func (r *scheduledEffectRegistrar) Register(descriptor releasecontract.EffectDescriptor) error {
	r.descriptors = append(r.descriptors, descriptor)
	return nil
}

func newScheduledReadinessWorker(t *testing.T, store *fakeStore, workflowStarter *fakeWorkflowStarter, agentStarter *fakeAgentStarter, guard *scheduledGuardSpy, now time.Time) *Worker {
	t.Helper()
	return newScheduledReadinessWorkerWithRegistrar(t, store, workflowStarter, agentStarter, guard, &scheduledEffectRegistrar{}, now)
}

func newScheduledReadinessWorkerWithRegistrar(t *testing.T, store *fakeStore, workflowStarter *fakeWorkflowStarter, agentStarter *fakeAgentStarter, guard *scheduledGuardSpy, registrar *scheduledEffectRegistrar, now time.Time) *Worker {
	t.Helper()
	contract, profile := loadScheduledReadinessAuthority(t)
	authorities, err := releasecontract.NewRuntimeAuthorities(contract, profile, guard)
	if err != nil {
		t.Fatalf("compile runtime authorities: %v", err)
	}
	worker, err := NewReadinessWorker(
		NewService(store, WithWorkflowStarter(workflowStarter), WithAgentStarter(agentStarter)),
		WorkerConfig{
			Now: nowFunc(now), Limit: 5, Guard: guard, Authorities: authorities, Effects: registrar,
		},
	)
	if err != nil {
		t.Fatalf("construct readiness worker: %v", err)
	}
	return worker
}

func scheduledReadinessFixture(now time.Time, targetType string) (*fakeStore, *fakeWorkflowStarter, *fakeAgentStarter) {
	store := &fakeStore{claimedRuns: []ClaimedScheduledTaskRun{{
		Task: ScheduledTask{
			ID: "sched_guarded", OrganizationID: "org_1", TargetType: targetType,
			TargetID: "target_1", CronExpression: "0 * * * *", Enabled: true, NextRunAt: &now,
		},
		Run: ScheduledTaskRun{
			ID: "schedrun_guarded", OrganizationID: "org_1", ScheduledTaskID: "sched_guarded",
			Status: RunStatusRunning, StartedAt: &now,
		},
	}}}
	workflowStarter := &fakeWorkflowStarter{
		result:    &workflow.WorkflowExecution{ID: "wexec_guarded", OrganizationID: "org_1", WorkflowID: "target_1", Status: workflow.ExecutionStatusRunning},
		runResult: &workflow.WorkflowExecution{ID: "wexec_guarded", OrganizationID: "org_1", WorkflowID: "target_1", Status: workflow.ExecutionStatusSucceeded},
	}
	agentStarter := &fakeAgentStarter{result: &agent.RunWithMessages{Run: &agent.Run{ID: "run_guarded", Status: agent.RunStatusCompleted}}}
	return store, workflowStarter, agentStarter
}

func nowFunc(now time.Time) func() time.Time {
	return func() time.Time { return now }
}

func loadScheduledReadinessAuthority(t *testing.T) (releasecontract.AuthoredContractV1, releasecontract.DeploymentProfile) {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve schedule readiness test source")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(source), "../../../.."))
	contract, err := releasecontract.Load(context.Background(), repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json")
	if err != nil {
		t.Fatalf("load release contract: %v", err)
	}
	for _, profile := range contract.Profiles {
		if profile.ID == "monolith" {
			return contract, profile
		}
	}
	t.Fatal("monolith profile missing")
	return releasecontract.AuthoredContractV1{}, releasecontract.DeploymentProfile{}
}
