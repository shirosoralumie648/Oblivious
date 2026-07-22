package http

import (
	"bytes"
	"mime/multipart"
	stdhttp "net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
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

	manifest := loadRouteSurfaceManifest(t)
	groupKeys := routeSurfaceGroupBOperationKeys(t, operationSets)
	expected := make([]OperationContractMetadataV1, 0, len(groupKeys))
	for _, operation := range manifest.Operations {
		if _, ok := groupKeys[routeSurfaceKey(operation.Method, operation.NormalizedPath)]; ok {
			expected = append(expected, operation)
		}
	}
	if len(expected) != 136 {
		t.Fatalf("Group B manifest discovery count = %d, want 136", len(expected))
	}

	var registrar *RouteSurfaceRegistrar
	NewRouterWithOptions(testConfig(), nil, RouterOptions{
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
		t.Fatalf("validate Group B registrar snapshot: %v", err)
	}
	actual := make([]RouteSurfaceDescriptor, 0, len(groupKeys))
	mountedCount := 0
	for _, descriptor := range registrar.Snapshot() {
		key := routeSurfaceKey(descriptor.Method, descriptor.Path)
		if _, ok := groupKeys[key]; !ok {
			continue
		}
		actual = append(actual, descriptor)
		if registrar.mountedPatterns()[key] == descriptor.Method+" "+descriptor.Path {
			mountedCount++
		}
	}
	if len(actual) != 136 || mountedCount != 136 {
		t.Fatalf("Group B registration closure failed: registrations=%d mounts=%d", len(actual), mountedCount)
	}
	observation, err := CompareRouteSurfaceSnapshot(manifest.Scope, expected, actual)
	if err != nil {
		t.Fatalf("compare Group B runtime snapshot: %v", err)
	}
	if observation.ParityResult != "pass" || observation.OperationCount != 136 || observation.MountedCount != 136 || observation.DescriptorCount != 136 || observation.MediaProbeCount == 0 || observation.CoreDigest != observation.RuntimeDigest {
		t.Fatalf("unexpected Group B observation: %+v", observation)
	}

	evidence := routeSurfaceRunGroupBContractHarness(t, manifest, operationSets, expected, actual)
	if evidence.mutationCases == 0 || evidence.registrations == 0 || evidence.descriptors == 0 || evidence.mediaProbes == 0 || evidence.dispatches == 0 {
		t.Fatalf(
			"Group B contract evidence is vacuous: mutations=%d registrations=%d descriptors=%d media=%d dispatches=%d",
			evidence.mutationCases,
			evidence.registrations,
			evidence.descriptors,
			evidence.mediaProbes,
			evidence.dispatches,
		)
	}
}

type routeSurfaceGroupBEvidence struct {
	mutationCases int
	registrations int
	descriptors   int
	mediaProbes   int
	dispatches    int
}

func routeSurfaceRunGroupBContractHarness(t *testing.T, manifest routeSurfaceManifest, operationSets [][]OperationContractMetadataV1, expected []OperationContractMetadataV1, actual []RouteSurfaceDescriptor) routeSurfaceGroupBEvidence {
	t.Helper()
	evidence := routeSurfaceGroupBEvidence{}

	t.Run("operation inventory is complete sorted and unique", func(t *testing.T) {
		actualIDs := routeSurfaceSortedGroupBOperationIDs(operationSets)
		if len(actualIDs) != 136 {
			t.Fatalf("Group B operation discovery count = %d, want 136", len(actualIDs))
		}
		for index, operationID := range actualIDs {
			if strings.TrimSpace(operationID) == "" {
				t.Fatalf("Group B operation ID %d is empty", index)
			}
			if index > 0 && actualIDs[index-1] >= operationID {
				t.Fatalf("Group B operation IDs are not strictly sorted and unique at %q", operationID)
			}
		}
		expectedIDs := make([]string, 0, len(expected))
		for _, operation := range expected {
			expectedIDs = append(expectedIDs, operation.OperationID)
		}
		sort.Strings(expectedIDs)
		if !reflect.DeepEqual(actualIDs, expectedIDs) {
			t.Fatalf("Group B owner/manifest operation IDs differ:\nowner=%v\nmanifest=%v", actualIDs, expectedIDs)
		}
	})

	t.Run("json markdown sse multipart and owner handlers dispatch", func(t *testing.T) {
		evidence.registrations, evidence.descriptors, evidence.mediaProbes, evidence.dispatches = routeSurfaceGroupBDispatchEvidence(t, actual)
	})

	t.Run("registration rejects one-field mutations", func(t *testing.T) {
		evidence.mutationCases += routeSurfaceGroupBRegistrationMutations(t, actual)
	})

	t.Run("immutable comparison rejects one-field mutations in both directions", func(t *testing.T) {
		evidence.mutationCases += routeSurfaceGroupBComparisonMutations(t, actual)
	})

	t.Run("duplicate excluded empty and direct mount bypass fail closed", func(t *testing.T) {
		evidence.mutationCases += routeSurfaceGroupBClosureMutations(t, manifest, expected, actual)
	})

	return evidence
}

func routeSurfaceGroupBOperationKeys(t *testing.T, operationSets [][]OperationContractMetadataV1) map[string]struct{} {
	t.Helper()
	keys := make(map[string]struct{}, 136)
	for _, operations := range operationSets {
		for _, operation := range operations {
			key := routeSurfaceKey(operation.Method, operation.NormalizedPath)
			if _, exists := keys[key]; exists {
				t.Fatalf("duplicate Group B operation key %s", key)
			}
			keys[key] = struct{}{}
		}
	}
	return keys
}

func routeSurfaceSortedGroupBOperationIDs(operationSets [][]OperationContractMetadataV1) []string {
	ids := make([]string, 0, 136)
	for _, operations := range operationSets {
		for _, operation := range operations {
			ids = append(ids, operation.OperationID)
		}
	}
	sort.Strings(ids)
	return ids
}

func routeSurfaceGroupBDispatchEvidence(t *testing.T, actual []RouteSurfaceDescriptor) (int, int, int, int) {
	t.Helper()
	probes := []struct {
		operationID string
		method      string
		path        string
	}{
		{"createConversation", stdhttp.MethodPost, "/api/v1/app/conversations"},
		{"exportConversationMarkdown", stdhttp.MethodGet, "/api/v1/app/conversations/conversation_1/export.md"},
		{"streamMessage", stdhttp.MethodPost, "/api/v1/app/conversations/conversation_1/messages/stream"},
		{"getPublishingChannel", stdhttp.MethodGet, "/api/v1/channels/channel_1"},
		{"listConsoleBillingInvoices", stdhttp.MethodGet, "/api/v1/console/invoices"},
		{"uploadKnowledgeDocument", stdhttp.MethodPost, "/api/v1/app/knowledge-bases/kb_1/documents/upload"},
		{"listWorkflows", stdhttp.MethodGet, "/api/v1/workflows"},
		{"listAdminObservabilityAlerts", stdhttp.MethodGet, "/api/v1/admin/observability/alerts"},
		{"getAdminReleaseEvidenceRAGIndexingProof", stdhttp.MethodGet, "/api/v1/admin/release-evidence/rag-indexing"},
	}
	descriptors := make(map[string]RouteSurfaceDescriptor, len(actual))
	for _, descriptor := range actual {
		descriptors[descriptor.OperationID] = cloneRouteSurfaceDescriptor(descriptor)
	}

	mux := stdhttp.NewServeMux()
	registrar, err := NewRouteSurfaceRegistrar(mux, routeSurfaceGroupATestPolicies(actual))
	if err != nil {
		t.Fatalf("construct Group B dispatch registrar: %v", err)
	}
	dispatchCounts := make(map[string]int, len(probes))
	mediaClasses := make(map[string]struct{})
	for _, probe := range probes {
		descriptor, ok := descriptors[probe.operationID]
		if !ok {
			t.Fatalf("Group B dispatch probe operation %s was not discovered", probe.operationID)
		}
		operationID := descriptor.OperationID
		probeDescriptor := descriptor
		registration := routeSurfaceRegistrationFromOperation(
			operationFromRouteSurfaceDescriptor(probeDescriptor),
			probeDescriptor.Auth,
			probeDescriptor.MiddlewareIDs,
			probeDescriptor.GuardEffectID,
			stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, request *stdhttp.Request) {
				if probeDescriptor.Request.MediaType != "" {
					wantMedia, _ := routeSurfaceBaseMedia(probeDescriptor.Request.MediaType)
					gotMedia, ok := routeSurfaceBaseMedia(request.Header.Get("Content-Type"))
					if !ok || gotMedia != wantMedia {
						t.Errorf("Group B dispatch %s request media = %q, want %q", operationID, gotMedia, wantMedia)
						w.WriteHeader(stdhttp.StatusUnsupportedMediaType)
						return
					}
				}
				dispatchCounts[operationID]++
				response := probeDescriptor.SuccessResponses[0]
				if response.MediaType != "" {
					w.Header().Set("Content-Type", response.MediaType)
				}
				status, parseErr := strconv.Atoi(response.Status)
				if parseErr != nil {
					t.Errorf("Group B dispatch %s response status: %v", operationID, parseErr)
					status = stdhttp.StatusInternalServerError
				}
				w.WriteHeader(status)
			}),
		)
		if err := registrar.Register(registration); err != nil {
			t.Fatalf("register Group B dispatch probe %s: %v", probe.operationID, err)
		}
		if descriptor.Request.MediaType != "" {
			media, _ := routeSurfaceBaseMedia(descriptor.Request.MediaType)
			mediaClasses[media] = struct{}{}
		}
		for _, response := range descriptor.SuccessResponses {
			if response.MediaType != "" {
				media, _ := routeSurfaceBaseMedia(response.MediaType)
				mediaClasses[media] = struct{}{}
			}
		}
	}
	if err := registrar.registerRoutingFallbacks(); err != nil {
		t.Fatalf("register Group B dispatch fallbacks: %v", err)
	}
	if err := registrar.validateSnapshot(); err != nil {
		t.Fatalf("validate Group B dispatch snapshot: %v", err)
	}

	for _, probe := range probes {
		descriptor := descriptors[probe.operationID]
		request := routeSurfaceGroupBProbeRequest(t, probe.method, probe.path, descriptor.Request.MediaType)
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, request)
		wantStatus, _ := strconv.Atoi(descriptor.SuccessResponses[0].Status)
		if recorder.Code != wantStatus {
			t.Fatalf("Group B injected dispatch %s %s = %d, want %d", probe.method, probe.path, recorder.Code, wantStatus)
		}
		if dispatchCounts[probe.operationID] != 1 {
			t.Fatalf("Group B injected dispatch %s count = %d, want 1", probe.operationID, dispatchCounts[probe.operationID])
		}
		wantMedia, _ := routeSurfaceBaseMedia(descriptor.SuccessResponses[0].MediaType)
		gotMedia, ok := routeSurfaceBaseMedia(recorder.Header().Get("Content-Type"))
		if !ok || gotMedia != wantMedia {
			t.Fatalf("Group B injected dispatch %s response media = %q, want %q", probe.operationID, gotMedia, wantMedia)
		}
	}
	for _, required := range []string{"application/json", "text/markdown", "text/event-stream", "multipart/form-data"} {
		if _, ok := mediaClasses[required]; !ok {
			t.Fatalf("Group B dispatch media class %s was not probed", required)
		}
	}

	return len(probes), len(registrar.Snapshot()), len(mediaClasses), len(dispatchCounts)
}

