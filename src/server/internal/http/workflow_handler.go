package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	stdhttp "net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
	"oblivious/server/internal/workflow"
)

type workflowService interface {
	CreateWorkflow(ctx context.Context, session auth.Session, req workflow.CreateWorkflowRequest) (*workflow.WorkflowDefinition, error)
	ListWorkflows(ctx context.Context, session auth.Session) ([]*workflow.WorkflowDefinition, error)
	MatchConversationTriggers(ctx context.Context, session auth.Session, req workflow.MatchConversationTriggersRequest) ([]workflow.ConversationTriggerMatch, error)
	MatchSemanticTriggers(ctx context.Context, session auth.Session, req workflow.MatchSemanticTriggersRequest) ([]workflow.SemanticTriggerMatch, error)
	GetWorkflow(ctx context.Context, session auth.Session, workflowID string) (*workflow.WorkflowDefinition, error)
	ListWorkflowVersions(ctx context.Context, session auth.Session, workflowID string) ([]*workflow.WorkflowDefinition, error)
	UpdateWorkflow(ctx context.Context, session auth.Session, req workflow.UpdateWorkflowRequest) (*workflow.WorkflowDefinition, error)
	RollbackWorkflow(ctx context.Context, session auth.Session, req workflow.RollbackWorkflowRequest) (*workflow.WorkflowDefinition, error)
	CreateWorkflowBranch(ctx context.Context, session auth.Session, req workflow.CreateWorkflowBranchRequest) (*workflow.WorkflowDefinition, error)
	PublishWorkflowBranch(ctx context.Context, session auth.Session, req workflow.PublishWorkflowBranchRequest) (*workflow.WorkflowDefinition, error)
	MergeWorkflowBranch(ctx context.Context, session auth.Session, req workflow.MergeWorkflowBranchRequest) (*workflow.WorkflowDefinition, error)
	DeleteWorkflow(ctx context.Context, session auth.Session, workflowID string) (*workflow.WorkflowDefinition, error)
	StartExecution(ctx context.Context, session auth.Session, workflowID string, input map[string]any) (*workflow.WorkflowExecution, error)
	StartExecutionWithTrigger(ctx context.Context, session auth.Session, req workflow.StartExecutionRequest) (*workflow.WorkflowExecution, error)
	TestNode(ctx context.Context, session auth.Session, workflowID string, nodeID string, input map[string]any) (*workflow.TestNodeResult, error)
	ListExecutions(ctx context.Context, session auth.Session, workflowID string) ([]*workflow.WorkflowExecution, error)
	GetExecution(ctx context.Context, session auth.Session, executionID string) (*workflow.WorkflowExecution, error)
	BuildExecutionDebugSnapshot(ctx context.Context, session auth.Session, executionID string) (*workflow.ExecutionDebugSnapshot, error)
	RunExecutionUntilBlocked(ctx context.Context, session auth.Session, executionID string) (*workflow.WorkflowExecution, error)
	CheckResourceLimits(ctx context.Context, session auth.Session, workflowID string, executionID string, usage workflow.WorkflowResourceUsage) (*workflow.WorkflowExecution, error)
	ResolvePausedFailure(ctx context.Context, session auth.Session, executionID string, req workflow.ResolveFailureDecisionRequest) (*workflow.WorkflowExecution, error)
	PauseExecution(ctx context.Context, session auth.Session, executionID string) (*workflow.WorkflowExecution, error)
	ResumeExecution(ctx context.Context, session auth.Session, executionID string, req workflow.ResumeExecutionRequest) (*workflow.WorkflowExecution, error)
	CancelExecution(ctx context.Context, session auth.Session, executionID string) (*workflow.WorkflowExecution, error)
}

type workflowHandler struct {
	service             workflowService
	webhookReplayStore  *workflowWebhookReplayStore
	webhookTimestampNow func() time.Time
}

func newWorkflowHandler(service workflowService) workflowHandler {
	return workflowHandler{
		service:             service,
		webhookReplayStore:  newWorkflowWebhookReplayStore(),
		webhookTimestampNow: func() time.Time { return time.Now().UTC() },
	}
}

type workflowServiceAdapter struct {
	service *workflow.Service
}

func newWorkflowServiceAdapter(service *workflow.Service) workflowServiceAdapter {
	return workflowServiceAdapter{service: service}
}

func (a workflowServiceAdapter) CreateWorkflow(ctx context.Context, session auth.Session, req workflow.CreateWorkflowRequest) (*workflow.WorkflowDefinition, error) {
	req.OrganizationID = session.OrganizationID
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	return a.service.CreateWorkflow(ctx, req)
}

func (a workflowServiceAdapter) ListWorkflows(ctx context.Context, session auth.Session) ([]*workflow.WorkflowDefinition, error) {
	return a.service.ListWorkflows(ctx, session.OrganizationID)
}

func (a workflowServiceAdapter) MatchConversationTriggers(ctx context.Context, session auth.Session, req workflow.MatchConversationTriggersRequest) ([]workflow.ConversationTriggerMatch, error) {
	req.OrganizationID = session.OrganizationID
	req.ConversationID = strings.TrimSpace(req.ConversationID)
	return a.service.MatchConversationTriggers(ctx, req)
}

