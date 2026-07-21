package http

import (
	"context"
	"database/sql"
	"go/ast"
	"go/parser"
	"go/token"
	stdhttp "net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"oblivious/server/internal/config"
	"oblivious/server/internal/releasecontract"
)

func TestRuntimeEffectRegistrySnapshotContract(t *testing.T) {
	database, err := sql.Open("postgres", "postgres://unused")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	descriptors := []releasecontract.EffectDescriptor{
		{ID: "mcp.transport.dispatch", CapabilityID: "mcp.tool_execution", Boundary: releasecontract.BoundaryOutbound, Owner: "mcp.Client"},
		{ID: "chat.provider.dispatch", CapabilityID: "relay.provider_inference", Boundary: releasecontract.BoundaryOutbound, Owner: "chat.RelayGateway"},
	}
	registry := releasecontract.NewEffectRegistry()
	runtime, err := buildRuntimeWithRouter(config.Config{Env: "test"}, database, RuntimeOptions{Effects: registry}, false,
		func(config.Config, *sql.DB, RouterOptions) (stdhttp.Handler, error) {
			for _, descriptor := range descriptors {
				if err := registry.Register(descriptor); err != nil {
					return nil, err
				}
			}
			return stdhttp.HandlerFunc(func(stdhttp.ResponseWriter, *stdhttp.Request) {}), nil
		})
	if err != nil {
		t.Fatalf("build controlled runtime: %v", err)
	}
	t.Cleanup(func() { _ = runtime.Close(context.Background()) })

	want := []releasecontract.EffectDescriptor{descriptors[1], descriptors[0]}
	first := runtime.EffectDescriptors()
	if !reflect.DeepEqual(first, want) {
		t.Fatalf("runtime descriptors = %#v, want %#v", first, want)
	}
	first[0].ID = "mutated"
	if got := runtime.EffectDescriptors(); !reflect.DeepEqual(got, want) {
		t.Fatalf("runtime descriptor snapshot retained mutable alias: %#v", got)
	}

	duplicateRegistry := releasecontract.NewEffectRegistry()
	failedRuntime, err := buildRuntimeWithRouter(config.Config{Env: "test"}, database, RuntimeOptions{Effects: duplicateRegistry}, false,
		func(config.Config, *sql.DB, RouterOptions) (stdhttp.Handler, error) {
			if err := duplicateRegistry.Register(descriptors[0]); err != nil {
				return nil, err
			}
			return nil, duplicateRegistry.Register(descriptors[0])
		})
	if failedRuntime != nil || !releasecontract.IsEffectCoverageCode(err, "effect_registry_duplicate") {
		t.Fatalf("duplicate construction = runtime %#v err %v, want nil/effect_registry_duplicate", failedRuntime, err)
	}
}

