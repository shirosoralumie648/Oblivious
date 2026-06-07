package http

import (
	"strings"

	"oblivious/server/internal/config"
	"oblivious/server/internal/mcp"
)

func buildAgentWebSearchProvider(cfg config.Config) mcp.WebSearchProvider {
	if strings.ToLower(strings.TrimSpace(cfg.AgentWebSearchProvider)) != "tavily" {
		return nil
	}
	return mcp.NewTavilyWebSearchProvider(mcp.TavilyWebSearchProviderConfig{
		Endpoint:    cfg.AgentWebSearchEndpoint,
		APIKey:      cfg.AgentWebSearchAPIKey,
		ResultLimit: cfg.AgentWebSearchResultLimit,
	})
}
