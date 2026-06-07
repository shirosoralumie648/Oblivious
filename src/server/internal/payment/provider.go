package payment

import (
	"fmt"
	"sort"
	"strings"
)

const (
	CodeProviderNotConfigured = "provider_not_configured"
	CodeUnsupportedProvider   = "unsupported_provider"
)

type Provider struct {
	Name       string
	Configured bool
}

type ProviderError struct {
	Code     string
	Provider string
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Provider)
}

type Registry struct {
	defaultProvider string
	providers       map[string]Provider
}

func NewRegistry(defaultProvider string) *Registry {
	return &Registry{
		defaultProvider: normalizeName(defaultProvider),
		providers:       map[string]Provider{},
	}
}

func (r *Registry) Register(provider Provider) {
	provider.Name = normalizeName(provider.Name)
	if provider.Name == "" {
		return
	}
	r.providers[provider.Name] = provider
}

func (r *Registry) Resolve(name string) (Provider, error) {
	name = normalizeName(name)
	if name == "" {
		name = r.defaultProvider
	}
	provider, ok := r.providers[name]
	if !ok {
		return Provider{}, &ProviderError{Code: CodeUnsupportedProvider, Provider: name}
	}
	if !provider.Configured {
		return Provider{}, &ProviderError{Code: CodeProviderNotConfigured, Provider: name}
	}
	return provider, nil
}

func (r *Registry) AvailableProviders() []Provider {
	providers := make([]Provider, 0, len(r.providers))
	for _, provider := range r.providers {
		if provider.Configured {
			providers = append(providers, provider)
		}
	}
	sort.Slice(providers, func(i, j int) bool {
		return providers[i].Name < providers[j].Name
	})
	return providers
}

func DefaultRegistry() *Registry {
	registry := NewRegistry("stripe")
	registry.Register(Provider{Name: "stripe", Configured: true})
	registry.Register(Provider{Name: "alipay", Configured: false})
	registry.Register(Provider{Name: "wechatpay", Configured: false})
	return registry
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
