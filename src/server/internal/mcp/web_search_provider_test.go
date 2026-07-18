package mcp

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"oblivious/server/internal/releasecontract"
)

func TestTavilyWebSearchReadinessDispatchContract(t *testing.T) {
	guard := &mcpReadinessGuard{}
	guard.allow.Store(true)
	contract, profile := loadMCPReadinessContract(t)
	authorities, err := releasecontract.NewRuntimeAuthorities(contract, profile, guard)
	if err != nil {
		t.Fatalf("build runtime authorities: %v", err)
	}
	registrar := newMCPReadinessRegistrar()
	transport := &countingTavilyTransport{}
	provider, err := NewAuthorizedTavilyWebSearchProvider(TavilyWebSearchProviderConfig{
		Endpoint: "https://tavily.example.test/search", APIKey: "secret", HTTPClient: &http.Client{Transport: transport}, ResultLimit: 1,
	}, WebSearchRuntimeOptions{Authorities: authorities, Guard: guard, Effects: registrar})
	if err != nil {
		t.Fatalf("construct authorized Tavily provider: %v", err)
	}
	for _, query := range []string{"first", "second"} {
		if _, err := provider.Search(t.Context(), query); err != nil {
			t.Fatalf("authorized search %q: %v", query, err)
		}
	}
	if registrar.count() != 1 || guard.count() != 2 || transport.count() != 2 {
		t.Fatalf("registration/guard/http counts = %d/%d/%d, want 1/2/2", registrar.count(), guard.count(), transport.count())
	}
	guard.allow.Store(false)
	if _, err := provider.Search(t.Context(), "expired"); err == nil {
		t.Fatal("expired Tavily search unexpectedly succeeded")
	}
	if transport.count() != 2 {
		t.Fatalf("denied Tavily transport count = %d, want unchanged 2", transport.count())
	}
	if _, err := NewAuthorizedTavilyWebSearchProvider(TavilyWebSearchProviderConfig{Endpoint: "https://tavily.example.test", APIKey: "secret"}, WebSearchRuntimeOptions{Authorities: authorities, Guard: guard, Effects: registrar}); err == nil {
		t.Fatal("duplicate Tavily descriptor construction unexpectedly succeeded")
	}
}

type countingTavilyTransport struct{ calls atomic.Int32 }

func (t *countingTavilyTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.calls.Add(1)
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"results":[]}`)), Header: make(http.Header)}, nil
}

func (t *countingTavilyTransport) count() int { return int(t.calls.Load()) }

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
