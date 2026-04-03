package task

import (
	"context"
	"testing"
	"time"

	"oblivious/server/internal/auth"
)

type fakeStore struct {
	approvedTaskID       string
	createdAuthorization string
	cancelledTaskID      string
	createdExecutionMode string
	createdGoal          string
	createdKnowledgeIDs  []string
	createdToolAllowList []string
	createdToolDenyList  []string
	createdTask          Task
	detailTask           TaskDetail
	listedTasks          []Task
	pausedTaskID         string
	requestedID          string
	resumedTaskID        string
	updatedBudgetLimit   int
	updatedBudgetTaskID  string
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
	authorizationScope string,
	budgetLimit int,
	knowledgeBaseIDs []string,
	toolAllowList []string,
	toolDenyList []string,
) (Task, error) {
	f.workspaceID = workspaceID
	f.createdGoal = goal
	f.createdExecutionMode = executionMode
	f.createdAuthorization = authorizationScope
	f.createdKnowledgeIDs = append([]string(nil), knowledgeBaseIDs...)
	f.createdToolAllowList = append([]string(nil), toolAllowList...)
	f.createdToolDenyList = append([]string(nil), toolDenyList...)
	return f.createdTask, nil
}

func (f *fakeStore) StartTask(ctx context.Context, workspaceID, taskID string) (TaskDetail, error) {
	f.workspaceID = workspaceID
	f.requestedID = taskID
	return f.detailTask, nil
}

func (f *fakeStore) ApproveTask(ctx context.Context, workspaceID, taskID string) (TaskDetail, error) {
	f.workspaceID = workspaceID
	f.approvedTaskID = taskID
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

func (f *fakeStore) UpdateTaskBudget(ctx context.Context, workspaceID, taskID string, budgetLimit int) (TaskDetail, error) {
	f.workspaceID = workspaceID
	f.updatedBudgetTaskID = taskID
	f.updatedBudgetLimit = budgetLimit
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

func TestCreateNormalizesGoalKnowledgeBaseIDsAndToolRules(t *testing.T) {
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
		"",
		0,
		[]string{"kb_1", " ", "kb_1", "kb_2"},
		[]string{" browser ", "shell", "browser", "dangerous"},
		[]string{"dangerous", " ", "email"},
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
	if store.createdAuthorization != "workspace_tools" {
		t.Fatalf("expected default authorization scope workspace_tools, got %q", store.createdAuthorization)
	}
	if len(store.createdKnowledgeIDs) != 2 || store.createdKnowledgeIDs[0] != "kb_1" || store.createdKnowledgeIDs[1] != "kb_2" {
		t.Fatalf("expected normalized knowledge ids [kb_1 kb_2], got %+v", store.createdKnowledgeIDs)
	}
	if len(store.createdToolAllowList) != 2 || store.createdToolAllowList[0] != "browser" || store.createdToolAllowList[1] != "shell" {
		t.Fatalf("expected normalized tool allow list [browser shell], got %+v", store.createdToolAllowList)
	}
	if len(store.createdToolDenyList) != 2 || store.createdToolDenyList[0] != "dangerous" || store.createdToolDenyList[1] != "email" {
		t.Fatalf("expected normalized tool deny list [dangerous email], got %+v", store.createdToolDenyList)
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
				AuthorizationScope: "workspace_tools",
				BudgetConsumed:     6,
				ExecutionMode:      "safe",
				Goal:               "Compile launch notes",
				ID:                 "task_9",
				StartedAt:          &startedAt,
				Status:             "running",
				Title:              "Compile launch notes",
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

func TestApproveReturnsRunningTaskDetailForWorkspace(t *testing.T) {
	startedAt := time.Date(2026, time.April, 3, 19, 0, 0, 0, time.UTC)
	store := &fakeStore{
		detailTask: TaskDetail{
			Task: Task{
				AuthorizationScope: "full_access",
				BudgetConsumed:     6,
				ExecutionMode:      "safe",
				Goal:               "Compile vendor outreach plan",
				ID:                 "task_9",
				StartedAt:          &startedAt,
				Status:             "running",
				Title:              "Compile vendor outreach plan",
			},
			KnowledgeBaseIDs: []string{"kb_1"},
			Steps: []TaskStep{
				{ID: "step_1", Status: "completed", StepIndex: 1, Title: "Understand the goal"},
				{ID: "step_2", Status: "completed", StepIndex: 2, Title: "Confirm execution boundary"},
				{ID: "step_3", Status: "running", StepIndex: 3, Title: "Deliver starter result"},
			},
		},
	}
	service := NewService(store)

	task, err := service.Approve(context.Background(), auth.Session{WorkspaceID: "workspace_1"}, "task_9")
	if err != nil {
		t.Fatalf("approve task: %v", err)
	}

	if store.approvedTaskID != "task_9" {
		t.Fatalf("expected approved task id task_9, got %s", store.approvedTaskID)
	}
	if task.Status != "running" || task.AuthorizationScope != "full_access" {
		t.Fatalf("unexpected approved task detail: %+v", task)
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

func TestUpdateBudgetNormalizesLimitForWorkspace(t *testing.T) {
	store := &fakeStore{
		detailTask: TaskDetail{
			Task: Task{
				AuthorizationScope: "workspace_tools",
				BudgetConsumed:     4,
				BudgetLimit:        0,
				ExecutionMode:      "standard",
				Goal:               "Review launch plan",
				ID:                 "task_3",
				Status:             "running",
				Title:              "Review launch plan",
			},
		},
	}
	service := NewService(store)

	task, err := service.UpdateBudget(context.Background(), auth.Session{WorkspaceID: "workspace_1"}, "task_3", -5)
	if err != nil {
		t.Fatalf("update budget: %v", err)
	}

	if store.updatedBudgetTaskID != "task_3" {
		t.Fatalf("expected updated task id task_3, got %s", store.updatedBudgetTaskID)
	}
	if store.updatedBudgetLimit != 0 {
		t.Fatalf("expected normalized budget limit 0, got %d", store.updatedBudgetLimit)
	}
	if task.BudgetLimit != 0 || task.Status != "running" {
		t.Fatalf("unexpected updated task detail: %+v", task)
	}
}
