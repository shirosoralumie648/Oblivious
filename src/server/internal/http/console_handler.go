package http

import (
	stdhttp "net/http"

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
