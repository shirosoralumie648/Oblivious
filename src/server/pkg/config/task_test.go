package config

import "testing"

func TestLoadTaskConfigDefaultsAndGRPCPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("TASK_PORT", "")
	t.Setenv("TASK_GRPC_PORT", "")
	t.Setenv("GRPC_PORT", "")

	cfg := LoadTaskConfig()
	if cfg.Port != "8084" || cfg.GRPCPort != "50065" {
		t.Fatalf("unexpected default ports: http=%q grpc=%q", cfg.Port, cfg.GRPCPort)
	}
}

func TestLoadTaskConfigPrefersTaskGRPCPortOverGenericPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("TASK_PORT", "18084")
	t.Setenv("TASK_GRPC_PORT", "150065")
	t.Setenv("GRPC_PORT", "150066")

	cfg := LoadTaskConfig()
	if cfg.Port != "18084" || cfg.GRPCPort != "150065" {
		t.Fatalf("unexpected configured ports: http=%q grpc=%q", cfg.Port, cfg.GRPCPort)
	}
}

func TestLoadTaskConfigFallsBackToGenericGRPCPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("TASK_GRPC_PORT", "")
	t.Setenv("GRPC_PORT", "150065")

	cfg := LoadTaskConfig()
	if cfg.GRPCPort != "150065" {
		t.Fatalf("expected generic GRPC_PORT fallback, got %q", cfg.GRPCPort)
	}
}
