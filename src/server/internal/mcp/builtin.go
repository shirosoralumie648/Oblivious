package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// BuiltinTool 内置工具接口
type BuiltinTool interface {
	Name() string
	Description() string
	InputSchema() any
	Execute(ctx context.Context, args map[string]any) (*ToolResult, error)
}

type WebSearchResult struct {
	Title   string
	URL     string
	Snippet string
}

type WebSearchProvider interface {
	Search(ctx context.Context, query string) ([]WebSearchResult, error)
}

var webSearchProvider WebSearchProvider

func SetWebSearchProvider(provider WebSearchProvider) {
	webSearchProvider = provider
}

func SetWebSearchProviderForTest(provider WebSearchProvider) func() {
	previous := webSearchProvider
	webSearchProvider = provider
	return func() {
		webSearchProvider = previous
	}
}

func WebSearchProviderConfigured() bool {
	return webSearchProvider != nil
}

// BuiltinTools 内置工具集合
var BuiltinTools = map[string]BuiltinTool{
	"web_search":     &WebSearchTool{},
	"calculator":     &CalculatorTool{},
	"datetime":       &DatetimeTool{},
	"http_request":   &HTTPRequestTool{},
	"json_formatter": &JSONFormatterTool{},
	"text_transform": &TextTransformTool{},
}

var defaultCommercialBuiltinEnabled = map[string]bool{
	"calculator":     true,
	"datetime":       true,
	"json_formatter": true,
	"text_transform": true,
	"web_search":     false,
	"http_request":   false,
}

// registerBuiltins merges category tool maps into BuiltinTools and the
// default commercial policy table. Category files (builtin_<category>.go)
// call it from init() so parallel implementations never edit shared maps.
// Tools that reach the network or other external systems must register with
// defaultCommercialEnabled=false, matching the existing commercial policy.
func registerBuiltins(tools map[string]BuiltinTool, defaultCommercialEnabled map[string]bool) {
	for name, tool := range tools {
		if _, exists := BuiltinTools[name]; exists {
			panic("mcp: duplicate builtin tool " + name)
		}
		BuiltinTools[name] = tool
	}
	for name, enabled := range defaultCommercialEnabled {
		defaultCommercialBuiltinEnabled[name] = enabled
	}
}

// IsDefaultCommercialBuiltin reports whether a builtin is safe and real enough
// to expose by default in commercial Agent tool definitions.
func IsDefaultCommercialBuiltin(name string) bool {
	return defaultCommercialBuiltinEnabled[name]
}

func disabledBuiltinResult(name, reason string) *ToolResult {
	return &ToolResult{
		Content: fmt.Sprintf("%s is disabled for default commercial use: %s", name, reason),
		IsError: true,
	}
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
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query is required")
	}
	query = strings.TrimSpace(query)
	if webSearchProvider != nil {
		results, err := webSearchProvider.Search(ctx, query)
		if err != nil {
			return &ToolResult{Content: err.Error(), IsError: true}, nil
		}
		return &ToolResult{Content: formatWebSearchResults(results)}, nil
	}

	return disabledBuiltinResult("web_search", "no search provider is configured"), nil
}

