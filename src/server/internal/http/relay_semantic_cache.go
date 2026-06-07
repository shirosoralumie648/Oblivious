package http

import (
	"database/sql"
	"fmt"
	"strings"

	"oblivious/server/internal/config"
	"oblivious/server/internal/memory"
	"oblivious/server/internal/relay"
	relaycache "oblivious/server/internal/relay/cache"
	"oblivious/server/internal/relay/handler"
)

type relaySemanticCacheConfig struct {
	Disabled bool
	Store    relaycache.SemanticCacheStore
	Embedder handler.SemanticCacheEmbedder
}

func buildRelaySemanticCacheConfig(cfg config.Config, database *sql.DB) relaySemanticCacheConfig {
	backend := strings.ToLower(strings.TrimSpace(cfg.RelaySemanticCacheBackend))
	if backend == "none" {
		return relaySemanticCacheConfig{Disabled: true}
	}
	cacheConfig := relaySemanticCacheConfig{Store: buildRelaySemanticCacheStore(cfg, database)}
	if backend == "sql" {
		cacheConfig.Embedder = memory.NewRelayEmbedder(
			"http://localhost:"+fmt.Sprintf("%d", cfg.Port)+"/v1",
			"text-embedding-3-small",
		)
	}
	return cacheConfig
}

func applyRelaySemanticCacheConfig(cfg *relay.Config, cacheConfig relaySemanticCacheConfig) {
	if cfg == nil {
		return
	}
	cfg.SemanticCacheDisabled = cacheConfig.Disabled
	cfg.SemanticCacheStore = cacheConfig.Store
	cfg.SemanticCacheEmbedder = cacheConfig.Embedder
}

func buildRelaySemanticCacheStore(cfg config.Config, database *sql.DB) relaycache.SemanticCacheStore {
	switch strings.ToLower(strings.TrimSpace(cfg.RelaySemanticCacheBackend)) {
	case "none":
		return nil
	case "sql":
		return relaycache.NewSQLSemanticCacheStore(database)
	default:
		return relaycache.NewInMemorySemanticCacheStore()
	}
}
