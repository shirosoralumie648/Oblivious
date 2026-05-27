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

type passwordResetRequest struct {
	Email string `json:"email"`
}

type passwordResetConfirmRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type sessionUserPayload struct {
	Email string `json:"email"`
	ID    string `json:"id"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type sessionWorkspacePayload struct {
	ID string `json:"id"`
}

type sessionMetaPayload struct {
	ExpiresAt string `json:"expiresAt"`
	ID        string `json:"id"`
}

type sessionResponse struct {
	OnboardingCompleted bool                    `json:"onboardingCompleted"`
	Preferences         userprefs.Preferences   `json:"preferences"`
	CSRFToken           string                  `json:"csrfToken"`
	Session             sessionMetaPayload      `json:"session"`
	User                sessionUserPayload      `json:"user"`
	Workspace           sessionWorkspacePayload `json:"workspace"`
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

	if !h.checkRateLimit(w, r, "auth.login", credentials.Email, auth.RateLimitPolicy{Limit: 5, Window: 5 * time.Minute, BlockDuration: 15 * time.Minute}) {
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

	if !h.checkRateLimit(w, r, "auth.register", credentials.Email, auth.RateLimitPolicy{Limit: 3, Window: 10 * time.Minute, BlockDuration: 30 * time.Minute}) {
		return
	}

	session, err := h.service.Register(r.Context(), credentials.Email, credentials.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
		return
	}

	h.middleware.setSessionCookie(w, session)
	h.writeSessionResponse(w, r, http.StatusOK, session)
}

func (h authHandler) requestPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req passwordResetRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if !h.checkRateLimit(w, r, "auth.password_reset_request", req.Email, auth.RateLimitPolicy{Limit: 3, Window: 15 * time.Minute, BlockDuration: 30 * time.Minute}) {
		return
	}
	token, err := h.service.RequestPasswordReset(r.Context(), req.Email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "password reset request failed")
		return
	}
	data := map[string]any{"requested": true}
	if h.middleware.config.Env == "test" && token != "" {
		data["token"] = token
	}
	writeSuccess(w, http.StatusOK, data)
}

func (h authHandler) confirmPasswordReset(w http.ResponseWriter, r *http.Request) {
	var req passwordResetConfirmRequest
	if !decodeRequestJSON(w, r, &req) {
		return
	}
	if !h.checkRateLimit(w, r, "auth.password_reset_confirm", req.Token, auth.RateLimitPolicy{Limit: 5, Window: 15 * time.Minute, BlockDuration: 30 * time.Minute}) {
		return
	}
	if err := h.service.ConfirmPasswordReset(r.Context(), req.Token, req.Password); err != nil {
		status := http.StatusBadRequest
		code := "invalid_request"
		if errors.Is(err, auth.ErrInvalidResetToken) {
			code = "invalid_token"
		}
		writeError(w, status, code, err.Error())
		return
	}
	writeSuccess(w, http.StatusOK, map[string]bool{"reset": true})
}

func (h authHandler) checkRateLimit(w http.ResponseWriter, r *http.Request, scope, key string, policy auth.RateLimitPolicy) bool {
	rateKey := clientIP(r) + ":" + key
	if err := h.service.CheckRateLimit(r.Context(), scope, rateKey, policy); err != nil {
		if errors.Is(err, auth.ErrRateLimited) {
			writeError(w, http.StatusTooManyRequests, "rate_limited", "rate limit exceeded")
			return false
		}
		writeError(w, http.StatusInternalServerError, "internal_error", "rate limit check failed")
		return false
	}
	return true
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
		CSRFToken:           h.middleware.csrfToken(session.ID),
		Session: sessionMetaPayload{
			ExpiresAt: session.ExpiresAt.UTC().Format(time.RFC3339),
			ID:        session.ID,
		},
		User: sessionUserPayload{
			Email: session.User.Email,
			ID:    session.User.ID,
			Name:  session.User.Name,
			Role:  session.User.Role,
		},
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
