package http

import (
	"go/ast"
	"go/parser"
	"go/token"
	stdhttp "net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
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

	evidence := routeSurfaceRunGroupAContractHarness(t, manifest, expected, actual)
	if evidence.mutationCases == 0 || evidence.registrations == 0 || evidence.descriptors == 0 || evidence.dispatches == 0 {
		t.Fatalf(
			"Group A contract evidence is vacuous: mutations=%d registrations=%d descriptors=%d dispatches=%d",
			evidence.mutationCases,
			evidence.registrations,
			evidence.descriptors,
			evidence.dispatches,
		)
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

type routeSurfaceGroupAEvidence struct {
	mutationCases int
	registrations int
	descriptors   int
	dispatches    int
}

func routeSurfaceRunGroupAContractHarness(t *testing.T, manifest routeSurfaceManifest, expected []OperationContractMetadataV1, actual []RouteSurfaceDescriptor) routeSurfaceGroupAEvidence {
	t.Helper()
	evidence := routeSurfaceGroupAEvidence{}

	t.Run("operation inventory is complete sorted and unique", func(t *testing.T) {
		actualIDs := routeSurfaceSortedGroupAOperationIDs()
		if len(actualIDs) != 58 {
			t.Fatalf("Group A operation discovery count = %d, want 58", len(actualIDs))
		}
		for index, operationID := range actualIDs {
			if strings.TrimSpace(operationID) == "" {
				t.Fatalf("Group A operation ID %d is empty", index)
			}
			if index > 0 && actualIDs[index-1] >= operationID {
				t.Fatalf("Group A operation IDs are not strictly sorted and unique at %q", operationID)
			}
		}

		expectedIDs := make([]string, 0, len(expected))
		for _, operation := range expected {
			expectedIDs = append(expectedIDs, operation.OperationID)
		}
		sort.Strings(expectedIDs)
		if !reflect.DeepEqual(actualIDs, expectedIDs) {
			t.Fatalf("Group A owner/manifest operation IDs differ:\nowner=%v\nmanifest=%v", actualIDs, expectedIDs)
		}
	})

	t.Run("exact representative handlers dispatch through injected counters", func(t *testing.T) {
		evidence.registrations, evidence.descriptors, evidence.dispatches = routeSurfaceGroupADispatchEvidence(t, actual)
	})

	t.Run("registration rejects one-field mutations", func(t *testing.T) {
		evidence.mutationCases += routeSurfaceGroupARegistrationMutations(t, actual)
	})

	t.Run("immutable comparison rejects one-field mutations in both directions", func(t *testing.T) {
		evidence.mutationCases += routeSurfaceGroupAComparisonMutations(t, actual)
	})

	t.Run("duplicate excluded empty and direct mount bypass fail closed", func(t *testing.T) {
		evidence.mutationCases += routeSurfaceGroupAClosureMutations(t, manifest, expected, actual)
	})

	return evidence
}

func routeSurfaceGroupADispatchEvidence(t *testing.T, actual []RouteSurfaceDescriptor) (int, int, int) {
	t.Helper()
	probes := []struct {
		operationID string
		method      string
		path        string
	}{
		{"login", stdhttp.MethodPost, "/api/v1/auth/login"},
		{"gatewayProxyCreateChatCompletion", stdhttp.MethodPost, "/api/v1/gateway/proxy/chat/completions"},
		{"listConversationsAlias", stdhttp.MethodGet, "/api/v1/conversations"},
		{"getTask", stdhttp.MethodGet, "/api/v1/app/tasks/task_1"},
		{"listScheduledTaskRuns", stdhttp.MethodGet, "/api/v1/scheduled-tasks/schedule_1/runs"},
	}
	descriptors := make(map[string]RouteSurfaceDescriptor, len(actual))
	policies := routeSurfaceGroupATestPolicies(actual)
	for _, descriptor := range actual {
		descriptors[descriptor.OperationID] = cloneRouteSurfaceDescriptor(descriptor)
	}

	mux := stdhttp.NewServeMux()
	registrar, err := NewRouteSurfaceRegistrar(mux, policies)
	if err != nil {
		t.Fatalf("construct Group A dispatch registrar: %v", err)
	}
	dispatchCounts := make(map[string]int, len(probes))
	for _, probe := range probes {
		descriptor, ok := descriptors[probe.operationID]
		if !ok {
			t.Fatalf("Group A dispatch probe operation %s was not discovered", probe.operationID)
		}
		operationID := descriptor.OperationID
		registration := routeSurfaceRegistrationFromOperation(
			operationFromRouteSurfaceDescriptor(descriptor),
			descriptor.Auth,
			descriptor.MiddlewareIDs,
			descriptor.GuardEffectID,
			stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, _ *stdhttp.Request) {
				dispatchCounts[operationID]++
				w.WriteHeader(stdhttp.StatusNoContent)
			}),
		)
		if err := registrar.Register(registration); err != nil {
			t.Fatalf("register Group A dispatch probe %s: %v", probe.operationID, err)
		}
	}
	if err := registrar.registerRoutingFallbacks(); err != nil {
		t.Fatalf("register Group A dispatch fallbacks: %v", err)
	}
	if err := registrar.validateSnapshot(); err != nil {
		t.Fatalf("validate Group A dispatch snapshot: %v", err)
	}

	for _, probe := range probes {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(probe.method, probe.path, strings.NewReader(`{}`))
		request.Header.Set("Content-Type", "application/json")
		mux.ServeHTTP(recorder, request)
		if recorder.Code != stdhttp.StatusNoContent {
			t.Fatalf("Group A injected dispatch %s %s = %d, want 204", probe.method, probe.path, recorder.Code)
		}
		if dispatchCounts[probe.operationID] != 1 {
			t.Fatalf("Group A injected dispatch %s count = %d, want 1", probe.operationID, dispatchCounts[probe.operationID])
		}
	}

	return len(probes), len(registrar.Snapshot()), len(dispatchCounts)
}

