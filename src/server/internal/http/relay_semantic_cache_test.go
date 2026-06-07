package http

import (
	"testing"

	"oblivious/server/internal/config"
	relaycache "oblivious/server/internal/relay/cache"
)

func TestBuildRelaySemanticCacheStoreCreatesMemoryStoreByDefault(t *testing.T) {
	store := buildRelaySemanticCacheStore(config.Config{}, nil)
	if _, ok := store.(*relaycache.InMemorySemanticCacheStore); !ok {
		t.Fatalf("expected in-memory semantic cache store by default, got %T", store)
	}
}

func TestBuildRelaySemanticCacheStoreCreatesSQLStore(t *testing.T) {
	store := buildRelaySemanticCacheStore(config.Config{RelaySemanticCacheBackend: "sql"}, nil)
	if _, ok := store.(*relaycache.SQLSemanticCacheStore); !ok {
		t.Fatalf("expected SQL semantic cache store, got %T", store)
	}
}

func TestBuildRelaySemanticCacheStoreCanBeDisabled(t *testing.T) {
	store := buildRelaySemanticCacheStore(config.Config{RelaySemanticCacheBackend: "none"}, nil)
	if store != nil {
		t.Fatalf("expected nil semantic cache store when backend is none, got %T", store)
	}
}

func TestBuildRelayConfigDisablesSemanticCacheWhenBackendIsNone(t *testing.T) {
	relayConfig := buildRelayConfig(config.Config{RelaySemanticCacheBackend: "none"}, nil, nil, nil, nil, nil, nil)

	if relayConfig == nil {
		t.Fatal("expected relay config")
	}
	if !relayConfig.SemanticCacheDisabled {
		t.Fatal("expected relay config to disable semantic cache")
	}
	if relayConfig.SemanticCacheStore != nil {
		t.Fatalf("expected no semantic cache store when disabled, got %T", relayConfig.SemanticCacheStore)
	}
}

func TestBuildRelayConfigAddsSemanticCacheEmbedderForSQLBackend(t *testing.T) {
	relayConfig := buildRelayConfig(config.Config{
		RelaySemanticCacheBackend: "sql",
		Port:                      18080,
	}, nil, nil, nil, nil, nil, nil)

	if relayConfig == nil {
		t.Fatal("expected relay config")
	}
	if relayConfig.SemanticCacheEmbedder == nil {
		t.Fatal("expected SQL semantic cache backend to configure query embedder")
	}
}
