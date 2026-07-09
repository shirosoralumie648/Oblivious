package http

import (
	"context"
	"testing"
	"time"

	"oblivious/server/internal/agent"
	"oblivious/server/internal/auth"
	"oblivious/server/internal/schedule"
	"oblivious/server/internal/workflow"
)

func TestDefaultScheduleServiceWiresAgentStarter(t *testing.T) {
	now := time.Date(2026, time.June, 5, 9, 0, 0, 0, time.UTC)
	store := &scheduleWiringStore{
		claimedRuns: []schedule.ClaimedScheduledTaskRun{
			{
				Task: schedule.ScheduledTask{
					ID:             "sched_agent_1",
					OrganizationID: "org_1",
					TargetType:     schedule.TargetTypeAgent,
					TargetID:       "agent_1",
					CronExpression: "0 * * * *",
					Enabled:        true,
					NextRunAt:      &now,
				},
				Run: schedule.ScheduledTaskRun{
					ID:              "schedrun_agent_1",
					OrganizationID:  "org_1",
					ScheduledTaskID: "sched_agent_1",
					Status:          schedule.RunStatusRunning,
					StartedAt:       &now,
				},
			},
		},
	}
	agentStarter := &scheduleWiringAgentStarter{
		result: &agent.RunWithMessages{Run: &agent.Run{ID: "run_agent_1", Status: agent.RunStatusCompleted}},
	}
	service := newScheduleService(store, nil, agentStarter)

	results, err := service.RunDueTasks(context.Background(), now, 1)
	if err != nil {
		t.Fatalf("RunDueTasks returned error: %v", err)
	}

	if agentStarter.calls != 1 {
		t.Fatalf("expected default schedule service to call agent starter once, got %d", agentStarter.calls)
	}
	if agentStarter.request.AgentID != "agent_1" || agentStarter.request.ConversationID != "schedrun_agent_1" {
		t.Fatalf("unexpected agent request: %+v", agentStarter.request)
	}
	if store.completeRunCalls != 1 || store.completedRunID != "schedrun_agent_1" {
		t.Fatalf("expected agent scheduled run completion, calls=%d run=%q", store.completeRunCalls, store.completedRunID)
	}
	if len(results) != 1 || results[0].Err != nil || results[0].Run.Status != schedule.RunStatusCompleted {
		t.Fatalf("unexpected due task result: %+v", results)
	}
}

func TestWorkflowServiceCanUseDefaultScheduleServiceForScheduleTriggerSync(t *testing.T) {
	workflowStore := newScheduleWiringWorkflowStore()
	scheduleStore := &scheduleWiringStore{}
	workflowService := workflow.NewService(workflowStore)
	_ = newScheduleService(scheduleStore, workflowService, nil)

	created, err := workflowService.CreateWorkflow(context.Background(), workflow.CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Scheduled workflow",
		Status:         workflow.WorkflowStatusPublished,
		Definition: map[string]any{
			"nodes": []any{map[string]any{"id": "start", "type": "manual"}},
			"triggers": map[string]any{
				"schedule": map[string]any{"id": "daily-report", "cron": "0 9 * * 1"},
			},
		},
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}

	if scheduleStore.syncWorkflowCalls != 1 {
		t.Fatalf("expected workflow create to sync schedule triggers once, got %d", scheduleStore.syncWorkflowCalls)
	}
	input := scheduleStore.syncedWorkflowInput
	if input.OrganizationID != "org_1" || input.WorkflowID != created.ID {
		t.Fatalf("unexpected synced workflow scope: %+v", input)
	}
	if len(input.Triggers) != 1 || input.Triggers[0].TriggerID != "daily-report" || input.Triggers[0].CronExpression != "0 9 * * 1" {
		t.Fatalf("unexpected synced schedule trigger input: %+v", input.Triggers)
	}
}

type scheduleWiringAgentStarter struct {
	calls   int
	session auth.Session
	request agent.StartRunRequest
	result  *agent.RunWithMessages
	err     error
}

func (s *scheduleWiringAgentStarter) StartRun(ctx context.Context, session auth.Session, request agent.StartRunRequest) (*agent.RunWithMessages, error) {
	_ = ctx
	s.calls++
	s.session = session
	s.request = request
	return s.result, s.err
}

type scheduleWiringStore struct {
	claimedRuns []schedule.ClaimedScheduledTaskRun

	syncedWorkflowInput schedule.SyncWorkflowScheduledTasksInput
	syncWorkflowCalls   int
	completeRunCalls    int
	completedTaskID     string
	completedRunID      string
}

var _ schedule.DueTaskStore = (*scheduleWiringStore)(nil)

func (s *scheduleWiringStore) CreateScheduledTask(ctx context.Context, input schedule.CreateScheduledTaskInput) (schedule.ScheduledTask, error) {
	_ = ctx
	_ = input
	return schedule.ScheduledTask{}, nil
}

