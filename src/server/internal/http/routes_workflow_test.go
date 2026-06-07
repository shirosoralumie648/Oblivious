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
	"oblivious/server/internal/workflow"
)

func TestRegisterWorkflowRoutesDispatchesExecutionActions(t *testing.T) {
	service := &workflowFakeService{
		executionDetail: &workflow.WorkflowExecution{ID: "wexec_1", WorkflowID: "workflow_1", Status: workflow.ExecutionStatusPaused},
	}
	handler := newWorkflowHandler(service)
	mux := stdhttp.NewServeMux()
	registerWorkflowRoutes(mux, passThroughAuthMiddleware{}, handler)

	request := workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/workflow_1/executions/wexec_1/resume", "")
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.resumedExecutionID != "wexec_1" {
		t.Fatalf("expected resumed execution wexec_1, got %q", service.resumedExecutionID)
	}
}

func TestRegisterWorkflowRoutesDispatchesWebhookTrigger(t *testing.T) {
	service := &workflowFakeService{
		started: &workflow.WorkflowExecution{ID: "wexec_webhook", WorkflowID: "workflow_1", Status: workflow.ExecutionStatusRunning},
	}
	handler := newWorkflowHandler(service)
	mux := stdhttp.NewServeMux()
	registerWorkflowRoutes(mux, passThroughAuthMiddleware{}, handler)

	request := workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/workflow_1/webhook", `{"source":"github","action":"opened"}`)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.workflowID != "workflow_1" || service.executionTriggerType != workflow.WorkflowTriggerWebhook {
		t.Fatalf("expected webhook trigger for workflow_1, got workflow=%q trigger=%q", service.workflowID, service.executionTriggerType)
	}
	if service.executionInput["source"] != "github" || service.executionTriggerPayload["payload"].(map[string]any)["action"] != "opened" {
		t.Fatalf("unexpected webhook payload input=%+v trigger=%+v", service.executionInput, service.executionTriggerPayload)
	}
}

func TestRegisterWorkflowRoutesDispatchesSemanticMatches(t *testing.T) {
	service := &workflowFakeService{
		semanticMatches: []workflow.SemanticTriggerMatch{
			{
				WorkflowID:         "workflow_1",
				WorkflowName:       "Incident triage",
				TriggerID:          "urgent-alerts",
				Keyword:            "urgent outage",
				SemanticThreshold:  0.85,
				Score:              0.91,
				MatchMethod:        "embedding",
				WorkflowDefinition: map[string]any{"nodes": []any{map[string]any{"id": "start"}}},
				TriggerDefinition:  map[string]any{"keywords": []any{"urgent outage"}},
			},
		},
	}
	handler := newWorkflowHandler(service)
	mux := stdhttp.NewServeMux()
	registerWorkflowRoutes(mux, passThroughAuthMiddleware{}, handler)

	request := workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/semantic-matches", `{"message":"urgent outage in production"}`)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.organizationID != "org_1" {
		t.Fatalf("expected organization org_1, got %q", service.organizationID)
	}
	if service.semanticMatchReq.OrganizationID != "" {
		t.Fatalf("expected adapter/handler boundary not to prefill organization, got %+v", service.semanticMatchReq)
	}
	if service.semanticMatchReq.UserID != "user_1" || service.semanticMatchReq.Message != "urgent outage in production" {
		t.Fatalf("unexpected semantic match request: %+v", service.semanticMatchReq)
	}
	if service.workflowID != "" {
		t.Fatalf("expected collection route not to be dispatched as workflow id, got %q", service.workflowID)
	}
	var response struct {
		Data []workflow.SemanticTriggerMatch `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 1 || response.Data[0].WorkflowID != "workflow_1" || response.Data[0].Keyword != "urgent outage" {
		t.Fatalf("unexpected semantic match response: %+v", response.Data)
	}
	if response.Data[0].Score != 0.91 || response.Data[0].MatchMethod != "embedding" {
		t.Fatalf("expected semantic score/method in response, got %+v", response.Data[0])
	}
}

func TestRegisterWorkflowRoutesDispatchesConversationMatches(t *testing.T) {
	service := &workflowFakeService{
		conversationMatches: []workflow.ConversationTriggerMatch{
			{
				WorkflowID:         "workflow_1",
				WorkflowName:       "Incident triage",
				WorkflowVersion:    2,
				TriggerID:          "conversation-main",
				ConversationID:     "conversation_incident",
				WorkflowDefinition: map[string]any{"nodes": []any{map[string]any{"id": "start"}}},
				TriggerDefinition:  map[string]any{"conversationId": "conversation_incident"},
			},
		},
	}
	handler := newWorkflowHandler(service)
	mux := stdhttp.NewServeMux()
	registerWorkflowRoutes(mux, passThroughAuthMiddleware{}, handler)

	request := workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/conversation-matches", `{"conversationId":" conversation_incident "}`)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.organizationID != "org_1" {
		t.Fatalf("expected organization org_1, got %q", service.organizationID)
	}
	if service.conversationMatchReq.OrganizationID != "" {
		t.Fatalf("expected adapter/handler boundary not to prefill organization, got %+v", service.conversationMatchReq)
	}
	if service.conversationMatchReq.ConversationID != "conversation_incident" {
		t.Fatalf("unexpected conversation match request: %+v", service.conversationMatchReq)
	}
	if service.workflowID != "" {
		t.Fatalf("expected collection route not to be dispatched as workflow id, got %q", service.workflowID)
	}
	var response struct {
		Data []workflow.ConversationTriggerMatch `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 1 || response.Data[0].WorkflowID != "workflow_1" || response.Data[0].ConversationID != "conversation_incident" {
		t.Fatalf("unexpected conversation match response: %+v", response.Data)
	}
	if response.Data[0].TriggerID != "conversation-main" || response.Data[0].WorkflowVersion != 2 {
		t.Fatalf("expected conversation trigger metadata in response, got %+v", response.Data[0])
	}
}