func (a workflowServiceAdapter) MatchSemanticTriggers(ctx context.Context, session auth.Session, req workflow.MatchSemanticTriggersRequest) ([]workflow.SemanticTriggerMatch, error) {
	req.OrganizationID = session.OrganizationID
	req.UserID = session.User.ID
	req.Message = strings.TrimSpace(req.Message)
	return a.service.MatchSemanticTriggers(ctx, req)
}

func (a workflowServiceAdapter) GetWorkflow(ctx context.Context, session auth.Session, workflowID string) (*workflow.WorkflowDefinition, error) {
	return a.service.GetWorkflow(ctx, session.OrganizationID, workflowID)
}

func (a workflowServiceAdapter) ListWorkflowVersions(ctx context.Context, session auth.Session, workflowID string) ([]*workflow.WorkflowDefinition, error) {
	return a.service.ListWorkflowVersions(ctx, session.OrganizationID, workflowID)
}

func (a workflowServiceAdapter) UpdateWorkflow(ctx context.Context, session auth.Session, req workflow.UpdateWorkflowRequest) (*workflow.WorkflowDefinition, error) {
	req.OrganizationID = session.OrganizationID
	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		req.Name = &trimmed
	}
	if req.Description != nil {
		trimmed := strings.TrimSpace(*req.Description)
		req.Description = &trimmed
	}
	return a.service.UpdateWorkflow(ctx, req)
}

func (a workflowServiceAdapter) RollbackWorkflow(ctx context.Context, session auth.Session, req workflow.RollbackWorkflowRequest) (*workflow.WorkflowDefinition, error) {
	req.OrganizationID = session.OrganizationID
	return a.service.RollbackWorkflow(ctx, req)
}

func (a workflowServiceAdapter) CreateWorkflowBranch(ctx context.Context, session auth.Session, req workflow.CreateWorkflowBranchRequest) (*workflow.WorkflowDefinition, error) {
	req.OrganizationID = session.OrganizationID
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	req.ExperimentKey = strings.TrimSpace(req.ExperimentKey)
	return a.service.CreateWorkflowBranch(ctx, req)
}

func (a workflowServiceAdapter) PublishWorkflowBranch(ctx context.Context, session auth.Session, req workflow.PublishWorkflowBranchRequest) (*workflow.WorkflowDefinition, error) {
	req.OrganizationID = session.OrganizationID
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	return a.service.PublishWorkflowBranch(ctx, req)
}

func (a workflowServiceAdapter) MergeWorkflowBranch(ctx context.Context, session auth.Session, req workflow.MergeWorkflowBranchRequest) (*workflow.WorkflowDefinition, error) {
	req.OrganizationID = session.OrganizationID
	return a.service.MergeWorkflowBranch(ctx, req)
}

func (a workflowServiceAdapter) DeleteWorkflow(ctx context.Context, session auth.Session, workflowID string) (*workflow.WorkflowDefinition, error) {
	return a.service.DeleteWorkflow(ctx, session.OrganizationID, workflowID)
}

func (a workflowServiceAdapter) StartExecution(ctx context.Context, session auth.Session, workflowID string, input map[string]any) (*workflow.WorkflowExecution, error) {
	return a.StartExecutionWithTrigger(ctx, session, workflow.StartExecutionRequest{
		WorkflowID: workflowID,
		Input:      input,
		Context:    workflowExecutionSessionContext(ctx, session),
	})
}

func (a workflowServiceAdapter) StartExecutionWithTrigger(ctx context.Context, session auth.Session, req workflow.StartExecutionRequest) (*workflow.WorkflowExecution, error) {
	req.OrganizationID = session.OrganizationID
	req.Context = mergeWorkflowHandlerMaps(workflowExecutionSessionContext(ctx, session), req.Context)
	return a.service.StartExecution(ctx, workflow.StartExecutionRequest{
		OrganizationID: req.OrganizationID,
		WorkflowID:     req.WorkflowID,
		TriggerType:    req.TriggerType,
		TriggerPayload: req.TriggerPayload,
		Input:          req.Input,
		Context:        req.Context,
	})
}

func workflowExecutionSessionContext(ctx context.Context, session auth.Session) map[string]any {
	contextValue := map[string]any{}
	if strings.TrimSpace(session.ID) != "" {
		contextValue["sessionId"] = strings.TrimSpace(session.ID)
	}
	if strings.TrimSpace(session.User.ID) != "" {
		contextValue["userId"] = strings.TrimSpace(session.User.ID)
	}
	if strings.TrimSpace(session.WorkspaceID) != "" {
		contextValue["workspaceId"] = strings.TrimSpace(session.WorkspaceID)
	}
	if requestID := strings.TrimSpace(requestIDFromContext(ctx)); requestID != "" {
		contextValue["requestId"] = requestID
	}
	return contextValue
}

func mergeWorkflowHandlerMaps(left map[string]any, right map[string]any) map[string]any {
	next := map[string]any{}
	for key, value := range left {
		next[key] = value
	}
	for key, value := range right {
		next[key] = value
	}
	return next
}

func (a workflowServiceAdapter) TestNode(ctx context.Context, session auth.Session, workflowID string, nodeID string, input map[string]any) (*workflow.TestNodeResult, error) {
	return a.service.TestNode(ctx, workflow.TestNodeRequest{
		OrganizationID: session.OrganizationID,
		WorkflowID:     workflowID,
		NodeID:         nodeID,
		Input:          input,
	})
}