func routeSurfaceGroupARegistrationMutations(t *testing.T, actual []RouteSurfaceDescriptor) int {
	t.Helper()
	baseline := routeSurfaceGroupADescriptorByID(t, actual, "listTasks")
	registration := routeSurfaceRegistrationFromOperation(
		operationFromRouteSurfaceDescriptor(baseline),
		baseline.Auth,
		baseline.MiddlewareIDs,
		baseline.GuardEffectID,
		stdhttp.HandlerFunc(func(stdhttp.ResponseWriter, *stdhttp.Request) {}),
	)
	mutations := []struct {
		name string
		code string
		edit func(*RouteSurfaceRegistration, *RouteSurfacePolicies)
	}{
		{"method", "route_surface_registration_invalid", func(reg *RouteSurfaceRegistration, _ *RouteSurfacePolicies) { reg.Method = "BREW" }},
		{"path", "route_surface_registration_invalid", func(reg *RouteSurfaceRegistration, _ *RouteSurfacePolicies) { reg.Path = "api/v1/app/tasks" }},
		{"auth", "route_surface_registration_invalid", func(reg *RouteSurfaceRegistration, _ *RouteSurfacePolicies) { reg.Auth = RouteSurfaceAuth("unknown") }},
		{"csrf", "route_surface_policy_missing", func(reg *RouteSurfaceRegistration, _ *RouteSurfacePolicies) { reg.CSRF = true }},
		{"guard", "route_surface_policy_missing", func(reg *RouteSurfaceRegistration, policies *RouteSurfacePolicies) {
			reg.GuardEffectID = "group-a.guard"
			policies.Guard = nil
		}},
		{"capability", "route_surface_capability_unknown", func(reg *RouteSurfaceRegistration, _ *RouteSurfacePolicies) { reg.CapabilityID = "group-a.unknown" }},
		{"request", "route_surface_schema_invalid", func(reg *RouteSurfaceRegistration, _ *RouteSurfacePolicies) {
			reg.Request.SchemaIdentity.Kind = "browser"
		}},
		{"response", "route_surface_response_invalid", func(reg *RouteSurfaceRegistration, _ *RouteSurfacePolicies) { reg.SuccessResponses[0].Status = "999" }},
	}

	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			candidate := routeSurfaceCloneGroupARegistration(registration)
			policies := routeSurfaceGroupATestPolicies(actual)
			mutation.edit(&candidate, &policies)
			registrar, err := NewRouteSurfaceRegistrar(stdhttp.NewServeMux(), policies)
			if err != nil {
				t.Fatalf("construct %s mutation registrar: %v", mutation.name, err)
			}
			if err := registrar.Register(candidate); routeSurfaceErrorCode(err) != mutation.code {
				t.Fatalf("Group A registration mutation %s error = %v, want %s", mutation.name, err, mutation.code)
			}
		})
	}
	return len(mutations)
}

