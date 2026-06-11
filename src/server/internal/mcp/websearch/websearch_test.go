package websearch

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"oblivious/server/internal/mcp"
)

func writeJSON(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("encode fake response: %v", err)
	}
}

func assertResults(t *testing.T, results []mcp.WebSearchResult, wantTitle, wantURL, wantSnippet string) {
	t.Helper()
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0].Title != wantTitle {
		t.Fatalf("title = %q, want %q", results[0].Title, wantTitle)
	}
	if results[0].URL != wantURL {
		t.Fatalf("url = %q, want %q", results[0].URL, wantURL)
	}
	if results[0].Snippet != wantSnippet {
		t.Fatalf("snippet = %q, want %q", results[0].Snippet, wantSnippet)
	}
}

func TestTavilySearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer tavily-key" {
			t.Errorf("Authorization = %q", got)
		}
		var body struct {
			Query string `json:"query"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Query != "golang" {
			t.Errorf("query = %q, err = %v", body.Query, err)
		}
		writeJSON(t, w, map[string]any{
			"results": []map[string]string{{"title": "Go", "url": "https://go.dev", "content": "The Go programming language"}},
		})
	}))
	defer server.Close()

	provider, err := NewTavily("tavily-key", server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	results, err := provider.Search(context.Background(), "golang")
	if err != nil {
		t.Fatal(err)
	}
	assertResults(t, results, "Go", "https://go.dev", "The Go programming language")
}

func TestBraveSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Subscription-Token"); got != "brave-key" {
			t.Errorf("X-Subscription-Token = %q", got)
		}
		if got := r.URL.Query().Get("q"); got != "golang" {
			t.Errorf("q = %q", got)
		}
		writeJSON(t, w, map[string]any{
			"web": map[string]any{
				"results": []map[string]string{{"title": "Go", "url": "https://go.dev", "description": "Go language"}},
			},
		})
	}))
	defer server.Close()

	provider, err := NewBrave("brave-key", server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	results, err := provider.Search(context.Background(), "golang")
	if err != nil {
		t.Fatal(err)
	}
	assertResults(t, results, "Go", "https://go.dev", "Go language")
}

func TestSerperSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-API-KEY"); got != "serper-key" {
			t.Errorf("X-API-KEY = %q", got)
		}
		writeJSON(t, w, map[string]any{
			"organic": []map[string]string{{"title": "Go", "link": "https://go.dev", "snippet": "Go language"}},
		})
	}))
	defer server.Close()

	provider, err := NewSerper("serper-key", server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	results, err := provider.Search(context.Background(), "golang")
	if err != nil {
		t.Fatal(err)
	}
	assertResults(t, results, "Go", "https://go.dev", "Go language")
}

func TestSerpAPISearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("api_key") != "serpapi-key" || query.Get("engine") != "google" || query.Get("q") != "golang" {
			t.Errorf("unexpected query: %v", query)
		}
		writeJSON(t, w, map[string]any{
			"organic_results": []map[string]string{{"title": "Go", "link": "https://go.dev", "snippet": "Go language"}},
		})
	}))
	defer server.Close()

	provider, err := NewSerpAPI("serpapi-key", server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	results, err := provider.Search(context.Background(), "golang")
	if err != nil {
		t.Fatal(err)
	}
	assertResults(t, results, "Go", "https://go.dev", "Go language")
}

func TestBingSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Ocp-Apim-Subscription-Key"); got != "bing-key" {
			t.Errorf("Ocp-Apim-Subscription-Key = %q", got)
		}
		writeJSON(t, w, map[string]any{
			"webPages": map[string]any{
				"value": []map[string]string{{"name": "Go", "url": "https://go.dev", "snippet": "Go language"}},
			},
		})
	}))
	defer server.Close()

	provider, err := NewBing("bing-key", server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	results, err := provider.Search(context.Background(), "golang")
	if err != nil {
		t.Fatal(err)
	}
	assertResults(t, results, "Go", "https://go.dev", "Go language")
}

func TestGoogleCSESearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("key") != "google-key" || query.Get("cx") != "cse-id" || query.Get("q") != "golang" {
			t.Errorf("unexpected query: %v", query)
		}
		writeJSON(t, w, map[string]any{
			"items": []map[string]string{{"title": "Go", "link": "https://go.dev", "snippet": "Go language"}},
		})
	}))
	defer server.Close()

	provider, err := NewGoogleCSE("google-key", "cse-id", server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	results, err := provider.Search(context.Background(), "golang")
	if err != nil {
		t.Fatal(err)
	}
	assertResults(t, results, "Go", "https://go.dev", "Go language")
}

func TestGoogleCSERequiresEngineID(t *testing.T) {
	if _, err := NewGoogleCSE("key", "", "", nil); err == nil {
		t.Fatal("expected error for missing CSE ID")
	}
}

func TestDuckDuckGoSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("q") != "golang" || query.Get("format") != "json" {
			t.Errorf("unexpected query: %v", query)
		}
		writeJSON(t, w, map[string]any{
			"Heading":      "Go",
			"AbstractText": "Go is a language",
			"AbstractURL":  "https://go.dev",
			"RelatedTopics": []map[string]any{
				{"Text": "Go FAQ", "FirstURL": "https://go.dev/doc/faq"},
				{"Topics": []map[string]any{{"Text": "Nested", "FirstURL": "https://go.dev/nested"}}},
			},
		})
	}))
	defer server.Close()

	provider, err := NewDuckDuckGo(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	results, err := provider.Search(context.Background(), "golang")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3 (abstract + topic + nested topic)", len(results))
	}
	assertResults(t, results, "Go", "https://go.dev", "Go is a language")
}

func TestSearXNGSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/search") {
			t.Errorf("path = %q, want /search suffix", r.URL.Path)
		}
		query := r.URL.Query()
		if query.Get("q") != "golang" || query.Get("format") != "json" {
			t.Errorf("unexpected query: %v", query)
		}
		writeJSON(t, w, map[string]any{
			"results": []map[string]string{{"title": "Go", "url": "https://go.dev", "content": "Go language"}},
		})
	}))
	defer server.Close()

	provider, err := NewSearXNG(server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	results, err := provider.Search(context.Background(), "golang")
	if err != nil {
		t.Fatal(err)
	}
	assertResults(t, results, "Go", "https://go.dev", "Go language")
}

func TestSearXNGRequiresEndpoint(t *testing.T) {
	if _, err := NewSearXNG("", nil); err == nil {
		t.Fatal("expected error for missing searxng endpoint")
	}
}

func TestExaSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "exa-key" {
			t.Errorf("x-api-key = %q", got)
		}
		writeJSON(t, w, map[string]any{
			"results": []map[string]string{{"title": "Go", "url": "https://go.dev", "text": "Go language"}},
		})
	}))
	defer server.Close()

	provider, err := NewExa("exa-key", server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	results, err := provider.Search(context.Background(), "golang")
	if err != nil {
		t.Fatal(err)
	}
	assertResults(t, results, "Go", "https://go.dev", "Go language")
}

func TestYouSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-API-Key"); got != "you-key" {
			t.Errorf("X-API-Key = %q", got)
		}
		if got := r.URL.Query().Get("query"); got != "golang" {
			t.Errorf("query = %q", got)
		}
		writeJSON(t, w, map[string]any{
			"hits": []map[string]any{{"title": "Go", "url": "https://go.dev", "description": "Go language"}},
		})
	}))
	defer server.Close()

	provider, err := NewYou("you-key", server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	results, err := provider.Search(context.Background(), "golang")
	if err != nil {
		t.Fatal(err)
	}
	assertResults(t, results, "Go", "https://go.dev", "Go language")
}

func TestKagiSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bot kagi-key" {
			t.Errorf("Authorization = %q", got)
		}
		writeJSON(t, w, map[string]any{
			"data": []map[string]any{
				{"t": 0, "title": "Go", "url": "https://go.dev", "snippet": "Go language"},
				{"t": 1, "list": []string{"related"}},
			},
		})
	}))
	defer server.Close()

	provider, err := NewKagi("kagi-key", server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	results, err := provider.Search(context.Background(), "golang")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1 (non-result rows skipped)", len(results))
	}
	assertResults(t, results, "Go", "https://go.dev", "Go language")
}

func TestMojeekSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("api_key") != "mojeek-key" || query.Get("fmt") != "json" {
			t.Errorf("unexpected query: %v", query)
		}
		writeJSON(t, w, map[string]any{
			"response": map[string]any{
				"results": []map[string]string{{"title": "Go", "url": "https://go.dev", "desc": "Go language"}},
			},
		})
	}))
	defer server.Close()

	provider, err := NewMojeek("mojeek-key", server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	results, err := provider.Search(context.Background(), "golang")
	if err != nil {
		t.Fatal(err)
	}
	assertResults(t, results, "Go", "https://go.dev", "Go language")
}

func TestJinaSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer jina-key" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.URL.Query().Get("q"); got != "golang" {
			t.Errorf("q = %q", got)
		}
		writeJSON(t, w, map[string]any{
			"data": []map[string]string{{"title": "Go", "url": "https://go.dev", "description": "Go language"}},
		})
	}))
	defer server.Close()

	provider, err := NewJina("jina-key", server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	results, err := provider.Search(context.Background(), "golang")
	if err != nil {
		t.Fatal(err)
	}
	assertResults(t, results, "Go", "https://go.dev", "Go language")
}

func TestBochaSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer bocha-key" {
			t.Errorf("Authorization = %q", got)
		}
		writeJSON(t, w, map[string]any{
			"data": map[string]any{
				"webPages": map[string]any{
					"value": []map[string]string{{"name": "Go", "url": "https://go.dev", "snippet": "Go language"}},
				},
			},
		})
	}))
	defer server.Close()

	provider, err := NewBocha("bocha-key", server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	results, err := provider.Search(context.Background(), "golang")
	if err != nil {
		t.Fatal(err)
	}
	assertResults(t, results, "Go", "https://go.dev", "Go language")
}

func TestBaiduSearch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer baidu-key" {
			t.Errorf("Authorization = %q", got)
		}
		var body struct {
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Messages) != 1 || body.Messages[0].Content != "golang" {
			t.Errorf("unexpected body: %+v, err=%v", body, err)
		}
		writeJSON(t, w, map[string]any{
			"references": []map[string]string{{"title": "Go", "url": "https://go.dev", "content": "Go language"}},
		})
	}))
	defer server.Close()

	provider, err := NewBaidu("baidu-key", server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	results, err := provider.Search(context.Background(), "golang")
	if err != nil {
		t.Fatal(err)
	}
	assertResults(t, results, "Go", "https://go.dev", "Go language")
}

func TestProvidersRejectEmptyQuery(t *testing.T) {
	provider, err := NewTavily("key", "http://unused.invalid", nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.Search(context.Background(), "   "); err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestProvidersSurfaceNon200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer server.Close()

	provider, err := NewBrave("key", server.URL, server.Client())
	if err != nil {
		t.Fatal(err)
	}
	_, err = provider.Search(context.Background(), "golang")
	if err == nil || !strings.Contains(err.Error(), "429") {
		t.Fatalf("expected 429 error, got %v", err)
	}
}

func TestProvidersRequireAPIKey(t *testing.T) {
	constructors := map[string]func() error{
		"tavily":  func() error { _, err := NewTavily("", "", nil); return err },
		"brave":   func() error { _, err := NewBrave("", "", nil); return err },
		"serper":  func() error { _, err := NewSerper("", "", nil); return err },
		"serpapi": func() error { _, err := NewSerpAPI("", "", nil); return err },
		"bing":    func() error { _, err := NewBing("", "", nil); return err },
		"exa":     func() error { _, err := NewExa("", "", nil); return err },
		"you":     func() error { _, err := NewYou("", "", nil); return err },
		"kagi":    func() error { _, err := NewKagi("", "", nil); return err },
		"mojeek":  func() error { _, err := NewMojeek("", "", nil); return err },
		"jina":    func() error { _, err := NewJina("", "", nil); return err },
		"bocha":   func() error { _, err := NewBocha("", "", nil); return err },
		"baidu":   func() error { _, err := NewBaidu("", "", nil); return err },
	}
	for name, construct := range constructors {
		if err := construct(); err == nil {
			t.Fatalf("%s: expected missing-key error", name)
		}
	}
}

type stubProvider struct {
	results []mcp.WebSearchResult
	err     error
	calls   int
}

func (s *stubProvider) Search(_ context.Context, _ string) ([]mcp.WebSearchResult, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.results, nil
}

func TestChainFallsBackOnError(t *testing.T) {
	failing := &stubProvider{err: errors.New("boom")}
	succeeding := &stubProvider{results: []mcp.WebSearchResult{{Title: "Go", URL: "https://go.dev", Snippet: "Go"}}}
	chain := &Chain{Providers: []mcp.WebSearchProvider{failing, succeeding}}

	results, err := chain.Search(context.Background(), "golang")
	if err != nil {
		t.Fatal(err)
	}
	if failing.calls != 1 || succeeding.calls != 1 {
		t.Fatalf("calls = %d/%d, want 1/1", failing.calls, succeeding.calls)
	}
	assertResults(t, results, "Go", "https://go.dev", "Go")
}

func TestChainReportsAllFailures(t *testing.T) {
	chain := &Chain{Providers: []mcp.WebSearchProvider{
		&stubProvider{err: errors.New("first failed")},
		&stubProvider{err: errors.New("second failed")},
	}}
	_, err := chain.Search(context.Background(), "golang")
	if err == nil || !strings.Contains(err.Error(), "first failed") || !strings.Contains(err.Error(), "second failed") {
		t.Fatalf("expected joined errors, got %v", err)
	}
}

func TestNewProviderFromConfigSelectsProviders(t *testing.T) {
	cases := []Config{
		{Provider: "tavily", APIKey: "k"},
		{Provider: "brave", APIKey: "k"},
		{Provider: "serper", APIKey: "k"},
		{Provider: "serpapi", APIKey: "k"},
		{Provider: "bing", APIKey: "k"},
		{Provider: "google_cse", APIKey: "k", GoogleCSEID: "cx"},
		{Provider: "duckduckgo"},
		{Provider: "searxng", Endpoint: "http://searx.local"},
		{Provider: "exa", APIKey: "k"},
		{Provider: "you", APIKey: "k"},
		{Provider: "kagi", APIKey: "k"},
		{Provider: "mojeek", APIKey: "k"},
		{Provider: "jina", APIKey: "k"},
		{Provider: "bocha", APIKey: "k"},
		{Provider: "baidu", APIKey: "k"},
	}
	if len(cases) != len(ProviderNames()) {
		t.Fatalf("test covers %d providers, ProviderNames lists %d", len(cases), len(ProviderNames()))
	}
	for _, cfg := range cases {
		provider, err := NewProviderFromConfig(cfg)
		if err != nil {
			t.Fatalf("%s: %v", cfg.Provider, err)
		}
		if provider == nil {
			t.Fatalf("%s: nil provider", cfg.Provider)
		}
	}
}

func TestNewProviderFromConfigUnknownProvider(t *testing.T) {
	if _, err := NewProviderFromConfig(Config{Provider: "altavista"}); err == nil {
		t.Fatal("expected unknown provider error")
	}
}

func TestNewProviderFromConfigEmptyName(t *testing.T) {
	if _, err := NewProviderFromConfig(Config{}); err == nil {
		t.Fatal("expected error for empty provider name")
	}
}

func TestNewProviderFromConfigBuildsChain(t *testing.T) {
	provider, err := NewProviderFromConfig(Config{Provider: "duckduckgo, searxng", Endpoint: "http://searx.local"})
	if err != nil {
		t.Fatal(err)
	}
	chain, ok := provider.(*Chain)
	if !ok {
		t.Fatalf("provider type = %T, want *Chain", provider)
	}
	if len(chain.Providers) != 2 {
		t.Fatalf("chain length = %d, want 2", len(chain.Providers))
	}
}

func TestNewProviderFromConfigChainMemberError(t *testing.T) {
	_, err := NewProviderFromConfig(Config{Provider: "duckduckgo,tavily"})
	if err == nil || !strings.Contains(err.Error(), "tavily") {
		t.Fatalf("expected chain member error mentioning tavily, got %v", err)
	}
}

func TestNewFromEnv(t *testing.T) {
	t.Setenv("OBLIVIOUS_WEBSEARCH_PROVIDER", "")
	t.Setenv("OBLIVIOUS_WEBSEARCH_API_KEY", "")
	t.Setenv("OBLIVIOUS_WEBSEARCH_ENDPOINT", "")
	if _, ok := NewFromEnv(); ok {
		t.Fatal("expected unconfigured when provider env is empty")
	}

	t.Setenv("OBLIVIOUS_WEBSEARCH_PROVIDER", "tavily")
	if _, ok := NewFromEnv(); ok {
		t.Fatal("expected unconfigured when tavily has no key")
	}

	t.Setenv("OBLIVIOUS_WEBSEARCH_API_KEY", "key")
	provider, ok := NewFromEnv()
	if !ok || provider == nil {
		t.Fatal("expected configured tavily provider")
	}

	t.Setenv("OBLIVIOUS_WEBSEARCH_PROVIDER", "google_cse")
	t.Setenv("GOOGLE_CSE_ID", "cx-from-fallback")
	provider, ok = NewFromEnv()
	if !ok || provider == nil {
		t.Fatal("expected configured google_cse provider via GOOGLE_CSE_ID fallback")
	}
}

// Compile-time interface checks for every provider.
var (
	_ mcp.WebSearchProvider = (*Tavily)(nil)
	_ mcp.WebSearchProvider = (*Brave)(nil)
	_ mcp.WebSearchProvider = (*Serper)(nil)
	_ mcp.WebSearchProvider = (*SerpAPI)(nil)
	_ mcp.WebSearchProvider = (*Bing)(nil)
	_ mcp.WebSearchProvider = (*GoogleCSE)(nil)
	_ mcp.WebSearchProvider = (*DuckDuckGo)(nil)
	_ mcp.WebSearchProvider = (*SearXNG)(nil)
	_ mcp.WebSearchProvider = (*Exa)(nil)
	_ mcp.WebSearchProvider = (*You)(nil)
	_ mcp.WebSearchProvider = (*Kagi)(nil)
	_ mcp.WebSearchProvider = (*Mojeek)(nil)
	_ mcp.WebSearchProvider = (*Jina)(nil)
	_ mcp.WebSearchProvider = (*Bocha)(nil)
	_ mcp.WebSearchProvider = (*Baidu)(nil)
	_ mcp.WebSearchProvider = (*Chain)(nil)
)
