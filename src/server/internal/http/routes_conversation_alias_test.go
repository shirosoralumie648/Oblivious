package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRegisterConversationAliasRoutesDispatchesCollectionAndMessages(t *testing.T) {
	handler := &conversationAliasFakeHandler{}
	mux := stdhttp.NewServeMux()
	auth := &recordingSessionMiddleware{}
	registerConversationAliasRoutes(mux, auth, handler)

	tests := []struct {
		name          string
		method        string
		path          string
		wantOperation string
		wantID        string
		wantMessageID string
	}{
		{name: "list conversations", method: stdhttp.MethodGet, path: "/api/v1/conversations", wantOperation: "listConversations"},
		{name: "create conversation", method: stdhttp.MethodPost, path: "/api/v1/conversations", wantOperation: "createConversation"},
		{name: "get conversation", method: stdhttp.MethodGet, path: "/api/v1/conversations/conversation_1", wantOperation: "getConversation", wantID: "conversation_1"},
		{name: "update conversation", method: stdhttp.MethodPut, path: "/api/v1/conversations/conversation_1", wantOperation: "updateConversation", wantID: "conversation_1"},
		{name: "delete conversation", method: stdhttp.MethodDelete, path: "/api/v1/conversations/conversation_1", wantOperation: "deleteConversation", wantID: "conversation_1"},
		{name: "fork conversation", method: stdhttp.MethodPost, path: "/api/v1/conversations/conversation_1/fork", wantOperation: "forkConversation", wantID: "conversation_1"},
		{name: "list messages", method: stdhttp.MethodGet, path: "/api/v1/conversations/conversation_1/messages", wantOperation: "listMessages", wantID: "conversation_1"},
		{name: "send message", method: stdhttp.MethodPost, path: "/api/v1/conversations/conversation_1/messages", wantOperation: "sendMessage", wantID: "conversation_1"},
		{name: "update message", method: stdhttp.MethodPut, path: "/api/v1/conversations/conversation_1/messages/message_1", wantOperation: "updateMessage", wantID: "conversation_1", wantMessageID: "message_1"},
		{name: "delete message", method: stdhttp.MethodDelete, path: "/api/v1/conversations/conversation_1/messages/message_1", wantOperation: "deleteMessage", wantID: "conversation_1", wantMessageID: "message_1"},
		{name: "bookmark message", method: stdhttp.MethodPost, path: "/api/v1/conversations/conversation_1/messages/message_1/bookmark", wantOperation: "bookmarkMessage", wantID: "conversation_1", wantMessageID: "message_1"},
		{name: "share message", method: stdhttp.MethodPost, path: "/api/v1/conversations/conversation_1/messages/message_1/share", wantOperation: "createMessageShare", wantID: "conversation_1", wantMessageID: "message_1"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler.reset()
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, strings.NewReader(`{"content":"hello"}`)))

			if recorder.Code != stdhttp.StatusOK {
				t.Fatalf("expected 200, got %d with body %s", recorder.Code, recorder.Body.String())
			}
			if handler.operation != test.wantOperation {
				t.Fatalf("expected operation %q, got %q", test.wantOperation, handler.operation)
			}
			if handler.conversationID != test.wantID {
				t.Fatalf("expected conversation ID %q, got %q", test.wantID, handler.conversationID)
			}
			if handler.messageID != test.wantMessageID {
				t.Fatalf("expected message ID %q, got %q", test.wantMessageID, handler.messageID)
			}
		})
	}

	if auth.requestCalls != len(tests) {
		t.Fatalf("expected session middleware for %d alias requests, got %d", len(tests), auth.requestCalls)
	}
}

func TestRegisterConversationAliasRoutesRejectsUnknownOrInvalidPaths(t *testing.T) {
	handler := &conversationAliasFakeHandler{}
	mux := stdhttp.NewServeMux()
	registerConversationAliasRoutes(mux, passThroughAuthMiddleware{}, handler)

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantCode   string
	}{
		{name: "unsupported collection method", method: stdhttp.MethodPut, path: "/api/v1/conversations", wantStatus: stdhttp.StatusMethodNotAllowed, wantCode: "method_not_allowed"},
		{name: "unknown child route", method: stdhttp.MethodGet, path: "/api/v1/conversations/conversation_1/config", wantStatus: stdhttp.StatusNotFound, wantCode: "not_found"},
		{name: "unknown message action", method: stdhttp.MethodGet, path: "/api/v1/conversations/conversation_1/messages/message_1/unknown", wantStatus: stdhttp.StatusNotFound, wantCode: "not_found"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequest(test.method, test.path, nil))

			if recorder.Code != test.wantStatus {
				t.Fatalf("expected %d, got %d with body %s", test.wantStatus, recorder.Code, recorder.Body.String())
			}
			if !strings.Contains(recorder.Body.String(), `"code":"`+test.wantCode+`"`) {
				t.Fatalf("expected error code %q, got %s", test.wantCode, recorder.Body.String())
			}
		})
	}
}

