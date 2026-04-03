package http

import (
	"context"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/task"
)

type taskFakeStore struct {
	approvedTaskID       string
	createdAuthorization string
	cancelledTaskID      string
	createdBudgetLimit   int
	createdExecutionMode string
	createdGoal          string
	createdKnowledgeIDs  []string
	createdTask          task.Task
	detailTask           task.TaskDetail
	listedTasks          []task.Task
	pausedTaskID         string
	requestedID          string
	resumedTaskID        string
	updatedBudgetLimit   int
	updatedBudgetTaskID  string
	workspaceID          string
}

func (f *taskFakeStore) ListTasks(ctx context.Context, workspaceID string) ([]task.Task, error) {
	f.workspaceID = workspaceID
	return f.listedTasks, nil
}

func (f *taskFakeStore) GetTask(ctx context.Context, workspaceID, taskID string) (task.TaskDetail, error) {
	f.workspaceID = workspaceID
	f.requestedID = taskID
	return f.detailTask, nil
}

func (f *taskFakeStore) CreateTask(
	ctx context.Context,
	workspaceID,
	title,
	goal,
	executionMode string,
	authorizationScope string,
	budgetLimit int,
	knowledgeBaseIDs []string,
) (task.Task, error) {
	f.workspaceID = workspaceID
	f.createdGoal = goal
	f.createdExecutionMode = executionMode
	f.createdAuthorization = authorizationScope
	f.createdBudgetLimit = budgetLimit
	f.createdKnowledgeIDs = append([]string(nil), knowledgeBaseIDs...)
	return f.createdTask, nil
}

func (f *taskFakeStore) StartTask(ctx context.Context, workspaceID, taskID string) (task.TaskDetail, error) {
	f.workspaceID = workspaceID
	f.requestedID = taskID
	return f.detailTask, nil
}

func (f *taskFakeStore) ApproveTask(ctx context.Context, workspaceID, taskID string) (task.TaskDetail, error) {
	f.workspaceID = workspaceID
	f.approvedTaskID = taskID
	return f.detailTask, nil
}

func (f *taskFakeStore) PauseTask(ctx context.Context, workspaceID, taskID string) (task.TaskDetail, error) {
	f.workspaceID = workspaceID
	f.pausedTaskID = taskID
	return f.detailTask, nil
}

func (f *taskFakeStore) ResumeTask(ctx context.Context, workspaceID, taskID string) (task.TaskDetail, error) {
	f.workspaceID = workspaceID
	f.resumedTaskID = taskID
	return f.detailTask, nil
}

func (f *taskFakeStore) CancelTask(ctx context.Context, workspaceID, taskID string) (task.TaskDetail, error) {
	f.workspaceID = workspaceID
	f.cancelledTaskID = taskID
	return f.detailTask, nil
}

func (f *taskFakeStore) UpdateTaskBudget(ctx context.Context, workspaceID, taskID string, budgetLimit int) (task.TaskDetail, error) {
	f.workspaceID = workspaceID
	f.updatedBudgetTaskID = taskID
	f.updatedBudgetLimit = budgetLimit
	return f.detailTask, nil
}

func TestTaskHandlerListReturnsWorkspaceTasks(t *testing.T) {
	store := &taskFakeStore{
		listedTasks: []task.Task{
			{
				ExecutionMode: "standard",
				Goal:          "Review launch plan",
				ID:            "task_1",
				Status:        "completed",
				Title:         "Review launch plan",
				UpdatedAt:     time.Date(2026, time.April, 3, 18, 30, 0, 0, time.UTC),
			},
		},
	}
	handler := newTaskHandler(task.NewService(store))
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/tasks", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.listTasks(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if store.workspaceID != "workspace_1" {
		t.Fatalf("expected workspace workspace_1, got %s", store.workspaceID)
	}
}

