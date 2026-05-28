package http

import (
	"context"
	"database/sql"
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"oblivious/server/internal/agent"
)

func prepareHTTPAgentWorkflowState(t *testing.T, database *sql.DB, userID, organizationID string) (*agent.Run, *agent.ToolRun, *agent.ToolRun) {
	t.Helper()

	migration, err := os.ReadFile("../../migrations/0031_agent_workflow_runs.sql")
	if err != nil {
		t.Fatalf("read agent workflow migration: %v", err)
	}
	if _, err := database.Exec(string(migration)); err != nil {
		t.Fatalf("apply agent workflow migration: %v", err)
	}

	store := agent.NewSQLStore(database)
	ctx := context.Background()
	ag, err := store.CreateAgent(ctx, userID, organizationID, &agent.CreateAgentRequest{
		Name: "Durable HTTP Agent",
		Tools: []agent.Tool{
			{Name: "datetime", Type: "builtin", Enabled: true, RequiresApproval: true},
		},
	})
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	conv, err := store.CreateConversation(ctx, ag.ID, userID, organizationID, "Durable HTTP Conversation")
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	run, err := store.CreateRun(ctx, &agent.CreateRunRequest{
		OrganizationID: organizationID,
		ConversationID: conv.ID,
		AgentID:        ag.ID,
		UserID:         userID,
		RequestID:      "req_http_agent_run",
		Status:         agent.RunStatusPendingApproval,
		MemoryEnabled:  true,
		MemorySearched: true,
	})
	if err != nil {
		t.Fatalf("create agent run: %v", err)
	}
	pending, err := store.CreateToolRun(ctx, &agent.CreateToolRunRequest{
		OrganizationID: organizationID,
		RunID:          run.ID,
		ConversationID: conv.ID,
		AgentID:        ag.ID,
		ToolCallID:     "call_pending_approval",
		ToolName:       "datetime",
		ToolType:       "builtin",
		Arguments:      map[string]any{},
		Status:         agent.ToolRunStatusPendingApproval,
		ApprovalStatus: agent.ApprovalStatusPending,
	})
	if err != nil {
		t.Fatalf("create pending tool run: %v", err)
	}
	failed, err := store.CreateToolRun(ctx, &agent.CreateToolRunRequest{
		OrganizationID: organizationID,
		RunID:          run.ID,
		ConversationID: conv.ID,
		AgentID:        ag.ID,
		ToolCallID:     "call_failed_retry",
		ToolName:       "datetime",
		ToolType:       "builtin",
		Arguments:      map[string]any{},
		Status:         agent.ToolRunStatusFailed,
		ApprovalStatus: agent.ApprovalStatusNotRequired,
		AttemptCount:   1,
		Error:          "first attempt failed",
	})
	if err != nil {
		t.Fatalf("create failed tool run: %v", err)
	}
	return run, pending, failed
}

func TestAgentRunStatusEndpointsExposeTenantScopedRunDetail(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)
	cookie, _, userID := registerHTTPUser(t, router, "agent-runs@example.com")
	_, organizationID := queryHTTPUserScope(t, database, userID)
	run, pendingToolRun, _ := prepareHTTPAgentWorkflowState(t, database, userID, organizationID)

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/agents/conversations/"+run.ConversationID+"/runs", nil)
	listRequest.AddCookie(cookie)
	router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("list runs expected 200, got %d with body %s", listRecorder.Code, listRecorder.Body.String())
	}
	var listResponse struct {
		Data []agent.Run `json:"data"`
	}
	if err := json.Unmarshal(listRecorder.Body.Bytes(), &listResponse); err != nil {
		t.Fatalf("decode list runs response: %v", err)
	}
	if len(listResponse.Data) != 1 || listResponse.Data[0].ID != run.ID || listResponse.Data[0].OrganizationID != organizationID {
		t.Fatalf("expected tenant-scoped run list, got %+v", listResponse.Data)
	}

	detailRecorder := httptest.NewRecorder()
	detailRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/agents/runs/"+run.ID, nil)
	detailRequest.AddCookie(cookie)
	router.ServeHTTP(detailRecorder, detailRequest)
	if detailRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("run detail expected 200, got %d with body %s", detailRecorder.Code, detailRecorder.Body.String())
	}
	var detailResponse struct {
		Data struct {
			Run      agent.Run       `json:"run"`
			ToolRuns []agent.ToolRun `json:"toolRuns"`
		} `json:"data"`
	}
	if err := json.Unmarshal(detailRecorder.Body.Bytes(), &detailResponse); err != nil {
		t.Fatalf("decode run detail response: %v", err)
	}
	if detailResponse.Data.Run.ID != run.ID || len(detailResponse.Data.ToolRuns) != 2 {
		t.Fatalf("expected run detail with two tool runs, got %+v", detailResponse.Data)
	}
	if detailResponse.Data.ToolRuns[0].ID != pendingToolRun.ID && detailResponse.Data.ToolRuns[1].ID != pendingToolRun.ID {
		t.Fatalf("expected pending tool run in detail payload, got %+v", detailResponse.Data.ToolRuns)
	}

	otherCookie, _, _ := registerHTTPUser(t, router, "agent-runs-other@example.com")
	crossRecorder := httptest.NewRecorder()
	crossRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/agents/runs/"+run.ID, nil)
	crossRequest.AddCookie(otherCookie)
	router.ServeHTTP(crossRecorder, crossRequest)
	if crossRecorder.Code != stdhttp.StatusNotFound && crossRecorder.Code != stdhttp.StatusForbidden {
		t.Fatalf("cross-tenant run detail expected 404/403, got %d with body %s", crossRecorder.Code, crossRecorder.Body.String())
	}
	if strings.Contains(crossRecorder.Body.String(), run.ID) {
		t.Fatalf("cross-tenant response leaked run id: %s", crossRecorder.Body.String())
	}
}

