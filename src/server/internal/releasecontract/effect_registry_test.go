package releasecontract

import (
	"context"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
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

	t.Run("package string shadow rejects conversions", func(t *testing.T) {
		for _, testCase := range []struct {
			name   string
			source string
		}{
			{
				name: "descriptor id",
				source: `package packagestringshadow
const descriptorID = "shadow.descriptor.id"
func register(options Options) error {
	return options.Effects.Register(EffectDescriptor{
		ID: string(descriptorID), CapabilityID: "mcp.tool_execution",
		Boundary: BoundaryOutbound, Owner: "shadow.DescriptorID",
	})
}
`,
			},
			{
				name: "capability id",
				source: `package packagestringshadow
func register(options Options) error {
	capability, _ := options.Authorities.CapabilityBindings.Resolve(EffectAgentToolBuiltin)
	return options.Effects.Register(EffectDescriptor{
		ID: "shadow.capability.id", CapabilityID: string(capability),
		Boundary: BoundaryOutbound, Owner: "shadow.CapabilityID",
	})
}
`,
			},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				root := t.TempDir()
				packageDirectory := filepath.Join(root, "src", "server", "internal", "packagestringshadow")
				if err := os.MkdirAll(packageDirectory, 0o755); err != nil {
					t.Fatalf("create package-string-shadow fixture: %v", err)
				}
				if err := os.WriteFile(filepath.Join(packageDirectory, "registration.go"), []byte(testCase.source), 0o644); err != nil {
					t.Fatalf("write package-string-shadow registration: %v", err)
				}
				shadow := "package packagestringshadow\nimport \"fmt\"\nvar string = fmt.Sprintln\n"
				if err := os.WriteFile(filepath.Join(packageDirectory, "shadow.go"), []byte(shadow), 0o644); err != nil {
					t.Fatalf("write package string shadow: %v", err)
				}
				registrations, err := discoverPackageRegistrations(root, packageDirectory, map[string]string{
					"EffectAgentToolBuiltin": string(EffectAgentToolBuiltin),
				})
				if err != nil {
					t.Fatalf("discover package-string-shadow fixture: %v", err)
				}
				if len(registrations) != 0 {
					t.Fatalf("package string shadow certified %s conversion: %#v", testCase.name, registrations)
				}
			})
		}
	})

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
		{"tool executor wrong capability resolver receiver", "agent.tool.builtin", "executor.go", "options.Authorities.CapabilityBindings.Resolve(effect.id)", "fake.Resolve(effect.id)"},
		{"schedule wrong registrar receiver", "worker.schedule.claim", "worker.go", "effects.Register(descriptor)", "other.Register(descriptor)"},
		{"tool executor generated resolve binding", "agent.tool.builtin", "executor.go", "CapabilityBindings.Resolve(effect.id)", "CapabilityBindings.Resolve(releasecontract.EffectAgentToolMCP)"},
		{"tool executor nested capability wrapper", "agent.tool.builtin", "executor.go", "CapabilityID: string(capability)", "CapabilityID: string(fake(capability))"},
		{"tool executor pointer helper capability wrapper", "agent.tool.builtin", "executor.go", "CapabilityID: string(capability)", "CapabilityID: string(pointer(capability))"},
		{"tool executor wrong capability fallback", "agent.tool.builtin", "executor.go", "CapabilityID: string(capability)", "CapabilityID: fake(capability)"},
		{"independent registry effect", "agent.tool.registry.builtin", "registry.go", "return entry.executor(ctx, args)", "return retiredExecutor(ctx, args)"},
		{"registry generated resolve binding", "agent.tool.registry.builtin", "registry.go", "CapabilityBindings.Resolve(effectID)", "CapabilityBindings.Resolve(releasecontract.EffectAgentToolMCP)"},
		{"independent websearch effect", "agent.tool.web_search", "websearch.go", "provider.Search(ctx, query)", "provider.RetiredSearch(ctx, query)"},
		{"archive sibling capability binding", "worker.archive.write", "archive.go", "CapabilityID: string(write)", "CapabilityID: string(deleteCapability)"},
		{"relay batch sibling capability binding", "worker.relay_batch.claim", "batch_polling_worker.go", "CapabilityID: string(claim)", "CapabilityID: string(finalize)"},
		{"billing helper wrong receiver", "http.billing.checkout", "billing_handler.go", "return newCheckoutFinancialReadiness(financial, \"http.billing.checkout\", \"http.billingHandler\")", "return fake.newCheckoutFinancialReadiness(financial, \"http.billing.checkout\", \"http.billingHandler\")"},
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

	t.Run("ranged registration row is immutable", func(t *testing.T) {
		cases := []struct {
			name         string
			descriptorID string
			file         string
			old          string
			injection    string
		}{
			{"schedule descriptor id write", "worker.schedule.claim", "worker.go", "if err := effects.Register(descriptor); err != nil {", `descriptor.ID = "worker.schedule.workflow.start"`},
			{"schedule capability id write", "worker.schedule.claim", "worker.go", "if err := effects.Register(descriptor); err != nil {", `descriptor.CapabilityID = "wrong"`},
			{"schedule owner write", "worker.schedule.claim", "worker.go", "if err := effects.Register(descriptor); err != nil {", `descriptor.Owner = "wrong.Owner"`},
			{"schedule boundary write", "worker.schedule.claim", "worker.go", "if err := effects.Register(descriptor); err != nil {", `descriptor.Boundary = releasecontract.BoundaryOutbound`},
			{"schedule whole row reassignment", "worker.schedule.claim", "worker.go", "if err := effects.Register(descriptor); err != nil {", `descriptor = descriptors[1]`},
			{"schedule address escape", "worker.schedule.claim", "worker.go", "if err := effects.Register(descriptor); err != nil {", `mutateDescriptor(&descriptor)`},
			{"schedule method escape", "worker.schedule.claim", "worker.go", "if err := effects.Register(descriptor); err != nil {", `descriptor.Mutate()`},
			{"schedule invoked closure id write", "worker.schedule.claim", "worker.go", "if err := effects.Register(descriptor); err != nil {", `mutate := func() { descriptor.ID = "worker.schedule.workflow.start" }; mutate()`},
			{"schedule invoked closure address escape", "worker.schedule.claim", "worker.go", "if err := effects.Register(descriptor); err != nil {", `mutate := func() { mutateDescriptor(&descriptor) }; mutate()`},
			{"schedule invoked closure method escape", "worker.schedule.claim", "worker.go", "if err := effects.Register(descriptor); err != nil {", `mutate := func() { descriptor.Mutate() }; mutate()`},
			{"generated row reassignment", "agent.tool.builtin", "executor.go", "if err := options.Effects.Register(descriptor); err != nil {", `descriptor = descriptors[len(descriptors)-1]`},
			{"generated row method escape", "agent.tool.builtin", "executor.go", "if err := options.Effects.Register(descriptor); err != nil {", `descriptor.Mutate()`},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				spec := runtimeDescriptorSpecs[testCase.descriptorID]
				fixtureRoot := t.TempDir()
				copyEffectPackage(t, repoRoot, fixtureRoot, spec.OwnerPackage)
				baseline, err := descriptorStructurePresent(fixtureRoot, testCase.descriptorID, spec)
				if err != nil || !baseline {
					t.Fatalf("baseline ranged descriptor present=%v err=%v", baseline, err)
				}
				path := filepath.Join(fixtureRoot, filepath.FromSlash(spec.OwnerPackage), testCase.file)
				content, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read ranged-registration fixture: %v", err)
				}
				if !strings.Contains(string(content), testCase.old) {
					t.Fatalf("ranged-registration target missing: %q", testCase.old)
				}
				mutated := strings.Replace(string(content), testCase.old, testCase.injection+"\n\t\t"+testCase.old, 1) + "\n// preserved ranged registration: " + testCase.old + "\n"
				if err := os.WriteFile(path, []byte(mutated), 0o644); err != nil {
					t.Fatalf("write ranged-registration mutation: %v", err)
				}
				present, err := descriptorStructurePresent(fixtureRoot, testCase.descriptorID, spec)
				if err != nil {
					t.Fatalf("discover ranged-registration mutation: %v", err)
				}
				if present {
					t.Fatalf("mutable ranged registration certified %s", testCase.descriptorID)
				}
			})
		}
	})

	t.Run("statically unreachable registration and helper calls do not count", func(t *testing.T) {
		cases := []struct {
			name         string
			descriptorID string
			file         string
			old          string
			replacement  string
		}{
			{
				name:         "if false range",
				descriptorID: "worker.schedule.claim",
				file:         "worker.go",
				old: "\tfor _, descriptor := range descriptors {\n" +
					"\t\tif err := effects.Register(descriptor); err != nil {\n" +
					"\t\t\treturn nil, err\n" +
					"\t\t}\n" +
					"\t}",
				replacement: "\tif false {\n\t\tfor _, descriptor := range descriptors {\n" +
					"\t\tif err := effects.Register(descriptor); err != nil {\n" +
					"\t\t\treturn nil, err\n" +
					"\t\t}\n\t\t}\n\t}",
			},
			{
				name:         "if false register",
				descriptorID: "worker.schedule.claim",
				file:         "worker.go",
				old: "\t\tif err := effects.Register(descriptor); err != nil {\n" +
					"\t\t\treturn nil, err\n" +
					"\t\t}",
				replacement: "\t\tif false {\n\t\t\tif err := effects.Register(descriptor); err != nil {\n" +
					"\t\t\t\treturn nil, err\n" +
					"\t\t\t}\n\t\t}",
			},
			{
				name:         "if false helper call",
				descriptorID: "http.billing.checkout",
				file:         "billing_handler.go",
				old:          "\treturn newCheckoutFinancialReadiness(financial, \"http.billing.checkout\", \"http.billingHandler\")",
				replacement:  "\tif false { _, _ = newCheckoutFinancialReadiness(financial, \"http.billing.checkout\", \"http.billingHandler\") }; return nil, nil",
			},
		}
		for _, mutation := range cases {
			t.Run(mutation.name, func(t *testing.T) {
				spec := runtimeDescriptorSpecs[mutation.descriptorID]
				fixtureRoot := t.TempDir()
				copyEffectPackage(t, repoRoot, fixtureRoot, spec.OwnerPackage)
				path := filepath.Join(fixtureRoot, filepath.FromSlash(spec.OwnerPackage), mutation.file)
				content, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read unreachable mutation target: %v", err)
				}
				if !strings.Contains(string(content), mutation.old) {
					t.Fatalf("unreachable mutation target missing: %q", mutation.old)
				}
				mutated := strings.Replace(string(content), mutation.old, mutation.replacement, 1) + "\n/* preserved unreachable marker:\n" + mutation.old + "\n*/\n"
				if err := os.WriteFile(path, []byte(mutated), 0o644); err != nil {
					t.Fatalf("write unreachable mutation: %v", err)
				}
				present, err := descriptorStructurePresent(fixtureRoot, mutation.descriptorID, spec)
				if err != nil {
					t.Fatalf("discover unreachable mutation: %v", err)
				}
				if present {
					t.Fatalf("statically unreachable evidence certified %s", mutation.descriptorID)
				}
			})
		}
	})

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

	t.Run("exact call paths reject dead and wrong receiver evidence", func(t *testing.T) {
		cases := []struct {
			name   string
			source string
			want   bool
		}{
			{
				name: "live exact receivers",
				source: `package exact
type readinessGate struct{}
func (readinessGate) require() error { return nil }
type effectStore struct{}
func (effectStore) Effect() {}
type Runner struct { readiness readinessGate; store effectStore }
func (r *Runner) run() { if err := r.readiness.require(); err != nil { return }; r.store.Effect() }
`,
				want: true,
			},
			{
				name: "statically false guard",
				source: `package exact
type readinessGate struct{}
func (readinessGate) require() {}
type effectStore struct{}
func (effectStore) Effect() {}
type Runner struct { readiness readinessGate; store effectStore }
func (r *Runner) run() { if false { r.readiness.require() }; r.store.Effect() }
`,
				want: false,
			},
			{
				name: "wrong guard receiver",
				source: `package exact
type readinessGate struct{}
func (readinessGate) require() {}
type effectStore struct{}
func (effectStore) Effect() {}
type Runner struct { readiness readinessGate; store effectStore }
func (r *Runner) run() { other.readiness.require(); r.store.Effect() }
`,
				want: false,
			},
			{
				name: "wrong effect receiver",
				source: `package exact
type readinessGate struct{}
func (readinessGate) require() {}
type effectStore struct{}
func (effectStore) Effect() {}
type Runner struct { readiness readinessGate; store effectStore }
func (r *Runner) run() { r.readiness.require(); other.store.Effect() }
`,
				want: false,
			},
			{
				name: "shadowed receiver object",
				source: `package exact
type readinessGate struct{}
func (readinessGate) require() {}
type effectStore struct{}
func (effectStore) Effect() {}
type Runner struct { readiness readinessGate; store effectStore }
func (r *Runner) run() { if true { r := &Runner{}; r.readiness.require() }; r.store.Effect() }
`,
				want: false,
			},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				directory := t.TempDir()
				if err := os.WriteFile(filepath.Join(directory, "exact.go"), []byte(testCase.source+"\n// preserved markers: r.readiness.require r.store.Effect\n"), 0o644); err != nil {
					t.Fatalf("write exact-call fixture: %v", err)
				}
				functions, err := loadSourceFunctions(directory)
				if err != nil {
					t.Fatalf("parse exact-call fixture: %v", err)
				}
				got := guardEffectContractPresent(functions, "Runner.run:r.readiness.require", "Runner.run:r.store.Effect")
				if got != testCase.want {
					t.Fatalf("exact call proof=%v want=%v", got, testCase.want)
				}
			})
		}
	})

	t.Run("guard dominance requires checked denial control", func(t *testing.T) {
		cases := []struct {
			name           string
			source         string
			guardContract  string
			effectContract string
			want           bool
		}{
			{
				name: "checked return",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { if err := guard(); err != nil { return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: true,
			},
			{
				name: "optional readiness checked return",
				source: `package dominance
type readiness struct{}
func (*readiness) guard() error { return nil }
func effect() {}
func run(r *readiness) { if r != nil { if err := r.guard(); err != nil { return } }; effect() }
`,
				guardContract: "run:r.guard", effectContract: "run:effect", want: true,
			},
			{
				name: "checked continue",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run(rows []string) { for range rows { if err := guard(); err != nil { continue }; effect() } }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: true,
			},
			{
				name: "checked break",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run(rows []string) { for range rows { if err := guard(); err != nil { break }; effect() } }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: true,
			},
			{
				name: "goto effect label",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { if err := guard(); err != nil { goto allowed }; return; allowed: effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "goto denial exit rejected conservatively",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { if err := guard(); err != nil { goto denied }; effect(); return; denied: return }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "runtime conditional bypass",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run(enabled bool) { if enabled { if err := guard(); err != nil { return } }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "ignored guard result",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { _ = guard(); effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "unchecked guard result",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { err := guard(); _ = err; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "denial falls through",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { if err := guard(); err != nil { _ = err }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "effect reachable on denial",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { if err := guard(); err != nil { effect(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "denial invoked closure contains effect",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { denialEffect := func() { effect() }; if err := guard(); err != nil { denialEffect(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "invoked guard closure dominates effect",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { guarded := func() error { return guard() }; if err := guarded(); err != nil { return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: true,
			},
			{
				name: "nested invoked denial closure contains effect",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { denial := func() { nested := func() { effect() }; nested() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "three level invoked denial closure contains effect",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { denial := func() { middle := func() { nested := func() { effect() }; nested() }; middle() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "captured denial helper from enclosing closure contains effect",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { denial := func() { effectFn := func() { effect() }; middle := func() { effectFn() }; middle() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "nearest enclosing shadowed denial helper contains effect",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { effectFn := func() {}; denial := func() { middle := func() { effectFn := func() { effect() }; nested := func() { effectFn() }; nested() }; middle() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "captured helper rebound in dynamic caller is ambiguous",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { effectFn := func() {}; middle := func() { effectFn() }; denial := func() { effectFn = func() { effect() }; middle() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "captured helper without dynamic rewrite remains safe",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { effectFn := func() {}; middle := func() { effectFn() }; denial := func() { middle() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: true,
			},
			{
				name: "different object shadow does not poison captured helper",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { effectFn := func() {}; middle := func() { effectFn() }; denial := func() { effectFn := func() { effect() }; middle() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: true,
			},
			{
				name: "captured helper range assignment is ambiguous",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { effectFn := func() {}; middle := func() { effectFn() }; denial := func() { for _, effectFn = range []func(){func() { effect() }} {}; middle() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "captured helper address escape is ambiguous",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { effectFn := func() {}; middle := func() { effectFn() }; denial := func() { pointer := &effectFn; _ = pointer; middle() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "invoked helper rebinding captured helper is ambiguous",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { effectFn := func() {}; middle := func() { effectFn() }; denial := func() { rewrite := func() { effectFn = func() { effect() } }; rewrite(); middle() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "outer lexical helper rebinding captured helper is ambiguous",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { effectFn := func() {}; rewrite := func() { effectFn = func() { effect() } }; middle := func() { effectFn() }; denial := func() { rewrite(); middle() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "callable alias rebinding captured helper is ambiguous",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { effectFn := func() {}; rewrite := func() { effectFn = func() { effect() } }; middle := func() { effectFn() }; denial := func() { alias := rewrite; alias(); middle() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "conditional callable binding before captured helper is ambiguous",
				source: `package dominance
func guard() error { return nil }
func condition() bool { return false }
func effect() {}
func run() { effectFn := func() {}; rewrite := func() { effectFn = func() { effect() } }; middle := func() { effectFn() }; denial := func() { alias := rewrite; if condition() { alias = func() {} }; alias(); middle() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "reassigned unproven local callable before captured helper is ambiguous",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run(unknown func()) { effectFn := func() {}; rewrite := func() { effectFn = func() { effect() } }; middle := func() { effectFn() }; denial := func() { alias := rewrite; alias = unknown; alias(); middle() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "same frame deferred writer runs after captured helper",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { effectFn := func() {}; middle := func() { effectFn() }; denial := func() { rewrite := func() { effectFn = func() { effect() } }; defer rewrite(); middle() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: true,
			},
			{
				name: "invoked helper deferred writer runs before return",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { effectFn := func() {}; middle := func() { effectFn() }; denial := func() { rewrite := func() { effectFn = func() { effect() } }; helper := func() { defer rewrite() }; helper(); middle() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "goroutine writer before captured helper is ambiguous",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { effectFn := func() {}; middle := func() { effectFn() }; denial := func() { rewrite := func() { effectFn = func() { effect() } }; go rewrite(); middle() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "immediate literal writer before captured helper is ambiguous",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { effectFn := func() {}; middle := func() { effectFn() }; denial := func() { func() { effectFn = func() { effect() } }(); middle() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "opaque callback capturing writer is ambiguous",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func invoke(callback func()) { callback() }
func run() { effectFn := func() {}; middle := func() { effectFn() }; denial := func() { invoke(func() { effectFn = func() { effect() } }); middle() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "opaque callback alias capturing writer is ambiguous",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func invoke(callback func()) { callback() }
func run() { effectFn := func() {}; middle := func() { effectFn() }; denial := func() { writer := func() { effectFn = func() { effect() } }; invoke(writer); middle() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: false,
			},
			{
				name: "opaque read only callback preserves captured helper",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func invoke(callback func()) { callback() }
func run() { effectFn := func() {}; middle := func() { effectFn() }; denial := func() { invoke(func() { _ = effectFn }); middle() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: true,
			},
			{
				name: "unpassed writer callback preserves captured helper",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { effectFn := func() {}; middle := func() { effectFn() }; denial := func() { writer := func() { effectFn = func() { effect() } }; _ = writer; middle() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: true,
			},
			{
				name: "range declaration shadow preserves captured helper",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { effectFn := func() {}; middle := func() { effectFn() }; denial := func() { for _, effectFn := range []func(){func() { effect() }} { _ = effectFn }; middle() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: true,
			},
			{
				name: "captured helper write after call remains safe",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { effectFn := func() {}; middle := func() { effectFn() }; denial := func() { middle(); effectFn = func() { effect() } }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: true,
			},
			{
				name: "uninvoked helper rebinding captured helper remains safe",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { effectFn := func() {}; middle := func() { effectFn() }; denial := func() { rewrite := func() { effectFn = func() { effect() } }; _ = rewrite; middle() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: true,
			},
			{
				name: "invoked read only helper preserves captured helper",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { effectFn := func() {}; middle := func() { effectFn() }; denial := func() { read := func() { _ = effectFn }; read(); middle() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: true,
			},
			{
				name: "self referenced denial closure terminates expansion",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { var denial func(); denial = func() { denial() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: true,
			},
			{
				name: "cross scope self referenced denial closure terminates expansion",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { denial := func() { var self func(); self = func() { self() }; self() }; if err := guard(); err != nil { denial(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: true,
			},
			{
				name: "cross scope mutual denial closures terminate expansion",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { var outer func(); outer = func() { var inner func(); inner = func() { outer() }; inner() }; if err := guard(); err != nil { outer(); return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: true,
			},
			{
				name: "three level invoked guard closure dominates effect",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { guarded := func() error { middle := func() error { nested := func() error { return guard() }; return nested() }; return middle() }; if err := guarded(); err != nil { return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: true,
			},
			{
				name: "cross enclosing three level guard closure dominates effect",
				source: `package dominance
func guard() error { return nil }
func effect() {}
func run() { guarded := func() error { nested := func() error { return guard() }; middle := func() error { return nested() }; return middle() }; if err := guarded(); err != nil { return }; effect() }
`,
				guardContract: "run:guard", effectContract: "run:effect", want: true,
			},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				directory := t.TempDir()
				if err := os.WriteFile(filepath.Join(directory, "dominance.go"), []byte(testCase.source), 0o644); err != nil {
					t.Fatalf("write guard-dominance fixture: %v", err)
				}
				functions, err := loadSourceFunctions(directory)
				if err != nil {
					t.Fatalf("parse guard-dominance fixture: %v", err)
				}
				got := guardEffectContractPresent(functions, testCase.guardContract, testCase.effectContract)
				if got != testCase.want {
					t.Fatalf("guard dominance proof=%v want=%v", got, testCase.want)
				}
			})
		}
	})

	t.Run("reachable walker prunes dead closures and branches", func(t *testing.T) {
		cases := []struct {
			name   string
			source string
			want   []string
		}{
			{
				name: "uninvoked closure",
				source: `package reachable
func run() { _ = func() { guard(); register(); effect() }; live() }
`,
				want: []string{"live"},
			},
			{
				name: "direct invoked closure",
				source: `package reachable
func run() { func() { guard() }() }
`,
				want: []string{"guard"},
			},
			{
				name: "if true dead else",
				source: `package reachable
func run() { if true { live() } else { guard(); register(); effect() } }
`,
				want: []string{"live"},
			},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				file, err := parser.ParseFile(token.NewFileSet(), "reachable.go", testCase.source, 0)
				if err != nil {
					t.Fatalf("parse reachable fixture: %v", err)
				}
				function := file.Decls[0].(*ast.FuncDecl)
				var got []string
				inspectReachableCalls(function.Body, func(call *ast.CallExpr) {
					if name := calledName(call.Fun); name != "" {
						got = append(got, name)
					}
				})
				sort.Strings(got)
				if strings.Join(got, ",") != strings.Join(testCase.want, ",") {
					t.Fatalf("reachable calls=%v want=%v", got, testCase.want)
				}
			})
		}
	})

	t.Run("uninvoked closure registration is not reachable", func(t *testing.T) {
		root := t.TempDir()
		packageDirectory := filepath.Join(root, "src", "server", "internal", "deadclosure")
		if err := os.MkdirAll(packageDirectory, 0o755); err != nil {
			t.Fatalf("create dead-closure fixture: %v", err)
		}
		source := `package deadclosure
func register(effects Registrar) error {
	deferred := func() error {
		return effects.Register(EffectDescriptor{ID: "dead.closure", CapabilityID: "mcp.tool_execution", Boundary: BoundaryOutbound, Owner: "dead.Closure"})
	}
	_ = deferred
	return nil
}
`
		if err := os.WriteFile(filepath.Join(packageDirectory, "dead.go"), []byte(source), 0o644); err != nil {
			t.Fatalf("write dead-closure fixture: %v", err)
		}
		discovered, err := DiscoverEffectSurfaces(root)
		if err != nil {
			t.Fatalf("discover dead-closure fixture: %v", err)
		}
		if len(discovered) != 0 {
			t.Fatalf("uninvoked closure registration was discovered: %#v", discovered)
		}
	})

	t.Run("descriptor provenance must precede consumption", func(t *testing.T) {
		cases := []struct {
			name   string
			source string
		}{
			{
				name: "assignment after register",
				source: `package late
func register(effects Registrar) error {
	var descriptor EffectDescriptor
	err := effects.Register(descriptor)
	descriptor = EffectDescriptor{ID: "late.assignment", CapabilityID: "mcp.tool_execution", Boundary: BoundaryOutbound, Owner: "late.Assignment"}
	return err
}
`,
			},
			{
				name: "append after range",
				source: `package late
func register(effects Registrar) error {
	var descriptors []EffectDescriptor
	for _, descriptor := range descriptors { _ = effects.Register(descriptor) }
	descriptors = append(descriptors, EffectDescriptor{ID: "late.append", CapabilityID: "mcp.tool_execution", Boundary: BoundaryOutbound, Owner: "late.Append"})
	return nil
}
`,
			},
		}
		for _, testCase := range cases {
			t.Run(testCase.name, func(t *testing.T) {
				root := t.TempDir()
				packageDirectory := filepath.Join(root, "src", "server", "internal", "late")
				if err := os.MkdirAll(packageDirectory, 0o755); err != nil {
					t.Fatalf("create late-provenance fixture: %v", err)
				}
				if err := os.WriteFile(filepath.Join(packageDirectory, "late.go"), []byte(testCase.source), 0o644); err != nil {
					t.Fatalf("write late-provenance fixture: %v", err)
				}
				discovered, err := DiscoverEffectSurfaces(root)
				if err != nil {
					t.Fatalf("discover late-provenance fixture: %v", err)
				}
				if len(discovered) != 0 {
					t.Fatalf("post-consumption provenance was accepted: %#v", discovered)
				}
			})
		}

		discoverFixture := func(t *testing.T, packageName, source string) []EffectSurface {
			t.Helper()
			root := t.TempDir()
			packageDirectory := filepath.Join(root, "src", "server", "internal", packageName)
			if err := os.MkdirAll(packageDirectory, 0o755); err != nil {
				t.Fatalf("create %s provenance fixture: %v", packageName, err)
			}
			if err := os.WriteFile(filepath.Join(packageDirectory, packageName+".go"), []byte(source), 0o644); err != nil {
				t.Fatalf("write %s provenance fixture: %v", packageName, err)
			}
			discovered, err := DiscoverEffectSurfaces(root)
			if err != nil {
				t.Fatalf("discover %s provenance fixture: %v", packageName, err)
			}
			return discovered
		}

		t.Run("scalar shadow cannot replace outer provenance", func(t *testing.T) {
			discovered := discoverFixture(t, "scalarshadow", `package scalarshadow
func register(effects Registrar) error {
	descriptor := EffectDescriptor{ID: "scope.scalar", CapabilityID: "wrong", Boundary: BoundaryOutbound, Owner: "scope.Scalar"}
	{
		descriptor := EffectDescriptor{ID: "scope.scalar", CapabilityID: "correct", Boundary: BoundaryOutbound, Owner: "scope.Scalar"}
		_ = descriptor
	}
	return effects.Register(descriptor)
}
`)
			if len(discovered) != 1 || discovered[0].CapabilityID != "wrong" {
				t.Fatalf("outer scalar borrowed shadow provenance: %#v", discovered)
			}
		})

		t.Run("collection shadow cannot replace outer provenance", func(t *testing.T) {
			discovered := discoverFixture(t, "collectionshadow", `package collectionshadow
func register(effects Registrar) error {
	descriptors := []EffectDescriptor{{ID: "scope.collection", CapabilityID: "wrong", Boundary: BoundaryOutbound, Owner: "scope.Collection"}}
	{
		descriptors := []EffectDescriptor{{ID: "scope.collection", CapabilityID: "correct", Boundary: BoundaryOutbound, Owner: "scope.Collection"}}
		_ = descriptors
	}
	for _, descriptor := range descriptors { _ = effects.Register(descriptor) }
	return nil
}
`)
			if len(discovered) != 1 || discovered[0].CapabilityID != "wrong" {
				t.Fatalf("outer collection borrowed shadow provenance: %#v", discovered)
			}
		})

		for _, testCase := range []struct {
			name        string
			packageName string
			source      string
		}{
			{
				name:        "unknown branch scalar write",
				packageName: "branchscalar",
				source: `package branchscalar
func register(effects Registrar, enabled bool) error {
	descriptor := EffectDescriptor{ID: "branch.scalar", CapabilityID: "wrong", Boundary: BoundaryOutbound, Owner: "branch.Scalar"}
	if enabled {
		descriptor = EffectDescriptor{ID: "branch.scalar", CapabilityID: "correct", Boundary: BoundaryOutbound, Owner: "branch.Scalar"}
	}
	return effects.Register(descriptor)
}
`,
			},
			{
				name:        "unknown branch collection reset",
				packageName: "branchreset",
				source: `package branchreset
func register(effects Registrar, enabled bool) error {
	descriptors := []EffectDescriptor{{ID: "branch.reset", CapabilityID: "correct", Boundary: BoundaryOutbound, Owner: "branch.Reset"}}
	if enabled {
		descriptors = nil
	} else {
		descriptors = append(descriptors, EffectDescriptor{ID: "branch.reset", CapabilityID: "correct", Boundary: BoundaryOutbound, Owner: "branch.Reset"})
	}
	for _, descriptor := range descriptors { _ = effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "unknown branch collection append",
				packageName: "branchappend",
				source: `package branchappend
func register(effects Registrar, enabled bool) error {
	var descriptors []EffectDescriptor
	if enabled {
		descriptors = append(descriptors, EffectDescriptor{ID: "branch.append", CapabilityID: "correct", Boundary: BoundaryOutbound, Owner: "branch.Append"})
	}
	for _, descriptor := range descriptors { _ = effects.Register(descriptor) }
	return nil
}
`,
			},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				discovered := discoverFixture(t, testCase.packageName, testCase.source)
				if len(discovered) != 0 {
					t.Fatalf("ambiguous conditional provenance was accepted: %#v", discovered)
				}
			})
		}

		for _, testCase := range []struct {
			name        string
			packageName string
			source      string
		}{
			{
				name:        "unknown range append",
				packageName: "rangeappend",
				source: `package rangeappend
func register(effects Registrar, values []int) error {
	var descriptors []EffectDescriptor
	for range values {
		descriptors = append(descriptors, EffectDescriptor{ID: "range.append", CapabilityID: "correct", Boundary: BoundaryOutbound, Owner: "range.Append"})
	}
	for _, descriptor := range descriptors { _ = effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "unknown range reset and append",
				packageName: "rangereset",
				source: `package rangereset
func register(effects Registrar, values []int) error {
	descriptors := []EffectDescriptor{{ID: "range.reset", CapabilityID: "correct", Boundary: BoundaryOutbound, Owner: "range.Reset"}}
	for range values {
		descriptors = nil
		descriptors = append(descriptors, EffectDescriptor{ID: "range.reset", CapabilityID: "wrong", Boundary: BoundaryOutbound, Owner: "range.Reset"})
	}
	for _, descriptor := range descriptors { _ = effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "for post scalar write",
				packageName: "forpostscalar",
				source: `package forpostscalar
func register(effects Registrar, enabled bool) error {
	descriptor := EffectDescriptor{ID: "for.post.scalar", CapabilityID: "correct", Boundary: BoundaryOutbound, Owner: "forpost.Scalar"}
	wrongDescriptor := EffectDescriptor{ID: "for.post.scalar", CapabilityID: "wrong", Boundary: BoundaryOutbound, Owner: "forpost.Scalar"}
	for ; enabled; descriptor = wrongDescriptor {
		enabled = false
	}
	return effects.Register(descriptor)
}
`,
			},
			{
				name:        "for post collection append",
				packageName: "forpostappend",
				source: `package forpostappend
func register(effects Registrar, enabled bool) error {
	descriptors := []EffectDescriptor{{ID: "for.post.append", CapabilityID: "correct", Boundary: BoundaryOutbound, Owner: "forpost.Append"}}
	for ; enabled; descriptors = append(descriptors, EffectDescriptor{ID: "for.post.append", CapabilityID: "wrong", Boundary: BoundaryOutbound, Owner: "forpost.Append"}) {
		enabled = false
	}
	for _, descriptor := range descriptors { _ = effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "for post collection reset",
				packageName: "forpostreset",
				source: `package forpostreset
func register(effects Registrar, enabled bool) error {
	descriptors := []EffectDescriptor{{ID: "for.post.reset", CapabilityID: "correct", Boundary: BoundaryOutbound, Owner: "forpost.Reset"}}
	for ; enabled; descriptors = nil {
		enabled = false
	}
	for _, descriptor := range descriptors { _ = effects.Register(descriptor) }
	return nil
}
`,
			},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				discovered := discoverFixture(t, testCase.packageName, testCase.source)
				if len(discovered) != 0 {
					t.Fatalf("ambiguous loop provenance was accepted: %#v", discovered)
				}
			})
		}

		for _, testCase := range []struct {
			name        string
			packageName string
			source      string
		}{
			{
				name:        "known range break before append",
				packageName: "rangebreak",
				source: `package rangebreak
func register(effects Registrar, stop bool) error {
	var descriptors []EffectDescriptor
	for range []int{1} {
		if stop { break }
		descriptors = append(descriptors, EffectDescriptor{ID: "range.break", CapabilityID: "correct", Boundary: BoundaryOutbound, Owner: "range.Break"})
	}
	for _, descriptor := range descriptors { _ = effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "known range continue before append",
				packageName: "rangecontinue",
				source: `package rangecontinue
func register(effects Registrar, stop bool) error {
	var descriptors []EffectDescriptor
	for range []int{1} {
		if stop { continue }
		descriptors = append(descriptors, EffectDescriptor{ID: "range.continue", CapabilityID: "correct", Boundary: BoundaryOutbound, Owner: "range.Continue"})
	}
	for _, descriptor := range descriptors { _ = effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "known range goto before append",
				packageName: "rangegoto",
				source: `package rangegoto
func register(effects Registrar, stop bool) error {
	var descriptors []EffectDescriptor
	for range []int{1} {
		if stop { goto done }
		descriptors = append(descriptors, EffectDescriptor{ID: "range.goto", CapabilityID: "correct", Boundary: BoundaryOutbound, Owner: "range.Goto"})
	}
done:
	for _, descriptor := range descriptors { _ = effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "known range return before append",
				packageName: "rangereturn",
				source: `package rangereturn
func register(effects Registrar, stop bool) error {
	var descriptors []EffectDescriptor
	for range []int{1} {
		if stop { return nil }
		descriptors = append(descriptors, EffectDescriptor{ID: "range.return", CapabilityID: "correct", Boundary: BoundaryOutbound, Owner: "range.Return"})
	}
	for _, descriptor := range descriptors { _ = effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "known range continue before scalar register",
				packageName: "rangescalar",
				source: `package rangescalar
func register(effects Registrar, stop bool) error {
	var descriptor EffectDescriptor
	for range []int{1} {
		if stop { continue }
		descriptor = EffectDescriptor{ID: "range.scalar", CapabilityID: "correct", Boundary: BoundaryOutbound, Owner: "range.Scalar"}
		_ = effects.Register(descriptor)
	}
	return nil
}
`,
			},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				discovered := discoverFixture(t, testCase.packageName, testCase.source)
				if len(discovered) != 0 {
					t.Fatalf("branch-controlled loop provenance was accepted: %#v", discovered)
				}
			})
		}

		for _, testCase := range []struct {
			name        string
			packageName string
			source      string
		}{
			{
				name:        "nested loop labeled continue outer",
				packageName: "continueouter",
				source: `package continueouter
func register(effects Registrar) error {
	var descriptors []EffectDescriptor
	effect := struct{ id string }{}
	var err error
outer:
	for range []int{1} {
		for range []int{1} {
			if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
				continue outer
			}
			descriptors = append(descriptors, EffectDescriptor{ID: "continue.outer", CapabilityID: "correct", Boundary: BoundaryOutbound, Owner: "continue.Outer"})
		}
	}
	for _, descriptor := range descriptors { _ = effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "labeled continue in sandbox condition",
				packageName: "continuelabeled",
				source: `package continuelabeled
func register(effects Registrar) error {
	var descriptors []EffectDescriptor
	effect := struct{ id string }{}
	var err error
current:
	for range []int{1} {
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue current
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "continue.labeled", CapabilityID: "correct", Boundary: BoundaryOutbound, Owner: "continue.Labeled"})
	}
	for _, descriptor := range descriptors { _ = effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "nested inner continue bypasses append",
				packageName: "continueinner",
				source: `package continueinner
func register(effects Registrar) error {
	var descriptors []EffectDescriptor
	effect := struct{ id string }{}
	var err error
	for range []int{1} {
		for range []int{1} {
			if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
				continue
			}
			descriptors = append(descriptors, EffectDescriptor{ID: "continue.inner", CapabilityID: "correct", Boundary: BoundaryOutbound, Owner: "continue.Inner"})
		}
	}
	for _, descriptor := range descriptors { _ = effects.Register(descriptor) }
	return nil
}
`,
			},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				discovered := discoverFixture(t, testCase.packageName, testCase.source)
				if len(discovered) != 0 {
					t.Fatalf("continue target ambiguity was accepted: %#v", discovered)
				}
			})
		}

		for _, testCase := range []struct {
			name        string
			packageName string
			source      string
		}{
			{
				name:        "sandbox continue shadowed effect",
				packageName: "sandboxeffectshadow",
				source: `package sandboxeffectshadow
func register(effects Registrar) error {
	var descriptors []EffectDescriptor
	effectsRows := []struct{ id string }{{id: "row"}}
	var err error
	for _, effect := range effectsRows {
		{
			effect := struct{ id string }{}
			if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
				continue
			}
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.effect.shadow", CapabilityID: "correct", Boundary: BoundaryOutbound, Owner: "sandbox.EffectShadow"})
	}
	for _, descriptor := range descriptors { _ = effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "sandbox continue shadowed error",
				packageName: "sandboxerrshadow",
				source: `package sandboxerrshadow
func register(effects Registrar) error {
	var descriptors []EffectDescriptor
	effectsRows := []struct{ id string }{{id: "row"}}
	var err error
	var otherErr error
	for _, effect := range effectsRows {
		{
			err := otherErr
			if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
				continue
			}
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.err.shadow", CapabilityID: "correct", Boundary: BoundaryOutbound, Owner: "sandbox.ErrShadow"})
	}
	for _, descriptor := range descriptors { _ = effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "sandbox continue unrelated error",
				packageName: "sandboxothererr",
				source: `package sandboxothererr
func register(effects Registrar) error {
	var descriptors []EffectDescriptor
	effectsRows := []struct{ id string }{{id: "row"}}
	var err error
	var otherErr error
	for _, effect := range effectsRows {
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(otherErr, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.other.err", CapabilityID: "correct", Boundary: BoundaryOutbound, Owner: "sandbox.OtherErr"})
	}
	for _, descriptor := range descriptors { _ = effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "sandbox continue another resolve error",
				packageName: "sandboxotherresolve",
				source: `package sandboxotherresolve
func register(options Options) error {
	var descriptors []EffectDescriptor
	effectsRows := []struct{ id string }{{id: "row"}}
	for _, effect := range effectsRows {
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		otherCapability, otherErr := options.Authorities.CapabilityBindings.Resolve(effect.id)
		_ = err
		_ = otherCapability
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(otherErr, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.other.resolve", CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.OtherResolve"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				discovered := discoverFixture(t, testCase.packageName, testCase.source)
				if len(discovered) != 0 {
					t.Fatalf("sandbox identity ambiguity was accepted: %#v", discovered)
				}
			})
		}

		for _, testCase := range []struct {
			name        string
			packageName string
			source      string
		}{
			{
				name:        "sandbox error overwritten by other effect",
				packageName: "sandboxerroverwrite",
				source: `package sandboxerroverwrite
func register(options Options) error {
	var descriptors []EffectDescriptor
	effectsRows := []struct{ id string }{{id: "row"}}
	otherEffect := struct{ id string }{id: "other"}
	for _, effect := range effectsRows {
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		otherCapability, err := options.Authorities.CapabilityBindings.Resolve(otherEffect.id)
		_ = otherCapability
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.error.overwrite", CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.ErrorOverwrite"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "sandbox capability and error reassigned",
				packageName: "sandboxreassigned",
				source: `package sandboxreassigned
func register(options Options) error {
	var descriptors []EffectDescriptor
	effectsRows := []struct{ id string }{{id: "row"}}
	otherEffect := struct{ id string }{id: "other"}
	for _, effect := range effectsRows {
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		capability, err = options.Authorities.CapabilityBindings.Resolve(otherEffect.id)
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.reassigned", CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.Reassigned"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				discovered := discoverFixture(t, testCase.packageName, testCase.source)
				if len(discovered) != 0 {
					t.Fatalf("stale Resolve provenance was accepted: %#v", discovered)
				}
			})
		}

		for _, testCase := range []struct {
			name        string
			packageName string
			source      string
		}{
			{
				name:        "sandbox error ordinary reassignment",
				packageName: "sandboxerrwrite",
				source: `package sandboxerrwrite
func register(options Options) error {
	var descriptors []EffectDescriptor
	effectsRows := []struct{ id string }{{id: "row"}}
	var otherErr error
	for _, effect := range effectsRows {
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		err = otherErr
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.err.write", CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.ErrWrite"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "sandbox capability ordinary reassignment",
				packageName: "sandboxcapwrite",
				source: `package sandboxcapwrite
func register(options Options) error {
	var descriptors []EffectDescriptor
	effectsRows := []struct{ id string }{{id: "row"}}
	var wrongCapability CapabilityID
	for _, effect := range effectsRows {
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		capability = wrongCapability
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.cap.write", CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.CapWrite"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "sandbox tuple ordinary reassignment",
				packageName: "sandboxtuplewrite",
				source: `package sandboxtuplewrite
func register(options Options) error {
	var descriptors []EffectDescriptor
	effectsRows := []struct{ id string }{{id: "row"}}
	var wrongCapability CapabilityID
	var otherErr error
	for _, effect := range effectsRows {
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		capability, err = wrongCapability, otherErr
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.tuple.write", CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.TupleWrite"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "sandbox conditional ordinary error write",
				packageName: "sandboxconditionalwrite",
				source: `package sandboxconditionalwrite
func register(options Options, overwrite bool) error {
	var descriptors []EffectDescriptor
	effectsRows := []struct{ id string }{{id: "row"}}
	var otherErr error
	for _, effect := range effectsRows {
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		if overwrite { err = otherErr }
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.conditional.write", CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.ConditionalWrite"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				discovered := discoverFixture(t, testCase.packageName, testCase.source)
				if len(discovered) != 0 {
					t.Fatalf("stale sandbox binding write was accepted: %#v", discovered)
				}
			})
		}

		for _, testCase := range []struct {
			name        string
			packageName string
			source      string
		}{
			{
				name:        "sandbox row selector reassignment",
				packageName: "sandboxrowselectorwrite",
				source: `package sandboxrowselectorwrite
func register(options Options) error {
	var descriptors []EffectDescriptor
	effectsRows := []struct{ id string }{{id: "row"}}
	for _, effect := range effectsRows {
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		effect.id = releasecontract.EffectAgentToolPythonSandbox
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.row.selector", CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.RowSelector"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "sandbox row object reassignment",
				packageName: "sandboxrowobjectwrite",
				source: `package sandboxrowobjectwrite
func register(options Options) error {
	var descriptors []EffectDescriptor
	effectsRows := []struct{ id string }{{id: "row"}}
	otherEffect := struct{ id string }{id: "other"}
	for _, effect := range effectsRows {
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		effect = otherEffect
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.row.object", CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.RowObject"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "sandbox row tuple reassignment",
				packageName: "sandboxrowtuplewrite",
				source: `package sandboxrowtuplewrite
func register(options Options) error {
	var descriptors []EffectDescriptor
	effectsRows := []struct{ id string }{{id: "row"}}
	otherEffect := struct{ id string }{id: "other"}
	for _, effect := range effectsRows {
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		effect, otherEffect = otherEffect, effect
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.row.tuple", CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.RowTuple"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "sandbox conditional row selector write",
				packageName: "sandboxrowconditionalwrite",
				source: `package sandboxrowconditionalwrite
func register(options Options, overwrite bool) error {
	var descriptors []EffectDescriptor
	effectsRows := []struct{ id string }{{id: "row"}}
	for _, effect := range effectsRows {
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		if overwrite { effect.id = releasecontract.EffectAgentToolPythonSandbox }
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.row.conditional", CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.RowConditional"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				discovered := discoverFixture(t, testCase.packageName, testCase.source)
				if len(discovered) != 0 {
					t.Fatalf("mutated sandbox row was accepted: %#v", discovered)
				}
			})
		}

		for _, testCase := range []struct {
			name        string
			packageName string
			source      string
		}{
			{
				name:        "sandbox row helper pointer escape",
				packageName: "sandboxrowhelperescape",
				source: `package sandboxrowhelperescape
func mutateRow(effect *struct{ id string }) {
	effect.id = releasecontract.EffectAgentToolPythonSandbox
}
func register(options Options) error {
	var descriptors []EffectDescriptor
	effectsRows := []struct{ id string }{{id: "row"}}
	for _, effect := range effectsRows {
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		mutateRow(&effect)
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.row.helper.escape", CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.RowHelperEscape"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "sandbox row local pointer alias",
				packageName: "sandboxrowaliasescape",
				source: `package sandboxrowaliasescape
func register(options Options) error {
	var descriptors []EffectDescriptor
	effectsRows := []struct{ id string }{{id: "row"}}
	for _, effect := range effectsRows {
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		pointer := &effect
		pointer.id = releasecontract.EffectAgentToolPythonSandbox
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.row.alias.escape", CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.RowAliasEscape"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "sandbox row conditional selector helper escape",
				packageName: "sandboxrowconditionalescape",
				source: `package sandboxrowconditionalescape
func mutateID(id *string) {
	*id = releasecontract.EffectAgentToolPythonSandbox
}
func register(options Options, overwrite bool) error {
	var descriptors []EffectDescriptor
	effectsRows := []struct{ id string }{{id: "row"}}
	for _, effect := range effectsRows {
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		if overwrite { mutateID(&effect.id) }
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.row.conditional.escape", CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.RowConditionalEscape"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				discovered := discoverFixture(t, testCase.packageName, testCase.source)
				if len(discovered) != 0 {
					t.Fatalf("escaped sandbox row was accepted: %#v", discovered)
				}
			})
		}

		for _, testCase := range []struct {
			name        string
			packageName string
			source      string
		}{
			{
				name:        "sandbox row pre-resolve local pointer alias",
				packageName: "sandboxrowpreresolvealias",
				source: `package sandboxrowpreresolvealias
func register(options Options) error {
	var descriptors []EffectDescriptor
	effectsRows := []struct{ id string }{{id: "row"}}
	for _, effect := range effectsRows {
		pointer := &effect
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		pointer.id = releasecontract.EffectAgentToolPythonSandbox
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.row.pre.resolve.alias", CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.RowPreResolveAlias"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "sandbox row pre-resolve selector pointer alias",
				packageName: "sandboxrowpreresolveselector",
				source: `package sandboxrowpreresolveselector
func register(options Options) error {
	var descriptors []EffectDescriptor
	effectsRows := []struct{ id string }{{id: "row"}}
	for _, effect := range effectsRows {
		pointer := &effect.id
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		*pointer = releasecontract.EffectAgentToolPythonSandbox
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.row.pre.resolve.selector", CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.RowPreResolveSelector"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "sandbox row conditional pre-resolve helper alias",
				packageName: "sandboxrowconditionalpreresolve",
				source: `package sandboxrowconditionalpreresolve
func mutateID(id *string) {
	*id = releasecontract.EffectAgentToolPythonSandbox
}
func register(options Options, overwrite bool) error {
	var descriptors []EffectDescriptor
	effectsRows := []struct{ id string }{{id: "row"}}
	for _, effect := range effectsRows {
		if overwrite { mutateID(&effect.id) }
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.row.conditional.pre.resolve", CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.RowConditionalPreResolve"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				discovered := discoverFixture(t, testCase.packageName, testCase.source)
				if len(discovered) != 0 {
					t.Fatalf("pre-resolve sandbox alias was accepted: %#v", discovered)
				}
			})
		}

		for _, testCase := range []struct {
			name        string
			packageName string
			source      string
		}{
			{
				name:        "sandbox row pointer receiver escape",
				packageName: "sandboxrowpointerreceiver",
				source: `package sandboxrowpointerreceiver
type effectRow struct{ id string }
func (row *effectRow) setSandbox() {
	row.id = releasecontract.EffectAgentToolPythonSandbox
}
func register(options Options) error {
	var descriptors []EffectDescriptor
	effectsRows := []effectRow{{id: "row"}}
	for _, effect := range effectsRows {
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		effect.setSandbox()
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.row.pointer.receiver", CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.RowPointerReceiver"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "sandbox row value method pointer escape",
				packageName: "sandboxrowvaluemethod",
				source: `package sandboxrowvaluemethod
type effectRow struct{ id string }
func (row effectRow) ptr() *effectRow {
	return &row
}
func register(options Options) error {
	var descriptors []EffectDescriptor
	effectsRows := []effectRow{{id: "row"}}
	for _, effect := range effectsRows {
		pointer := effect.ptr()
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		pointer.id = releasecontract.EffectAgentToolPythonSandbox
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.row.value.method", CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.RowValueMethod"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "sandbox row conditional pointer receiver escape",
				packageName: "sandboxrowconditionalreceiver",
				source: `package sandboxrowconditionalreceiver
type effectRow struct{ id string }
func (row *effectRow) setSandbox() {
	row.id = releasecontract.EffectAgentToolPythonSandbox
}
func register(options Options, overwrite bool) error {
	var descriptors []EffectDescriptor
	effectsRows := []effectRow{{id: "row"}}
	for _, effect := range effectsRows {
		if overwrite { effect.setSandbox() }
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.row.conditional.receiver", CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.RowConditionalReceiver"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				discovered := discoverFixture(t, testCase.packageName, testCase.source)
				if len(discovered) != 0 {
					t.Fatalf("implicit sandbox row escape was accepted: %#v", discovered)
				}
			})
		}

		for _, testCase := range []struct {
			name        string
			packageName string
			source      string
		}{
			{
				name:        "sandbox row setter method value escape",
				packageName: "sandboxrowsettervalue",
				source: `package sandboxrowsettervalue
type effectRow struct{ id string }
func (row *effectRow) setSandbox() {
	row.id = releasecontract.EffectAgentToolPythonSandbox
}
func register(options Options) error {
	var descriptors []EffectDescriptor
	effectsRows := []effectRow{{id: "row"}}
	for _, effect := range effectsRows {
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		setter := effect.setSandbox
		setter()
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.row.setter.value", CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.RowSetterValue"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "sandbox row pointer method value escape",
				packageName: "sandboxrowpointervalue",
				source: `package sandboxrowpointervalue
type effectRow struct{ id string }
func (row effectRow) ptr() *effectRow {
	return &row
}
func register(options Options) error {
	var descriptors []EffectDescriptor
	effectsRows := []effectRow{{id: "row"}}
	for _, effect := range effectsRows {
		pointerFactory := effect.ptr
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		pointer := pointerFactory()
		pointer.id = releasecontract.EffectAgentToolPythonSandbox
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.row.pointer.value", CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.RowPointerValue"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "sandbox row conditional method value escape",
				packageName: "sandboxrowconditionalvalue",
				source: `package sandboxrowconditionalvalue
type effectRow struct{ id string }
func (row *effectRow) setSandbox() {
	row.id = releasecontract.EffectAgentToolPythonSandbox
}
func register(options Options, overwrite bool) error {
	var descriptors []EffectDescriptor
	effectsRows := []effectRow{{id: "row"}}
	for _, effect := range effectsRows {
		if overwrite {
			setter := effect.setSandbox
			setter()
		}
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: "sandbox.row.conditional.value", CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.RowConditionalValue"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				discovered := discoverFixture(t, testCase.packageName, testCase.source)
				if len(discovered) != 0 {
					t.Fatalf("sandbox row method value was accepted: %#v", discovered)
				}
			})
		}

		for _, testCase := range []struct {
			name        string
			packageName string
			source      string
		}{
			{
				name:        "sandbox descriptor id method escape",
				packageName: "sandboxdescriptoridmethod",
				source: `package sandboxdescriptoridmethod
type effectRow struct{ id string }
func (row *effectRow) mutateAndID(id string) string {
	row.id = releasecontract.EffectAgentToolPythonSandbox
	return id
}
func register(options Options) error {
	var descriptors []EffectDescriptor
		effectsRows := []effectRow{{id: "row"}}
	for _, effect := range effectsRows {
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: effect.mutateAndID("sandbox.descriptor.id.method"), CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.DescriptorIDMethod"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "sandbox descriptor owner first method escape",
				packageName: "sandboxdescriptorownermethod",
				source: `package sandboxdescriptorownermethod
type effectRow struct{ id, owner string }
func (row *effectRow) mutateAndOwner(owner string) string {
	row.id = releasecontract.EffectAgentToolPythonSandbox
	return owner
}
func register(options Options) error {
	var descriptors []EffectDescriptor
		effectsRows := []effectRow{{id: "row", owner: "sandbox.DescriptorOwnerMethod"}}
	for _, effect := range effectsRows {
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{Owner: effect.mutateAndOwner("sandbox.DescriptorOwnerMethod"), ID: "sandbox.descriptor.owner.method", CapabilityID: string(capability), Boundary: BoundaryOutbound})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "sandbox descriptor id arbitrary wrapper",
				packageName: "sandboxdescriptoridwrapper",
				source: `package sandboxdescriptoridwrapper
func fake(value string) string { return value }
func register(options Options) error {
	var descriptors []EffectDescriptor
		effectsRows := []struct{ id string }{{"sandbox.descriptor.id.wrapper"}}
	for _, effect := range effectsRows {
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{ID: fake(effect.id), CapabilityID: string(capability), Boundary: BoundaryOutbound, Owner: "sandbox.DescriptorIDWrapper"})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
			{
				name:        "sandbox descriptor owner arbitrary wrapper",
				packageName: "sandboxdescriptorownerwrapper",
				source: `package sandboxdescriptorownerwrapper
func fake(value string) string { return value }
func register(options Options) error {
	var descriptors []EffectDescriptor
		effectsRows := []struct{ id, owner string }{{"sandbox.descriptor.owner.wrapper", "sandbox.DescriptorOwnerWrapper"}}
	for _, effect := range effectsRows {
		capability, err := options.Authorities.CapabilityBindings.Resolve(effect.id)
		if effect.id == releasecontract.EffectAgentToolPythonSandbox && releasecontract.IsReadinessCode(err, releasecontract.CodeCapabilityUnknown) {
			continue
		}
		descriptors = append(descriptors, EffectDescriptor{Owner: fake(effect.owner), ID: string(effect.id), CapabilityID: string(capability), Boundary: BoundaryOutbound})
	}
	for _, descriptor := range descriptors { _ = options.Effects.Register(descriptor) }
	return nil
}
`,
			},
		} {
			t.Run(testCase.name, func(t *testing.T) {
				discovered := discoverFixture(t, testCase.packageName, testCase.source)
				if len(discovered) != 0 {
					t.Fatalf("unsafe descriptor endpoint was accepted: %#v", discovered)
				}
			})
		}

		t.Run("arbitrary scalar string wrapper is not transparent", func(t *testing.T) {
			discovered := discoverFixture(t, "scalarwrapper", `package scalarwrapper
func fake(value string) string { return value }
func register(effects Registrar) error {
	return effects.Register(EffectDescriptor{ID: fake("scalar.wrapper"), CapabilityID: "correct", Boundary: BoundaryOutbound, Owner: "scalar.Wrapper"})
}
`)
			if len(discovered) != 0 {
				t.Fatalf("arbitrary scalar wrapper was evaluated transparently: %#v", discovered)
			}
		})

		t.Run("last descriptor assignment wins", func(t *testing.T) {
			root := t.TempDir()
			packageDirectory := filepath.Join(root, "src", "server", "internal", "reaching")
			if err := os.MkdirAll(packageDirectory, 0o755); err != nil {
				t.Fatalf("create reaching fixture: %v", err)
			}
			source := `package reaching
func register(effects Registrar) error {
		descriptor := EffectDescriptor{ID: "reaching.assignment", CapabilityID: "good", Boundary: BoundaryOutbound, Owner: "reaching.Owner"}
		descriptor = EffectDescriptor{ID: "reaching.assignment", CapabilityID: "wrong", Boundary: BoundaryOutbound, Owner: "reaching.Owner"}
		return effects.Register(descriptor)
	}
`
			if err := os.WriteFile(filepath.Join(packageDirectory, "reaching.go"), []byte(source), 0o644); err != nil {
				t.Fatalf("write reaching assignment fixture: %v", err)
			}
			discovered, err := DiscoverEffectSurfaces(root)
			if err != nil {
				t.Fatalf("discover reaching assignment fixture: %v", err)
			}
			if len(discovered) != 1 || discovered[0].CapabilityID != "wrong" {
				t.Fatalf("stale assignment certified: %#v", discovered)
			}
		})

		t.Run("last collection state wins", func(t *testing.T) {
			root := t.TempDir()
			packageDirectory := filepath.Join(root, "src", "server", "internal", "reaching")
			if err := os.MkdirAll(packageDirectory, 0o755); err != nil {
				t.Fatalf("create reaching collection fixture: %v", err)
			}
			source := `package reaching
func register(effects Registrar) error {
		descriptors := []EffectDescriptor{{ID: "reaching.collection", CapabilityID: "good", Boundary: BoundaryOutbound, Owner: "reaching.Owner"}}
		descriptors = nil
		descriptors = append(descriptors, EffectDescriptor{ID: "reaching.collection", CapabilityID: "wrong", Boundary: BoundaryOutbound, Owner: "reaching.Owner"})
		for _, descriptor := range descriptors { _ = effects.Register(descriptor) }
		return nil
	}
`
			if err := os.WriteFile(filepath.Join(packageDirectory, "reaching.go"), []byte(source), 0o644); err != nil {
				t.Fatalf("write reaching collection fixture: %v", err)
			}
			discovered, err := DiscoverEffectSurfaces(root)
			if err != nil {
				t.Fatalf("discover reaching collection fixture: %v", err)
			}
			if len(discovered) != 1 || discovered[0].CapabilityID != "wrong" {
				t.Fatalf("stale collection state certified: %#v", discovered)
			}
		})
	})
}

func TestEffectCoverageFirstFailureIsDeterministic(t *testing.T) {
	unknownSurface := func(descriptorID string) EffectSurface {
		return EffectSurface{
			SeamID: descriptorID + "@unknown.Owner", DescriptorID: descriptorID, OwnerPackage: "unknown", OwnerSymbol: "unknown.Owner",
			CapabilityID: "mcp.tool_execution", Boundary: BoundaryOutbound, ASTCall: "structural", RegistrationSymbol: "newUnknown",
			GuardCall: "run:guard", EffectCall: "run:effect", ProfileDisposition: CommitmentCommitted,
		}
	}
	for _, surfaces := range [][]EffectSurface{
		{unknownSurface("z.unknown"), unknownSurface("a.unknown")},
		{unknownSurface("a.unknown"), unknownSurface("z.unknown")},
	} {
		err := validateEffectSurfaceManifestAgainstSpecs(EffectSurfaceManifest{SchemaVersion: EffectSurfaceSchemaV1, Surfaces: surfaces}, runtimeDescriptorSpecs)
		var coverageErr *EffectCoverageError
		if !errors.As(err, &coverageErr) || coverageErr.Code != "effect_manifest_unknown_descriptor" || coverageErr.Field != "a.unknown" {
			t.Fatalf("manifest first error = %v, want a.unknown", err)
		}
	}

	runtimeRows := []EffectDescriptor{
		{ID: "z.unknown", CapabilityID: "mcp.tool_execution", Boundary: BoundaryOutbound, Owner: "unknown.Owner"},
		{ID: "a.unknown", CapabilityID: "mcp.tool_execution", Boundary: BoundaryOutbound, Owner: "unknown.Owner"},
	}
	for _, rows := range [][]EffectDescriptor{runtimeRows, {runtimeRows[1], runtimeRows[0]}} {
		err := joinRuntime(nil, DeploymentProfile{}, rows, selectedRuntimeConfiguration())
		var coverageErr *EffectCoverageError
		if !errors.As(err, &coverageErr) || coverageErr.Code != "effect_coverage_extra_runtime" || coverageErr.Field != "a.unknown" {
			t.Fatalf("runtime first error = %v, want a.unknown", err)
		}
	}

	selection := selectedRuntimeConfiguration()
	runtimeWithTwoDisabledGroups := selectedRuntimeDescriptors(selection)
	runtimeWithTwoDisabledGroups = append(runtimeWithTwoDisabledGroups, descriptorForSpec("chat.provider.fallback"), descriptorForSpec("worker.archive.claim"))
	expected := make([]EffectSurface, 0, len(runtimeDescriptorSpecs))
	for _, descriptorID := range sortedRuntimeDescriptorSpecIDs(runtimeDescriptorSpecs) {
		expected = append(expected, effectSurfaceForSpec(descriptorID, runtimeDescriptorSpecs[descriptorID]))
	}
	err := joinRuntime(expected, DeploymentProfile{}, runtimeWithTwoDisabledGroups, selection)
	var coverageErr *EffectCoverageError
	if !errors.As(err, &coverageErr) || coverageErr.Code != "effect_coverage_selection_drift" || coverageErr.Field != "channel.archive" {
		t.Fatalf("selector first error = %v, want channel.archive", err)
	}
}

func TestEffectCoverageRequiresRuntimeAuthoritiesContract(t *testing.T) {
	_, manifest, contract, profile := effectCoverageFixture(t)
	selection := selectedRuntimeConfiguration()
	authorities, err := NewRuntimeAuthorities(contract, profile, effectCoverageGuard{})
	if err != nil {
		t.Fatalf("build runtime authorities: %v", err)
	}
	runtimeDescriptors := selectedRuntimeDescriptors(selection)
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
			err := VerifyEffectCoverage(EffectCoverageOptions{Manifest: manifest, Contract: testCase.contract, Profile: testCase.profile, Authorities: testCase.authority, Runtime: runtimeDescriptors, Selection: selection})
			if !IsEffectCoverageCode(err, testCase.expected) {
				t.Fatalf("error = %v, want %s", err, testCase.expected)
			}
		})
	}
	missingEffect := authorities
	missingEffect.CapabilityBindings.bindings = cloneEffectBindings(authorities.CapabilityBindings.bindings)
	delete(missingEffect.CapabilityBindings.bindings, EffectAgentToolBuiltin)
	if err := VerifyEffectCoverage(EffectCoverageOptions{Manifest: manifest, Contract: contract, Profile: profile, Authorities: missingEffect, Runtime: runtimeDescriptors, Selection: selection}); !IsEffectCoverageCode(err, "effect_coverage_unknown_effect") {
		t.Fatalf("missing effect binding error = %v", err)
	}
	mismatchedEffect := authorities
	mismatchedEffect.CapabilityBindings.bindings = cloneEffectBindings(authorities.CapabilityBindings.bindings)
	mismatchedEffect.CapabilityBindings.bindings[EffectAgentToolBuiltin] = authoredEffectCapabilities[EffectAgentToolCustomAPIHTTP]
	if err := VerifyEffectCoverage(EffectCoverageOptions{Manifest: manifest, Contract: contract, Profile: profile, Authorities: mismatchedEffect, Runtime: runtimeDescriptors, Selection: selection}); !IsEffectCoverageCode(err, "effect_coverage_capability_mismatch") {
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
	selection := selectedRuntimeConfiguration()
	authorities, err := NewRuntimeAuthorities(contract, profile, effectCoverageGuard{})
	if err != nil {
		t.Fatalf("build runtime authorities: %v", err)
	}
	runtimeDescriptors := selectedRuntimeDescriptors(selection)
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
	if err := VerifyEffectCoverage(EffectCoverageOptions{RepoRoot: repoRoot, Manifest: manifest, Contract: contract, Profile: profile, Authorities: authorities, Registry: registry, Selection: selection}); err != nil {
		t.Fatalf("valid exact join: %v", err)
	}
	disabledOptionalSelection := &EffectRuntimeConfiguration{WebSearchProvider: "provider"}
	if err := VerifyEffectCoverage(EffectCoverageOptions{Manifest: manifest, Contract: contract, Profile: profile, Authorities: authorities, Runtime: selectedRuntimeDescriptors(disabledOptionalSelection), Selection: disabledOptionalSelection}); err != nil {
		t.Fatalf("explicitly disabled optional cohorts: %v", err)
	}

	missing := withoutDescriptor(runtimeDescriptors, "agent.tool.builtin")
	missingProvider := withoutDescriptor(runtimeDescriptors, "mcp.websearch.provider")
	multipleProvider := append(cloneDescriptors(runtimeDescriptors), descriptorForSpec("mcp.websearch.chain"))
	missingScheduleCohort := cloneDescriptors(runtimeDescriptors)
	for _, descriptorID := range []string{"worker.schedule.claim", "worker.schedule.workflow.start", "worker.schedule.workflow.continue", "worker.schedule.agent.start"} {
		missingScheduleCohort = withoutDescriptor(missingScheduleCohort, descriptorID)
	}
	partialOptional := append(cloneDescriptors(missingScheduleCohort), descriptorForSpec("worker.schedule.claim"))
	extra := append(cloneDescriptors(runtimeDescriptors), EffectDescriptor{ID: "unknown.effect", CapabilityID: "mcp.tool_execution", Boundary: BoundaryOutbound, Owner: "unknown.Owner"})
	duplicate := append(cloneDescriptors(runtimeDescriptors), runtimeDescriptors[0])
	excluded := append(cloneDescriptors(runtimeDescriptors), descriptorForSpec("agent.tool.web_search"))
	ownerDrift := cloneDescriptors(runtimeDescriptors)
	ownerDrift[0].Owner = "wrong.Owner"
	capabilityDrift := cloneDescriptors(runtimeDescriptors)
	capabilityDrift[0].CapabilityID = "mcp.custom_execution"
	cases := []struct {
		name      string
		runtime   []EffectDescriptor
		selection *EffectRuntimeConfiguration
		expected  string
	}{
		{"zero runtime", nil, selection, "effect_coverage_zero_runtime"},
		{"missing selection", runtimeDescriptors, nil, "effect_coverage_configuration_required"},
		{"missing", missing, selection, "effect_coverage_missing_runtime"},
		{"extra", extra, selection, "effect_coverage_extra_runtime"},
		{"duplicate", duplicate, selection, "effect_coverage_duplicate_runtime"},
		{"excluded independent", excluded, selection, "effect_coverage_profile_drift"},
		{"owner drift", ownerDrift, selection, "effect_coverage_runtime_drift"},
		{"capability drift", capabilityDrift, selection, "effect_coverage_runtime_drift"},
		{"missing selected schedule cohort", missingScheduleCohort, selection, "effect_coverage_selection_drift"},
		{"missing provider selection", missingProvider, selection, "effect_coverage_selection_drift"},
		{"multiple provider selection", multipleProvider, selection, "effect_coverage_selection_drift"},
		{"partial optional cohort", partialOptional, selection, "effect_coverage_selection_drift"},
		{"wrong selected websearch", runtimeDescriptors, &EffectRuntimeConfiguration{ScheduleWorkerEnabled: true, RelayRuntimeEnabled: true, WebSearchProvider: "tavily"}, "effect_coverage_selection_drift"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := VerifyEffectCoverage(EffectCoverageOptions{Manifest: manifest, Contract: contract, Profile: profile, Authorities: authorities, Runtime: testCase.runtime, Selection: testCase.selection})
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

func selectedRuntimeConfiguration() *EffectRuntimeConfiguration {
	return &EffectRuntimeConfiguration{ScheduleWorkerEnabled: true, RelayRuntimeEnabled: true, WebSearchProvider: "provider"}
}

func selectedRuntimeDescriptors(selection *EffectRuntimeConfiguration) []EffectDescriptor {
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
				selected := map[string]bool{
					"schedule.worker": selection.ScheduleWorkerEnabled,
					"channel.archive": selection.ChannelArchiveEnabled,
					"relay.batch":     selection.RelayBatchEnabled,
					"relay.runtime":   selection.RelayRuntimeEnabled,
					"chat.fallback":   selection.ChatFallbackEnabled,
				}[spec.Configuration.Group]
				if !selected {
					continue
				}
			}
			if spec.Configuration.Mode == effectSelectorOneOf && spec.Configuration.Value != selection.WebSearchProvider {
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
