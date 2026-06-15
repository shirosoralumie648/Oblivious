package config

import "testing"

func TestLoadWorkflowConfigDefaultsAndGRPCPort(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("WORKFLOW_PORT", "")
	t.Setenv("PORT", "")
	t.Setenv("WORKFLOW_GRPC_PORT", "")
	t.Setenv("GRPC_PORT", "")

	cfg := LoadWorkflowConfig()
	if cfg.Port != "8082" || cfg.GRPCPort != "50064" {
		t.Fatalf("unexpected default ports: http=%q grpc=%q", cfg.Port, cfg.GRPCPort)
	}
}

func TestLoadWorkflowConfigPrefersDedicatedPorts(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("WORKFLOW_PORT", "18082")
	t.Setenv("PORT", "19082")
	t.Setenv("WORKFLOW_GRPC_PORT", "150064")
	t.Setenv("GRPC_PORT", "150061")

	cfg := LoadWorkflowConfig()
	if cfg.Port != "18082" || cfg.GRPCPort != "150064" {
		t.Fatalf("unexpected configured ports: http=%q grpc=%q", cfg.Port, cfg.GRPCPort)
	}
}

func TestLoadWorkflowConfigFallsBackToGenericPorts(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/oblivious?sslmode=disable")
	t.Setenv("SESSION_SECRET", "test-secret")
	t.Setenv("WORKFLOW_PORT", "")
	t.Setenv("PORT", "19082")
	t.Setenv("WORKFLOW_GRPC_PORT", "")
	t.Setenv("GRPC_PORT", "150064")

	cfg := LoadWorkflowConfig()
	if cfg.Port != "19082" || cfg.GRPCPort != "150064" {
		t.Fatalf("expected generic port fallback, got http=%q grpc=%q", cfg.Port, cfg.GRPCPort)
	}
}
