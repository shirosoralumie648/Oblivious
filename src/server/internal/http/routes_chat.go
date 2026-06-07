package http

import (
	stdhttp "net/http"
	"strings"
)

func registerChatRoutes(mux *stdhttp.ServeMux, authMiddleware authMiddleware, chatHandler chatHandler) {
	mux.Handle("/api/v1/app/message-shares/", stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		shareID := strings.TrimPrefix(r.URL.Path, "/api/v1/app/message-shares/")
		if shareID == "" || strings.Contains(shareID, "/") {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}
		chatHandler.getMessageShare(w, r, shareID)
	}))
	mux.Handle("/api/v1/app/conversation-shares/", stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		shareID := strings.TrimPrefix(r.URL.Path, "/api/v1/app/conversation-shares/")
		if shareID == "" || strings.Contains(shareID, "/") {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}
		chatHandler.getConversationShare(w, r, shareID)
	}))
	mux.Handle("/api/v1/app/models", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		chatHandler.listModels(w, r)
	})))
	mux.Handle("/api/v1/app/personas", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.Method != stdhttp.MethodGet {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		chatHandler.listPersonas(w, r)
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
		if len(parts) == 0 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		conversationID := parts[0]
		if len(parts) == 1 {
			switch r.Method {
			case stdhttp.MethodGet:
				chatHandler.getConversation(w, r, conversationID)
			case stdhttp.MethodPut:
				chatHandler.updateConversation(w, r, conversationID)
			case stdhttp.MethodDelete:
				chatHandler.deleteConversation(w, r, conversationID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}
		if len(parts) == 2 && parts[1] == "export.md" {
			if r.Method != stdhttp.MethodGet {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			chatHandler.exportConversationMarkdown(w, r, conversationID)
			return
		}
		if len(parts) == 2 && parts[1] == "share" {
			if r.Method != stdhttp.MethodPost {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			chatHandler.createConversationShare(w, r, conversationID)
			return
		}
		if parts[1] == "messages" {
			if len(parts) == 2 {
				switch r.Method {
				case stdhttp.MethodGet:
					chatHandler.listMessages(w, r, conversationID)
				case stdhttp.MethodPost:
					chatHandler.sendMessage(w, r, conversationID)
				default:
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
				return
			}
			if len(parts) == 3 && parts[2] == "stream" {
				if r.Method != stdhttp.MethodPost {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
					return
				}
				chatHandler.streamMessage(w, r, conversationID)
				return
			}
			if len(parts) == 3 && parts[2] != "" {
				switch r.Method {
				case stdhttp.MethodPut:
					chatHandler.updateMessage(w, r, conversationID, parts[2])
				case stdhttp.MethodDelete:
					chatHandler.deleteMessage(w, r, conversationID, parts[2])
				default:
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
				return
			}
			if len(parts) == 4 && parts[2] != "" {
				switch parts[3] {
				case "bookmark":
					if r.Method != stdhttp.MethodPost {
						writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
						return
					}
					chatHandler.bookmarkMessage(w, r, conversationID, parts[2])
					return
				case "share":
					if r.Method != stdhttp.MethodPost {
						writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
						return
					}
					chatHandler.createMessageShare(w, r, conversationID, parts[2])
					return
				}
			}
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		if len(parts) != 2 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

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
		case "fork":
			switch r.Method {
			case stdhttp.MethodPost:
				chatHandler.forkConversationFromSource(w, r, conversationID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
		default:
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
		}
	})))
}
