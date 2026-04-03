package http

import (
	"encoding/json"
	stdhttp "net/http"
	"strings"

	"oblivious/server/internal/task"
)

type taskHandler struct {
	service *task.Service
}

type createTaskRequest struct {
	AuthorizationScope string   `json:"authorizationScope"`
	BudgetLimit        int      `json:"budgetLimit"`
	ExecutionMode      string   `json:"executionMode"`
	Goal               string   `json:"goal"`
	KnowledgeBaseIDs   []string `json:"knowledgeBaseIds"`
	ToolAllowList      []string `json:"toolAllowList"`
	ToolDenyList       []string `json:"toolDenyList"`
	Title              string   `json:"title"`
}

type updateTaskBudgetRequest struct {
	BudgetLimit int `json:"budgetLimit"`
}

func newTaskHandler(service *task.Service) taskHandler {
	return taskHandler{service: service}
}

func (h taskHandler) listTasks(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	tasks, err := h.service.List(r.Context(), session)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "list tasks failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, tasks)
}

func (h taskHandler) getTask(w stdhttp.ResponseWriter, r *stdhttp.Request, taskID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	detail, err := h.service.Get(r.Context(), session, taskID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "get task failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, detail)
}

func (h taskHandler) createTask(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload createTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}
	if strings.TrimSpace(payload.Goal) == "" {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "goal is required")
		return
	}

	createdTask, err := h.service.Create(
		r.Context(),
		session,
		strings.TrimSpace(payload.Title),
		strings.TrimSpace(payload.Goal),
		strings.TrimSpace(payload.ExecutionMode),
		strings.TrimSpace(payload.AuthorizationScope),
		payload.BudgetLimit,
		payload.KnowledgeBaseIDs,
		payload.ToolAllowList,
		payload.ToolDenyList,
	)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "create task failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, createdTask)
}

func (h taskHandler) startTask(w stdhttp.ResponseWriter, r *stdhttp.Request, taskID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	detail, err := h.service.Start(r.Context(), session, taskID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "start task failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, detail)
}

func (h taskHandler) approveTask(w stdhttp.ResponseWriter, r *stdhttp.Request, taskID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	detail, err := h.service.Approve(r.Context(), session, taskID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "approve task failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, detail)
}

func (h taskHandler) pauseTask(w stdhttp.ResponseWriter, r *stdhttp.Request, taskID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	detail, err := h.service.Pause(r.Context(), session, taskID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "pause task failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, detail)
}

func (h taskHandler) resumeTask(w stdhttp.ResponseWriter, r *stdhttp.Request, taskID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	detail, err := h.service.Resume(r.Context(), session, taskID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "resume task failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, detail)
}

func (h taskHandler) cancelTask(w stdhttp.ResponseWriter, r *stdhttp.Request, taskID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	detail, err := h.service.Cancel(r.Context(), session, taskID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "cancel task failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, detail)
}

func (h taskHandler) updateTaskBudget(w stdhttp.ResponseWriter, r *stdhttp.Request, taskID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload updateTaskBudgetRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	detail, err := h.service.UpdateBudget(r.Context(), session, taskID, payload.BudgetLimit)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "update budget failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, detail)
}
