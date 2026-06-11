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

func TestBuildAgentWebSearchProviderSupportsMultiProviderRegistry(t *testing.T) {
	cases := []struct {
		name    string
		cfg     config.Config
		wantNil bool
	}{
		{
			name:    "brave with key",
			cfg:     config.Config{AgentWebSearchProvider: "brave", AgentWebSearchAPIKey: "brave-secret"},
			wantNil: false,
		},
		{
			name:    "brave without key fails closed",
			cfg:     config.Config{AgentWebSearchProvider: "brave"},
			wantNil: true,
		},
		{
			name:    "duckduckgo without key",
			cfg:     config.Config{AgentWebSearchProvider: "duckduckgo"},
			wantNil: false,
		},
		{
			name:    "google_cse with key and cse id",
			cfg:     config.Config{AgentWebSearchProvider: "google_cse", AgentWebSearchAPIKey: "cse-key", AgentWebSearchGoogleCSEID: "cse-id"},
			wantNil: false,
		},
		{
			name:    "google_cse missing cse id fails closed",
			cfg:     config.Config{AgentWebSearchProvider: "google_cse", AgentWebSearchAPIKey: "cse-key"},
			wantNil: true,
		},
		{
			name:    "fallback chain",
			cfg:     config.Config{AgentWebSearchProvider: "brave,duckduckgo", AgentWebSearchAPIKey: "brave-secret"},
			wantNil: false,
		},
		{
			name:    "unknown provider fails closed",
			cfg:     config.Config{AgentWebSearchProvider: "unknown_provider", AgentWebSearchAPIKey: "secret"},
			wantNil: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			provider := buildAgentWebSearchProvider(tc.cfg)
			if tc.wantNil && provider != nil {
				t.Fatalf("expected nil provider, got %#v", provider)
			}
			if !tc.wantNil && provider == nil {
				t.Fatal("expected configured provider, got nil")
			}
		})
	}
}

func TestBuildAgentWebSearchProviderMultiProviderSearches(t *testing.T) {
	upstream := httptest.NewServer(stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
		if got := r.Header.Get("X-Subscription-Token"); got != "brave-secret" {
			t.Errorf("brave auth header = %q, want brave-secret", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"web":{"results":[{"title":"Brave wired","url":"https://brave.example.test","description":"multi provider"}]}}`))
	}))
	defer upstream.Close()

	provider := buildAgentWebSearchProvider(config.Config{
		AgentWebSearchProvider: "brave",
		AgentWebSearchAPIKey:   "brave-secret",
		AgentWebSearchEndpoint: upstream.URL,
	})
	if provider == nil {
		t.Fatal("expected brave provider, got nil")
	}
	results, err := provider.Search(t.Context(), "multi provider search")
	if err != nil {
		t.Fatalf("brave search returned error: %v", err)
	}
	if len(results) != 1 || results[0].Title != "Brave wired" {
		t.Fatalf("brave search results = %+v, want Brave wired", results)
	}
}
