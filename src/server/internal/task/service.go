package task

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"oblivious/server/internal/auth"
)

var ErrInvalidGoal = errors.New("goal is required")

type Task struct {
	AuthorizationScope string     `json:"authorizationScope"`
	BudgetConsumed     int        `json:"budgetConsumed"`
	BudgetLimit        int        `json:"budgetLimit"`
	CreatedAt          time.Time  `json:"createdAt"`
	ExecutionMode      string     `json:"executionMode"`
	FinishedAt         *time.Time `json:"finishedAt,omitempty"`
	Goal               string     `json:"goal"`
	ID                 string     `json:"id"`
	ResultSummary      string     `json:"resultSummary,omitempty"`
	StartedAt          *time.Time `json:"startedAt,omitempty"`
	Status             string     `json:"status"`
	Title              string     `json:"title"`
	UpdatedAt          time.Time  `json:"updatedAt"`
}

type TaskStep struct {
	CreatedAt  time.Time  `json:"createdAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	ID         string     `json:"id"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	Status     string     `json:"status"`
	StepIndex  int        `json:"stepIndex"`
	Title      string     `json:"title"`
	UpdatedAt  time.Time  `json:"updatedAt"`
}

type TaskDetail struct {
	Task
	KnowledgeBaseIDs []string   `json:"knowledgeBaseIds"`
	Steps            []TaskStep `json:"steps"`
}

type Store interface {
	CancelTask(ctx context.Context, workspaceID, taskID string) (TaskDetail, error)
	CreateTask(
		ctx context.Context,
		workspaceID,
		title,
		goal,
		executionMode string,
		authorizationScope string,
		budgetLimit int,
		knowledgeBaseIDs []string,
	) (Task, error)
	ApproveTask(ctx context.Context, workspaceID, taskID string) (TaskDetail, error)
	GetTask(ctx context.Context, workspaceID, taskID string) (TaskDetail, error)
	ListTasks(ctx context.Context, workspaceID string) ([]Task, error)
	PauseTask(ctx context.Context, workspaceID, taskID string) (TaskDetail, error)
	ResumeTask(ctx context.Context, workspaceID, taskID string) (TaskDetail, error)
	StartTask(ctx context.Context, workspaceID, taskID string) (TaskDetail, error)
	UpdateTaskBudget(ctx context.Context, workspaceID, taskID string, budgetLimit int) (TaskDetail, error)
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) List(ctx context.Context, session auth.Session) ([]Task, error) {
	tasks, err := s.store.ListTasks(ctx, session.WorkspaceID)
	if err != nil {
		return nil, err
	}
	if tasks == nil {
		return []Task{}, nil
	}

	for index := range tasks {
		tasks[index] = normalizeTask(tasks[index])
	}

	return tasks, nil
}

func (s *Service) Get(ctx context.Context, session auth.Session, taskID string) (TaskDetail, error) {
	detail, err := s.store.GetTask(ctx, session.WorkspaceID, strings.TrimSpace(taskID))
	if err != nil {
		return TaskDetail{}, err
	}

	return normalizeTaskDetail(detail), nil
}

func (s *Service) Create(
	ctx context.Context,
	session auth.Session,
	title,
	goal,
	executionMode string,
	authorizationScope string,
	budgetLimit int,
	knowledgeBaseIDs []string,
) (Task, error) {
	normalizedGoal := strings.TrimSpace(goal)
	if normalizedGoal == "" {
		return Task{}, ErrInvalidGoal
	}

	normalizedTitle := strings.TrimSpace(title)
	if normalizedTitle == "" {
		normalizedTitle = normalizedGoal
	}

	if budgetLimit < 0 {
		budgetLimit = 0
	}

	return s.store.CreateTask(
		ctx,
		session.WorkspaceID,
		normalizedTitle,
		normalizedGoal,
		normalizeExecutionMode(executionMode),
		normalizeAuthorizationScope(authorizationScope),
		budgetLimit,
		normalizeKnowledgeBaseIDs(knowledgeBaseIDs),
	)
}

func (s *Service) Start(ctx context.Context, session auth.Session, taskID string) (TaskDetail, error) {
	trimmedTaskID := strings.TrimSpace(taskID)
	if trimmedTaskID == "" {
		return TaskDetail{}, sql.ErrNoRows
	}

	detail, err := s.store.StartTask(ctx, session.WorkspaceID, trimmedTaskID)
	if err != nil {
		return TaskDetail{}, err
	}

	return normalizeTaskDetail(detail), nil
}

func (s *Service) Approve(ctx context.Context, session auth.Session, taskID string) (TaskDetail, error) {
	detail, err := s.store.ApproveTask(ctx, session.WorkspaceID, strings.TrimSpace(taskID))
	if err != nil {
		return TaskDetail{}, err
	}

	return normalizeTaskDetail(detail), nil
}

func (s *Service) Pause(ctx context.Context, session auth.Session, taskID string) (TaskDetail, error) {
	detail, err := s.store.PauseTask(ctx, session.WorkspaceID, strings.TrimSpace(taskID))
	if err != nil {
		return TaskDetail{}, err
	}

	return normalizeTaskDetail(detail), nil
}

func (s *Service) Resume(ctx context.Context, session auth.Session, taskID string) (TaskDetail, error) {
	detail, err := s.store.ResumeTask(ctx, session.WorkspaceID, strings.TrimSpace(taskID))
	if err != nil {
		return TaskDetail{}, err
	}

	return normalizeTaskDetail(detail), nil
}

func (s *Service) Cancel(ctx context.Context, session auth.Session, taskID string) (TaskDetail, error) {
	detail, err := s.store.CancelTask(ctx, session.WorkspaceID, strings.TrimSpace(taskID))
	if err != nil {
		return TaskDetail{}, err
	}

	return normalizeTaskDetail(detail), nil
}

func (s *Service) UpdateBudget(ctx context.Context, session auth.Session, taskID string, budgetLimit int) (TaskDetail, error) {
	trimmedTaskID := strings.TrimSpace(taskID)
	if trimmedTaskID == "" {
		return TaskDetail{}, sql.ErrNoRows
	}

	if budgetLimit < 0 {
		budgetLimit = 0
	}

	detail, err := s.store.UpdateTaskBudget(ctx, session.WorkspaceID, trimmedTaskID, budgetLimit)
	if err != nil {
		return TaskDetail{}, err
	}

	return normalizeTaskDetail(detail), nil
}

func normalizeExecutionMode(executionMode string) string {
	switch strings.TrimSpace(executionMode) {
	case "safe":
		return "safe"
	case "auto":
		return "auto"
	default:
		return "standard"
	}
}

func normalizeKnowledgeBaseIDs(ids []string) []string {
	if len(ids) == 0 {
		return []string{}
	}

	normalized := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}

		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}

	if len(normalized) == 0 {
		return []string{}
	}

	return normalized
}

func normalizeAuthorizationScope(scope string) string {
	switch strings.TrimSpace(scope) {
	case "knowledge_only":
		return "knowledge_only"
	case "full_access":
		return "full_access"
	default:
		return "workspace_tools"
	}
}

func normalizeTask(task Task) Task {
	task.AuthorizationScope = normalizeAuthorizationScope(task.AuthorizationScope)
	return task
}

func normalizeTaskDetail(detail TaskDetail) TaskDetail {
	detail.Task = normalizeTask(detail.Task)
	if detail.KnowledgeBaseIDs == nil {
		detail.KnowledgeBaseIDs = []string{}
	}
	if detail.Steps == nil {
		detail.Steps = []TaskStep{}
	}

	return detail
}