func routeSurfaceGroupAComparisonMutations(t *testing.T, actual []RouteSurfaceDescriptor) int {
	t.Helper()
	baselineDescriptor := routeSurfaceGroupADescriptorByID(t, actual, "gatewayProxyCreateChatCompletion")
	baselineRegistration := routeSurfaceRegistrationFromOperation(
		operationFromRouteSurfaceDescriptor(baselineDescriptor),
		baselineDescriptor.Auth,
		baselineDescriptor.MiddlewareIDs,
		baselineDescriptor.GuardEffectID,
		stdhttp.HandlerFunc(func(stdhttp.ResponseWriter, *stdhttp.Request) {}),
	)
	if err := routeSurfaceCompareGroupARegistration(baselineRegistration, baselineDescriptor); err != nil {
		t.Fatalf("compare Group A baseline registration: %v", err)
	}

	mutations := []struct {
		name       string
		editReg    func(*RouteSurfaceRegistration)
		editActual func(*RouteSurfaceDescriptor)
	}{
		{"method", func(value *RouteSurfaceRegistration) { value.Method = stdhttp.MethodPut }, func(value *RouteSurfaceDescriptor) { value.Method = stdhttp.MethodPut }},
		{"path", func(value *RouteSurfaceRegistration) { value.Path += "/mutated" }, func(value *RouteSurfaceDescriptor) { value.Path += "/mutated" }},
		{"auth", func(value *RouteSurfaceRegistration) { value.Auth = RouteSurfaceAuthAdmin }, func(value *RouteSurfaceDescriptor) { value.Auth = RouteSurfaceAuthAdmin }},
		{"csrf", func(value *RouteSurfaceRegistration) { value.CSRF = false }, func(value *RouteSurfaceDescriptor) { value.CSRF = false }},
		{"guard", func(value *RouteSurfaceRegistration) { value.GuardEffectID = "group-a.guard" }, func(value *RouteSurfaceDescriptor) { value.GuardEffectID = "group-a.guard" }},
		{"capability", func(value *RouteSurfaceRegistration) { value.CapabilityID += ".mutated" }, func(value *RouteSurfaceDescriptor) { value.CapabilityID += ".mutated" }},
		{"request", func(value *RouteSurfaceRegistration) { value.Request.MediaType = "text/plain" }, func(value *RouteSurfaceDescriptor) { value.Request.MediaType = "text/plain" }},
		{"response", func(value *RouteSurfaceRegistration) { value.SuccessResponses[0].Status = "201" }, func(value *RouteSurfaceDescriptor) { value.SuccessResponses[0].Status = "201" }},
	}

	for _, mutation := range mutations {
		t.Run(mutation.name+" registration", func(t *testing.T) {
			candidate := routeSurfaceCloneGroupARegistration(baselineRegistration)
			mutation.editReg(&candidate)
			if err := routeSurfaceCompareGroupARegistration(candidate, baselineDescriptor); routeSurfaceErrorCode(err) != "route_surface_group_a_snapshot_mismatch" {
				t.Fatalf("Group A registration-side mutation %s error = %v", mutation.name, err)
			}
		})
		t.Run(mutation.name+" descriptor", func(t *testing.T) {
			candidate := cloneRouteSurfaceDescriptor(baselineDescriptor)
			mutation.editActual(&candidate)
			if err := routeSurfaceCompareGroupARegistration(baselineRegistration, candidate); routeSurfaceErrorCode(err) != "route_surface_group_a_snapshot_mismatch" {
				t.Fatalf("Group A descriptor-side mutation %s error = %v", mutation.name, err)
			}
		})
	}
	return len(mutations) * 2
}

