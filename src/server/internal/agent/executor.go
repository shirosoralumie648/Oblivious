package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"oblivious/server/internal/mcp"
)

// ToolExecutor 工具执行器
type ToolExecutor struct {
	mcpClient         *mcp.Client
	builtinTools      map[string]mcp.BuiltinTool
	webSearchProvider mcp.WebSearchProvider
}

// NewToolExecutor 创建工具执行器
func NewToolExecutor(mcpClient *mcp.Client) *ToolExecutor {
	return &ToolExecutor{
		mcpClient:    mcpClient,
		builtinTools: mcp.BuiltinTools,
	}
}

func (e *ToolExecutor) SetWebSearchProvider(provider mcp.WebSearchProvider) {
	e.webSearchProvider = provider
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
	case "custom":
		if normalizeCustomToolRuntime(targetTool.Runtime) == "python" {
			return e.executeCustomPython(ctx, targetTool, toolCall)
		}
		return e.executeCustomAPI(ctx, targetTool, toolCall)
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
	if !e.isCommercialBuiltinEnabled(toolCall.Name) {
		return &ExecuteResult{
			Content: fmt.Sprintf("builtin tool %s is disabled for default commercial use", toolCall.Name),
			IsError: true,
		}, nil
	}
	if toolCall.Name == "web_search" && e.webSearchProvider != nil {
		restore := mcp.SetWebSearchProviderForTest(e.webSearchProvider)
		defer restore()
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

// isSafeSSRFURL 检查给定的 URL 是否安全，防止 SSRF 攻击
func isSafeSSRFURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}

	host := u.Hostname()
	ips, err := net.LookupIP(host)
	if err != nil {
		return err
	}

	for _, ip := range ips {
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("URL resolves to private or restricted IP: %s", ip.String())
		}
	}

	return nil
}

// Override for testing
var ssrfBypassForTest bool

// executeCustomAPI 执行自定义 API 工具
func (e *ToolExecutor) executeCustomAPI(ctx context.Context, tool *Tool, toolCall *ToolCall) (*ExecuteResult, error) {
	if tool.ServerID == "" {
		return nil, fmt.Errorf("custom API endpoint not specified")
	}
	if !ssrfBypassForTest {
		if err := isSafeSSRFURL(tool.ServerID); err != nil {
			return nil, fmt.Errorf("custom API endpoint is not allowed (SSRF protection): %w", err)
		}
	}
	arguments := toolCall.Arguments
	if arguments == nil {
		arguments = map[string]any{}
	}
	body, err := json.Marshal(arguments)
	if err != nil {
		return nil, fmt.Errorf("marshal custom API arguments: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tool.ServerID, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create custom API request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return &ExecuteResult{
			Content: err.Error(),
			IsError: true,
		}, nil
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read custom API response: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return &ExecuteResult{
			Content: fmt.Sprintf("custom API error: status=%d body=%s", resp.StatusCode, string(respBody)),
			IsError: true,
		}, nil
	}

	return &ExecuteResult{
		Content: string(respBody),
	}, nil
}

func (e *ToolExecutor) executeCustomPython(ctx context.Context, tool *Tool, toolCall *ToolCall) (*ExecuteResult, error) {
	sourceCode := strings.TrimSpace(tool.SourceCode)
	if sourceCode == "" {
		return nil, fmt.Errorf("custom Python source code not specified")
	}
	arguments := toolCall.Arguments
	if arguments == nil {
		arguments = map[string]any{}
	}
	timeout := customPythonTimeout(tool.TimeoutSeconds)
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	payload, err := json.Marshal(map[string]any{
		"args": arguments,
		"code": sourceCode,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal custom Python payload: %w", err)
	}

	cmd := exec.CommandContext(runCtx, "python3", "-I", "-S", "-c", customPythonWrapper)
	cmd.Stdin = bytes.NewReader(payload)
	output, err := cmd.CombinedOutput()
	if runCtx.Err() == context.DeadlineExceeded {
		return &ExecuteResult{
			Content: fmt.Sprintf("custom Python timed out after %s", timeout),
			IsError: true,
		}, nil
	}
	if err != nil {
		return &ExecuteResult{
			Content: fmt.Sprintf("custom Python error: %s", strings.TrimSpace(string(output))),
			IsError: true,
		}, nil
	}

	return &ExecuteResult{
		Content: strings.TrimSpace(string(output)),
	}, nil
}

func customPythonTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		seconds = 5
	}
	if seconds > 30 {
		seconds = 30
	}
	return time.Duration(seconds) * time.Second
}

