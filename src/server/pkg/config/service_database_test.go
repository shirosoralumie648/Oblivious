package config

import "testing"

func TestLoadBillingConfigUsesServiceDatabaseURLInMicroservicesMode(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://main")
	t.Setenv("DB_URL_BILLING", "postgres://billing")
	t.Setenv("OBLIVIOUS_DB_MODE", "microservices")
	t.Setenv("SESSION_SECRET", "test-secret")

	cfg := LoadBillingConfig()
	if cfg.DatabaseURL != "postgres://billing" {
		t.Fatalf("LoadBillingConfig DatabaseURL = %q, want billing database", cfg.DatabaseURL)
	}
}

func TestLoadRAGConfigKeepsMainDatabaseURLInMonolithMode(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://main")
	t.Setenv("DB_URL_RAG", "postgres://rag")
	t.Setenv("OBLIVIOUS_DB_MODE", "monolith")
	t.Setenv("SESSION_SECRET", "test-secret")

	cfg := LoadRAGConfig()
	if cfg.DatabaseURL != "postgres://main" {
		t.Fatalf("LoadRAGConfig DatabaseURL = %q, want main database", cfg.DatabaseURL)
	}
}

func TestLoadRAGConfigUsesServiceDatabaseURLInMicroservicesMode(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://main")
	t.Setenv("DB_URL_RAG", "postgres://rag")
	t.Setenv("OBLIVIOUS_DB_MODE", "microservices")
	t.Setenv("SESSION_SECRET", "test-secret")

	cfg := LoadRAGConfig()
	if cfg.DatabaseURL != "postgres://rag" {
		t.Fatalf("LoadRAGConfig DatabaseURL = %q, want rag database", cfg.DatabaseURL)
	}
}

func TestServiceConfigsUseServiceDatabaseURLsInMicroservicesMode(t *testing.T) {
	tests := []struct {
		name     string
		envKey   string
		envValue string
		load     func() (string, error)
	}{
		{
			name:     "relay",
			envKey:   "DB_URL_RELAY",
			envValue: "postgres://relay",
			load: func() (string, error) {
				cfg, err := LoadRelayConfig()
				if err != nil {
					return "", err
				}
				return cfg.DatabaseURL, nil
			},
		},
		{
			name:     "admin",
			envKey:   "DB_URL_ADMIN",
			envValue: "postgres://admin",
			load: func() (string, error) {
				return LoadAdminConfig().DatabaseURL, nil
			},
		},
		{
			name:     "agent",
			envKey:   "DB_URL_AGENT",
			envValue: "postgres://agent",
			load: func() (string, error) {
				return LoadAgentConfig().DatabaseURL, nil
			},
		},
		{
			name:     "workflow",
			envKey:   "DB_URL_WORKFLOW",
			envValue: "postgres://workflow",
			load: func() (string, error) {
				return LoadWorkflowConfig().DatabaseURL, nil
			},
		},
		{
			name:     "task",
			envKey:   "DB_URL_TASK",
			envValue: "postgres://task",
			load: func() (string, error) {
				return LoadTaskConfig().DatabaseURL, nil
			},
		},
		{
			name:     "observability",
			envKey:   "DB_URL_OBSERVABILITY",
			envValue: "postgres://observability",
			load: func() (string, error) {
				return LoadObservabilityConfig().DatabaseURL, nil
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://main")
			t.Setenv(tc.envKey, tc.envValue)
			t.Setenv("OBLIVIOUS_DB_MODE", "microservices")
			t.Setenv("SESSION_SECRET", "test-secret")

			got, err := tc.load()
			if err != nil {
				t.Fatalf("%s config load failed: %v", tc.name, err)
			}
			if got != tc.envValue {
				t.Fatalf("%s DatabaseURL = %q, want %q", tc.name, got, tc.envValue)
			}
		})
	}
}
