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
	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
)

func TestAgentHandlerCreateRejectsInvalidDefaultExecutionModeAsBadRequest(t *testing.T) {
	handler := newAgentHandler(agent.NewService(nil, nil))
	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/app/agents",
		strings.NewReader(`{"name":"Bad mode","config":{"defaultExecutionMode":"manual"}}`),
	).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		User:           auth.User{ID: "user_1"},
	}))
	recorder := httptest.NewRecorder()

	handler.createAgent(recorder, request)

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"code":"invalid_request"`) || !strings.Contains(recorder.Body.String(), "defaultExecutionMode must be react or planning") {
		t.Fatalf("expected default execution mode validation response, got %s", recorder.Body.String())
	}
}

func TestAgentHandlerUpdateRejectsInvalidDefaultExecutionModeAsBadRequest(t *testing.T) {
	store := &agentHTTPConfigStore{agent: &agent.Agent{
		ID:             "agent_1",
		OrganizationID: "org_1",
		UserID:         "user_1",
		Name:           "Existing agent",
		Config:         agent.Config{DefaultExecutionMode: agent.ExecutionModeReact},
	}}
	handler := newAgentHandler(agent.NewService(store, nil))
	request := httptest.NewRequest(
		stdhttp.MethodPut,
		"/api/v1/app/agents/agent_1",
		strings.NewReader(`{"config":{"defaultExecutionMode":"manual"}}`),
	).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		User:           auth.User{ID: "user_1"},
	}))
	recorder := httptest.NewRecorder()

	handler.updateAgent(recorder, request, "agent_1")

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"code":"invalid_request"`) || !strings.Contains(recorder.Body.String(), "defaultExecutionMode must be react or planning") {
		t.Fatalf("expected default execution mode validation response, got %s", recorder.Body.String())
	}
}

func TestAgentHandlerSendMessageRejectsInvalidOverrideModeAsBadRequest(t *testing.T) {
	store := &agentHTTPConfigStore{
		agent: &agent.Agent{
			ID:             "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
			Model:          "gpt-4o-mini",
		},
		conversation: &agent.Conversation{
			ID:             "conv_1",
			AgentID:        "agent_1",
			OrganizationID: "org_1",
			UserID:         "user_1",
		},
	}
	handler := newAgentHandler(agent.NewService(store, nil))
	request := httptest.NewRequest(
		stdhttp.MethodPost,
		"/api/v1/app/agents/conversations/conv_1/messages",
		strings.NewReader(`{"content":"hello","mode":"manual"}`),
	).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		User:           auth.User{ID: "user_1"},
	}))
	recorder := httptest.NewRecorder()

	handler.sendMessage(recorder, request, "conv_1")

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"code":"invalid_request"`) || !strings.Contains(recorder.Body.String(), "mode must be react or planning") {
		t.Fatalf("expected mode validation response, got %s", recorder.Body.String())
	}
	if len(store.messages) != 0 {
		t.Fatalf("expected invalid mode to stop before message persistence, got %+v", store.messages)
	}
}

func TestAgentHandlerSendMessageAcceptsCamelAndSnakeExecutionOverrides(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "camel",
			body: `{"content":"hello","mode":"react","maxIterations":1,"tokenBudget":1000}`,
		},
		{
			name: "snake",
			body: `{"content":"hello","mode":"react","max_iterations":1,"token_budget":1000}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &agentHTTPConfigStore{
				agent: &agent.Agent{
					ID:             "agent_1",
					OrganizationID: "org_1",
					UserID:         "user_1",
					Model:          "gpt-4o-mini",
				},
				conversation: &agent.Conversation{
					ID:             "conv_1",
					AgentID:        "agent_1",
					OrganizationID: "org_1",
					UserID:         "user_1",
				},
			}
			handler := newAgentHandler(agent.NewService(store, &agentHTTPFakeGateway{reply: "ok"}))
			request := httptest.NewRequest(
				stdhttp.MethodPost,
				"/api/v1/app/agents/conversations/conv_1/messages",
				strings.NewReader(tt.body),
			).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
				OrganizationID: "org_1",
				User:           auth.User{ID: "user_1"},
			}))
			recorder := httptest.NewRecorder()

			handler.sendMessage(recorder, request, "conv_1")

			if recorder.Code != stdhttp.StatusOK {
				t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
			}
			if len(store.messages) != 2 || store.messages[1].Content != "ok" {
				t.Fatalf("expected user and assistant messages, got %+v", store.messages)
			}
		})
	}
}

type agentHTTPConfigStore struct {
	agent        *agent.Agent
	conversation *agent.Conversation
	messages     []*agent.Message
}

func (s *agentHTTPConfigStore) CreateAgent(ctx context.Context, userID, organizationID string, req *agent.CreateAgentRequest) (*agent.Agent, error) {
	return nil, nil
}