func (a workflowServiceAdapter) ListExecutions(ctx context.Context, session auth.Session, workflowID string) ([]*workflow.WorkflowExecution, error) {
	if _, err := a.service.GetWorkflow(ctx, session.OrganizationID, workflowID); err != nil {
		return nil, err
	}
	return a.service.ListExecutions(ctx, session.OrganizationID, workflowID)
}

func (a workflowServiceAdapter) GetExecution(ctx context.Context, session auth.Session, executionID string) (*workflow.WorkflowExecution, error) {
	return a.service.GetExecution(ctx, session.OrganizationID, executionID)
}

func (a workflowServiceAdapter) BuildExecutionDebugSnapshot(ctx context.Context, session auth.Session, executionID string) (*workflow.ExecutionDebugSnapshot, error) {
	return a.service.BuildExecutionDebugSnapshot(ctx, session.OrganizationID, executionID)
}

func (a workflowServiceAdapter) RunExecutionUntilBlocked(ctx context.Context, session auth.Session, executionID string) (*workflow.WorkflowExecution, error) {
	return a.service.RunExecutionUntilBlocked(ctx, session.OrganizationID, executionID)
}

func (a workflowServiceAdapter) CheckResourceLimits(ctx context.Context, session auth.Session, workflowID string, executionID string, usage workflow.WorkflowResourceUsage) (*workflow.WorkflowExecution, error) {
	return a.service.CheckResourceLimits(ctx, session.OrganizationID, executionID, usage)
}

func (a workflowServiceAdapter) ResolvePausedFailure(ctx context.Context, session auth.Session, executionID string, req workflow.ResolveFailureDecisionRequest) (*workflow.WorkflowExecution, error) {
	return a.service.ResolvePausedFailure(ctx, session.OrganizationID, executionID, req)
}

func (a workflowServiceAdapter) PauseExecution(ctx context.Context, session auth.Session, executionID string) (*workflow.WorkflowExecution, error) {
	return a.service.PauseExecution(ctx, session.OrganizationID, executionID)
}

func (a workflowServiceAdapter) ResumeExecution(ctx context.Context, session auth.Session, executionID string, req workflow.ResumeExecutionRequest) (*workflow.WorkflowExecution, error) {
	return a.service.ResumeExecution(ctx, session.OrganizationID, executionID, req)
}

func (a workflowServiceAdapter) CancelExecution(ctx context.Context, session auth.Session, executionID string) (*workflow.WorkflowExecution, error) {
	return a.service.CancelExecution(ctx, session.OrganizationID, executionID)
}

type workflowSemanticTriggerDispatcher struct {
	service workflowService
}

func (d workflowSemanticTriggerDispatcher) TriggerSemanticWorkflows(ctx context.Context, req chat.SemanticWorkflowTriggerRequest) error {
	if d.service == nil {
		return nil
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		return nil
	}
	session := auth.Session{
		OrganizationID: req.OrganizationID,
		WorkspaceID:    req.WorkspaceID,
		User:           auth.User{ID: req.UserID},
	}
	conversationMatches, err := d.service.MatchConversationTriggers(ctx, session, workflow.MatchConversationTriggersRequest{
		ConversationID: req.ConversationID,
	})
	if err != nil {
		return err
	}
	for _, match := range conversationMatches {
		input := map[string]any{
			"conversationId": req.ConversationID,
			"message":        message,
			"userId":         req.UserID,
			"workspaceId":    req.WorkspaceID,
		}
		triggerPayload := map[string]any{
			"conversationId":    req.ConversationID,
			"message":           message,
			"userId":            req.UserID,
			"workspaceId":       req.WorkspaceID,
			"workflowTriggerId": match.TriggerID,
		}
		if _, err := startTriggeredWorkflowUntilBlocked(ctx, d.service, session, workflow.StartExecutionRequest{
			WorkflowID:     match.WorkflowID,
			TriggerType:    workflow.WorkflowTriggerConversation,
			TriggerPayload: triggerPayload,
			Input:          input,
		}); err != nil {
			return err
		}
	}
	matches, err := d.service.MatchSemanticTriggers(ctx, session, workflow.MatchSemanticTriggersRequest{
		UserID:  req.UserID,
		Message: message,
	})
	if err != nil {
		return err
	}
	for _, match := range matches {
		input := map[string]any{
			"conversationId": req.ConversationID,
			"message":        message,
			"userId":         req.UserID,
			"workspaceId":    req.WorkspaceID,
		}
		triggerPayload := map[string]any{
			"conversationId":    req.ConversationID,
			"message":           message,
			"userId":            req.UserID,
			"workspaceId":       req.WorkspaceID,
			"workflowTriggerId": match.TriggerID,
			"keyword":           match.Keyword,
		}
		if match.Score > 0 {
			triggerPayload["score"] = match.Score
		}
		if strings.TrimSpace(match.MatchMethod) != "" {
			triggerPayload["matchMethod"] = match.MatchMethod
		}
		if _, err := startTriggeredWorkflowUntilBlocked(ctx, d.service, session, workflow.StartExecutionRequest{
			WorkflowID:     match.WorkflowID,
			TriggerType:    workflow.WorkflowTriggerSemantic,
			TriggerPayload: triggerPayload,
			Input:          input,
		}); err != nil {
			return err
		}
	}
	return nil
}