func routeSurfaceGroupBProbeRequest(t *testing.T, method, path, mediaType string) *stdhttp.Request {
	t.Helper()
	baseMedia, ok := routeSurfaceBaseMedia(mediaType)
	if !ok {
		t.Fatalf("invalid Group B probe media %q", mediaType)
	}
	if baseMedia == "multipart/form-data" {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, err := writer.CreateFormFile("file", "document.txt")
		if err != nil {
			t.Fatalf("create Group B multipart probe: %v", err)
		}
		if _, err := part.Write([]byte("group-b")); err != nil {
			t.Fatalf("write Group B multipart probe: %v", err)
		}
		if err := writer.Close(); err != nil {
			t.Fatalf("close Group B multipart probe: %v", err)
		}
		request := httptest.NewRequest(method, path, &body)
		request.Header.Set("Content-Type", writer.FormDataContentType())
		return request
	}
	body := strings.NewReader("")
	if baseMedia == "application/json" {
		body = strings.NewReader(`{}`)
	}
	request := httptest.NewRequest(method, path, body)
	if baseMedia != "" {
		request.Header.Set("Content-Type", baseMedia)
	}
	return request
}

func routeSurfaceGroupBRegistrationMutations(t *testing.T, actual []RouteSurfaceDescriptor) int {
	t.Helper()
	baseline := routeSurfaceGroupBDescriptorByID(t, actual, "listWorkflows")
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
		{"path", "route_surface_registration_invalid", func(reg *RouteSurfaceRegistration, _ *RouteSurfacePolicies) { reg.Path = "api/v1/workflows" }},
		{"auth", "route_surface_registration_invalid", func(reg *RouteSurfaceRegistration, _ *RouteSurfacePolicies) { reg.Auth = RouteSurfaceAuth("unknown") }},
		{"csrf", "route_surface_policy_missing", func(reg *RouteSurfaceRegistration, _ *RouteSurfacePolicies) { reg.CSRF = true }},
		{"guard", "route_surface_policy_missing", func(reg *RouteSurfaceRegistration, policies *RouteSurfacePolicies) {
			reg.GuardEffectID = "group-b.guard"
			policies.Guard = nil
		}},
		{"capability", "route_surface_capability_unknown", func(reg *RouteSurfaceRegistration, _ *RouteSurfacePolicies) { reg.CapabilityID = "group-b.unknown" }},
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
				t.Fatalf("Group B registration mutation %s error = %v, want %s", mutation.name, err, mutation.code)
			}
		})
	}
	return len(mutations)
}