type conversationAliasFakeHandler struct {
	operation      string
	conversationID string
	messageID      string
}

func (h *conversationAliasFakeHandler) reset() {
	h.operation = ""
	h.conversationID = ""
	h.messageID = ""
}

func (h *conversationAliasFakeHandler) listConversations(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
	h.operation = "listConversations"
	writeSuccess(w, stdhttp.StatusOK, map[string]string{"operation": h.operation})
}

func (h *conversationAliasFakeHandler) createConversation(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
	h.operation = "createConversation"
	writeSuccess(w, stdhttp.StatusOK, map[string]string{"operation": h.operation})
}

func (h *conversationAliasFakeHandler) getConversation(w stdhttp.ResponseWriter, _ *stdhttp.Request, conversationID string) {
	h.operation = "getConversation"
	h.conversationID = conversationID
	writeSuccess(w, stdhttp.StatusOK, map[string]string{"operation": h.operation})
}

func (h *conversationAliasFakeHandler) updateConversation(w stdhttp.ResponseWriter, _ *stdhttp.Request, conversationID string) {
	h.operation = "updateConversation"
	h.conversationID = conversationID
	writeSuccess(w, stdhttp.StatusOK, map[string]string{"operation": h.operation})
}

func (h *conversationAliasFakeHandler) deleteConversation(w stdhttp.ResponseWriter, _ *stdhttp.Request, conversationID string) {
	h.operation = "deleteConversation"
	h.conversationID = conversationID
	writeSuccess(w, stdhttp.StatusOK, map[string]string{"operation": h.operation})
}

func (h *conversationAliasFakeHandler) forkConversationFromSource(w stdhttp.ResponseWriter, _ *stdhttp.Request, conversationID string) {
	h.operation = "forkConversation"
	h.conversationID = conversationID
	writeSuccess(w, stdhttp.StatusOK, map[string]string{"operation": h.operation})
}

func (h *conversationAliasFakeHandler) listMessages(w stdhttp.ResponseWriter, _ *stdhttp.Request, conversationID string) {
	h.operation = "listMessages"
	h.conversationID = conversationID
	writeSuccess(w, stdhttp.StatusOK, map[string]string{"operation": h.operation})
}

func (h *conversationAliasFakeHandler) sendMessage(w stdhttp.ResponseWriter, _ *stdhttp.Request, conversationID string) {
	h.operation = "sendMessage"
	h.conversationID = conversationID
	writeSuccess(w, stdhttp.StatusOK, map[string]string{"operation": h.operation})
}

func (h *conversationAliasFakeHandler) updateMessage(w stdhttp.ResponseWriter, _ *stdhttp.Request, conversationID, messageID string) {
	h.operation = "updateMessage"
	h.conversationID = conversationID
	h.messageID = messageID
	writeSuccess(w, stdhttp.StatusOK, map[string]string{"operation": h.operation})
}

func (h *conversationAliasFakeHandler) deleteMessage(w stdhttp.ResponseWriter, _ *stdhttp.Request, conversationID, messageID string) {
	h.operation = "deleteMessage"
	h.conversationID = conversationID
	h.messageID = messageID
	writeSuccess(w, stdhttp.StatusOK, map[string]string{"operation": h.operation})
}

func (h *conversationAliasFakeHandler) bookmarkMessage(w stdhttp.ResponseWriter, _ *stdhttp.Request, conversationID, messageID string) {
	h.operation = "bookmarkMessage"
	h.conversationID = conversationID
	h.messageID = messageID
	writeSuccess(w, stdhttp.StatusOK, map[string]string{"operation": h.operation})
}

func (h *conversationAliasFakeHandler) createMessageShare(w stdhttp.ResponseWriter, _ *stdhttp.Request, conversationID, messageID string) {
	h.operation = "createMessageShare"
	h.conversationID = conversationID
	h.messageID = messageID
	writeSuccess(w, stdhttp.StatusOK, map[string]string{"operation": h.operation})
}
