package releasecontract

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestEffectSurfaceManifestRuntimeDescriptorContract(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", ".."))
	manifestPath := filepath.Join(repoRoot, "config", "release", "readiness-effect-surface.v1.json")
	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	var document struct {
		Surfaces []map[string]any `json:"surfaces"`
	}
	if err := json.Unmarshal(content, &document); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}
	required := map[string]bool{
		"admin.channel.model.mutation": false,
		"http.billing.checkout":        false,
		"http.marketplace.checkout":    false,
		"mcp.transport.dispatch":       false,
		"mcp.websearch.builtin":        false,
		"mcp.websearch.tavily":         false,
		"mcp.websearch.chain":          false,
		"mcp.websearch.provider":       false,
		"chat.provider.dispatch":       false,
		"chat.provider.fallback":       false,
	}
	for _, surface := range document.Surfaces {
		descriptorID, _ := surface["descriptorId"].(string)
		if _, tracked := required[descriptorID]; tracked {
			required[descriptorID] = true
		}
		if descriptorID == "" {
			continue
		}
		for _, field := range []string{"effectId", "registrationSymbol", "guardCall", "effectCall"} {
			if value, _ := surface[field].(string); strings.TrimSpace(value) == "" {
				t.Fatalf("descriptor %q missing %s", descriptorID, field)
			}
		}
	}
	for descriptorID, found := range required {
		if !found {
			t.Errorf("manifest missing runtime descriptor %q", descriptorID)
		}
	}
	source, err := os.ReadFile(filepath.Join(repoRoot, "src", "server", "internal", "releasecontract", "effect_registry.go"))
	if err != nil {
		t.Fatalf("read effect registry: %v", err)
	}
	if !strings.Contains(string(source), "runtimeDescriptorSpecs") {
		t.Fatal("effect registry missing sole runtimeDescriptorSpecs allowlist")
	}
	manifest, err := LoadEffectSurfaceManifest(manifestPath)
	if err != nil {
		t.Fatalf("load strict manifest: %v", err)
	}
	descriptorCount := 0
	for _, surface := range manifest.Surfaces {
		if surface.DescriptorID != "" {
			descriptorCount++
		}
	}
	if descriptorCount != len(runtimeDescriptorSpecs) {
		t.Fatalf("manifest descriptors=%d specs=%d", descriptorCount, len(runtimeDescriptorSpecs))
	}

	missing := manifest
	missing.Surfaces = append([]EffectSurface(nil), manifest.Surfaces...)
	for index, surface := range missing.Surfaces {
		if surface.DescriptorID == "chat.provider.dispatch" {
			missing.Surfaces = append(missing.Surfaces[:index], missing.Surfaces[index+1:]...)
			break
		}
	}
	if err := validateEffectSurfaceManifest(missing); !IsEffectCoverageCode(err, "effect_manifest_missing_descriptor") {
		t.Fatalf("missing manifest descriptor error = %v", err)
	}
	extra := manifest
	extra.Surfaces = append(append([]EffectSurface(nil), manifest.Surfaces...), EffectSurface{
		SeamID: "unknown.effect@unknown.Owner", DescriptorID: "unknown.effect", OwnerPackage: "unknown", OwnerSymbol: "unknown.Owner",
		CapabilityID: "mcp.tool_execution", Boundary: BoundaryOutbound, ASTCall: "structural", RegistrationSymbol: "newUnknown", GuardCall: "run:guard", EffectCall: "run:effect", ProfileDisposition: CommitmentCommitted,
	})
	if err := validateEffectSurfaceManifest(extra); !IsEffectCoverageCode(err, "effect_manifest_unknown_descriptor") {
		t.Fatalf("extra manifest descriptor error = %v", err)
	}
	missingMap := cloneRuntimeDescriptorSpecs(runtimeDescriptorSpecs)
	delete(missingMap, "chat.provider.dispatch")
	if err := validateEffectSurfaceManifestAgainstSpecs(manifest, missingMap); !IsEffectCoverageCode(err, "effect_manifest_unknown_descriptor") {
		t.Fatalf("missing map row error = %v", err)
	}
	extraMap := cloneRuntimeDescriptorSpecs(runtimeDescriptorSpecs)
	extraMap["unknown.effect"] = runtimeDescriptorSpec{
		EffectID: EffectToolBuiltin, CapabilityID: authoredEffectCapabilities[EffectToolBuiltin], Owner: "unknown.Owner", Boundary: BoundaryOutbound,
		OwnerPackage: "unknown", RegistrationSymbol: "newUnknown", GuardCall: "run:guard", EffectCall: "run:effect", Disposition: CommitmentCommitted,
	}
	if err := validateEffectSurfaceManifestAgainstSpecs(manifest, extraMap); !IsEffectCoverageCode(err, "effect_manifest_missing_descriptor") {
		t.Fatalf("extra map row error = %v", err)
	}
	for name, mutate := range map[string]func(runtimeDescriptorSpec) runtimeDescriptorSpec{
		"owner":    func(spec runtimeDescriptorSpec) runtimeDescriptorSpec { spec.Owner = "wrong.Owner"; return spec },
		"boundary": func(spec runtimeDescriptorSpec) runtimeDescriptorSpec { spec.Boundary = BoundaryHTTP; return spec },
		"effect": func(spec runtimeDescriptorSpec) runtimeDescriptorSpec {
			spec.EffectID = EffectToolWebSearch
			return spec
		},
	} {
		t.Run("map "+name+" drift", func(t *testing.T) {
			mutated := cloneRuntimeDescriptorSpecs(runtimeDescriptorSpecs)
			mutated["chat.provider.dispatch"] = mutate(mutated["chat.provider.dispatch"])
			if err := validateEffectSurfaceManifestAgainstSpecs(manifest, mutated); !IsEffectCoverageCode(err, "effect_manifest_descriptor_drift") {
				t.Fatalf("map %s drift error = %v", name, err)
			}
		})
	}

	registry := NewEffectRegistry()
	valid := descriptorForSpec("agent.tool.builtin")
	if err := registry.Register(valid); err != nil {
		t.Fatalf("register exact descriptor: %v", err)
	}
	for name, descriptor := range map[string]EffectDescriptor{
		"prefix wildcard": {ID: "chat.unlisted", CapabilityID: "relay.provider_inference", Boundary: BoundaryOutbound, Owner: "chat.RelayGateway"},
		"owner":           {ID: valid.ID, CapabilityID: valid.CapabilityID, Boundary: valid.Boundary, Owner: "agent.ToolExecutor.other"},
		"boundary":        {ID: valid.ID, CapabilityID: valid.CapabilityID, Boundary: BoundaryHTTP, Owner: valid.Owner},
		"capability":      {ID: valid.ID, CapabilityID: "mcp.custom_execution", Boundary: valid.Boundary, Owner: valid.Owner},
	} {
		t.Run(name, func(t *testing.T) {
			fresh := NewEffectRegistry()
			if err := fresh.Register(descriptor); err == nil {
				t.Fatalf("mismatched descriptor registered: %#v", descriptor)
			}
		})
	}
}

