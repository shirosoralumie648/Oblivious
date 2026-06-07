package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
	"oblivious/server/internal/workflow"
)

type workflowFakeService struct {
	cancelledExecutionID        string
	checkedExecutionID          string
	created                     *workflow.WorkflowDefinition
	createReq                   workflow.CreateWorkflowRequest
	deleted                     *workflow.WorkflowDefinition
	executions                  []*workflow.WorkflowExecution
	executionDetail             *workflow.WorkflowExecution
	debugSnapshot               *workflow.ExecutionDebugSnapshot
	executionInput              map[string]any
	executionContext            map[string]any
	executionTriggerPayload     map[string]any
	executionTriggerType        workflow.WorkflowTriggerType
	startRequests               []workflow.StartExecutionRequest
	runUntilBlockedExecutionID  string
	runUntilBlockedExecutionIDs []string
	runUntilBlockedResult       *workflow.WorkflowExecution
	runUntilBlockedErr          error
	listed                      []*workflow.WorkflowDefinition
	conversationMatches         []workflow.ConversationTriggerMatch
	conversationMatchReq        workflow.MatchConversationTriggersRequest
	semanticMatches             []workflow.SemanticTriggerMatch
	semanticMatchReq            workflow.MatchSemanticTriggersRequest
	organizationID              string
	pausedExecutionID           string
	resourceUsage               workflow.WorkflowResourceUsage
	resolvedFailureDecision     workflow.ResolveFailureDecisionRequest
	resolvedFailureExecutionID  string
	resumedExecutionID          string
	resumedExecutionRequest     workflow.ResumeExecutionRequest
	startCalls                  int
	started                     *workflow.WorkflowExecution
	testNodeInput               map[string]any
	testNodeID                  string
	testNodeResult              *workflow.TestNodeResult
	updated                     *workflow.WorkflowDefinition
	updateReq                   workflow.UpdateWorkflowRequest
	versions                    []*workflow.WorkflowDefinition
	workflowDetail              *workflow.WorkflowDefinition
	workflowID                  string
	branchReq                   workflow.CreateWorkflowBranchRequest
	branched                    *workflow.WorkflowDefinition
	rollbackReq                 workflow.RollbackWorkflowRequest
	rolledBack                  *workflow.WorkflowDefinition

	createErr         error
	getErr            error
	listErr           error
	listVersionsErr   error
	listExecutionsErr error
	semanticMatchErr  error
	getExecutionErr   error
	debugSnapshotErr  error
	startErr          error
	pauseErr          error
	resourceCheckErr  error
	resolveFailureErr error
	resumeErr         error
	cancelErr         error
	deleteErr         error
	testNodeErr       error
	updateErr         error
	rollbackErr       error
	branchErr         error
}

func (s *workflowFakeService) CreateWorkflow(ctx context.Context, session auth.Session, req workflow.CreateWorkflowRequest) (*workflow.WorkflowDefinition, error) {
	s.organizationID = session.OrganizationID
	s.createReq = req
	return s.created, s.createErr
}

func (s *workflowFakeService) ListWorkflows(ctx context.Context, session auth.Session) ([]*workflow.WorkflowDefinition, error) {
	s.organizationID = session.OrganizationID
	return s.listed, s.listErr
}

func (s *workflowFakeService) MatchConversationTriggers(ctx context.Context, session auth.Session, req workflow.MatchConversationTriggersRequest) ([]workflow.ConversationTriggerMatch, error) {
	s.organizationID = session.OrganizationID
	s.conversationMatchReq = req
	return s.conversationMatches, nil
}

func (s *workflowFakeService) MatchSemanticTriggers(ctx context.Context, session auth.Session, req workflow.MatchSemanticTriggersRequest) ([]workflow.SemanticTriggerMatch, error) {
	s.organizationID = session.OrganizationID
	s.semanticMatchReq = req
	return s.semanticMatches, s.semanticMatchErr
}

func (s *workflowFakeService) GetWorkflow(ctx context.Context, session auth.Session, workflowID string) (*workflow.WorkflowDefinition, error) {
	s.organizationID = session.OrganizationID
	s.workflowID = workflowID
	return s.workflowDetail, s.getErr
}

func (s *workflowFakeService) ListWorkflowVersions(ctx context.Context, session auth.Session, workflowID string) ([]*workflow.WorkflowDefinition, error) {
	s.organizationID = session.OrganizationID
	s.workflowID = workflowID
	return s.versions, s.listVersionsErr
}

func (s *workflowFakeService) UpdateWorkflow(ctx context.Context, session auth.Session, req workflow.UpdateWorkflowRequest) (*workflow.WorkflowDefinition, error) {
	s.organizationID = session.OrganizationID
	s.updateReq = req
	return s.updated, s.updateErr
}

func (s *workflowFakeService) RollbackWorkflow(ctx context.Context, session auth.Session, req workflow.RollbackWorkflowRequest) (*workflow.WorkflowDefinition, error) {
	s.organizationID = session.OrganizationID
	s.rollbackReq = req
	return s.rolledBack, s.rollbackErr
}

func (s *workflowFakeService) CreateWorkflowBranch(ctx context.Context, session auth.Session, req workflow.CreateWorkflowBranchRequest) (*workflow.WorkflowDefinition, error) {
	s.organizationID = session.OrganizationID
	s.branchReq = req
	return s.branched, s.branchErr
}

func (s *workflowFakeService) DeleteWorkflow(ctx context.Context, session auth.Session, workflowID string) (*workflow.WorkflowDefinition, error) {
	s.organizationID = session.OrganizationID
	s.workflowID = workflowID
	return s.deleted, s.deleteErr
}

func (s *workflowFakeService) StartExecution(ctx context.Context, session auth.Session, workflowID string, input map[string]any) (*workflow.WorkflowExecution, error) {
	s.startCalls++
	s.organizationID = session.OrganizationID
	s.workflowID = workflowID
	s.executionInput = input
	return s.started, s.startErr
}

func (s *workflowFakeService) StartExecutionWithTrigger(ctx context.Context, session auth.Session, req workflow.StartExecutionRequest) (*workflow.WorkflowExecution, error) {
	s.startCalls++
	s.organizationID = session.OrganizationID
	s.workflowID = req.WorkflowID
	s.executionInput = req.Input
	s.executionTriggerType = req.TriggerType
	s.executionTriggerPayload = req.TriggerPayload
	s.executionContext = req.Context
	s.startRequests = append(s.startRequests, req)
	return s.started, s.startErr
}

func (s *workflowFakeService) RunExecutionUntilBlocked(ctx context.Context, session auth.Session, executionID string) (*workflow.WorkflowExecution, error) {
	s.organizationID = session.OrganizationID
	s.runUntilBlockedExecutionID = executionID
	s.runUntilBlockedExecutionIDs = append(s.runUntilBlockedExecutionIDs, executionID)
	return s.runUntilBlockedResult, s.runUntilBlockedErr
}

func (s *workflowFakeService) TestNode(ctx context.Context, session auth.Session, workflowID string, nodeID string, input map[string]any) (*workflow.TestNodeResult, error) {
	s.organizationID = session.OrganizationID
	s.workflowID = workflowID
	s.testNodeID = nodeID
	s.testNodeInput = input
	return s.testNodeResult, s.testNodeErr
}