func (s *agentHTTPConfigStore) GetAgent(ctx context.Context, id, organizationID string) (*agent.Agent, error) {
	if s.agent != nil && s.agent.ID == id && s.agent.OrganizationID == organizationID {
		return s.agent, nil
	}
	return nil, nil
}

func (s *agentHTTPConfigStore) ListAgents(ctx context.Context, userID, organizationID string) ([]*agent.Agent, error) {
	return nil, nil
}

func (s *agentHTTPConfigStore) UpdateAgent(ctx context.Context, id, organizationID string, req *agent.UpdateAgentRequest) (*agent.Agent, error) {
	return s.agent, nil
}

func (s *agentHTTPConfigStore) DeleteAgent(ctx context.Context, id, organizationID string) error {
	return nil
}

func (s *agentHTTPConfigStore) CreateConversation(ctx context.Context, agentID, userID, organizationID string, title string) (*agent.Conversation, error) {
	return nil, nil
}

func (s *agentHTTPConfigStore) GetConversation(ctx context.Context, id, organizationID string) (*agent.Conversation, error) {
	if s.conversation != nil && s.conversation.ID == id && s.conversation.OrganizationID == organizationID {
		return s.conversation, nil
	}
	return nil, nil
}

func (s *agentHTTPConfigStore) ListConversations(ctx context.Context, agentID, userID, organizationID string) ([]*agent.Conversation, error) {
	return nil, nil
}

func (s *agentHTTPConfigStore) DeleteConversation(ctx context.Context, id, organizationID string) error {
	return nil
}

func (s *agentHTTPConfigStore) CreateMessage(ctx context.Context, conversationID, organizationID, role, content string, toolCalls []agent.ToolCall, toolCallID string) (*agent.Message, error) {
	msg := &agent.Message{
		ID:             role + "_message",
		ConversationID: conversationID,
		OrganizationID: organizationID,
		Role:           role,
		Content:        content,
		ToolCalls:      append([]agent.ToolCall(nil), toolCalls...),
		ToolCallID:     toolCallID,
	}
	s.messages = append(s.messages, msg)
	return msg, nil
}

func (s *agentHTTPConfigStore) ListMessages(ctx context.Context, conversationID, organizationID string) ([]*agent.Message, error) {
	var messages []*agent.Message
	for _, msg := range s.messages {
		if msg.ConversationID == conversationID && msg.OrganizationID == organizationID {
			messages = append(messages, msg)
		}
	}
	return messages, nil
}

func (s *agentHTTPConfigStore) CreateRun(ctx context.Context, req *agent.CreateRunRequest) (*agent.Run, error) {
	return nil, nil
}

func (s *agentHTTPConfigStore) GetRun(ctx context.Context, organizationID, id string) (*agent.Run, error) {
	return nil, nil
}

func (s *agentHTTPConfigStore) ListRuns(ctx context.Context, organizationID, conversationID string) ([]*agent.Run, error) {
	return nil, nil
}

func (s *agentHTTPConfigStore) UpdateRun(ctx context.Context, organizationID, id string, req agent.UpdateRunRequest) (*agent.Run, error) {
	return nil, nil
}

func (s *agentHTTPConfigStore) CreateToolRun(ctx context.Context, req *agent.CreateToolRunRequest) (*agent.ToolRun, error) {
	return nil, nil
}

func (s *agentHTTPConfigStore) GetToolRun(ctx context.Context, organizationID, id string) (*agent.ToolRun, error) {
	return nil, nil
}

func (s *agentHTTPConfigStore) ListToolRuns(ctx context.Context, organizationID, runID string) ([]*agent.ToolRun, error) {
	return nil, nil
}

func (s *agentHTTPConfigStore) UpdateToolRun(ctx context.Context, organizationID, id string, req agent.UpdateToolRunRequest) (*agent.ToolRun, error) {
	return nil, nil
}

func (s *agentHTTPConfigStore) CreatePlanStep(ctx context.Context, req *agent.CreatePlanStepRequest) (*agent.PlanStep, error) {
	return nil, nil
}

func (s *agentHTTPConfigStore) GetPlanStep(ctx context.Context, organizationID, id string) (*agent.PlanStep, error) {
	return nil, nil
}

func (s *agentHTTPConfigStore) ListPlanSteps(ctx context.Context, organizationID, runID string) ([]*agent.PlanStep, error) {
	return nil, nil
}

func (s *agentHTTPConfigStore) UpdatePlanStep(ctx context.Context, organizationID, id string, req agent.UpdatePlanStepRequest) (*agent.PlanStep, error) {
	return nil, nil
}

func (s *agentHTTPConfigStore) DeletePlanStep(ctx context.Context, organizationID, id string) (*agent.PlanStep, error) {
	return nil, nil
}

