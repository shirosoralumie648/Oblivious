package config

import "testing"

func TestLoadAgentConfigDefaultsAndGRPCPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("AGENT_PORT", "")
	t.Setenv("AGENT_GRPC_PORT", "")
	t.Setenv("GRPC_PORT", "")

	cfg := LoadAgentConfig()
	if cfg.Port != "8083" || cfg.GRPCPort != "50063" {
		t.Fatalf("unexpected default ports: http=%q grpc=%q", cfg.Port, cfg.GRPCPort)
	}
}

func TestLoadAgentConfigPrefersAgentGRPCPortOverGenericPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("AGENT_PORT", "18083")
	t.Setenv("AGENT_GRPC_PORT", "150063")
	t.Setenv("GRPC_PORT", "150064")

	cfg := LoadAgentConfig()
	if cfg.Port != "18083" || cfg.GRPCPort != "150063" {
		t.Fatalf("unexpected configured ports: http=%q grpc=%q", cfg.Port, cfg.GRPCPort)
	}
}

func TestLoadAgentConfigFallsBackToGenericGRPCPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("AGENT_GRPC_PORT", "")
	t.Setenv("GRPC_PORT", "150064")

	cfg := LoadAgentConfig()
	if cfg.GRPCPort != "150064" {
		t.Fatalf("expected generic GRPC_PORT fallback, got %q", cfg.GRPCPort)
	}
}
