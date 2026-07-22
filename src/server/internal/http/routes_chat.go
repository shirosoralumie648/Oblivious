package http

import (
	stdhttp "net/http"
	"strings"
)

func chatRouteSurfaceOperations() []OperationContractMetadataV1 {
	return routeSurfaceOperationsFromSpecs([]routeSurfaceOperationSpec{
		{"GET", "/api/v1/app/conversation-shares/{shareId}", "getConversationShare", "public", false, "chat.conversation_use", "", "none", "", "200", "application/json", "inline", "sha256:69087f563f2beb796f810f49ee9f38e4edb8d033ea868022aeabeb868f0fcacf"},
		{"GET", "/api/v1/app/conversations", "listConversations", "cookie", false, "chat.conversation_use", "", "none", "", "200", "application/json", "inline", "sha256:72453e54e4de63bab0fe91273be56130336c9cb73ef94633c9b869cb2b0dd09f"},
		{"POST", "/api/v1/app/conversations", "createConversation", "cookie+csrf", true, "chat.conversation_use", "application/json", "ref", "#/components/schemas/CreateConversationRequest", "200", "application/json", "inline", "sha256:f1f6b9205d914a18251c6c466cb6fa898b2094cc6bb12b8d6444c110aeee0981"},
		{"DELETE", "/api/v1/app/conversations/{conversationId}", "deleteConversation", "cookie+csrf", true, "chat.conversation_use", "", "none", "", "200", "application/json", "inline", "sha256:b384c15ff9aa5ec7791da9dd63b400e82381096af40f0d166bb649762d269876"},
		{"GET", "/api/v1/app/conversations/{conversationId}", "getConversation", "cookie", false, "chat.conversation_use", "", "none", "", "200", "application/json", "inline", "sha256:f1f6b9205d914a18251c6c466cb6fa898b2094cc6bb12b8d6444c110aeee0981"},
		{"PUT", "/api/v1/app/conversations/{conversationId}", "updateConversation", "cookie+csrf", true, "chat.conversation_use", "application/json", "ref", "#/components/schemas/UpdateConversationRequest", "200", "application/json", "inline", "sha256:f1f6b9205d914a18251c6c466cb6fa898b2094cc6bb12b8d6444c110aeee0981"},
		{"GET", "/api/v1/app/conversations/{conversationId}/config", "getConversationConfig", "cookie", false, "chat.conversation_use", "", "none", "", "200", "application/json", "inline", "sha256:d1136472d3ebc74b9e5a5401a0a62140617e4a979398a51be8f9588726658063"},
		{"PUT", "/api/v1/app/conversations/{conversationId}/config", "updateConversationConfig", "cookie+csrf", true, "chat.conversation_use", "application/json", "ref", "#/components/schemas/UpdateConversationConfigRequest", "200", "application/json", "inline", "sha256:d1136472d3ebc74b9e5a5401a0a62140617e4a979398a51be8f9588726658063"},
		{"POST", "/api/v1/app/conversations/{conversationId}/convert-to-task", "convertConversationToTask", "cookie+csrf", true, "chat.conversation_use", "", "none", "", "200", "application/json", "inline", "sha256:89577119329140d65a9e0b2cbafc66ce01d912b5acf05f1884f06674fb601892"},
		{"GET", "/api/v1/app/conversations/{conversationId}/export.md", "exportConversationMarkdown", "cookie", false, "chat.export", "", "none", "", "200", "text/markdown", "inline", "sha256:00404e686415370f1711c4d7acfa2905444d3cf23cef2e10c47d445ebe690f96"},
		{"POST", "/api/v1/app/conversations/{conversationId}/fork", "forkConversation", "cookie+csrf", true, "chat.conversation_use", "application/json", "ref", "#/components/schemas/ForkConversationRequest", "200", "application/json", "inline", "sha256:f1f6b9205d914a18251c6c466cb6fa898b2094cc6bb12b8d6444c110aeee0981"},
		{"GET", "/api/v1/app/conversations/{conversationId}/messages", "listMessages", "cookie", false, "chat.conversation_use", "", "none", "", "200", "application/json", "inline", "sha256:36d846883dd942e6fb2ba16e74f20d4e80dcb40ba3f8ff0ae4c6a37946c32982"},
		{"POST", "/api/v1/app/conversations/{conversationId}/messages", "sendMessage", "cookie+csrf", true, "chat.conversation_use", "application/json", "ref", "#/components/schemas/SendMessageRequest", "200", "application/json", "inline", "sha256:36d846883dd942e6fb2ba16e74f20d4e80dcb40ba3f8ff0ae4c6a37946c32982"},
		{"POST", "/api/v1/app/conversations/{conversationId}/messages/stream", "streamMessage", "cookie+csrf", true, "chat.conversation_use", "application/json", "ref", "#/components/schemas/SendMessageRequest", "200", "text/event-stream", "inline", "sha256:00404e686415370f1711c4d7acfa2905444d3cf23cef2e10c47d445ebe690f96"},
		{"DELETE", "/api/v1/app/conversations/{conversationId}/messages/{messageId}", "deleteMessage", "cookie+csrf", true, "chat.conversation_use", "", "none", "", "200", "application/json", "inline", "sha256:aa47e090524f39102ac7e933f06929264e89ea4ff11d8534ed61ffc29698fb21"},
		{"PUT", "/api/v1/app/conversations/{conversationId}/messages/{messageId}", "updateMessage", "cookie+csrf", true, "chat.conversation_use", "application/json", "ref", "#/components/schemas/UpdateMessageRequest", "200", "application/json", "inline", "sha256:0cd36401823b311dc502e0cf402ce1d161752d35cd1938bebaa094972cfdb56a"},
		{"POST", "/api/v1/app/conversations/{conversationId}/messages/{messageId}/bookmark", "bookmarkMessage", "cookie+csrf", true, "chat.conversation_use", "application/json", "ref", "#/components/schemas/BookmarkMessageRequest", "200", "application/json", "inline", "sha256:0cd36401823b311dc502e0cf402ce1d161752d35cd1938bebaa094972cfdb56a"},
		{"POST", "/api/v1/app/conversations/{conversationId}/messages/{messageId}/share", "createMessageShare", "cookie+csrf", true, "chat.conversation_use", "application/json", "ref", "#/components/schemas/CreateMessageShareRequest", "201", "application/json", "inline", "sha256:3953f6360b8c9b7b6df3027b77fb6f0335d2e6bee63c1c2a7aea229830eaf8b2"},
		{"POST", "/api/v1/app/conversations/{conversationId}/share", "createConversationShare", "cookie+csrf", true, "chat.conversation_use", "application/json", "ref", "#/components/schemas/CreateConversationShareRequest", "201", "application/json", "inline", "sha256:0c245ebad5d3d5a65f727ea01c44d03636bc71727128ff29192c3c32b07c1dc4"},
		{"GET", "/api/v1/app/message-shares/{shareId}", "getMessageShare", "public", false, "chat.conversation_use", "", "none", "", "200", "application/json", "inline", "sha256:710af4459f3714d0481abfcc8ce2a10899422e0d44f1d19c8db0f8a090b69a3f"},
		{"GET", "/api/v1/app/models", "listModels", "cookie", false, "chat.conversation_use", "", "none", "", "200", "application/json", "inline", "sha256:35500b7f379dadc005422aa3113238180ba0b8883b4e73566ece8989b6382401"},
		{"GET", "/api/v1/app/personas", "listPersonas", "cookie", false, "chat.conversation_use", "", "none", "", "200", "application/json", "inline", "sha256:8d5da83f6df9203a20e399bb505b477adc2876c27ef4074b26387fc32c8bbe7d"},
		{"POST", "/api/v1/app/personas", "createPersona", "cookie+csrf", true, "chat.conversation_use", "application/json", "ref", "#/components/schemas/PersonaRequest", "200", "application/json", "inline", "sha256:a33b574e9a6891def83e40968fb715fe646528dc3b93f7efa6be297731a08452"},
		{"DELETE", "/api/v1/app/personas/{personaId}", "deletePersona", "cookie+csrf", true, "chat.conversation_use", "", "none", "", "200", "application/json", "inline", "sha256:c20ef5327772e38e60d379abc27600b8ca07be613439bc27517cf767b9317a27"},
		{"GET", "/api/v1/app/personas/{personaId}", "getPersona", "cookie", false, "chat.conversation_use", "", "none", "", "200", "application/json", "inline", "sha256:a33b574e9a6891def83e40968fb715fe646528dc3b93f7efa6be297731a08452"},
		{"PUT", "/api/v1/app/personas/{personaId}", "updatePersona", "cookie+csrf", true, "chat.conversation_use", "application/json", "ref", "#/components/schemas/PersonaRequest", "200", "application/json", "inline", "sha256:a33b574e9a6891def83e40968fb715fe646528dc3b93f7efa6be297731a08452"},
	})
}