func startTriggeredWorkflowUntilBlocked(ctx context.Context, service workflowService, session auth.Session, req workflow.StartExecutionRequest) (*workflow.WorkflowExecution, error) {
	execution, err := service.StartExecutionWithTrigger(ctx, session, req)
	if err != nil {
		return nil, err
	}
	if execution == nil || strings.TrimSpace(execution.ID) == "" {
		return execution, nil
	}
	return service.RunExecutionUntilBlocked(ctx, session, execution.ID)
}

func runExecutionIfRunnable(ctx context.Context, service workflowService, session auth.Session, execution *workflow.WorkflowExecution) (*workflow.WorkflowExecution, error) {
	if execution == nil || strings.TrimSpace(execution.ID) == "" {
		return execution, nil
	}
	if execution.Status != workflow.ExecutionStatusRunning && execution.Status != workflow.ExecutionStatusPartialSuccess {
		return execution, nil
	}
	return service.RunExecutionUntilBlocked(ctx, session, execution.ID)
}

type workflowCreateRequest struct {
	Name        string                  `json:"name"`
	Description string                  `json:"description,omitempty"`
	Status      workflow.WorkflowStatus `json:"status,omitempty"`
	Definition  map[string]any          `json:"definition"`
	Variables   map[string]any          `json:"variables,omitempty"`
}

type workflowUpdateRequest struct {
	Name        *string                  `json:"name,omitempty"`
	Description *string                  `json:"description,omitempty"`
	Status      *workflow.WorkflowStatus `json:"status,omitempty"`
	Definition  map[string]any           `json:"definition,omitempty"`
	Variables   map[string]any           `json:"variables,omitempty"`
}

type workflowRollbackRequest struct {
	Version int `json:"version"`
}

type workflowCreateBranchRequest struct {
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	Version        int    `json:"version"`
	ExperimentKey  string `json:"experimentKey,omitempty"`
	TrafficPercent int    `json:"trafficPercent,omitempty"`
}

type workflowPublishBranchRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

type workflowStartExecutionRequest struct {
	Input              map[string]any `json:"input,omitempty"`
	ExecutionMode      string         `json:"execution_mode,omitempty"`
	ExecutionModeCamel string         `json:"executionMode,omitempty"`
}

type workflowSemanticMatchRequest struct {
	Message string `json:"message"`
}

type workflowConversationMatchRequest struct {
	ConversationID string `json:"conversationId"`
}

type workflowTestNodeRequest struct {
	NodeID       string         `json:"nodeId"`
	NodeIDLegacy string         `json:"node_id"`
	Input        map[string]any `json:"input,omitempty"`
}

type workflowResourceCheckRequest struct {
	TotalTokens        int       `json:"totalTokens"`
	NodeExecutionCount int       `json:"nodeExecutionCount"`
	Now                time.Time `json:"now,omitempty"`
}

type workflowFailureDecisionRequest struct {
	Action     workflow.FailureAction `json:"action"`
	Input      map[string]any         `json:"input,omitempty"`
	NextNodeID string                 `json:"nextNodeId,omitempty"`
	NodeID     string                 `json:"nodeId,omitempty"`
}

type workflowResumeExecutionRequest struct {
	NodeID string         `json:"nodeId,omitempty"`
	Input  map[string]any `json:"input,omitempty"`
}

func normalizeWorkflowFailureDecisionAction(action workflow.FailureAction) workflow.FailureAction {
	switch workflow.FailureAction(strings.TrimSpace(string(action))) {
	case workflow.FailureActionRetry, "retry_with_input", "edit_input_retry":
		return workflow.FailureActionRetry
	case workflow.FailureActionContinue, "skip":
		return workflow.FailureActionContinue
	case workflow.FailureActionFail, "terminate":
		return workflow.FailureActionFail
	default:
		return action
	}
}

const workflowWebhookTimestampTolerance = 5 * time.Minute
const workflowRedactedSecret = "********"

type workflowWebhookReplayStore struct {
	mu   sync.Mutex
	seen map[string]time.Time
}

func newWorkflowWebhookReplayStore() *workflowWebhookReplayStore {
	return &workflowWebhookReplayStore{seen: make(map[string]time.Time)}
}

func (s *workflowWebhookReplayStore) Record(key string, expiresAt time.Time, now time.Time) bool {
	if s == nil || strings.TrimSpace(key) == "" {
		return true
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	for seenKey, expiry := range s.seen {
		if !expiry.After(now) {
			delete(s.seen, seenKey)
		}
	}
	if expiry, ok := s.seen[key]; ok && expiry.After(now) {
		return false
	}
	s.seen[key] = expiresAt
	return true
}

func (h workflowHandler) listWorkflows(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	workflows, err := h.service.ListWorkflows(r.Context(), session)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, redactWorkflowDefinitions(workflows))
}

func (h workflowHandler) createWorkflow(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload workflowCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	created, err := h.service.CreateWorkflow(r.Context(), session, workflow.CreateWorkflowRequest{
		Name:        strings.TrimSpace(payload.Name),
		Description: strings.TrimSpace(payload.Description),
		Status:      payload.Status,
		Definition:  payload.Definition,
		Variables:   payload.Variables,
	})
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusCreated, redactWorkflowDefinition(created))
}

