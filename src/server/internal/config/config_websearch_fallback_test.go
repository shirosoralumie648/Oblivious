package config

import (
	"os"
	"testing"
)

func TestLoadAgentWebSearchFallbackConfig(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("SESSION_SECRET", "test-secret")
	os.Setenv("AGENT_WEB_SEARCH_PROVIDER", "tavily")
	os.Setenv("AGENT_WEB_SEARCH_FALLBACK", "brave,duckduckgo,searxng")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("SESSION_SECRET")
		os.Unsetenv("AGENT_WEB_SEARCH_PROVIDER")
		os.Unsetenv("AGENT_WEB_SEARCH_FALLBACK")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.AgentWebSearchProvider != "tavily" {
		t.Errorf("expected provider tavily, got %s", cfg.AgentWebSearchProvider)
	}

	expected := []string{"brave", "duckduckgo", "searxng"}
	if len(cfg.AgentWebSearchFallback) != len(expected) {
		t.Fatalf("expected %d fallback providers, got %d", len(expected), len(cfg.AgentWebSearchFallback))
	}

	for i, exp := range expected {
		if cfg.AgentWebSearchFallback[i] != exp {
			t.Errorf("fallback[%d]: expected %s, got %s", i, exp, cfg.AgentWebSearchFallback[i])
		}
	}
}

func TestLoadAgentWebSearchFallbackWithWhitespace(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("SESSION_SECRET", "test-secret")
	os.Setenv("AGENT_WEB_SEARCH_PROVIDER", "tavily")
	os.Setenv("AGENT_WEB_SEARCH_FALLBACK", " brave , duckduckgo ,  searxng  ")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("SESSION_SECRET")
		os.Unsetenv("AGENT_WEB_SEARCH_PROVIDER")
		os.Unsetenv("AGENT_WEB_SEARCH_FALLBACK")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	expected := []string{"brave", "duckduckgo", "searxng"}
	if len(cfg.AgentWebSearchFallback) != len(expected) {
		t.Fatalf("expected %d fallback providers, got %d", len(expected), len(cfg.AgentWebSearchFallback))
	}

	for i, exp := range expected {
		if cfg.AgentWebSearchFallback[i] != exp {
			t.Errorf("fallback[%d]: expected %s, got %s", i, exp, cfg.AgentWebSearchFallback[i])
		}
	}
}

func TestLoadAgentWebSearchNoFallback(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("SESSION_SECRET", "test-secret")
	os.Setenv("AGENT_WEB_SEARCH_PROVIDER", "tavily")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("SESSION_SECRET")
		os.Unsetenv("AGENT_WEB_SEARCH_PROVIDER")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.AgentWebSearchProvider != "tavily" {
		t.Errorf("expected provider tavily, got %s", cfg.AgentWebSearchProvider)
	}

	if len(cfg.AgentWebSearchFallback) != 0 {
		t.Errorf("expected no fallback providers, got %d", len(cfg.AgentWebSearchFallback))
	}
}

func TestLoadAgentWebSearchEmptyFallbackElements(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://localhost/test")
	os.Setenv("SESSION_SECRET", "test-secret")
	os.Setenv("AGENT_WEB_SEARCH_PROVIDER", "tavily")
	os.Setenv("AGENT_WEB_SEARCH_FALLBACK", "brave,,duckduckgo,  ,searxng")
	defer func() {
		os.Unsetenv("DATABASE_URL")
		os.Unsetenv("SESSION_SECRET")
		os.Unsetenv("AGENT_WEB_SEARCH_PROVIDER")
		os.Unsetenv("AGENT_WEB_SEARCH_FALLBACK")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	expected := []string{"brave", "duckduckgo", "searxng"}
	if len(cfg.AgentWebSearchFallback) != len(expected) {
		t.Fatalf("expected %d fallback providers (empty entries filtered), got %d", len(expected), len(cfg.AgentWebSearchFallback))
	}
}