func TestStructuralEffectDiscoveryMutationContract(t *testing.T) {
	markerRoot := t.TempDir()
	executorPath := filepath.Join(markerRoot, "src", "server", "internal", "agent", "executor.go")
	if err := os.MkdirAll(filepath.Dir(executorPath), 0o755); err != nil {
		t.Fatalf("create fixture directory: %v", err)
	}
	fixture := `package agent

// The old marker remains after the registration, guard, and effect were removed.
const retiredDescriptorMarker = "builtin"
`
	if err := os.WriteFile(executorPath, []byte(fixture), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	discovered, err := DiscoverEffectSurfaces(markerRoot)
	if err != nil {
		t.Fatalf("discover mutated fixture: %v", err)
	}
	for _, surface := range discovered {
		if strings.HasPrefix(surface.SeamID, "agent.tool.builtin@") {
			t.Fatalf("marker-only source certified removed effect chain: %#v", surface)
		}
	}

	repoRoot, manifest, _, _ := effectCoverageFixture(t)
	production, err := DiscoverEffectSurfaces(repoRoot)
	if err != nil {
		t.Fatalf("discover production source: %v", err)
	}
	if len(production) == 0 {
		t.Fatal("production discovery returned zero rows")
	}
	if err := joinStatic(manifest.Surfaces, production); err != nil {
		t.Fatalf("production structural join: %v; discovered=%#v", err, production)
	}
	descriptorCounts := make(map[string]int, len(runtimeDescriptorSpecs))
	for _, surface := range production {
		if surface.DescriptorID != "" {
			descriptorCounts[surface.DescriptorID]++
		}
	}
	if len(descriptorCounts) != len(runtimeDescriptorSpecs) {
		t.Fatalf("production descriptor rows=%d want=%d: %#v", len(descriptorCounts), len(runtimeDescriptorSpecs), descriptorCounts)
	}
	for descriptorID := range runtimeDescriptorSpecs {
		if descriptorCounts[descriptorID] != 1 {
			t.Fatalf("production descriptor %s count=%d want=1", descriptorID, descriptorCounts[descriptorID])
		}
	}

	t.Run("extra production seam", func(t *testing.T) {
		extraRoot := t.TempDir()
		extraPackage := filepath.Join(extraRoot, "src", "server", "internal", "extra")
		if err := os.MkdirAll(extraPackage, 0o755); err != nil {
			t.Fatalf("create extra seam fixture: %v", err)
		}
		extraSource := `package extra

import "oblivious/server/internal/releasecontract"

func register(effects releasecontract.EffectRegistrar) error {
	unused := releasecontract.EffectDescriptor{
		ID: "unused.source.effect", CapabilityID: "mcp.tool_execution",
		Boundary: releasecontract.BoundaryOutbound, Owner: "extra.Unused",
	}
	_ = unused
	return effects.Register(releasecontract.EffectDescriptor{
		ID: "unknown.source.effect", CapabilityID: "mcp.tool_execution",
		Boundary: releasecontract.BoundaryOutbound, Owner: "extra.Owner",
	})
}
`
		if err := os.WriteFile(filepath.Join(extraPackage, "extra.go"), []byte(extraSource), 0o644); err != nil {
			t.Fatalf("write extra seam fixture: %v", err)
		}
		extraProduction, err := DiscoverEffectSurfaces(extraRoot)
		if err != nil {
			t.Fatalf("discover extra source seam: %v", err)
		}
		if len(extraProduction) != 1 || extraProduction[0].DescriptorID != "unknown.source.effect" {
			t.Fatalf("registered source inventory = %#v", extraProduction)
		}
		if err := joinStatic(nil, extraProduction); !IsEffectCoverageCode(err, "effect_coverage_extra_static") {
			t.Fatalf("extra source seam error = %v, discovered=%#v", err, extraProduction)
		}
	})

	t.Run("registered descriptor cannot borrow unused correct literal", func(t *testing.T) {
		descriptorID := "worker.schedule.claim"
		spec := runtimeDescriptorSpecs[descriptorID]
		fixtureRoot := t.TempDir()
		copyEffectPackage(t, repoRoot, fixtureRoot, spec.OwnerPackage)
		path := filepath.Join(fixtureRoot, filepath.FromSlash(spec.OwnerPackage), "worker.go")
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read registered-literal fixture: %v", err)
		}
		correct := `{ID: "worker.schedule.claim", CapabilityID: string(claimCapability), Boundary: releasecontract.BoundaryWorkerClaim, Owner: "schedule.Worker"}`
		wrong := `{ID: "worker.schedule.claim", CapabilityID: string(workflowCapability), Boundary: releasecontract.BoundaryWorkerClaim, Owner: "schedule.Worker"}`
		mutated := strings.Replace(string(content), correct, wrong, 1)
		loop := "\tfor _, descriptor := range descriptors {"
		unused := "\t_ = releasecontract.EffectDescriptor{" + correct[1:len(correct)-1] + "}\n" + loop
		mutated = strings.Replace(mutated, loop, unused, 1)
		if mutated == string(content) || !strings.Contains(mutated, wrong) || !strings.Contains(mutated, unused) {
			t.Fatal("registered-literal mutation was not applied")
		}
		if err := os.WriteFile(path, []byte(mutated), 0o644); err != nil {
			t.Fatalf("write registered-literal fixture: %v", err)
		}
		discovered, err := DiscoverEffectSurfaces(fixtureRoot)
		if err != nil {
			t.Fatalf("discover registered-literal fixture: %v", err)
		}
		for _, surface := range discovered {
			if surface.DescriptorID == descriptorID {
				t.Fatalf("wrong registered descriptor borrowed unused correct literal: %#v", surface)
			}
		}
	})

	mutations := []struct {
		name         string
		descriptorID string
		file         string
		old          string
		replacement  string
	}{
		{"worker registration", "worker.schedule.claim", "worker.go", "effects.Register(descriptor)", "effects.RetiredRegister(descriptor)"},
		{"schedule sibling capability binding", "worker.schedule.claim", "worker.go", "CapabilityID: string(claimCapability)", "CapabilityID: string(workflowCapability)"},
		{"channel retry guard", "worker.channel_retry.claim", "service_legacy.go", "s.readiness.requireRetryClaim(ctx)", "s.readiness.retiredRequireRetryClaim(ctx)"},
		{"channel delivery effect", "channel.delivery.send", "service_legacy.go", "deliverer.DeliverOutbound(ctx, req.Config, raw)", "deliverer.RetiredDeliverOutbound(ctx, req.Config, raw)"},
		{"registration owner", "chat.provider.dispatch", "relay_gateway.go", "\"chat.RelayGateway\"", "\"chat.OtherGateway\""},
		{"registration boundary", "mcp.transport.dispatch", "client.go", "Boundary:     releasecontract.BoundaryOutbound", "Boundary:     releasecontract.BoundaryHTTP"},
		{"registration effect", "admin.channel.model.mutation", "service.go", "Resolve(releasecontract.EffectHTTPMutation)", "Resolve(releasecontract.EffectToolBuiltin)"},
		{"relay chat guard", "chat.provider.dispatch", "relay_gateway.go", "g.readiness.requireDispatch(ctx, req.Model)", "g.readiness.retiredRequireDispatch(ctx, req.Model)"},
		{"admin effect", "admin.channel.model.mutation", "channel_service.go", "s.store.TestChannel(ctx, actor.OrganizationID, id)", "s.store.RetiredTestChannel(ctx, actor.OrganizationID, id)"},
		{"marketplace checkout effect", "http.marketplace.checkout", "marketplace_handler.go", "checkoutCreator.CreateCheckoutSession(r.Context(), h.checkoutConfig", "checkoutCreator.RetiredCreateCheckoutSession(r.Context(), h.checkoutConfig"},
		{"billing checkout effect", "http.billing.checkout", "billing_handler.go", "checkoutCreator.CreateCheckoutSession(r.Context(), h.checkoutConfig, checkoutReq)", "checkoutCreator.RetiredCreateCheckoutSession(r.Context(), h.checkoutConfig, checkoutReq)"},
		{"marketplace settlement sibling capability binding", "marketplace.settlement.intent", "settlement.go", "CapabilityID: string(readiness.settlement)", "CapabilityID: string(readiness.payout)"},
		{"marketplace payout effect", "marketplace.payout.dispatch", "settlement.go", "s.payoutProvider.CreatePayout(ctx, request)", "s.payoutProvider.RetiredCreatePayout(ctx, request)"},
		{"mcp transport effect", "mcp.transport.dispatch", "client.go", "c.httpClient.Do(httpReq)", "c.httpClient.RetiredDo(httpReq)"},
		{"mcp builtin effect", "mcp.websearch.builtin", "builtin.go", "t.provider.Search(ctx, strings.TrimSpace(query))", "t.provider.RetiredSearch(ctx, strings.TrimSpace(query))"},
		{"mcp tavily effect", "mcp.websearch.tavily", "web_search_provider.go", "p.client.Do(req)", "p.client.RetiredDo(req)"},
		{"websearch chain receiver guard", "mcp.websearch.chain", "websearch.go", "c.readiness.authorize(ctx)", "c.readiness.retiredAuthorize(ctx)"},
		{"websearch chain receiver effect", "mcp.websearch.chain", "websearch.go", "provider.Search(ctx, query)", "provider.RetiredSearch(ctx, query)"},
		{"websearch provider receiver guard", "mcp.websearch.provider", "websearch.go", "p.readiness.authorize(ctx)", "p.readiness.retiredAuthorize(ctx)"},
		{"websearch provider receiver effect", "mcp.websearch.provider", "websearch.go", "p.provider.Search(ctx, query)", "p.provider.RetiredSearch(ctx, query)"},
		{"tool executor guard", "agent.tool.builtin", "executor.go", "e.authorizeTool(ctx, persistedTool)", "e.retiredAuthorizeTool(ctx, persistedTool)"},
		{"tool executor generated resolve binding", "agent.tool.builtin", "executor.go", "CapabilityBindings.Resolve(effect.id)", "CapabilityBindings.Resolve(releasecontract.EffectAgentToolMCP)"},
		{"independent registry effect", "agent.tool.registry.builtin", "registry.go", "return entry.executor(ctx, args)", "return retiredExecutor(ctx, args)"},
		{"registry generated resolve binding", "agent.tool.registry.builtin", "registry.go", "CapabilityBindings.Resolve(effectID)", "CapabilityBindings.Resolve(releasecontract.EffectAgentToolMCP)"},
		{"independent websearch effect", "agent.tool.web_search", "websearch.go", "provider.Search(ctx, query)", "provider.RetiredSearch(ctx, query)"},
		{"archive sibling capability binding", "worker.archive.write", "archive.go", "CapabilityID: string(write)", "CapabilityID: string(deleteCapability)"},
		{"relay batch sibling capability binding", "worker.relay_batch.claim", "batch_polling_worker.go", "CapabilityID: string(claim)", "CapabilityID: string(finalize)"},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			spec := runtimeDescriptorSpecs[mutation.descriptorID]
			fixtureRoot := t.TempDir()
			copyEffectPackage(t, repoRoot, fixtureRoot, spec.OwnerPackage)
			baseline, err := descriptorStructurePresent(fixtureRoot, mutation.descriptorID, spec)
			if err != nil || !baseline {
				t.Fatalf("baseline descriptor structure present=%v err=%v", baseline, err)
			}
			path := filepath.Join(fixtureRoot, filepath.FromSlash(spec.OwnerPackage), mutation.file)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read mutation target: %v", err)
			}
			if !strings.Contains(string(content), mutation.old) {
				t.Fatalf("mutation target %q missing in %s", mutation.old, path)
			}
			mutated := strings.Replace(string(content), mutation.old, mutation.replacement, 1) + "\n// preserved marker: " + mutation.old + "\n"
			if err := os.WriteFile(path, []byte(mutated), 0o644); err != nil {
				t.Fatalf("write mutation: %v", err)
			}
			present, err := descriptorStructurePresent(fixtureRoot, mutation.descriptorID, spec)
			if err != nil {
				t.Fatalf("discover mutation: %v", err)
			}
			if present {
				t.Fatalf("marker-preserving mutation certified %s", mutation.descriptorID)
			}
		})
	}

	orderRoot := t.TempDir()
	orderPackage := filepath.Join(orderRoot, "src", "server", "internal", "order")
	if err := os.MkdirAll(orderPackage, 0o755); err != nil {
		t.Fatalf("create order fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(orderPackage, "order.go"), []byte("package order\nfunc run() { effect(); guard() }\n"), 0o644); err != nil {
		t.Fatalf("write order fixture: %v", err)
	}
	functions, err := loadSourceFunctions(orderPackage)
	if err != nil {
		t.Fatalf("parse order fixture: %v", err)
	}
	if guardEffectContractPresent(functions, "run:guard", "run:effect") {
		t.Fatal("guard moved after effect was accepted")
	}
}