func (h workflowHandler) matchSemanticTriggers(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload workflowSemanticMatchRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	message := strings.TrimSpace(payload.Message)
	if message == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "message is required")
		return
	}

	matches, err := h.service.MatchSemanticTriggers(r.Context(), session, workflow.MatchSemanticTriggersRequest{
		UserID:  session.User.ID,
		Message: message,
	})
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, matches)
}

func (h workflowHandler) matchConversationTriggers(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload workflowConversationMatchRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	conversationID := strings.TrimSpace(payload.ConversationID)
	if conversationID == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "conversationId is required")
		return
	}

	matches, err := h.service.MatchConversationTriggers(r.Context(), session, workflow.MatchConversationTriggersRequest{
		ConversationID: conversationID,
	})
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, matches)
}

func (h workflowHandler) getWorkflow(w stdhttp.ResponseWriter, r *stdhttp.Request, workflowID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	definition, err := h.service.GetWorkflow(r.Context(), session, workflowID)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, redactWorkflowDefinition(definition))
}

func (h workflowHandler) listWorkflowVersions(w stdhttp.ResponseWriter, r *stdhttp.Request, workflowID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	versions, err := h.service.ListWorkflowVersions(r.Context(), session, workflowID)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, redactWorkflowDefinitions(versions))
}

func (h workflowHandler) updateWorkflow(w stdhttp.ResponseWriter, r *stdhttp.Request, workflowID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload workflowUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	if payload.Name != nil {
		trimmed := strings.TrimSpace(*payload.Name)
		payload.Name = &trimmed
	}
	if payload.Description != nil {
		trimmed := strings.TrimSpace(*payload.Description)
		payload.Description = &trimmed
	}
	if payload.Definition != nil && workflowDefinitionHasRedactedSecret(payload.Definition) {
		existing, err := h.service.GetWorkflow(r.Context(), session, workflowID)
		if err != nil {
			writeWorkflowError(w, err)
			return
		}
		var existingDefinition map[string]any
		if existing != nil {
			existingDefinition = existing.Definition
		}
		payload.Definition = restoreRedactedWorkflowSecrets(payload.Definition, existingDefinition)
	}

	updated, err := h.service.UpdateWorkflow(r.Context(), session, workflow.UpdateWorkflowRequest{
		WorkflowID:  workflowID,
		Name:        payload.Name,
		Description: payload.Description,
		Status:      payload.Status,
		Definition:  payload.Definition,
		Variables:   payload.Variables,
	})
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, redactWorkflowDefinition(updated))
}

func (h workflowHandler) rollbackWorkflow(w stdhttp.ResponseWriter, r *stdhttp.Request, workflowID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload workflowRollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	rolledBack, err := h.service.RollbackWorkflow(r.Context(), session, workflow.RollbackWorkflowRequest{
		WorkflowID: workflowID,
		Version:    payload.Version,
	})
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, redactWorkflowDefinition(rolledBack))
}

func (h workflowHandler) createWorkflowBranch(w stdhttp.ResponseWriter, r *stdhttp.Request, workflowID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload workflowCreateBranchRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	branched, err := h.service.CreateWorkflowBranch(r.Context(), session, workflow.CreateWorkflowBranchRequest{
		WorkflowID:     workflowID,
		Version:        payload.Version,
		Name:           strings.TrimSpace(payload.Name),
		Description:    strings.TrimSpace(payload.Description),
		ExperimentKey:  strings.TrimSpace(payload.ExperimentKey),
		TrafficPercent: payload.TrafficPercent,
	})
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusCreated, redactWorkflowDefinition(branched))
}

func (h workflowHandler) publishWorkflowBranch(w stdhttp.ResponseWriter, r *stdhttp.Request, workflowID string, branchID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload workflowPublishBranchRequest
	if r.Body != nil && r.Body != stdhttp.NoBody {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
			return
		}
	}

	published, err := h.service.PublishWorkflowBranch(r.Context(), session, workflow.PublishWorkflowBranchRequest{
		BranchID:    branchID,
		Name:        strings.TrimSpace(payload.Name),
		Description: strings.TrimSpace(payload.Description),
	})
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusCreated, redactWorkflowDefinition(published))
}

func (h workflowHandler) mergeWorkflowBranch(w stdhttp.ResponseWriter, r *stdhttp.Request, workflowID string, branchID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	merged, err := h.service.MergeWorkflowBranch(r.Context(), session, workflow.MergeWorkflowBranchRequest{
		BranchID: branchID,
	})
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, redactWorkflowDefinition(merged))
}

func (h workflowHandler) deleteWorkflow(w stdhttp.ResponseWriter, r *stdhttp.Request, workflowID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	archived, err := h.service.DeleteWorkflow(r.Context(), session, workflowID)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, redactWorkflowDefinition(archived))
}