func (s *workflowFakeService) ListExecutions(ctx context.Context, session auth.Session, workflowID string) ([]*workflow.WorkflowExecution, error) {
	s.organizationID = session.OrganizationID
	s.workflowID = workflowID
	return s.executions, s.listExecutionsErr
}

func (s *workflowFakeService) GetExecution(ctx context.Context, session auth.Session, executionID string) (*workflow.WorkflowExecution, error) {
	s.organizationID = session.OrganizationID
	s.pausedExecutionID = executionID
	return s.executionDetail, s.getExecutionErr
}

func (s *workflowFakeService) BuildExecutionDebugSnapshot(ctx context.Context, session auth.Session, executionID string) (*workflow.ExecutionDebugSnapshot, error) {
	s.organizationID = session.OrganizationID
	s.pausedExecutionID = executionID
	return s.debugSnapshot, s.debugSnapshotErr
}

func (s *workflowFakeService) CheckResourceLimits(ctx context.Context, session auth.Session, workflowID string, executionID string, usage workflow.WorkflowResourceUsage) (*workflow.WorkflowExecution, error) {
	s.organizationID = session.OrganizationID
	s.workflowID = workflowID
	s.checkedExecutionID = executionID
	s.resourceUsage = usage
	return s.executionDetail, s.resourceCheckErr
}

func (s *workflowFakeService) ResolvePausedFailure(ctx context.Context, session auth.Session, executionID string, req workflow.ResolveFailureDecisionRequest) (*workflow.WorkflowExecution, error) {
	s.organizationID = session.OrganizationID
	s.resolvedFailureExecutionID = executionID
	s.resolvedFailureDecision = req
	return s.executionDetail, s.resolveFailureErr
}

func (s *workflowFakeService) PauseExecution(ctx context.Context, session auth.Session, executionID string) (*workflow.WorkflowExecution, error) {
	s.organizationID = session.OrganizationID
	s.pausedExecutionID = executionID
	return s.executionDetail, s.pauseErr
}

func (s *workflowFakeService) ResumeExecution(ctx context.Context, session auth.Session, executionID string, req workflow.ResumeExecutionRequest) (*workflow.WorkflowExecution, error) {
	s.organizationID = session.OrganizationID
	s.resumedExecutionID = executionID
	s.resumedExecutionRequest = req
	return s.executionDetail, s.resumeErr
}

func (s *workflowFakeService) CancelExecution(ctx context.Context, session auth.Session, executionID string) (*workflow.WorkflowExecution, error) {
	s.organizationID = session.OrganizationID
	s.cancelledExecutionID = executionID
	return s.executionDetail, s.cancelErr
}

func workflowTestRequest(method, path, body string) *stdhttp.Request {
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, path, reader).WithContext(context.WithValue(context.Background(), sessionContextKey, auth.Session{
		OrganizationID: "org_1",
		User:           auth.User{ID: "user_1"},
		WorkspaceID:    "workspace_1",
	}))
	request.Header.Set("Content-Type", "application/json")
	return request
}

func workflowWebhookSignature(secret string, timestamp string, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write([]byte(body))
	return hex.EncodeToString(mac.Sum(nil))
}

func workflowWebhookTimestamp(now time.Time) string {
	return now.UTC().Format(time.RFC3339)
}

func TestWorkflowHandlerCreateWorkflowPassesDefinition(t *testing.T) {
	now := time.Date(2026, time.June, 4, 9, 0, 0, 0, time.UTC)
	service := &workflowFakeService{
		created: &workflow.WorkflowDefinition{
			ID:             "workflow_1",
			OrganizationID: "org_1",
			Name:           "Incident triage",
			Status:         workflow.WorkflowStatusDraft,
			Version:        1,
			CreatedAt:      now,
			UpdatedAt:      now,
		},
	}
	handler := newWorkflowHandler(service)
	recorder := httptest.NewRecorder()

	handler.createWorkflow(recorder, workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows", `{
		"name": " Incident triage ",
		"description": "Route critical alerts",
		"status": "published",
		"definition": {"nodes":[{"id":"start","type":"trigger"}]},
		"variables": {"severity":"critical"}
	}`))

	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.organizationID != "org_1" {
		t.Fatalf("expected organization org_1, got %q", service.organizationID)
	}
	if service.createReq.Name != "Incident triage" || service.createReq.Description != "Route critical alerts" {
		t.Fatalf("unexpected create request: %+v", service.createReq)
	}
	if service.createReq.Status != workflow.WorkflowStatusPublished {
		t.Fatalf("expected published status, got %q", service.createReq.Status)
	}
	nodes, ok := service.createReq.Definition["nodes"].([]any)
	if !ok || len(nodes) != 1 {
		t.Fatalf("expected one node in definition, got %+v", service.createReq.Definition["nodes"])
	}
	if service.createReq.Variables["severity"] != "critical" {
		t.Fatalf("expected severity variable, got %+v", service.createReq.Variables)
	}
}

func TestWorkflowHandlerUpdateWorkflowPassesPatch(t *testing.T) {
	service := &workflowFakeService{
		updated: &workflow.WorkflowDefinition{ID: "workflow_1", OrganizationID: "org_1", Name: "Incident triage"},
	}
	handler := newWorkflowHandler(service)
	recorder := httptest.NewRecorder()

	handler.updateWorkflow(recorder, workflowTestRequest(stdhttp.MethodPut, "/api/v1/workflows/workflow_1", `{
		"name": " Incident triage ",
		"description": " Updated route ",
		"status": "published",
		"definition": {"nodes":[{"id":"start"}]},
		"variables": {"severity":"critical"}
	}`), "workflow_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.updateReq.WorkflowID != "workflow_1" || service.updateReq.OrganizationID != "" {
		t.Fatalf("unexpected update request identity before adapter enrichment: %+v", service.updateReq)
	}
	if service.updateReq.Name == nil || *service.updateReq.Name != "Incident triage" {
		t.Fatalf("expected trimmed name pointer, got %+v", service.updateReq.Name)
	}
	if service.updateReq.Description == nil || *service.updateReq.Description != "Updated route" {
		t.Fatalf("expected trimmed description pointer, got %+v", service.updateReq.Description)
	}
	if service.updateReq.Status == nil || *service.updateReq.Status != workflow.WorkflowStatusPublished {
		t.Fatalf("expected published status, got %+v", service.updateReq.Status)
	}
	if service.updateReq.Variables["severity"] != "critical" {
		t.Fatalf("expected variables to pass through, got %+v", service.updateReq.Variables)
	}
}

func TestWorkflowHandlerListWorkflowVersionsReturnsHistory(t *testing.T) {
	service := &workflowFakeService{
		versions: []*workflow.WorkflowDefinition{
			{ID: "workflow_1", OrganizationID: "org_1", Name: "Incident triage", Status: workflow.WorkflowStatusDraft, Version: 1},
			{ID: "workflow_1", OrganizationID: "org_1", Name: "Incident triage", Status: workflow.WorkflowStatusPublished, Version: 2},
		},
	}
	handler := newWorkflowHandler(service)
	recorder := httptest.NewRecorder()

	handler.listWorkflowVersions(recorder, workflowTestRequest(stdhttp.MethodGet, "/api/v1/workflows/workflow_1/versions", ""), "workflow_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.organizationID != "org_1" || service.workflowID != "workflow_1" {
		t.Fatalf("unexpected version history identity org=%q workflow=%q", service.organizationID, service.workflowID)
	}
	var response struct {
		Data []workflow.WorkflowDefinition `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(response.Data) != 2 || response.Data[1].Version != 2 || response.Data[1].Status != workflow.WorkflowStatusPublished {
		t.Fatalf("unexpected version history response: %+v", response.Data)
	}
}

