package mcp

import (
	"context"
	"strings"
	"testing"
)

type registryProbeTool struct {
	name string
}

func (t *registryProbeTool) Name() string        { return t.name }
func (t *registryProbeTool) Description() string { return "registry probe tool" }
func (t *registryProbeTool) InputSchema() any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

func (t *registryProbeTool) Execute(_ context.Context, _ map[string]any) (*ToolResult, error) {
	return &ToolResult{Content: "probe"}, nil
}

func TestRegisterBuiltinsAddsToolsAndPolicy(t *testing.T) {
	enabledName := "test_registry_probe_enabled"
	disabledName := "test_registry_probe_disabled"
	t.Cleanup(func() {
		delete(BuiltinTools, enabledName)
		delete(BuiltinTools, disabledName)
		delete(defaultCommercialBuiltinEnabled, enabledName)
		delete(defaultCommercialBuiltinEnabled, disabledName)
	})

	registerBuiltins(
		map[string]BuiltinTool{
			enabledName:  &registryProbeTool{name: enabledName},
			disabledName: &registryProbeTool{name: disabledName},
		},
		map[string]bool{
			enabledName:  true,
			disabledName: false,
		},
	)

	if _, ok := GetBuiltinTool(enabledName); !ok {
		t.Fatalf("expected %s to be registered", enabledName)
	}
	if _, ok := GetBuiltinTool(disabledName); !ok {
		t.Fatalf("expected %s to be registered", disabledName)
	}
	if !IsDefaultCommercialBuiltin(enabledName) {
		t.Fatalf("expected %s to be default commercial enabled", enabledName)
	}
	if IsDefaultCommercialBuiltin(disabledName) {
		t.Fatalf("expected %s to be disabled by default commercial policy", disabledName)
	}
}

func TestRegisterBuiltinsPanicsOnDuplicate(t *testing.T) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected duplicate registration to panic")
		}
		if !strings.Contains(recovered.(string), "calculator") {
			t.Fatalf("expected panic message to mention duplicate name, got %v", recovered)
		}
	}()

	registerBuiltins(
		map[string]BuiltinTool{"calculator": &registryProbeTool{name: "calculator"}},
		nil,
	)
}
