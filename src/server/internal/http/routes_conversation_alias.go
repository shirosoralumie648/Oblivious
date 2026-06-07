package http

import (
	stdhttp "net/http"
	"strings"
)

type conversationAliasHandler interface {
	listConversations(stdhttp.ResponseWriter, *stdhttp.Request)
	createConversation(stdhttp.ResponseWriter, *stdhttp.Request)
	getConversation(stdhttp.ResponseWriter, *stdhttp.Request, string)
	updateConversation(stdhttp.ResponseWriter, *stdhttp.Request, string)
	deleteConversation(stdhttp.ResponseWriter, *stdhttp.Request, string)
	forkConversationFromSource(stdhttp.ResponseWriter, *stdhttp.Request, string)
	listMessages(stdhttp.ResponseWriter, *stdhttp.Request, string)
	sendMessage(stdhttp.ResponseWriter, *stdhttp.Request, string)
	updateMessage(stdhttp.ResponseWriter, *stdhttp.Request, string, string)
	deleteMessage(stdhttp.ResponseWriter, *stdhttp.Request, string, string)
	bookmarkMessage(stdhttp.ResponseWriter, *stdhttp.Request, string, string)
	createMessageShare(stdhttp.ResponseWriter, *stdhttp.Request, string, string)
}

func registerConversationAliasRoutes(mux *stdhttp.ServeMux, authMiddleware sessionMiddleware, handler conversationAliasHandler) {
	mux.Handle("/api/v1/conversations", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			handler.listConversations(w, r)
		case stdhttp.MethodPost:
			handler.createConversation(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))

	mux.Handle("/api/v1/conversations/", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/v1/conversations/")
		parts := strings.Split(trimmedPath, "/")
		if len(parts) == 0 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		conversationID := parts[0]
		if len(parts) == 1 {
			switch r.Method {
			case stdhttp.MethodGet:
				handler.getConversation(w, r, conversationID)
			case stdhttp.MethodPut:
				handler.updateConversation(w, r, conversationID)
			case stdhttp.MethodDelete:
				handler.deleteConversation(w, r, conversationID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "messages" {
			switch r.Method {
			case stdhttp.MethodGet:
				handler.listMessages(w, r, conversationID)
			case stdhttp.MethodPost:
				handler.sendMessage(w, r, conversationID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 3 && parts[1] == "messages" && parts[2] != "" {
			switch r.Method {
			case stdhttp.MethodPut:
				handler.updateMessage(w, r, conversationID, parts[2])
			case stdhttp.MethodDelete:
				handler.deleteMessage(w, r, conversationID, parts[2])
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 4 && parts[1] == "messages" && parts[2] != "" {
			switch parts[3] {
			case "bookmark":
				if r.Method != stdhttp.MethodPost {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
					return
				}
				handler.bookmarkMessage(w, r, conversationID, parts[2])
			case "share":
				if r.Method != stdhttp.MethodPost {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
					return
				}
				handler.createMessageShare(w, r, conversationID, parts[2])
			default:
				writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "fork" {
			switch r.Method {
			case stdhttp.MethodPost:
				handler.forkConversationFromSource(w, r, conversationID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	})))
}
