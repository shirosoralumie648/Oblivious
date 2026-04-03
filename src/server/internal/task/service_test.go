package task

import (
	"context"
	"testing"
	"time"

	"oblivious/server/internal/auth"
)

type fakeStore struct {
	cancelledTaskID      string
	createdExecutionMode string
	createdGoal          string
	createdKnowledgeIDs  []string
	createdTask          Task
	detailTask           TaskDetail
	listedTasks          []Task
	pausedTaskID         string
	requestedID          string
	resumedTaskID        string
	workspaceID          string
}

func (f *fakeStore) ListTasks(ctx context.Context, workspaceID string) ([]Task, error) {
	f.workspaceID = workspaceID
	return f.listedTasks, nil
}

func (f *fakeStore) GetTask(ctx context.Context, workspaceID, taskID string) (TaskDetail, error) {
	f.workspaceID = workspaceID
	f.requestedID = taskID
	return f.detailTask, nil
}

func (f *fakeStore) CreateTask(
	ctx context.Context,
	workspaceID,
	title,
	goal,
	executionMode string,
	budgetLimit int,
	knowledgeBaseIDs []string,
) (Task, error) {
	f.workspaceID = workspaceID
	f.createdGoal = goal
	f.createdExecutionMode = executionMode
	f.createdKnowledgeIDs = append([]string(nil), knowledgeBaseIDs...)
	return f.createdTask, nil
}

func (f *fakeStore) StartTask(ctx context.Context, workspaceID, taskID string) (TaskDetail, error) {
	f.workspaceID = workspaceID
	f.requestedID = taskID
	return f.detailTask, nil
}

func (f *fakeStore) PauseTask(ctx context.Context, workspaceID, taskID string) (TaskDetail, error) {
	f.workspaceID = workspaceID
	f.pausedTaskID = taskID
	return f.detailTask, nil
}

func (f *fakeStore) ResumeTask(ctx context.Context, workspaceID, taskID string) (TaskDetail, error) {
	f.workspaceID = workspaceID
	f.resumedTaskID = taskID
	return f.detailTask, nil
}

func (f *fakeStore) CancelTask(ctx context.Context, workspaceID, taskID string) (TaskDetail, error) {
	f.workspaceID = workspaceID
	f.cancelledTaskID = taskID
	return f.detailTask, nil
}

func TestListReturnsWorkspaceTasks(t *testing.T) {
	store := &fakeStore{
		listedTasks: []Task{
			{
				ExecutionMode: "standard",
				Goal:          "Research model providers",
				ID:            "task_1",
				Status:        "completed",
				Title:         "Research model providers",
				UpdatedAt:     time.Date(2026, time.April, 3, 18, 0, 0, 0, time.UTC),
			},
		},
	}
	service := NewService(store)

	tasks, err := service.List(context.Background(), auth.Session{WorkspaceID: "workspace_1"})
	if err != nil {
		t.Fatalf("list tasks: %v", err)
	}

	if store.workspaceID != "workspace_1" {
		t.Fatalf("expected workspace workspace_1, got %s", store.workspaceID)
	}
	if len(tasks) != 1 || tasks[0].ID != "task_1" {
		t.Fatalf("unexpected tasks: %+v", tasks)
	}
}

func TestCreateNormalizesGoalAndKnowledgeBaseIDs(t *testing.T) {
	store := &fakeStore{
		createdTask: Task{
			ExecutionMode: "standard",
			Goal:          "Summarize the roadmap",
			ID:            "task_1",
			Status:        "draft",
			Title:         "Summarize the roadmap",
		},
	}
	service := NewService(store)

	task, err := service.Create(
		context.Background(),
		auth.Session{WorkspaceID: "workspace_1"},
		"",
		"  Summarize the roadmap  ",
		"",
		0,
		[]string{"kb_1", " ", "kb_1", "kb_2"},
	)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	if store.createdGoal != "Summarize the roadmap" {
		t.Fatalf("expected trimmed goal, got %q", store.createdGoal)
	}
	if store.createdExecutionMode != "standard" {
		t.Fatalf("expected default execution mode standard, got %q", store.createdExecutionMode)
	}
	if len(store.createdKnowledgeIDs) != 2 || store.createdKnowledgeIDs[0] != "kb_1" || store.createdKnowledgeIDs[1] != "kb_2" {
		t.Fatalf("expected normalized knowledge ids [kb_1 kb_2], got %+v", store.createdKnowledgeIDs)
	}
	if task.Status != "draft" {
		t.Fatalf("expected draft task, got %+v", task)
	}
}

