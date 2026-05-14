package http

import (
	stdhttp "net/http"
	"strings"
)

func registerTaskRoutes(mux *stdhttp.ServeMux, authMiddleware authMiddleware, taskHandler taskHandler) {
	mux.Handle("/api/v1/app/tasks", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			taskHandler.listTasks(w, r)
		case stdhttp.MethodPost:
			taskHandler.createTask(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))
	mux.Handle("/api/v1/app/tasks/", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/v1/app/tasks/")
		parts := strings.Split(trimmedPath, "/")
		if len(parts) == 0 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		taskID := parts[0]
		if len(parts) == 1 {
			switch r.Method {
			case stdhttp.MethodGet:
				taskHandler.getTask(w, r, taskID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "start" {
			switch r.Method {
			case stdhttp.MethodPost:
				taskHandler.startTask(w, r, taskID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "approve" {
			switch r.Method {
			case stdhttp.MethodPost:
				taskHandler.approveTask(w, r, taskID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "pause" {
			switch r.Method {
			case stdhttp.MethodPost:
				taskHandler.pauseTask(w, r, taskID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "resume" {
			switch r.Method {
			case stdhttp.MethodPost:
				taskHandler.resumeTask(w, r, taskID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "cancel" {
			switch r.Method {
			case stdhttp.MethodPost:
				taskHandler.cancelTask(w, r, taskID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		if len(parts) == 2 && parts[1] == "budget" {
			switch r.Method {
			case stdhttp.MethodPost:
				taskHandler.updateTaskBudget(w, r, taskID)
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
			return
		}

		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	})))
}
