package http

import (
	stdhttp "net/http"
	"strings"
)

func scheduleRouteSurfaceOperations() []OperationContractMetadataV1 {
	const capability = "task.scheduled_execution"
	const scheduleResponse = "sha256:b2b807b44032edba9771de0a83f8f8d0f66a00f9bbcd3fb11d1b6d71488ff2f5"
	return []OperationContractMetadataV1{
		routeSurfaceJSONOperation(stdhttp.MethodGet, "/api/v1/scheduled-tasks", "listScheduledTasks", "cookie", capability, false, "", "200", "sha256:4b434d001e41243518ce376a7f1011be330783c32a205ef615a794d93e6a6498"),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/scheduled-tasks", "createScheduledTask", "cookie+csrf", capability, true, "#/components/schemas/CreateScheduledTaskRequest", "201", scheduleResponse),
		routeSurfaceJSONOperation(stdhttp.MethodGet, "/api/v1/scheduled-tasks/{scheduledTaskId}/runs", "listScheduledTaskRuns", "cookie", capability, false, "", "200", "sha256:86708b0d68f43529891a5451837148dc1578906aa7fddbac50ab138f766c05a9"),
		routeSurfaceJSONOperation(stdhttp.MethodPatch, "/api/v1/scheduled-tasks/{scheduledTaskId}/status", "updateScheduledTaskStatus", "cookie+csrf", capability, true, "#/components/schemas/UpdateScheduledTaskStatusRequest", "200", scheduleResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/scheduled-tasks/{scheduledTaskId}/run", "runScheduledTaskNow", "cookie+csrf", capability, true, "", "202", "sha256:d224f2555947de3792a8ccc3441b69ba06d63214109f42d96dc1c47b8930a37e"),
	}
}

func scheduleRouteHandler(scheduleHandler scheduleHandler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.URL.Path == "/api/v1/scheduled-tasks" {
			if r.Method == stdhttp.MethodGet {
				scheduleHandler.listScheduledTasks(w, r)
			} else {
				scheduleHandler.createScheduledTask(w, r)
			}
			return
		}

		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/scheduled-tasks/"), "/"), "/")
		if len(parts) != 2 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}
		switch parts[1] {
		case "runs":
			scheduleHandler.listScheduledTaskRuns(w, r, parts[0])
		case "status":
			scheduleHandler.updateScheduledTaskStatus(w, r, parts[0])
		case "run":
			scheduleHandler.runScheduledTaskNow(w, r, parts[0])
		default:
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
		}
	})
}

func registerScheduleRouteSurfaces(registrar *RouteSurfaceRegistrar, scheduleHandler scheduleHandler) error {
	sharedHandler := scheduleRouteHandler(scheduleHandler)
	operations := scheduleRouteSurfaceOperations()
	bindings := make([]routeSurfaceBinding, 0, len(operations))
	for _, operation := range operations {
		bindings = append(bindings, routeSurfaceBinding{Operation: operation, Auth: RouteSurfaceAuthSession, Handler: sharedHandler})
	}
	return registerRouteSurfaceBindings(registrar, bindings)
}

func registerScheduleRoutes(mux *stdhttp.ServeMux, authMiddleware sessionMiddleware, scheduleHandler scheduleHandler) {
	if err := registerScheduleRouteSurfaces(mustRouteSurfaceAdapterRegistrar(mux, authMiddleware), scheduleHandler); err != nil {
		panic(err)
	}
}
