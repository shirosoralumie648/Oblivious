package tools

import (
	"context"
	"fmt"
	"log"

	"oblivious/server/internal/mcp"
	"oblivious/server/internal/mcp/websearch"
)

type WebsearchTool struct {
	providers        map[string]mcp.WebSearchProvider
	selectedProvider string
	fallbackChain    []string
}

func NewWebsearchTool(provider string, fallback []string, apiKey, endpoint, googleCSEID string) (*WebsearchTool, error) {
	if provider == "" {
		provider = "tavily"
	}

	providers := make(map[string]mcp.WebSearchProvider)
	allProviders := append([]string{provider}, fallback...)

	for _, name := range allProviders {
		p, err := websearch.NewProviderFromConfig(websearch.Config{
			Provider:    name,
			APIKey:      apiKey,
			Endpoint:    endpoint,
			GoogleCSEID: googleCSEID,
		})
		if err != nil {
			log.Printf("websearch: failed to initialize provider %s: %v", name, err)
			continue
		}
		providers[name] = p
	}

	if len(providers) == 0 {
		return nil, fmt.Errorf("no websearch providers could be initialized")
	}

	return &WebsearchTool{
		providers:        providers,
		selectedProvider: provider,
		fallbackChain:    fallback,
	}, nil
}

func (t *WebsearchTool) Execute(ctx context.Context, query string) ([]mcp.WebSearchResult, error) {
	providerNames := append([]string{t.selectedProvider}, t.fallbackChain...)

	for _, name := range providerNames {
		provider, ok := t.providers[name]
		if !ok {
			continue
		}

		results, err := provider.Search(ctx, query)
		if err == nil {
			log.Printf("websearch: %s succeeded", name)
			return results, nil
		}
		log.Printf("websearch: %s failed (%v), trying next", name, err)
	}

	return nil, fmt.Errorf("all websearch providers exhausted")
}