func TestRegisterWorkflowRoutesDispatchesSignedWebhookWithoutSession(t *testing.T) {
	service := &workflowFakeService{
		workflowDetail: &workflow.WorkflowDefinition{
			ID:             "workflow_1",
			OrganizationID: "org_1",
			Definition:     map[string]any{"webhook_secret": "top-secret"},
		},
		started: &workflow.WorkflowExecution{ID: "wexec_webhook", WorkflowID: "workflow_1", Status: workflow.ExecutionStatusRunning},
	}
	handler := newWorkflowHandler(service)
	mux := stdhttp.NewServeMux()
	authMiddleware := recordingWorkflowAuthMiddleware{}
	registerWorkflowRoutes(mux, &authMiddleware, handler)

	body := `{"source":"github","action":"opened"}`
	timestamp := workflowWebhookTimestamp(time.Now())
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/workflows/webhooks/org_1/workflow_1", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Oblivious-Timestamp", timestamp)
	request.Header.Set("X-Oblivious-Signature", workflowWebhookSignature("top-secret", timestamp, body))
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if authMiddleware.calls != 0 {
		t.Fatalf("expected public signed webhook to bypass session middleware, got %d calls", authMiddleware.calls)
	}
	if service.organizationID != "org_1" || service.workflowID != "workflow_1" || service.executionTriggerType != workflow.WorkflowTriggerWebhook {
		t.Fatalf("unexpected webhook trigger org=%q workflow=%q type=%q", service.organizationID, service.workflowID, service.executionTriggerType)
	}
}

