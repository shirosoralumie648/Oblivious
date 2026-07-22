package http

import (
	stdhttp "net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestRouteSurfaceGroupBContract(t *testing.T) {
	owners := []string{
		"routes_chat.go",
		"routes_channel.go",
		"routes_console.go",
		"routes_knowledge.go",
		"routes_knowledge_alias.go",
		"routes_workflow.go",
		"routes_observability_alert.go",
		"routes_release_evidence.go",
	}
	registrarCalls := map[string]int{
		"registerChatRouteSurfaces":               2,
		"registerPublishingChannelRouteSurfaces":  1,
		"registerConsoleRouteSurfaces":            1,
		"registerKnowledgeRouteSurfaces":          1,
		"registerKnowledgeAliasRouteSurfaces":     1,
		"registerWorkflowRouteSurfaces":           1,
		"registerObservabilityAlertRouteSurfaces": 1,
		"registerReleaseEvidenceRouteSurfaces":    1,
	}
	legacyCalls := []string{
		"registerChatRoutes",
		"registerPublishingChannelRoutes",
		"registerConsoleRoutes",
		"registerKnowledgeRoutes",
		"registerKnowledgeAliasRoutes",
		"registerWorkflowRoutes",
		"registerObservabilityAlertRoutes",
		"registerReleaseEvidenceRoutes",
	}

	if len(owners) != 8 {
		t.Fatalf("Group B owner discovery count = %d, want 8", len(owners))
	}
	for _, owner := range owners {
		source := string(routeSurfaceReadSourceFile(t, owner))
		if count := routeSurfaceDirectHandleCount(t, source); count != 0 {
			t.Fatalf("%s retains %d direct ServeMux mounts", owner, count)
		}
		if !strings.Contains(source, "RouteSurfaceRegistration") && !strings.Contains(source, "routeSurfaceBinding") {
			t.Fatalf("%s has no typed route surface registration", owner)
		}
	}

	productionCalls := routeSurfaceFunctionCalls(t, filepath.Join(routeSurfaceHTTPSourceDir(t), "router.go"))
	for name, want := range registrarCalls {
		if productionCalls[name] != want {
			t.Fatalf("production router call count for %s = %d, want %d", name, productionCalls[name], want)
		}
	}
	for _, name := range legacyCalls {
		if productionCalls[name] != 0 {
			t.Fatalf("production router retains legacy adapter %s", name)
		}
	}

	operationSets := [][]OperationContractMetadataV1{
		chatRouteSurfaceOperations(),
		publishingChannelRouteSurfaceOperations(),
		consoleRouteSurfaceOperations(),
		knowledgeRouteSurfaceOperations(),
		knowledgeAliasRouteSurfaceOperations(),
		workflowRouteSurfaceOperations(),
		observabilityAlertRouteSurfaceOperations(),
		releaseEvidenceRouteSurfaceOperations(),
	}
	operationCount := 0
	for ownerIndex, operations := range operationSets {
		if len(operations) == 0 {
			t.Fatalf("Group B owner %s has zero operations", owners[ownerIndex])
		}
		for _, operation := range operations {
			if strings.TrimSpace(operation.OperationID) == "" || strings.TrimSpace(operation.CapabilityID) == "" {
				t.Fatalf("Group B owner %s has incomplete operation metadata: %+v", owners[ownerIndex], operation)
			}
		}
		operationCount += len(operations)
	}
	if operationCount != 136 {
		t.Fatalf("Group B operation discovery count = %d, want 136", operationCount)
	}
}

