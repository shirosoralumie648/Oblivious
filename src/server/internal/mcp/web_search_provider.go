package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"oblivious/server/internal/releasecontract"
)

// WebSearchRuntimeOptions is the startup-owned authority carrier for a
// web-search provider. A provider may not reconstruct release state or accept
// capability authority from request/config data.
type WebSearchRuntimeOptions struct {
	Authorities releasecontract.RuntimeAuthorities
	Guard       releasecontract.Guard
	Effects     releasecontract.EffectRegistrar
}

type TavilyWebSearchProviderConfig struct {
	Endpoint    string
	APIKey      string
	ResultLimit int
	HTTPClient  *http.Client
}

type tavilyWebSearchProvider struct {
	endpoint    string
	apiKey      string
	resultLimit int
	client      *http.Client
	readiness   *webSearchReadiness
}

type webSearchReadiness struct {
	authorities releasecontract.RuntimeAuthorities
	capability  releasecontract.CapabilityID
}

func newWebSearchReadiness(options WebSearchRuntimeOptions, descriptorID, owner string) (*webSearchReadiness, error) {
	if options.Guard == nil || options.Effects == nil || !options.Authorities.Valid() {
		return nil, &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessUnavailable, Field: "mcp.webSearch"}
	}
	capability, err := options.Authorities.CapabilityBindings.Resolve(releasecontract.EffectToolWebSearch)
	if err != nil {
		return nil, err
	}
	if err := options.Effects.Register(releasecontract.EffectDescriptor{
		ID: descriptorID, CapabilityID: string(capability), Boundary: releasecontract.BoundaryOutbound, Owner: owner,
	}); err != nil {
		return nil, err
	}
	return &webSearchReadiness{authorities: options.Authorities, capability: capability}, nil
}

func (r *webSearchReadiness) authorize(ctx context.Context) error {
	if r == nil {
		return nil
	}
	capability, err := r.authorities.CatalogAuthorizer.ResolveAndRequire(ctx, releasecontract.CatalogSubject{
		Kind: releasecontract.CatalogSubjectTool, ID: "web_search", Runtime: releasecontract.CatalogRuntimeNetwork,
	}, releasecontract.BoundaryOutbound)
	if err != nil {
		return err
	}
	if capability != r.capability {
		return &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityUnknown, Field: "mcp.webSearchCapability"}
	}
	return nil
}

func NewTavilyWebSearchProvider(config TavilyWebSearchProviderConfig) WebSearchProvider {
	provider, _ := newTavilyWebSearchProvider(config, nil)
	return provider
}

// NewAuthorizedTavilyWebSearchProvider constructs a Tavily provider bound to
// the immutable startup authority. It registers its stable descriptor exactly
// once and fails closed before constructing the provider on missing authority.
func NewAuthorizedTavilyWebSearchProvider(config TavilyWebSearchProviderConfig, options WebSearchRuntimeOptions) (WebSearchProvider, error) {
	return newTavilyWebSearchProvider(config, &options)
}

func newTavilyWebSearchProvider(config TavilyWebSearchProviderConfig, options *WebSearchRuntimeOptions) (WebSearchProvider, error) {
	endpoint := strings.TrimSpace(config.Endpoint)
	apiKey := strings.TrimSpace(config.APIKey)
	if endpoint == "" || apiKey == "" {
		if options != nil {
			return nil, fmt.Errorf("mcp: tavily endpoint and api key are required")
		}
		return nil, nil
	}
	resultLimit := config.ResultLimit
	if resultLimit < 1 {
		resultLimit = 5
	}
	client := config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	provider := &tavilyWebSearchProvider{
		endpoint:    endpoint,
		apiKey:      apiKey,
		resultLimit: resultLimit,
		client:      client,
	}
	if options == nil {
		return provider, nil
	}
	readiness, err := newWebSearchReadiness(*options, "mcp.websearch.tavily", "mcp.TavilyWebSearchProvider")
	if err != nil {
		return nil, err
	}
	provider.readiness = readiness
	return provider, nil
}

func (p *tavilyWebSearchProvider) Search(ctx context.Context, query string) ([]WebSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if err := p.readiness.authorize(ctx); err != nil {
		return nil, err
	}

	payload := map[string]any{
		"api_key":     p.apiKey,
		"query":       query,
		"max_results": p.resultLimit,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal web search request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create web search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("web search request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read web search response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("web search provider error: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var decoded struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
			Snippet string `json:"snippet"`
		} `json:"results"`
	}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return nil, fmt.Errorf("parse web search response: %w", err)
	}

	results := make([]WebSearchResult, 0, len(decoded.Results))
	for _, item := range decoded.Results {
		title := strings.TrimSpace(item.Title)
		url := strings.TrimSpace(item.URL)
		snippet := strings.TrimSpace(item.Content)
		if snippet == "" {
			snippet = strings.TrimSpace(item.Snippet)
		}
		if title == "" && url == "" && snippet == "" {
			continue
		}
		results = append(results, WebSearchResult{
			Title:   title,
			URL:     url,
			Snippet: snippet,
		})
	}
	return results, nil
}
