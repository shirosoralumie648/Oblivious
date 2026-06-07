package http

import (
	"encoding/json"
	"errors"
	stdhttp "net/http"
	"time"

	"oblivious/server/internal/schedule"
)

type scheduleHandler struct {
	service *schedule.Service
}

type createScheduledTaskRequest struct {
	Name           string `json:"name"`
	TargetType     string `json:"targetType"`
	TargetID       string `json:"targetId"`
	CronExpression string `json:"cronExpression"`
	Enabled        bool   `json:"enabled"`
}

type updateScheduledTaskStatusRequest struct {
	Enabled bool `json:"enabled"`
}

func newScheduleHandler(service *schedule.Service) scheduleHandler {
	return scheduleHandler{service: service}
}

func (h scheduleHandler) listScheduledTasks(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	tasks, err := h.service.List(r.Context(), session)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "list scheduled tasks failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, tasks)
}

func (h scheduleHandler) createScheduledTask(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload createScheduledTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	task, err := h.service.Create(r.Context(), session, schedule.CreateScheduledTaskInput{
		Name:           payload.Name,
		TargetType:     payload.TargetType,
		TargetID:       payload.TargetID,
		CronExpression: payload.CronExpression,
		Enabled:        payload.Enabled,
	})
	if err != nil {
		if isScheduleValidationError(err) {
			writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "create scheduled task failed")
		return
	}

	writeSuccess(w, stdhttp.StatusCreated, task)
}

func (h scheduleHandler) updateScheduledTaskStatus(w stdhttp.ResponseWriter, r *stdhttp.Request, scheduledTaskID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload updateScheduledTaskStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	task, err := h.service.UpdateEnabled(r.Context(), session, scheduledTaskID, payload.Enabled, time.Now().UTC())
	if err != nil {
		if isScheduleValidationError(err) {
			writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "scheduled task not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "update scheduled task status failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, task)
}

func (h scheduleHandler) runScheduledTaskNow(w stdhttp.ResponseWriter, r *stdhttp.Request, scheduledTaskID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	result, err := h.service.RunNow(r.Context(), session, scheduledTaskID)
	if err != nil {
		if isScheduleValidationError(err) {
			writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "scheduled task not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "run scheduled task failed")
		return
	}

	writeSuccess(w, stdhttp.StatusAccepted, result.Run)
}

func (h scheduleHandler) listScheduledTaskRuns(w stdhttp.ResponseWriter, r *stdhttp.Request, scheduledTaskID string) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	runs, err := h.service.ListRuns(r.Context(), session, scheduledTaskID)
	if err != nil {
		if isScheduleValidationError(err) {
			writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "list scheduled task runs failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, runs)
}

func isScheduleValidationError(err error) bool {
	return errors.Is(err, schedule.ErrInvalidCronExpression) ||
		errors.Is(err, schedule.ErrInvalidOrganization) ||
		errors.Is(err, schedule.ErrInvalidRunStatus) ||
		errors.Is(err, schedule.ErrInvalidScheduledTaskName) ||
		errors.Is(err, schedule.ErrInvalidScheduledTaskID) ||
		errors.Is(err, schedule.ErrInvalidTargetID) ||
		errors.Is(err, schedule.ErrInvalidTargetType)
}
