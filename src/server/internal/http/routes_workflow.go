package http

import (
	stdhttp "net/http"
	"strings"
)

type sessionMiddleware interface {
	requireSession(stdhttp.Handler) stdhttp.Handler
}

func workflowRouteSurfaceOperations() []OperationContractMetadataV1 {
	return routeSurfaceOperationsFromSpecs([]routeSurfaceOperationSpec{
		{"GET", "/api/v1/workflows", "listWorkflows", "cookie", false, "workflow.graph_execution", "", "none", "", "200", "application/json", "ref", "#/components/schemas/WorkflowDefinitionsEnvelope"},
		{"POST", "/api/v1/workflows", "createWorkflow", "cookie+csrf", true, "workflow.graph_execution", "application/json", "ref", "#/components/schemas/CreateWorkflowRequest", "201", "application/json", "ref", "#/components/schemas/WorkflowDefinitionEnvelope"},
		{"POST", "/api/v1/workflows/conversation-matches", "matchWorkflowConversationTriggers", "cookie+csrf", true, "workflow.graph_execution", "application/json", "ref", "#/components/schemas/WorkflowConversationMatchRequest", "200", "application/json", "ref", "#/components/schemas/WorkflowConversationMatchesEnvelope"},
		{"POST", "/api/v1/workflows/debug-retention/prune", "pruneWorkflowExecutionDebugRetention", "cookie+csrf", true, "workflow.graph_execution", "application/json", "ref", "#/components/schemas/WorkflowExecutionDebugRetentionPruneRequest", "200", "application/json", "ref", "#/components/schemas/WorkflowExecutionDebugRetentionPruneResultEnvelope"},
		{"POST", "/api/v1/workflows/semantic-matches", "matchWorkflowSemanticTriggers", "cookie+csrf", true, "workflow.graph_execution", "application/json", "ref", "#/components/schemas/WorkflowSemanticMatchRequest", "200", "application/json", "ref", "#/components/schemas/WorkflowSemanticMatchesEnvelope"},
		{"POST", "/api/v1/workflows/webhooks/{organizationId}/{workflowId}", "triggerSignedWorkflowWebhook", "public", false, "workflow.graph_execution", "application/json", "inline", "sha256:82ef96cebaf5fbe16269fd18b0240d78f5b9b90a4155a17eb797115b09148ecf", "201", "application/json", "ref", "#/components/schemas/WorkflowExecutionEnvelope"},
		{"DELETE", "/api/v1/workflows/{workflowId}", "deleteWorkflow", "cookie+csrf", true, "workflow.graph_execution", "", "none", "", "200", "application/json", "ref", "#/components/schemas/WorkflowDefinitionEnvelope"},
		{"GET", "/api/v1/workflows/{workflowId}", "getWorkflow", "cookie", false, "workflow.graph_execution", "", "none", "", "200", "application/json", "ref", "#/components/schemas/WorkflowDefinitionEnvelope"},
		{"PUT", "/api/v1/workflows/{workflowId}", "updateWorkflow", "cookie+csrf", true, "workflow.graph_execution", "application/json", "ref", "#/components/schemas/UpdateWorkflowRequest", "200", "application/json", "ref", "#/components/schemas/WorkflowDefinitionEnvelope"},
		{"POST", "/api/v1/workflows/{workflowId}/branches", "createWorkflowBranch", "cookie+csrf", true, "workflow.graph_execution", "application/json", "ref", "#/components/schemas/CreateWorkflowBranchRequest", "201", "application/json", "ref", "#/components/schemas/WorkflowDefinitionEnvelope"},
		{"POST", "/api/v1/workflows/{workflowId}/branches/{branchId}/merge", "mergeWorkflowBranch", "cookie+csrf", true, "workflow.graph_execution", "", "none", "", "200", "application/json", "ref", "#/components/schemas/WorkflowDefinitionEnvelope"},
		{"POST", "/api/v1/workflows/{workflowId}/branches/{branchId}/publish", "publishWorkflowBranch", "cookie+csrf", true, "workflow.graph_execution", "application/json", "ref", "#/components/schemas/PublishWorkflowBranchRequest", "201", "application/json", "ref", "#/components/schemas/WorkflowDefinitionEnvelope"},
		{"POST", "/api/v1/workflows/{workflowId}/execute", "executeWorkflow", "cookie+csrf", true, "workflow.graph_execution", "application/json", "ref", "#/components/schemas/ExecuteWorkflowRequest", "201", "application/json", "ref", "#/components/schemas/WorkflowExecutionEnvelope"},
		{"GET", "/api/v1/workflows/{workflowId}/executions", "listWorkflowExecutions", "cookie", false, "workflow.graph_execution", "", "none", "", "200", "application/json", "ref", "#/components/schemas/WorkflowExecutionsEnvelope"},
		{"GET", "/api/v1/workflows/{workflowId}/executions/{executionId}", "getWorkflowExecution", "cookie", false, "workflow.graph_execution", "", "none", "", "200", "application/json", "ref", "#/components/schemas/WorkflowExecutionEnvelope"},
		{"POST", "/api/v1/workflows/{workflowId}/executions/{executionId}/cancel", "cancelWorkflowExecution", "cookie+csrf", true, "workflow.graph_execution", "", "none", "", "200", "application/json", "ref", "#/components/schemas/WorkflowExecutionEnvelope"},
		{"GET", "/api/v1/workflows/{workflowId}/executions/{executionId}/debug-snapshot", "getWorkflowExecutionDebugSnapshot", "cookie", false, "workflow.graph_execution", "", "none", "", "200", "application/json", "ref", "#/components/schemas/WorkflowExecutionDebugSnapshotEnvelope"},
		{"POST", "/api/v1/workflows/{workflowId}/executions/{executionId}/decision", "decideWorkflowExecutionFailure", "cookie+csrf", true, "workflow.graph_execution", "application/json", "ref", "#/components/schemas/WorkflowFailureDecisionRequest", "200", "application/json", "ref", "#/components/schemas/WorkflowExecutionEnvelope"},
		{"POST", "/api/v1/workflows/{workflowId}/executions/{executionId}/pause", "pauseWorkflowExecution", "cookie+csrf", true, "workflow.graph_execution", "", "none", "", "200", "application/json", "ref", "#/components/schemas/WorkflowExecutionEnvelope"},
		{"POST", "/api/v1/workflows/{workflowId}/executions/{executionId}/resource-check", "checkWorkflowExecutionResources", "cookie+csrf", true, "workflow.graph_execution", "application/json", "ref", "#/components/schemas/WorkflowResourceCheckRequest", "200", "application/json", "ref", "#/components/schemas/WorkflowExecutionEnvelope"},
		{"POST", "/api/v1/workflows/{workflowId}/executions/{executionId}/resume", "resumeWorkflowExecution", "cookie+csrf", true, "workflow.graph_execution", "application/json", "ref", "#/components/schemas/WorkflowResumeExecutionRequest", "200", "application/json", "ref", "#/components/schemas/WorkflowExecutionEnvelope"},
		{"GET", "/api/v1/workflows/{workflowId}/executions/{executionId}/state-replay", "getWorkflowExecutionStateReplay", "cookie", false, "workflow.replay", "", "none", "", "200", "application/json", "ref", "#/components/schemas/WorkflowExecutionStateReplayEnvelope"},
		{"POST", "/api/v1/workflows/{workflowId}/rollback", "rollbackWorkflow", "cookie+csrf", true, "workflow.replay", "application/json", "ref", "#/components/schemas/RollbackWorkflowRequest", "200", "application/json", "ref", "#/components/schemas/WorkflowDefinitionEnvelope"},
		{"POST", "/api/v1/workflows/{workflowId}/test-node", "testWorkflowNode", "cookie+csrf", true, "workflow.graph_execution", "application/json", "ref", "#/components/schemas/WorkflowNodeTestRequest", "200", "application/json", "ref", "#/components/schemas/WorkflowNodeTestResultEnvelope"},
		{"GET", "/api/v1/workflows/{workflowId}/versions", "listWorkflowVersions", "cookie", false, "workflow.replay", "", "none", "", "200", "application/json", "ref", "#/components/schemas/WorkflowDefinitionsEnvelope"},
		{"POST", "/api/v1/workflows/{workflowId}/webhook", "triggerWorkflowWebhook", "cookie+csrf", true, "workflow.graph_execution", "application/json", "inline", "sha256:82ef96cebaf5fbe16269fd18b0240d78f5b9b90a4155a17eb797115b09148ecf", "201", "application/json", "ref", "#/components/schemas/WorkflowExecutionEnvelope"},
	})
}

