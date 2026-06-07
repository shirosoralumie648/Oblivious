package mcp

import (
	"context"
	"fmt"
)

const LocalBuiltinServerID = "local_builtin_safe"

type LocalServerDefinition struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ToolCount   int    `json:"toolCount"`
}

type LocalCatalog struct{}

func NewLocalCatalog() LocalCatalog {
	return LocalCatalog{}
}

func (c LocalCatalog) ListServers(ctx context.Context) []LocalServerDefinition {
	_ = ctx
	return []LocalServerDefinition{{
		ID:          LocalBuiltinServerID,
		Name:        "Oblivious Safe Builtins",
		Description: "Tenant-safe local MCP tools exposed by this server",
		ToolCount:   len(ListDefaultCommercialBuiltinTools()),
	}}
}

func (c LocalCatalog) ListTools(ctx context.Context, serverID string) ([]ToolDefinition, error) {
	_ = ctx
	if serverID != LocalBuiltinServerID {
		return nil, fmt.Errorf("local MCP server not found: %s", serverID)
	}
	return ListDefaultCommercialBuiltinTools(), nil
}