func TestProductionEffectCoverageContract(t *testing.T) {
	contract, profile := loadFactoryReadinessContract(t)
	repoRoot, err := filepath.Abs("../../../..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	manifest, err := releasecontract.LoadEffectSurfaceManifest(filepath.Join(repoRoot, "config/release/readiness-effect-surface.v1.json"))
	if err != nil {
		t.Fatalf("load effect surface manifest: %v", err)
	}
	guard := &factoryReadinessGuard{}
	guard.allow.Store(true)
	authorities, err := releasecontract.NewRuntimeAuthorities(contract, profile, guard)
	if err != nil {
		t.Fatalf("build runtime authorities: %v", err)
	}
	database, err := sql.Open("postgres", "postgres://unused")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })

	assertProductionCoverageFixtureConstructors(t, repoRoot)
	snapshots := make(map[string][]releasecontract.EffectDescriptor)
	for _, testCase := range []struct {
		name, configuredProvider, selectedProvider, selectedDescriptor string
	}{
		{name: "direct Tavily", configuredProvider: "tavily", selectedProvider: "tavily", selectedDescriptor: "mcp.websearch.tavily"},
		{name: "fallback chain", configuredProvider: "brave,duckduckgo", selectedProvider: "chain", selectedDescriptor: "mcp.websearch.chain"},
		{name: "single provider", configuredProvider: "brave", selectedProvider: "provider", selectedDescriptor: "mcp.websearch.provider"},
	} {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			cfg := testConfig()
			cfg.AgentWebSearchProvider = testCase.configuredProvider
			cfg.AgentWebSearchEndpoint = "https://search.invalid"
			cfg.AgentWebSearchAPIKey = "test-key"
			registry := releasecontract.NewEffectRegistry()
			runtime, err := BuildRuntime(cfg, database, RuntimeOptions{
				Readiness: strictRouterReadinessStub{}, Guard: guard, Effects: registry, Authorities: authorities,
			})
			if err != nil {
				t.Fatalf("strict production BuildRuntime: %v", err)
			}
			t.Cleanup(func() { _ = runtime.Close(context.Background()) })
			descriptors := runtime.EffectDescriptors()
			assertProductionDescriptorFamilies(t, descriptors, testCase.selectedDescriptor)
			selection := &releasecontract.EffectRuntimeConfiguration{WebSearchProvider: testCase.selectedProvider}
			if err := releasecontract.VerifyEffectCoverage(releasecontract.EffectCoverageOptions{
				RepoRoot: repoRoot, Manifest: manifest, Runtime: descriptors, Contract: contract, Profile: profile, Authorities: authorities, Selection: selection,
			}); err != nil {
				t.Fatalf("production effect coverage: %v", err)
			}
			snapshots[testCase.selectedProvider] = append([]releasecontract.EffectDescriptor(nil), descriptors...)
		})
	}

	baseline := snapshots["tavily"]
	chain := snapshots["chain"]
	missingManifest := manifest
	missingManifest.Surfaces = withoutProductionSurface(manifest.Surfaces, "chat.provider.dispatch")
	duplicateManifest := manifest
	duplicateManifest.Surfaces = append(append([]releasecontract.EffectSurface(nil), manifest.Surfaces...), productionSurfaceByDescriptor(t, manifest, "chat.provider.dispatch"))
	extraManifest := manifest
	extraManifest.Surfaces = append(append([]releasecontract.EffectSurface(nil), manifest.Surfaces...), releasecontract.EffectSurface{
		SeamID: "unknown.effect@unknown.Owner", DescriptorID: "unknown.effect", OwnerPackage: "unknown", OwnerSymbol: "unknown.Owner",
		CapabilityID: "mcp.tool_execution", Boundary: releasecontract.BoundaryOutbound, ASTCall: "structural", RegistrationSymbol: "newUnknown",
		GuardCall: "run:guard", EffectCall: "run:effect", ProfileDisposition: releasecontract.CommitmentCommitted,
	})
	missingStaticRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(missingStaticRoot, "src/server"), 0o755); err != nil {
		t.Fatalf("create static mutation root: %v", err)
	}
	modifiedContract := contract
	modifiedContract.ReasonCodes = append([]releasecontract.ReasonCode(nil), contract.ReasonCodes...)
	modifiedContract.ReasonCodes[0].Description += " changed"
	extraRuntime := append(append([]releasecontract.EffectDescriptor(nil), baseline...), releasecontract.EffectDescriptor{
		ID: "unknown.effect", CapabilityID: "mcp.tool_execution", Boundary: releasecontract.BoundaryOutbound, Owner: "unknown.Owner",
	})
	duplicateRuntime := append(append([]releasecontract.EffectDescriptor(nil), baseline...), baseline[0])
	excludedRuntime := append(append([]releasecontract.EffectDescriptor(nil), baseline...), productionDescriptorFromManifest(t, manifest, "agent.tool.web_search"))
	wrongOwnerRuntime := append([]releasecontract.EffectDescriptor(nil), baseline...)
	wrongOwnerRuntime[0].Owner = "wrong.Owner"
	missingProvider := withoutProductionDescriptor(baseline, "mcp.websearch.tavily")
	multipleProviders := append(append([]releasecontract.EffectDescriptor(nil), baseline...), productionDescriptorByID(t, chain, "mcp.websearch.chain"))

	baseOptions := func() releasecontract.EffectCoverageOptions {
		return releasecontract.EffectCoverageOptions{
			Manifest: manifest, Runtime: baseline, Contract: contract, Profile: profile, Authorities: authorities,
			Selection: &releasecontract.EffectRuntimeConfiguration{WebSearchProvider: "tavily"},
		}
	}
	mutations := []struct {
		name, expected string
		mutate         func(*releasecontract.EffectCoverageOptions)
	}{
		{name: "manifest missing", expected: "effect_manifest_missing_descriptor", mutate: func(o *releasecontract.EffectCoverageOptions) { o.Manifest = missingManifest }},
		{name: "manifest extra", expected: "effect_manifest_unknown_descriptor", mutate: func(o *releasecontract.EffectCoverageOptions) { o.Manifest = extraManifest }},
		{name: "manifest duplicate", expected: "effect_manifest_duplicate", mutate: func(o *releasecontract.EffectCoverageOptions) { o.Manifest = duplicateManifest }},
		{name: "static missing", expected: "effect_coverage_missing_static", mutate: func(o *releasecontract.EffectCoverageOptions) { o.RepoRoot = missingStaticRoot }},
		{name: "runtime zero", expected: "effect_coverage_zero_runtime", mutate: func(o *releasecontract.EffectCoverageOptions) { o.Runtime = nil }},
		{name: "runtime missing", expected: "effect_coverage_missing_runtime", mutate: func(o *releasecontract.EffectCoverageOptions) {
			o.Runtime = withoutProductionDescriptor(baseline, "agent.tool.builtin")
		}},
		{name: "runtime extra", expected: "effect_coverage_extra_runtime", mutate: func(o *releasecontract.EffectCoverageOptions) { o.Runtime = extraRuntime }},
		{name: "runtime duplicate", expected: "effect_coverage_duplicate_runtime", mutate: func(o *releasecontract.EffectCoverageOptions) { o.Runtime = duplicateRuntime }},
		{name: "runtime excluded", expected: "effect_coverage_profile_drift", mutate: func(o *releasecontract.EffectCoverageOptions) { o.Runtime = excludedRuntime }},
		{name: "runtime owner", expected: "effect_coverage_runtime_drift", mutate: func(o *releasecontract.EffectCoverageOptions) { o.Runtime = wrongOwnerRuntime }},
		{name: "profile", expected: "effect_coverage_profile_invalid", mutate: func(o *releasecontract.EffectCoverageOptions) { o.Profile = releasecontract.DeploymentProfile{} }},
		{name: "authority missing", expected: "effect_coverage_authorities_required", mutate: func(o *releasecontract.EffectCoverageOptions) { o.Authorities = releasecontract.RuntimeAuthorities{} }},
		{name: "authority contract", expected: "effect_coverage_authorities_mismatch", mutate: func(o *releasecontract.EffectCoverageOptions) { o.Contract = modifiedContract }},
		{name: "configuration missing provider", expected: "effect_coverage_selection_drift", mutate: func(o *releasecontract.EffectCoverageOptions) { o.Runtime = missingProvider }},
		{name: "configuration wrong provider", expected: "effect_coverage_selection_drift", mutate: func(o *releasecontract.EffectCoverageOptions) { o.Runtime = chain }},
		{name: "configuration multiple providers", expected: "effect_coverage_selection_drift", mutate: func(o *releasecontract.EffectCoverageOptions) { o.Runtime = multipleProviders }},
		{name: "configuration unsupported", expected: "effect_coverage_selection_drift", mutate: func(o *releasecontract.EffectCoverageOptions) {
			o.Selection = &releasecontract.EffectRuntimeConfiguration{WebSearchProvider: "unsupported"}
		}},
	}
	for _, mutation := range mutations {
		mutation := mutation
		t.Run("reject "+mutation.name, func(t *testing.T) {
			options := baseOptions()
			mutation.mutate(&options)
			if err := releasecontract.VerifyEffectCoverage(options); !releasecontract.IsEffectCoverageCode(err, mutation.expected) {
				t.Fatalf("error = %v, want %s", err, mutation.expected)
			}
		})
	}
}