func TestWorkflowHandlerRollbackWorkflowPassesVersion(t *testing.T) {
	service := &workflowFakeService{
		rolledBack: &workflow.WorkflowDefinition{
			ID:             "workflow_1",
			OrganizationID: "org_1",
			Name:           "Incident triage",
			Status:         workflow.WorkflowStatusDraft,
			Version:        3,
			Definition:     map[string]any{"nodes": []any{map[string]any{"id": "manual-start"}}},
		},
	}
	handler := newWorkflowHandler(service)
	recorder := httptest.NewRecorder()

	handler.rollbackWorkflow(recorder, workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/workflow_1/rollback", `{"version":1}`), "workflow_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.organizationID != "org_1" {
		t.Fatalf("expected organization org_1, got %q", service.organizationID)
	}
	if service.rollbackReq.WorkflowID != "workflow_1" || service.rollbackReq.Version != 1 || service.rollbackReq.OrganizationID != "" {
		t.Fatalf("unexpected rollback request before adapter enrichment: %+v", service.rollbackReq)
	}
	var response struct {
		Data workflow.WorkflowDefinition `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Version != 3 {
		t.Fatalf("expected rolled back workflow version 3, got %+v", response.Data)
	}
}

func TestWorkflowHandlerCreateWorkflowBranchPassesExperimentRequest(t *testing.T) {
	service := &workflowFakeService{
		branched: &workflow.WorkflowDefinition{
			ID:             "workflow_branch",
			OrganizationID: "org_1",
			Name:           "Incident Router Experiment B",
			Status:         workflow.WorkflowStatusDraft,
			Version:        1,
			Definition: map[string]any{
				"branch": map[string]any{
					"sourceWorkflowId": "workflow_1",
					"sourceVersion":    2,
					"experimentKey":    "routing-copy-v2",
					"trafficPercent":   25,
				},
				"nodes": []any{map[string]any{"id": "manual-start"}},
			},
		},
	}
	handler := newWorkflowHandler(service)
	recorder := httptest.NewRecorder()

	handler.createWorkflowBranch(recorder, workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/workflow_1/branches", `{
		"name": " Incident Router Experiment B ",
		"description": " Variant branch ",
		"version": 2,
		"experimentKey": " routing-copy-v2 ",
		"trafficPercent": 25
	}`), "workflow_1")

	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.branchReq.WorkflowID != "workflow_1" || service.branchReq.OrganizationID != "" {
		t.Fatalf("unexpected branch request identity before adapter enrichment: %+v", service.branchReq)
	}
	if service.branchReq.Name != "Incident Router Experiment B" || service.branchReq.Description != "Variant branch" {
		t.Fatalf("expected trimmed branch strings, got %+v", service.branchReq)
	}
	if service.branchReq.Version != 2 || service.branchReq.ExperimentKey != "routing-copy-v2" || service.branchReq.TrafficPercent != 25 {
		t.Fatalf("unexpected branch request controls: %+v", service.branchReq)
	}
	var response struct {
		Data workflow.WorkflowDefinition `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.ID != "workflow_branch" || response.Data.Status != workflow.WorkflowStatusDraft {
		t.Fatalf("unexpected branch response: %+v", response.Data)
	}
}

func TestWorkflowHandlerDeleteWorkflowArchives(t *testing.T) {
	service := &workflowFakeService{
		deleted: &workflow.WorkflowDefinition{ID: "workflow_1", OrganizationID: "org_1", Status: workflow.WorkflowStatusArchived},
	}
	handler := newWorkflowHandler(service)
	recorder := httptest.NewRecorder()

	handler.deleteWorkflow(recorder, workflowTestRequest(stdhttp.MethodDelete, "/api/v1/workflows/workflow_1", ""), "workflow_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.workflowID != "workflow_1" || service.organizationID != "org_1" {
		t.Fatalf("unexpected delete identity workflow=%q org=%q", service.workflowID, service.organizationID)
	}
}

func TestWorkflowHandlerStartExecutionPassesInput(t *testing.T) {
	service := &workflowFakeService{
		started: &workflow.WorkflowExecution{
			ID:             "wexec_1",
			WorkflowID:     "workflow_1",
			OrganizationID: "org_1",
			Status:         workflow.ExecutionStatusRunning,
		},
	}
	handler := newWorkflowHandler(service)
	recorder := httptest.NewRecorder()

	handler.startExecution(recorder, workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/workflow_1/execute", `{"input":{"ticket":"INC-1","priority":1}}`), "workflow_1")

	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.workflowID != "workflow_1" {
		t.Fatalf("expected workflow_1, got %q", service.workflowID)
	}
	if service.executionInput["ticket"] != "INC-1" || service.executionInput["priority"].(float64) != 1 {
		t.Fatalf("unexpected execution input: %+v", service.executionInput)
	}
	if service.executionContext["userId"] != "user_1" || service.executionContext["workspaceId"] != "workspace_1" {
		t.Fatalf("expected session context to be propagated to workflow execution, got %+v", service.executionContext)
	}
}

func TestWorkflowHandlerSemanticMatchesRequireMessage(t *testing.T) {
	service := &workflowFakeService{}
	handler := newWorkflowHandler(service)
	recorder := httptest.NewRecorder()

	handler.matchSemanticTriggers(recorder, workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/semantic-matches", `{"message":"   "}`))

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error == nil || response.Error.Code != "invalid_request" {
		t.Fatalf("expected invalid_request error, got %+v", response.Error)
	}
	if service.semanticMatchReq.Message != "" {
		t.Fatalf("expected empty message not to call semantic matcher, got %+v", service.semanticMatchReq)
	}
}

func TestWorkflowHandlerConversationMatchesRequireConversationID(t *testing.T) {
	service := &workflowFakeService{}
	handler := newWorkflowHandler(service)
	recorder := httptest.NewRecorder()

	handler.matchConversationTriggers(recorder, workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/conversation-matches", `{"conversationId":"   "}`))

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error == nil || response.Error.Code != "invalid_request" {
		t.Fatalf("expected invalid_request error, got %+v", response.Error)
	}
	if service.conversationMatchReq.ConversationID != "" {
		t.Fatalf("expected empty conversation ID not to call conversation matcher, got %+v", service.conversationMatchReq)
	}
}

func TestWorkflowSemanticTriggerDispatcherStartsMatchedWorkflows(t *testing.T) {
	service := &workflowFakeService{
		conversationMatches: []workflow.ConversationTriggerMatch{
			{
				WorkflowID:      "workflow_conversation",
				TriggerID:       "conversation_trigger",
				ConversationID:  "conversation_1",
				WorkflowVersion: 3,
			},
		},
		semanticMatches: []workflow.SemanticTriggerMatch{
			{
				WorkflowID:  "workflow_1",
				TriggerID:   "trigger_1",
				Keyword:     "urgent outage",
				Score:       0.91,
				MatchMethod: "embedding",
			},
		},
		started:               &workflow.WorkflowExecution{ID: "wexec_1", WorkflowID: "workflow_1", Status: workflow.ExecutionStatusRunning},
		runUntilBlockedResult: &workflow.WorkflowExecution{ID: "wexec_1", WorkflowID: "workflow_1", Status: workflow.ExecutionStatusSucceeded},
	}
	dispatcher := workflowSemanticTriggerDispatcher{service: service}

	err := dispatcher.TriggerSemanticWorkflows(context.Background(), chat.SemanticWorkflowTriggerRequest{
		ConversationID: "conversation_1",
		Message:        " urgent outage in production ",
		OrganizationID: "org_1",
		UserID:         "user_1",
		WorkspaceID:    "workspace_1",
	})
	if err != nil {
		t.Fatalf("trigger semantic workflows: %v", err)
	}

	if service.organizationID != "org_1" {
		t.Fatalf("expected organization org_1, got %q", service.organizationID)
	}
	if service.semanticMatchReq.UserID != "user_1" || service.semanticMatchReq.Message != "urgent outage in production" {
		t.Fatalf("unexpected semantic match request: %+v", service.semanticMatchReq)
	}
	if service.conversationMatchReq.ConversationID != "conversation_1" {
		t.Fatalf("unexpected conversation match request: %+v", service.conversationMatchReq)
	}
	if service.startCalls != 2 {
		t.Fatalf("expected conversation and semantic executions, got %d starts", service.startCalls)
	}
	if len(service.runUntilBlockedExecutionIDs) != 2 || service.runUntilBlockedExecutionIDs[0] != "wexec_1" || service.runUntilBlockedExecutionIDs[1] != "wexec_1" {
		t.Fatalf("expected every matched triggered workflow to run until blocked, got %+v", service.runUntilBlockedExecutionIDs)
	}
	if service.startRequests[0].WorkflowID != "workflow_conversation" || service.startRequests[0].TriggerType != workflow.WorkflowTriggerConversation {
		t.Fatalf("expected first start to be conversation workflow, got %+v", service.startRequests[0])
	}
	if service.startRequests[0].TriggerPayload["workflowTriggerId"] != "conversation_trigger" || service.startRequests[0].TriggerPayload["conversationId"] != "conversation_1" {
		t.Fatalf("unexpected conversation trigger payload: %+v", service.startRequests[0].TriggerPayload)
	}
	if service.startRequests[1].WorkflowID != "workflow_1" || service.startRequests[1].TriggerType != workflow.WorkflowTriggerSemantic {
		t.Fatalf("expected second start to be semantic workflow, got %+v", service.startRequests[1])
	}
	if service.startRequests[1].Input["conversationId"] != "conversation_1" || service.startRequests[1].Input["message"] != "urgent outage in production" || service.startRequests[1].Input["userId"] != "user_1" {
		t.Fatalf("unexpected semantic execution input: %+v", service.startRequests[1].Input)
	}
	if service.startRequests[1].TriggerPayload["workflowTriggerId"] != "trigger_1" ||
		service.startRequests[1].TriggerPayload["keyword"] != "urgent outage" ||
		service.startRequests[1].TriggerPayload["score"] != 0.91 ||
		service.startRequests[1].TriggerPayload["matchMethod"] != "embedding" {
		t.Fatalf("unexpected semantic trigger payload: %+v", service.startRequests[1].TriggerPayload)
	}
}

func TestWorkflowHandlerStartExecutionAutoModeRunsUntilBlocked(t *testing.T) {
	service := &workflowFakeService{
		started: &workflow.WorkflowExecution{
			ID:             "wexec_1",
			WorkflowID:     "workflow_1",
			OrganizationID: "org_1",
			Status:         workflow.ExecutionStatusRunning,
		},
		runUntilBlockedResult: &workflow.WorkflowExecution{
			ID:             "wexec_1",
			WorkflowID:     "workflow_1",
			OrganizationID: "org_1",
			Status:         workflow.ExecutionStatusSucceeded,
		},
	}
	handler := newWorkflowHandler(service)
	recorder := httptest.NewRecorder()

	handler.startExecution(recorder, workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/workflow_1/execute", `{"input":{"ticket":"INC-1"},"executionMode":"auto"}`), "workflow_1")

	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.runUntilBlockedExecutionID != "wexec_1" {
		t.Fatalf("expected auto execution to run until blocked, got %q", service.runUntilBlockedExecutionID)
	}
	var response struct {
		Data workflow.WorkflowExecution `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Status != workflow.ExecutionStatusSucceeded {
		t.Fatalf("expected auto mode response to return advanced execution, got %+v", response.Data)
	}
}

func TestWorkflowHandlerWebhookStartsExecutionWithRawPayload(t *testing.T) {
	service := &workflowFakeService{
		started: &workflow.WorkflowExecution{
			ID:             "wexec_webhook",
			WorkflowID:     "workflow_1",
			OrganizationID: "org_1",
			Status:         workflow.ExecutionStatusRunning,
		},
		runUntilBlockedResult: &workflow.WorkflowExecution{
			ID:             "wexec_webhook",
			WorkflowID:     "workflow_1",
			OrganizationID: "org_1",
			Status:         workflow.ExecutionStatusSucceeded,
		},
	}
	handler := newWorkflowHandler(service)
	recorder := httptest.NewRecorder()

	handler.triggerWebhook(recorder, workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/workflow_1/webhook", `{"event":"issue.created","ticket":{"id":"INC-1"},"urgent":true}`), "workflow_1")

	if recorder.Code != stdhttp.StatusCreated {
		t.Fatalf("expected 201, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.organizationID != "org_1" || service.workflowID != "workflow_1" {
		t.Fatalf("unexpected webhook identity org=%q workflow=%q", service.organizationID, service.workflowID)
	}
	if service.executionTriggerType != workflow.WorkflowTriggerWebhook {
		t.Fatalf("expected webhook trigger, got %q", service.executionTriggerType)
	}
	if service.executionInput["event"] != "issue.created" || service.executionInput["urgent"] != true {
		t.Fatalf("expected raw webhook payload as input, got %+v", service.executionInput)
	}
	ticket := service.executionTriggerPayload["payload"].(map[string]any)["ticket"].(map[string]any)
	if ticket["id"] != "INC-1" {
		t.Fatalf("expected raw webhook payload in trigger context, got %+v", service.executionTriggerPayload)
	}
	if service.executionTriggerPayload["method"] != stdhttp.MethodPost {
		t.Fatalf("expected webhook method in trigger payload, got %+v", service.executionTriggerPayload)
	}
	if service.runUntilBlockedExecutionID != "wexec_webhook" {
		t.Fatalf("expected webhook trigger to run until blocked, got %q", service.runUntilBlockedExecutionID)
	}
	var response struct {
		Data workflow.WorkflowExecution `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Status != workflow.ExecutionStatusSucceeded {
		t.Fatalf("expected webhook response to return advanced execution, got %+v", response.Data)
	}
}

func TestWorkflowHandlerSignedWebhookRequiresSignature(t *testing.T) {
	service := &workflowFakeService{
		workflowDetail: &workflow.WorkflowDefinition{
			ID:             "workflow_1",
			OrganizationID: "org_1",
			Definition:     map[string]any{"webhook_secret": "top-secret"},
		},
		started: &workflow.WorkflowExecution{ID: "wexec_webhook", WorkflowID: "workflow_1", Status: workflow.ExecutionStatusRunning},
	}
	handler := newWorkflowHandler(service)
	recorder := httptest.NewRecorder()
	body := `{"event":"issue.created"}`
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/workflows/webhooks/org_1/workflow_1", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	handler.triggerSignedWebhook(recorder, request, "org_1", "workflow_1")

	if recorder.Code != stdhttp.StatusUnauthorized {
		t.Fatalf("expected 401, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error == nil || response.Error.Code != "webhook_signature_required" {
		t.Fatalf("expected missing signature error, got %+v", response.Error)
	}
	if service.startCalls != 0 {
		t.Fatalf("expected missing signature not to start execution, got %d starts", service.startCalls)
	}
}

func TestWorkflowHandlerSignedWebhookRequiresTimestamp(t *testing.T) {
	service := &workflowFakeService{
		workflowDetail: &workflow.WorkflowDefinition{
			ID:             "workflow_1",
			OrganizationID: "org_1",
			Definition:     map[string]any{"webhook_secret": "top-secret"},
		},
		started: &workflow.WorkflowExecution{ID: "wexec_webhook", WorkflowID: "workflow_1", Status: workflow.ExecutionStatusRunning},
	}
	handler := newWorkflowHandler(service)
	recorder := httptest.NewRecorder()
	body := `{"event":"issue.created"}`
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/workflows/webhooks/org_1/workflow_1", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Oblivious-Signature", workflowWebhookSignature("top-secret", "1770123600", body))

	handler.triggerSignedWebhook(recorder, request, "org_1", "workflow_1")

	if recorder.Code != stdhttp.StatusUnauthorized {
		t.Fatalf("expected 401, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error == nil || response.Error.Code != "webhook_timestamp_required" {
		t.Fatalf("expected missing timestamp error, got %+v", response.Error)
	}
	if service.startCalls != 0 {
		t.Fatalf("expected missing timestamp not to start execution, got %d starts", service.startCalls)
	}
}

func TestWorkflowHandlerSignedWebhookRejectsInvalidSignature(t *testing.T) {
	service := &workflowFakeService{
		workflowDetail: &workflow.WorkflowDefinition{
			ID:             "workflow_1",
			OrganizationID: "org_1",
			Definition:     map[string]any{"webhookSecret": "top-secret"},
		},
		started: &workflow.WorkflowExecution{ID: "wexec_webhook", WorkflowID: "workflow_1", Status: workflow.ExecutionStatusRunning},
	}
	handler := newWorkflowHandler(service)
	recorder := httptest.NewRecorder()
	body := `{"event":"issue.created"}`
	timestamp := workflowWebhookTimestamp(time.Now())
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/workflows/webhooks/org_1/workflow_1", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Oblivious-Timestamp", timestamp)
	request.Header.Set("X-Oblivious-Signature", workflowWebhookSignature("wrong-secret", timestamp, body))

	handler.triggerSignedWebhook(recorder, request, "org_1", "workflow_1")

	if recorder.Code != stdhttp.StatusUnauthorized {
		t.Fatalf("expected 401, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error == nil || response.Error.Code != "invalid_signature" {
		t.Fatalf("expected invalid signature error, got %+v", response.Error)
	}
	if service.startCalls != 0 {
		t.Fatalf("expected invalid signature not to start execution, got %d starts", service.startCalls)
	}
}

func TestWorkflowHandlerSignedWebhookRejectsExpiredTimestamp(t *testing.T) {
	service := &workflowFakeService{
		workflowDetail: &workflow.WorkflowDefinition{
			ID:             "workflow_1",
			OrganizationID: "org_1",
			Definition:     map[string]any{"webhook_secret": "top-secret"},
		},
		started: &workflow.WorkflowExecution{ID: "wexec_webhook", WorkflowID: "workflow_1", Status: workflow.ExecutionStatusRunning},
	}
	handler := newWorkflowHandler(service)
	recorder := httptest.NewRecorder()
	body := `{"event":"issue.created"}`
	timestamp := workflowWebhookTimestamp(time.Now().Add(-10 * time.Minute))
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/workflows/webhooks/org_1/workflow_1", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Oblivious-Timestamp", timestamp)
	request.Header.Set("X-Oblivious-Signature", workflowWebhookSignature("top-secret", timestamp, body))

	handler.triggerSignedWebhook(recorder, request, "org_1", "workflow_1")

	if recorder.Code != stdhttp.StatusUnauthorized {
		t.Fatalf("expected 401, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error == nil || response.Error.Code != "webhook_timestamp_expired" {
		t.Fatalf("expected expired timestamp error, got %+v", response.Error)
	}
	if service.startCalls != 0 {
		t.Fatalf("expected expired timestamp not to start execution, got %d starts", service.startCalls)
	}
}

func TestWorkflowHandlerSignedWebhookStartsExecutionForValidSignature(t *testing.T) {
	tests := []struct {
		name       string
		definition map[string]any
	}{
		{name: "snake secret", definition: map[string]any{"webhook_secret": "top-secret"}},
		{name: "camel secret", definition: map[string]any{"webhookSecret": "top-secret"}},
		{name: "trigger secret", definition: map[string]any{"triggers": map[string]any{"webhook": map[string]any{"secret": "top-secret"}}}},
		{name: "trigger array secret", definition: map[string]any{"triggers": map[string]any{"webhook": []any{map[string]any{"id": "github", "secret": "top-secret"}}}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := &workflowFakeService{
				workflowDetail: &workflow.WorkflowDefinition{
					ID:             "workflow_1",
					OrganizationID: "org_1",
					Definition:     tc.definition,
				},
				started: &workflow.WorkflowExecution{ID: "wexec_webhook", WorkflowID: "workflow_1", Status: workflow.ExecutionStatusRunning},
			}
			handler := newWorkflowHandler(service)
			recorder := httptest.NewRecorder()
			body := `{"source":"github","action":"opened"}`
			timestamp := workflowWebhookTimestamp(time.Now())
			request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/workflows/webhooks/org_1/workflow_1", strings.NewReader(body))
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Oblivious-Timestamp", timestamp)
			request.Header.Set("X-Oblivious-Signature", "sha256="+workflowWebhookSignature("top-secret", timestamp, body))

			handler.triggerSignedWebhook(recorder, request, "org_1", "workflow_1")

			if recorder.Code != stdhttp.StatusCreated {
				t.Fatalf("expected 201, got %d with body %s", recorder.Code, recorder.Body.String())
			}
			if service.startCalls != 1 {
				t.Fatalf("expected one execution start, got %d", service.startCalls)
			}
			if service.organizationID != "org_1" || service.workflowID != "workflow_1" {
				t.Fatalf("unexpected webhook identity org=%q workflow=%q", service.organizationID, service.workflowID)
			}
			if service.executionTriggerType != workflow.WorkflowTriggerWebhook {
				t.Fatalf("expected webhook trigger, got %q", service.executionTriggerType)
			}
			if service.executionInput["source"] != "github" || service.executionInput["action"] != "opened" {
				t.Fatalf("expected signed webhook payload as input, got %+v", service.executionInput)
			}
			if service.executionTriggerPayload["method"] != stdhttp.MethodPost {
				t.Fatalf("expected webhook method in trigger payload, got %+v", service.executionTriggerPayload)
			}
			if service.executionTriggerPayload["timestamp"] != timestamp {
				t.Fatalf("expected webhook timestamp in trigger payload, got %+v", service.executionTriggerPayload)
			}
			if service.executionContext["webhookPath"] != "/api/v1/workflows/webhooks/org_1/workflow_1" {
				t.Fatalf("expected webhook path in context, got %+v", service.executionContext)
			}
		})
	}
}

func TestWorkflowHandlerSignedWebhookRejectsReplay(t *testing.T) {
	service := &workflowFakeService{
		workflowDetail: &workflow.WorkflowDefinition{
			ID:             "workflow_1",
			OrganizationID: "org_1",
			Definition:     map[string]any{"webhook_secret": "top-secret"},
		},
		started: &workflow.WorkflowExecution{ID: "wexec_webhook", WorkflowID: "workflow_1", Status: workflow.ExecutionStatusRunning},
	}
	handler := newWorkflowHandler(service)
	body := `{"event":"issue.created"}`
	timestamp := workflowWebhookTimestamp(time.Now())
	signature := workflowWebhookSignature("top-secret", timestamp, body)

	firstRecorder := httptest.NewRecorder()
	firstRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/workflows/webhooks/org_1/workflow_1", strings.NewReader(body))
	firstRequest.Header.Set("Content-Type", "application/json")
	firstRequest.Header.Set("X-Oblivious-Timestamp", timestamp)
	firstRequest.Header.Set("X-Oblivious-Signature", signature)
	handler.triggerSignedWebhook(firstRecorder, firstRequest, "org_1", "workflow_1")
	if firstRecorder.Code != stdhttp.StatusCreated {
		t.Fatalf("first request expected 201, got %d with body %s", firstRecorder.Code, firstRecorder.Body.String())
	}

	secondRecorder := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/workflows/webhooks/org_1/workflow_1", strings.NewReader(body))
	secondRequest.Header.Set("Content-Type", "application/json")
	secondRequest.Header.Set("X-Oblivious-Timestamp", timestamp)
	secondRequest.Header.Set("X-Oblivious-Signature", signature)
	handler.triggerSignedWebhook(secondRecorder, secondRequest, "org_1", "workflow_1")

	if secondRecorder.Code != stdhttp.StatusConflict {
		t.Fatalf("second request expected 409, got %d with body %s", secondRecorder.Code, secondRecorder.Body.String())
	}
	var response Envelope
	if err := json.Unmarshal(secondRecorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error == nil || response.Error.Code != "webhook_replay_detected" {
		t.Fatalf("expected replay error, got %+v", response.Error)
	}
	if service.startCalls != 1 {
		t.Fatalf("expected replay not to start a second execution, got %d starts", service.startCalls)
	}
}

func TestWorkflowHandlerSignedWebhookRequiresConfiguredSecret(t *testing.T) {
	service := &workflowFakeService{
		workflowDetail: &workflow.WorkflowDefinition{
			ID:             "workflow_1",
			OrganizationID: "org_1",
			Definition:     map[string]any{"nodes": []any{}},
		},
		started: &workflow.WorkflowExecution{ID: "wexec_webhook", WorkflowID: "workflow_1", Status: workflow.ExecutionStatusRunning},
	}
	handler := newWorkflowHandler(service)
	recorder := httptest.NewRecorder()
	body := `{"event":"issue.created"}`
	timestamp := workflowWebhookTimestamp(time.Now())
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/workflows/webhooks/org_1/workflow_1", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Oblivious-Timestamp", timestamp)
	request.Header.Set("X-Oblivious-Signature", workflowWebhookSignature("top-secret", timestamp, body))

	handler.triggerSignedWebhook(recorder, request, "org_1", "workflow_1")

	if recorder.Code != stdhttp.StatusBadRequest {
		t.Fatalf("expected 400, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response Envelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Error == nil || response.Error.Code != "webhook_secret_required" {
		t.Fatalf("expected missing secret error, got %+v", response.Error)
	}
	if service.startCalls != 0 {
		t.Fatalf("expected missing secret not to start execution, got %d starts", service.startCalls)
	}
}

func TestWorkflowHandlerTestNodeAcceptsCamelAndSnakeNodeID(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{name: "camel", body: `{"nodeId":"notify","input":{"ticket":"INC-1"}}`},
		{name: "snake", body: `{"node_id":"notify","input":{"ticket":"INC-1"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &workflowFakeService{
				testNodeResult: &workflow.TestNodeResult{
					WorkflowID: "workflow_1",
					NodeID:     "notify",
					Status:     workflow.ExecutionStatusSucceeded,
				},
			}
			handler := newWorkflowHandler(service)
			recorder := httptest.NewRecorder()

			handler.testNode(recorder, workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/workflow_1/test-node", tc.body), "workflow_1")

			if recorder.Code != stdhttp.StatusOK {
				t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
			}
			if service.workflowID != "workflow_1" || service.testNodeID != "notify" {
				t.Fatalf("unexpected test node identity workflow=%q node=%q", service.workflowID, service.testNodeID)
			}
			if service.testNodeInput["ticket"] != "INC-1" {
				t.Fatalf("expected input to pass through, got %+v", service.testNodeInput)
			}
		})
	}
}

func TestWorkflowHandlerTestNodeReturnsFailedResultWithOKStatus(t *testing.T) {
	service := &workflowFakeService{
		testNodeResult: &workflow.TestNodeResult{
			WorkflowID: "workflow_1",
			NodeID:     "notify",
			Status:     workflow.ExecutionStatusFailed,
			Input:      map[string]any{"ticket": "INC-1"},
			Output:     map[string]any{"statusCode": 500},
		},
		testNodeErr: errors.New("upstream timeout"),
	}
	handler := newWorkflowHandler(service)
	recorder := httptest.NewRecorder()

	handler.testNode(recorder, workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/workflow_1/test-node", `{
		"nodeId":"notify",
		"input":{"ticket":"INC-1"}
	}`), "workflow_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200 for structured failed node result, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data workflow.TestNodeResult `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Status != workflow.ExecutionStatusFailed || response.Data.Output["statusCode"] != float64(500) {
		t.Fatalf("expected failed node result in response, got %+v", response.Data)
	}
}

func TestWorkflowHandlerExecutionTransitions(t *testing.T) {
	service := &workflowFakeService{
		executionDetail: &workflow.WorkflowExecution{
			ID:             "wexec_1",
			WorkflowID:     "workflow_1",
			OrganizationID: "org_1",
			Status:         workflow.ExecutionStatusPaused,
		},
		runUntilBlockedResult: &workflow.WorkflowExecution{
			ID:             "wexec_1",
			WorkflowID:     "workflow_1",
			OrganizationID: "org_1",
			Status:         workflow.ExecutionStatusSucceeded,
		},
	}
	handler := newWorkflowHandler(service)

	recorder := httptest.NewRecorder()
	handler.pauseExecution(recorder, workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/workflow_1/executions/wexec_1/pause", ""), "wexec_1")
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("pause expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.pausedExecutionID != "wexec_1" {
		t.Fatalf("pause expected wexec_1, got %q", service.pausedExecutionID)
	}

	service.executionDetail = &workflow.WorkflowExecution{
		ID:             "wexec_1",
		WorkflowID:     "workflow_1",
		OrganizationID: "org_1",
		Status:         workflow.ExecutionStatusRunning,
	}
	recorder = httptest.NewRecorder()
	handler.resumeExecution(recorder, workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/workflow_1/executions/wexec_1/resume", ""), "wexec_1")
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("resume expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.resumedExecutionID != "wexec_1" {
		t.Fatalf("resume expected wexec_1, got %q", service.resumedExecutionID)
	}
	if service.runUntilBlockedExecutionID != "wexec_1" {
		t.Fatalf("resume expected execution to run until blocked, got %q", service.runUntilBlockedExecutionID)
	}

	recorder = httptest.NewRecorder()
	handler.cancelExecution(recorder, workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/workflow_1/executions/wexec_1/cancel", ""), "wexec_1")
	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("cancel expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.cancelledExecutionID != "wexec_1" {
		t.Fatalf("cancel expected wexec_1, got %q", service.cancelledExecutionID)
	}
}

func TestWorkflowHandlerResumeExecutionPassesUserInputPayload(t *testing.T) {
	service := &workflowFakeService{
		executionDetail: &workflow.WorkflowExecution{
			ID:             "wexec_1",
			WorkflowID:     "workflow_1",
			OrganizationID: "org_1",
			Status:         workflow.ExecutionStatusRunning,
		},
		runUntilBlockedResult: &workflow.WorkflowExecution{
			ID:             "wexec_1",
			WorkflowID:     "workflow_1",
			OrganizationID: "org_1",
			Status:         workflow.ExecutionStatusSucceeded,
		},
	}
	handler := newWorkflowHandler(service)
	recorder := httptest.NewRecorder()

	handler.resumeExecution(recorder, workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/workflow_1/executions/wexec_1/resume", `{
		"nodeId": "approval",
		"input": {"approved": true, "approver": "ops"}
	}`), "wexec_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.resumedExecutionID != "wexec_1" || service.resumedExecutionRequest.NodeID != "approval" {
		t.Fatalf("unexpected resume request identity: execution=%q request=%+v", service.resumedExecutionID, service.resumedExecutionRequest)
	}
	if service.resumedExecutionRequest.Input["approved"] != true || service.resumedExecutionRequest.Input["approver"] != "ops" {
		t.Fatalf("expected submitted user input to pass through, got %+v", service.resumedExecutionRequest.Input)
	}
	if service.runUntilBlockedExecutionID != "wexec_1" {
		t.Fatalf("expected resumed execution to run until blocked, got %q", service.runUntilBlockedExecutionID)
	}
}

func TestWorkflowHandlerCheckResourceLimitsPassesUsage(t *testing.T) {
	now := time.Date(2026, time.June, 4, 10, 30, 0, 0, time.UTC)
	service := &workflowFakeService{
		executionDetail: &workflow.WorkflowExecution{
			ID:             "wexec_1",
			WorkflowID:     "workflow_1",
			OrganizationID: "org_1",
			Status:         workflow.ExecutionStatusRunning,
		},
		runUntilBlockedResult: &workflow.WorkflowExecution{
			ID:             "wexec_1",
			WorkflowID:     "workflow_1",
			OrganizationID: "org_1",
			Status:         workflow.ExecutionStatusSucceeded,
		},
	}
	handler := newWorkflowHandler(service)
	recorder := httptest.NewRecorder()

	handler.checkResourceLimits(recorder, workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/workflow_1/executions/wexec_1/resource-check", `{
		"totalTokens": 321,
		"nodeExecutionCount": 7,
		"now": "2026-06-04T10:30:00Z"
	}`), "workflow_1", "wexec_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.organizationID != "org_1" || service.workflowID != "workflow_1" || service.checkedExecutionID != "wexec_1" {
		t.Fatalf("unexpected resource check identity org=%q workflow=%q execution=%q", service.organizationID, service.workflowID, service.checkedExecutionID)
	}
	if service.resourceUsage.TotalTokens != 321 || service.resourceUsage.NodeExecutionCount != 7 || !service.resourceUsage.Now.Equal(now) {
		t.Fatalf("unexpected resource usage: %+v", service.resourceUsage)
	}
}

func TestWorkflowHandlerCheckResourceLimitsReturnsUpdatedExecutionWhenLimitIsReached(t *testing.T) {
	service := &workflowFakeService{
		executionDetail: &workflow.WorkflowExecution{
			ID:             "wexec_1",
			WorkflowID:     "workflow_1",
			OrganizationID: "org_1",
			Status:         workflow.ExecutionStatusMaxIterations,
		},
		resourceCheckErr: workflow.ErrWorkflowResourceLimit,
	}
	handler := newWorkflowHandler(service)
	recorder := httptest.NewRecorder()

	handler.checkResourceLimits(recorder, workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/workflow_1/executions/wexec_1/resource-check", `{
		"nodeExecutionCount": 1001
	}`), "workflow_1", "wexec_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Data workflow.WorkflowExecution `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Status != workflow.ExecutionStatusMaxIterations {
		t.Fatalf("expected max_iterations execution, got %+v", response.Data)
	}
}

func TestWorkflowHandlerResolvePausedFailureDecisionRetriesNode(t *testing.T) {
	service := &workflowFakeService{
		executionDetail: &workflow.WorkflowExecution{
			ID:             "wexec_1",
			WorkflowID:     "workflow_1",
			OrganizationID: "org_1",
			Status:         workflow.ExecutionStatusRunning,
			NodeExecutions: []workflow.WorkflowNodeExecution{
				{NodeID: "classify", Status: workflow.NodeStatusPending, Attempt: 2, Input: map[string]any{"priority": "urgent"}},
			},
		},
		runUntilBlockedResult: &workflow.WorkflowExecution{
			ID:             "wexec_1",
			WorkflowID:     "workflow_1",
			OrganizationID: "org_1",
			Status:         workflow.ExecutionStatusSucceeded,
		},
	}
	handler := newWorkflowHandler(service)
	recorder := httptest.NewRecorder()

	handler.resolvePausedFailure(recorder, workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/workflow_1/executions/wexec_1/decision", `{
		"action": "retry",
		"nodeId": "classify",
		"input": {"priority": "urgent"}
	}`), "wexec_1")

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if service.organizationID != "org_1" || service.resolvedFailureExecutionID != "wexec_1" {
		t.Fatalf("unexpected decision identity org=%q execution=%q", service.organizationID, service.resolvedFailureExecutionID)
	}
	if service.resolvedFailureDecision.Action != workflow.FailureActionRetry || service.resolvedFailureDecision.NodeID != "classify" {
		t.Fatalf("unexpected failure decision request: %+v", service.resolvedFailureDecision)
	}
	if service.resolvedFailureDecision.Input["priority"] != "urgent" {
		t.Fatalf("expected edited retry input, got %+v", service.resolvedFailureDecision.Input)
	}
	if service.runUntilBlockedExecutionID != "wexec_1" {
		t.Fatalf("retry decision expected execution to run until blocked, got %q", service.runUntilBlockedExecutionID)
	}
	var response struct {
		Data workflow.WorkflowExecution `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Data.Status != workflow.ExecutionStatusSucceeded {
		t.Fatalf("expected advanced execution response, got %+v", response.Data)
	}
}

func TestWorkflowHandlerWorkflowDecisionSupportsPausedFailureUserActions(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantAction workflow.FailureAction
		wantInput  map[string]any
		response   *workflow.WorkflowExecution
		advanced   *workflow.WorkflowExecution
	}{
		{
			name:       "skip node",
			body:       `{"action":"skip","nodeId":"classify"}`,
			wantAction: workflow.FailureActionContinue,
			response:   &workflow.WorkflowExecution{ID: "wexec_1", WorkflowID: "workflow_1", OrganizationID: "org_1", Status: workflow.ExecutionStatusRunning},
			advanced:   &workflow.WorkflowExecution{ID: "wexec_1", WorkflowID: "workflow_1", OrganizationID: "org_1", Status: workflow.ExecutionStatusSucceeded},
		},
		{
			name:       "retry with edited input",
			body:       `{"action":"retry_with_input","nodeId":"classify","input":{"priority":"urgent"}}`,
			wantAction: workflow.FailureActionRetry,
			wantInput:  map[string]any{"priority": "urgent"},
			response: &workflow.WorkflowExecution{
				ID:             "wexec_1",
				WorkflowID:     "workflow_1",
				OrganizationID: "org_1",
				Status:         workflow.ExecutionStatusRunning,
				NodeExecutions: []workflow.WorkflowNodeExecution{
					{NodeID: "classify", Status: workflow.NodeStatusPending, Attempt: 2, Input: map[string]any{"priority": "urgent"}},
				},
			},
			advanced: &workflow.WorkflowExecution{ID: "wexec_1", WorkflowID: "workflow_1", OrganizationID: "org_1", Status: workflow.ExecutionStatusSucceeded},
		},
		{
			name:       "edit input and retry",
			body:       `{"action":"edit_input_retry","nodeId":"classify","input":{"priority":"high"}}`,
			wantAction: workflow.FailureActionRetry,
			wantInput:  map[string]any{"priority": "high"},
			response:   &workflow.WorkflowExecution{ID: "wexec_1", WorkflowID: "workflow_1", OrganizationID: "org_1", Status: workflow.ExecutionStatusRunning},
			advanced:   &workflow.WorkflowExecution{ID: "wexec_1", WorkflowID: "workflow_1", OrganizationID: "org_1", Status: workflow.ExecutionStatusSucceeded},
		},
		{
			name:       "terminate workflow",
			body:       `{"action":"terminate","nodeId":"classify"}`,
			wantAction: workflow.FailureActionFail,
			response:   &workflow.WorkflowExecution{ID: "wexec_1", WorkflowID: "workflow_1", OrganizationID: "org_1", Status: workflow.ExecutionStatusFailed},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &workflowFakeService{executionDetail: tt.response, runUntilBlockedResult: tt.advanced}
			handler := newWorkflowHandler(service)
			recorder := httptest.NewRecorder()

			handler.resolvePausedFailure(recorder, workflowTestRequest(stdhttp.MethodPost, "/api/v1/workflows/workflow_1/executions/wexec_1/decision", tt.body), "wexec_1")

			if recorder.Code != stdhttp.StatusOK {
				t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
			}
			if service.resolvedFailureExecutionID != "wexec_1" || service.resolvedFailureDecision.NodeID != "classify" {
				t.Fatalf("unexpected decision identity execution=%q request=%+v", service.resolvedFailureExecutionID, service.resolvedFailureDecision)
			}
			if service.resolvedFailureDecision.Action != tt.wantAction {
				t.Fatalf("expected action %q, got %+v", tt.wantAction, service.resolvedFailureDecision)
			}
			for key, want := range tt.wantInput {
				if service.resolvedFailureDecision.Input[key] != want {
					t.Fatalf("expected input %s=%v, got %+v", key, want, service.resolvedFailureDecision.Input)
				}
			}
			if tt.advanced != nil {
				if service.runUntilBlockedExecutionID != "wexec_1" {
					t.Fatalf("expected runnable decision to advance execution, got %q", service.runUntilBlockedExecutionID)
				}
			} else if service.runUntilBlockedExecutionID != "" {
				t.Fatalf("expected terminal decision not to advance execution, got %q", service.runUntilBlockedExecutionID)
			}
			var response struct {
				Data workflow.WorkflowExecution `json:"data"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			wantResponse := tt.response
			if tt.advanced != nil {
				wantResponse = tt.advanced
			}
			if response.Data.ID != "wexec_1" || response.Data.WorkflowID != "workflow_1" || response.Data.Status != wantResponse.Status {
				t.Fatalf("expected refreshable execution detail, got %+v", response.Data)
			}
		})
	}
}

func TestWorkflowHandlerMapsServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "invalid input", err: workflow.ErrInvalidInput, wantStatus: stdhttp.StatusBadRequest, wantCode: "invalid_request"},
		{name: "not found", err: workflow.ErrNotFound, wantStatus: stdhttp.StatusNotFound, wantCode: "not_found"},
		{name: "invalid transition", err: workflow.ErrInvalidTransition, wantStatus: stdhttp.StatusConflict, wantCode: "invalid_state"},
		{name: "resource limit", err: workflow.ErrWorkflowResourceLimit, wantStatus: stdhttp.StatusConflict, wantCode: "resource_limit"},
		{name: "internal", err: errors.New("database offline"), wantStatus: stdhttp.StatusInternalServerError, wantCode: "internal_error"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service := &workflowFakeService{getErr: tc.err}
			handler := newWorkflowHandler(service)
			recorder := httptest.NewRecorder()

			handler.getWorkflow(recorder, workflowTestRequest(stdhttp.MethodGet, "/api/v1/workflows/workflow_1", ""), "workflow_1")

			if recorder.Code != tc.wantStatus {
				t.Fatalf("expected %d, got %d with body %s", tc.wantStatus, recorder.Code, recorder.Body.String())
			}
			var response Envelope
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Error == nil || response.Error.Code != tc.wantCode {
				t.Fatalf("expected error code %q, got %+v", tc.wantCode, response.Error)
			}
		})
	}
}
