package tools

import (
	"context"

	"oblivious/server/internal/mcp"
)

// RegisterCallAgentTool registers the call_agent tool in the registry.
// This should be called during service initialization with a properly configured agent service.
func RegisterCallAgentTool(registry *Registry, agentService AgentService) {
	registry.RegisterCustom(
		ToolMetadata{
			Name:             "call_agent",
			Description:      "Invoke another agent as a sub-agent to handle a specific task. Supports recursive delegation up to a configured depth limit.",
			Category:         CategoryCustom,
			RequiresApproval: false,
			RiskLevel:        "low",
			Tags:             []string{"agent", "delegation", "recursive"},
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"agentId": map[string]any{
						"type":        "string",
						"description": "The ID of the agent to invoke",
					},
					"requestText": map[string]any{
						"type":        "string",
						"description": "The task or question to send to the sub-agent",
					},
					"recursionDepth": map[string]any{
						"type":        "integer",
						"description": "Current recursion depth (internal use)",
						"default":     0,
					},
					"maxDepth": map[string]any{
						"type":        "integer",
						"description": "Maximum recursion depth allowed",
						"default":     3,
					},
				},
				"required": []string{"agentId", "requestText"},
			},
		},
		func(ctx context.Context, args map[string]any) (*mcp.ToolResult, error) {
			tool := NewCallAgentTool(agentService)

			agentID, _ := args["agentId"].(string)
			requestText, _ := args["requestText"].(string)
			recursionDepth, _ := args["recursionDepth"].(float64)
			maxDepth, _ := args["maxDepth"].(float64)

			if maxDepth == 0 {
				maxDepth = 3
			}

			input := CallAgentInput{
				AgentID:        agentID,
				RequestText:    requestText,
				RecursionDepth: int(recursionDepth),
				MaxDepth:       int(maxDepth),
			}

			output, err := tool.Execute(ctx, input)
			if err != nil {
				return &mcp.ToolResult{
					Content: err.Error(),
					IsError: true,
				}, nil
			}

			return &mcp.ToolResult{
				Content: output.Result,
				IsError: false,
			}, nil
		},
	)
}
