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
	"oblivious/server/internal/schedule"
)

func TestScheduledTasksRouteCreatesAndListsTasks(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)
	cookie, csrfToken, userID := registerHTTPUser(t, router, "schedule-route@example.com")
	_, organizationID := queryHTTPUserScope(t, database, userID)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/scheduled-tasks", strings.NewReader(`{"name":"Hourly workflow","targetType":"workflow","targetId":"workflow_1","cronExpression":"0 * * * *","enabled":true}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	addCSRF(createRequest, csrfToken)
	router.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != stdhttp.StatusCreated {
		t.Fatalf("create scheduled task expected 201, got %d with body %s", createRecorder.Code, createRecorder.Body.String())
	}

	var createResponse struct {
		Data struct {
			ID             string `json:"id"`
			OrganizationID string `json:"organizationId"`
			Name           string `json:"name"`
			TargetType     string `json:"targetType"`
			TargetID       string `json:"targetId"`
			CronExpression string `json:"cronExpression"`
			Enabled        bool   `json:"enabled"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &createResponse); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if createResponse.Data.ID == "" || createResponse.Data.OrganizationID != organizationID {
		t.Fatalf("unexpected created scheduled task: %+v organization=%s", createResponse.Data, organizationID)
	}
	if createResponse.Data.Name != "Hourly workflow" || createResponse.Data.TargetType != "workflow" || createResponse.Data.TargetID != "workflow_1" || createResponse.Data.CronExpression != "0 * * * *" || !createResponse.Data.Enabled {
		t.Fatalf("unexpected created scheduled task payload: %+v", createResponse.Data)
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/scheduled-tasks", nil)
	listRequest.AddCookie(cookie)
	router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("list scheduled tasks expected 200, got %d with body %s", listRecorder.Code, listRecorder.Body.String())
	}

	var listResponse struct {
		Data []struct {
			ID             string `json:"id"`
			OrganizationID string `json:"organizationId"`
			Name           string `json:"name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	if len(listResponse.Data) != 1 || listResponse.Data[0].ID != createResponse.Data.ID || listResponse.Data[0].OrganizationID != organizationID {
		t.Fatalf("unexpected listed scheduled tasks: %+v", listResponse.Data)
	}
	if listResponse.Data[0].Name != "Hourly workflow" {
		t.Fatalf("expected listed scheduled task name, got %+v", listResponse.Data)
	}
}

