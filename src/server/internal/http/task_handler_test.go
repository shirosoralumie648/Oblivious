package http

import (
	"context"
	"database/sql"
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
	createdToolAllowList []string
	createdToolDenyList  []string
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
	toolAllowList []string,
	toolDenyList []string,
) (task.Task, error) {
	f.workspaceID = workspaceID
	f.createdGoal = goal
	f.createdExecutionMode = executionMode
	f.createdAuthorization = authorizationScope
	f.createdBudgetLimit = budgetLimit
	f.createdKnowledgeIDs = append([]string(nil), knowledgeBaseIDs...)
	f.createdToolAllowList = append([]string(nil), toolAllowList...)
	f.createdToolDenyList = append([]string(nil), toolDenyList...)
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
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/tasks", strings.NewReader(`{"goal":"Draft onboarding checklist","executionMode":"safe","authorizationScope":"full_access","budgetLimit":25,"knowledgeBaseIds":["kb_1","kb_3"],"toolAllowList":["browser","shell","browser"],"toolDenyList":["shell","email"]}`)).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
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
	if len(store.createdToolAllowList) != 1 || store.createdToolAllowList[0] != "browser" {
		t.Fatalf("unexpected tool allow list: %+v", store.createdToolAllowList)
	}
	if len(store.createdToolDenyList) != 2 || store.createdToolDenyList[0] != "shell" || store.createdToolDenyList[1] != "email" {
		t.Fatalf("unexpected tool deny list: %+v", store.createdToolDenyList)
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

func TestTaskHandlerGetTaskIncludesEmptyToolRules(t *testing.T) {
	store := &taskFakeStore{
		detailTask: task.TaskDetail{
			Task: task.Task{
				AuthorizationScope: "workspace_tools",
				BudgetLimit:        12,
				ExecutionMode:      "standard",
				Goal:               "Review launch plan",
				ID:                 "task_1",
				Status:             "completed",
				Title:              "Review launch plan",
			},
			KnowledgeBaseIDs: []string{},
			Steps:            []task.TaskStep{},
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

	var response map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	data, ok := response["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected response data object, got %+v", response["data"])
	}

	toolAllowList, ok := data["toolAllowList"].([]any)
	if !ok || len(toolAllowList) != 0 {
		t.Fatalf("expected empty toolAllowList, got %+v", data["toolAllowList"])
	}

	toolDenyList, ok := data["toolDenyList"].([]any)
	if !ok || len(toolDenyList) != 0 {
		t.Fatalf("expected empty toolDenyList, got %+v", data["toolDenyList"])
	}
}

func TestTaskHandlerGetTaskIncludesRuntimeMetadata(t *testing.T) {
	startedAt := time.Date(2026, time.April, 4, 9, 0, 0, 0, time.UTC)
	store := &taskFakeStore{
		detailTask: task.TaskDetail{
			Task: task.Task{
				AuthorizationScope: "workspace_tools",
				BudgetConsumed:     4,
				BudgetLimit:        12,
				ExecutionMode:      "standard",
				Goal:               "Review launch plan",
				ID:                 "task_runtime",
				StartedAt:          &startedAt,
				Status:             "running",
				Title:              "Review launch plan",
			},
			KnowledgeBaseIDs: []string{"kb_2"},
			Steps: []task.TaskStep{
				{ID: "step_1", Status: "completed", StepIndex: 1, Title: "Understand the goal"},
				{ID: "step_2", Status: "running", StepIndex: 2, Title: "Review workspace context"},
				{ID: "step_3", Status: "pending", StepIndex: 3, Title: "Deliver runtime result"},
			},
		},
	}
	handler := newTaskHandler(task.NewService(store))
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/tasks/task_runtime", nil).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		WorkspaceID: "workspace_1",
	}))
	recorder := httptest.NewRecorder()

	handler.getTask(recorder, request, "task_runtime")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}

	var response struct {
		Data task.TaskDetail `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.CurrentStep != "Review workspace context" {
		t.Fatalf("expected current step in response, got %+v", response.Data)
	}
	if len(response.Data.Events) == 0 {
		t.Fatalf("expected runtime events in response, got %+v", response.Data)
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

func TestTaskApproveRouteRequiresAwaitingConfirmationAndWorkspaceScope(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)

	cookie, csrfToken, userID := registerHTTPUser(t, router, "task-approve-owner@example.com")
	workspaceID, _ := queryHTTPUserScope(t, database, userID)
	_, _, otherUserID := registerHTTPUser(t, router, "task-approve-other@example.com")
	otherWorkspaceID, _ := queryHTTPUserScope(t, database, otherUserID)

	validTaskID := "task_approve_valid"
	seedTaskApprovalFixture(t, database, workspaceID, userID, validTaskID, task.TaskStatusAwaitingConfirmation, 0)

	validRecorder := httptest.NewRecorder()
	validRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/tasks/"+validTaskID+"/approve", nil)
	validRequest.AddCookie(cookie)
	addCSRF(validRequest, csrfToken)
	router.ServeHTTP(validRecorder, validRequest)
	if validRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("valid approve expected 200, got %d with body %s", validRecorder.Code, validRecorder.Body.String())
	}

	var validResponse struct {
		Data task.TaskDetail `json:"data"`
	}
	if err := json.Unmarshal(validRecorder.Body.Bytes(), &validResponse); err != nil {
		t.Fatalf("decode valid approve response: %v", err)
	}
	if validResponse.Data.Status != task.TaskStatusRunning || len(validResponse.Data.Steps) != 3 {
		t.Fatalf("unexpected valid approve response: %+v", validResponse.Data)
	}
	assertTaskApprovalSnapshot(t, queryTaskApprovalSnapshot(t, database, validTaskID), taskApprovalSnapshot{
		status:         task.TaskStatusRunning,
		budgetConsumed: 10,
		stepStatuses:   []string{task.TaskStepStatusCompleted, task.TaskStepStatusCompleted, task.TaskStepStatusRunning},
	})

	blockedCases := []struct {
		name        string
		taskID      string
		workspaceID string
		userID      string
		status      string
	}{
		{name: "completed task", taskID: "task_approve_completed", workspaceID: workspaceID, userID: userID, status: task.TaskStatusCompleted},
		{name: "cancelled task", taskID: "task_approve_cancelled", workspaceID: workspaceID, userID: userID, status: task.TaskStatusCancelled},
		{name: "running task", taskID: "task_approve_running", workspaceID: workspaceID, userID: userID, status: task.TaskStatusRunning},
		{name: "draft task", taskID: "task_approve_draft", workspaceID: workspaceID, userID: userID, status: task.TaskStatusDraft},
		{name: "cross workspace task", taskID: "task_approve_cross_workspace", workspaceID: otherWorkspaceID, userID: otherUserID, status: task.TaskStatusAwaitingConfirmation},
	}
	wantUnchanged := taskApprovalSnapshot{
		status:         "",
		budgetConsumed: 7,
		stepStatuses:   []string{task.TaskStepStatusCompleted, task.TaskStepStatusAwaitingConfirmation, task.TaskStepStatusPending},
	}
	for _, tc := range blockedCases {
		t.Run(tc.name, func(t *testing.T) {
			seedTaskApprovalFixture(t, database, tc.workspaceID, tc.userID, tc.taskID, tc.status, wantUnchanged.budgetConsumed)
			want := wantUnchanged
			want.status = tc.status

			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/tasks/"+tc.taskID+"/approve", nil)
			request.AddCookie(cookie)
			addCSRF(request, csrfToken)
			router.ServeHTTP(recorder, request)
			if recorder.Code < 400 {
				t.Fatalf("blocked approve expected non-success, got %d with body %s", recorder.Code, recorder.Body.String())
			}

			assertTaskApprovalSnapshot(t, queryTaskApprovalSnapshot(t, database, tc.taskID), want)
		})
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

type taskApprovalSnapshot struct {
	status         string
	budgetConsumed int
	stepStatuses   []string
}

func seedTaskApprovalFixture(t *testing.T, database *sql.DB, workspaceID, userID, taskID, status string, budgetConsumed int) {
	t.Helper()

	if _, err := database.Exec(`
		INSERT INTO tasks (
			id,
			workspace_id,
			user_id,
			title,
			goal,
			execution_mode,
			authorization_scope,
			status,
			budget_limit,
			budget_consumed,
			created_at,
			updated_at
		)
		VALUES ($1, $2, $3, $4, $5, 'safe', 'full_access', $6, 40, $7, NOW(), NOW())
	`, taskID, workspaceID, userID, taskID+" title", taskID+" goal", status, budgetConsumed); err != nil {
		t.Fatalf("seed task %s: %v", taskID, err)
	}

	steps := []struct {
		index  int
		status string
		title  string
	}{
		{index: 1, status: task.TaskStepStatusCompleted, title: "Understand the request"},
		{index: 2, status: task.TaskStepStatusAwaitingConfirmation, title: "Confirm execution boundary"},
		{index: 3, status: task.TaskStepStatusPending, title: "Execute the approved task"},
	}
	for _, step := range steps {
		if _, err := database.Exec(`
			INSERT INTO task_steps (
				id,
				task_id,
				step_index,
				title,
				status,
				created_at,
				updated_at,
				started_at,
				finished_at
			)
			VALUES ($1, $2, $3, $4, $5, NOW(), NOW(), CASE WHEN $5 = 'completed' THEN NOW() ELSE NULL END, CASE WHEN $5 = 'completed' THEN NOW() ELSE NULL END)
		`, taskID+"_step_"+string(rune('0'+step.index)), taskID, step.index, step.title, step.status); err != nil {
			t.Fatalf("seed task step %s.%d: %v", taskID, step.index, err)
		}
	}
}

func queryTaskApprovalSnapshot(t *testing.T, database *sql.DB, taskID string) taskApprovalSnapshot {
	t.Helper()

	var snapshot taskApprovalSnapshot
	if err := database.QueryRow(`
		SELECT status, budget_consumed
		FROM tasks
		WHERE id = $1
	`, taskID).Scan(&snapshot.status, &snapshot.budgetConsumed); err != nil {
		t.Fatalf("query task approval snapshot %s: %v", taskID, err)
	}

	rows, err := database.Query(`
		SELECT status
		FROM task_steps
		WHERE task_id = $1
		ORDER BY step_index ASC
	`, taskID)
	if err != nil {
		t.Fatalf("query task step approval snapshot %s: %v", taskID, err)
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			t.Fatalf("scan task step approval snapshot %s: %v", taskID, err)
		}
		snapshot.stepStatuses = append(snapshot.stepStatuses, status)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read task step approval snapshot %s: %v", taskID, err)
	}
	return snapshot
}

func assertTaskApprovalSnapshot(t *testing.T, got, want taskApprovalSnapshot) {
	t.Helper()

	if got.status != want.status || got.budgetConsumed != want.budgetConsumed || strings.Join(got.stepStatuses, ",") != strings.Join(want.stepStatuses, ",") {
		t.Fatalf("unexpected task approval snapshot:\n got: %+v\nwant: %+v", got, want)
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
				Status:        "running",
				Title:         "Review launch plan",
			},
			Steps: []task.TaskStep{
				{ID: "step_1", Status: "completed", StepIndex: 1, Title: "Understand the goal"},
				{ID: "step_2", Status: "running", StepIndex: 2, Title: "Review workspace context"},
				{ID: "step_3", Status: "pending", StepIndex: 3, Title: "Deliver runtime result"},
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

	var response struct {
		Data task.TaskDetail `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Status != "running" || response.Data.ResultSummary != "" {
		t.Fatalf("unexpected resumed task detail: %+v", response.Data)
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