func (s *scheduleWiringStore) SyncWorkflowScheduledTasks(ctx context.Context, input schedule.SyncWorkflowScheduledTasksInput) ([]schedule.ScheduledTask, error) {
	_ = ctx
	s.syncWorkflowCalls++
	s.syncedWorkflowInput = input
	return nil, nil
}

func (s *scheduleWiringStore) ListScheduledTasks(ctx context.Context, organizationID string) ([]schedule.ScheduledTask, error) {
	_ = ctx
	_ = organizationID
	return nil, nil
}

func (s *scheduleWiringStore) GetScheduledTask(ctx context.Context, organizationID string, scheduledTaskID string) (schedule.ScheduledTask, error) {
	_ = ctx
	return schedule.ScheduledTask{
		ID:             scheduledTaskID,
		OrganizationID: organizationID,
		TargetType:     schedule.TargetTypeAgent,
		TargetID:       "agent_1",
		CronExpression: "0 * * * *",
		Enabled:        true,
	}, nil
}

func (s *scheduleWiringStore) UpdateScheduledTaskEnabled(ctx context.Context, organizationID string, scheduledTaskID string, input schedule.UpdateScheduledTaskEnabledInput) (schedule.ScheduledTask, error) {
	_ = ctx
	return schedule.ScheduledTask{
		ID:             scheduledTaskID,
		OrganizationID: organizationID,
		TargetType:     schedule.TargetTypeAgent,
		TargetID:       "agent_1",
		CronExpression: "0 * * * *",
		Enabled:        input.Enabled,
		NextRunAt:      input.NextRunAt,
	}, nil
}

func (s *scheduleWiringStore) RecordScheduledTaskRun(ctx context.Context, input schedule.RecordScheduledTaskRunInput) (schedule.ScheduledTaskRun, error) {
	_ = ctx
	_ = input
	return schedule.ScheduledTaskRun{}, nil
}

func (s *scheduleWiringStore) CompleteManualScheduledTaskRun(ctx context.Context, organizationID string, scheduledTaskID string, scheduledTaskRunID string, finishedAt time.Time) (schedule.ScheduledTaskRun, error) {
	_ = ctx
	return schedule.ScheduledTaskRun{
		ID:              scheduledTaskRunID,
		OrganizationID:  organizationID,
		ScheduledTaskID: scheduledTaskID,
		Status:          schedule.RunStatusCompleted,
		FinishedAt:      &finishedAt,
	}, nil
}

func (s *scheduleWiringStore) UpdateScheduledTaskRun(ctx context.Context, organizationID string, scheduledTaskRunID string, input schedule.UpdateScheduledTaskRunInput) (schedule.ScheduledTaskRun, error) {
	_ = ctx
	_ = input
	return schedule.ScheduledTaskRun{
		ID:             scheduledTaskRunID,
		OrganizationID: organizationID,
		Status:         schedule.RunStatusFailed,
	}, nil
}

func (s *scheduleWiringStore) ListScheduledTaskRuns(ctx context.Context, organizationID string, scheduledTaskID string) ([]schedule.ScheduledTaskRun, error) {
	_ = ctx
	_ = organizationID
	_ = scheduledTaskID
	return nil, nil
}

func (s *scheduleWiringStore) CountRunningScheduledTaskRuns(ctx context.Context, organizationID string, scheduledTaskID string) (int, error) {
	_ = ctx
	_ = organizationID
	_ = scheduledTaskID
	return 0, nil
}

func (s *scheduleWiringStore) ClaimDueScheduledTaskRuns(ctx context.Context, input schedule.ClaimDueScheduledTaskRunsInput) ([]schedule.ClaimedScheduledTaskRun, error) {
	_ = ctx
	_ = input
	return s.claimedRuns, nil
}

func (s *scheduleWiringStore) CompleteScheduledTaskRun(ctx context.Context, organizationID string, scheduledTaskID string, scheduledTaskRunID string, input schedule.CompleteScheduledTaskRunInput) (schedule.ScheduledTaskRun, error) {
	_ = ctx
	_ = input
	s.completeRunCalls++
	s.completedTaskID = scheduledTaskID
	s.completedRunID = scheduledTaskRunID
	return schedule.ScheduledTaskRun{
		ID:              scheduledTaskRunID,
		OrganizationID:  organizationID,
		ScheduledTaskID: scheduledTaskID,
		Status:          schedule.RunStatusCompleted,
		FinishedAt:      &input.FinishedAt,
	}, nil
}

func (s *scheduleWiringStore) FailScheduledTaskRun(ctx context.Context, organizationID string, scheduledTaskID string, scheduledTaskRunID string, input schedule.FailScheduledTaskRunInput) (schedule.ScheduledTaskRun, error) {
	_ = ctx
	_ = input
	return schedule.ScheduledTaskRun{
		ID:              scheduledTaskRunID,
		OrganizationID:  organizationID,
		ScheduledTaskID: scheduledTaskID,
		Status:          schedule.RunStatusFailed,
		FinishedAt:      &input.FinishedAt,
		Error:           input.Error,
	}, nil
}

