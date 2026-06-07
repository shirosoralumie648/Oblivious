package http

import (
	stdhttp "net/http"
	"strings"
)

func registerAgentMemoryRoutes(mux *stdhttp.ServeMux, authMiddleware sessionMiddleware, handler agentMemoriesHandler) {
	mux.Handle("/api/v1/agent/memories", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			handler.searchMemories(w, r)
		case stdhttp.MethodPost:
			handler.createMemory(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
	mux.Handle("/api/v1/agent/memories/", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		memoryID := strings.TrimPrefix(r.URL.Path, "/api/v1/agent/memories/")
		if memoryID == "export" {
			if r.Method != stdhttp.MethodGet {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			handler.exportMemories(w, r)
			return
		}
		if memoryID == "import" {
			if r.Method != stdhttp.MethodPost {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			handler.importMemories(w, r)
			return
		}
		if memoryID == "" || strings.Contains(memoryID, "/") {
			writeError(w, stdhttp.StatusNotFound, "not_found", "agent memory not found")
			return
		}
		switch r.Method {
		case stdhttp.MethodPatch:
			handler.updateMemory(w, r, memoryID)
		case stdhttp.MethodDelete:
			handler.deleteMemory(w, r, memoryID)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
}