func TestEffectCoverageRequiresRuntimeAuthoritiesContract(t *testing.T) {
	_, manifest, contract, profile := effectCoverageFixture(t)
	authorities, err := NewRuntimeAuthorities(contract, profile, effectCoverageGuard{})
	if err != nil {
		t.Fatalf("build runtime authorities: %v", err)
	}
	runtimeDescriptors := selectedRuntimeDescriptors()
	modifiedContract := contract
	modifiedContract.ReasonCodes = append([]ReasonCode(nil), contract.ReasonCodes...)
	modifiedContract.ReasonCodes[0].Description += " changed"
	var excludedProfile DeploymentProfile
	for _, candidate := range contract.Profiles {
		if candidate.ID == "dual" {
			excludedProfile = candidate
		}
	}
	cases := []struct {
		name      string
		contract  AuthoredContractV1
		profile   DeploymentProfile
		authority RuntimeAuthorities
		expected  string
	}{
		{"zero authorities", contract, profile, RuntimeAuthorities{}, "effect_coverage_authorities_required"},
		{"empty contract", AuthoredContractV1{}, profile, authorities, "effect_coverage_contract_required"},
		{"missing profile", contract, DeploymentProfile{}, authorities, "effect_coverage_profile_invalid"},
		{"noncommitted profile", contract, excludedProfile, authorities, "effect_coverage_profile_invalid"},
		{"authority contract mismatch", modifiedContract, profile, authorities, "effect_coverage_authorities_mismatch"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := VerifyEffectCoverage(EffectCoverageOptions{Manifest: manifest, Contract: testCase.contract, Profile: testCase.profile, Authorities: testCase.authority, Runtime: runtimeDescriptors})
			if !IsEffectCoverageCode(err, testCase.expected) {
				t.Fatalf("error = %v, want %s", err, testCase.expected)
			}
		})
	}
	missingEffect := authorities
	missingEffect.CapabilityBindings.bindings = cloneEffectBindings(authorities.CapabilityBindings.bindings)
	delete(missingEffect.CapabilityBindings.bindings, EffectAgentToolBuiltin)
	if err := VerifyEffectCoverage(EffectCoverageOptions{Manifest: manifest, Contract: contract, Profile: profile, Authorities: missingEffect, Runtime: runtimeDescriptors}); !IsEffectCoverageCode(err, "effect_coverage_unknown_effect") {
		t.Fatalf("missing effect binding error = %v", err)
	}
	mismatchedEffect := authorities
	mismatchedEffect.CapabilityBindings.bindings = cloneEffectBindings(authorities.CapabilityBindings.bindings)
	mismatchedEffect.CapabilityBindings.bindings[EffectAgentToolBuiltin] = authoredEffectCapabilities[EffectAgentToolCustomAPIHTTP]
	if err := VerifyEffectCoverage(EffectCoverageOptions{Manifest: manifest, Contract: contract, Profile: profile, Authorities: mismatchedEffect, Runtime: runtimeDescriptors}); !IsEffectCoverageCode(err, "effect_coverage_capability_mismatch") {
		t.Fatalf("mismatched effect binding error = %v", err)
	}
}

