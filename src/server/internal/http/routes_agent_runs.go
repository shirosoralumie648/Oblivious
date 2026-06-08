package http

import (
	stdhttp "net/http"
	"strings"
)

func registerAgentRunRoutes(mux *stdhttp.ServeMux, authMiddleware sessionMiddleware, handler agentRunsHandler) {
	mux.Handle("/api/v1/agent/tools", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			handler.listTools(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))

	mux.Handle("/api/v1/agent/runs", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodPost:
			handler.createRun(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))

	mux.Handle("/api/v1/agent/runs/", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		trimmedPath := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/agent/runs/"), "/")
		parts := strings.Split(trimmedPath, "/")
		if len(parts) == 0 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		runID := parts[0]
		if len(parts) == 2 {
			switch parts[1] {
			case "approve-tool":
				if r.Method == stdhttp.MethodPost {
					handler.approveTool(w, r, runID)
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "reject-tool":
				if r.Method == stdhttp.MethodPost {
					handler.rejectTool(w, r, runID)
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "retry-tool":
				if r.Method == stdhttp.MethodPost {
					handler.retryTool(w, r, runID)
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "approve-plan-step":
				if r.Method == stdhttp.MethodPost {
					handler.approvePlanStep(w, r, runID)
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "update-plan-step":
				if r.Method == stdhttp.MethodPatch {
					handler.updatePlanStep(w, r, runID)
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "move-plan-step":
				if r.Method == stdhttp.MethodPost {
					handler.movePlanStep(w, r, runID)
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "execute-plan-step":
				if r.Method == stdhttp.MethodPost {
					handler.executePlanStep(w, r, runID)
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			default:
				writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			}
			return
		}

		if len(parts) != 1 {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		switch r.Method {
		case stdhttp.MethodGet:
			handler.getRun(w, r, runID)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
}