func normalizeCustomToolRuntime(runtime string) string {
	switch strings.ToLower(strings.TrimSpace(runtime)) {
	case "python":
		return "python"
	default:
		return "api"
	}
}

const customPythonWrapper = `
import json
import math
import re
import sys

payload = json.loads(sys.stdin.read() or "{}")
args = payload.get("args") or {}
code = payload.get("code") or ""
safe_builtins = {
    "abs": abs,
    "all": all,
    "any": any,
    "bool": bool,
    "dict": dict,
    "enumerate": enumerate,
    "float": float,
    "int": int,
    "len": len,
    "list": list,
    "max": max,
    "min": min,
    "range": range,
    "round": round,
    "set": set,
    "sorted": sorted,
    "str": str,
    "sum": sum,
    "tuple": tuple,
}
globals_dict = {"__builtins__": safe_builtins, "json": json, "math": math, "re": re}
locals_dict = {}
try:
    exec(code, globals_dict, locals_dict)
    main = locals_dict.get("main") or globals_dict.get("main")
    if callable(main):
        result = main(args)
    else:
        result = locals_dict.get("result")
    if isinstance(result, str):
        sys.stdout.write(result)
    else:
        sys.stdout.write(json.dumps(result, ensure_ascii=False))
except Exception as exc:
    print(f"{exc.__class__.__name__}: {exc}", file=sys.stderr)
    sys.exit(1)
`

// GetToolDefinitions 获取 Agent 可用的工具定义
func (e *ToolExecutor) GetToolDefinitions(ctx context.Context, agent *Agent) ([]ToolDefinition, error) {
	var definitions []ToolDefinition

	for _, t := range agent.Tools {
		if !t.Enabled {
			continue
		}

		def := ToolDefinition{
			Name:             t.Name,
			Description:      t.Description,
			ToolType:         normalizeToolType(t.Type),
			RequiresApproval: t.RequiresApproval,
			RiskLevel:        toolDefinitionRiskLevel(t),
		}

		// 获取 InputSchema
		switch t.Type {
		case "builtin":
			if !e.isCommercialBuiltinEnabled(t.Name) {
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
		case "custom":
			def.InputSchema = t.InputSchema
		}

		definitions = append(definitions, def)
	}

	return definitions, nil
}

func (e *ToolExecutor) isCommercialBuiltinEnabled(name string) bool {
	if mcp.IsDefaultCommercialBuiltin(name) {
		return true
	}
	return name == "web_search" && e.webSearchProvider != nil
}

// ToolDefinition 工具定义（用于 OpenAI function calling）
type ToolDefinition struct {
	Name             string `json:"name"`
	Description      string `json:"description,omitempty"`
	InputSchema      any    `json:"inputSchema,omitempty"`
	ToolType         string `json:"toolType"`
	RequiresApproval bool   `json:"requiresApproval"`
	RiskLevel        string `json:"riskLevel"`
}

func normalizeToolType(value string) string {
	switch value {
	case "builtin", "mcp", "custom":
		return value
	default:
		return "builtin"
	}
}

func toolDefinitionRiskLevel(tool Tool) string {
	if normalized := normalizeToolRiskLevel(tool.RiskLevel); normalized != "" {
		return normalized
	}
	return inferToolRiskLevel(tool.Name)
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
