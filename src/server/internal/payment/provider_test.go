package payment

import (
	"errors"
	"testing"
)

func TestRegistryDefaultsBlankProviderToStripe(t *testing.T) {
	registry := NewRegistry("stripe")
	registry.Register(Provider{Name: "stripe", Configured: true})

	provider, err := registry.Resolve("")
	if err != nil {
		t.Fatalf("resolve blank provider: %v", err)
	}
	if provider.Name != "stripe" {
		t.Fatalf("expected default stripe provider, got %q", provider.Name)
	}
}

func TestRegistryRejectsRegisteredButUnconfiguredProvider(t *testing.T) {
	registry := NewRegistry("stripe")
	registry.Register(Provider{Name: "stripe", Configured: true})
	registry.Register(Provider{Name: "alipay", Configured: false})

	_, err := registry.Resolve("alipay")
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected ProviderError, got %T %v", err, err)
	}
	if providerErr.Code != CodeProviderNotConfigured || providerErr.Provider != "alipay" {
		t.Fatalf("expected unconfigured alipay error, got %+v", providerErr)
	}
}

func TestRegistryRejectsUnknownProvider(t *testing.T) {
	registry := NewRegistry("stripe")
	registry.Register(Provider{Name: "stripe", Configured: true})

	_, err := registry.Resolve("paypal")
	var providerErr *ProviderError
	if !errors.As(err, &providerErr) {
		t.Fatalf("expected ProviderError, got %T %v", err, err)
	}
	if providerErr.Code != CodeUnsupportedProvider || providerErr.Provider != "paypal" {
		t.Fatalf("expected unsupported paypal error, got %+v", providerErr)
	}
}

func TestRegistryAvailableProvidersOnlyIncludesConfiguredProviders(t *testing.T) {
	registry := NewRegistry("stripe")
	registry.Register(Provider{Name: "stripe", Configured: true})
	registry.Register(Provider{Name: "alipay", Configured: false})
	registry.Register(Provider{Name: "wechatpay", Configured: false})

	providers := registry.AvailableProviders()

	if len(providers) != 1 {
		t.Fatalf("expected one available provider, got %+v", providers)
	}
	if providers[0].Name != "stripe" {
		t.Fatalf("expected only stripe to be available, got %+v", providers)
	}
}

func TestDefaultRegistryKeepsDomesticProvidersCNYButUnconfigured(t *testing.T) {
	registry := DefaultRegistry()

	for _, providerName := range []string{"alipay", "wechatpay"} {
		_, err := registry.Resolve(providerName)
		var providerErr *ProviderError
		if !errors.As(err, &providerErr) || providerErr.Code != CodeProviderNotConfigured {
			t.Fatalf("expected %s to be registered but unconfigured, got %T %v", providerName, err, err)
		}
	}

	registry.Register(Provider{Name: "alipay", Configured: true, Currency: "cny"})
	provider, err := registry.Resolve("alipay")
	if err != nil {
		t.Fatalf("resolve configured alipay: %v", err)
	}
	if provider.Currency != "cny" {
		t.Fatalf("expected configured alipay currency cny, got %q", provider.Currency)
	}
}
