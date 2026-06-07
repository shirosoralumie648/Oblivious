package http

import (
	"encoding/json"
	stdhttp "net/http"
	"strings"

	"oblivious/server/internal/console"
	"oblivious/server/internal/userprefs"
)

type consoleHandler struct {
	preferencesService *userprefs.Service
	service            *console.Service
}

func newConsoleHandler(service *console.Service, preferencesService *userprefs.Service) consoleHandler {
	return consoleHandler{
		preferencesService: preferencesService,
		service:            service,
	}
}

func (h consoleHandler) getUsage(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	summary, err := h.service.GetUsage(r.Context(), session)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "get usage summary failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, summary)
}

func (h consoleHandler) getAccess(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	preferences, err := h.preferencesService.Get(r.Context(), session.User.ID)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "get access summary failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, h.service.GetAccess(session, preferences))
}

func (h consoleHandler) getModels(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	models, err := h.service.GetModels(r.Context(), session)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "get model summaries failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, models)
}

func (h consoleHandler) getBilling(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	summary, err := h.service.GetBilling(r.Context(), session)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "get billing summary failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, summary)
}

func (h consoleHandler) listBillingInvoices(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	invoices, err := h.service.ListBillingInvoices(r.Context(), session)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "list billing invoices failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, invoices)
}

func (h consoleHandler) listAPITokens(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	tokens, err := h.service.ListAPITokens(r.Context(), session)
	if err != nil {
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "list api tokens failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, tokens)
}

func (h consoleHandler) createAPIToken(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var input console.CreateAPITokenRequest
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", "invalid api token request")
		return
	}
	created, err := h.service.CreateAPIToken(r.Context(), session, input)
	if err != nil {
		writeError(w, stdhttp.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	writeSuccess(w, stdhttp.StatusCreated, created)
}

func (h consoleHandler) listAPITokenUsage(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	tokenID := apiTokenIDFromUsagePath(r.URL.Path)
	usage, err := h.service.ListAPITokenUsage(r.Context(), session, tokenID)
	if err != nil {
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "api token not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "list api token usage failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, usage)
}

func (h consoleHandler) revokeAPIToken(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, stdhttp.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	tokenID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/console/api-tokens/"), "/")
	if err := h.service.RevokeAPIToken(r.Context(), session, tokenID); err != nil {
		if isNotFoundError(err) {
			writeError(w, stdhttp.StatusNotFound, "not_found", "api token not found")
			return
		}
		writeError(w, stdhttp.StatusInternalServerError, "internal_error", "revoke api token failed")
		return
	}

	writeSuccess(w, stdhttp.StatusOK, map[string]string{"status": "revoked"})
}

func apiTokenIDFromUsagePath(path string) string {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/api/v1/console/api-tokens/"), "/")
	return strings.TrimSuffix(trimmed, "/usage")
}
