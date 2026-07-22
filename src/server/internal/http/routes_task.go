package http

import (
	stdhttp "net/http"
	"strings"
)

func taskRouteSurfaceOperations() []OperationContractMetadataV1 {
	const capability = "task.scheduled_execution"
	const taskResponse = "sha256:dbe3c956df7f528951ec7bf8f0260bcab344f86da7d91ba5af5127c2e6568232"
	return []OperationContractMetadataV1{
		routeSurfaceJSONOperation(stdhttp.MethodGet, "/api/v1/app/tasks", "listTasks", "cookie", capability, false, "", "200", "sha256:e47f376506a26a5a19eef82594c1c21a2d09c9c4dcd5d8f02769282d877d46fe"),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/app/tasks", "createTask", "cookie+csrf", capability, true, "#/components/schemas/CreateTaskRequest", "200", "sha256:1f67d825fc555fcd6cb45213e31c4108722a75def96e79b5986107cb38cd4acf"),
		routeSurfaceJSONOperation(stdhttp.MethodGet, "/api/v1/app/tasks/{taskId}", "getTask", "cookie", capability, false, "", "200", taskResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/app/tasks/{taskId}/start", "startTask", "cookie+csrf", capability, true, "", "200", taskResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/app/tasks/{taskId}/approve", "approveTask", "cookie+csrf", capability, true, "", "200", taskResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/app/tasks/{taskId}/pause", "pauseTask", "cookie+csrf", capability, true, "", "200", taskResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/app/tasks/{taskId}/resume", "resumeTask", "cookie+csrf", capability, true, "", "200", taskResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/app/tasks/{taskId}/cancel", "cancelTask", "cookie+csrf", capability, true, "", "200", taskResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/app/tasks/{taskId}/budget", "updateTaskBudget", "cookie+csrf", capability, true, "#/components/schemas/UpdateTaskBudgetRequest", "200", taskResponse),
	}
}

func taskRouteHandler(taskHandler taskHandler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.URL.Path == "/api/v1/app/tasks" {
			if r.Method == stdhttp.MethodGet {
				taskHandler.listTasks(w, r)
			} else {
				taskHandler.createTask(w, r)
			}
			return
		}

		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/app/tasks/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}
		taskID := parts[0]
		if len(parts) == 1 {
			taskHandler.getTask(w, r, taskID)
			return
		}
		if len(parts) != 2 {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}
		switch parts[1] {
		case "start":
			taskHandler.startTask(w, r, taskID)
		case "approve":
			taskHandler.approveTask(w, r, taskID)
		case "pause":
			taskHandler.pauseTask(w, r, taskID)
		case "resume":
			taskHandler.resumeTask(w, r, taskID)
		case "cancel":
			taskHandler.cancelTask(w, r, taskID)
		case "budget":
			taskHandler.updateTaskBudget(w, r, taskID)
		default:
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
		}
	})
}

func registerTaskRouteSurfaces(registrar *RouteSurfaceRegistrar, taskHandler taskHandler) error {
	sharedHandler := taskRouteHandler(taskHandler)
	operations := taskRouteSurfaceOperations()
	bindings := make([]routeSurfaceBinding, 0, len(operations))
	for _, operation := range operations {
		bindings = append(bindings, routeSurfaceBinding{Operation: operation, Auth: RouteSurfaceAuthSession, Handler: sharedHandler})
	}
	return registerRouteSurfaceBindings(registrar, bindings)
}

func registerTaskRoutes(mux *stdhttp.ServeMux, authMiddleware authMiddleware, taskHandler taskHandler) {
	if err := registerTaskRouteSurfaces(mustRouteSurfaceAdapterRegistrar(mux, authMiddleware), taskHandler); err != nil {
		panic(err)
	}
}
