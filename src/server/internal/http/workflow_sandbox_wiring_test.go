package http

import (
	"context"
	"strings"
	"testing"

	"oblivious/server/internal/config"
	"oblivious/server/internal/workflow"
	"oblivious/server/internal/workflow/sandbox"
)

func TestBuildWorkflowSandboxCodeRunnerDisabledByDefault(t *testing.T) {
	if runner := buildWorkflowSandboxCodeRunner(config.Config{}); runner != nil {
		t.Fatalf("expected nil runner when sandbox disabled, got %#v", runner)
	}
}

func TestBuildWorkflowSandboxCodeRunnerEnabled(t *testing.T) {
	runner := buildWorkflowSandboxCodeRunner(config.Config{
		WorkflowSandboxEnabled:          true,
		WorkflowSandboxAllowedLanguages: "python, javascript",
		WorkflowSandboxMemoryMB:         512,
		WorkflowSandboxCPUs:             2,
		WorkflowSandboxDefaultTimeoutMS: 5000,
		WorkflowSandboxMaxTimeoutMS:     20000,
	})
	if runner == nil {
		t.Fatal("expected sandbox runner when enabled, got nil")
	}
	if _, ok := runner.(*sandbox.DockerSandboxRunner); !ok {
		t.Fatalf("expected *sandbox.DockerSandboxRunner, got %T", runner)
	}
}

func workflowSandboxCodeNodeDefinition() map[string]any {
	return map[string]any{
		"nodes": []any{map[string]any{
			"id":   "run_code",
			"type": "code",
			"input": map[string]any{
				"language": "ruby",
				"code":     "puts 'hello'",
			},
		}},
	}
}

func runWorkflowSandboxCodeNode(t *testing.T, service *workflow.Service) error {
	t.Helper()
	ctx := context.Background()
	created, err := service.CreateWorkflow(ctx, workflow.CreateWorkflowRequest{
		OrganizationID: "org_1",
		Name:           "Sandbox code flow",
		Status:         workflow.WorkflowStatusPublished,
		Definition:     workflowSandboxCodeNodeDefinition(),
	})
	if err != nil {
		t.Fatalf("CreateWorkflow returned error: %v", err)
	}
	execution, err := service.StartExecution(ctx, workflow.StartExecutionRequest{
		OrganizationID: "org_1",
		WorkflowID:     created.ID,
		Input:          map[string]any{},
		Context:        map[string]any{"userId": "user_1", "workspaceId": "workspace_1"},
	})
	if err != nil {
		t.Fatalf("StartExecution returned error: %v", err)
	}
	return service.RunReadyNode(ctx, "org_1", execution.ID, "run_code")
}

func TestNewConfiguredWorkflowServiceCodeNodeWithoutSandbox(t *testing.T) {
	service := newConfiguredWorkflowServiceWithStore(config.Config{Port: 8080}, newWorkflowServiceMemoryStore())

	err := runWorkflowSandboxCodeNode(t, service)
	if err == nil {
		t.Fatal("expected default code node execution to fail for non-javascript without sandbox")
	}
	if !strings.Contains(err.Error(), "code runner is required") {
		t.Fatalf("expected default 'code runner is required' error, got %v", err)
	}
}

func TestNewConfiguredWorkflowServiceCodeNodeWithSandboxEnabled(t *testing.T) {
	service := newConfiguredWorkflowServiceWithStore(config.Config{
		Port:                            8080,
		WorkflowSandboxEnabled:          true,
		WorkflowSandboxAllowedLanguages: "python",
	}, newWorkflowServiceMemoryStore())

	err := runWorkflowSandboxCodeNode(t, service)
	if err == nil {
		t.Fatal("expected sandbox policy rejection for disallowed language")
	}
	if !strings.Contains(err.Error(), "not allowed by policy") {
		t.Fatalf("expected sandbox language policy error proving the docker runner is wired, got %v", err)
	}
}
