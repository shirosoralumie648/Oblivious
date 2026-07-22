package http

import (
	stdhttp "net/http"
	"strings"
)

func agentRunRouteSurfaceOperations() []OperationContractMetadataV1 {
	const runCapability = "agent.run"
	const toolCapability = "agent.tool_execution"
	const runResponse = "sha256:03ffd67a6dc4e838427fe47cd0ae608ca1e695bd320e1b6fff66f4e578350c57"
	return []OperationContractMetadataV1{
		routeSurfaceJSONOperation(stdhttp.MethodGet, "/api/v1/agent/tools", "listAgentTools", "cookie", toolCapability, false, "", "200", "sha256:8290fde04cb7a1aaf0ffa5b0848e8eb339c926ec56c044858dee57549c3e7b7d"),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/agent/runs", "createAgentRun", "cookie+csrf", runCapability, true, "#/components/schemas/AgentRunCreateRequest", "201", runResponse),
		routeSurfaceJSONOperation(stdhttp.MethodGet, "/api/v1/agent/runs/{runId}", "getAgentRun", "cookie", runCapability, false, "", "200", runResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/agent/runs/{runId}/approve-tool", "approveAgentToolRun", "cookie+csrf", toolCapability, true, "#/components/schemas/AgentToolDecisionRequest", "200", runResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/agent/runs/{runId}/reject-tool", "rejectAgentToolRun", "cookie+csrf", toolCapability, true, "#/components/schemas/AgentToolDecisionRequest", "200", runResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/agent/runs/{runId}/retry-tool", "retryAgentToolRun", "cookie+csrf", toolCapability, true, "#/components/schemas/AgentToolDecisionRequest", "200", runResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/agent/runs/{runId}/continue-budget", "continueAgentRunWithBudget", "cookie+csrf", runCapability, true, "#/components/schemas/AgentRunContinueBudgetRequest", "200", runResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/agent/runs/{runId}/adjust-plan", "adjustAgentRunPlan", "cookie+csrf", runCapability, true, "#/components/schemas/AgentRunAdjustPlanRequest", "200", runResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/agent/runs/{runId}/continue-plan", "continueAgentRunPlan", "cookie+csrf", runCapability, true, "", "200", runResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/agent/runs/{runId}/approve-plan-step", "approveAgentPlanStep", "cookie+csrf", runCapability, true, "#/components/schemas/AgentPlanStepDecisionRequest", "200", runResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/agent/runs/{runId}/skip-plan-step", "skipAgentPlanStep", "cookie+csrf", runCapability, true, "#/components/schemas/AgentPlanStepDecisionRequest", "200", runResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/agent/runs/{runId}/retry-plan-step", "retryAgentPlanStep", "cookie+csrf", runCapability, true, "#/components/schemas/AgentPlanStepDecisionRequest", "200", runResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPatch, "/api/v1/agent/runs/{runId}/update-plan-step", "updateAgentPlanStep", "cookie+csrf", runCapability, true, "#/components/schemas/AgentPlanStepUpdateRequest", "200", runResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/agent/runs/{runId}/create-plan-step", "createAgentPlanStep", "cookie+csrf", runCapability, true, "#/components/schemas/AgentPlanStepCreateRequest", "201", runResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/agent/runs/{runId}/move-plan-step", "moveAgentPlanStep", "cookie+csrf", runCapability, true, "#/components/schemas/AgentPlanStepMoveRequest", "200", runResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/agent/runs/{runId}/delete-plan-step", "deleteAgentPlanStep", "cookie+csrf", runCapability, true, "#/components/schemas/AgentPlanStepDecisionRequest", "200", runResponse),
		routeSurfaceJSONOperation(stdhttp.MethodPost, "/api/v1/agent/runs/{runId}/execute-plan-step", "executeAgentPlanStep", "cookie+csrf", runCapability, true, "#/components/schemas/AgentPlanStepDecisionRequest", "200", runResponse),
	}
}

func agentRunRouteHandler(handler agentRunsHandler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if r.URL.Path == "/api/v1/agent/tools" {
			handler.listTools(w, r)
			return
		}
		if r.URL.Path == "/api/v1/agent/runs" {
			handler.createRun(w, r)
			return
		}

		trimmedPath := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/agent/runs/"), "/")
		parts := strings.Split(trimmedPath, "/")
		if len(parts) == 0 || parts[0] == "" {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		runID := parts[0]
		if len(parts) == 1 {
			handler.getRun(w, r, runID)
			return
		}
		if len(parts) != 2 {
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			return
		}

		switch parts[1] {
		case "approve-tool":
			handler.approveTool(w, r, runID)
		case "reject-tool":
			handler.rejectTool(w, r, runID)
		case "retry-tool":
			handler.retryTool(w, r, runID)
		case "continue-budget":
			handler.continueBudget(w, r, runID)
		case "adjust-plan":
			handler.adjustPlan(w, r, runID)
		case "continue-plan":
			handler.continuePlan(w, r, runID)
		case "approve-plan-step":
			handler.approvePlanStep(w, r, runID)
		case "skip-plan-step":
			handler.skipPlanStep(w, r, runID)
		case "retry-plan-step":
			handler.retryPlanStep(w, r, runID)
		case "update-plan-step":
			handler.updatePlanStep(w, r, runID)
		case "create-plan-step":
			handler.createPlanStep(w, r, runID)
		case "move-plan-step":
			handler.movePlanStep(w, r, runID)
		case "delete-plan-step":
			handler.deletePlanStep(w, r, runID)
		case "execute-plan-step":
			handler.executePlanStep(w, r, runID)
		default:
			writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
		}
	})
}

func registerAgentRunRouteSurfaces(registrar *RouteSurfaceRegistrar, handler agentRunsHandler) error {
	sharedHandler := agentRunRouteHandler(handler)
	operations := agentRunRouteSurfaceOperations()
	bindings := make([]routeSurfaceBinding, 0, len(operations))
	for _, operation := range operations {
		bindings = append(bindings, routeSurfaceBinding{Operation: operation, Auth: RouteSurfaceAuthSession, Handler: sharedHandler})
	}
	return registerRouteSurfaceBindings(registrar, bindings)
}

func registerAgentRunRoutes(mux *stdhttp.ServeMux, authMiddleware sessionMiddleware, handler agentRunsHandler) {
	if err := registerAgentRunRouteSurfaces(mustRouteSurfaceAdapterRegistrar(mux, authMiddleware), handler); err != nil {
		panic(err)
	}
}
