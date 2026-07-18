package releasecontract

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReadinessEffectCoverageContract(t *testing.T) {
	registry := NewEffectRegistry()
	first := EffectDescriptor{ID: "agent.tool.builtin", CapabilityID: "mcp.tool_execution", Boundary: BoundaryOutbound, Owner: "test.Owner"}
	second := EffectDescriptor{ID: "agent.tool.mcp", CapabilityID: "mcp.custom_execution", Boundary: BoundaryOutbound, Owner: "test.Owner"}
	if err := registry.Register(first); err != nil {
		t.Fatalf("register first: %v", err)
	}
	before := registry.Snapshot()
	if err := registry.Register(second); err != nil {
		t.Fatalf("register second: %v", err)
	}
	if err := registry.Register(first); !IsEffectCoverageCode(err, "effect_registry_duplicate") {
		t.Fatalf("duplicate error = %v", err)
	}
	if err := registry.Register(EffectDescriptor{ID: "caller.effect", CapabilityID: "mcp.tool_execution", Boundary: BoundaryOutbound, Owner: "test.Owner"}); !IsEffectCoverageCode(err, "effect_registry_unknown") {
		t.Fatalf("unknown error = %v", err)
	}
	if len(before) != 1 || len(registry.Snapshot()) != 2 {
		t.Fatalf("registry snapshot mutated unexpectedly: before=%#v after=%#v", before, registry.Snapshot())
	}

	_, filename, _, _ := runtime.Caller(0)
	manifestPath := filepath.Join(filepath.Dir(filename), "..", "..", "..", "..", "config", "release", "readiness-effect-surface.v1.json")
	manifest, err := LoadEffectSurfaceManifest(manifestPath)
	if err != nil {
		t.Fatalf("load expected inventory: %v", err)
	}
	if len(manifest.Surfaces) < 12 {
		t.Fatalf("expected standalone roots in inventory, got %d rows", len(manifest.Surfaces))
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", "..", "..", ".."))
	discovered, err := DiscoverEffectSurfaces(repoRoot)
	if err != nil {
		t.Fatalf("discover production effect surfaces: %v", err)
	}
	if err := joinStatic(manifest.Surfaces, discovered); err != nil {
		t.Fatalf("expected/static exact join: %v", err)
	}
	for _, owner := range []string{"agent.ToolExecutor.builtin", "agent.ToolExecutor.custom_api_http", "agent.ToolExecutor.local_python", "agent.ToolExecutor.python_sandbox", "agent.ToolExecutor.mcp"} {
		found := false
		for _, surface := range manifest.Surfaces {
			if strings.Contains(surface.SeamID, owner) {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing ToolExecutor seam %q", owner)
		}
	}

	minimal := EffectSurfaceManifest{SchemaVersion: EffectSurfaceSchemaV1, Surfaces: []EffectSurface{{
		SeamID: "agent.tool.builtin@test.Owner", OwnerPackage: "test", OwnerSymbol: "test.Owner", CapabilityID: "mcp.tool_execution", Boundary: BoundaryOutbound, ASTCall: "runtime", ProfileDisposition: CommitmentCommitted,
	}, {
		SeamID: "agent.tool.mcp@test.Owner", OwnerPackage: "test", OwnerSymbol: "test.Owner", CapabilityID: "mcp.custom_execution", Boundary: BoundaryOutbound, ASTCall: "runtime", ProfileDisposition: CommitmentConditional,
	}}}
	if err := VerifyEffectCoverage(EffectCoverageOptions{Manifest: minimal, Runtime: registry.Snapshot()}); err != nil {
		t.Fatalf("valid exact join: %v", err)
	}
	if err := VerifyEffectCoverage(EffectCoverageOptions{Manifest: minimal, Runtime: []EffectDescriptor{first}}); !IsEffectCoverageCode(err, "effect_coverage_zero_runtime") && !IsEffectCoverageCode(err, "effect_coverage_missing_runtime") {
		t.Fatalf("missing runtime error = %v", err)
	}
	if err := VerifyEffectCoverage(EffectCoverageOptions{Manifest: minimal, Runtime: append(registry.Snapshot(), EffectDescriptor{ID: "extra", CapabilityID: "mcp.tool_execution", Boundary: BoundaryOutbound, Owner: "test.Owner"})}); !IsEffectCoverageCode(err, "effect_coverage_extra_runtime") {
		t.Fatalf("extra runtime error = %v", err)
	}
}
