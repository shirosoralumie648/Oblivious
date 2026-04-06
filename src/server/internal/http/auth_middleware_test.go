package http

import (
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/config"
)

type stubAuthStore struct {
	session auth.Session
}

func (s stubAuthStore) CreateConversation(context.Context, string) (auth.Conversation, error) {
	panic("unexpected CreateConversation call")
}

func (s stubAuthStore) CreateUserWithWorkspace(context.Context, string, string) (auth.Session, error) {
	panic("unexpected CreateUserWithWorkspace call")
}

func (s stubAuthStore) CreateSessionForUser(context.Context, string, string) (auth.Session, error) {
	panic("unexpected CreateSessionForUser call")
}

func (s stubAuthStore) DeleteSession(context.Context, string) error {
	panic("unexpected DeleteSession call")
}

func (s stubAuthStore) GetConversationsByUser(context.Context, string) ([]auth.Conversation, error) {
	panic("unexpected GetConversationsByUser call")
}

func (s stubAuthStore) GetSession(_ context.Context, sessionID string) (auth.Session, error) {
	if sessionID != s.session.ID {
		return auth.Session{}, auth.ErrSessionNotFound
	}

	return s.session, nil
}

func TestSetSessionCookieSignsSessionID(t *testing.T) {
	middleware := newAuthMiddleware(config.Config{
		SessionCookieName:   "oblivious_session",
		SessionCookieSecure: false,
		SessionSecret:       "test-secret",
	}, auth.NewService(nil))
	recorder := httptest.NewRecorder()

	middleware.setSessionCookie(recorder, auth.Session{
		ID:        "session_123",
		ExpiresAt: time.Unix(1_700_000_000, 0).UTC(),
	})

	response := recorder.Result()
	if len(response.Cookies()) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(response.Cookies()))
	}

	cookie := response.Cookies()[0]
	if cookie.Name != "oblivious_session" {
		t.Fatalf("expected cookie name oblivious_session, got %q", cookie.Name)
	}
	if cookie.Value == "session_123" {
		t.Fatal("expected cookie value to be signed instead of exposing raw session id")
	}
	if cookie.SameSite != stdhttp.SameSiteLaxMode {
		t.Fatalf("expected SameSite Lax, got %v", cookie.SameSite)
	}
}

func TestCurrentSessionRejectsUnsignedCookieValue(t *testing.T) {
	session := auth.Session{
		ID:        "session_123",
		ExpiresAt: time.Unix(1_700_000_000, 0).UTC(),
	}
	middleware := newAuthMiddleware(config.Config{
		SessionCookieName:   "oblivious_session",
		SessionCookieSecure: false,
		SessionSecret:       "test-secret",
	}, auth.NewService(stubAuthStore{session: session}))
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/auth/me", nil)
	request.AddCookie(&stdhttp.Cookie{
		Name:  "oblivious_session",
		Value: "session_123",
	})

	_, ok := middleware.currentSession(request)
	if ok {
		t.Fatal("expected unsigned cookie value to be rejected")
	}
}

func TestCurrentSessionAcceptsSignedCookieValue(t *testing.T) {
	session := auth.Session{
		ID:        "session_123",
		ExpiresAt: time.Unix(1_700_000_000, 0).UTC(),
	}
	middleware := newAuthMiddleware(config.Config{
		SessionCookieName:   "oblivious_session",
		SessionCookieSecure: false,
		SessionSecret:       "test-secret",
	}, auth.NewService(stubAuthStore{session: session}))
	recorder := httptest.NewRecorder()

	middleware.setSessionCookie(recorder, session)

	response := recorder.Result()
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/auth/me", nil)
	request.AddCookie(response.Cookies()[0])

	currentSession, ok := middleware.currentSession(request)
	if !ok {
		t.Fatal("expected signed cookie value to be accepted")
	}
	if currentSession.ID != session.ID {
		t.Fatalf("expected session id %q, got %q", session.ID, currentSession.ID)
	}
}