func formatWebSearchResults(results []WebSearchResult) string {
	if len(results) == 0 {
		return "No search results found."
	}
	var builder strings.Builder
	for i, result := range results {
		if i > 0 {
			builder.WriteString("\n\n")
		}
		builder.WriteString(fmt.Sprintf("%d. %s", i+1, strings.TrimSpace(result.Title)))
		if strings.TrimSpace(result.URL) != "" {
			builder.WriteString("\n")
			builder.WriteString(strings.TrimSpace(result.URL))
		}
		if strings.TrimSpace(result.Snippet) != "" {
			builder.WriteString("\n")
			builder.WriteString(strings.TrimSpace(result.Snippet))
		}
	}
	return builder.String()
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
	expression = strings.TrimSpace(expression)
	if expression == "" {
		return nil, fmt.Errorf("expression is required")
	}

	value, err := evaluateArithmetic(expression)
	if err != nil {
		return nil, err
	}

	return &ToolResult{Content: "Result: " + strconv.FormatFloat(value, 'f', -1, 64)}, nil
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

// JSONFormatterTool formats or compacts caller-provided JSON without I/O.
type JSONFormatterTool struct{}

func (t *JSONFormatterTool) Name() string {
	return "json_formatter"
}

func (t *JSONFormatterTool) Description() string {
	return "Format or compact JSON text"
}

func (t *JSONFormatterTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"json": map[string]any{
				"type":        "string",
				"description": "JSON text to format",
			},
			"format": map[string]any{
				"type":        "string",
				"description": "Output format",
				"enum":        []string{"pretty", "compact"},
				"default":     "pretty",
			},
		},
		"required": []string{"json"},
	}
}

func (t *JSONFormatterTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	input, ok := args["json"].(string)
	if !ok || strings.TrimSpace(input) == "" {
		return nil, fmt.Errorf("json is required")
	}

	format, _ := args["format"].(string)
	format = strings.TrimSpace(strings.ToLower(format))
	if format == "" {
		format = "pretty"
	}

	var value any
	decoder := json.NewDecoder(strings.NewReader(input))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return &ToolResult{Content: "invalid JSON: " + err.Error(), IsError: true}, nil
	}
	if decoder.Decode(&value) != io.EOF {
		return &ToolResult{Content: "invalid JSON: multiple JSON values provided", IsError: true}, nil
	}

	var output []byte
	var err error
	switch format {
	case "pretty":
		output, err = json.MarshalIndent(value, "", "  ")
	case "compact":
		output, err = json.Marshal(value)
	default:
		return nil, fmt.Errorf("unsupported format %q", format)
	}
	if err != nil {
		return &ToolResult{Content: "invalid JSON: " + err.Error(), IsError: true}, nil
	}
	return &ToolResult{Content: string(output)}, nil
}

// TextTransformTool applies deterministic string transforms without I/O.
type TextTransformTool struct{}

func (t *TextTransformTool) Name() string {
	return "text_transform"
}

func (t *TextTransformTool) Description() string {
	return "Apply deterministic text transformations"
}

func (t *TextTransformTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "Text to transform",
			},
			"operation": map[string]any{
				"type":        "string",
				"description": "Text transform operation",
				"enum":        []string{"uppercase", "lowercase", "trim", "collapse_whitespace"},
			},
		},
		"required": []string{"text", "operation"},
	}
}

func (t *TextTransformTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	text, ok := args["text"].(string)
	if !ok {
		return nil, fmt.Errorf("text is required")
	}
	operation, ok := args["operation"].(string)
	if !ok || strings.TrimSpace(operation) == "" {
		return nil, fmt.Errorf("operation is required")
	}

	switch strings.TrimSpace(strings.ToLower(operation)) {
	case "uppercase":
		return &ToolResult{Content: strings.ToUpper(text)}, nil
	case "lowercase":
		return &ToolResult{Content: strings.ToLower(text)}, nil
	case "trim":
		return &ToolResult{Content: strings.TrimSpace(text)}, nil
	case "collapse_whitespace":
		return &ToolResult{Content: strings.Join(strings.Fields(text), " ")}, nil
	default:
		return nil, fmt.Errorf("unsupported operation %q", operation)
	}
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
	_ = ctx

	if method == "" {
		method = "GET"
	}
	if url == "" {
		return nil, fmt.Errorf("url is required")
	}

	return disabledBuiltinResult("http_request", "tenant-safe outbound HTTP policy is not configured"), nil
}

// GetBuiltinTool 获取内置工具
func GetBuiltinTool(name string) (BuiltinTool, bool) {
	tool, ok := BuiltinTools[name]
	return tool, ok
}

