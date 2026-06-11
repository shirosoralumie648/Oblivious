package mcp

import (
	"context"
	"encoding/json"
	"regexp"
)

func init() {
	registerBuiltins(map[string]BuiltinTool{
		"regex_match":    &RegexMatchTool{},
		"regex_find":     &RegexFindTool{},
		"regex_find_all": &RegexFindAllTool{},
		"regex_replace":  &RegexReplaceTool{},
		"regex_split":    &RegexSplitTool{},
		"regex_extract":  &RegexExtractTool{},
		"regex_validate": &RegexValidateTool{},
		"regex_escape":   &RegexEscapeTool{},
	}, map[string]bool{
		"regex_match":    true,
		"regex_find":     true,
		"regex_find_all": true,
		"regex_replace":  true,
		"regex_split":    true,
		"regex_extract":  true,
		"regex_validate": true,
		"regex_escape":   true,
	})
}

type RegexMatchTool struct{}

func (t *RegexMatchTool) Name() string {
	return "regex_match"
}

func (t *RegexMatchTool) Description() string {
	return "Check if text matches regex pattern"
}

func (t *RegexMatchTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "Text to match against",
				"default":     "",
			},
			"pattern": map[string]any{
				"type":        "string",
				"description": "Regular expression pattern",
				"default":     ".*",
			},
		},
	}
}

func (t *RegexMatchTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	pattern, _ := args["pattern"].(string)
	if pattern == "" {
		pattern = ".*"
	}

	matched, err := regexp.MatchString(pattern, text)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}

	if matched {
		return &ToolResult{Content: "true"}, nil
	}
	return &ToolResult{Content: "false"}, nil
}

type RegexFindTool struct{}

func (t *RegexFindTool) Name() string {
	return "regex_find"
}

func (t *RegexFindTool) Description() string {
	return "Find first regex match"
}

func (t *RegexFindTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "Text to search in",
				"default":     "",
			},
			"pattern": map[string]any{
				"type":        "string",
				"description": "Regular expression pattern",
				"default":     ".*",
			},
		},
	}
}

func (t *RegexFindTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	pattern, _ := args["pattern"].(string)
	if pattern == "" {
		pattern = ".*"
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}

	match := re.FindString(text)
	return &ToolResult{Content: match}, nil
}

type RegexFindAllTool struct{}

func (t *RegexFindAllTool) Name() string {
	return "regex_find_all"
}

func (t *RegexFindAllTool) Description() string {
	return "Find all regex matches"
}

func (t *RegexFindAllTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "Text to search in",
				"default":     "",
			},
			"pattern": map[string]any{
				"type":        "string",
				"description": "Regular expression pattern",
				"default":     ".*",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of matches (-1 for all)",
				"default":     -1,
			},
		},
	}
}

func (t *RegexFindAllTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	pattern, _ := args["pattern"].(string)
	if pattern == "" {
		pattern = ".*"
	}

	limit := -1
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	} else if l, ok := args["limit"].(int); ok {
		limit = l
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}

	matches := re.FindAllString(text, limit)
	if matches == nil {
		matches = []string{}
	}

	data, _ := json.Marshal(matches)
	return &ToolResult{Content: string(data)}, nil
}

type RegexReplaceTool struct{}

func (t *RegexReplaceTool) Name() string {
	return "regex_replace"
}

func (t *RegexReplaceTool) Description() string {
	return "Replace text matching regex"
}

func (t *RegexReplaceTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "Text to perform replacement on",
				"default":     "",
			},
			"pattern": map[string]any{
				"type":        "string",
				"description": "Regular expression pattern",
				"default":     ".*",
			},
			"replacement": map[string]any{
				"type":        "string",
				"description": "Replacement text",
				"default":     "",
			},
		},
	}
}

func (t *RegexReplaceTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	pattern, _ := args["pattern"].(string)
	if pattern == "" {
		pattern = ".*"
	}
	replacement, _ := args["replacement"].(string)

	re, err := regexp.Compile(pattern)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}

	result := re.ReplaceAllString(text, replacement)
	return &ToolResult{Content: result}, nil
}

type RegexSplitTool struct{}

func (t *RegexSplitTool) Name() string {
	return "regex_split"
}

func (t *RegexSplitTool) Description() string {
	return "Split text by regex pattern"
}

func (t *RegexSplitTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "Text to split",
				"default":     "",
			},
			"pattern": map[string]any{
				"type":        "string",
				"description": "Regular expression pattern",
				"default":     "\\s+",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Maximum number of splits (-1 for all)",
				"default":     -1,
			},
		},
	}
}

func (t *RegexSplitTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	pattern, _ := args["pattern"].(string)
	if pattern == "" {
		pattern = "\\s+"
	}

	limit := -1
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	} else if l, ok := args["limit"].(int); ok {
		limit = l
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}

	parts := re.Split(text, limit)
	data, _ := json.Marshal(parts)
	return &ToolResult{Content: string(data)}, nil
}

type RegexExtractTool struct{}

func (t *RegexExtractTool) Name() string {
	return "regex_extract"
}

func (t *RegexExtractTool) Description() string {
	return "Extract named capture groups"
}

func (t *RegexExtractTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "Text to extract from",
				"default":     "",
			},
			"pattern": map[string]any{
				"type":        "string",
				"description": "Regular expression with named groups",
				"default":     "(?P<match>.*)",
			},
		},
	}
}

func (t *RegexExtractTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	pattern, _ := args["pattern"].(string)
	if pattern == "" {
		pattern = "(?P<match>.*)"
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}

	match := re.FindStringSubmatch(text)
	if match == nil {
		data, _ := json.Marshal(map[string]string{})
		return &ToolResult{Content: string(data)}, nil
	}

	result := make(map[string]string)
	names := re.SubexpNames()
	for i, name := range names {
		if i > 0 && name != "" && i < len(match) {
			result[name] = match[i]
		}
	}

	data, _ := json.Marshal(result)
	return &ToolResult{Content: string(data)}, nil
}

type RegexValidateTool struct{}

func (t *RegexValidateTool) Name() string {
	return "regex_validate"
}

func (t *RegexValidateTool) Description() string {
	return "Validate regex pattern syntax"
}

func (t *RegexValidateTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Regular expression pattern to validate",
				"default":     ".*",
			},
		},
	}
}

func (t *RegexValidateTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	pattern, _ := args["pattern"].(string)
	if pattern == "" {
		pattern = ".*"
	}

	_, err := regexp.Compile(pattern)
	if err != nil {
		return &ToolResult{Content: err.Error()}, nil
	}

	return &ToolResult{Content: "valid"}, nil
}

type RegexEscapeTool struct{}

func (t *RegexEscapeTool) Name() string {
	return "regex_escape"
}

func (t *RegexEscapeTool) Description() string {
	return "Escape special regex characters"
}

func (t *RegexEscapeTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "Text to escape",
				"default":     "",
			},
		},
	}
}

func (t *RegexEscapeTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	escaped := regexp.QuoteMeta(text)
	return &ToolResult{Content: escaped}, nil
}
