package tools

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"oblivious/server/internal/mcp"
	"oblivious/server/internal/releasecontract"
)

func TestToolRegistryReadinessExecutionContract(t *testing.T) {
	guard := &registryReadinessGuard{}
	guard.allow.Store(true)
	contract, profile := loadRegistryReadinessContract(t)
	registrar := newRegistryReadinessRegistrar()
	authorities, err := releasecontract.NewRuntimeAuthorities(contract, profile, guard)
	if err != nil {
		t.Fatalf("build runtime authorities: %v", err)
	}
	registry, err := NewRegistryWithOptions(RegistryRuntimeOptions{Guard: guard, Authorities: authorities, Effects: registrar})
	if err != nil {
		t.Fatalf("NewRegistryWithOptions: %v", err)
	}
	var builtinCalls, customCalls, mcpCalls atomic.Int32
	registry.RegisterBuiltin("calculator", &registryProbeTool{name: "calculator", calls: &builtinCalls})
	registry.RegisterCustom(ToolMetadata{Name: "custom_http", Extra: map[string]any{"capabilityId": "caller.forced"}}, func(context.Context, map[string]any) (*mcp.ToolResult, error) {
		customCalls.Add(1)
		return &mcp.ToolResult{Content: "custom"}, nil
	})
	registry.RegisterMCP(ToolMetadata{Name: "remote", ServerID: "server-1", Extra: map[string]any{"capabilityId": "stale"}}, func(context.Context, map[string]any) (*mcp.ToolResult, error) {
		mcpCalls.Add(1)
		return &mcp.ToolResult{Content: "mcp"}, nil
	})

	for i := 0; i < 2; i++ {
		for _, name := range []string{"calculator", "custom_http", "remote"} {
			if _, err := registry.Execute(context.Background(), name, nil); err != nil {
				t.Fatalf("Execute(%s): %v", name, err)
			}
		}
	}
	if got := registrar.count(); got != 3 {
		t.Fatalf("registrar count = %d, want 3", got)
	}
	if got := guard.count(); got != 6 {
		t.Fatalf("guard count = %d, want 6", got)
	}
	if builtinCalls.Load() != 2 || customCalls.Load() != 2 || mcpCalls.Load() != 2 {
		t.Fatalf("executor counts = %d/%d/%d, want 2/2/2", builtinCalls.Load(), customCalls.Load(), mcpCalls.Load())
	}

	guard.allow.Store(false)
	if _, err := registry.Execute(context.Background(), "calculator", nil); err == nil {
		t.Fatal("denied registry execution returned nil error")
	}
	if builtinCalls.Load() != 2 {
		t.Fatalf("denied builtin executor count = %d, want unchanged 2", builtinCalls.Load())
	}
	if _, err := NewRegistryWithOptions(RegistryRuntimeOptions{Guard: guard, Authorities: authorities, Effects: registrar}); err == nil {
		t.Fatal("duplicate registry owner construction unexpectedly succeeded")
	}
}

type registryProbeTool struct {
	name  string
	calls *atomic.Int32
}

func (t *registryProbeTool) Name() string        { return t.name }
func (t *registryProbeTool) Description() string { return "probe" }
func (t *registryProbeTool) InputSchema() any    { return map[string]any{"type": "object"} }
func (t *registryProbeTool) Execute(context.Context, map[string]any) (*mcp.ToolResult, error) {
	t.calls.Add(1)
	return &mcp.ToolResult{Content: t.name}, nil
}

type registryReadinessGuard struct {
	allow atomic.Bool
	calls atomic.Int32
}

func (g *registryReadinessGuard) Require(context.Context, string, releasecontract.Boundary) error {
	g.calls.Add(1)
	if !g.allow.Load() {
		return &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityBlocked, Field: "test"}
	}
	return nil
}

func (g *registryReadinessGuard) count() int { return int(g.calls.Load()) }

type registryReadinessRegistrar struct {
	mu          sync.Mutex
	descriptors map[string]releasecontract.EffectDescriptor
}

func newRegistryReadinessRegistrar() *registryReadinessRegistrar {
	return &registryReadinessRegistrar{descriptors: make(map[string]releasecontract.EffectDescriptor)}
}

func (r *registryReadinessRegistrar) Register(descriptor releasecontract.EffectDescriptor) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.descriptors[descriptor.ID]; exists {
		return fmt.Errorf("duplicate descriptor %s", descriptor.ID)
	}
	r.descriptors[descriptor.ID] = descriptor
	return nil
}

func (r *registryReadinessRegistrar) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.descriptors)
}

func loadRegistryReadinessContract(t *testing.T) (releasecontract.AuthoredContractV1, releasecontract.DeploymentProfile) {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve registry readiness test source")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(source), "../../../../../"))
	contract, err := releasecontract.Load(context.Background(), repoRoot, "config/release/contract.v1.json", "config/release/contract.schema.json")
	if err != nil {
		t.Fatalf("load release contract: %v", err)
	}
	for _, profile := range contract.Profiles {
		if profile.ID == "monolith" {
			return contract, profile
		}
	}
	t.Fatal("monolith profile missing")
	return releasecontract.AuthoredContractV1{}, releasecontract.DeploymentProfile{}
}

func TestWebsearchReadinessFallbackContract(t *testing.T) {
	guard := &registryReadinessGuard{}
	guard.allow.Store(true)
	contract, profile := loadRegistryReadinessContract(t)
	registrar := newRegistryReadinessRegistrar()
	authorities, err := releasecontract.NewRuntimeAuthorities(contract, profile, guard)
	if err != nil {
		t.Fatalf("build runtime authorities: %v", err)
	}
	readiness, err := newWebsearchReadiness(WebsearchRuntimeOptions{Guard: guard, Authorities: authorities, Effects: registrar})
	if err != nil {
		t.Fatalf("new websearch readiness: %v", err)
	}
	primary := &websearchProbeProvider{err: errors.New("primary failed")}
	fallback := &websearchProbeProvider{results: []mcp.WebSearchResult{{Title: "fallback"}}}
	tool := &WebsearchTool{
		providers:        map[string]mcp.WebSearchProvider{"primary": primary, "fallback": fallback},
		selectedProvider: "primary",
		fallbackChain:    []string{"fallback"},
		readiness:        readiness,
	}
	if _, err := tool.Execute(context.Background(), "golang"); err != nil {
		t.Fatalf("authorized fallback: %v", err)
	}
	if primary.calls != 1 || fallback.calls != 1 || guard.count() != 2 || registrar.count() != 1 {
		t.Fatalf("calls guard/providers/registrar = %d/%d/%d/%d, want 2/1/1/1", guard.count(), primary.calls, fallback.calls, registrar.count())
	}
	guard.allow.Store(false)
	if _, err := tool.Execute(context.Background(), "golang"); err == nil {
		t.Fatal("denied fallback returned nil error")
	}
	if fallback.calls != 1 || registrar.count() != 1 {
		t.Fatalf("denied fallback changed downstream/registrar counts = %d/%d", fallback.calls, registrar.count())
	}
}

type websearchProbeProvider struct {
	results []mcp.WebSearchResult
	err     error
	calls   int
}

func (p *websearchProbeProvider) Search(context.Context, string) ([]mcp.WebSearchResult, error) {
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	return p.results, nil
}