func TestRouteSurfaceGroupBWorkflowFallbackRegistration(t *testing.T) {
	mux := stdhttp.NewServeMux()
	operations := workflowRouteSurfaceOperations()
	registrar, err := NewRouteSurfaceRegistrar(mux, routeSurfaceGroupATestPolicies(routeSurfaceDescriptorsForOperations(operations)))
	if err != nil {
		t.Fatalf("construct workflow registrar: %v", err)
	}
	for _, operation := range operations {
		auth := RouteSurfaceAuthSession
		if operation.Security == "public" {
			auth = RouteSurfaceAuthPublic
		}
		if err := registrar.Register(routeSurfaceRegistrationFromOperation(operation, auth, nil, "", stdhttp.HandlerFunc(func(stdhttp.ResponseWriter, *stdhttp.Request) {}))); err != nil {
			t.Fatalf("register exact workflow operation %s: %v", operation.OperationID, err)
		}
	}
	if err := registrar.registerRoutingFallbacks(); err != nil {
		patterns := make([]string, 0, len(registrar.fallbacks))
		for pattern := range registrar.fallbacks {
			patterns = append(patterns, pattern)
		}
		sort.Strings(patterns)
		t.Fatalf("register workflow fallbacks: %v; installed=%v", err, patterns)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/workflows/webhooks/executions/execution_1", nil)
	workflowRouteHandler(workflowHandler{}).ServeHTTP(recorder, request)
	if recorder.Code != stdhttp.StatusMethodNotAllowed {
		t.Fatalf("reserved signed-webhook path GET = %d, want 405", recorder.Code)
	}
}

func TestRouteSurfaceGroupBChannelConflictBridge(t *testing.T) {
	operations := publishingChannelRouteSurfaceOperations()
	mux := stdhttp.NewServeMux()
	registrar, err := NewRouteSurfaceRegistrar(mux, routeSurfaceGroupATestPolicies(routeSurfaceDescriptorsForOperations(operations)))
	if err != nil {
		t.Fatalf("construct channel registrar: %v", err)
	}
	dispatches := 0
	shared := stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
		dispatches++
		w.WriteHeader(stdhttp.StatusNoContent)
	})
	if err := registerRouteSurfaceBindings(registrar, routeSurfaceBindingsForHandler(operations, RouteSurfaceAuthSession, shared)); err != nil {
		t.Fatalf("register channel surfaces: %v", err)
	}
	if len(registrar.Snapshot()) != len(operations) {
		t.Fatalf("channel snapshot count = %d, want %d", len(registrar.Snapshot()), len(operations))
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodPost, "/api/v1/channels/channel_1/retry-failed-messages", strings.NewReader(`{}`))
	mux.ServeHTTP(recorder, request)
	if recorder.Code != stdhttp.StatusNoContent || dispatches != 1 {
		t.Fatalf("channel conflict bridge dispatch = status %d count %d, want 204/1", recorder.Code, dispatches)
	}
}

func TestRouteSurfaceConflictBridgePreservesLogicalHandlers(t *testing.T) {
	operations := []OperationContractMetadataV1{
		routeSurfaceJSONOperation(stdhttp.MethodGet, "/api/v1/bridge/fixed/{itemId}", "getFixedBridgeItem", "cookie", "channel.delivery", false, "", "200", "#/components/schemas/BridgeItem"),
		routeSurfaceJSONOperation(stdhttp.MethodGet, "/api/v1/bridge/{itemId}/action", "getBridgeItemAction", "cookie", "channel.delivery", false, "", "200", "#/components/schemas/BridgeItem"),
		routeSurfaceJSONOperation(stdhttp.MethodGet, "/api/v1/bridge/{itemId}/other", "getBridgeItemOther", "cookie", "channel.delivery", false, "", "200", "#/components/schemas/BridgeItem"),
	}
	mux := stdhttp.NewServeMux()
	registrar, err := NewRouteSurfaceRegistrar(mux, routeSurfaceGroupATestPolicies(routeSurfaceDescriptorsForOperations(operations)))
	if err != nil {
		t.Fatalf("construct conflict registrar: %v", err)
	}
	dispatches := make([]int, len(operations))
	for index, operation := range operations {
		index := index
		handler := stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
			dispatches[index]++
			w.WriteHeader(stdhttp.StatusNoContent)
		})
		if err := registrar.Register(routeSurfaceRegistrationFromOperation(operation, RouteSurfaceAuthSession, nil, "", handler)); err != nil {
			t.Fatalf("register conflict operation %s: %v", operation.OperationID, err)
		}
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(stdhttp.MethodGet, "/api/v1/bridge/item_1/other", nil)
	mux.ServeHTTP(recorder, request)
	if recorder.Code != stdhttp.StatusNoContent || dispatches[2] != 1 || dispatches[0] != 0 || dispatches[1] != 0 {
		t.Fatalf("logical bridge dispatch = status %d counts %v, want 204/[0 0 1]", recorder.Code, dispatches)
	}
}

func routeSurfaceDescriptorsForOperations(operations []OperationContractMetadataV1) []RouteSurfaceDescriptor {
	descriptors := make([]RouteSurfaceDescriptor, 0, len(operations))
	for _, operation := range operations {
		auth := RouteSurfaceAuthSession
		if operation.Security == "public" {
			auth = RouteSurfaceAuthPublic
		}
		descriptors = append(descriptors, RouteSurfaceDescriptor{CapabilityID: operation.CapabilityID, Auth: auth})
	}
	return descriptors
}