func routeSurfaceGroupAClosureMutations(t *testing.T, manifest routeSurfaceManifest, expected []OperationContractMetadataV1, actual []RouteSurfaceDescriptor) int {
	t.Helper()
	baseline := routeSurfaceGroupADescriptorByID(t, actual, "listTasks")
	registration := routeSurfaceRegistrationFromOperation(
		operationFromRouteSurfaceDescriptor(baseline),
		baseline.Auth,
		baseline.MiddlewareIDs,
		baseline.GuardEffectID,
		stdhttp.HandlerFunc(func(stdhttp.ResponseWriter, *stdhttp.Request) {}),
	)

	t.Run("duplicate", func(t *testing.T) {
		registrar, err := NewRouteSurfaceRegistrar(stdhttp.NewServeMux(), routeSurfaceGroupATestPolicies(actual))
		if err != nil {
			t.Fatalf("construct duplicate registrar: %v", err)
		}
		if err := registrar.Register(registration); err != nil {
			t.Fatalf("register duplicate baseline: %v", err)
		}
		if err := registrar.Register(registration); routeSurfaceErrorCode(err) != "route_surface_duplicate" {
			t.Fatalf("duplicate registration error = %v", err)
		}
		duplicate := append(routeSurfaceCloneDescriptors(actual), cloneRouteSurfaceDescriptor(actual[0]))
		if _, err := CompareRouteSurfaceSnapshot(manifest.Scope, expected, duplicate); routeSurfaceErrorCode(err) != "route_surface_duplicate" {
			t.Fatalf("duplicate descriptor error = %v", err)
		}
	})

	t.Run("excluded", func(t *testing.T) {
		excluded := cloneRouteSurfaceDescriptor(actual[0])
		excluded.Method = stdhttp.MethodGet
		excluded.Path = "/healthz"
		if _, err := CompareRouteSurfaceSnapshot(manifest.Scope, expected, append(routeSurfaceCloneDescriptors(actual), excluded)); routeSurfaceErrorCode(err) != "route_surface_excluded_registered" {
			t.Fatalf("excluded descriptor error = %v", err)
		}
	})

	t.Run("zero inventory", func(t *testing.T) {
		empty, err := NewRouteSurfaceRegistrar(stdhttp.NewServeMux(), routeSurfaceGroupATestPolicies(actual))
		if err != nil {
			t.Fatalf("construct empty registrar: %v", err)
		}
		if err := empty.validateSnapshot(); routeSurfaceErrorCode(err) != "route_surface_inventory_empty" {
			t.Fatalf("zero registration inventory error = %v", err)
		}
		if _, err := CompareRouteSurfaceSnapshot(manifest.Scope, nil, nil); routeSurfaceErrorCode(err) != "route_surface_inventory_empty" {
			t.Fatalf("zero comparison inventory error = %v", err)
		}
	})

	t.Run("direct mount and missing descriptor", func(t *testing.T) {
		directSource := `package fixture
import "net/http"
func register(mux *http.ServeMux, handler http.Handler) {
	mux.Handle("GET /api/v1/group-a/bypass", handler)
}`
		if count := routeSurfaceDirectHandleCount(t, directSource); count != 1 {
			t.Fatalf("direct Group A bypass fixture count = %d, want 1", count)
		}

		mux := stdhttp.NewServeMux()
		registrar, err := NewRouteSurfaceRegistrar(mux, routeSurfaceGroupATestPolicies(actual))
		if err != nil {
			t.Fatalf("construct split registrar: %v", err)
		}
		if err := registrar.Register(registration); err != nil {
			t.Fatalf("register split baseline: %v", err)
		}
		secondDescriptor := routeSurfaceGroupADescriptorByID(t, actual, "gatewayProxyCreateChatCompletion")
		secondRegistration := routeSurfaceRegistrationFromOperation(
			operationFromRouteSurfaceDescriptor(secondDescriptor),
			secondDescriptor.Auth,
			secondDescriptor.MiddlewareIDs,
			secondDescriptor.GuardEffectID,
			stdhttp.HandlerFunc(func(stdhttp.ResponseWriter, *stdhttp.Request) {}),
		)
		if err := registrar.Register(secondRegistration); err != nil {
			t.Fatalf("register split control: %v", err)
		}
		key := routeSurfaceKey(registration.Method, registration.Path)
		pattern := registrar.mounts[key]
		directPattern := "GET /api/v1/group-a/bypass"
		mux.Handle(directPattern, stdhttp.HandlerFunc(func(stdhttp.ResponseWriter, *stdhttp.Request) {}))
		registrar.mounts[directPattern] = directPattern
		if err := registrar.validateSnapshot(); routeSurfaceErrorCode(err) != "route_surface_mount_descriptor_mismatch" {
			t.Fatalf("direct mount without descriptor error = %v", err)
		}
		delete(registrar.mounts, directPattern)
		delete(registrar.mounts, key)
		if err := registrar.validateSnapshot(); routeSurfaceErrorCode(err) != "route_surface_mount_descriptor_mismatch" {
			t.Fatalf("descriptor without exact mount error = %v", err)
		}
		registrar.mounts[key] = pattern
	})

	return 6
}