func assertProductionDescriptorFamilies(t *testing.T, descriptors []releasecontract.EffectDescriptor, selectedProvider string) {
	t.Helper()
	families := map[string][]string{
		"admin":    {"admin.channel.model.mutation", "http.admin.refund"},
		"agent":    {"agent.tool.builtin", "agent.tool.custom_api_http", "agent.tool.local_python", "agent.tool.mcp"},
		"channel":  {"channel.delivery.send", "worker.channel_retry.claim"},
		"chat":     {"chat.provider.dispatch"},
		"commerce": {"http.billing.checkout", "http.marketplace.checkout", "marketplace.payout.dispatch", "marketplace.settlement.intent"},
		"mcp":      {"http.mcp.mutation", "mcp.transport.dispatch", "mcp.websearch.builtin", selectedProvider},
	}
	counts := make(map[string]int, len(descriptors))
	for index, descriptor := range descriptors {
		if index > 0 && descriptors[index-1].ID >= descriptor.ID {
			t.Fatalf("runtime descriptors are not strictly sorted at %q", descriptor.ID)
		}
		counts[descriptor.ID]++
	}
	asserted := 0
	for family, ids := range families {
		familyCount := 0
		for _, id := range ids {
			familyCount += counts[id]
			if counts[id] != 1 {
				t.Fatalf("producer family %s descriptor %s count = %d, want 1", family, id, counts[id])
			}
		}
		if familyCount != len(ids) {
			t.Fatalf("producer family %s count = %d, want %d", family, familyCount, len(ids))
		}
		asserted += familyCount
	}
	if asserted != len(descriptors) || len(descriptors) != 17 {
		t.Fatalf("asserted/actual descriptors = %d/%d, want 17/17", asserted, len(descriptors))
	}
	for _, excluded := range []string{
		"agent.tool.python_sandbox", "agent.tool.registry.builtin", "agent.tool.registry.custom", "agent.tool.registry.mcp", "agent.tool.web_search",
		"chat.provider.fallback", "relay.provider.dispatch", "worker.schedule.claim", "worker.archive.claim", "worker.relay_batch.claim",
	} {
		if counts[excluded] != 0 {
			t.Fatalf("excluded/ownerless descriptor %s count = %d, want 0", excluded, counts[excluded])
		}
	}
	for _, provider := range []string{"mcp.websearch.tavily", "mcp.websearch.chain", "mcp.websearch.provider"} {
		want := 0
		if provider == selectedProvider {
			want = 1
		}
		if counts[provider] != want {
			t.Fatalf("provider descriptor %s count = %d, want %d", provider, counts[provider], want)
		}
	}
}