func TestDefaultRouterSyncsWorkflowScheduleTriggersToScheduledTasks(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)
	cookie, csrfToken, userID := registerHTTPUser(t, router, "workflow-schedule-sync@example.com")
	_, organizationID := queryHTTPUserScope(t, database, userID)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/workflows", strings.NewReader(`{
		"name": "Scheduled Workflow",
		"status": "published",
		"definition": {
			"nodes": [{"id": "start", "type": "manual"}],
			"triggers": {
				"schedule": {"id": "daily-report", "cron": "0 9 * * 1"}
			}
		}
	}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)
	addCSRF(request, csrfToken)

	router.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected workflow create 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data workflow.WorkflowDefinition `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode workflow create response: %v", err)
	}
	if response.Data.ID == "" {
		t.Fatalf("expected created workflow id, got %+v", response.Data)
	}

	var count int
	var workflowTriggerID string
	var cronExpression string
	err := database.QueryRow(`
		SELECT COUNT(*), COALESCE(MAX(workflow_trigger_id), ''), COALESCE(MAX(cron_expression), '')
		FROM scheduled_tasks
		WHERE organization_id = $1 AND target_type = 'workflow' AND target_id = $2
	`, organizationID, response.Data.ID).Scan(&count, &workflowTriggerID, &cronExpression)
	if err != nil {
		t.Fatalf("query synced scheduled task: %v", err)
	}
	if count != 1 || workflowTriggerID != "daily-report" || cronExpression != "0 9 * * 1" {
		t.Fatalf("expected synced workflow scheduled task, count=%d trigger=%q cron=%q", count, workflowTriggerID, cronExpression)
	}
}

func TestRegisterWorkflowRoutesDispatchesResourceCheck(t *testing.T) {
	service := &workflowFakeService{
		executionDetail: &workflow.WorkflowExecution{ID: "wexec_1", WorkflowID: "workflow_1", Status: workflow.ExecutionStatusRunning},
	}
	handler := newWorkflowHandler(service)
	mux := stdhttp.NewServeMux()
	registerWorkflowRoutes(mux, passThroughAuthMiddleware{}, handler)

	request := workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/workflow_1/executions/wexec_1/resource-check", `{
		"totalTokens": 42,
		"nodeExecutionCount": 3
	}`)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.workflowID != "workflow_1" || service.checkedExecutionID != "wexec_1" {
		t.Fatalf("expected resource check workflow_1/wexec_1, got workflow=%q execution=%q", service.workflowID, service.checkedExecutionID)
	}
	if service.resourceUsage.TotalTokens != 42 || service.resourceUsage.NodeExecutionCount != 3 {
		t.Fatalf("unexpected resource usage: %+v", service.resourceUsage)
	}
}

func TestRegisterWorkflowRoutesDispatchesExecutionDebugSnapshot(t *testing.T) {
	service := &workflowFakeService{
		debugSnapshot: &workflow.ExecutionDebugSnapshot{
			ExecutionID: "wexec_1",
			WorkflowID:  "workflow_1",
			Status:      workflow.ExecutionStatusSucceeded,
			VariableSnapshot: workflow.ExecutionVariableSnapshot{
				Input:       map[string]any{"ticket": "INC-1"},
				Context:     map[string]any{"trigger": map[string]any{"type": "manual"}},
				NodeOutputs: map[string]map[string]any{"start": {"ticket": "INC-1"}},
			},
			Trace: []workflow.ExecutionDebugTraceEntry{
				{
					NodeID:     "start",
					NodeType:   "start",
					Status:     workflow.NodeStatusSucceeded,
					Attempt:    1,
					Input:      map[string]any{"ticket": "INC-1"},
					Output:     map[string]any{"ticket": "INC-1"},
					DurationMS: 12,
				},
			},
			Outputs: map[string]map[string]any{"start": {"ticket": "INC-1"}},
			Performance: workflow.ExecutionDebugPerformance{
				TotalDurationMS:  12,
				NodeDurationsMS:  map[string]int{"start": 12},
				BottleneckNodeID: "start",
			},
			Logs: []workflow.ExecutionDebugLogEntry{},
		},
	}
	handler := newWorkflowHandler(service)
	mux := stdhttp.NewServeMux()
	registerWorkflowRoutes(mux, passThroughAuthMiddleware{}, handler)

	request := workflowTestRequest(stdhttp.MethodGet, "/api/v1/workflows/workflow_1/executions/wexec_1/debug-snapshot", "")
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.organizationID != "org_1" || service.pausedExecutionID != "wexec_1" {
		t.Fatalf("expected debug snapshot for org_1/wexec_1, got org=%q execution=%q", service.organizationID, service.pausedExecutionID)
	}
	var response struct {
		Data workflow.ExecutionDebugSnapshot `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.ExecutionID != "wexec_1" || response.Data.WorkflowID != "workflow_1" || response.Data.Status != workflow.ExecutionStatusSucceeded {
		t.Fatalf("unexpected debug snapshot identity: %+v", response.Data)
	}
	if response.Data.VariableSnapshot.Input["ticket"] != "INC-1" {
		t.Fatalf("expected input variable snapshot, got %+v", response.Data.VariableSnapshot)
	}
	if len(response.Data.Trace) != 1 || response.Data.Trace[0].NodeID != "start" || response.Data.Trace[0].DurationMS != 12 {
		t.Fatalf("expected trace entry with duration, got %+v", response.Data.Trace)
	}
	if response.Data.Outputs["start"]["ticket"] != "INC-1" || response.Data.Performance.BottleneckNodeID != "start" || len(response.Data.Logs) != 0 {
		t.Fatalf("expected outputs/performance/logs in snapshot, got %+v", response.Data)
	}
}

