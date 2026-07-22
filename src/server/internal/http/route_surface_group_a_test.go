package http

import (
	"go/ast"
	"go/parser"
	"go/token"
	stdhttp "net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestRouteSurfaceGroupAContract(t *testing.T) {
	owners := []string{
		"routes_auth.go",
		"routes_agent_memories.go",
		"routes_agent_runs.go",
		"routes_conversation_alias.go",
		"routes_gateway.go",
		"routes_preferences.go",
		"routes_task.go",
		"routes_schedule.go",
	}
	registrarCalls := map[string]int{
		"registerAuthRouteSurfaces":              1,
		"registerAgentMemoryRouteSurfaces":       1,
		"registerAgentRunRouteSurfaces":          1,
		"registerConversationAliasRouteSurfaces": 2,
		"registerGatewayRouteSurfaces":           1,
		"registerPreferenceRouteSurfaces":        1,
		"registerTaskRouteSurfaces":              1,
		"registerScheduleRouteSurfaces":          1,
	}
	legacyCalls := []string{
		"registerAuthRoutes",
		"registerAgentMemoryRoutes",
		"registerAgentRunRoutes",
		"registerConversationAliasRoutes",
		"registerGatewayRoutes",
		"registerPreferenceRoutes",
		"registerTaskRoutes",
		"registerScheduleRoutes",
	}

	if len(owners) != 8 {
		t.Fatalf("Group A owner discovery count = %d, want 8", len(owners))
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

	manifest := loadRouteSurfaceManifest(t)
	expected := make([]OperationContractMetadataV1, 0, 58)
	for _, operation := range manifest.Operations {
		if routeSurfaceGroupAOwnsOperation(operation.NormalizedPath) {
			expected = append(expected, operation)
		}
	}
	if len(expected) != 58 {
		t.Fatalf("Group A manifest discovery count = %d, want 58", len(expected))
	}

	var registrar *RouteSurfaceRegistrar
	router := NewRouterWithOptions(testConfig(), nil, RouterOptions{
		AuthStore: stubAuthStore{session: routeSurfaceUserSession()},
		RouteSurfaceRegistrarFactory: func(mux *stdhttp.ServeMux, policies RouteSurfacePolicies) (*RouteSurfaceRegistrar, error) {
			created, err := NewRouteSurfaceRegistrar(mux, policies)
			registrar = created
			return created, err
		},
	})
	if registrar == nil {
		t.Fatal("production router did not construct the shared registrar")
	}
	if err := registrar.validateSnapshot(); err != nil {
		t.Fatalf("validate Group A registrar snapshot: %v", err)
	}
	actual := make([]RouteSurfaceDescriptor, 0, len(expected))
	for _, descriptor := range registrar.Snapshot() {
		if routeSurfaceGroupAOwnsOperation(descriptor.Path) {
			actual = append(actual, descriptor)
		}
	}
	if len(actual) != 58 || len(registrar.mountedPatterns()) < len(actual) {
		t.Fatalf("Group A registration closure failed: registrations=%d mounts=%d", len(actual), len(registrar.mountedPatterns()))
	}
	observation, err := CompareRouteSurfaceSnapshot(manifest.Scope, expected, actual)
	if err != nil {
		t.Fatalf("compare Group A runtime snapshot: %v", err)
	}
	if observation.ParityResult != "pass" || observation.OperationCount != 58 || observation.MountedCount != 58 || observation.DescriptorCount != 58 || observation.CoreDigest != observation.RuntimeDigest {
		t.Fatalf("unexpected Group A observation: %+v", observation)
	}

	dispatches := []struct {
		method string
		path   string
		public bool
	}{
		{stdhttp.MethodPost, "/api/v1/auth/login", true},
		{stdhttp.MethodGet, "/api/v1/agent/memories", false},
		{stdhttp.MethodPost, "/api/v1/gateway/proxy/chat/completions", false},
		{stdhttp.MethodGet, "/api/v1/app/tasks/task_1", false},
		{stdhttp.MethodGet, "/api/v1/scheduled-tasks/schedule_1/runs", false},
	}
	for _, dispatch := range dispatches {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(dispatch.method, dispatch.path, strings.NewReader(`{}`))
		request.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(recorder, request)
		if dispatch.public {
			if recorder.Code == stdhttp.StatusUnauthorized || routeSurfaceLooksUnregistered(recorder.Code, recorder.Body.String()) {
				t.Fatalf("public Group A dispatch %s %s did not reach its handler: %d %s", dispatch.method, dispatch.path, recorder.Code, recorder.Body.String())
			}
			continue
		}
		if recorder.Code != stdhttp.StatusUnauthorized {
			t.Fatalf("session Group A dispatch %s %s = %d, want 401", dispatch.method, dispatch.path, recorder.Code)
		}
	}
}

func routeSurfaceGroupAOwnsOperation(path string) bool {
	if path == "/api/v1/gateway/proxy/chat/completions" || path == "/api/v1/app/me/preferences" {
		return true
	}
	prefixes := []string{
		"/api/v1/auth/",
		"/api/v1/agent/memories",
		"/api/v1/agent/tools",
		"/api/v1/agent/runs",
		"/api/v1/conversations",
		"/api/v1/app/tasks",
		"/api/v1/scheduled-tasks",
	}
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func routeSurfaceFunctionCalls(t *testing.T, sourceFile string) map[string]int {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, sourceFile, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", sourceFile, err)
	}
	calls := map[string]int{}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		ident, ok := call.Fun.(*ast.Ident)
		if ok {
			calls[ident.Name]++
		}
		return true
	})
	return calls
}

func routeSurfaceSortedGroupAOperationIDs() []string {
	operations := [][]OperationContractMetadataV1{
		authRouteSurfaceOperations(),
		agentMemoryRouteSurfaceOperations(),
		agentRunRouteSurfaceOperations(),
		conversationAliasRouteSurfaceOperations(),
		gatewayRouteSurfaceOperations(),
		preferenceRouteSurfaceOperations(),
		taskRouteSurfaceOperations(),
		scheduleRouteSurfaceOperations(),
	}
	ids := make([]string, 0, 58)
	for _, ownerOperations := range operations {
		for _, operation := range ownerOperations {
			ids = append(ids, operation.OperationID)
		}
	}
	sort.Strings(ids)
	return ids
}