type effectCoverageGuard struct{}

func (effectCoverageGuard) Require(context.Context, string, Boundary) error { return nil }

func effectCoverageFixture(t *testing.T) (string, EffectSurfaceManifest, AuthoredContractV1, DeploymentProfile) {
	t.Helper()
	_, filename, _, _ := runtime.Caller(0)
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", ".."))
	manifest, err := LoadEffectSurfaceManifest(filepath.Join(repoRoot, "config", "release", "readiness-effect-surface.v1.json"))
	if err != nil {
		t.Fatalf("load effect manifest: %v", err)
	}
	contractContent, err := os.ReadFile(filepath.Join(repoRoot, "config", "release", "contract.v1.json"))
	if err != nil {
		t.Fatalf("read release contract: %v", err)
	}
	var contract AuthoredContractV1
	if err := json.Unmarshal(contractContent, &contract); err != nil {
		t.Fatalf("decode release contract: %v", err)
	}
	var profile DeploymentProfile
	for _, candidate := range contract.Profiles {
		if candidate.ID == "monolith" {
			profile = candidate
			break
		}
	}
	if profile.ID == "" {
		t.Fatal("monolith profile missing from release contract")
	}
	return repoRoot, manifest, contract, profile
}

func TestReadinessEffectCoverageContract(t *testing.T) {
	repoRoot, manifest, contract, profile := effectCoverageFixture(t)
	authorities, err := NewRuntimeAuthorities(contract, profile, effectCoverageGuard{})
	if err != nil {
		t.Fatalf("build runtime authorities: %v", err)
	}
	runtimeDescriptors := selectedRuntimeDescriptors()
	registry := NewEffectRegistry()
	for _, descriptor := range runtimeDescriptors {
		if err := registry.Register(descriptor); err != nil {
			t.Fatalf("register %s: %v", descriptor.ID, err)
		}
	}
	before := registry.Snapshot()
	first := runtimeDescriptors[0]
	if err := registry.Register(first); !IsEffectCoverageCode(err, "effect_registry_duplicate") {
		t.Fatalf("duplicate error = %v", err)
	}
	if err := registry.Register(EffectDescriptor{ID: "chat.caller.effect", CapabilityID: "relay.provider_inference", Boundary: BoundaryOutbound, Owner: "chat.RelayGateway"}); !IsEffectCoverageCode(err, "effect_registry_unknown") {
		t.Fatalf("unknown error = %v", err)
	}
	if len(before) != len(runtimeDescriptors) || len(registry.Snapshot()) != len(runtimeDescriptors) {
		t.Fatalf("registry snapshot mutated unexpectedly: before=%#v after=%#v", before, registry.Snapshot())
	}
	discovered, err := DiscoverEffectSurfaces(repoRoot)
	if err != nil {
		t.Fatalf("discover production effect surfaces: %v", err)
	}
	if err := joinStatic(manifest.Surfaces, discovered); err != nil {
		t.Fatalf("expected/static exact join: %v", err)
	}
	if err := VerifyEffectCoverage(EffectCoverageOptions{RepoRoot: repoRoot, Manifest: manifest, Contract: contract, Profile: profile, Authorities: authorities, Registry: registry}); err != nil {
		t.Fatalf("valid exact join: %v", err)
	}

	missing := withoutDescriptor(runtimeDescriptors, "agent.tool.builtin")
	missingProvider := withoutDescriptor(runtimeDescriptors, "mcp.websearch.provider")
	multipleProvider := append(cloneDescriptors(runtimeDescriptors), descriptorForSpec("mcp.websearch.chain"))
	partialOptional := append(cloneDescriptors(runtimeDescriptors), descriptorForSpec("worker.schedule.claim"))
	extra := append(cloneDescriptors(runtimeDescriptors), EffectDescriptor{ID: "unknown.effect", CapabilityID: "mcp.tool_execution", Boundary: BoundaryOutbound, Owner: "unknown.Owner"})
	duplicate := append(cloneDescriptors(runtimeDescriptors), runtimeDescriptors[0])
	excluded := append(cloneDescriptors(runtimeDescriptors), descriptorForSpec("agent.tool.web_search"))
	ownerDrift := cloneDescriptors(runtimeDescriptors)
	ownerDrift[0].Owner = "wrong.Owner"
	capabilityDrift := cloneDescriptors(runtimeDescriptors)
	capabilityDrift[0].CapabilityID = "mcp.custom_execution"
	cases := []struct {
		name     string
		runtime  []EffectDescriptor
		expected string
	}{
		{"zero runtime", nil, "effect_coverage_zero_runtime"},
		{"missing", missing, "effect_coverage_missing_runtime"},
		{"extra", extra, "effect_coverage_extra_runtime"},
		{"duplicate", duplicate, "effect_coverage_duplicate_runtime"},
		{"excluded independent", excluded, "effect_coverage_profile_drift"},
		{"owner drift", ownerDrift, "effect_coverage_runtime_drift"},
		{"capability drift", capabilityDrift, "effect_coverage_runtime_drift"},
		{"missing provider selection", missingProvider, "effect_coverage_selection_drift"},
		{"multiple provider selection", multipleProvider, "effect_coverage_selection_drift"},
		{"partial optional cohort", partialOptional, "effect_coverage_selection_drift"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := VerifyEffectCoverage(EffectCoverageOptions{Manifest: manifest, Contract: contract, Profile: profile, Authorities: authorities, Runtime: testCase.runtime})
			if !IsEffectCoverageCode(err, testCase.expected) {
				t.Fatalf("error = %v, want %s", err, testCase.expected)
			}
		})
	}
}

