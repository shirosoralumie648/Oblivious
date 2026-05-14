package http

import (
	stdhttp "net/http"
	"strings"
)

func registerKnowledgeRoutes(mux *stdhttp.ServeMux, authMiddleware authMiddleware, knowledgeHandler knowledgeHandler) {
	mux.Handle("/api/v1/app/knowledge-bases", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			knowledgeHandler.listKnowledgeBases(w, r)
		case stdhttp.MethodPost:
			knowledgeHandler.createKnowledgeBase(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
	mux.Handle("/api/v1/app/knowledge-bases/", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/v1/app/knowledge-bases/")
		parts := strings.Split(trimmedPath, "/")
		if len(parts) == 0 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		knowledgeBaseID := parts[0]
		if len(parts) == 1 {
			switch r.Method {
			case stdhttp.MethodGet:
				knowledgeHandler.getKnowledgeBase(w, r, knowledgeBaseID)
			case stdhttp.MethodPut:
				knowledgeHandler.updateKnowledgeBase(w, r, knowledgeBaseID)
			case stdhttp.MethodDelete:
				knowledgeHandler.deleteKnowledgeBase(w, r, knowledgeBaseID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "documents" {
			switch r.Method {
			case stdhttp.MethodGet:
				knowledgeHandler.listKnowledgeDocuments(w, r, knowledgeBaseID)
			case stdhttp.MethodPost:
				knowledgeHandler.createKnowledgeDocument(w, r, knowledgeBaseID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "retrieve" {
			switch r.Method {
			case stdhttp.MethodPost:
				knowledgeHandler.retrieveKnowledge(w, r, knowledgeBaseID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 3 && parts[1] == "documents" && parts[2] != "" {
			documentID := parts[2]
			switch r.Method {
			case stdhttp.MethodPut:
				knowledgeHandler.updateKnowledgeDocument(w, r, knowledgeBaseID, documentID)
			case stdhttp.MethodDelete:
				knowledgeHandler.deleteKnowledgeDocument(w, r, knowledgeBaseID, documentID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	})))
}
