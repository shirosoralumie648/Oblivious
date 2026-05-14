package http

import (
	"encoding/json"
	stdhttp "net/http"

	"oblivious/server/internal/quota"
)

type quotaHandler struct {
	service *quota.Service
}

func newQuotaHandler(service *quota.Service) quotaHandler {
	return quotaHandler{service: service}
}

// GET /api/v1/app/quota
func (h quotaHandler) getQuota(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	quota, err := h.service.GetBalance(r.Context(), session.User.ID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, quota)
}

// GET /api/v1/app/packages
func (h quotaHandler) listPackages(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	packages, err := h.service.ListPackages(r.Context(), true)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusOK, packages)
}

// POST /api/v1/app/quota/topup
type topupRequest struct {
	Amount float64 `json:"amount"`
}

func (h quotaHandler) topup(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var req topupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	if req.Amount <= 0 {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "amount must be positive")
		return
	}

	if err := h.service.Topup(r.Context(), session.User.ID, req.Amount); err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", err.Error())
		return
	}

	quota, _ := h.service.GetBalance(r.Context(), session.User.ID)
	writeSuccess(w, stdhttp.StatusOK, quota)
}
