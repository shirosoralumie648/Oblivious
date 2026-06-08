package http

import (
	"encoding/json"
	"errors"
	"io"
	stdhttp "net/http"
	"strings"

	"oblivious/server/internal/agent"
	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
)

type agentRunsHandler struct {
	service *agent.Service
}

func newAgentRunsHandler(service *agent.Service) agentRunsHandler {
	return agentRunsHandler{service: service}
}

type agentRunCreateRequest struct {
	AgentID             string            `json:"agent_id"`
	AgentIDCamel        string            `json:"agentId"`
	ConversationID      string            `json:"conversation_id"`
	ConversationIDCamel string            `json:"conversationId"`
	Input               string            `json:"input"`
	Message             string            `json:"message"`
	Mode                agentRunModeField `json:"mode"`
	TokenBudget         *int              `json:"token_budget"`
	TokenBudgetCamel    *int              `json:"tokenBudget"`
	MaxIterations       *int              `json:"max_iterations"`
	MaxIterationsCamel  *int              `json:"maxIterations"`
}

type agentRunModeField struct {
	Value string
	Set   bool
}

func (f *agentRunModeField) UnmarshalJSON(data []byte) error {
	f.Set = true
	if string(data) == "null" {
		f.Value = ""
		return nil
	}
	return json.Unmarshal(data, &f.Value)
}

type agentRunToolRequest struct {
	ToolRunID      string `json:"tool_run_id"`
	ToolRunIDCamel string `json:"toolRunId"`
	Reason         string `json:"reason"`
}

type agentRunPlanStepRequest struct {
	PlanStepID      string `json:"plan_step_id"`
	PlanStepIDCamel string `json:"planStepId"`
	Reason          string `json:"reason"`
}

type agentRunPlanStepUpdateRequest struct {
	PlanStepID      string         `json:"plan_step_id"`
	PlanStepIDCamel string         `json:"planStepId"`
	Title           *string        `json:"title"`
	ToolName        *string        `json:"tool_name"`
	ToolNameCamel   *string        `json:"toolName"`
	Input           map[string]any `json:"input"`
}

type agentRunPlanStepCreateRequest struct {
	AfterPlanStepID      *string        `json:"after_plan_step_id"`
	AfterPlanStepIDCamel *string        `json:"afterPlanStepId"`
	Title                string         `json:"title"`
	ToolName             string         `json:"tool_name"`
	ToolNameCamel        string         `json:"toolName"`
	Input                map[string]any `json:"input"`
}

type agentRunPlanStepMoveRequest struct {
	PlanStepID      string `json:"plan_step_id"`
	PlanStepIDCamel string `json:"planStepId"`
	Direction       string `json:"direction"`
}

type agentRunResponse struct {
	ID        string            `json:"id"`
	Status    string            `json:"status"`
	Run       *agent.Run        `json:"run"`
	ToolRuns  []*agent.ToolRun  `json:"toolRuns"`
	PlanSteps []*agent.PlanStep `json:"planSteps,omitempty"`
	Messages  []*agent.Message  `json:"messages"`
}

func (h agentRunsHandler) createRun(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req agentRunCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	agentID := strings.TrimSpace(firstAgentRunNonEmpty(req.AgentID, req.AgentIDCamel))
	conversationID := strings.TrimSpace(firstAgentRunNonEmpty(req.ConversationID, req.ConversationIDCamel))
	input := strings.TrimSpace(firstAgentRunNonEmpty(req.Input, req.Message))
	if agentID == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "agent_id is required")
		return
	}
	if conversationID == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "conversation_id is required")
		return
	}
	if input == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "input is required")
		return
	}

	mode := strings.ToLower(strings.TrimSpace(req.Mode.Value))
	if !req.Mode.Set {
		defaultMode, err := h.service.DefaultExecutionModeForRun(r.Context(), session, agentID)
		if err != nil {
			writeAgentWorkflowError(w, err)
			return
		}
		mode = defaultMode
	} else if mode == "" || (mode != agent.ExecutionModeReact && mode != agent.ExecutionModePlanning) {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "mode must be react or planning")
		return
	}

	ctx := chat.WithRelayRequestMetadata(r.Context(), chat.RelayRequestMetadata{
		OrganizationID: session.OrganizationID,
		UserID:         session.User.ID,
		WorkspaceID:    session.WorkspaceID,
		RequestID:      requestIDFromContext(r.Context()),
	})
	startReq := agent.StartRunRequest{
		AgentID:        agentID,
		ConversationID: conversationID,
		Input:          input,
		MaxIterations:  firstIntPointer(req.MaxIterations, req.MaxIterationsCamel),
		TokenBudget:    firstIntPointer(req.TokenBudget, req.TokenBudgetCamel),
	}
	var result *agent.RunWithMessages
	var err error
	if mode == agent.ExecutionModePlanning {
		result, err = h.service.StartPlanningRun(ctx, session, startReq)
	} else {
		result, err = h.service.StartRun(ctx, session, startReq)
	}
	if err != nil {
		writeAgentWorkflowError(w, err)
		return
	}

	writeSuccess(w, stdhttp.StatusCreated, newAgentRunResponse(result))
}

