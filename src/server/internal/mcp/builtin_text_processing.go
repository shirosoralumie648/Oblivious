package mcp

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"unicode/utf8"
)

func init() {
	registerBuiltins(map[string]BuiltinTool{
		"text_uppercase":   &TextUppercaseTool{},
		"text_lowercase":   &TextLowercaseTool{},
		"text_titlecase":   &TextTitlecaseTool{},
		"text_trim":        &TextTrimTool{},
		"text_trim_prefix": &TextTrimPrefixTool{},
		"text_trim_suffix": &TextTrimSuffixTool{},
		"text_replace":     &TextReplaceTool{},
		"text_split":       &TextSplitTool{},
		"text_join":        &TextJoinTool{},
		"text_reverse":     &TextReverseTool{},
		"text_truncate":    &TextTruncateTool{},
		"text_pad_left":    &TextPadLeftTool{},
		"text_pad_right":   &TextPadRightTool{},
		"text_word_count":  &TextWordCountTool{},
		"text_line_count":  &TextLineCountTool{},
		"text_char_count":  &TextCharCountTool{},
	}, map[string]bool{
		"text_uppercase":   true,
		"text_lowercase":   true,
		"text_titlecase":   true,
		"text_trim":        true,
		"text_trim_prefix": true,
		"text_trim_suffix": true,
		"text_replace":     true,
		"text_split":       true,
		"text_join":        true,
		"text_reverse":     true,
		"text_truncate":    true,
		"text_pad_left":    true,
		"text_pad_right":   true,
		"text_word_count":  true,
		"text_line_count":  true,
		"text_char_count":  true,
	})
}

type TextUppercaseTool struct{}

func (t *TextUppercaseTool) Name() string        { return "text_uppercase" }
func (t *TextUppercaseTool) Description() string { return "Convert text to uppercase" }
func (t *TextUppercaseTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{"type": "string", "description": "Text to convert", "default": ""},
		},
	}
}
func (t *TextUppercaseTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	return &ToolResult{Content: strings.ToUpper(text)}, nil
}

type TextLowercaseTool struct{}

func (t *TextLowercaseTool) Name() string        { return "text_lowercase" }
func (t *TextLowercaseTool) Description() string { return "Convert text to lowercase" }
func (t *TextLowercaseTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{"type": "string", "description": "Text to convert", "default": ""},
		},
	}
}
func (t *TextLowercaseTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	return &ToolResult{Content: strings.ToLower(text)}, nil
}

type TextTitlecaseTool struct{}

func (t *TextTitlecaseTool) Name() string        { return "text_titlecase" }
func (t *TextTitlecaseTool) Description() string { return "Convert text to title case" }
func (t *TextTitlecaseTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{"type": "string", "description": "Text to convert", "default": ""},
		},
	}
}
func (t *TextTitlecaseTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	return &ToolResult{Content: strings.Title(text)}, nil
}

type TextTrimTool struct{}

func (t *TextTrimTool) Name() string        { return "text_trim" }
func (t *TextTrimTool) Description() string { return "Trim whitespace from both ends" }
func (t *TextTrimTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{"type": "string", "description": "Text to trim", "default": ""},
		},
	}
}
func (t *TextTrimTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	return &ToolResult{Content: strings.TrimSpace(text)}, nil
}

type TextTrimPrefixTool struct{}

func (t *TextTrimPrefixTool) Name() string        { return "text_trim_prefix" }
func (t *TextTrimPrefixTool) Description() string { return "Trim prefix from text" }
func (t *TextTrimPrefixTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text":   map[string]any{"type": "string", "description": "Text to process", "default": ""},
			"prefix": map[string]any{"type": "string", "description": "Prefix to remove", "default": ""},
		},
	}
}
func (t *TextTrimPrefixTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	prefix, _ := args["prefix"].(string)
	return &ToolResult{Content: strings.TrimPrefix(text, prefix)}, nil
}

