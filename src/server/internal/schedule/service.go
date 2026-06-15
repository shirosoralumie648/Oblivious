package schedule

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"oblivious/server/internal/agent"
	"oblivious/server/internal/auth"
	"oblivious/server/internal/workflow"
)

const (
	TargetTypeWorkflow = "workflow"
	TargetTypeAgent    = "agent"

	RunStatusQueued    = "queued"
	RunStatusRunning   = "running"
	RunStatusCompleted = "completed"
	RunStatusFailed    = "failed"
	RunStatusCancelled = "cancelled"
)

var standardCronParser = cron.NewParser(
	cron.Minute |
		cron.Hour |
		cron.Dom |
		cron.Month |
		cron.Dow |
		cron.Descriptor,
)

var (
	ErrDueTaskStoreUnavailable  = errors.New("schedule due task store is unavailable")
	ErrInvalidCronExpression    = errors.New("cron expression is required")
	ErrInvalidOrganization      = errors.New("organization is required")
	ErrInvalidRunStatus         = errors.New("run status must be queued, running, completed, failed, or cancelled")
	ErrInvalidScheduledTaskName = errors.New("scheduled task name is required")
	ErrInvalidScheduledTaskID   = errors.New("scheduled task id is required")
	ErrInvalidTargetID          = errors.New("target id is required")
	ErrInvalidTargetType        = errors.New("target type must be workflow or agent")
)

type ScheduledTask struct {
	ID                string     `json:"id"`
	OrganizationID    string     `json:"organizationId"`
	Name              string     `json:"name"`
	TargetType        string     `json:"targetType"`
	TargetID          string     `json:"targetId"`
	WorkflowTriggerID string     `json:"workflowTriggerId,omitempty"`
	CronExpression    string     `json:"cronExpression"`
	Enabled           bool       `json:"enabled"`
	LastRunAt         *time.Time `json:"lastRunAt,omitempty"`
	NextRunAt         *time.Time `json:"nextRunAt,omitempty"`
	CreatedAt         time.Time  `json:"createdAt"`
	UpdatedAt         time.Time  `json:"updatedAt"`
}