// ListBuiltinTools 列出所有内置工具
func ListBuiltinTools() []ToolDefinition {
	tools := make([]ToolDefinition, 0, len(BuiltinTools))
	names := make([]string, 0, len(BuiltinTools))
	for name := range BuiltinTools {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		tool := BuiltinTools[name]
		tools = append(tools, ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.InputSchema(),
		})
	}
	return tools
}

// ListDefaultCommercialBuiltinTools returns only builtins that may be exposed
// to commercial Agents without extra provider or outbound-network policy.
func ListDefaultCommercialBuiltinTools() []ToolDefinition {
	tools := make([]ToolDefinition, 0, len(BuiltinTools))
	names := make([]string, 0, len(BuiltinTools))
	for name := range BuiltinTools {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !IsDefaultCommercialBuiltin(name) {
			continue
		}
		tool := BuiltinTools[name]
		tools = append(tools, ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.InputSchema(),
		})
	}
	return tools
}

type arithmeticParser struct {
	input string
	pos   int
}

func evaluateArithmetic(input string) (float64, error) {
	p := &arithmeticParser{input: input}
	value, err := p.parseExpression()
	if err != nil {
		return 0, err
	}
	p.skipSpaces()
	if p.pos != len(p.input) {
		return 0, fmt.Errorf("invalid expression: unexpected token %q", p.input[p.pos:])
	}
	return value, nil
}

func (p *arithmeticParser) parseExpression() (float64, error) {
	value, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpaces()
		if p.match('+') {
			next, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			value += next
			continue
		}
		if p.match('-') {
			next, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			value -= next
			continue
		}
		return value, nil
	}
}

func (p *arithmeticParser) parseTerm() (float64, error) {
	value, err := p.parseUnary()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpaces()
		if p.match('*') {
			next, err := p.parseUnary()
			if err != nil {
				return 0, err
			}
			value *= next
			continue
		}
		if p.match('/') {
			next, err := p.parseUnary()
			if err != nil {
				return 0, err
			}
			if next == 0 {
				return 0, fmt.Errorf("invalid expression: division by zero")
			}
			value /= next
			continue
		}
		return value, nil
	}
}

func (p *arithmeticParser) parseUnary() (float64, error) {
	p.skipSpaces()
	if p.match('+') {
		return p.parseUnary()
	}
	if p.match('-') {
		value, err := p.parseUnary()
		if err != nil {
			return 0, err
		}
		return -value, nil
	}
	return p.parsePrimary()
}

func (p *arithmeticParser) parsePrimary() (float64, error) {
	p.skipSpaces()
	if p.match('(') {
		value, err := p.parseExpression()
		if err != nil {
			return 0, err
		}
		p.skipSpaces()
		if !p.match(')') {
			return 0, fmt.Errorf("invalid expression: missing closing parenthesis")
		}
		return value, nil
	}
	return p.parseNumber()
}

func (p *arithmeticParser) parseNumber() (float64, error) {
	p.skipSpaces()
	start := p.pos
	sawDigit := false
	sawDot := false

	for p.pos < len(p.input) {
		r := rune(p.input[p.pos])
		switch {
		case unicode.IsDigit(r):
			sawDigit = true
			p.pos++
		case r == '.' && !sawDot:
			sawDot = true
			p.pos++
		default:
			goto done
		}
	}

done:
	if !sawDigit {
		return 0, fmt.Errorf("invalid expression: expected number at %q", p.input[start:])
	}
	value, err := strconv.ParseFloat(p.input[start:p.pos], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid expression: %w", err)
	}
	return value, nil
}

func (p *arithmeticParser) skipSpaces() {
	for p.pos < len(p.input) && unicode.IsSpace(rune(p.input[p.pos])) {
		p.pos++
	}
}

func (p *arithmeticParser) match(ch byte) bool {
	if p.pos >= len(p.input) || p.input[p.pos] != ch {
		return false
	}
	p.pos++
	return true
}
