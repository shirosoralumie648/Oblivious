package http

import (
	"encoding/json"
	"net/http"

	"oblivious/server/internal/userprefs"
)

type preferencesHandler struct {
	service *userprefs.Service
}

type updatePreferencesRequest struct {
	DefaultMode         string `json:"defaultMode"`
	ModelStrategy       string `json:"modelStrategy"`
	NetworkEnabledHint  bool   `json:"networkEnabledHint"`
	OnboardingCompleted bool   `json:"onboardingCompleted"`
}

func newPreferencesHandler(service *userprefs.Service) preferencesHandler {
	return preferencesHandler{service: service}
}

func (h preferencesHandler) get(w http.ResponseWriter, r *http.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	preferences, err := h.service.Get(r.Context(), session.User.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "get preferences failed")
		return
	}

	writeSuccess(w, http.StatusOK, preferences)
}

func (h preferencesHandler) update(w http.ResponseWriter, r *http.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	var payload updatePreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid json body")
		return
	}

	preferences, err := h.service.Update(r.Context(), session.User.ID, userprefs.Preferences{
		DefaultMode:         payload.DefaultMode,
		ModelStrategy:       payload.ModelStrategy,
		NetworkEnabledHint:  payload.NetworkEnabledHint,
		OnboardingCompleted: payload.OnboardingCompleted,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "update preferences failed")
		return
	}

	writeSuccess(w, http.StatusOK, preferences)
}