func TestRegisterWorkflowRoutesDispatchesPausedFailureDecision(t *testing.T) {
	service := &workflowFakeService{
		executionDetail: &workflow.WorkflowExecution{ID: "wexec_1", WorkflowID: "workflow_1", Status: workflow.ExecutionStatusRunning},
	}
	handler := newWorkflowHandler(service)
	mux := stdhttp.NewServeMux()
	registerWorkflowRoutes(mux, passThroughAuthMiddleware{}, handler)

	request := workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/workflow_1/executions/wexec_1/decision", `{
		"action": "skip",
		"nodeId": "classify"
	}`)
	recorder := httptest.NewRecorder()

	mux.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.resolvedFailureExecutionID != "wexec_1" {
		t.Fatalf("expected decision for wexec_1, got %q", service.resolvedFailureExecutionID)
	}
	if service.resolvedFailureDecision.Action != workflow.FailureActionContinue || service.resolvedFailureDecision.NodeID != "classify" {
		t.Fatalf("unexpected failure decision: %+v", service.resolvedFailureDecision)
	}
}

func TestRegisterWorkflowRoutesDispatchesWorkflowCrudAndTestNode(t *testing.T) {
	service := &workflowFakeService{
		updated:        &workflow.WorkflowDefinition{ID: "workflow_1", Name: "Updated", Status: workflow.WorkflowStatusPublished},
		deleted:        &workflow.WorkflowDefinition{ID: "workflow_1", Status: workflow.WorkflowStatusArchived},
		testNodeResult: &workflow.TestNodeResult{WorkflowID: "workflow_1", NodeID: "start", Status: workflow.ExecutionStatusSucceeded},
	}
	handler := newWorkflowHandler(service)
	mux := stdhttp.NewServeMux()
	registerWorkflowRoutes(mux, passThroughAuthMiddleware{}, handler)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, workflowTestRequest(stdhttp.MethodPut, "/api/v1/workflows/workflow_1", `{"name":"Updated","definition":{"nodes":[{"id":"start"}]}}`))
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("PUT expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.updateReq.WorkflowID != "workflow_1" {
		t.Fatalf("expected updated workflow_1, got %+v", service.updateReq)
	}

	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, workflowTestRequest(stdhttp.MethodDelete, "/api/v1/workflows/workflow_1", ""))
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("DELETE expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.workflowID != "workflow_1" {
		t.Fatalf("expected deleted workflow_1, got %q", service.workflowID)
	}

	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/workflow_1/test-node", `{"nodeId":"start","input":{"dryRun":true}}`))
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("test-node expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.workflowID != "workflow_1" || service.testNodeID != "start" {
		t.Fatalf("expected test node workflow_1/start, got workflow=%q node=%q", service.workflowID, service.testNodeID)
	}
}

func TestRegisterWorkflowRoutesDispatchesVersionHistoryAndRollback(t *testing.T) {
	service := &workflowFakeService{
		rolledBack: &workflow.WorkflowDefinition{ID: "workflow_1", Name: "Rolled back", Status: workflow.WorkflowStatusDraft, Version: 4},
		versions: []*workflow.WorkflowDefinition{
			{ID: "workflow_1", Name: "Original", Status: workflow.WorkflowStatusDraft, Version: 1},
			{ID: "workflow_1", Name: "Published", Status: workflow.WorkflowStatusPublished, Version: 2},
		},
	}
	handler := newWorkflowHandler(service)
	mux := stdhttp.NewServeMux()
	registerWorkflowRoutes(mux, passThroughAuthMiddleware{}, handler)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, workflowTestRequest(stdhttp.MethodGet, "/api/v1/workflows/workflow_1/versions", ""))
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("versions expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.workflowID != "workflow_1" {
		t.Fatalf("expected versions for workflow_1, got %q", service.workflowID)
	}

	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/workflow_1/rollback", `{"version":1}`))
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("rollback expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.rollbackReq.WorkflowID != "workflow_1" || service.rollbackReq.Version != 1 {
		t.Fatalf("expected rollback workflow_1 version 1, got %+v", service.rollbackReq)
	}
}

func TestRegisterWorkflowRoutesDispatchesCreateWorkflowBranch(t *testing.T) {
	service := &workflowFakeService{
		branched: &workflow.WorkflowDefinition{
			ID:      "workflow_branch",
			Name:    "Incident Router Branch",
			Status:  workflow.WorkflowStatusDraft,
			Version: 1,
		},
	}
	handler := newWorkflowHandler(service)
	mux := stdhttp.NewServeMux()
	registerWorkflowRoutes(mux, passThroughAuthMiddleware{}, handler)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/workflow_1/branches", `{
		"name": "Incident Router Branch",
		"description": "Experiment B",
		"version": 2,
		"experimentKey": "routing-copy-v2",
		"trafficPercent": 25
	}`))

	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("branches expected 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.branchReq.WorkflowID != "workflow_1" || service.branchReq.Version != 2 {
		t.Fatalf("expected branch workflow_1 version 2, got %+v", service.branchReq)
	}
	if service.branchReq.Name != "Incident Router Branch" || service.branchReq.Description != "Experiment B" {
		t.Fatalf("unexpected branch copy: %+v", service.branchReq)
	}
	if service.branchReq.ExperimentKey != "routing-copy-v2" || service.branchReq.TrafficPercent != 25 {
		t.Fatalf("unexpected branch experiment controls: %+v", service.branchReq)
	}
}

func TestRegisterWorkflowRoutesDispatchesBranchPublishAndMerge(t *testing.T) {
	service := &workflowFakeService{
		publishedBranch: &workflow.WorkflowDefinition{
			ID:      "workflow_published_branch",
			Name:    "Published Branch",
			Status:  workflow.WorkflowStatusPublished,
			Version: 1,
		},
		mergedBranch: &workflow.WorkflowDefinition{
			ID:      "workflow_1",
			Name:    "Incident Router",
			Status:  workflow.WorkflowStatusPublished,
			Version: 3,
		},
	}
	handler := newWorkflowHandler(service)
	mux := stdhttp.NewServeMux()
	registerWorkflowRoutes(mux, passThroughAuthMiddleware{}, handler)

	publishRecorder := httptest.NewRecorder()
	mux.ServeHTTP(publishRecorder, workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/workflow_1/branches/workflow_branch/publish", `{
		"name": "Published Branch"
	}`))
	if publishRecorder.Code != stdhttp.StatusCreated {
		t.Fatalf("publish branch expected 201, got %d with body %s", publishRecorder.Code, publishRecorder.Body.String())
	}
	if service.publishBranchReq.BranchID != "workflow_branch" || service.publishBranchReq.Name != "Published Branch" {
		t.Fatalf("unexpected publish branch request: %+v", service.publishBranchReq)
	}

	mergeRecorder := httptest.NewRecorder()
	mux.ServeHTTP(mergeRecorder, workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/workflow_1/branches/workflow_branch/merge", ""))
	if mergeRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("merge branch expected 200, got %d with body %s", mergeRecorder.Code, mergeRecorder.Body.String())
	}
	if service.mergeBranchReq.BranchID != "workflow_branch" {
		t.Fatalf("unexpected merge branch request: %+v", service.mergeBranchReq)
	}
}

type passThroughAuthMiddleware struct{}

func (passThroughAuthMiddleware) requireSession(next stdhttp.Handler) stdhttp.Handler {
	return next
}

type recordingWorkflowAuthMiddleware struct {
	calls int
}

func (m *recordingWorkflowAuthMiddleware) requireSession(next stdhttp.Handler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		m.calls++
		request := r.WithContext(context.WithValue(r.Context(), sessionContextKey, auth.Session{
			OrganizationID: "org_1",
			User:           auth.User{ID: "user_1"},
			WorkspaceID:    "workspace_1",
		}))
		next.ServeHTTP(w, request)
	})
}