func descriptorForSpec(descriptorID string) EffectDescriptor {
	spec := runtimeDescriptorSpecs[descriptorID]
	return EffectDescriptor{ID: descriptorID, CapabilityID: string(spec.CapabilityID), Boundary: spec.Boundary, Owner: spec.Owner}
}

func cloneRuntimeDescriptorSpecs(source map[string]runtimeDescriptorSpec) map[string]runtimeDescriptorSpec {
	result := make(map[string]runtimeDescriptorSpec, len(source))
	for descriptorID, spec := range source {
		result[descriptorID] = spec
	}
	return result
}

func selectedRuntimeDescriptors() []EffectDescriptor {
	ids := make([]string, 0, len(runtimeDescriptorSpecs))
	for descriptorID := range runtimeDescriptorSpecs {
		ids = append(ids, descriptorID)
	}
	sort.Strings(ids)
	descriptors := make([]EffectDescriptor, 0, len(ids))
	for _, descriptorID := range ids {
		spec := runtimeDescriptorSpecs[descriptorID]
		if spec.Disposition == CommitmentExcluded {
			continue
		}
		if spec.Configuration != nil {
			if spec.Configuration.Mode == effectSelectorOptional {
				continue
			}
			if spec.Configuration.Mode == effectSelectorOneOf && spec.Configuration.Value != "provider" {
				continue
			}
		}
		descriptors = append(descriptors, descriptorForSpec(descriptorID))
	}
	return descriptors
}

