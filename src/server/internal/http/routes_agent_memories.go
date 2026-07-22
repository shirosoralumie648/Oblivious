package http

import (
	stdhttp "net/http"
	"strings"
)

func agentMemoryRouteSurfaceOperations() []OperationContractMetadataV1 {
	const capability = "agent.run"
	const memoryResponse = "sha256:af53dfcdb4e6e9481d3009c90adab7fabee272e2616e7e6dc522ff6c88654363"
	const memoryListResponse = "sha256:feb07d2db56d996376e3a733e1474de9fc1ca236d0b59b0e9da8e1c8ec849ecf"
	return []OperationContractMetadataV1{
		routeSurfaceJSONOperation(stdhttp.MethodGet, "/api/v1/agent/memories", "listAgentMemories", "cookie", capability, false, "", "200", memoryListResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/agent/memories", "createAgentMemory", "cookie+csrf", capability, true, "#/components/schemas/AgentMemoryRequest", "201", memoryResponse),
		routeSurfaceJSONOperation(stdhttp.MethodGet, "/api/v1/agent/memories/export", "exportAgentMemories", "cookie", capability, false, "", "200", "sha256:232f354f4dd9d0a14cdd95514a8328bc9e107e1fbb7a5d85fa2a321728649247"),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/agent/memories/import", "importAgentMemories", "cookie+csrf", capability, true, "#/components/schemas/AgentMemoryImportRequest", "201", memoryListResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPatch, "/api/v1/agent/memories/{memoryId}", "updateAgentMemory", "cookie+csrf", capability, true, "#/components/schemas/AgentMemoryUpdateRequest", "200", memoryResponse),
		routeSurfaceNoContentOperation(stdhttp.MethodDelete, "/api/v1/agent/memories/{memoryId}", "deleteAgentMemory", "cookie+csrf", capability, true),
	}
}

func agentMemoryRouteHandler(handler agentMemoriesHandler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		switch r.URL.Path {
		case "/api/v1/agent/memories":
			switch r.Method {
			case stdhttp.MethodGet:
				handler.searchMemories(w, r)
			case stdhttp.MethodPost:
				handler.createMemory(w, r)
			}
		case "/api/v1/agent/memories/export":
			handler.exportMemories(w, r)
		case "/api/v1/agent/memories/import":
			handler.importMemories(w, r)
		default:
			memoryID := strings.TrimPrefix(r.URL.Path, "/api/v1/agent/memories/")
			if memoryID == "" || strings.Contains(memoryID, "/") {
				writeError(w, stdhttp.StatusNotFound, "not_found", "agent memory not found")
				return
			}
			if r.Method == stdhttp.MethodPatch {
				handler.updateMemory(w, r, memoryID)
				return
			}
			handler.deleteMemory(w, r, memoryID)
		}
	})
}

func registerAgentMemoryRouteSurfaces(registrar *RouteSurfaceRegistrar, handler agentMemoriesHandler) error {
	sharedHandler := agentMemoryRouteHandler(handler)
	operations := agentMemoryRouteSurfaceOperations()
	bindings := make([]routeSurfaceBinding, 0, len(operations))
	for _, operation := range operations {
		bindings = append(bindings, routeSurfaceBinding{Operation: operation, Auth: RouteSurfaceAuthSession, Handler: sharedHandler})
	}
	return registerRouteSurfaceBindings(registrar, bindings)
}

func registerAgentMemoryRoutes(mux *stdhttp.ServeMux, authMiddleware sessionMiddleware, handler agentMemoriesHandler) {
	if err := registerAgentMemoryRouteSurfaces(mustRouteSurfaceAdapterRegistrar(mux, authMiddleware), handler); err != nil {
		panic(err)
	}
}
