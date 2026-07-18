package tools

import (
	"context"
	"fmt"
	"log"

	"oblivious/server/internal/mcp"
	"oblivious/server/internal/mcp/websearch"
	"oblivious/server/internal/releasecontract"
)

type WebsearchTool struct {
	providers        map[string]mcp.WebSearchProvider
	selectedProvider string
	fallbackChain    []string
	readiness        *websearchReadiness
}

type WebsearchRuntimeOptions struct {
	Guard       releasecontract.Guard
	Authorities releasecontract.RuntimeAuthorities
	Effects     releasecontract.EffectRegistrar
}

type websearchReadiness struct {
	authorities releasecontract.RuntimeAuthorities
	webSearch   releasecontract.CapabilityID
}

func newWebsearchReadiness(options WebsearchRuntimeOptions) (*websearchReadiness, error) {
	if options.Guard == nil || options.Effects == nil || !options.Authorities.Valid() {
		return nil, &releasecontract.ReadinessError{Code: releasecontract.CodeReadinessUnavailable, Field: "agent.tools.websearch"}
	}
	webSearch, err := options.Authorities.CapabilityBindings.Resolve(releasecontract.EffectToolWebSearch)
	if err != nil {
		return nil, err
	}
	if err := options.Effects.Register(releasecontract.EffectDescriptor{
		ID:           "agent.tool.web_search",
		CapabilityID: string(webSearch),
		Boundary:     releasecontract.BoundaryOutbound,
		Owner:        "agent.tools.WebsearchTool",
	}); err != nil {
		return nil, err
	}
	return &websearchReadiness{authorities: options.Authorities, webSearch: webSearch}, nil
}

func (r *websearchReadiness) authorize(ctx context.Context) error {
	if r == nil {
		return nil
	}
	capabilityID, err := r.authorities.CatalogAuthorizer.ResolveAndRequire(ctx, releasecontract.CatalogSubject{
		Kind:    releasecontract.CatalogSubjectTool,
		ID:      "web_search",
		Runtime: releasecontract.CatalogRuntimeNetwork,
	}, releasecontract.BoundaryOutbound)
	if err != nil {
		return err
	}
	if capabilityID != r.webSearch {
		return &releasecontract.ReadinessError{Code: releasecontract.CodeCapabilityUnknown, Field: "agent.tools.websearchCapability"}
	}
	return nil
}

func NewWebsearchTool(provider string, fallback []string, apiKey, endpoint, googleCSEID string) (*WebsearchTool, error) {
	return newWebsearchTool(provider, fallback, apiKey, endpoint, googleCSEID, nil)
}

// NewWebsearchToolWithOptions constructs the outer fallback tool with the
// same startup authority used by its provider chain. Each provider attempt is
// independently re-authorized; registration happens only here.
func NewWebsearchToolWithOptions(provider string, fallback []string, apiKey, endpoint, googleCSEID string, options WebsearchRuntimeOptions) (*WebsearchTool, error) {
	readiness, err := newWebsearchReadiness(options)
	if err != nil {
		return nil, err
	}
	return newWebsearchTool(provider, fallback, apiKey, endpoint, googleCSEID, readiness)
}

func newWebsearchTool(provider string, fallback []string, apiKey, endpoint, googleCSEID string, readiness *websearchReadiness) (*WebsearchTool, error) {
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
		readiness:        readiness,
	}, nil
}

func (t *WebsearchTool) Execute(ctx context.Context, query string) ([]mcp.WebSearchResult, error) {
	providerNames := append([]string{t.selectedProvider}, t.fallbackChain...)

	for _, name := range providerNames {
		provider, ok := t.providers[name]
		if !ok {
			continue
		}
		if t.readiness != nil {
			if err := t.readiness.authorize(ctx); err != nil {
				return nil, err
			}
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
