package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/userprefs"
)

type authHandler struct {
	middleware         authMiddleware
	service            *auth.Service
	preferencesService *userprefs.Service
}

type credentialsRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type sessionUserPayload struct {
	Email string `json:"email"`
	ID    string `json:"id"`
}

type sessionWorkspacePayload struct {
	ID string `json:"id"`
}

type sessionMetaPayload struct {
	ExpiresAt string `json:"expiresAt"`
	ID        string `json:"id"`
}

type sessionResponse struct {
	OnboardingCompleted bool                        `json:"onboardingCompleted"`
	Preferences         userprefs.Preferences       `json:"preferences"`
	Session             sessionMetaPayload          `json:"session"`
	User                sessionUserPayload          `json:"user"`
	Workspace           sessionWorkspacePayload     `json:"workspace"`
}

func newAuthHandler(service *auth.Service, middleware authMiddleware, preferencesService *userprefs.Service) authHandler {
	return authHandler{
		middleware:         middleware,
		service:            service,
		preferencesService: preferencesService,
	}
}

func (h authHandler) login(w http.ResponseWriter, r *http.Request) {
	credentials, ok := decodeCredentials(w, r)
	if !ok {
		return
	}

	session, err := h.service.Login(r.Context(), credentials.Email, credentials.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
			return
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "login failed")
		return
	}

	h.middleware.setSessionCookie(w, session)
	h.writeSessionResponse(w, r, http.StatusOK, session)
}

func (h authHandler) logout(w http.ResponseWriter, r *http.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	if err := h.service.Logout(r.Context(), session.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "logout failed")
		return
	}

	h.middleware.clearSessionCookie(w)
	writeSuccess(w, http.StatusOK, map[string]bool{"loggedOut": true})
}

func (h authHandler) me(w http.ResponseWriter, r *http.Request) {
	session, ok := sessionFromContext(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
		return
	}

	h.writeSessionResponse(w, r, http.StatusOK, session)
}

func (h authHandler) register(w http.ResponseWriter, r *http.Request) {
	credentials, ok := decodeCredentials(w, r)
	if !ok {
		return
	}

	session, err := h.service.Register(r.Context(), credentials.Email, credentials.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "register failed")
		return
	}

	h.middleware.setSessionCookie(w, session)
	h.writeSessionResponse(w, r, http.StatusOK, session)
}

func (h authHandler) writeSessionResponse(w http.ResponseWriter, r *http.Request, status int, session auth.Session) {
	preferences, err := h.preferencesService.Get(r.Context(), session.User.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "load session state failed")
		return
	}

	writeSuccess(w, status, sessionResponse{
		OnboardingCompleted: preferences.OnboardingCompleted,
		Preferences:         preferences,
		Session: sessionMetaPayload{
			ExpiresAt: session.ExpiresAt.UTC().Format(time.RFC3339),
			ID:        session.ID,
		},
		User: sessionUserPayload{Email: session.User.Email, ID: session.User.ID},
		Workspace: sessionWorkspacePayload{ID: session.WorkspaceID},
	})
}

func decodeCredentials(w http.ResponseWriter, r *http.Request) (credentialsRequest, bool) {
	if r.Method == "" {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return credentialsRequest{}, false
	}

	var payload credentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid json body")
		return credentialsRequest{}, false
	}
	if payload.Email == "" || payload.Password == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "email and password are required")
		return credentialsRequest{}, false
	}

	return payload, true
}

func sessionFromContext(r *http.Request) (auth.Session, bool) {
	session, ok := r.Context().Value(sessionContextKey).(auth.Session)
	return session, ok
}