func chatRouteHandler(chatHandler chatHandler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/app/message-shares/") {
			shareID := strings.TrimPrefix(r.URL.Path, "/api/v1/app/message-shares/")
			if shareID == "" || strings.Contains(shareID, "/") {
				writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
				return
			}
			chatHandler.getMessageShare(w, r, shareID)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/app/conversation-shares/") {
			shareID := strings.TrimPrefix(r.URL.Path, "/api/v1/app/conversation-shares/")
			if shareID == "" || strings.Contains(shareID, "/") {
				writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
				return
			}
			chatHandler.getConversationShare(w, r, shareID)
			return
		}
		if r.URL.Path == "/api/v1/app/models" {
			chatHandler.listModels(w, r)
			return
		}
		if r.URL.Path == "/api/v1/app/personas" {
			if r.Method == stdhttp.MethodGet {
				chatHandler.listPersonas(w, r)
			} else {
				chatHandler.createPersona(w, r)
			}
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v1/app/personas/") {
			personaID := strings.TrimPrefix(r.URL.Path, "/api/v1/app/personas/")
			if personaID == "" || strings.Contains(personaID, "/") {
				writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
				return
			}
			switch r.Method {
			case stdhttp.MethodGet:
				chatHandler.getPersona(w, r, personaID)
			case stdhttp.MethodPut:
				chatHandler.updatePersona(w, r, personaID)
			default:
				chatHandler.deletePersona(w, r, personaID)
			}
			return
		}
		if r.URL.Path == "/api/v1/app/conversations" {
			if r.Method == stdhttp.MethodGet {
				chatHandler.listConversations(w, r)
			} else {
				chatHandler.createConversation(w, r)
			}
			return
		}

		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/app/conversations/"), "/")
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
			default:
				chatHandler.deleteConversation(w, r, conversationID)
			}
			return
		}
		if parts[1] == "messages" {
			switch {
			case len(parts) == 2 && r.Method == stdhttp.MethodGet:
				chatHandler.listMessages(w, r, conversationID)
			case len(parts) == 2:
				chatHandler.sendMessage(w, r, conversationID)
			case len(parts) == 3 && parts[2] == "stream":
				chatHandler.streamMessage(w, r, conversationID)
			case len(parts) == 3 && r.Method == stdhttp.MethodPut:
				chatHandler.updateMessage(w, r, conversationID, parts[2])
			case len(parts) == 3:
				chatHandler.deleteMessage(w, r, conversationID, parts[2])
			case len(parts) == 4 && parts[3] == "bookmark":
				chatHandler.bookmarkMessage(w, r, conversationID, parts[2])
			case len(parts) == 4 && parts[3] == "share":
				chatHandler.createMessageShare(w, r, conversationID, parts[2])
			default:
				writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			}
			return
		}
		if len(parts) != 2 {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}
		switch parts[1] {
		case "export.md":
			chatHandler.exportConversationMarkdown(w, r, conversationID)
		case "share":
			chatHandler.createConversationShare(w, r, conversationID)
		case "config":
			if r.Method == stdhttp.MethodGet {
				chatHandler.getConversationConfig(w, r, conversationID)
			} else {
				chatHandler.updateConversationConfig(w, r, conversationID)
			}
		case "convert-to-task":
			chatHandler.convertConversationToTask(w, r, conversationID)
		case "fork":
			chatHandler.forkConversationFromSource(w, r, conversationID)
		default:
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
		}
	})
}

func registerChatRouteSurfaces(registrar *RouteSurfaceRegistrar, chatHandler chatHandler) error {
	operations := chatRouteSurfaceOperations()
	return registerRouteSurfaceBindings(registrar, routeSurfaceBindingsForHandler(operations, RouteSurfaceAuthSession, chatRouteHandler(chatHandler)))
}

func registerChatRoutes(mux *stdhttp.ServeMux, authMiddleware authMiddleware, chatHandler chatHandler) {
	if err := registerChatRouteSurfaces(mustRouteSurfaceAdapterRegistrar(mux, authMiddleware), chatHandler); err != nil {
		panic(err)
	}
}
