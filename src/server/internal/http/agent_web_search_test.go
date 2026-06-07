package http

import (
	"encoding/json"
	stdhttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oblivious/server/internal/config"
	"oblivious/server/internal/mcp"
)

func TestBuildAgentWebSearchProviderFromConfigEnablesWebSearch(t *testing.T) {
	var called bool
	upstream := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		called = true
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode request body: %v", err)
			return
		}
		if payload["api_key"] != "cfg-secret" || payload["query"] != "configured search" || payload["max_results"] != float64(1) {
			t.Errorf("unexpected web search payload: %#v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"title":"Config wired search","url":"https://search.example.test/cfg","snippet":"configured through service config"}]}`))
	}))
	defer upstream.Close()

	provider := buildAgentWebSearchProvider(config.Config{
		AgentWebSearchProvider:    "tavily",
		AgentWebSearchEndpoint:    upstream.URL,
		AgentWebSearchAPIKey:      "cfg-secret",
		AgentWebSearchResultLimit: 1,
	})
	if provider == nil {
		t.Fatal("expected configured web search provider, got nil")
	}
	restore := mcp.SetWebSearchProviderForTest(provider)
	defer restore()

	tool, ok := mcp.GetBuiltinTool("web_search")
	if !ok {
		t.Fatal("web_search builtin not found")
	}
	result, err := tool.Execute(t.Context(), map[string]any{"query": "configured search"})
	if err != nil {
		t.Fatalf("web_search returned error: %v", err)
	}
	if !called {
		t.Fatal("expected config-backed web_search to call upstream provider")
	}
	if result == nil || result.IsError || !strings.Contains(result.Content, "Config wired search") {
		t.Fatalf("web_search result = %+v, want config-backed provider content", result)
	}
	if strings.Contains(strings.ToLower(result.Content), "placeholder") {
		t.Fatalf("web_search used placeholder output with configured provider: %q", result.Content)
	}
}

func TestBuildAgentWebSearchProviderFailClosedWhenIncomplete(t *testing.T) {
	cases := []config.Config{
		{AgentWebSearchProvider: "tavily", AgentWebSearchEndpoint: "https://search.example.test"},
		{AgentWebSearchProvider: "tavily", AgentWebSearchAPIKey: "secret"},
		{AgentWebSearchProvider: "", AgentWebSearchEndpoint: "https://search.example.test", AgentWebSearchAPIKey: "secret"},
		{AgentWebSearchProvider: "unsupported", AgentWebSearchEndpoint: "https://search.example.test", AgentWebSearchAPIKey: "secret"},
	}
	for _, cfg := range cases {
		if provider := buildAgentWebSearchProvider(cfg); provider != nil {
			t.Fatalf("expected incomplete config to disable provider, got %#v for cfg=%+v", provider, cfg)
		}
	}

	restore := mcp.SetWebSearchProviderForTest(nil)
	defer restore()

	tool, ok := mcp.GetBuiltinTool("web_search")
	if !ok {
		t.Fatal("web_search builtin not found")
	}
	result, err := tool.Execute(t.Context(), map[string]any{"query": "disabled config"})
	if err != nil {
		t.Fatalf("web_search returned error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("web_search result = %+v, want explicit disabled result", result)
	}
	lowerContent := strings.ToLower(result.Content)
	if !strings.Contains(lowerContent, "disabled") || strings.Contains(lowerContent, "placeholder") {
		t.Fatalf("web_search disabled content = %q, want disabled without placeholder", result.Content)
	}
}