type ScheduledTaskRun struct {
	ID              string     `json:"id"`
	OrganizationID  string     `json:"organizationId"`
	ScheduledTaskID string     `json:"scheduledTaskId"`
	Status          string     `json:"status"`
	StartedAt       *time.Time `json:"startedAt,omitempty"`
	FinishedAt      *time.Time `json:"finishedAt,omitempty"`
	Error           string     `json:"error,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

type CreateScheduledTaskInput struct {
	OrganizationID string
	Name           string
	TargetType     string
	TargetID       string
	CronExpression string
	Enabled        bool
	Now            time.Time
	NextRunAt      *time.Time
}

type SyncWorkflowScheduledTasksInput struct {
	OrganizationID string
	WorkflowID     string
	Triggers       []SyncWorkflowScheduledTaskTriggerInput
	Now            time.Time
}

type SyncWorkflowScheduledTaskTriggerInput struct {
	TriggerID      string
	Name           string
	CronExpression string
	Enabled        bool
	NextRunAt      *time.Time
}

type RecordScheduledTaskRunInput struct {
	OrganizationID  string
	ScheduledTaskID string
	Status          string
	StartedAt       *time.Time
	FinishedAt      *time.Time
	Error           string
}

type UpdateScheduledTaskEnabledInput struct {
	Enabled   bool
	NextRunAt *time.Time
}

type UpdateScheduledTaskRunInput struct {
	Status     string
	FinishedAt *time.Time
	Error      string
}

type ClaimDueScheduledTaskRunsInput struct {
	Now   time.Time
	Limit int
}

type ClaimedScheduledTaskRun struct {
	Task ScheduledTask
	Run  ScheduledTaskRun
}

type CompleteScheduledTaskRunInput struct {
	FinishedAt time.Time
	NextRunAt  time.Time
}

type FailScheduledTaskRunInput struct {
	FinishedAt time.Time
	Error      string
	NextRunAt  time.Time
}

type Store interface {
	CreateScheduledTask(ctx context.Context, input CreateScheduledTaskInput) (ScheduledTask, error)
	SyncWorkflowScheduledTasks(ctx context.Context, input SyncWorkflowScheduledTasksInput) ([]ScheduledTask, error)
	ListScheduledTasks(ctx context.Context, organizationID string) ([]ScheduledTask, error)
	GetScheduledTask(ctx context.Context, organizationID string, scheduledTaskID string) (ScheduledTask, error)
	UpdateScheduledTaskEnabled(ctx context.Context, organizationID string, scheduledTaskID string, input UpdateScheduledTaskEnabledInput) (ScheduledTask, error)
	RecordScheduledTaskRun(ctx context.Context, input RecordScheduledTaskRunInput) (ScheduledTaskRun, error)
	CompleteManualScheduledTaskRun(ctx context.Context, organizationID string, scheduledTaskID string, scheduledTaskRunID string, finishedAt time.Time) (ScheduledTaskRun, error)
	UpdateScheduledTaskRun(ctx context.Context, organizationID string, scheduledTaskRunID string, input UpdateScheduledTaskRunInput) (ScheduledTaskRun, error)
	ListScheduledTaskRuns(ctx context.Context, organizationID string, scheduledTaskID string) ([]ScheduledTaskRun, error)
	CountRunningScheduledTaskRuns(ctx context.Context, organizationID string, scheduledTaskID string) (int, error)
}

type DueTaskStore interface {
	ClaimDueScheduledTaskRuns(ctx context.Context, input ClaimDueScheduledTaskRunsInput) ([]ClaimedScheduledTaskRun, error)
	CompleteScheduledTaskRun(ctx context.Context, organizationID string, scheduledTaskID string, scheduledTaskRunID string, input CompleteScheduledTaskRunInput) (ScheduledTaskRun, error)
	FailScheduledTaskRun(ctx context.Context, organizationID string, scheduledTaskID string, scheduledTaskRunID string, input FailScheduledTaskRunInput) (ScheduledTaskRun, error)
}

type WorkflowStarter interface {
	StartExecution(ctx context.Context, req workflow.StartExecutionRequest) (*workflow.WorkflowExecution, error)
	RunExecutionUntilBlocked(ctx context.Context, organizationID, executionID string) (*workflow.WorkflowExecution, error)
}

type AgentStarter interface {
	StartRun(ctx context.Context, session auth.Session, req agent.StartRunRequest) (*agent.RunWithMessages, error)
}

type Service struct {
	store           Store
	workflowStarter WorkflowStarter
	agentStarter    AgentStarter
}

type ServiceOption func(*Service)

func WithWorkflowStarter(starter WorkflowStarter) ServiceOption {
	return func(service *Service) {
		service.workflowStarter = starter
	}
}

func WithAgentStarter(starter AgentStarter) ServiceOption {
	return func(service *Service) {
		service.agentStarter = starter
	}
}

func NewService(store Store, options ...ServiceOption) *Service {
	service := &Service{store: store}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}

func (s *Service) Create(ctx context.Context, session auth.Session, input CreateScheduledTaskInput) (ScheduledTask, error) {
	normalized, err := normalizeCreateInput(session.OrganizationID, input)
	if err != nil {
		return ScheduledTask{}, err
	}
	return s.store.CreateScheduledTask(ctx, normalized)
}

func (s *Service) SyncWorkflowScheduleTriggers(ctx context.Context, req workflow.WorkflowScheduleSyncRequest) error {
	normalized, err := normalizeWorkflowScheduleSyncInput(req)
	if err != nil {
		return err
	}
	_, err = s.store.SyncWorkflowScheduledTasks(ctx, normalized)
	return err
}

func (s *Service) List(ctx context.Context, session auth.Session) ([]ScheduledTask, error) {
	organizationID := strings.TrimSpace(session.OrganizationID)
	if organizationID == "" {
		return nil, ErrInvalidOrganization
	}

	tasks, err := s.store.ListScheduledTasks(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	if tasks == nil {
		return []ScheduledTask{}, nil
	}
	return tasks, nil
}

func (s *Service) UpdateEnabled(ctx context.Context, session auth.Session, scheduledTaskID string, enabled bool, now time.Time) (ScheduledTask, error) {
	organizationID := strings.TrimSpace(session.OrganizationID)
	if organizationID == "" {
		return ScheduledTask{}, ErrInvalidOrganization
	}
	taskID := strings.TrimSpace(scheduledTaskID)
	if taskID == "" {
		return ScheduledTask{}, ErrInvalidScheduledTaskID
	}

	task, err := s.store.GetScheduledTask(ctx, organizationID, taskID)
	if err != nil {
		return ScheduledTask{}, err
	}

	var nextRunAt *time.Time
	if enabled {
		next, err := nextRunTime(task.CronExpression, now)
		if err != nil {
			return ScheduledTask{}, ErrInvalidCronExpression
		}
		nextRunAt = &next
	}

	return s.store.UpdateScheduledTaskEnabled(ctx, organizationID, taskID, UpdateScheduledTaskEnabledInput{
		Enabled:   enabled,
		NextRunAt: nextRunAt,
	})
}

func (s *Service) RecordRun(ctx context.Context, session auth.Session, input RecordScheduledTaskRunInput) (ScheduledTaskRun, error) {
	normalized, err := normalizeRecordRunInput(session.OrganizationID, input)
	if err != nil {
		return ScheduledTaskRun{}, err
	}
	if normalized.Status == RunStatusRunning {
		running, err := s.store.CountRunningScheduledTaskRuns(ctx, normalized.OrganizationID, normalized.ScheduledTaskID)
		if err != nil {
			return ScheduledTaskRun{}, err
		}
		if running > 0 {
			normalized.Status = RunStatusQueued
			normalized.StartedAt = nil
		}
	}
	return s.store.RecordScheduledTaskRun(ctx, normalized)
}

func (s *Service) UpdateRun(ctx context.Context, session auth.Session, scheduledTaskRunID string, input UpdateScheduledTaskRunInput) (ScheduledTaskRun, error) {
	organizationID := strings.TrimSpace(session.OrganizationID)
	if organizationID == "" {
		return ScheduledTaskRun{}, ErrInvalidOrganization
	}
	runID := strings.TrimSpace(scheduledTaskRunID)
	if runID == "" {
		return ScheduledTaskRun{}, ErrInvalidScheduledTaskID
	}
	normalized, err := normalizeUpdateRunInput(input)
	if err != nil {
		return ScheduledTaskRun{}, err
	}
	return s.store.UpdateScheduledTaskRun(ctx, organizationID, runID, normalized)
}

func (s *Service) RunNow(ctx context.Context, session auth.Session, scheduledTaskID string) (DueTaskRunResult, error) {
	organizationID := strings.TrimSpace(session.OrganizationID)
	if organizationID == "" {
		return DueTaskRunResult{}, ErrInvalidOrganization
	}
	taskID := strings.TrimSpace(scheduledTaskID)
	if taskID == "" {
		return DueTaskRunResult{}, ErrInvalidScheduledTaskID
	}

	task, err := s.store.GetScheduledTask(ctx, organizationID, taskID)
	if err != nil {
		return DueTaskRunResult{}, err
	}
	startedAt := time.Now().UTC()
	run, err := s.RecordRun(ctx, session, RecordScheduledTaskRunInput{
		ScheduledTaskID: taskID,
		Status:          RunStatusRunning,
		StartedAt:       &startedAt,
	})
	if err != nil {
		return DueTaskRunResult{Task: task}, err
	}

	result := DueTaskRunResult{Task: task, Run: run}
	if run.Status == RunStatusQueued {
		return result, nil
	}

	targetType := strings.ToLower(strings.TrimSpace(task.TargetType))
	if targetType == TargetTypeAgent {
		if s.agentStarter == nil {
			return s.failManualRun(ctx, result, session, errors.New("agent starter is not configured")), errors.New("agent starter is not configured")
		}
		agentRun, err := s.agentStarter.StartRun(ctx, session, agent.StartRunRequest{
			AgentID:        strings.TrimSpace(task.TargetID),
			ConversationID: strings.TrimSpace(run.ID),
			Input:          scheduledAgentInput(task, run),
		})
		result.AgentRun = agentRun
		if err != nil {
			failed := s.failManualRun(ctx, result, session, err)
			return failed, err
		}
		return s.completeManualRun(ctx, result), nil
	}
	if targetType != TargetTypeWorkflow {
		return result, ErrInvalidTargetType
	}
	if s.workflowStarter == nil {
		return s.failManualRun(ctx, result, session, errors.New("workflow starter is not configured")), workflow.ErrInvalidInput
	}

	execution, err := s.startClaimedWorkflow(ctx, task, run, map[string]any{"manual": true})
	result.Execution = execution
	if err != nil {
		failed := s.failManualRun(ctx, result, session, err)
		return failed, err
	}
	completed := s.completeManualRun(ctx, result)
	if completed.Err != nil {
		return completed, completed.Err
	}
	return completed, nil
}

func (s *Service) TriggerWorkflow(ctx context.Context, session auth.Session, task ScheduledTask, payload map[string]any) (*workflow.WorkflowExecution, error) {
	if s.workflowStarter == nil {
		return nil, workflow.ErrInvalidInput
	}

	organizationID := strings.TrimSpace(session.OrganizationID)
	if organizationID == "" {
		return nil, ErrInvalidOrganization
	}
	if strings.TrimSpace(task.OrganizationID) != organizationID {
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
	if taskID == "" {
		return nil, ErrInvalidScheduledTaskID
	}

	now := time.Now().UTC()
	run, err := s.RecordRun(ctx, session, RecordScheduledTaskRunInput{
		ScheduledTaskID: taskID,
		Status:          RunStatusRunning,
		StartedAt:       &now,
	})
	if err != nil {
		return nil, err
	}

	execution, err := s.startClaimedWorkflow(ctx, ScheduledTask{
		ID:             taskID,
		OrganizationID: organizationID,
		TargetType:     TargetTypeWorkflow,
		TargetID:       workflowID,
		CronExpression: task.CronExpression,
	}, run, payload)
	finishedAt := time.Now().UTC()
	if err != nil {
		_, _ = s.UpdateRun(ctx, session, run.ID, UpdateScheduledTaskRunInput{
			Status:     RunStatusFailed,
			FinishedAt: &finishedAt,
			Error:      err.Error(),
		})
		return nil, err
	}
	_, _ = s.UpdateRun(ctx, session, run.ID, UpdateScheduledTaskRunInput{
		Status:     RunStatusCompleted,
		FinishedAt: &finishedAt,
	})
	return execution, nil
}

func (s *Service) completeManualRun(ctx context.Context, result DueTaskRunResult) DueTaskRunResult {
	finishedAt := time.Now().UTC()
	completedRun, err := s.store.CompleteManualScheduledTaskRun(ctx, result.Task.OrganizationID, result.Task.ID, result.Run.ID, finishedAt)
	if err != nil {
		result.Err = err
		return result
	}
	result.Run = completedRun
	return result
}

func (s *Service) failManualRun(ctx context.Context, result DueTaskRunResult, session auth.Session, err error) DueTaskRunResult {
	finishedAt := time.Now().UTC()
	result.Err = err
	if failedRun, updateErr := s.UpdateRun(ctx, session, result.Run.ID, UpdateScheduledTaskRunInput{
		Status:     RunStatusFailed,
		FinishedAt: &finishedAt,
		Error:      err.Error(),
	}); updateErr == nil {
		result.Run = failedRun
	}
	return result
}

func (s *Service) ListRuns(ctx context.Context, session auth.Session, scheduledTaskID string) ([]ScheduledTaskRun, error) {
	organizationID := strings.TrimSpace(session.OrganizationID)
	if organizationID == "" {
		return nil, ErrInvalidOrganization
	}

	taskID := strings.TrimSpace(scheduledTaskID)
	if taskID == "" {
		return nil, ErrInvalidScheduledTaskID
	}
	if _, err := s.store.GetScheduledTask(ctx, organizationID, taskID); err != nil {
		return nil, err
	}

	runs, err := s.store.ListScheduledTaskRuns(ctx, organizationID, taskID)
	if err != nil {
		return nil, err
	}
	if runs == nil {
		return []ScheduledTaskRun{}, nil
	}
	return runs, nil
}

func normalizeCreateInput(organizationID string, input CreateScheduledTaskInput) (CreateScheduledTaskInput, error) {
	normalized := CreateScheduledTaskInput{
		OrganizationID: strings.TrimSpace(organizationID),
		Name:           strings.TrimSpace(input.Name),
		TargetType:     strings.ToLower(strings.TrimSpace(input.TargetType)),
		TargetID:       strings.TrimSpace(input.TargetID),
		CronExpression: strings.TrimSpace(input.CronExpression),
		Enabled:        input.Enabled,
	}

	if normalized.OrganizationID == "" {
		return CreateScheduledTaskInput{}, ErrInvalidOrganization
	}
	if normalized.Name == "" {
		return CreateScheduledTaskInput{}, ErrInvalidScheduledTaskName
	}
	switch normalized.TargetType {
	case TargetTypeWorkflow, TargetTypeAgent:
	default:
		return CreateScheduledTaskInput{}, ErrInvalidTargetType
	}
	if normalized.TargetID == "" {
		return CreateScheduledTaskInput{}, ErrInvalidTargetID
	}
	if !isValidCronExpression(normalized.CronExpression) {
		return CreateScheduledTaskInput{}, ErrInvalidCronExpression
	}
	if normalized.Enabled {
		nextRunAt, err := nextRunTime(normalized.CronExpression, input.Now)
		if err != nil {
			return CreateScheduledTaskInput{}, ErrInvalidCronExpression
		}
		normalized.NextRunAt = &nextRunAt
	}

	return normalized, nil
}

func normalizeWorkflowScheduleSyncInput(req workflow.WorkflowScheduleSyncRequest) (SyncWorkflowScheduledTasksInput, error) {
	normalized := SyncWorkflowScheduledTasksInput{
		OrganizationID: strings.TrimSpace(req.OrganizationID),
		WorkflowID:     strings.TrimSpace(req.WorkflowID),
		Now:            req.Now,
		Triggers:       make([]SyncWorkflowScheduledTaskTriggerInput, 0, len(req.Triggers)),
	}
	if normalized.OrganizationID == "" {
		return SyncWorkflowScheduledTasksInput{}, ErrInvalidOrganization
	}
	if normalized.WorkflowID == "" {
		return SyncWorkflowScheduledTasksInput{}, ErrInvalidTargetID
	}
	for _, trigger := range req.Triggers {
		item := SyncWorkflowScheduledTaskTriggerInput{
			TriggerID:      strings.TrimSpace(trigger.ID),
			Name:           strings.TrimSpace(trigger.Name),
			CronExpression: strings.TrimSpace(trigger.CronExpression),
			Enabled:        trigger.Enabled,
		}
		if item.TriggerID == "" {
			return SyncWorkflowScheduledTasksInput{}, ErrInvalidScheduledTaskID
		}
		if item.Name == "" {
			item.Name = item.TriggerID
		}
		if !isValidCronExpression(item.CronExpression) {
			return SyncWorkflowScheduledTasksInput{}, ErrInvalidCronExpression
		}
		if item.Enabled {
			nextRunAt, err := nextRunTime(item.CronExpression, normalized.Now)
			if err != nil {
				return SyncWorkflowScheduledTasksInput{}, ErrInvalidCronExpression
			}
			item.NextRunAt = &nextRunAt
		}
		normalized.Triggers = append(normalized.Triggers, item)
	}
	return normalized, nil
}

func normalizeRecordRunInput(organizationID string, input RecordScheduledTaskRunInput) (RecordScheduledTaskRunInput, error) {
	normalized := RecordScheduledTaskRunInput{
		OrganizationID:  strings.TrimSpace(organizationID),
		ScheduledTaskID: strings.TrimSpace(input.ScheduledTaskID),
		Status:          strings.ToLower(strings.TrimSpace(input.Status)),
		StartedAt:       input.StartedAt,
		FinishedAt:      input.FinishedAt,
		Error:           strings.TrimSpace(input.Error),
	}

	if normalized.OrganizationID == "" {
		return RecordScheduledTaskRunInput{}, ErrInvalidOrganization
	}
	if normalized.ScheduledTaskID == "" {
		return RecordScheduledTaskRunInput{}, ErrInvalidScheduledTaskID
	}
	if !validRunStatus(normalized.Status) {
		return RecordScheduledTaskRunInput{}, ErrInvalidRunStatus
	}

	return normalized, nil
}

func normalizeUpdateRunInput(input UpdateScheduledTaskRunInput) (UpdateScheduledTaskRunInput, error) {
	normalized := UpdateScheduledTaskRunInput{
		Status:     strings.ToLower(strings.TrimSpace(input.Status)),
		FinishedAt: input.FinishedAt,
		Error:      strings.TrimSpace(input.Error),
	}
	if !validRunStatus(normalized.Status) {
		return UpdateScheduledTaskRunInput{}, ErrInvalidRunStatus
	}
	return normalized, nil
}

func validRunStatus(status string) bool {
	switch status {
	case RunStatusQueued, RunStatusRunning, RunStatusCompleted, RunStatusFailed, RunStatusCancelled:
		return true
	default:
		return false
	}
}

func isValidCronExpression(expression string) bool {
	if strings.TrimSpace(expression) == "" {
		return false
	}
	_, err := standardCronParser.Parse(expression)
	return err == nil
}

func nextRunTime(expression string, now time.Time) (time.Time, error) {
	schedule, err := standardCronParser.Parse(expression)
	if err != nil {
		return time.Time{}, err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return schedule.Next(now.UTC()), nil
}