func (s *agentHTTPConfigStore) CreateMemory(ctx context.Context, req *agent.CreateMemoryStoreRequest) (*agent.Memory, error) {
	return nil, nil
}

func (s *agentHTTPConfigStore) ListMemories(ctx context.Context, organizationID, userID string, req agent.ListMemoriesRequest) ([]*agent.Memory, error) {
	return nil, nil
}

type agentHTTPFakeGateway struct {
	reply string
}

func (g *agentHTTPFakeGateway) GenerateReply(ctx context.Context, messages []chat.Message, config chat.ConversationConfig) (string, error) {
	return g.reply, nil
}

func (g *agentHTTPFakeGateway) GenerateReplyStream(ctx context.Context, messages []chat.Message, config chat.ConversationConfig, onChunk func(string) error) error {
	return onChunk(g.reply)
}

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
	if retryResponse.Data.Status != agent.ToolRunStatusCompleted || retryResponse.Data.AttemptCount != 2 || retryResponse.Data.Error != "" || retryResponse.Data.ResultContent == "" {
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

	completedApproveRecorder := httptest.NewRecorder()
	completedApproveRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/agents/tool-runs/"+pendingToolRun.ID+"/approve", strings.NewReader(`{"reason":"second approval"}`))
	completedApproveRequest.Header.Set("Content-Type", "application/json")
	completedApproveRequest.AddCookie(cookie)
	addCSRF(completedApproveRequest, csrfToken)
	router.ServeHTTP(completedApproveRecorder, completedApproveRequest)
	if completedApproveRecorder.Code != stdhttp.StatusConflict {
		t.Fatalf("approve completed tool run expected 409, got %d with body %s", completedApproveRecorder.Code, completedApproveRecorder.Body.String())
	}

	completedRejectRecorder := httptest.NewRecorder()
	completedRejectRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/agents/tool-runs/"+pendingToolRun.ID+"/reject", strings.NewReader(`{"reason":"late reject"}`))
	completedRejectRequest.Header.Set("Content-Type", "application/json")
	completedRejectRequest.AddCookie(cookie)
	addCSRF(completedRejectRequest, csrfToken)
	router.ServeHTTP(completedRejectRecorder, completedRejectRequest)
	if completedRejectRecorder.Code != stdhttp.StatusConflict {
		t.Fatalf("reject completed tool run expected 409, got %d with body %s", completedRejectRecorder.Code, completedRejectRecorder.Body.String())
	}

	rejectedApproveRecorder := httptest.NewRecorder()
	rejectedApproveRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/agents/tool-runs/"+rejectToolRun.ID+"/approve", strings.NewReader(`{"reason":"override rejection"}`))
	rejectedApproveRequest.Header.Set("Content-Type", "application/json")
	rejectedApproveRequest.AddCookie(cookie)
	addCSRF(rejectedApproveRequest, csrfToken)
	router.ServeHTTP(rejectedApproveRecorder, rejectedApproveRequest)
	if rejectedApproveRecorder.Code != stdhttp.StatusConflict {
		t.Fatalf("approve rejected tool run expected 409, got %d with body %s", rejectedApproveRecorder.Code, rejectedApproveRecorder.Body.String())
	}

	var guardedStatus, guardedApprovalStatus, guardedApprovedBy, guardedReason string
	if err := database.QueryRow(`
		SELECT status, approval_status, COALESCE(approved_by_user_id, ''), approval_decision_reason
		FROM agent_tool_runs
		WHERE id = $1
	`, pendingToolRun.ID).Scan(&guardedStatus, &guardedApprovalStatus, &guardedApprovedBy, &guardedReason); err != nil {
		t.Fatalf("query guarded completed tool run: %v", err)
	}
	if guardedStatus != agent.ToolRunStatusCompleted || guardedApprovalStatus != agent.ApprovalStatusApproved || guardedApprovedBy != userID || guardedReason != "reviewed" {
		t.Fatalf("completed approval bypass mutated tool run state: status=%q approval=%q approvedBy=%q reason=%q", guardedStatus, guardedApprovalStatus, guardedApprovedBy, guardedReason)
	}

	var rejectedStatus, rejectedApprovalStatus, rejectedReason string
	if err := database.QueryRow(`
		SELECT status, approval_status, approval_decision_reason
		FROM agent_tool_runs
		WHERE id = $1
	`, rejectToolRun.ID).Scan(&rejectedStatus, &rejectedApprovalStatus, &rejectedReason); err != nil {
		t.Fatalf("query guarded rejected tool run: %v", err)
	}
	if rejectedStatus != agent.ToolRunStatusRejected || rejectedApprovalStatus != agent.ApprovalStatusRejected || rejectedReason != "unsafe" {
		t.Fatalf("rejected approval bypass mutated tool run state: status=%q approval=%q reason=%q", rejectedStatus, rejectedApprovalStatus, rejectedReason)
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