func TestStartReturnsTaskDetailForWorkspace(t *testing.T) {
	startedAt := time.Date(2026, time.April, 3, 19, 0, 0, 0, time.UTC)
	store := &fakeStore{
		detailTask: TaskDetail{
			Task: Task{
				BudgetConsumed: 6,
				ExecutionMode:  "safe",
				Goal:           "Compile launch notes",
				ID:             "task_9",
				StartedAt:      &startedAt,
				Status:         "running",
				Title:          "Compile launch notes",
			},
			KnowledgeBaseIDs: []string{"kb_1"},
			Steps: []TaskStep{
				{ID: "step_1", Status: "completed", StepIndex: 1, Title: "Understand the goal"},
				{ID: "step_2", Status: "running", StepIndex: 2, Title: "Review workspace context"},
				{ID: "step_3", Status: "pending", StepIndex: 3, Title: "Deliver starter result"},
			},
		},
	}
	service := NewService(store)

	task, err := service.Start(context.Background(), auth.Session{WorkspaceID: "workspace_1"}, "task_9")
	if err != nil {
		t.Fatalf("start task: %v", err)
	}

	if store.workspaceID != "workspace_1" {
		t.Fatalf("expected workspace workspace_1, got %s", store.workspaceID)
	}
	if store.requestedID != "task_9" {
		t.Fatalf("expected task id task_9, got %s", store.requestedID)
	}
	if task.Status != "running" || len(task.Steps) != 3 {
		t.Fatalf("unexpected task detail: %+v", task)
	}
	if task.BudgetConsumed != 6 || task.StartedAt == nil || !task.StartedAt.Equal(startedAt) {
		t.Fatalf("expected budget/timing to be preserved, got %+v", task)
	}
}

func TestPauseReturnsPausedTaskDetailForWorkspace(t *testing.T) {
	store := &fakeStore{
		detailTask: TaskDetail{
			Task: Task{
				ExecutionMode: "standard",
				Goal:          "Review launch plan",
				ID:            "task_3",
				Status:        "paused",
				Title:         "Review launch plan",
			},
			Steps: []TaskStep{
				{ID: "step_1", Status: "completed", StepIndex: 1, Title: "Understand the goal"},
				{ID: "step_2", Status: "running", StepIndex: 2, Title: "Review workspace context"},
			},
		},
	}
	service := NewService(store)

	task, err := service.Pause(context.Background(), auth.Session{WorkspaceID: "workspace_1"}, "task_3")
	if err != nil {
		t.Fatalf("pause task: %v", err)
	}

	if store.pausedTaskID != "task_3" {
		t.Fatalf("expected paused task id task_3, got %s", store.pausedTaskID)
	}
	if task.Status != "paused" {
		t.Fatalf("unexpected paused task detail: %+v", task)
	}
}

func TestResumeReturnsCompletedTaskDetailForWorkspace(t *testing.T) {
	startedAt := time.Date(2026, time.April, 3, 19, 0, 0, 0, time.UTC)
	finishedAt := time.Date(2026, time.April, 3, 19, 12, 0, 0, time.UTC)
	store := &fakeStore{
		detailTask: TaskDetail{
			Task: Task{
				BudgetConsumed: 8,
				ExecutionMode:  "standard",
				FinishedAt:     &finishedAt,
				Goal:           "Review launch plan",
				ID:             "task_3",
				ResultSummary:  "Completed a starter SOLO run for: Review launch plan",
				StartedAt:      &startedAt,
				Status:         "completed",
				Title:          "Review launch plan",
			},
			Steps: []TaskStep{
				{ID: "step_1", Status: "completed", StepIndex: 1, Title: "Understand the goal"},
				{ID: "step_2", Status: "completed", StepIndex: 2, Title: "Review workspace context"},
				{ID: "step_3", Status: "completed", StepIndex: 3, Title: "Deliver starter result"},
			},
		},
	}
	service := NewService(store)

	task, err := service.Resume(context.Background(), auth.Session{WorkspaceID: "workspace_1"}, "task_3")
	if err != nil {
		t.Fatalf("resume task: %v", err)
	}

	if store.resumedTaskID != "task_3" {
		t.Fatalf("expected resumed task id task_3, got %s", store.resumedTaskID)
	}
	if task.Status != "completed" || task.ResultSummary == "" {
		t.Fatalf("unexpected resumed task detail: %+v", task)
	}
	if task.BudgetConsumed != 8 || task.FinishedAt == nil || !task.FinishedAt.Equal(finishedAt) {
		t.Fatalf("expected budget/timing to be preserved, got %+v", task)
	}
}

func TestCancelReturnsCancelledTaskDetailForWorkspace(t *testing.T) {
	store := &fakeStore{
		detailTask: TaskDetail{
			Task: Task{
				ExecutionMode: "standard",
				Goal:          "Review launch plan",
				ID:            "task_3",
				Status:        "cancelled",
				Title:         "Review launch plan",
			},
		},
	}
	service := NewService(store)

	task, err := service.Cancel(context.Background(), auth.Session{WorkspaceID: "workspace_1"}, "task_3")
	if err != nil {
		t.Fatalf("cancel task: %v", err)
	}

	if store.cancelledTaskID != "task_3" {
		t.Fatalf("expected cancelled task id task_3, got %s", store.cancelledTaskID)
	}
	if task.Status != "cancelled" {
		t.Fatalf("unexpected cancelled task detail: %+v", task)
	}
}
