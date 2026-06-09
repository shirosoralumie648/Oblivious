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

func TestRegisterChatRoutesDispatchesAppConversationFork(t *testing.T) {
	store := &chatFakeStore{}
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

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/conversations/conversation_1/fork", strings.NewReader(`{"branchFromMessageId":"message_3","title":"Branch"}`))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(&stdhttp.Cookie{
		Name:  "oblivious_session",
		Value: authMiddleware.writeSessionCookieValue("session_1"),
	})
	mux.ServeHTTP(recorder, request)

	if recorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
	}
	if store.lastConversationID != "conversation_1" || store.lastBranchMessageID != "message_3" || store.lastOrganizationID != "org_1" {
		t.Fatalf("unexpected fork route target: conversation=%s branch=%s organization=%s", store.lastConversationID, store.lastBranchMessageID, store.lastOrganizationID)
	}
}

func TestRegisterChatRoutesDispatchesAppConversationStreamAndExport(t *testing.T) {
	store := &chatFakeStore{
		messages: []chat.Message{
			{ID: "message_1", Role: "user", Content: "Plan launch"},
			{ID: "message_2", Role: "assistant", Content: "Launch plan ready"},
		},
	}
	handler := newChatHandler(chat.NewService(store, streamingReplyGenerator{chunks: []string{"Hello", " stream"}}, "demo-reply", nil))
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
	cookie := &stdhttp.Cookie{
		Name:  "oblivious_session",
		Value: authMiddleware.writeSessionCookieValue("session_1"),
	}

	exportRecorder := httptest.NewRecorder()
	exportRequest := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/app/conversations/conversation_1/export.md", nil)
	exportRequest.AddCookie(cookie)
	mux.ServeHTTP(exportRecorder, exportRequest)

	if exportRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected export 200, got %d with body %s", exportRecorder.Code, exportRecorder.Body.String())
	}
	if !strings.HasPrefix(exportRecorder.Header().Get("Content-Type"), "text/markdown") {
		t.Fatalf("expected markdown export content type, got %q", exportRecorder.Header().Get("Content-Type"))
	}
	if body := exportRecorder.Body.String(); !strings.Contains(body, "## User\n\nPlan launch") || !strings.Contains(body, "## Assistant\n\nLaunch plan ready") {
		t.Fatalf("unexpected markdown export body: %s", body)
	}

	streamRecorder := httptest.NewRecorder()
	streamRequest := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/app/conversations/conversation_1/messages/stream", strings.NewReader(`{"content":"hello"}`))
	streamRequest.Header.Set("Content-Type", "application/json")
	streamRequest.AddCookie(cookie)
	mux.ServeHTTP(streamRecorder, streamRequest)

	if streamRecorder.Code != stdhttp.StatusOK {
		t.Fatalf("expected stream 200, got %d with body %s", streamRecorder.Code, streamRecorder.Body.String())
	}
	if !strings.HasPrefix(streamRecorder.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("expected event-stream content type, got %q", streamRecorder.Header().Get("Content-Type"))
	}
	if body := streamRecorder.Body.String(); body != "data: Hello\n\ndata:  stream\n\ndata: [DONE]\n\n" {
		t.Fatalf("unexpected stream body: %q", body)
	}
}
