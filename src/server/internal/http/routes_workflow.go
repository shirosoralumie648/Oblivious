package http

import (
	stdhttp "net/http"
	"strings"
)

type sessionMiddleware interface {
	requireSession(stdhttp.Handler) stdhttp.Handler
}

func registerWorkflowRoutes(mux *stdhttp.ServeMux, authMiddleware sessionMiddleware, workflowHandler workflowHandler) {
	mux.Handle("/api/v1/workflows/webhooks/", stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/v1/workflows/webhooks/")
		parts := strings.Split(trimmedPath, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		switch r.Method {
		case stdhttp.MethodPost:
			workflowHandler.triggerSignedWebhook(w, r, parts[0], parts[1])
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	}))

	mux.Handle("/api/v1/workflows", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			workflowHandler.listWorkflows(w, r)
		case stdhttp.MethodPost:
			workflowHandler.createWorkflow(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))

	mux.Handle("/api/v1/workflows/semantic-matches", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodPost:
			workflowHandler.matchSemanticTriggers(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))

	mux.Handle("/api/v1/workflows/conversation-matches", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodPost:
			workflowHandler.matchConversationTriggers(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))

	mux.Handle("/api/v1/workflows/", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/v1/workflows/")
		parts := strings.Split(trimmedPath, "/")
		if len(parts) == 0 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		workflowID := parts[0]
		if len(parts) == 1 {
			switch r.Method {
			case stdhttp.MethodGet:
				workflowHandler.getWorkflow(w, r, workflowID)
			case stdhttp.MethodPut:
				workflowHandler.updateWorkflow(w, r, workflowID)
			case stdhttp.MethodDelete:
				workflowHandler.deleteWorkflow(w, r, workflowID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "execute" {
			switch r.Method {
			case stdhttp.MethodPost:
				workflowHandler.startExecution(w, r, workflowID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "webhook" {
			switch r.Method {
			case stdhttp.MethodPost:
				workflowHandler.triggerWebhook(w, r, workflowID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "versions" {
			switch r.Method {
			case stdhttp.MethodGet:
				workflowHandler.listWorkflowVersions(w, r, workflowID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "branches" {
			switch r.Method {
			case stdhttp.MethodPost:
				workflowHandler.createWorkflowBranch(w, r, workflowID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 4 && parts[1] == "branches" && parts[2] != "" {
			switch parts[3] {
			case "publish":
				if r.Method == stdhttp.MethodPost {
					workflowHandler.publishWorkflowBranch(w, r, workflowID, parts[2])
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "merge":
				if r.Method == stdhttp.MethodPost {
					workflowHandler.mergeWorkflowBranch(w, r, workflowID, parts[2])
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			default:
				writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "rollback" {
			switch r.Method {
			case stdhttp.MethodPost:
				workflowHandler.rollbackWorkflow(w, r, workflowID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "test-node" {
			switch r.Method {
			case stdhttp.MethodPost:
				workflowHandler.testNode(w, r, workflowID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "executions" {
			switch r.Method {
			case stdhttp.MethodGet:
				workflowHandler.listExecutions(w, r, workflowID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 3 && parts[1] == "executions" && parts[2] != "" {
			switch r.Method {
			case stdhttp.MethodGet:
				workflowHandler.getExecution(w, r, parts[2])
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 4 && parts[1] == "executions" && parts[2] != "" {
			switch parts[3] {
			case "debug-snapshot":
				if r.Method == stdhttp.MethodGet {
					workflowHandler.getExecutionDebugSnapshot(w, r, parts[2])
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "resource-check":
				if r.Method == stdhttp.MethodPost {
					workflowHandler.checkResourceLimits(w, r, workflowID, parts[2])
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "decision":
				if r.Method == stdhttp.MethodPost {
					workflowHandler.resolvePausedFailure(w, r, parts[2])
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "pause":
				if r.Method == stdhttp.MethodPost {
					workflowHandler.pauseExecution(w, r, parts[2])
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "resume":
				if r.Method == stdhttp.MethodPost {
					workflowHandler.resumeExecution(w, r, parts[2])
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			case "cancel":
				if r.Method == stdhttp.MethodPost {
					workflowHandler.cancelExecution(w, r, parts[2])
				} else {
					writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				}
			default:
				writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			}
			return
		}

		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	})))
}