func (h workflowHandler) startExecution(w stdhttp.ResponseWriter, r *stdhttp.Request, workflowID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload workflowStartExecutionRequest
	if r.Body != nil && r.Body != stdhttp.NoBody {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
			return
		}
	}

	execution, err := h.service.StartExecutionWithTrigger(r.Context(), session, workflow.StartExecutionRequest{
		WorkflowID: workflowID,
		Input:      payload.Input,
		Context:    workflowExecutionSessionContext(r.Context(), session),
	})
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	if strings.ToLower(strings.TrimSpace(firstWorkflowNonEmpty(payload.ExecutionMode, payload.ExecutionModeCamel))) == "auto" {
		execution, err = h.service.RunExecutionUntilBlocked(r.Context(), session, execution.ID)
		if err != nil {
			writeWorkflowError(w, err)
			return
		}
	}
	writeSuccess(w, stdhttp.StatusCreated, redactWorkflowExecution(execution))
}

func (h workflowHandler) triggerWebhook(w stdhttp.ResponseWriter, r *stdhttp.Request, workflowID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload map[string]any
	if r.Body != nil && r.Body != stdhttp.NoBody {
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
			return
		}
	}
	if payload == nil {
		payload = map[string]any{}
	}

	execution, err := startTriggeredWorkflowUntilBlocked(r.Context(), h.service, session, workflow.StartExecutionRequest{
		WorkflowID:     workflowID,
		TriggerType:    workflow.WorkflowTriggerWebhook,
		TriggerPayload: map[string]any{"method": r.Method, "payload": payload},
		Input:          payload,
		Context:        map[string]any{"webhookPath": r.URL.Path},
	})
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusCreated, redactWorkflowExecution(execution))
}

func (h workflowHandler) triggerSignedWebhook(w stdhttp.ResponseWriter, r *stdhttp.Request, organizationID string, workflowID string) {
	session := auth.Session{OrganizationID: strings.TrimSpace(organizationID)}
	if session.OrganizationID == "" || strings.TrimSpace(workflowID) == "" {
		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
		return
	}

	definition, err := h.service.GetWorkflow(r.Context(), session, workflowID)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}

	secret := workflowWebhookSecret(definition.Definition)
	if secret == "" {
		writeError(w, stdhttp.StatusBadRequest, "webhook_secret_required", "workflow webhook secret is required")
		return
	}

	rawBody, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	signature := strings.TrimSpace(r.Header.Get("X-Oblivious-Signature"))
	if signature == "" {
		writeError(w, stdhttp.StatusUnauthorized, "webhook_signature_required", "webhook signature is required")
		return
	}

	timestamp := strings.TrimSpace(r.Header.Get("X-Oblivious-Timestamp"))
	if timestamp == "" {
		writeError(w, stdhttp.StatusUnauthorized, "webhook_timestamp_required", "webhook timestamp is required")
		return
	}
	now := time.Now().UTC()
	if h.webhookTimestampNow != nil {
		now = h.webhookTimestampNow().UTC()
	}
	signedAt, err := parseWorkflowWebhookTimestamp(timestamp)
	if err != nil || now.Sub(signedAt) > workflowWebhookTimestampTolerance || signedAt.Sub(now) > workflowWebhookTimestampTolerance {
		writeError(w, stdhttp.StatusUnauthorized, "webhook_timestamp_expired", "webhook timestamp is outside the allowed window")
		return
	}
	if !validWorkflowWebhookSignature(signature, secret, timestamp, rawBody) {
		writeError(w, stdhttp.StatusUnauthorized, "invalid_signature", "invalid webhook signature")
		return
	}
	replayKey := workflowWebhookReplayKey(session.OrganizationID, workflowID, timestamp, signature, rawBody)
	if h.webhookReplayStore != nil && !h.webhookReplayStore.Record(replayKey, signedAt.Add(workflowWebhookTimestampTolerance), now) {
		writeError(w, stdhttp.StatusConflict, "webhook_replay_detected", "webhook replay detected")
		return
	}

	var payload map[string]any
	if len(rawBody) > 0 {
		if err := json.Unmarshal(rawBody, &payload); err != nil {
			writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
			return
		}
	}
	if payload == nil {
		payload = map[string]any{}
	}

	execution, err := startTriggeredWorkflowUntilBlocked(r.Context(), h.service, session, workflow.StartExecutionRequest{
		WorkflowID:     workflowID,
		TriggerType:    workflow.WorkflowTriggerWebhook,
		TriggerPayload: map[string]any{"method": r.Method, "payload": payload, "timestamp": timestamp},
		Input:          payload,
		Context:        map[string]any{"webhookPath": r.URL.Path},
	})
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusCreated, redactWorkflowExecution(execution))
}

func workflowWebhookSecret(definition map[string]any) string {
	if secret := stringValue(definition["webhook_secret"]); secret != "" {
		return secret
	}
	if secret := stringValue(definition["webhookSecret"]); secret != "" {
		return secret
	}
	triggers, ok := definition["triggers"].(map[string]any)
	if !ok {
		return ""
	}
	return workflowWebhookSecretFromTrigger(triggers["webhook"])
}

func workflowWebhookSecretFromTrigger(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		for _, key := range []string{"secret", "webhook_secret", "webhookSecret"} {
			if secret := stringValue(typed[key]); secret != "" {
				return secret
			}
		}
	case []any:
		for _, item := range typed {
			if secret := workflowWebhookSecretFromTrigger(item); secret != "" {
				return secret
			}
		}
	case []map[string]any:
		for _, item := range typed {
			if secret := workflowWebhookSecretFromTrigger(item); secret != "" {
				return secret
			}
		}
	}
	return ""
}

