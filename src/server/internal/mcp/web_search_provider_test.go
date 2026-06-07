package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTavilyWebSearchProviderEnablesBuiltinWithRealHTTPResult(t *testing.T) {
	var (
		called  bool
		payload map[string]any
	)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			t.Errorf("content type = %q, want application/json", r.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode request body: %v", err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"title":"Configured search","url":"https://search.example.test/result","content":"real provider response"}]}`))
	}))
	defer upstream.Close()

	provider := NewTavilyWebSearchProvider(TavilyWebSearchProviderConfig{
		Endpoint:    upstream.URL,
		APIKey:      "secret-key",
		ResultLimit: 2,
	})
	if provider == nil {
		t.Fatal("expected configured Tavily provider, got nil")
	}
	restore := SetWebSearchProviderForTest(provider)
	defer restore()

	tool, ok := GetBuiltinTool("web_search")
	if !ok {
		t.Fatal("web_search builtin not found")
	}
	result, err := tool.Execute(t.Context(), map[string]any{"query": "configured provider"})
	if err != nil {
		t.Fatalf("web_search returned error: %v", err)
	}
	if !called {
		t.Fatal("expected web_search to call configured HTTP provider")
	}
	if payload["api_key"] != "secret-key" || payload["query"] != "configured provider" || payload["max_results"] != float64(2) {
		t.Fatalf("unexpected Tavily request payload: %#v", payload)
	}
	if result == nil || result.IsError {
		t.Fatalf("web_search result = %+v, want provider-backed success", result)
	}
	lowerContent := strings.ToLower(result.Content)
	if !strings.Contains(result.Content, "Configured search") || !strings.Contains(result.Content, "real provider response") {
		t.Fatalf("web_search content = %q, want configured provider result", result.Content)
	}
	if strings.Contains(lowerContent, "placeholder") {
		t.Fatalf("web_search used placeholder output with configured provider: %q", result.Content)
	}
}

func TestTavilyWebSearchProviderFailClosedWithoutEndpointOrKey(t *testing.T) {
	if provider := NewTavilyWebSearchProvider(TavilyWebSearchProviderConfig{Endpoint: "https://search.example.test"}); provider != nil {
		t.Fatalf("expected missing api key to disable provider, got %#v", provider)
	}
	if provider := NewTavilyWebSearchProvider(TavilyWebSearchProviderConfig{APIKey: "secret-key"}); provider != nil {
		t.Fatalf("expected missing endpoint to disable provider, got %#v", provider)
	}

	restore := SetWebSearchProviderForTest(nil)
	defer restore()

	tool, ok := GetBuiltinTool("web_search")
	if !ok {
		t.Fatal("web_search builtin not found")
	}
	result, err := tool.Execute(t.Context(), map[string]any{"query": "disabled provider"})
	if err != nil {
		t.Fatalf("web_search returned error: %v", err)
	}
	if result == nil || !result.IsError {
		t.Fatalf("web_search result = %+v, want explicit disabled error", result)
	}
	lowerContent := strings.ToLower(result.Content)
	if !strings.Contains(lowerContent, "disabled") {
		t.Fatalf("web_search content = %q, want disabled message", result.Content)
	}
	if strings.Contains(lowerContent, "placeholder") {
		t.Fatalf("web_search returned placeholder output while disabled: %q", result.Content)
	}
}