func TestAgentToolRunApprovalRejectRetryEndpointsAreTenantScoped(t *testing.T) {
	database := testDatabase(t)
	router := NewRouter(testConfig(), database)
	cookie, csrfToken, userID := registerHTTPUser(t, router, "agent-tool-runs@example.com")
	_, organizationID := queryHTTPUserScope(t, database, userID)
	_, pendingToolRun, failedToolRun := prepareHTTPAgentWorkflowState(t, database, userID, organizationID)

	approveRecorder := httptest.NewRecorder()
	approveRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/agents/tool-runs/"+pendingToolRun.ID+"/approve", strings.NewReader(`{"reason":"reviewed"}`))
	approveRequest.Header.Set("Content-Type", "application/json")
	approveRequest.AddCookie(cookie)
	addCSRF(approveRequest, csrfToken)
	router.ServeHTTP(approveRecorder, approveRequest)
	if approveRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("approve tool run expected 200, got %d with body %s", approveRecorder.Code, approveRecorder.Body.String())
	}
	var approveResponse struct {
		Data agent.ToolRun `json:"data"`
	}
	if err := json.Unmarshal(approveRecorder.Body.Bytes(), &approveResponse); err != nil {
		t.Fatalf("decode approve response: %v", err)
	}
	if approveResponse.Data.ApprovalStatus != agent.ApprovalStatusApproved || approveResponse.Data.ApprovedByUserID != userID {
		t.Fatalf("expected approved tool run with acting user, got %+v", approveResponse.Data)
	}

	rejectRun, rejectToolRun, _ := prepareHTTPAgentWorkflowState(t, database, userID, organizationID)
	_ = rejectRun
	rejectRecorder := httptest.NewRecorder()
	rejectRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/agents/tool-runs/"+rejectToolRun.ID+"/reject", strings.NewReader(`{"reason":"unsafe"}`))
	rejectRequest.Header.Set("Content-Type", "application/json")
	rejectRequest.AddCookie(cookie)
	addCSRF(rejectRequest, csrfToken)
	router.ServeHTTP(rejectRecorder, rejectRequest)
	if rejectRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("reject tool run expected 200, got %d with body %s", rejectRecorder.Code, rejectRecorder.Body.String())
	}
	var rejectResponse struct {
		Data agent.ToolRun `json:"data"`
	}
	if err := json.Unmarshal(rejectRecorder.Body.Bytes(), &rejectResponse); err != nil {
		t.Fatalf("decode reject response: %v", err)
	}
	if rejectResponse.Data.Status != agent.ToolRunStatusRejected || rejectResponse.Data.ApprovalStatus != agent.ApprovalStatusRejected || rejectResponse.Data.ApprovalDecisionReason != "unsafe" {
		t.Fatalf("expected rejected tool run with reason, got %+v", rejectResponse.Data)
	}

	retryRecorder := httptest.NewRecorder()
	retryRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/agents/tool-runs/"+failedToolRun.ID+"/retry", nil)
	retryRequest.AddCookie(cookie)
	addCSRF(retryRequest, csrfToken)
	router.ServeHTTP(retryRecorder, retryRequest)
	if retryRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("retry tool run expected 200, got %d with body %s", retryRecorder.Code, retryRecorder.Body.String())
	}
	var retryResponse struct {
		Data agent.ToolRun `json:"data"`
	}
	if err := json.Unmarshal(retryRecorder.Body.Bytes(), &retryResponse); err != nil {
		t.Fatalf("decode retry response: %v", err)
	}
	if retryResponse.Data.Status != agent.ToolRunStatusRunning || retryResponse.Data.AttemptCount != 2 || retryResponse.Data.Error != "" {
		t.Fatalf("expected retry attempt evidence, got %+v", retryResponse.Data)
	}

	completedRetryRecorder := httptest.NewRecorder()
	completedRetryRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/agents/tool-runs/"+pendingToolRun.ID+"/retry", nil)
	completedRetryRequest.AddCookie(cookie)
	addCSRF(completedRetryRequest, csrfToken)
	router.ServeHTTP(completedRetryRecorder, completedRetryRequest)
	if completedRetryRecorder.Code != stdhttp.StatusConflict && completedRetryRecorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("retry non-failed tool run expected 409/400, got %d with body %s", completedRetryRecorder.Code, completedRetryRecorder.Body.String())
	}

	otherCookie, otherCSRF, _ := registerHTTPUser(t, router, "agent-tool-runs-other@example.com")
	crossRecorder := httptest.NewRecorder()
	crossRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/agents/tool-runs/"+failedToolRun.ID+"/retry", nil)
	crossRequest.AddCookie(otherCookie)
	addCSRF(crossRequest, otherCSRF)
	router.ServeHTTP(crossRecorder, crossRequest)
	if crossRecorder.Code != stdhttp.StatusNotFound && crossRecorder.Code != stdhttp.StatusForbidden {
		t.Fatalf("cross-tenant retry expected 404/403, got %d with body %s", crossRecorder.Code, crossRecorder.Body.String())
	}
	if strings.Contains(crossRecorder.Body.String(), failedToolRun.ID) {
		t.Fatalf("cross-tenant response leaked tool run id: %s", crossRecorder.Body.String())
	}
}
