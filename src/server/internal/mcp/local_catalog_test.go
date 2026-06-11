package mcp

import (
	"context"
	"testing"
)

func TestLocalCatalogDiscoversSafeBuiltinServerAndTools(t *testing.T) {
	ctx := context.Background()
	catalog := NewLocalCatalog()

	servers := catalog.ListServers(ctx)
	if len(servers) != 1 {
		t.Fatalf("ListServers returned %d servers, want 1", len(servers))
	}
	if servers[0].ID == "" {
		t.Fatalf("local server ID is empty: %+v", servers[0])
	}
	if servers[0].ToolCount < 4 {
		t.Fatalf("local server tool count = %d, want >= 4", servers[0].ToolCount)
	}

	tools, err := catalog.ListTools(ctx, servers[0].ID)
	if err != nil {
		t.Fatalf("ListTools returned error: %v", err)
	}

	names := make(map[string]bool, len(tools))
	for _, tool := range tools {
		names[tool.Name] = true
		if tool.InputSchema == nil {
			t.Fatalf("tool %s has nil input schema", tool.Name)
		}
	}
	for _, name := range []string{"calculator", "datetime", "json_formatter", "text_transform"} {
		if !names[name] {
			t.Fatalf("expected local catalog to expose %s, got names=%v", name, names)
		}
	}
	for _, name := range []string{"web_search", "http_request"} {
		if names[name] {
			t.Fatalf("expected local catalog not to expose %s by default, got names=%v", name, names)
		}
	}
}