type TextTrimSuffixTool struct{}

func (t *TextTrimSuffixTool) Name() string        { return "text_trim_suffix" }
func (t *TextTrimSuffixTool) Description() string { return "Trim suffix from text" }
func (t *TextTrimSuffixTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text":   map[string]any{"type": "string", "description": "Text to process", "default": ""},
			"suffix": map[string]any{"type": "string", "description": "Suffix to remove", "default": ""},
		},
	}
}
func (t *TextTrimSuffixTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	suffix, _ := args["suffix"].(string)
	return &ToolResult{Content: strings.TrimSuffix(text, suffix)}, nil
}

type TextReplaceTool struct{}

func (t *TextReplaceTool) Name() string        { return "text_replace" }
func (t *TextReplaceTool) Description() string { return "Replace all occurrences of substring" }
func (t *TextReplaceTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{"type": "string", "description": "Text to process", "default": ""},
			"old":  map[string]any{"type": "string", "description": "Substring to replace", "default": ""},
			"new":  map[string]any{"type": "string", "description": "Replacement substring", "default": ""},
		},
	}
}
func (t *TextReplaceTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	old, _ := args["old"].(string)
	new, _ := args["new"].(string)
	return &ToolResult{Content: strings.ReplaceAll(text, old, new)}, nil
}

type TextSplitTool struct{}

func (t *TextSplitTool) Name() string        { return "text_split" }
func (t *TextSplitTool) Description() string { return "Split text by delimiter" }
func (t *TextSplitTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text":      map[string]any{"type": "string", "description": "Text to split", "default": ""},
			"delimiter": map[string]any{"type": "string", "description": "Delimiter string", "default": ","},
		},
	}
}
func (t *TextSplitTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	delimiter, ok := args["delimiter"].(string)
	if !ok || delimiter == "" {
		delimiter = ","
	}
	parts := strings.Split(text, delimiter)
	data, _ := json.Marshal(parts)
	return &ToolResult{Content: string(data)}, nil
}

type TextJoinTool struct{}

func (t *TextJoinTool) Name() string        { return "text_join" }
func (t *TextJoinTool) Description() string { return "Join array with delimiter" }
func (t *TextJoinTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"parts":     map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Array of strings", "default": []string{}},
			"delimiter": map[string]any{"type": "string", "description": "Delimiter string", "default": ","},
		},
	}
}
func (t *TextJoinTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	partsRaw, ok := args["parts"].([]any)
	var parts []string
	if ok {
		for _, p := range partsRaw {
			if s, ok := p.(string); ok {
				parts = append(parts, s)
			}
		}
	}
	delimiter, ok := args["delimiter"].(string)
	if !ok || delimiter == "" {
		delimiter = ","
	}
	return &ToolResult{Content: strings.Join(parts, delimiter)}, nil
}

type TextReverseTool struct{}

func (t *TextReverseTool) Name() string        { return "text_reverse" }
func (t *TextReverseTool) Description() string { return "Reverse text (Unicode-aware)" }
func (t *TextReverseTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{"type": "string", "description": "Text to reverse", "default": ""},
		},
	}
}
func (t *TextReverseTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	runes := []rune(text)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return &ToolResult{Content: string(runes)}, nil
}

type TextTruncateTool struct{}

func (t *TextTruncateTool) Name() string        { return "text_truncate" }
func (t *TextTruncateTool) Description() string { return "Truncate text to max length" }
func (t *TextTruncateTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text":       map[string]any{"type": "string", "description": "Text to truncate", "default": ""},
			"max_length": map[string]any{"type": "integer", "description": "Maximum length", "default": 50},
			"suffix":     map[string]any{"type": "string", "description": "Suffix to append", "default": "..."},
		},
	}
}
func (t *TextTruncateTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	maxLength := 50
	if ml, ok := args["max_length"].(float64); ok {
		maxLength = int(ml)
	} else if ml, ok := args["max_length"].(int); ok {
		maxLength = ml
	}
	suffix, ok := args["suffix"].(string)
	if !ok {
		suffix = "..."
	}
	runes := []rune(text)
	if len(runes) <= maxLength {
		return &ToolResult{Content: text}, nil
	}
	return &ToolResult{Content: string(runes[:maxLength]) + suffix}, nil
}