func routeSurfaceCompareGroupARegistration(registration RouteSurfaceRegistration, descriptor RouteSurfaceDescriptor) error {
	policies := routeSurfaceGroupATestPolicies([]RouteSurfaceDescriptor{descriptor})
	policies.AllowedCapabilities[registration.CapabilityID] = struct{}{}
	for _, middlewareID := range registration.MiddlewareIDs {
		policies.Middleware[middlewareID] = routeSurfacePassMiddleware
	}
	normalized, err := descriptorFromRegistration(registration, policies)
	if err != nil || !reflect.DeepEqual(normalized, descriptor) {
		return routeSurfaceError("route_surface_group_a_snapshot_mismatch", descriptor.OperationID)
	}
	return nil
}

func routeSurfaceGroupATestPolicies(descriptors []RouteSurfaceDescriptor) RouteSurfacePolicies {
	auth := map[RouteSurfaceAuth]RouteSurfaceMiddleware{
		RouteSurfaceAuthSession: routeSurfacePassMiddleware,
		RouteSurfaceAuthAdmin:   routeSurfacePassMiddleware,
		RouteSurfaceAuthBearer:  routeSurfacePassMiddleware,
	}
	middleware := make(map[string]RouteSurfaceMiddleware)
	capabilities := make(map[string]struct{})
	for _, descriptor := range descriptors {
		capabilities[descriptor.CapabilityID] = struct{}{}
		for _, middlewareID := range descriptor.MiddlewareIDs {
			middleware[middlewareID] = routeSurfacePassMiddleware
		}
	}
	return RouteSurfacePolicies{
		Auth:                auth,
		Middleware:          middleware,
		CSRF:                routeSurfacePassMiddleware,
		Guard:               func(_, _ string, next stdhttp.Handler) stdhttp.Handler { return next },
		AllowedCapabilities: capabilities,
	}
}

func routeSurfacePassMiddleware(next stdhttp.Handler) stdhttp.Handler { return next }

func routeSurfaceGroupADescriptorByID(t *testing.T, descriptors []RouteSurfaceDescriptor, operationID string) RouteSurfaceDescriptor {
	t.Helper()
	for _, descriptor := range descriptors {
		if descriptor.OperationID == operationID {
			return cloneRouteSurfaceDescriptor(descriptor)
		}
	}
	t.Fatalf("Group A descriptor %s was not discovered", operationID)
	return RouteSurfaceDescriptor{}
}

func routeSurfaceCloneGroupARegistration(registration RouteSurfaceRegistration) RouteSurfaceRegistration {
	registration.MiddlewareIDs = append([]string(nil), registration.MiddlewareIDs...)
	registration.SuccessResponses = append([]StatusMediaSchemaIdentityV1(nil), registration.SuccessResponses...)
	return registration
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