func TestTaskHandlerCreateTaskAcceptsKnowledgeBaseIDs(t *testing.T) {
	store := &taskFakeStore{
		createdTask: task.Task{
			AuthorizationScope: "full_access",
			BudgetLimit:        25,
			ExecutionMode:      "safe",
			Goal:               "Draft onboarding checklist",
			ID:                 "task_1",
			Status:             "draft",
			Title:              "Draft onboarding checklist",
		},
	}
	handler := newTaskHandler(task.NewService(store))
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/tasks", strings.NewReader(`{"goal":"Draft onboarding checklist","executionMode":"safe","authorizationScope":"full_access","budgetLimit":25,"knowledgeBaseIds":["kb_1","kb_3"]}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.createTask(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.createdExecutionMode != "safe" || store.createdBudgetLimit != 25 {
		t.Fatalf("unexpected create args: mode=%s budget=%d", store.createdExecutionMode, store.createdBudgetLimit)
	}
	if store.createdAuthorization != "full_access" {
		t.Fatalf("unexpected authorization scope: %s", store.createdAuthorization)
	}
	if len(store.createdKnowledgeIDs) != 2 || store.createdKnowledgeIDs[0] != "kb_1" || store.createdKnowledgeIDs[1] != "kb_3" {
		t.Fatalf("unexpected knowledge ids: %+v", store.createdKnowledgeIDs)
	}
}

func TestTaskHandlerGetTaskReturnsTaskDetail(t *testing.T) {
	startedAt := time.Date(2026, time.April, 3, 18, 0, 0, 0, time.UTC)
	finishedAt := time.Date(2026, time.April, 3, 18, 30, 0, 0, time.UTC)
	store := &taskFakeStore{
		detailTask: task.TaskDetail{
			Task: task.Task{
				AuthorizationScope: "workspace_tools",
				BudgetConsumed:     12,
				BudgetLimit:        12,
				ExecutionMode:      "standard",
				FinishedAt:         &finishedAt,
				Goal:               "Review launch plan",
				ID:                 "task_1",
				ResultSummary:      "Completed a starter SOLO run for: Review launch plan",
				StartedAt:          &startedAt,
				Status:             "completed",
				Title:              "Review launch plan",
			},
			KnowledgeBaseIDs: []string{"kb_2"},
			Steps: []task.TaskStep{
				{ID: "step_1", Status: "completed", StepIndex: 1, Title: "Understand the goal"},
			},
		},
	}
	handler := newTaskHandler(task.NewService(store))
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/tasks/task_1", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.getTask(recorder, request, "task_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data task.TaskDetail `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if store.requestedID != "task_1" {
		t.Fatalf("expected task id task_1, got %s", store.requestedID)
	}
	if response.Data.Status != "completed" || len(response.Data.KnowledgeBaseIDs) != 1 {
		t.Fatalf("unexpected task detail: %+v", response.Data)
	}
	if response.Data.BudgetConsumed != 12 || response.Data.StartedAt == nil || response.Data.FinishedAt == nil {
		t.Fatalf("expected budget/timing fields in response, got %+v", response.Data)
	}
}

func TestTaskHandlerStartReturnsTaskDetail(t *testing.T) {
	store := &taskFakeStore{
		detailTask: task.TaskDetail{
			Task: task.Task{
				AuthorizationScope: "workspace_tools",
				ExecutionMode:      "standard",
				Goal:               "Review launch plan",
				ID:                 "task_1",
				Status:             "running",
				Title:              "Review launch plan",
			},
			KnowledgeBaseIDs: []string{"kb_2"},
			Steps: []task.TaskStep{
				{ID: "step_1", Status: "completed", StepIndex: 1, Title: "Understand the goal"},
				{ID: "step_2", Status: "running", StepIndex: 2, Title: "Review workspace context"},
			},
		},
	}
	handler := newTaskHandler(task.NewService(store))
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/tasks/task_1/start", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.startTask(recorder, request, "task_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data task.TaskDetail `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if store.requestedID != "task_1" {
		t.Fatalf("expected task id task_1, got %s", store.requestedID)
	}
	if response.Data.Status != "running" || len(response.Data.Steps) != 2 {
		t.Fatalf("unexpected task detail: %+v", response.Data)
	}
}