func redactWorkflowDefinitions(definitions []*workflow.WorkflowDefinition) []*workflow.WorkflowDefinition {
	redacted := make([]*workflow.WorkflowDefinition, 0, len(definitions))
	for _, definition := range definitions {
		redacted = append(redacted, redactWorkflowDefinition(definition))
	}
	return redacted
}

func redactWorkflowExecutions(executions []*workflow.WorkflowExecution) []*workflow.WorkflowExecution {
	redacted := make([]*workflow.WorkflowExecution, 0, len(executions))
	for _, execution := range executions {
		redacted = append(redacted, redactWorkflowExecution(execution))
	}
	return redacted
}

func redactWorkflowExecution(execution *workflow.WorkflowExecution) *workflow.WorkflowExecution {
	if execution == nil {
		return nil
	}
	clone := *execution
	clone.WorkflowSnapshot = redactWorkflowDefinitionMap(execution.WorkflowSnapshot)
	if execution.NodeExecutions != nil {
		clone.NodeExecutions = append([]workflow.WorkflowNodeExecution(nil), execution.NodeExecutions...)
	}
	return &clone
}

func redactWorkflowDefinition(definition *workflow.WorkflowDefinition) *workflow.WorkflowDefinition {
	if definition == nil {
		return nil
	}
	clone := *definition
	clone.Definition = redactWorkflowDefinitionMap(definition.Definition)
	return &clone
}

func redactWorkflowDefinitionMap(definition map[string]any) map[string]any {
	if definition == nil {
		return nil
	}
	redacted := make(map[string]any, len(definition))
	for key, value := range definition {
		if isWorkflowSecretKey(key) && stringValue(value) != "" {
			redacted[key] = workflowRedactedSecret
			continue
		}
		redacted[key] = redactWorkflowDefinitionValue(value)
	}
	return redacted
}

func redactWorkflowDefinitionValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return redactWorkflowDefinitionMap(typed)
	case []any:
		redacted := make([]any, 0, len(typed))
		for _, item := range typed {
			redacted = append(redacted, redactWorkflowDefinitionValue(item))
		}
		return redacted
	case []map[string]any:
		redacted := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			redacted = append(redacted, redactWorkflowDefinitionMap(item))
		}
		return redacted
	default:
		return value
	}
}

func restoreRedactedWorkflowSecrets(next map[string]any, existing map[string]any) map[string]any {
	if next == nil {
		return nil
	}
	restored := make(map[string]any, len(next))
	for key, value := range next {
		if isWorkflowSecretKey(key) && stringValue(value) == workflowRedactedSecret {
			if existingSecret := stringValue(existingValue(existing, key)); existingSecret != "" {
				restored[key] = existingSecret
				continue
			}
		}
		restored[key] = restoreRedactedWorkflowSecretValue(value, existingValue(existing, key))
	}
	return restored
}

func restoreRedactedWorkflowSecretValue(next any, existing any) any {
	switch typed := next.(type) {
	case map[string]any:
		existingMap, _ := existing.(map[string]any)
		return restoreRedactedWorkflowSecrets(typed, existingMap)
	case []any:
		restored := make([]any, 0, len(typed))
		for index, item := range typed {
			restored = append(restored, restoreRedactedWorkflowSecretValue(item, existingSequenceItem(existing, index)))
		}
		return restored
	case []map[string]any:
		restored := make([]map[string]any, 0, len(typed))
		for index, item := range typed {
			existingMap, _ := existingSequenceItem(existing, index).(map[string]any)
			restored = append(restored, restoreRedactedWorkflowSecrets(item, existingMap))
		}
		return restored
	default:
		return next
	}
}

func workflowDefinitionHasRedactedSecret(definition map[string]any) bool {
	for key, value := range definition {
		if isWorkflowSecretKey(key) && stringValue(value) == workflowRedactedSecret {
			return true
		}
		switch typed := value.(type) {
		case map[string]any:
			if workflowDefinitionHasRedactedSecret(typed) {
				return true
			}
		case []any:
			for _, item := range typed {
				if itemMap, ok := item.(map[string]any); ok && workflowDefinitionHasRedactedSecret(itemMap) {
					return true
				}
			}
		case []map[string]any:
			for _, item := range typed {
				if workflowDefinitionHasRedactedSecret(item) {
					return true
				}
			}
		}
	}
	return false
}

func existingValue(existing map[string]any, key string) any {
	if existing == nil {
		return nil
	}
	return existing[key]
}

func existingSequenceItem(items any, index int) any {
	if index < 0 {
		return nil
	}
	switch typed := items.(type) {
	case []any:
		if index >= len(typed) {
			return nil
		}
		return typed[index]
	case []map[string]any:
		if index >= len(typed) {
			return nil
		}
		return typed[index]
	default:
		return nil
	}
}

func isWorkflowSecretKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(key), "_", ""))
	return normalized == "secret" || normalized == "webhooksecret"
}