func workflowRouteHandler(workflowHandler workflowHandler) stdhttp.Handler {
	return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/workflows/webhooks/") {
			if r.Method != stdhttp.MethodPost {
				writeError(w, stdhttp.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/workflows/webhooks/"), "/")
			if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
				writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
				return
			}
			workflowHandler.triggerSignedWebhook(w, r, parts[0], parts[1])
			return
		}
		switch r.URL.Path {
		case "/api/v1/workflows":
			if r.Method == stdhttp.MethodGet {
				workflowHandler.listWorkflows(w, r)
			} else {
				workflowHandler.createWorkflow(w, r)
			}
			return
		case "/api/v1/workflows/semantic-matches":
			workflowHandler.matchSemanticTriggers(w, r)
			return
		case "/api/v1/workflows/conversation-matches":
			workflowHandler.matchConversationTriggers(w, r)
			return
		case "/api/v1/workflows/debug-retention/prune":
			workflowHandler.pruneExecutionDebugData(w, r)
			return
		}

		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/workflows/"), "/")
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
			default:
				workflowHandler.deleteWorkflow(w, r, workflowID)
			}
			return
		}
		if len(parts) == 2 {
			switch parts[1] {
			case "execute":
				workflowHandler.startExecution(w, r, workflowID)
			case "webhook":
				workflowHandler.triggerWebhook(w, r, workflowID)
			case "versions":
				workflowHandler.listWorkflowVersions(w, r, workflowID)
			case "branches":
				workflowHandler.createWorkflowBranch(w, r, workflowID)
			case "rollback":
				workflowHandler.rollbackWorkflow(w, r, workflowID)
			case "test-node":
				workflowHandler.testNode(w, r, workflowID)
			case "executions":
				workflowHandler.listExecutions(w, r, workflowID)
			default:
				writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			}
			return
		}
		if len(parts) == 4 && parts[1] == "branches" && parts[2] != "" {
			if parts[3] == "publish" {
				workflowHandler.publishWorkflowBranch(w, r, workflowID, parts[2])
			} else if parts[3] == "merge" {
				workflowHandler.mergeWorkflowBranch(w, r, workflowID, parts[2])
			} else {
				writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			}
			return
		}
		if len(parts) == 3 && parts[1] == "executions" && parts[2] != "" {
			workflowHandler.getExecution(w, r, parts[2])
			return
		}
		if len(parts) == 4 && parts[1] == "executions" && parts[2] != "" {
			switch parts[3] {
			case "debug-snapshot":
				workflowHandler.getExecutionDebugSnapshot(w, r, parts[2])
			case "state-replay":
				workflowHandler.getExecutionStateReplay(w, r, parts[2])
			case "resource-check":
				workflowHandler.checkResourceLimits(w, r, workflowID, parts[2])
			case "decision":
				workflowHandler.resolvePausedFailure(w, r, parts[2])
			case "pause":
				workflowHandler.pauseExecution(w, r, parts[2])
			case "resume":
				workflowHandler.resumeExecution(w, r, parts[2])
			case "cancel":
				workflowHandler.cancelExecution(w, r, parts[2])
			default:
				writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
			}
			return
		}
		writeError(w, stdhttp.StatusNotFound, "not_found", "route not found")
	})
}

func registerWorkflowRouteSurfaces(registrar *RouteSurfaceRegistrar, workflowHandler workflowHandler) error {
	operations := workflowRouteSurfaceOperations()
	return registerRouteSurfaceBindings(registrar, routeSurfaceBindingsForHandler(operations, RouteSurfaceAuthSession, workflowRouteHandler(workflowHandler)))
}

func registerWorkflowRoutes(mux *stdhttp.ServeMux, authMiddleware sessionMiddleware, workflowHandler workflowHandler) {
	if err := registerWorkflowRouteSurfaces(mustRouteSurfaceAdapterRegistrar(mux, authMiddleware), workflowHandler); err != nil {
		panic(err)
	}
}