func TestTaskHandlerApproveReturnsTaskDetail(t *testing.T) {
	store := &taskFakeStore{
		detailTask: task.TaskDetail{
			Task: task.Task{
				AuthorizationScope: "full_access",
				ExecutionMode:      "safe",
				Goal:               "Review launch plan",
				ID:                 "task_1",
				Status:             "running",
				Title:              "Review launch plan",
			},
			KnowledgeBaseIDs: []string{"kb_2"},
			Steps: []task.TaskStep{
				{ID: "step_1", Status: "completed", StepIndex: 1, Title: "Understand the goal"},
				{ID: "step_2", Status: "completed", StepIndex: 2, Title: "Confirm execution boundary"},
				{ID: "step_3", Status: "running", StepIndex: 3, Title: "Deliver starter result"},
			},
		},
	}
	handler := newTaskHandler(task.NewService(store))
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/tasks/task_1/approve", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.approveTask(recorder, request, "task_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data task.TaskDetail `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if store.approvedTaskID != "task_1" {
		t.Fatalf("expected task id task_1, got %s", store.approvedTaskID)
	}
	if response.Data.Status != "running" || response.Data.AuthorizationScope != "full_access" {
		t.Fatalf("unexpected approved task detail: %+v", response.Data)
	}
}

func TestTaskHandlerPauseReturnsTaskDetail(t *testing.T) {
	store := &taskFakeStore{
		detailTask: task.TaskDetail{
			Task: task.Task{
				ExecutionMode: "standard",
				Goal:          "Review launch plan",
				ID:            "task_1",
				Status:        "paused",
				Title:         "Review launch plan",
			},
		},
	}
	handler := newTaskHandler(task.NewService(store))
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/tasks/task_1/pause", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.pauseTask(recorder, request, "task_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.pausedTaskID != "task_1" {
		t.Fatalf("expected paused task id task_1, got %s", store.pausedTaskID)
	}
}

func TestTaskHandlerUpdateBudgetReturnsTaskDetail(t *testing.T) {
	store := &taskFakeStore{
		detailTask: task.TaskDetail{
			Task: task.Task{
				AuthorizationScope: "workspace_tools",
				BudgetConsumed:     4,
				BudgetLimit:        30,
				ExecutionMode:      "standard",
				Goal:               "Review launch plan",
				ID:                 "task_1",
				Status:             "running",
				Title:              "Review launch plan",
			},
		},
	}
	handler := newTaskHandler(task.NewService(store))
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/tasks/task_1/budget", strings.NewReader(`{"budgetLimit":30}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.updateTaskBudget(recorder, request, "task_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data task.TaskDetail `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if store.updatedBudgetTaskID != "task_1" || store.updatedBudgetLimit != 30 {
		t.Fatalf("unexpected budget update args: task=%s budget=%d", store.updatedBudgetTaskID, store.updatedBudgetLimit)
	}
	if response.Data.BudgetLimit != 30 || response.Data.Status != "running" {
		t.Fatalf("unexpected updated task detail: %+v", response.Data)
	}
}

func TestTaskHandlerResumeReturnsTaskDetail(t *testing.T) {
	store := &taskFakeStore{
		detailTask: task.TaskDetail{
			Task: task.Task{
				ExecutionMode: "standard",
				Goal:          "Review launch plan",
				ID:            "task_1",
				ResultSummary: "Completed a starter SOLO run for: Review launch plan",
				Status:        "completed",
				Title:         "Review launch plan",
			},
		},
	}
	handler := newTaskHandler(task.NewService(store))
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/tasks/task_1/resume", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.resumeTask(recorder, request, "task_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.resumedTaskID != "task_1" {
		t.Fatalf("expected resumed task id task_1, got %s", store.resumedTaskID)
	}
}

func TestTaskHandlerCancelReturnsTaskDetail(t *testing.T) {
	store := &taskFakeStore{
		detailTask: task.TaskDetail{
			Task: task.Task{
				ExecutionMode: "standard",
				Goal:          "Review launch plan",
				ID:            "task_1",
				Status:        "cancelled",
				Title:         "Review launch plan",
			},
		},
	}
	handler := newTaskHandler(task.NewService(store))
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/tasks/task_1/cancel", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.cancelTask(recorder, request, "task_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.cancelledTaskID != "task_1" {
		t.Fatalf("expected cancelled task id task_1, got %s", store.cancelledTaskID)
	}
}
