package schedule

import (
	"context"
	"errors"
	"strings"
	"time"

	"oblivious/server/internal/agent"
	"oblivious/server/internal/auth"
	"oblivious/server/internal/workflow"
)

const defaultDueTaskClaimLimit = 50

const defaultWorkerInterval = time.Minute

type WorkerConfig struct {
	Interval time.Duration
	Limit    int
	Now      func() time.Time
	OnError  func(error)
}

type Worker struct {
	service  *Service
	interval time.Duration
	limit    int
	now      func() time.Time
	onError  func(error)
}

func NewWorker(service *Service, config WorkerConfig) *Worker {
	interval := config.Interval
	if interval <= 0 {
		interval = defaultWorkerInterval
	}
	limit := config.Limit
	if limit <= 0 {
		limit = defaultDueTaskClaimLimit
	}
	now := config.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Worker{
		service:  service,
		interval: interval,
		limit:    limit,
		now:      now,
		onError:  config.OnError,
	}
}

func (w *Worker) Run(ctx context.Context) {
	if w == nil || w.service == nil {
		return
	}
	w.runOnce(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *Worker) runOnce(ctx context.Context) {
	if err := ctx.Err(); err != nil {
		return
	}
	if _, err := w.service.RunDueTasks(ctx, w.now(), w.limit); err != nil && w.onError != nil {
		w.onError(err)
	}
}

type DueTaskRunResult struct {
	Task      ScheduledTask
	Run       ScheduledTaskRun
	Execution *workflow.WorkflowExecution
	AgentRun  *agent.RunWithMessages
	Err       error
}

func (s *Service) RunDueTasks(ctx context.Context, now time.Time, limit int) ([]DueTaskRunResult, error) {
	if s.workflowStarter == nil && s.agentStarter == nil {
		return nil, workflow.ErrInvalidInput
	}
	dueStore, ok := s.store.(DueTaskStore)
	if !ok {
		return nil, ErrDueTaskStoreUnavailable
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	if limit <= 0 {
		limit = defaultDueTaskClaimLimit
	}

	claimedRuns, err := dueStore.ClaimDueScheduledTaskRuns(ctx, ClaimDueScheduledTaskRunsInput{
		Now:   now,
		Limit: limit,
	})
	if err != nil {
		return nil, err
	}

	results := make([]DueTaskRunResult, 0, len(claimedRuns))
	for _, claimed := range claimedRuns {
		targetType := strings.ToLower(strings.TrimSpace(claimed.Task.TargetType))
		if targetType == TargetTypeAgent {
			results = append(results, s.runClaimedAgentTask(ctx, dueStore, claimed, now))
			continue
		}
		results = append(results, s.runClaimedWorkflowTask(ctx, dueStore, claimed, now))
	}

	return results, nil
}

func (s *Service) runClaimedWorkflowTask(ctx context.Context, dueStore DueTaskStore, claimed ClaimedScheduledTaskRun, now time.Time) DueTaskRunResult {
	result := DueTaskRunResult{Task: claimed.Task, Run: claimed.Run}
	session := auth.Session{OrganizationID: claimed.Task.OrganizationID}

	if s.workflowStarter == nil {
		return s.failClaimedRun(ctx, dueStore, result, session, errors.New("workflow starter is not configured"), now)
	}

	execution, err := s.startClaimedWorkflow(ctx, claimed.Task, claimed.Run, nil)
	finishedAt := time.Now().UTC()
	if err != nil {
		return s.failClaimedRunAt(ctx, dueStore, result, session, err, now, finishedAt)
	}

	execution, err = s.workflowStarter.RunExecutionUntilBlocked(ctx, claimed.Task.OrganizationID, execution.ID)
	finishedAt = time.Now().UTC()
	if err != nil {
		result.Execution = execution
		return s.failClaimedRunAt(ctx, dueStore, result, session, err, now, finishedAt)
	}
	if execution == nil || !isTerminalWorkflowExecutionStatus(execution.Status) {
		result.Execution = execution
		return result
	}
	if isFailedWorkflowExecutionStatus(execution.Status) {
		errorMessage := "scheduled workflow execution failed"
		if execution != nil && execution.Error != nil {
			if message, ok := execution.Error["message"].(string); ok && strings.TrimSpace(message) != "" {
				errorMessage = strings.TrimSpace(message)
			}
		}
		result.Execution = execution
		return s.failClaimedRunAt(ctx, dueStore, result, session, errors.New(errorMessage), now, finishedAt)
	}

	result.Execution = execution
	return s.completeClaimedRun(ctx, dueStore, result, now, finishedAt)
}

func (s *Service) runClaimedAgentTask(ctx context.Context, dueStore DueTaskStore, claimed ClaimedScheduledTaskRun, now time.Time) DueTaskRunResult {
	result := DueTaskRunResult{Task: claimed.Task, Run: claimed.Run}
	session := auth.Session{
		OrganizationID: claimed.Task.OrganizationID,
		User: auth.User{
			ID: "scheduled_task:" + strings.TrimSpace(claimed.Task.ID),
		},
	}
	if s.agentStarter == nil {
		return s.failClaimedRun(ctx, dueStore, result, session, errors.New("agent starter is not configured"), now)
	}

	agentRun, err := s.agentStarter.StartRun(ctx, session, agent.StartRunRequest{
		AgentID:        strings.TrimSpace(claimed.Task.TargetID),
		ConversationID: strings.TrimSpace(claimed.Run.ID),
		Input:          scheduledAgentInput(claimed.Task, claimed.Run),
	})
	finishedAt := time.Now().UTC()
	if err != nil {
		result.AgentRun = agentRun
		return s.failClaimedRunAt(ctx, dueStore, result, session, err, now, finishedAt)
	}
	result.AgentRun = agentRun
	return s.completeClaimedRun(ctx, dueStore, result, now, finishedAt)
}

func (s *Service) completeClaimedRun(ctx context.Context, dueStore DueTaskStore, result DueTaskRunResult, now, finishedAt time.Time) DueTaskRunResult {
	nextRunAt, err := nextRunTime(result.Task.CronExpression, now)
	if err != nil {
		return s.failClaimedRunAt(ctx, dueStore, result, auth.Session{OrganizationID: result.Task.OrganizationID}, err, now, finishedAt)
	}
	completedRun, err := dueStore.CompleteScheduledTaskRun(ctx, result.Task.OrganizationID, result.Task.ID, result.Run.ID, CompleteScheduledTaskRunInput{
		FinishedAt: finishedAt,
		NextRunAt:  nextRunAt,
	})
	if err != nil {
		result.Err = err
		return result
	}
	result.Run = completedRun
	return result
}

func (s *Service) failClaimedRun(ctx context.Context, dueStore DueTaskStore, result DueTaskRunResult, session auth.Session, err error, scheduleBase time.Time) DueTaskRunResult {
	return s.failClaimedRunAt(ctx, dueStore, result, session, err, scheduleBase, time.Now().UTC())
}

func (s *Service) failClaimedRunAt(ctx context.Context, dueStore DueTaskStore, result DueTaskRunResult, session auth.Session, err error, scheduleBase, finishedAt time.Time) DueTaskRunResult {
	result.Err = err
	if dueStore != nil {
		nextRunAt, nextErr := nextRunTime(result.Task.CronExpression, scheduleBase)
		if nextErr == nil {
			if failedRun, failErr := dueStore.FailScheduledTaskRun(ctx, result.Task.OrganizationID, result.Task.ID, result.Run.ID, FailScheduledTaskRunInput{
				FinishedAt: finishedAt,
				Error:      err.Error(),
				NextRunAt:  nextRunAt,
			}); failErr == nil {
				result.Run = failedRun
				return result
			}
		}
	}
	if failedRun, updateErr := s.UpdateRun(ctx, session, result.Run.ID, UpdateScheduledTaskRunInput{
		Status:     RunStatusFailed,
		FinishedAt: &finishedAt,
		Error:      err.Error(),
	}); updateErr == nil {
		result.Run = failedRun
	}
	return result
}

func scheduledAgentInput(task ScheduledTask, run ScheduledTaskRun) string {
	return "Scheduled task " + strings.TrimSpace(task.ID) + " run " + strings.TrimSpace(run.ID)
}

func isTerminalWorkflowExecutionStatus(status workflow.ExecutionStatus) bool {
	switch status {
	case workflow.ExecutionStatusSucceeded,
		workflow.ExecutionStatusCompleted,
		workflow.ExecutionStatusPartialSuccess,
		workflow.ExecutionStatusFailed,
		workflow.ExecutionStatusCancelled,
		workflow.ExecutionStatusTimedOut,
		workflow.ExecutionStatusMaxIterations:
		return true
	default:
		return false
	}
}

func isFailedWorkflowExecutionStatus(status workflow.ExecutionStatus) bool {
	switch status {
	case workflow.ExecutionStatusFailed,
		workflow.ExecutionStatusCancelled,
		workflow.ExecutionStatusTimedOut,
		workflow.ExecutionStatusMaxIterations:
		return true
	default:
		return false
	}
}

func (s *Service) startClaimedWorkflow(ctx context.Context, task ScheduledTask, run ScheduledTaskRun, payload map[string]any) (*workflow.WorkflowExecution, error) {
	organizationID := strings.TrimSpace(task.OrganizationID)
	if organizationID == "" || strings.TrimSpace(run.OrganizationID) != organizationID {
		return nil, ErrInvalidOrganization
	}
	if strings.ToLower(strings.TrimSpace(task.TargetType)) != TargetTypeWorkflow {
		return nil, ErrInvalidTargetType
	}
	workflowID := strings.TrimSpace(task.TargetID)
	if workflowID == "" {
		return nil, ErrInvalidTargetID
	}
	taskID := strings.TrimSpace(task.ID)
	if taskID == "" || strings.TrimSpace(run.ScheduledTaskID) != taskID {
		return nil, ErrInvalidScheduledTaskID
	}
	runID := strings.TrimSpace(run.ID)
	if runID == "" {
		return nil, ErrInvalidScheduledTaskID
	}

	triggerPayload := map[string]any{
		"scheduledTaskId":    taskID,
		"scheduledTaskRunId": runID,
		"cronExpression":     strings.TrimSpace(task.CronExpression),
		"payload":            map[string]any{},
	}
	if payload != nil {
		triggerPayload["payload"] = payload
		if manual, ok := payload["manual"].(bool); ok {
			triggerPayload["manual"] = manual
		}
	}

	input := payload
	if input == nil {
		input = map[string]any{}
	}
	return s.workflowStarter.StartExecution(ctx, workflow.StartExecutionRequest{
		OrganizationID: organizationID,
		WorkflowID:     workflowID,
		TriggerType:    workflow.WorkflowTriggerSchedule,
		TriggerPayload: triggerPayload,
		Input:          input,
		Context:        map[string]any{"scheduledTaskRunId": runID},
	})
}
