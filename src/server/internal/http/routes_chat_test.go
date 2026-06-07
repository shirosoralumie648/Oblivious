package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"oblivious/server/internal/auth"
	"oblivious/server/internal/chat"
	"oblivious/server/internal/config"
)

func TestRegisterChatRoutesDispatchesAppConversationDetailCRUD(t *testing.T) {
	store := &chatFakeStore{
		conversation: chat.Conversation{
			ID:        "conversation_1",
			Title:     "Launch review",
			CreatedAt: time.Date(2026, time.June, 4, 10, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, time.June, 4, 10, 0, 0, 0, time.UTC),
		},
	}
	handler := newChatHandler(chat.NewService(store, noopReplyGenerator{}, "demo-reply", nil))
	session := auth.Session{
		ID:             "session_1",
		OrganizationID: "org_1",
		WorkspaceID:    "workspace_1",
		User:           auth.User{ID: "user_1", Email: "user@example.com"},
		ExpiresAt:      time.Date(2026, time.June, 5, 10, 0, 0, 0, time.UTC),
	}
	authMiddleware := newAuthMiddleware(config.Config{
		SessionCookieName: "oblivious_session",
		SessionSecret:     "test-secret",
	}, auth.NewService(stubAuthStore{session: session}))
	mux := stdhttp.NewServeMux()
	registerChatRoutes(mux, authMiddleware, handler)

	tests := []struct {
		name               string
		method             string
		path               string
		wantConversationID string
	}{
		{name: "get conversation", method: stdhttp.MethodGet, path: "/api/v1/app/conversations/conversation_1", wantConversationID: "conversation_1"},
		{name: "update conversation", method: stdhttp.MethodPut, path: "/api/v1/app/conversations/conversation_1", wantConversationID: "conversation_1"},
		{name: "delete conversation", method: stdhttp.MethodDelete, path: "/api/v1/app/conversations/conversation_1", wantConversationID: "conversation_1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store.lastConversationID = ""
			store.deletedConversationID = ""
			store.lastOrganizationID = ""
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(`{"title":"Renamed"}`))
			request.AddCookie(&stdhttp.Cookie{
				Name:  "oblivious_session",
				Value: authMiddleware.writeSessionCookieValue("session_1"),
			})
			mux.ServeHTTP(recorder, request)

			if recorder.Code != stdhttp.StatusOK {
				t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
			}
			gotConversationID := store.lastConversationID
			if test.method == stdhttp.MethodDelete {
				gotConversationID = store.deletedConversationID
			}
			if gotConversationID != test.wantConversationID || store.lastOrganizationID != "org_1" {
				t.Fatalf("unexpected route target: conversation=%s organization=%s", gotConversationID, store.lastOrganizationID)
			}
		})
	}
}
