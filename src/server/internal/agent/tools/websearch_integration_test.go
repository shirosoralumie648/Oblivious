package tools

import (
	"context"
	"os"
	"testing"

	"oblivious/server/internal/config"
	"oblivious/server/internal/mcp"
)

func TestWebsearchTool_IntegrationWithConfig(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("SESSION_SECRET", "test-secret")
	os.Setenv("AGENT_WEB_SEARCH_PROVIDER", "tavily")
	os.Setenv("AGENT_WEB_SEARCH_FALLBACK", "brave,duckduckgo")
	os.Setenv("AGENT_WEB_SEARCH_API_KEY", "test-key")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("SESSION_SECRET")
		os.Unsetenv("AGENT_WEB_SEARCH_PROVIDER")
		os.Unsetenv("AGENT_WEB_SEARCH_FALLBACK")
		os.Unsetenv("AGENT_WEB_SEARCH_API_KEY")
	}()

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("config.Load() failed: %v", err)
	}

	if cfg.AgentWebSearchProvider != "tavily" {
		t.Errorf("expected provider tavily, got %s", cfg.AgentWebSearchProvider)
	}

	if len(cfg.AgentWebSearchFallback) != 2 {
		t.Errorf("expected 2 fallback providers, got %d", len(cfg.AgentWebSearchFallback))
	}

	tool, err := NewWebsearchTool(
		cfg.AgentWebSearchProvider,
		cfg.AgentWebSearchFallback,
		cfg.AgentWebSearchAPIKey,
		cfg.AgentWebSearchEndpoint,
		cfg.AgentWebSearchGoogleCSEID,
	)

	if err != nil {
		t.Fatalf("NewWebsearchTool() failed: %v", err)
	}

	if tool.selectedProvider != "tavily" {
		t.Errorf("expected selectedProvider tavily, got %s", tool.selectedProvider)
	}

	if len(tool.fallbackChain) != 2 {
		t.Errorf("expected 2 fallback providers, got %d", len(tool.fallbackChain))
	}

	if len(tool.providers) < 1 {
		t.Error("expected at least 1 provider initialized")
	}
}

func TestWebsearchTool_IntegrationWithMockProviders(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("SESSION_SECRET", "test-secret")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("SESSION_SECRET")
	}()

	primary := &mockWebSearchProvider{name: "tavily", fail: true}
	fallback1 := &mockWebSearchProvider{name: "brave", fail: true}
	fallback2 := &mockWebSearchProvider{
		name: "duckduckgo",
		fail: false,
		results: []mcp.WebSearchResult{
			{Title: "Success", URL: "https://example.com", Snippet: "Found"},
		},
	}

	tool := &WebsearchTool{
		providers: map[string]mcp.WebSearchProvider{
			"tavily":     primary,
			"brave":      fallback1,
			"duckduckgo": fallback2,
		},
		selectedProvider: "tavily",
		fallbackChain:    []string{"brave", "duckduckgo"},
	}

	ctx := context.Background()
	_, err := tool.Execute(ctx, "test")

	if err != nil {
		t.Errorf("expected fallback to succeed, got error: %v", err)
	}
}

func TestWebsearchTool_DefaultProvider(t *testing.T) {
	tool, err := NewWebsearchTool("", []string{}, "", "", "")
	if err == nil {
		t.Log("Tool created with default provider successfully")
		if tool.selectedProvider != "tavily" {
			t.Errorf("expected default provider tavily, got %s", tool.selectedProvider)
		}
	}
}

func TestWebsearchTool_MultipleProvidersChain(t *testing.T) {
	providers := []string{"brave", "duckduckgo", "searxng"}

	tool, err := NewWebsearchTool("tavily", providers, "test-key", "", "")
	if err != nil {
		t.Logf("Tool initialization returned error (expected if no valid providers): %v", err)
		return
	}

	if len(tool.fallbackChain) != len(providers) {
		t.Errorf("expected %d fallback providers, got %d", len(providers), len(tool.fallbackChain))
	}

	for i, expected := range providers {
		if tool.fallbackChain[i] != expected {
			t.Errorf("fallbackChain[%d]: expected %s, got %s", i, expected, tool.fallbackChain[i])
		}
	}
}