func stringValue(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func validWorkflowWebhookSignature(header string, secret string, timestamp string, rawBody []byte) bool {
	got := strings.TrimSpace(header)
	got = strings.TrimPrefix(got, "sha256=")
	got = strings.TrimSpace(got)
	if got == "" {
		return false
	}

	expectedMAC := hmac.New(sha256.New, []byte(secret))
	_, _ = expectedMAC.Write([]byte(timestamp))
	_, _ = expectedMAC.Write([]byte("."))
	_, _ = expectedMAC.Write(rawBody)
	expected := hex.EncodeToString(expectedMAC.Sum(nil))
	return hmac.Equal([]byte(strings.ToLower(got)), []byte(expected))
}

func parseWorkflowWebhookTimestamp(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, errors.New("webhook timestamp is required")
	}
	if unixSeconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		return time.Unix(unixSeconds, 0).UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, err
	}
	return parsed.UTC(), nil
}

func workflowWebhookReplayKey(organizationID string, workflowID string, timestamp string, signature string, rawBody []byte) string {
	digest := sha256.Sum256(rawBody)
	signature = strings.TrimSpace(strings.TrimPrefix(signature, "sha256="))
	return strings.Join([]string{
		strings.TrimSpace(organizationID),
		strings.TrimSpace(workflowID),
		strings.TrimSpace(timestamp),
		strings.ToLower(signature),
		hex.EncodeToString(digest[:]),
	}, ":")
}

func (h workflowHandler) testNode(w stdhttp.ResponseWriter, r *stdhttp.Request, workflowID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload workflowTestNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	nodeID := strings.TrimSpace(payload.NodeID)
	if nodeID == "" {
		nodeID = strings.TrimSpace(payload.NodeIDLegacy)
	}

	result, err := h.service.TestNode(r.Context(), session, workflowID, nodeID, payload.Input)
	if err != nil {
		if result != nil {
			writeSuccess(w, stdhttp.StatusOK, result)
			return
		}
		writeWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, result)
}

func (h workflowHandler) listExecutions(w stdhttp.ResponseWriter, r *stdhttp.Request, workflowID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	executions, err := h.service.ListExecutions(r.Context(), session, workflowID)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, redactWorkflowExecutions(executions))
}

func (h workflowHandler) getExecution(w stdhttp.ResponseWriter, r *stdhttp.Request, executionID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	execution, err := h.service.GetExecution(r.Context(), session, executionID)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, redactWorkflowExecution(execution))
}

func (h workflowHandler) getExecutionDebugSnapshot(w stdhttp.ResponseWriter, r *stdhttp.Request, executionID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	snapshot, err := h.service.BuildExecutionDebugSnapshot(r.Context(), session, executionID)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, snapshot)
}

func (h workflowHandler) checkResourceLimits(w stdhttp.ResponseWriter, r *stdhttp.Request, workflowID string, executionID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload workflowResourceCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	execution, err := h.service.CheckResourceLimits(r.Context(), session, workflowID, executionID, workflow.WorkflowResourceUsage{
		Now:                payload.Now,
		TotalTokens:        payload.TotalTokens,
		NodeExecutionCount: payload.NodeExecutionCount,
	})
	if err != nil {
		if errors.Is(err, workflow.ErrWorkflowResourceLimit) && execution != nil {
			writeSuccess(w, stdhttp.StatusOK, redactWorkflowExecution(execution))
			return
		}
		writeWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, redactWorkflowExecution(execution))
}

func (h workflowHandler) resolvePausedFailure(w stdhttp.ResponseWriter, r *stdhttp.Request, executionID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload workflowFailureDecisionRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	execution, err := h.service.ResolvePausedFailure(r.Context(), session, executionID, workflow.ResolveFailureDecisionRequest{
		Action:     normalizeWorkflowFailureDecisionAction(payload.Action),
		Input:      payload.Input,
		NextNodeID: strings.TrimSpace(payload.NextNodeID),
		NodeID:     strings.TrimSpace(payload.NodeID),
	})
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	execution, err = runExecutionIfRunnable(r.Context(), h.service, session, execution)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, redactWorkflowExecution(execution))
}

func (h workflowHandler) pauseExecution(w stdhttp.ResponseWriter, r *stdhttp.Request, executionID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	execution, err := h.service.PauseExecution(r.Context(), session, executionID)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, redactWorkflowExecution(execution))
}

func (h workflowHandler) resumeExecution(w stdhttp.ResponseWriter, r *stdhttp.Request, executionID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload workflowResumeExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	execution, err := h.service.ResumeExecution(r.Context(), session, executionID, workflow.ResumeExecutionRequest{
		NodeID: strings.TrimSpace(payload.NodeID),
		Input:  payload.Input,
	})
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	execution, err = runExecutionIfRunnable(r.Context(), h.service, session, execution)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, redactWorkflowExecution(execution))
}

func (h workflowHandler) cancelExecution(w stdhttp.ResponseWriter, r *stdhttp.Request, executionID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	execution, err := h.service.CancelExecution(r.Context(), session, executionID)
	if err != nil {
		writeWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, redactWorkflowExecution(execution))
}

func writeWorkflowError(w stdhttp.ResponseWriter, err error) {
	switch {
	case errors.Is(err, workflow.ErrInvalidInput):
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, workflow.ErrNotFound):
		writeError(w, stdhttp.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, workflow.ErrInvalidTransition):
		writeError(w, stdhttp.StatusConflict, "invalid_state", err.Error())
	case errors.Is(err, workflow.ErrWorkflowResourceLimit):
		writeError(w, stdhttp.StatusConflict, "resource_limit", err.Error())
	default:
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
	}
}

func firstWorkflowNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
