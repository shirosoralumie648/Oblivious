package http

import (
	stdhttp "net/http"
	"strings"
)

func registerKnowledgeAliasRoutes(mux *stdhttp.ServeMux, authMiddleware sessionMiddleware, knowledgeHandler knowledgeHandler) {
	mux.Handle("/api/v1/knowledge-bases", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			knowledgeHandler.listKnowledgeBases(w, r)
		case stdhttp.MethodPost:
			knowledgeHandler.createKnowledgeBase(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))

	mux.Handle("/api/v1/knowledge-bases/", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		trimmedPath := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/knowledge-bases/"), "/")
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

		if len(parts) == 3 && parts[1] == "documents" && parts[2] == "upload" {
			if r.Method == stdhttp.MethodPost {
				knowledgeHandler.uploadKnowledgeDocument(w, r, knowledgeBaseID)
			} else {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "retrieve" {
			if r.Method == stdhttp.MethodPost {
				knowledgeHandler.retrieveKnowledge(w, r, knowledgeBaseID)
			} else {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "retrieval-test-cases" {
			switch r.Method {
			case stdhttp.MethodGet:
				knowledgeHandler.listRetrievalTestCases(w, r, knowledgeBaseID)
			case stdhttp.MethodPost:
				knowledgeHandler.createRetrievalTestCase(w, r, knowledgeBaseID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 3 && parts[1] == "retrieval-test-cases" && parts[2] == "run" {
			if r.Method == stdhttp.MethodPost {
				knowledgeHandler.runRetrievalTestCases(w, r, knowledgeBaseID)
			} else {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 4 && parts[1] == "documents" && parts[2] != "" && parts[3] == "versions" {
			if r.Method == stdhttp.MethodGet {
				knowledgeHandler.listKnowledgeDocumentVersions(w, r, knowledgeBaseID, parts[2])
			} else {
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

		if len(parts) == 4 && parts[1] == "documents" && parts[2] != "" && parts[3] == "chunks" {
			if r.Method == stdhttp.MethodGet {
				knowledgeHandler.listKnowledgeDocumentChunks(w, r, knowledgeBaseID, parts[2])
			} else {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 5 && parts[1] == "documents" && parts[2] != "" && parts[3] == "chunks" && parts[4] != "" {
			if r.Method == stdhttp.MethodPut {
				knowledgeHandler.updateKnowledgeDocumentChunk(w, r, knowledgeBaseID, parts[2], parts[4])
			} else {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 6 && parts[1] == "documents" && parts[2] != "" && parts[3] == "chunks" && parts[4] != "" {
			if r.Method != stdhttp.MethodPost {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			switch parts[5] {
			case "split":
				knowledgeHandler.splitKnowledgeDocumentChunk(w, r, knowledgeBaseID, parts[2], parts[4])
			case "merge":
				knowledgeHandler.mergeKnowledgeDocumentChunks(w, r, knowledgeBaseID, parts[2], parts[4])
			default:
				writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			}
			return
		}

		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	})))

	mux.Handle("/api/v1/documents/", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		documentID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/documents/"), "/")
		if documentID == "" || strings.Contains(documentID, "/") {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}
		if r.Method != stdhttp.MethodDelete {
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		knowledgeHandler.deleteKnowledgeDocumentByID(w, r, documentID)
	})))
}
