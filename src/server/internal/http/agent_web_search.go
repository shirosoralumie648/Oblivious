package http

import (
	"log"
	"strings"

	"oblivious/server/internal/config"
	"oblivious/server/internal/mcp"
	"oblivious/server/internal/mcp/websearch"
)

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
