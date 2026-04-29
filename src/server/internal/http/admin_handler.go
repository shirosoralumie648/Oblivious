package http

import (
	"encoding/json"
	stdhttp "net/http"
	"strconv"

	"oblivious/server/internal/admin"
)

type adminHandler struct {
	service *admin.Service
}

func newAdminHandler(service *admin.Service) adminHandler {
	return adminHandler{service: service}
}

func (h adminHandler) getStats(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	stats, err := h.service.GetSystemStats(r.Context())
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, stats)
}

func (h adminHandler) listUsers(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	filter := admin.UserListFilter{
		Search: r.URL.Query().Get("search"),
		Role:   r.URL.Query().Get("role"),
		PlanID: r.URL.Query().Get("planID"),
		Status: r.URL.Query().Get("status"),
		Sort:   r.URL.Query().Get("sort"),
		Limit:  limit,
		Offset: offset,
	}

	users, total, err := h.service.ListUsers(r.Context(), filter)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]interface{}{
		"users": users,
		"total": total,
	})
}

func (h adminHandler) getUser(w stdhttp.ResponseWriter, r *stdhttp.Request, userID string) {
	user, err := h.service.GetUserDetail(r.Context(), userID)
	if err != nil {
		writeError(w, stdhttp.StatusNotFound, "not_found", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, user)
}

type updateUserQuotaRequest struct {
	Balance float64 `json:"balance"`
}

func (h adminHandler) updateUserQuota(w stdhttp.ResponseWriter, r *stdhttp.Request, userID string) {
	var req updateUserQuotaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	if err := h.service.UpdateUserQuota(r.Context(), userID, req.Balance); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	user, _ := h.service.GetUserDetail(r.Context(), userID)
	writeSuccess(w, stdhttp.StatusOK, user)
}

func (h adminHandler) deleteUser(w stdhttp.ResponseWriter, r *stdhttp.Request, userID string) {
	if err := h.service.DeleteUser(r.Context(), userID); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}
	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "deleted"})
}
