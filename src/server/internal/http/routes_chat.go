package http

import (
	stdhttp "net/http"
	"strings"
)

func registerChatRoutes(mux *stdhttp.ServeMux, authMiddleware authMiddleware, chatHandler chatHandler) {
	mux.Handle("/api/v1/app/models", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		chatHandler.listModels(w, r)
	})))
	mux.Handle("/api/v1/app/conversations", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			chatHandler.listConversations(w, r)
		case stdhttp.MethodPost:
			chatHandler.createConversation(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
	mux.Handle("/api/v1/app/conversations/", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/v1/app/conversations/")
		parts := strings.Split(trimmedPath, "/")
		if len(parts) != 2 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		conversationID := parts[0]
		switch parts[1] {
		case "messages":
			switch r.Method {
			case stdhttp.MethodGet:
				chatHandler.listMessages(w, r, conversationID)
			case stdhttp.MethodPost:
				chatHandler.sendMessage(w, r, conversationID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
		case "config":
			switch r.Method {
			case stdhttp.MethodGet:
				chatHandler.getConversationConfig(w, r, conversationID)
			case stdhttp.MethodPut:
				chatHandler.updateConversationConfig(w, r, conversationID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
		case "convert-to-task":
			switch r.Method {
			case stdhttp.MethodPost:
				chatHandler.convertConversationToTask(w, r, conversationID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
		default:
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
		}
	})))
}
