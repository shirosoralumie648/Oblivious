// Package websearch provides web search provider clients for the builtin
// web_search tool. Every client satisfies mcp.WebSearchProvider and is driven
// by explicit configuration passed to its constructor — credentials are never
// read from the environment inside a client (see NewFromEnv for env wiring).
package websearch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"oblivious/server/internal/mcp"
)

// Config selects and configures a provider for NewProviderFromConfig.
type Config struct {
	// Provider is the provider name (e.g. "tavily"). A comma-separated list
	// (e.g. "tavily,brave,duckduckgo") builds a fallback Chain that tries
	// each provider in order until one succeeds.
	Provider string
	// APIKey is the provider credential. Some providers (duckduckgo,
	// searxng) work without one.
	APIKey string
	// Endpoint overrides the provider's default API endpoint. Required for
	// searxng (self-hosted); useful for tests and proxies elsewhere.
	Endpoint string
	// GoogleCSEID is the Programmable Search Engine ID, required by the
	// google_cse provider.
	GoogleCSEID string
	// Client overrides the default HTTP client (15s timeout).
	Client *http.Client
}

// ProviderNames lists every supported single-provider name.
func ProviderNames() []string {
	return []string{
		"baidu", "bing", "bocha", "brave", "duckduckgo", "exa", "google_cse",
		"jina", "kagi", "mojeek", "searxng", "serpapi", "serper", "tavily", "you",
	}
}

// NewProviderFromConfig builds a provider (or fallback chain) from cfg.
func NewProviderFromConfig(cfg Config) (mcp.WebSearchProvider, error) {
	name := strings.ToLower(strings.TrimSpace(cfg.Provider))
	if name == "" {
		return nil, errors.New("websearch: provider name is required")
	}
	if strings.Contains(name, ",") {
		var providers []mcp.WebSearchProvider
		for _, part := range strings.Split(name, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			sub := cfg
			sub.Provider = part
			provider, err := NewProviderFromConfig(sub)
			if err != nil {
				return nil, fmt.Errorf("websearch: chain member %q: %w", part, err)
			}
			providers = append(providers, provider)
		}
		if len(providers) == 0 {
			return nil, errors.New("websearch: provider chain is empty")
		}
		return &Chain{Providers: providers}, nil
	}

	switch name {
	case "tavily":
		return NewTavily(cfg.APIKey, cfg.Endpoint, cfg.Client)
	case "brave":
		return NewBrave(cfg.APIKey, cfg.Endpoint, cfg.Client)
	case "serper":
		return NewSerper(cfg.APIKey, cfg.Endpoint, cfg.Client)
	case "serpapi":
		return NewSerpAPI(cfg.APIKey, cfg.Endpoint, cfg.Client)
	case "bing":
		return NewBing(cfg.APIKey, cfg.Endpoint, cfg.Client)
	case "google_cse":
		return NewGoogleCSE(cfg.APIKey, cfg.GoogleCSEID, cfg.Endpoint, cfg.Client)
	case "duckduckgo":
		return NewDuckDuckGo(cfg.Endpoint, cfg.Client)
	case "searxng":
		return NewSearXNG(cfg.Endpoint, cfg.Client)
	case "exa":
		return NewExa(cfg.APIKey, cfg.Endpoint, cfg.Client)
	case "you":
		return NewYou(cfg.APIKey, cfg.Endpoint, cfg.Client)
	case "kagi":
		return NewKagi(cfg.APIKey, cfg.Endpoint, cfg.Client)
	case "mojeek":
		return NewMojeek(cfg.APIKey, cfg.Endpoint, cfg.Client)
	case "jina":
		return NewJina(cfg.APIKey, cfg.Endpoint, cfg.Client)
	case "bocha":
		return NewBocha(cfg.APIKey, cfg.Endpoint, cfg.Client)
	case "baidu":
		return NewBaidu(cfg.APIKey, cfg.Endpoint, cfg.Client)
	default:
		return nil, fmt.Errorf("websearch: unknown provider %q", cfg.Provider)
	}
}

// NewFromEnv builds a provider from OBLIVIOUS_WEBSEARCH_* environment
// variables. It returns (nil, false) when no provider is configured or the
// configuration is invalid.
//
//	OBLIVIOUS_WEBSEARCH_PROVIDER       provider name or comma-separated chain
//	OBLIVIOUS_WEBSEARCH_API_KEY        credential for the selected provider(s)
//	OBLIVIOUS_WEBSEARCH_ENDPOINT       endpoint override (required for searxng)
//	OBLIVIOUS_WEBSEARCH_GOOGLE_CSE_ID  Programmable Search Engine ID
//	GOOGLE_CSE_ID                      fallback for the above
func NewFromEnv() (mcp.WebSearchProvider, bool) {
	name := strings.TrimSpace(os.Getenv("OBLIVIOUS_WEBSEARCH_PROVIDER"))
	if name == "" {
		return nil, false
	}
	cseID := strings.TrimSpace(os.Getenv("OBLIVIOUS_WEBSEARCH_GOOGLE_CSE_ID"))
	if cseID == "" {
		cseID = strings.TrimSpace(os.Getenv("GOOGLE_CSE_ID"))
	}
	provider, err := NewProviderFromConfig(Config{
		Provider:    name,
		APIKey:      strings.TrimSpace(os.Getenv("OBLIVIOUS_WEBSEARCH_API_KEY")),
		Endpoint:    strings.TrimSpace(os.Getenv("OBLIVIOUS_WEBSEARCH_ENDPOINT")),
		GoogleCSEID: cseID,
	})
	if err != nil {
		return nil, false
	}
	return provider, true
}

// Chain tries providers in order, returning the first successful response.
type Chain struct {
	Providers []mcp.WebSearchProvider
}

func (c *Chain) Search(ctx context.Context, query string) ([]mcp.WebSearchResult, error) {
	if len(c.Providers) == 0 {
		return nil, errors.New("websearch: chain has no providers")
	}
	var errs []error
	for _, provider := range c.Providers {
		results, err := provider.Search(ctx, query)
		if err == nil {
			return results, nil
		}
		errs = append(errs, err)
	}
	return nil, fmt.Errorf("websearch: all chain providers failed: %w", errors.Join(errs...))
}

const defaultTimeout = 15 * time.Second

const maxResponseBytes = 4 << 20

func defaultClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return &http.Client{Timeout: defaultTimeout}
}

func pickEndpoint(override, fallback string) string {
	override = strings.TrimSpace(override)
	if override != "" {
		return strings.TrimRight(override, "/")
	}
	return fallback
}

func truncateForError(s string, limit int) string {
	s = strings.TrimSpace(s)
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "..."
}

// doJSON executes req, enforces a 200 status, and decodes the body into out.
func doJSON(client *http.Client, req *http.Request, out any) error {
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("websearch: request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("websearch: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("websearch: %s returned status %d: %s", req.URL.Host, resp.StatusCode, truncateForError(string(body), 200))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("websearch: decode response from %s: %w", req.URL.Host, err)
	}
	return nil
}

func postJSON(ctx context.Context, client *http.Client, endpoint string, headers map[string]string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("websearch: encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("websearch: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return doJSON(client, req, out)
}

func getJSON(ctx context.Context, client *http.Client, rawURL string, headers map[string]string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("websearch: build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	return doJSON(client, req, out)
}

func validateQuery(query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", errors.New("websearch: query is required")
	}
	return query, nil
}

func requireKey(provider, key string) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", fmt.Errorf("websearch: %s requires an API key", provider)
	}
	return key, nil
}
