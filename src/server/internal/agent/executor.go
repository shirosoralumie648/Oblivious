package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"oblivious/server/internal/mcp"
)

// ToolExecutor 工具执行器
type ToolExecutor struct {
	mcpClient    *mcp.Client
	builtinTools map[string]mcp.BuiltinTool
}

// NewToolExecutor 创建工具执行器
func NewToolExecutor(mcpClient *mcp.Client) *ToolExecutor {
	return &ToolExecutor{
		mcpClient:    mcpClient,
		builtinTools: mcp.BuiltinTools,
	}
}

// ExecuteResult 工具执行结果
type ExecuteResult struct {
	Content string `json:"content"`
	IsError bool   `json:"isError,omitempty"`
}

// Execute 执行工具调用
func (e *ToolExecutor) Execute(ctx context.Context, agent *Agent, toolCall *ToolCall) (*ExecuteResult, error) {
	// 查找工具配置
	var targetTool *Tool
	for _, t := range agent.Tools {
		if t.Name == toolCall.Name && t.Enabled {
			targetTool = &t
			break
		}
	}
	if targetTool == nil {
		return nil, fmt.Errorf("tool not found or disabled: %s", toolCall.Name)
	}

	// 根据工具类型执行
	switch targetTool.Type {
	case "builtin":
		return e.executeBuiltin(ctx, toolCall)
	case "mcp":
		return e.executeMCP(ctx, agent.OrganizationID, targetTool.ServerID, toolCall)
	default:
		return nil, fmt.Errorf("unknown tool type: %s", targetTool.Type)
	}
}

// executeBuiltin 执行内置工具
func (e *ToolExecutor) executeBuiltin(ctx context.Context, toolCall *ToolCall) (*ExecuteResult, error) {
	tool, ok := e.builtinTools[toolCall.Name]
	if !ok {
		return nil, fmt.Errorf("builtin tool not found: %s", toolCall.Name)
	}
	if !mcp.IsDefaultCommercialBuiltin(toolCall.Name) {
		return &ExecuteResult{
			Content: fmt.Sprintf("builtin tool %s is disabled for default commercial use", toolCall.Name),
			IsError: true,
		}, nil
	}

	result, err := tool.Execute(ctx, toolCall.Arguments)
	if err != nil {
		return &ExecuteResult{
			Content: err.Error(),
			IsError: true,
		}, nil
	}

	return &ExecuteResult{
		Content: result.Content,
		IsError: result.IsError,
	}, nil
}

// executeMCP 执行 MCP 工具
func (e *ToolExecutor) executeMCP(ctx context.Context, organizationID, serverID string, toolCall *ToolCall) (*ExecuteResult, error) {
	if e.mcpClient == nil {
		return nil, fmt.Errorf("MCP client not configured")
	}
	if serverID == "" {
		return nil, fmt.Errorf("MCP server not specified")
	}

	result, err := e.mcpClient.CallTool(ctx, serverID, organizationID, toolCall.Name, toolCall.Arguments)
	if err != nil {
		return &ExecuteResult{
			Content: err.Error(),
			IsError: true,
		}, nil
	}

	return &ExecuteResult{
		Content: result.Content,
		IsError: result.IsError,
	}, nil
}

// GetToolDefinitions 获取 Agent 可用的工具定义
func (e *ToolExecutor) GetToolDefinitions(ctx context.Context, agent *Agent) ([]ToolDefinition, error) {
	var definitions []ToolDefinition

	for _, t := range agent.Tools {
		if !t.Enabled {
			continue
		}

		def := ToolDefinition{
			Name:        t.Name,
			Description: t.Description,
		}

		// 获取 InputSchema
		switch t.Type {
		case "builtin":
			if !mcp.IsDefaultCommercialBuiltin(t.Name) {
				continue
			}
			if builtin, ok := e.builtinTools[t.Name]; ok {
				def.InputSchema = builtin.InputSchema()
				if def.Description == "" {
					def.Description = builtin.Description()
				}
			}
		case "mcp":
			if e.mcpClient != nil && t.ServerID != "" {
				mcpTools, err := e.mcpClient.ListTools(t.ServerID, agent.OrganizationID)
				if err == nil {
					for _, mt := range mcpTools {
						if mt.Name == t.Name {
							def.InputSchema = mt.InputSchema
							if def.Description == "" {
								def.Description = mt.Description
							}
							break
						}
					}
				}
			}
		}

		definitions = append(definitions, def)
	}

	return definitions, nil
}

// ToolDefinition 工具定义（用于 OpenAI function calling）
type ToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"inputSchema,omitempty"`
}

// ToOpenAITool 转换为 OpenAI 工具格式
func (d ToolDefinition) ToOpenAITool() map[string]any {
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        d.Name,
			"description": d.Description,
			"parameters":  d.InputSchema,
		},
	}
}

// ParseToolCallsFromResponse 从 LLM 响应解析工具调用
func ParseToolCallsFromResponse(response map[string]any) ([]ToolCall, error) {
	toolCallsRaw, ok := response["tool_calls"]
	if !ok {
		return nil, nil
	}

	toolCallsArray, ok := toolCallsRaw.([]any)
	if !ok {
		return nil, nil
	}

	var toolCalls []ToolCall
	for _, tcRaw := range toolCallsArray {
		tcMap, ok := tcRaw.(map[string]any)
		if !ok {
			continue
		}

		id, _ := tcMap["id"].(string)
		function, ok := tcMap["function"].(map[string]any)
		if !ok {
			continue
		}

		name, _ := function["name"].(string)
		argsRaw, _ := function["arguments"].(string)

		var arguments map[string]any
		if argsRaw != "" {
			json.Unmarshal([]byte(argsRaw), &arguments)
		}

		toolCalls = append(toolCalls, ToolCall{
			ID:        id,
			Name:      name,
			Arguments: arguments,
		})
	}

	return toolCalls, nil
}
