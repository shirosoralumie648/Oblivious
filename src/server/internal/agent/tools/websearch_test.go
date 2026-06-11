package tools

import (
	"context"
	"errors"
	"testing"

	"oblivious/server/internal/mcp"
)

type mockWebSearchProvider struct {
	name    string
	fail    bool
	results []mcp.WebSearchResult
}

func (m *mockWebSearchProvider) Search(ctx context.Context, query string) ([]mcp.WebSearchResult, error) {
	if m.fail {
		return nil, errors.New(m.name + " failed")
	}
	return m.results, nil
}

func TestWebsearchTool_PrimarySuccess(t *testing.T) {
	primaryProvider := &mockWebSearchProvider{
		name: "tavily",
		fail: false,
		results: []mcp.WebSearchResult{
			{Title: "Result 1", URL: "https://example.com/1", Snippet: "Snippet 1"},
		},
	}

	fallbackProvider := &mockWebSearchProvider{
		name: "brave",
		fail: false,
		results: []mcp.WebSearchResult{
			{Title: "Result 2", URL: "https://example.com/2", Snippet: "Snippet 2"},
		},
	}

	tool := &WebsearchTool{
		providers: map[string]mcp.WebSearchProvider{
			"tavily": primaryProvider,
			"brave":  fallbackProvider,
		},
		selectedProvider: "tavily",
		fallbackChain:    []string{"brave"},
	}

	results, err := tool.Execute(context.Background(), "test query")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Title != "Result 1" {
		t.Errorf("expected primary provider result, got %s", results[0].Title)
	}
}

func TestWebsearchTool_FallbackChain(t *testing.T) {
	primaryProvider := &mockWebSearchProvider{
		name: "tavily",
		fail: true,
	}

	fallback1 := &mockWebSearchProvider{
		name: "brave",
		fail: true,
	}

	fallback2 := &mockWebSearchProvider{
		name: "duckduckgo",
		fail: false,
		results: []mcp.WebSearchResult{
			{Title: "DDG Result", URL: "https://example.com/ddg", Snippet: "DDG Snippet"},
		},
	}

	tool := &WebsearchTool{
		providers: map[string]mcp.WebSearchProvider{
			"tavily":     primaryProvider,
			"brave":      fallback1,
			"duckduckgo": fallback2,
		},
		selectedProvider: "tavily",
		fallbackChain:    []string{"brave", "duckduckgo"},
	}

	results, err := tool.Execute(context.Background(), "test query")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Title != "DDG Result" {
		t.Errorf("expected fallback2 result, got %s", results[0].Title)
	}
}

func TestWebsearchTool_AllProvidersExhausted(t *testing.T) {
	primaryProvider := &mockWebSearchProvider{
		name: "tavily",
		fail: true,
	}

	fallbackProvider := &mockWebSearchProvider{
		name: "brave",
		fail: true,
	}

	tool := &WebsearchTool{
		providers: map[string]mcp.WebSearchProvider{
			"tavily": primaryProvider,
			"brave":  fallbackProvider,
		},
		selectedProvider: "tavily",
		fallbackChain:    []string{"brave"},
	}

	_, err := tool.Execute(context.Background(), "test query")
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err.Error() != "all websearch providers exhausted" {
		t.Errorf("expected exhausted error, got %v", err)
	}
}

func TestWebsearchTool_MissingProviderInMap(t *testing.T) {
	fallbackProvider := &mockWebSearchProvider{
		name: "brave",
		fail: false,
		results: []mcp.WebSearchResult{
			{Title: "Brave Result", URL: "https://example.com/brave", Snippet: "Brave"},
		},
	}

	tool := &WebsearchTool{
		providers: map[string]mcp.WebSearchProvider{
			"brave": fallbackProvider,
		},
		selectedProvider: "tavily",
		fallbackChain:    []string{"brave"},
	}

	results, err := tool.Execute(context.Background(), "test query")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].Title != "Brave Result" {
		t.Errorf("expected fallback result when primary missing, got %s", results[0].Title)
	}
}

func TestWebsearchTool_EmptyFallback(t *testing.T) {
	primaryProvider := &mockWebSearchProvider{
		name: "tavily",
		fail: false,
		results: []mcp.WebSearchResult{
			{Title: "Primary", URL: "https://example.com", Snippet: "Test"},
		},
	}

	tool := &WebsearchTool{
		providers: map[string]mcp.WebSearchProvider{
			"tavily": primaryProvider,
		},
		selectedProvider: "tavily",
		fallbackChain:    []string{},
	}

	results, err := tool.Execute(context.Background(), "test query")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestWebsearchTool_NoProviders(t *testing.T) {
	tool := &WebsearchTool{
		providers:        map[string]mcp.WebSearchProvider{},
		selectedProvider: "tavily",
		fallbackChain:    []string{"brave"},
	}

	_, err := tool.Execute(context.Background(), "test query")
	if err == nil {
		t.Fatal("expected error when no providers available")
	}
}