func (h agentRunsHandler) getRun(w stdhttp.ResponseWriter, r *stdhttp.Request, runID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	result, err := h.service.GetRunWithMessages(r.Context(), session, strings.TrimSpace(runID))
	if err != nil {
		writeAgentWorkflowError(w, err)
		return
	}

	writeSuccess(w, stdhttp.StatusOK, newAgentRunResponse(result))
}

func (h agentRunsHandler) approveTool(w stdhttp.ResponseWriter, r *stdhttp.Request, runID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req agentRunToolRequest
	if err := decodeOptionalAgentRunJSONBody(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	toolRunID, ok := h.resolveToolRunID(w, r, session, runID, firstAgentRunNonEmpty(req.ToolRunID, req.ToolRunIDCamel), func(toolRun *agent.ToolRun) bool {
		return toolRun.Status == agent.ToolRunStatusPendingApproval && toolRun.ApprovalStatus == agent.ApprovalStatusPending
	})
	if !ok {
		return
	}

	if _, err := h.service.ApproveToolRun(r.Context(), session, toolRunID, strings.TrimSpace(req.Reason)); err != nil {
		writeAgentWorkflowError(w, err)
		return
	}
	result, err := h.service.GetRunWithMessages(r.Context(), session, strings.TrimSpace(runID))
	if err != nil {
		writeAgentWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, newAgentRunResponse(result))
}

func (h agentRunsHandler) rejectTool(w stdhttp.ResponseWriter, r *stdhttp.Request, runID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req agentRunToolRequest
	if err := decodeOptionalAgentRunJSONBody(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	toolRunID, ok := h.resolveToolRunID(w, r, session, runID, firstAgentRunNonEmpty(req.ToolRunID, req.ToolRunIDCamel), func(toolRun *agent.ToolRun) bool {
		return toolRun.Status == agent.ToolRunStatusPendingApproval && toolRun.ApprovalStatus == agent.ApprovalStatusPending
	})
	if !ok {
		return
	}

	if _, err := h.service.RejectToolRun(r.Context(), session, toolRunID, strings.TrimSpace(req.Reason)); err != nil {
		writeAgentWorkflowError(w, err)
		return
	}
	result, err := h.service.GetRunWithMessages(r.Context(), session, strings.TrimSpace(runID))
	if err != nil {
		writeAgentWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, newAgentRunResponse(result))
}

func (h agentRunsHandler) retryTool(w stdhttp.ResponseWriter, r *stdhttp.Request, runID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req agentRunToolRequest
	if err := decodeOptionalAgentRunJSONBody(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	toolRunID, ok := h.resolveToolRunID(w, r, session, runID, firstAgentRunNonEmpty(req.ToolRunID, req.ToolRunIDCamel), func(toolRun *agent.ToolRun) bool {
		return toolRun.Status == agent.ToolRunStatusFailed
	})
	if !ok {
		return
	}

	if _, err := h.service.RetryToolRun(r.Context(), session, toolRunID); err != nil {
		writeAgentWorkflowError(w, err)
		return
	}
	result, err := h.service.GetRunWithMessages(r.Context(), session, strings.TrimSpace(runID))
	if err != nil {
		writeAgentWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, newAgentRunResponse(result))
}

func (h agentRunsHandler) approvePlanStep(w stdhttp.ResponseWriter, r *stdhttp.Request, runID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req agentRunPlanStepRequest
	if err := decodeOptionalAgentRunJSONBody(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	planStepID, ok := h.resolvePlanStepID(w, r, session, runID, firstAgentRunNonEmpty(req.PlanStepID, req.PlanStepIDCamel), func(step *agent.PlanStep) bool {
		return step.Status == agent.PlanStepStatusPending
	})
	if !ok {
		return
	}

	if _, err := h.service.ApprovePlanStep(r.Context(), session, planStepID, strings.TrimSpace(req.Reason)); err != nil {
		writeAgentWorkflowError(w, err)
		return
	}
	result, err := h.service.GetRunWithMessages(r.Context(), session, strings.TrimSpace(runID))
	if err != nil {
		writeAgentWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, newAgentRunResponse(result))
}

func (h agentRunsHandler) skipPlanStep(w stdhttp.ResponseWriter, r *stdhttp.Request, runID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req agentRunPlanStepRequest
	if err := decodeOptionalAgentRunJSONBody(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	planStepID, ok := h.resolvePlanStepID(w, r, session, runID, firstAgentRunNonEmpty(req.PlanStepID, req.PlanStepIDCamel), func(step *agent.PlanStep) bool {
		return step.Status == agent.PlanStepStatusPending || step.Status == agent.PlanStepStatusApproved || step.Status == agent.PlanStepStatusFailed
	})
	if !ok {
		return
	}

	if _, err := h.service.SkipPlanStep(r.Context(), session, planStepID, strings.TrimSpace(req.Reason)); err != nil {
		writeAgentWorkflowError(w, err)
		return
	}
	result, err := h.service.GetRunWithMessages(r.Context(), session, strings.TrimSpace(runID))
	if err != nil {
		writeAgentWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, newAgentRunResponse(result))
}

func (h agentRunsHandler) createPlanStep(w stdhttp.ResponseWriter, r *stdhttp.Request, runID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req agentRunPlanStepCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	toolName := firstAgentRunNonEmpty(req.ToolName, req.ToolNameCamel)
	afterPlanStepID := firstStringPointer(req.AfterPlanStepID, req.AfterPlanStepIDCamel)
	if _, err := h.service.CreatePlanStepDraft(r.Context(), session, strings.TrimSpace(runID), agent.CreatePlanStepDraftRequest{
		AfterPlanStepID: afterPlanStepID,
		Title:           req.Title,
		ToolName:        toolName,
		Input:           req.Input,
	}); err != nil {
		writeAgentWorkflowError(w, err)
		return
	}
	result, err := h.service.GetRunWithMessages(r.Context(), session, strings.TrimSpace(runID))
	if err != nil {
		writeAgentWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusCreated, newAgentRunResponse(result))
}

func (h agentRunsHandler) updatePlanStep(w stdhttp.ResponseWriter, r *stdhttp.Request, runID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req agentRunPlanStepUpdateRequest
	if err := decodeOptionalAgentRunJSONBody(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	planStepID, ok := h.resolvePlanStepID(w, r, session, runID, firstAgentRunNonEmpty(req.PlanStepID, req.PlanStepIDCamel), func(step *agent.PlanStep) bool {
		return step.Status == agent.PlanStepStatusPending || step.Status == agent.PlanStepStatusApproved
	})
	if !ok {
		return
	}

	toolName := firstStringPointer(req.ToolName, req.ToolNameCamel)
	if _, err := h.service.UpdatePlanStepDraft(r.Context(), session, planStepID, agent.UpdatePlanStepDraftRequest{
		Title:    req.Title,
		ToolName: toolName,
		Input:    req.Input,
	}); err != nil {
		writeAgentWorkflowError(w, err)
		return
	}
	result, err := h.service.GetRunWithMessages(r.Context(), session, strings.TrimSpace(runID))
	if err != nil {
		writeAgentWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, newAgentRunResponse(result))
}

func (h agentRunsHandler) movePlanStep(w stdhttp.ResponseWriter, r *stdhttp.Request, runID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req agentRunPlanStepMoveRequest
	if err := decodeOptionalAgentRunJSONBody(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	planStepID, ok := h.resolvePlanStepID(w, r, session, runID, firstAgentRunNonEmpty(req.PlanStepID, req.PlanStepIDCamel), func(step *agent.PlanStep) bool {
		return step.Status == agent.PlanStepStatusPending || step.Status == agent.PlanStepStatusApproved
	})
	if !ok {
		return
	}

	if _, err := h.service.MovePlanStep(r.Context(), session, planStepID, req.Direction); err != nil {
		writeAgentWorkflowError(w, err)
		return
	}
	result, err := h.service.GetRunWithMessages(r.Context(), session, strings.TrimSpace(runID))
	if err != nil {
		writeAgentWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, newAgentRunResponse(result))
}

func (h agentRunsHandler) deletePlanStep(w stdhttp.ResponseWriter, r *stdhttp.Request, runID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req agentRunPlanStepRequest
	if err := decodeOptionalAgentRunJSONBody(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	planStepID, ok := h.resolvePlanStepID(w, r, session, runID, firstAgentRunNonEmpty(req.PlanStepID, req.PlanStepIDCamel), func(step *agent.PlanStep) bool {
		return step.Status == agent.PlanStepStatusPending || step.Status == agent.PlanStepStatusApproved
	})
	if !ok {
		return
	}

	if _, err := h.service.DeletePlanStepDraft(r.Context(), session, planStepID); err != nil {
		writeAgentWorkflowError(w, err)
		return
	}
	result, err := h.service.GetRunWithMessages(r.Context(), session, strings.TrimSpace(runID))
	if err != nil {
		writeAgentWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, newAgentRunResponse(result))
}

func (h agentRunsHandler) executePlanStep(w stdhttp.ResponseWriter, r *stdhttp.Request, runID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req agentRunPlanStepRequest
	if err := decodeOptionalAgentRunJSONBody(r, &req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	planStepID, ok := h.resolvePlanStepID(w, r, session, runID, firstAgentRunNonEmpty(req.PlanStepID, req.PlanStepIDCamel), func(step *agent.PlanStep) bool {
		return step.Status == agent.PlanStepStatusApproved || (step.Status == agent.PlanStepStatusPending && step.ApprovalStatus == agent.ApprovalStatusNotRequired)
	})
	if !ok {
		return
	}

	if _, err := h.service.ExecutePlanStep(r.Context(), session, planStepID); err != nil {
		writeAgentWorkflowError(w, err)
		return
	}
	result, err := h.service.GetRunWithMessages(r.Context(), session, strings.TrimSpace(runID))
	if err != nil {
		writeAgentWorkflowError(w, err)
		return
	}
	writeSuccess(w, stdhttp.StatusOK, newAgentRunResponse(result))
}

func (h agentRunsHandler) listTools(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	agentID := strings.TrimSpace(firstAgentRunNonEmpty(r.URL.Query().Get("agentId"), r.URL.Query().Get("agent_id")))
	if agentID == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "agentId is required")
		return
	}

	tools, err := h.service.ListAvailableTools(r.Context(), session, agentID)
	if err != nil {
		if err.Error() == "agent not found" {
			writeError(w, stdhttp.StatusNotFound, "not_found", err.Error())
			return
		}
		if err.Error() == "access denied" {
			writeError(w, stdhttp.StatusForbidden, "forbidden", err.Error())
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, tools)
}

func (h agentRunsHandler) resolveToolRunID(w stdhttp.ResponseWriter, r *stdhttp.Request, session auth.Session, runID, requestedToolRunID string, match func(*agent.ToolRun) bool) (string, bool) {
	detail, err := h.service.GetRunDetail(r.Context(), session, strings.TrimSpace(runID))
	if err != nil {
		writeAgentWorkflowError(w, err)
		return "", false
	}

	requestedToolRunID = strings.TrimSpace(requestedToolRunID)
	if requestedToolRunID != "" {
		for _, toolRun := range detail.ToolRuns {
			if toolRun.ID == requestedToolRunID {
				return requestedToolRunID, true
			}
		}
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "toolRunId does not belong to run")
		return "", false
	}

	var selected string
	for _, toolRun := range detail.ToolRuns {
		if !match(toolRun) {
			continue
		}
		if selected != "" {
			writeError(w, stdhttp.StatusBadRequest, "invalid_request", "toolRunId is required when multiple matching tool runs exist")
			return "", false
		}
		selected = toolRun.ID
	}
	if selected == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "toolRunId is required when no matching tool run exists")
		return "", false
	}
	return selected, true
}

func (h agentRunsHandler) resolvePlanStepID(w stdhttp.ResponseWriter, r *stdhttp.Request, session auth.Session, runID, requestedPlanStepID string, match func(*agent.PlanStep) bool) (string, bool) {
	detail, err := h.service.GetRunDetail(r.Context(), session, strings.TrimSpace(runID))
	if err != nil {
		writeAgentWorkflowError(w, err)
		return "", false
	}

	requestedPlanStepID = strings.TrimSpace(requestedPlanStepID)
	if requestedPlanStepID != "" {
		for _, step := range detail.PlanSteps {
			if step.ID == requestedPlanStepID {
				return requestedPlanStepID, true
			}
		}
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "planStepId does not belong to run")
		return "", false
	}

	var selected string
	for _, step := range detail.PlanSteps {
		if !match(step) {
			continue
		}
		if selected != "" {
			writeError(w, stdhttp.StatusBadRequest, "invalid_request", "planStepId is required when multiple matching plan steps exist")
			return "", false
		}
		selected = step.ID
	}
	if selected == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "planStepId is required when no matching plan step exists")
		return "", false
	}
	return selected, true
}

func newAgentRunResponse(result *agent.RunWithMessages) agentRunResponse {
	response := agentRunResponse{}
	if result == nil {
		return response
	}
	response.Run = result.Run
	response.ToolRuns = result.ToolRuns
	response.PlanSteps = result.PlanSteps
	response.Messages = result.Messages
	if result.Run != nil {
		response.ID = result.Run.ID
		response.Status = result.Run.Status
	}
	return response
}

func decodeOptionalAgentRunJSONBody(r *stdhttp.Request, target any) error {
	if r.Body == nil {
		return nil
	}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return err
	}
	return nil
}

func firstAgentRunNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstIntPointer(values ...*int) *int {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func firstStringPointer(values ...*string) *string {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}