func routeSurfaceGroupBComparisonMutations(t *testing.T, actual []RouteSurfaceDescriptor) int {
	t.Helper()
	baselineDescriptor := routeSurfaceGroupBDescriptorByID(t, actual, "streamMessage")
	baselineRegistration := routeSurfaceRegistrationFromOperation(
		operationFromRouteSurfaceDescriptor(baselineDescriptor),
		baselineDescriptor.Auth,
		baselineDescriptor.MiddlewareIDs,
		baselineDescriptor.GuardEffectID,
		stdhttp.HandlerFunc(func(stdhttp.ResponseWriter, *stdhttp.Request) {}),
	)
	if err := routeSurfaceCompareGroupBRegistration(baselineRegistration, baselineDescriptor); err != nil {
		t.Fatalf("compare Group B baseline registration: %v", err)
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
		{"guard", func(value *RouteSurfaceRegistration) { value.GuardEffectID = "group-b.guard" }, func(value *RouteSurfaceDescriptor) { value.GuardEffectID = "group-b.guard" }},
		{"capability", func(value *RouteSurfaceRegistration) { value.CapabilityID += ".mutated" }, func(value *RouteSurfaceDescriptor) { value.CapabilityID += ".mutated" }},
		{"request", func(value *RouteSurfaceRegistration) { value.Request.MediaType = "text/plain" }, func(value *RouteSurfaceDescriptor) { value.Request.MediaType = "text/plain" }},
		{"response", func(value *RouteSurfaceRegistration) { value.SuccessResponses[0].Status = "201" }, func(value *RouteSurfaceDescriptor) { value.SuccessResponses[0].Status = "201" }},
	}

	for _, mutation := range mutations {
		t.Run(mutation.name+" registration", func(t *testing.T) {
			candidate := routeSurfaceCloneGroupARegistration(baselineRegistration)
			mutation.editReg(&candidate)
			if err := routeSurfaceCompareGroupBRegistration(candidate, baselineDescriptor); routeSurfaceErrorCode(err) != "route_surface_group_b_snapshot_mismatch" {
				t.Fatalf("Group B registration-side mutation %s error = %v", mutation.name, err)
			}
		})
		t.Run(mutation.name+" descriptor", func(t *testing.T) {
			candidate := cloneRouteSurfaceDescriptor(baselineDescriptor)
			mutation.editActual(&candidate)
			if err := routeSurfaceCompareGroupBRegistration(baselineRegistration, candidate); routeSurfaceErrorCode(err) != "route_surface_group_b_snapshot_mismatch" {
				t.Fatalf("Group B descriptor-side mutation %s error = %v", mutation.name, err)
			}
		})
	}
	return len(mutations) * 2
}