type TextPadLeftTool struct{}

func (t *TextPadLeftTool) Name() string        { return "text_pad_left" }
func (t *TextPadLeftTool) Description() string { return "Pad text on the left" }
func (t *TextPadLeftTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text":     map[string]any{"type": "string", "description": "Text to pad", "default": ""},
			"length":   map[string]any{"type": "integer", "description": "Target length", "default": 10},
			"pad_char": map[string]any{"type": "string", "description": "Padding character", "default": " "},
		},
	}
}
func (t *TextPadLeftTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	length := 10
	if l, ok := args["length"].(float64); ok {
		length = int(l)
	} else if l, ok := args["length"].(int); ok {
		length = l
	}
	padChar, ok := args["pad_char"].(string)
	if !ok || padChar == "" {
		padChar = " "
	}
	runes := []rune(text)
	if len(runes) >= length {
		return &ToolResult{Content: text}, nil
	}
	padRune := []rune(padChar)[0]
	padding := strings.Repeat(string(padRune), length-len(runes))
	return &ToolResult{Content: padding + text}, nil
}

type TextPadRightTool struct{}

func (t *TextPadRightTool) Name() string        { return "text_pad_right" }
func (t *TextPadRightTool) Description() string { return "Pad text on the right" }
func (t *TextPadRightTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text":     map[string]any{"type": "string", "description": "Text to pad", "default": ""},
			"length":   map[string]any{"type": "integer", "description": "Target length", "default": 10},
			"pad_char": map[string]any{"type": "string", "description": "Padding character", "default": " "},
		},
	}
}
func (t *TextPadRightTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	length := 10
	if l, ok := args["length"].(float64); ok {
		length = int(l)
	} else if l, ok := args["length"].(int); ok {
		length = l
	}
	padChar, ok := args["pad_char"].(string)
	if !ok || padChar == "" {
		padChar = " "
	}
	runes := []rune(text)
	if len(runes) >= length {
		return &ToolResult{Content: text}, nil
	}
	padRune := []rune(padChar)[0]
	padding := strings.Repeat(string(padRune), length-len(runes))
	return &ToolResult{Content: text + padding}, nil
}

type TextWordCountTool struct{}

func (t *TextWordCountTool) Name() string        { return "text_word_count" }
func (t *TextWordCountTool) Description() string { return "Count words in text" }
func (t *TextWordCountTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{"type": "string", "description": "Text to count words", "default": ""},
		},
	}
}
func (t *TextWordCountTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	words := strings.Fields(text)
	return &ToolResult{Content: strconv.Itoa(len(words))}, nil
}

type TextLineCountTool struct{}

func (t *TextLineCountTool) Name() string        { return "text_line_count" }
func (t *TextLineCountTool) Description() string { return "Count lines in text" }
func (t *TextLineCountTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{"type": "string", "description": "Text to count lines", "default": ""},
		},
	}
}
func (t *TextLineCountTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	if text == "" {
		return &ToolResult{Content: "0"}, nil
	}
	count := strings.Count(text, "\n") + 1
	return &ToolResult{Content: strconv.Itoa(count)}, nil
}

type TextCharCountTool struct{}

func (t *TextCharCountTool) Name() string        { return "text_char_count" }
func (t *TextCharCountTool) Description() string { return "Count characters (runes) in text" }
func (t *TextCharCountTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{"type": "string", "description": "Text to count characters", "default": ""},
		},
	}
}
func (t *TextCharCountTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	count := utf8.RuneCountInString(text)
	return &ToolResult{Content: strconv.Itoa(count)}, nil
}