func assertProductionCoverageFixtureConstructors(t *testing.T, repoRoot string) {
	t.Helper()
	path := filepath.Join(repoRoot, "src/server/internal/http/runtime_effect_coverage_test.go")
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		t.Fatalf("parse production coverage fixture: %v", err)
	}
	forbidden := map[string]bool{
		"NewWebsearchToolWithOptions":   true,
		"NewRegistryWithOptions":        true,
		"NewDefaultRegistryWithOptions": true,
	}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if ok && forbidden[selector.Sel.Name] {
			t.Errorf("production coverage fixture constructs forbidden %s", selector.Sel.Name)
		}
		return true
	})
}

func productionSurfaceByDescriptor(t *testing.T, manifest releasecontract.EffectSurfaceManifest, descriptorID string) releasecontract.EffectSurface {
	t.Helper()
	for _, surface := range manifest.Surfaces {
		if surface.DescriptorID == descriptorID {
			return surface
		}
	}
	t.Fatalf("manifest descriptor %s missing", descriptorID)
	return releasecontract.EffectSurface{}
}

func productionDescriptorFromManifest(t *testing.T, manifest releasecontract.EffectSurfaceManifest, descriptorID string) releasecontract.EffectDescriptor {
	t.Helper()
	surface := productionSurfaceByDescriptor(t, manifest, descriptorID)
	return releasecontract.EffectDescriptor{ID: descriptorID, CapabilityID: surface.CapabilityID, Boundary: surface.Boundary, Owner: surface.OwnerSymbol}
}

func productionDescriptorByID(t *testing.T, descriptors []releasecontract.EffectDescriptor, descriptorID string) releasecontract.EffectDescriptor {
	t.Helper()
	for _, descriptor := range descriptors {
		if descriptor.ID == descriptorID {
			return descriptor
		}
	}
	t.Fatalf("runtime descriptor %s missing", descriptorID)
	return releasecontract.EffectDescriptor{}
}

func withoutProductionDescriptor(source []releasecontract.EffectDescriptor, descriptorID string) []releasecontract.EffectDescriptor {
	result := make([]releasecontract.EffectDescriptor, 0, len(source))
	for _, descriptor := range source {
		if descriptor.ID != descriptorID {
			result = append(result, descriptor)
		}
	}
	return result
}

func withoutProductionSurface(source []releasecontract.EffectSurface, descriptorID string) []releasecontract.EffectSurface {
	result := make([]releasecontract.EffectSurface, 0, len(source))
	for _, surface := range source {
		if surface.DescriptorID != descriptorID {
			result = append(result, surface)
		}
	}
	return result
}
