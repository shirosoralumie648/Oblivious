package http

import (
	"log"
	"strings"

	"oblivious/server/internal/config"
	"oblivious/server/internal/mcp"
	"oblivious/server/internal/mcp/websearch"
)

// buildAgentWebSearchProviderWithOptions is the authority-required factory
// used by readiness-aware composition. The legacy one-result wrapper below is
// retained until Plan 02 owns server.go wiring.
func buildAgentWebSearchProviderWithOptions(cfg config.Config, options mcp.WebSearchRuntimeOptions) (mcp.WebSearchProvider, error) {
	providerName := strings.ToLower(strings.TrimSpace(cfg.AgentWebSearchProvider))
	if providerName == "" {
		return nil, nil
	}
	if providerName == "tavily" {
		return mcp.NewAuthorizedTavilyWebSearchProvider(mcp.TavilyWebSearchProviderConfig{
			Endpoint:    cfg.AgentWebSearchEndpoint,
			APIKey:      cfg.AgentWebSearchAPIKey,
			ResultLimit: cfg.AgentWebSearchResultLimit,
		}, options)
	}
	return websearch.NewProviderFromConfig(websearch.Config{
		Provider:    providerName,
		APIKey:      cfg.AgentWebSearchAPIKey,
		Endpoint:    cfg.AgentWebSearchEndpoint,
		GoogleCSEID: cfg.AgentWebSearchGoogleCSEID,
	}, websearch.RuntimeOptions{Authorities: options.Authorities, Guard: options.Guard, Effects: options.Effects})
}

func buildAgentWebSearchProvider(cfg config.Config) mcp.WebSearchProvider {
	providerName := strings.ToLower(strings.TrimSpace(cfg.AgentWebSearchProvider))
	if providerName == "" {
		return nil
	}
	if providerName == "tavily" {
		return mcp.NewTavilyWebSearchProvider(mcp.TavilyWebSearchProviderConfig{
			Endpoint:    cfg.AgentWebSearchEndpoint,
			APIKey:      cfg.AgentWebSearchAPIKey,
			ResultLimit: cfg.AgentWebSearchResultLimit,
		})
	}
	provider, err := websearch.NewProviderFromConfig(websearch.Config{
		Provider:    providerName,
		APIKey:      cfg.AgentWebSearchAPIKey,
		Endpoint:    cfg.AgentWebSearchEndpoint,
		GoogleCSEID: cfg.AgentWebSearchGoogleCSEID,
	})
	if err != nil {
		log.Printf("warning: agent web search provider %q not configured: %v", providerName, err)
		return nil
	}
	return provider
}
