package mcp

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// BuiltinTool 内置工具接口
type BuiltinTool interface {
	Name() string
	Description() string
	InputSchema() any
	Execute(ctx context.Context, args map[string]any) (*ToolResult, error)
}

// BuiltinTools 内置工具集合
var BuiltinTools = map[string]BuiltinTool{
	"web_search":   &WebSearchTool{},
	"calculator":   &CalculatorTool{},
	"datetime":     &DatetimeTool{},
	"http_request": &HTTPRequestTool{},
}

// WebSearchTool 网页搜索工具
type WebSearchTool struct{}

func (t *WebSearchTool) Name() string {
	return "web_search"
}

func (t *WebSearchTool) Description() string {
	return "Search the web for information"
}

func (t *WebSearchTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "The search query",
			},
		},
		"required": []string{"query"},
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	query, ok := args["query"].(string)
	if !ok {
		return nil, fmt.Errorf("query is required")
	}

	// 简化实现：返回占位结果
	// 实际实现需要集成搜索 API
	return &ToolResult{
		Content: fmt.Sprintf("Search results for: %s (placeholder - integrate with search API)", query),
	}, nil
}

// CalculatorTool 计算器工具
type CalculatorTool struct{}

func (t *CalculatorTool) Name() string {
	return "calculator"
}

func (t *CalculatorTool) Description() string {
	return "Perform mathematical calculations"
}

func (t *CalculatorTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"expression": map[string]any{
				"type":        "string",
				"description": "Mathematical expression to evaluate",
			},
		},
		"required": []string{"expression"},
	}
}

func (t *CalculatorTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	expression, ok := args["expression"].(string)
	if !ok {
		return nil, fmt.Errorf("expression is required")
	}

	// 简化实现：仅支持基本运算
	// 实际实现应使用表达式解析库
	result := fmt.Sprintf("Result of '%s' = (placeholder - implement expression parser)", expression)
	return &ToolResult{Content: result}, nil
}

// DatetimeTool 日期时间工具
type DatetimeTool struct{}

func (t *DatetimeTool) Name() string {
	return "datetime"
}

func (t *DatetimeTool) Description() string {
	return "Get current date and time"
}

func (t *DatetimeTool) InputSchema() any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *DatetimeTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	now := time.Now()
	return &ToolResult{
		Content: fmt.Sprintf("Current date and time: %s", now.Format(time.RFC3339)),
	}, nil
}

// HTTPRequestTool HTTP 请求工具
type HTTPRequestTool struct {
	client *http.Client
}

func (t *HTTPRequestTool) Name() string {
	return "http_request"
}

func (t *HTTPRequestTool) Description() string {
	return "Make HTTP requests to external APIs"
}

func (t *HTTPRequestTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"method": map[string]any{
				"type":        "string",
				"description": "HTTP method (GET, POST, etc.)",
				"enum":        []string{"GET", "POST", "PUT", "DELETE"},
			},
			"url": map[string]any{
				"type":        "string",
				"description": "The URL to request",
			},
			"headers": map[string]any{
				"type":        "object",
				"description": "HTTP headers",
			},
			"body": map[string]any{
				"type":        "string",
				"description": "Request body (for POST/PUT)",
			},
		},
		"required": []string{"method", "url"},
	}
}

func (t *HTTPRequestTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	method, _ := args["method"].(string)
	url, _ := args["url"].(string)

	if method == "" {
		method = "GET"
	}
	if url == "" {
		return nil, fmt.Errorf("url is required")
	}

	if t.client == nil {
		t.client = &http.Client{Timeout: 30 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 添加 headers
	if headers, ok := args["headers"].(map[string]any); ok {
		for k, v := range headers {
			if vs, ok := v.(string); ok {
				req.Header.Set(k, vs)
			}
		}
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	return &ToolResult{
		Content: fmt.Sprintf("HTTP %s %s -> Status: %d", method, url, resp.StatusCode),
	}, nil
}

// GetBuiltinTool 获取内置工具
func GetBuiltinTool(name string) (BuiltinTool, bool) {
	tool, ok := BuiltinTools[name]
	return tool, ok
}

// ListBuiltinTools 列出所有内置工具
func ListBuiltinTools() []ToolDefinition {
	tools := make([]ToolDefinition, 0, len(BuiltinTools))
	for _, tool := range BuiltinTools {
		tools = append(tools, ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.InputSchema(),
		})
	}
	return tools
}
