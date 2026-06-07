package http

import (
	stdhttp "net/http"
	"strings"
)

func registerScheduleRoutes(mux *stdhttp.ServeMux, authMiddleware sessionMiddleware, scheduleHandler scheduleHandler) {
	mux.Handle("/api/v1/scheduled-tasks", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.Method {
		case stdhttp.MethodGet:
			scheduleHandler.listScheduledTasks(w, r)
		case stdhttp.MethodPost:
			scheduleHandler.createScheduledTask(w, r)
		default:
			writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		}
	})))

	mux.Handle("/api/v1/scheduled-tasks/", authMiddleware.requireSession(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		trimmedPath := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/scheduled-tasks/"), "/")
		parts := strings.Split(trimmedPath, "/")
		if len(parts) != 2 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		switch parts[1] {
		case "runs":
			switch r.Method {
			case stdhttp.MethodGet:
				scheduleHandler.listScheduledTaskRuns(w, r, parts[0])
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
		case "status":
			switch r.Method {
			case stdhttp.MethodPatch:
				scheduleHandler.updateScheduledTaskStatus(w, r, parts[0])
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
		case "run":
			switch r.Method {
			case stdhttp.MethodPost:
				scheduleHandler.runScheduledTaskNow(w, r, parts[0])
			default:
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			}
		default:
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
		}
	})))
}