func routeSurfaceCompareGroupBRegistration(registration RouteSurfaceRegistration, descriptor RouteSurfaceDescriptor) error {
	policies := routeSurfaceGroupATestPolicies([]RouteSurfaceDescriptor{descriptor})
	policies.AllowedCapabilities[registration.CapabilityID] = struct{}{}
	for _, middlewareID := range registration.MiddlewareIDs {
		policies.Middleware[middlewareID] = routeSurfacePassMiddleware
	}
	normalized, err := descriptorFromRegistration(registration, policies)
	if err != nil || !reflect.DeepEqual(normalized, descriptor) {
		return routeSurfaceError("route_surface_group_b_snapshot_mismatch", descriptor.OperationID)
	}
	return nil
}

func routeSurfaceGroupBDescriptorByID(t *testing.T, descriptors []RouteSurfaceDescriptor, operationID string) RouteSurfaceDescriptor {
	t.Helper()
	for _, descriptor := range descriptors {
		if descriptor.OperationID == operationID {
			return cloneRouteSurfaceDescriptor(descriptor)
		}
	}
	t.Fatalf("Group B descriptor %s was not discovered", operationID)
	return RouteSurfaceDescriptor{}
}

func routeSurfaceGroupBClosureMutations(t *testing.T, manifest routeSurfaceManifest, expected []OperationContractMetadataV1, actual []RouteSurfaceDescriptor) int {
	t.Helper()
	baseline := routeSurfaceGroupBDescriptorByID(t, actual, "listWorkflows")
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
	mux.Handle("GET /api/v1/group-b/bypass", handler)
}`
		if count := routeSurfaceDirectHandleCount(t, directSource); count != 1 {
			t.Fatalf("direct Group B bypass fixture count = %d, want 1", count)
		}

		mux := stdhttp.NewServeMux()
		registrar, err := NewRouteSurfaceRegistrar(mux, routeSurfaceGroupATestPolicies(actual))
		if err != nil {
			t.Fatalf("construct split registrar: %v", err)
		}
		if err := registrar.Register(registration); err != nil {
			t.Fatalf("register split baseline: %v", err)
		}
		secondDescriptor := routeSurfaceGroupBDescriptorByID(t, actual, "getAdminReleaseEvidenceRAGIndexingProof")
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
		directPattern := "GET /api/v1/group-b/bypass"
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
