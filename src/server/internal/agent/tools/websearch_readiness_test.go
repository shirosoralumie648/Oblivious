package tools

import (
	"context"
	"errors"
	"testing"

	"oblivious/server/internal/mcp"
	"oblivious/server/internal/releasecontract"
)

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
