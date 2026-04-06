package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
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

	sessionID, ok := m.readSessionCookieValue(cookie.Value)
	if !ok {
		return auth.Session{}, false
	}

	session, err := m.service.Session(r.Context(), sessionID)
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
		Value:    m.writeSessionCookieValue(session.ID),
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

func (m authMiddleware) writeSessionCookieValue(sessionID string) string {
	mac := hmac.New(sha256.New, []byte(m.config.SessionSecret))
	mac.Write([]byte(sessionID))

	return sessionID + "." + hex.EncodeToString(mac.Sum(nil))
}

func (m authMiddleware) readSessionCookieValue(cookieValue string) (string, bool) {
	sessionID, signature, found := strings.Cut(cookieValue, ".")
	if !found || sessionID == "" || signature == "" {
		return "", false
	}

	expectedMAC := hmac.New(sha256.New, []byte(m.config.SessionSecret))
	expectedMAC.Write([]byte(sessionID))
	expectedSignature := expectedMAC.Sum(nil)

	providedSignature, err := hex.DecodeString(signature)
	if err != nil {
		return "", false
	}

	if !hmac.Equal(providedSignature, expectedSignature) {
		return "", false
	}

	return sessionID, true
}