func TestScheduledTasksRouteListsRunsForTaskWithinSessionOrganization(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)
	cookie, csrfToken, userID := registerHTTPUser(t, router, "schedule-runs-route@example.com")
	_, organizationID := queryHTTPUserScope(t, database, userID)

	createRecorder := httptest.NewRecorder()
	createRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/scheduled-tasks", strings.NewReader(`{"name":"Hourly workflow","targetType":"workflow","targetId":"workflow_1","cronExpression":"0 * * * *","enabled":true}`))
	createRequest.Header.Set("Content-Type", "application/json")
	createRequest.AddCookie(cookie)
	addCSRF(createRequest, csrfToken)
	router.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != stdhttp.StatusCreated {
		t.Fatalf("create scheduled task expected 201, got %d with body %s", createRecorder.Code, createRecorder.Body.String())
	}

	var createResponse struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(createRecorder.Body.Bytes(), &createResponse); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	startedAt := time.Date(2026, time.June, 5, 9, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(90 * time.Second)
	insertScheduledTaskRun(t, database, "schedrun_route_older", organizationID, createResponse.Data.ID, "running", &startedAt, nil, "")
	insertScheduledTaskRun(t, database, "schedrun_route_newer", organizationID, createResponse.Data.ID, "failed", &startedAt, &finishedAt, "workflow failed")

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/scheduled-tasks/"+createResponse.Data.ID+"/runs", nil)
	listRequest.AddCookie(cookie)
	router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("list scheduled task runs expected 200, got %d with body %s", listRecorder.Code, listRecorder.Body.String())
	}

	var listResponse struct {
		Data []struct {
			ID              string `json:"id"`
			OrganizationID  string `json:"organizationId"`
			ScheduledTaskID string `json:"scheduledTaskId"`
			Status          string `json:"status"`
			Error           string `json:"error"`
		} `json:"data"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("decode list runs response: %v", err)
	}
	if len(listResponse.Data) != 2 {
		t.Fatalf("expected two scheduled task runs, got %+v", listResponse.Data)
	}
	if listResponse.Data[0].ID != "schedrun_route_newer" || listResponse.Data[0].Status != "failed" || listResponse.Data[0].Error != "workflow failed" {
		t.Fatalf("expected newest failed run first, got %+v", listResponse.Data)
	}
	for _, run := range listResponse.Data {
		if run.OrganizationID != organizationID || run.ScheduledTaskID != createResponse.Data.ID {
			t.Fatalf("list leaked wrong organization or task: %+v", run)
		}
	}
}

func TestRegisterScheduleRoutesDispatchesRunsRoute(t *testing.T) {
	store := &scheduleRouteFakeStore{
		listedRuns: []schedule.ScheduledTaskRun{
			{ID: "schedrun_1", OrganizationID: "org_1", ScheduledTaskID: "sched_1", Status: schedule.RunStatusRunning},
		},
	}
	handler := newScheduleHandler(schedule.NewService(store))
	mux := stdhttp.NewServeMux()
	registerScheduleRoutes(mux, passThroughAuthMiddleware{}, handler)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, scheduleTestRequest(stdhttp.MethodGet, "/api/v1/scheduled-tasks/sched_1/runs", ""))

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.listedRunsOrgID != "org_1" || store.listedRunsTaskID != "sched_1" {
		t.Fatalf("expected route to list org_1 sched_1 runs, got org=%q task=%q", store.listedRunsOrgID, store.listedRunsTaskID)
	}
	if !strings.Contains(recorder.Body.String(), `"id":"schedrun_1"`) {
		t.Fatalf("expected scheduled task run response, got %s", recorder.Body.String())
	}
}

func TestRegisterScheduleRoutesCreatesAndListsNamedTasks(t *testing.T) {
	store := &scheduleRouteFakeStore{
		createdTask: schedule.ScheduledTask{
			ID:             "sched_1",
			OrganizationID: "org_1",
			Name:           "Daily digest",
			TargetType:     schedule.TargetTypeWorkflow,
			TargetID:       "workflow_1",
			CronExpression: "0 9 * * *",
			Enabled:        true,
		},
		listedTasks: []schedule.ScheduledTask{
			{
				ID:             "sched_1",
				OrganizationID: "org_1",
				Name:           "Daily digest",
				TargetType:     schedule.TargetTypeWorkflow,
				TargetID:       "workflow_1",
				CronExpression: "0 9 * * *",
				Enabled:        true,
			},
		},
	}
	handler := newScheduleHandler(schedule.NewService(store))
	mux := stdhttp.NewServeMux()
	registerScheduleRoutes(mux, passThroughAuthMiddleware{}, handler)

	createRecorder := httptest.NewRecorder()
	mux.ServeHTTP(createRecorder, scheduleTestRequest(stdhttp.MethodPost, "/api/v1/scheduled-tasks", `{"name":"  Daily digest  ","targetType":"workflow","targetId":"workflow_1","cronExpression":"0 9 * * *","enabled":true}`))

	if createRecorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected create 201, got %d with body %s", createRecorder.Code, createRecorder.Body.String())
	}
	if store.createdInput.Name != "Daily digest" {
		t.Fatalf("expected route to trim and forward scheduled task name, got %+v", store.createdInput)
	}
	if !strings.Contains(createRecorder.Body.String(), `"name":"Daily digest"`) {
		t.Fatalf("expected created scheduled task name response, got %s", createRecorder.Body.String())
	}

	listRecorder := httptest.NewRecorder()
	mux.ServeHTTP(listRecorder, scheduleTestRequest(stdhttp.MethodGet, "/api/v1/scheduled-tasks", ""))
	if listRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected list 200, got %d with body %s", listRecorder.Code, listRecorder.Body.String())
	}
	if !strings.Contains(listRecorder.Body.String(), `"name":"Daily digest"`) {
		t.Fatalf("expected listed scheduled task name response, got %s", listRecorder.Body.String())
	}
}

func TestRegisterScheduleRoutesRejectsBlankTaskName(t *testing.T) {
	handler := newScheduleHandler(schedule.NewService(&scheduleRouteFakeStore{}))
	mux := stdhttp.NewServeMux()
	registerScheduleRoutes(mux, passThroughAuthMiddleware{}, handler)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, scheduleTestRequest(stdhttp.MethodPost, "/api/v1/scheduled-tasks", `{"name":"   ","targetType":"workflow","targetId":"workflow_1","cronExpression":"0 9 * * *","enabled":true}`))

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("expected blank name 400, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), schedule.ErrInvalidScheduledTaskName.Error()) {
		t.Fatalf("expected scheduled task name validation error, got %s", recorder.Body.String())
	}
}

func TestRegisterScheduleRoutesDispatchesStatusAndRunNowRoutes(t *testing.T) {
	store := &scheduleRouteFakeStore{
		gotTask: schedule.ScheduledTask{
			ID:             "sched_1",
			OrganizationID: "org_1",
			TargetType:     schedule.TargetTypeWorkflow,
			TargetID:       "workflow_1",
			CronExpression: "0 * * * *",
			Enabled:        false,
		},
		updatedTask: schedule.ScheduledTask{
			ID:             "sched_1",
			OrganizationID: "org_1",
			TargetType:     schedule.TargetTypeWorkflow,
			TargetID:       "workflow_1",
			CronExpression: "0 * * * *",
			Enabled:        true,
		},
		recordedRun: schedule.ScheduledTaskRun{
			ID:              "schedrun_1",
			OrganizationID:  "org_1",
			ScheduledTaskID: "sched_1",
			Status:          schedule.RunStatusQueued,
		},
	}
	handler := newScheduleHandler(schedule.NewService(store))
	mux := stdhttp.NewServeMux()
	registerScheduleRoutes(mux, passThroughAuthMiddleware{}, handler)

	statusRecorder := httptest.NewRecorder()
	mux.ServeHTTP(statusRecorder, scheduleTestRequest(stdhttp.MethodPatch, "/api/v1/scheduled-tasks/sched_1/status", `{"enabled":true}`))
	if statusRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected status route 200, got %d with body %s", statusRecorder.Code, statusRecorder.Body.String())
	}
	if store.updateEnabledTaskID != "sched_1" || !store.updateEnabledInput.Enabled {
		t.Fatalf("expected status route to enable sched_1, task=%q input=%+v", store.updateEnabledTaskID, store.updateEnabledInput)
	}
	if !strings.Contains(statusRecorder.Body.String(), `"enabled":true`) {
		t.Fatalf("expected enabled task response, got %s", statusRecorder.Body.String())
	}

	runRecorder := httptest.NewRecorder()
	mux.ServeHTTP(runRecorder, scheduleTestRequest(stdhttp.MethodPost, "/api/v1/scheduled-tasks/sched_1/run", `{}`))
	if runRecorder.Code != stdhttp.StatusAccepted {
		t.Fatalf("expected run-now route 202, got %d with body %s", runRecorder.Code, runRecorder.Body.String())
	}
	if store.recordedRunInput.ScheduledTaskID != "sched_1" || store.recordedRunInput.Status != schedule.RunStatusRunning {
		t.Fatalf("expected run-now route to record running run for sched_1, got %+v", store.recordedRunInput)
	}
	if !strings.Contains(runRecorder.Body.String(), `"status":"queued"`) {
		t.Fatalf("expected scheduled run response, got %s", runRecorder.Body.String())
	}
}

func scheduleTestRequest(method, path, body string) *stdhttp.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return request.WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		User:           auth.User{ID: "user_1"},
		WorkspaceID:    "workspace_1",
	}))
}

type scheduleRouteFakeStore struct {
	listedRuns       []schedule.ScheduledTaskRun
	listedTasks      []schedule.ScheduledTask
	listedRunsOrgID  string
	listedRunsTaskID string
	listedTasksOrgID string
	gotTask          schedule.ScheduledTask
	createdTask      schedule.ScheduledTask
	updatedTask      schedule.ScheduledTask
	recordedRun      schedule.ScheduledTaskRun

	createdInput        schedule.CreateScheduledTaskInput
	updateEnabledTaskID string
	updateEnabledInput  schedule.UpdateScheduledTaskEnabledInput
	recordedRunInput    schedule.RecordScheduledTaskRunInput
}

func (s *scheduleRouteFakeStore) CreateScheduledTask(ctx context.Context, input schedule.CreateScheduledTaskInput) (schedule.ScheduledTask, error) {
	s.createdInput = input
	task := s.createdTask
	if task.ID == "" {
		task = schedule.ScheduledTask{
			ID:             "sched_1",
			OrganizationID: input.OrganizationID,
			Name:           input.Name,
			TargetType:     input.TargetType,
			TargetID:       input.TargetID,
			CronExpression: input.CronExpression,
			Enabled:        input.Enabled,
			NextRunAt:      input.NextRunAt,
		}
	}
	return task, nil
}

func (s *scheduleRouteFakeStore) SyncWorkflowScheduledTasks(ctx context.Context, input schedule.SyncWorkflowScheduledTasksInput) ([]schedule.ScheduledTask, error) {
	return nil, nil
}

func (s *scheduleRouteFakeStore) ListScheduledTasks(ctx context.Context, organizationID string) ([]schedule.ScheduledTask, error) {
	s.listedTasksOrgID = organizationID
	return s.listedTasks, nil
}

func (s *scheduleRouteFakeStore) GetScheduledTask(ctx context.Context, organizationID string, scheduledTaskID string) (schedule.ScheduledTask, error) {
	task := s.gotTask
	if task.ID == "" {
		task = schedule.ScheduledTask{
			ID:             scheduledTaskID,
			OrganizationID: organizationID,
			TargetType:     schedule.TargetTypeWorkflow,
			TargetID:       "workflow_1",
			CronExpression: "0 * * * *",
			Enabled:        true,
		}
	}
	return task, nil
}

func (s *scheduleRouteFakeStore) UpdateScheduledTaskEnabled(ctx context.Context, organizationID string, scheduledTaskID string, input schedule.UpdateScheduledTaskEnabledInput) (schedule.ScheduledTask, error) {
	s.updateEnabledTaskID = scheduledTaskID
	s.updateEnabledInput = input
	task := s.updatedTask
	if task.ID == "" {
		task = schedule.ScheduledTask{
			ID:             scheduledTaskID,
			OrganizationID: organizationID,
			TargetType:     schedule.TargetTypeWorkflow,
			TargetID:       "workflow_1",
			CronExpression: "0 * * * *",
			Enabled:        input.Enabled,
			NextRunAt:      input.NextRunAt,
		}
	}
	return task, nil
}

func (s *scheduleRouteFakeStore) RecordScheduledTaskRun(ctx context.Context, input schedule.RecordScheduledTaskRunInput) (schedule.ScheduledTaskRun, error) {
	s.recordedRunInput = input
	run := s.recordedRun
	if run.ID == "" {
		run = schedule.ScheduledTaskRun{
			ID:              "schedrun_1",
			OrganizationID:  input.OrganizationID,
			ScheduledTaskID: input.ScheduledTaskID,
			Status:          input.Status,
			StartedAt:       input.StartedAt,
		}
	}
	return run, nil
}

func (s *scheduleRouteFakeStore) CompleteManualScheduledTaskRun(ctx context.Context, organizationID string, scheduledTaskID string, scheduledTaskRunID string, finishedAt time.Time) (schedule.ScheduledTaskRun, error) {
	return schedule.ScheduledTaskRun{
		ID:              scheduledTaskRunID,
		OrganizationID:  organizationID,
		ScheduledTaskID: scheduledTaskID,
		Status:          schedule.RunStatusCompleted,
		FinishedAt:      &finishedAt,
	}, nil
}

func (s *scheduleRouteFakeStore) UpdateScheduledTaskRun(ctx context.Context, organizationID string, scheduledTaskRunID string, input schedule.UpdateScheduledTaskRunInput) (schedule.ScheduledTaskRun, error) {
	return schedule.ScheduledTaskRun{}, nil
}

func (s *scheduleRouteFakeStore) ListScheduledTaskRuns(ctx context.Context, organizationID string, scheduledTaskID string) ([]schedule.ScheduledTaskRun, error) {
	s.listedRunsOrgID = organizationID
	s.listedRunsTaskID = scheduledTaskID
	return s.listedRuns, nil
}

func (s *scheduleRouteFakeStore) CountRunningScheduledTaskRuns(ctx context.Context, organizationID string, scheduledTaskID string) (int, error) {
	return 0, nil
}

func insertScheduledTaskRun(t *testing.T, database *sql.DB, id string, organizationID string, scheduledTaskID string, status string, startedAt *time.Time, finishedAt *time.Time, errorText string) {
	t.Helper()

	if _, err := database.Exec(`
		INSERT INTO scheduled_task_runs (id, organization_id, scheduled_task_id, status, started_at, finished_at, error, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
	`, id, organizationID, scheduledTaskID, status, startedAt, finishedAt, errorText); err != nil {
		t.Fatalf("insert scheduled task run: %v", err)
	}
}