type scheduleWiringWorkflowStore struct {
	workflows map[string]*workflow.WorkflowDefinition
	nextID    int
}

func newScheduleWiringWorkflowStore() *scheduleWiringWorkflowStore {
	return &scheduleWiringWorkflowStore{workflows: map[string]*workflow.WorkflowDefinition{}, nextID: 1}
}

func (s *scheduleWiringWorkflowStore) CreateWorkflow(ctx context.Context, req workflow.CreateWorkflowRequest) (*workflow.WorkflowDefinition, error) {
	_ = ctx
	id := "workflow_wiring_1"
	if s.nextID > 1 {
		id = "workflow_wiring_2"
	}
	s.nextID++
	now := time.Now().UTC()
	created := &workflow.WorkflowDefinition{
		ID:             id,
		OrganizationID: req.OrganizationID,
		Name:           req.Name,
		Description:    req.Description,
		Status:         req.Status,
		Version:        1,
		Definition:     req.Definition,
		Variables:      req.Variables,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if created.Status == "" {
		created.Status = workflow.WorkflowStatusDraft
	}
	s.workflows[id] = created
	return created, nil
}

func (s *scheduleWiringWorkflowStore) GetWorkflow(ctx context.Context, organizationID, id string) (*workflow.WorkflowDefinition, error) {
	_ = ctx
	workflowDefinition := s.workflows[id]
	if workflowDefinition == nil || workflowDefinition.OrganizationID != organizationID {
		return nil, nil
	}
	return workflowDefinition, nil
}

func (s *scheduleWiringWorkflowStore) ListWorkflows(ctx context.Context, organizationID string) ([]*workflow.WorkflowDefinition, error) {
	_ = ctx
	return nil, nil
}

func (s *scheduleWiringWorkflowStore) ListWorkflowVersions(ctx context.Context, organizationID, workflowID string) ([]*workflow.WorkflowDefinition, error) {
	_ = ctx
	return nil, nil
}

func (s *scheduleWiringWorkflowStore) GetWorkflowVersion(ctx context.Context, organizationID, workflowID string, version int) (*workflow.WorkflowDefinition, error) {
	_ = ctx
	return nil, nil
}

func (s *scheduleWiringWorkflowStore) UpdateWorkflow(ctx context.Context, req workflow.UpdateWorkflowStoreRequest) (*workflow.WorkflowDefinition, error) {
	_ = ctx
	return nil, nil
}

func (s *scheduleWiringWorkflowStore) CreateExecution(ctx context.Context, req workflow.CreateExecutionRequest) (*workflow.WorkflowExecution, error) {
	_ = ctx
	return nil, nil
}

func (s *scheduleWiringWorkflowStore) ListExecutions(ctx context.Context, organizationID, workflowID string) ([]*workflow.WorkflowExecution, error) {
	_ = ctx
	return nil, nil
}

func (s *scheduleWiringWorkflowStore) GetExecution(ctx context.Context, organizationID, id string) (*workflow.WorkflowExecution, error) {
	_ = ctx
	return nil, nil
}

func (s *scheduleWiringWorkflowStore) ListExecutionEvents(ctx context.Context, organizationID, executionID string) ([]workflow.WorkflowExecutionEvent, error) {
	_ = ctx
	return nil, nil
}

func (s *scheduleWiringWorkflowStore) ListActiveExecutionHealth(ctx context.Context, organizationID string, statuses []workflow.ExecutionStatus) ([]workflow.WorkflowExecutionHealthSummary, error) {
	_ = ctx
	return nil, nil
}

func (s *scheduleWiringWorkflowStore) CountRunningExecutions(ctx context.Context, organizationID, workflowID string) (int, error) {
	_ = ctx
	return 0, nil
}

func (s *scheduleWiringWorkflowStore) CountRunningExecutionsForOrganization(ctx context.Context, organizationID string) (int, error) {
	_ = ctx
	return 0, nil
}

func (s *scheduleWiringWorkflowStore) UpdateExecutionStatus(ctx context.Context, organizationID, id string, status workflow.ExecutionStatus, completedAt *time.Time) (*workflow.WorkflowExecution, error) {
	_ = ctx
	return nil, nil
}

func (s *scheduleWiringWorkflowStore) UpdateExecutionStatusIfCurrent(ctx context.Context, organizationID, id string, fromStatus, status workflow.ExecutionStatus, completedAt *time.Time) (*workflow.WorkflowExecution, error) {
	_ = ctx
	return nil, nil
}

func (s *scheduleWiringWorkflowStore) CreateNodeExecution(ctx context.Context, organizationID, executionID string, req workflow.CreateNodeExecutionRequest) (*workflow.WorkflowNodeExecution, error) {
	_ = ctx
	return nil, nil
}
