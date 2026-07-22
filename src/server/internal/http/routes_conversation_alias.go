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

func conversationAliasRouteSurfaceOperations() []OperationContractMetadataV1 {
	const capability = "chat.conversation_use"
	const conversationResponse = "sha256:f1f6b9205d914a18251c6c466cb6fa898b2094cc6bb12b8d6444c110aeee0981"
	const messagesResponse = "sha256:36d846883dd942e6fb2ba16e74f20d4e80dcb40ba3f8ff0ae4c6a37946c32982"
	const messageResponse = "sha256:0cd36401823b311dc502e0cf402ce1d161752d35cd1938bebaa094972cfdb56a"
	return []OperationContractMetadataV1{
		routeSurfaceJSONOperation(stdhttp.MethodGet, "/api/v1/conversations", "listConversationsAlias", "cookie", capability, false, "", "200", "sha256:72453e54e4de63bab0fe91273be56130336c9cb73ef94633c9b869cb2b0dd09f"),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/conversations", "createConversationAlias", "cookie+csrf", capability, true, "#/components/schemas/CreateConversationRequest", "200", conversationResponse),
		routeSurfaceJSONOperation(stdhttp.MethodGet, "/api/v1/conversations/{conversationId}", "getConversationAlias", "cookie", capability, false, "", "200", conversationResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPut, "/api/v1/conversations/{conversationId}", "updateConversationAlias", "cookie+csrf", capability, true, "#/components/schemas/UpdateConversationRequest", "200", conversationResponse),
		routeSurfaceJSONOperation(stdhttp.MethodDelete, "/api/v1/conversations/{conversationId}", "deleteConversationAlias", "cookie+csrf", capability, true, "", "200", "sha256:b384c15ff9aa5ec7791da9dd63b400e82381096af40f0d166bb649762d269876"),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/conversations/{conversationId}/fork", "forkConversationAlias", "cookie+csrf", capability, true, "#/components/schemas/ForkConversationRequest", "200", conversationResponse),
		routeSurfaceJSONOperation(stdhttp.MethodGet, "/api/v1/conversations/{conversationId}/messages", "listMessagesAlias", "cookie", capability, false, "", "200", messagesResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/conversations/{conversationId}/messages", "sendMessageAlias", "cookie+csrf", capability, true, "#/components/schemas/SendMessageRequest", "200", messagesResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPut, "/api/v1/conversations/{conversationId}/messages/{messageId}", "updateMessageAlias", "cookie+csrf", capability, true, "#/components/schemas/UpdateMessageRequest", "200", messageResponse),
		routeSurfaceJSONOperation(stdhttp.MethodDelete, "/api/v1/conversations/{conversationId}/messages/{messageId}", "deleteMessageAlias", "cookie+csrf", capability, true, "", "200", "sha256:aa47e090524f39102ac7e933f06929264e89ea4ff11d8534ed61ffc29698fb21"),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/conversations/{conversationId}/messages/{messageId}/bookmark", "bookmarkMessageAlias", "cookie+csrf", capability, true, "#/components/schemas/BookmarkMessageRequest", "200", messageResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/conversations/{conversationId}/messages/{messageId}/share", "createMessageShareAlias", "cookie+csrf", capability, true, "#/components/schemas/CreateMessageShareRequest", "201", "sha256:3953f6360b8c9b7b6df3027b77fb6f0335d2e6bee63c1c2a7aea229830eaf8b2"),
	}
}

func conversationAliasRouteHandler(handler conversationAliasHandler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.URL.Path == "/api/v1/conversations" {
			if r.Method == stdhttp.MethodGet {
				handler.listConversations(w, r)
			} else {
				handler.createConversation(w, r)
			}
			return
		}

		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/conversations/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}
		conversationID := parts[0]
		switch {
		case len(parts) == 1 && r.Method == stdhttp.MethodGet:
			handler.getConversation(w, r, conversationID)
		case len(parts) == 1 && r.Method == stdhttp.MethodPut:
			handler.updateConversation(w, r, conversationID)
		case len(parts) == 1 && r.Method == stdhttp.MethodDelete:
			handler.deleteConversation(w, r, conversationID)
		case len(parts) == 2 && parts[1] == "fork":
			handler.forkConversationFromSource(w, r, conversationID)
		case len(parts) == 2 && parts[1] == "messages" && r.Method == stdhttp.MethodGet:
			handler.listMessages(w, r, conversationID)
		case len(parts) == 2 && parts[1] == "messages" && r.Method == stdhttp.MethodPost:
			handler.sendMessage(w, r, conversationID)
		case len(parts) == 3 && parts[1] == "messages" && r.Method == stdhttp.MethodPut:
			handler.updateMessage(w, r, conversationID, parts[2])
		case len(parts) == 3 && parts[1] == "messages" && r.Method == stdhttp.MethodDelete:
			handler.deleteMessage(w, r, conversationID, parts[2])
		case len(parts) == 4 && parts[1] == "messages" && parts[3] == "bookmark":
			handler.bookmarkMessage(w, r, conversationID, parts[2])
		case len(parts) == 4 && parts[1] == "messages" && parts[3] == "share":
			handler.createMessageShare(w, r, conversationID, parts[2])
		default:
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
		}
	})
}

func registerConversationAliasRouteSurfaces(registrar *RouteSurfaceRegistrar, handler conversationAliasHandler) error {
	sharedHandler := conversationAliasRouteHandler(handler)
	operations := conversationAliasRouteSurfaceOperations()
	bindings := make([]routeSurfaceBinding, 0, len(operations))
	for _, operation := range operations {
		bindings = append(bindings, routeSurfaceBinding{Operation: operation, Auth: RouteSurfaceAuthSession, Handler: sharedHandler})
	}
	return registerRouteSurfaceBindings(registrar, bindings)
}

func registerConversationAliasRoutes(mux *stdhttp.ServeMux, authMiddleware sessionMiddleware, handler conversationAliasHandler) {
	if err := registerConversationAliasRouteSurfaces(mustRouteSurfaceAdapterRegistrar(mux, authMiddleware), handler); err != nil {
		panic(err)
	}
}
