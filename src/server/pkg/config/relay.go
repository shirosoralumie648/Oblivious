package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type RelayConfig struct {
	CommonConfig
	Port               int
	CORSAllowedOrigins []string

	RelayEnabled                 bool
	RelayDefaultModel            string
	RelayRateLimitBackend        string
	RelayRateLimitRedisKeyPrefix string
	RelaySemanticCacheBackend    string
}

func LoadRelayConfig() (RelayConfig, error) {
	common, err := LoadCommon()
	if err != nil {
		return RelayConfig{}, err
	}

	portRaw := strings.TrimSpace(os.Getenv("SERVER_PORT"))
	if portRaw == "" {
		portRaw = "8080"
	}
	port, err := strconv.Atoi(portRaw)
	if err != nil || port < 1 || port > 65535 {
		return RelayConfig{}, fmt.Errorf("invalid SERVER_PORT: %q", portRaw)
	}

	originsRaw := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	var origins []string
	if originsRaw != "" {
		for _, part := range strings.Split(originsRaw, ",") {
			if value := strings.TrimSpace(part); value != "" {
				origins = append(origins, value)
			}
		}
	}

	relayEnabled := true
	if raw := strings.TrimSpace(os.Getenv("RELAY_ENABLED")); raw != "" {
		relayEnabled = strings.EqualFold(raw, "true")
	}

	relayDefaultModel := strings.TrimSpace(os.Getenv("RELAY_DEFAULT_MODEL"))
	if relayDefaultModel == "" {
		relayDefaultModel = "gpt-4o-mini"
	}

	relaySemanticCacheBackend := strings.ToLower(strings.TrimSpace(os.Getenv("RELAY_SEMANTIC_CACHE_BACKEND")))
	if relaySemanticCacheBackend == "" {
		relaySemanticCacheBackend = "memory"
	}
	if relaySemanticCacheBackend != "none" && relaySemanticCacheBackend != "memory" && relaySemanticCacheBackend != "sql" {
		return RelayConfig{}, fmt.Errorf("invalid RELAY_SEMANTIC_CACHE_BACKEND: %q", relaySemanticCacheBackend)
	}

	relayRateLimitBackend := strings.ToLower(strings.TrimSpace(os.Getenv("RELAY_RATE_LIMIT_BACKEND")))
	if relayRateLimitBackend == "" {
		relayRateLimitBackend = "memory"
	}
	if relayRateLimitBackend != "none" && relayRateLimitBackend != "memory" && relayRateLimitBackend != "redis" {
		return RelayConfig{}, fmt.Errorf("invalid RELAY_RATE_LIMIT_BACKEND: %q", relayRateLimitBackend)
	}

	relayRateLimitRedisKeyPrefix := strings.TrimSpace(os.Getenv("RELAY_RATE_LIMIT_REDIS_KEY_PREFIX"))

	return RelayConfig{
		CommonConfig:                 common,
		Port:                         port,
		CORSAllowedOrigins:           origins,
		RelayEnabled:                 relayEnabled,
		RelayDefaultModel:            relayDefaultModel,
		RelayRateLimitBackend:        relayRateLimitBackend,
		RelayRateLimitRedisKeyPrefix: relayRateLimitRedisKeyPrefix,
		RelaySemanticCacheBackend:    relaySemanticCacheBackend,
	}, nil
}
