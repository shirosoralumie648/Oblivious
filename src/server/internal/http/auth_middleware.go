package http

import (
	"context"
	"net/http"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/config"
)

const sessionContextKey contextKey = "session"

type authMiddleware struct {
	config  config.Config
	service *auth.Service
}

func newAuthMiddleware(cfg config.Config, service *auth.Service) authMiddleware {
	return authMiddleware{
		config:  cfg,
		service: service,
	}
}

func (m authMiddleware) currentSession(r *http.Request) (auth.Session, bool) {
	cookie, err := r.Cookie(m.config.SessionCookieName)
	if err != nil || cookie.Value == "" {
		return auth.Session{}, false
	}

	session, err := m.service.Session(r.Context(), cookie.Value)
	if err != nil {
		return auth.Session{}, false
	}

	return session, true
}

func (m authMiddleware) requireSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, ok := m.currentSession(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "unauthorized", "authentication required")
			return
		}

		ctx := context.WithValue(r.Context(), sessionContextKey, session)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m authMiddleware) setSessionCookie(w http.ResponseWriter, session auth.Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     m.config.SessionCookieName,
		Value:    session.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   m.config.SessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  session.ExpiresAt,
	})
}

func (m authMiddleware) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     m.config.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   m.config.SessionCookieSecure,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}