func cloneDescriptors(source []EffectDescriptor) []EffectDescriptor {
	return append([]EffectDescriptor(nil), source...)
}

func withoutDescriptor(source []EffectDescriptor, descriptorID string) []EffectDescriptor {
	result := make([]EffectDescriptor, 0, len(source))
	for _, descriptor := range source {
		if descriptor.ID != descriptorID {
			result = append(result, descriptor)
		}
	}
	return result
}

func copyEffectPackage(t *testing.T, sourceRoot, destinationRoot, packagePath string) {
	t.Helper()
	sourceDirectory := filepath.Join(sourceRoot, filepath.FromSlash(packagePath))
	destinationDirectory := filepath.Join(destinationRoot, filepath.FromSlash(packagePath))
	if err := os.MkdirAll(destinationDirectory, 0o755); err != nil {
		t.Fatalf("create package fixture: %v", err)
	}
	entries, err := os.ReadDir(sourceDirectory)
	if err != nil {
		t.Fatalf("read source package: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(sourceDirectory, entry.Name()))
		if err != nil {
			t.Fatalf("read source file %s: %v", entry.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(destinationDirectory, entry.Name()), content, 0o644); err != nil {
			t.Fatalf("write fixture file %s: %v", entry.Name(), err)
		}
	}
	catalogSource := filepath.Join(sourceRoot, "src", "server", "internal", "releasecontract", "catalog.go")
	catalogDestination := filepath.Join(destinationRoot, "src", "server", "internal", "releasecontract", "catalog.go")
	if err := os.MkdirAll(filepath.Dir(catalogDestination), 0o755); err != nil {
		t.Fatalf("create catalog fixture: %v", err)
	}
	catalog, err := os.ReadFile(catalogSource)
	if err != nil {
		t.Fatalf("read catalog fixture: %v", err)
	}
	if err := os.WriteFile(catalogDestination, catalog, 0o644); err != nil {
		t.Fatalf("write catalog fixture: %v", err)
	}
}
