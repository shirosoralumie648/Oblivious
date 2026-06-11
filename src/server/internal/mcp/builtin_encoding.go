package mcp

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"strings"
	"unicode/utf8"
)

func init() {
	registerBuiltins(map[string]BuiltinTool{
		"base64_encode":     &Base64EncodeTool{},
		"base64_decode":     &Base64DecodeTool{},
		"hex_encode":        &HexEncodeTool{},
		"hex_decode":        &HexDecodeTool{},
		"url_encode":        &URLEncodeTool{},
		"url_decode":        &URLDecodeTool{},
		"html_escape":       &HTMLEscapeTool{},
		"html_unescape":     &HTMLUnescapeTool{},
		"base64_url_encode": &Base64URLEncodeTool{},
		"base64_url_decode": &Base64URLDecodeTool{},
		"json_escape":       &JSONEscapeTool{},
		"csv_escape":        &CSVEscapeTool{},
	}, map[string]bool{
		"base64_encode":     true,
		"base64_decode":     true,
		"hex_encode":        true,
		"hex_decode":        true,
		"url_encode":        true,
		"url_decode":        true,
		"html_escape":       true,
		"html_unescape":     true,
		"base64_url_encode": true,
		"base64_url_decode": true,
		"json_escape":       true,
		"csv_escape":        true,
	})
}

type Base64EncodeTool struct{}

func (t *Base64EncodeTool) Name() string { return "base64_encode" }
func (t *Base64EncodeTool) Description() string {
	return "Encode text to base64"
}
func (t *Base64EncodeTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "Text to encode",
				"default":     "",
			},
		},
	}
}
func (t *Base64EncodeTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	return &ToolResult{Content: base64.StdEncoding.EncodeToString([]byte(text))}, nil
}

type Base64DecodeTool struct{}

func (t *Base64DecodeTool) Name() string { return "base64_decode" }
func (t *Base64DecodeTool) Description() string {
	return "Decode base64 to text"
}
func (t *Base64DecodeTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"encoded": map[string]any{
				"type":        "string",
				"description": "Base64 encoded text",
				"default":     "",
			},
		},
	}
}
func (t *Base64DecodeTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	encoded, _ := args["encoded"].(string)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return &ToolResult{Content: "invalid base64: " + err.Error(), IsError: true}, nil
	}
	if !utf8.Valid(decoded) {
		return &ToolResult{Content: "decoded bytes are not valid UTF-8", IsError: true}, nil
	}
	return &ToolResult{Content: string(decoded)}, nil
}

type HexEncodeTool struct{}

func (t *HexEncodeTool) Name() string { return "hex_encode" }
func (t *HexEncodeTool) Description() string {
	return "Encode bytes to hexadecimal"
}
func (t *HexEncodeTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "Text to encode",
				"default":     "",
			},
		},
	}
}
func (t *HexEncodeTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	return &ToolResult{Content: hex.EncodeToString([]byte(text))}, nil
}

type HexDecodeTool struct{}

func (t *HexDecodeTool) Name() string { return "hex_decode" }
func (t *HexDecodeTool) Description() string {
	return "Decode hexadecimal to text"
}
func (t *HexDecodeTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"hex": map[string]any{
				"type":        "string",
				"description": "Hex encoded text",
				"default":     "",
			},
		},
	}
}
func (t *HexDecodeTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	hexStr, _ := args["hex"].(string)
	decoded, err := hex.DecodeString(hexStr)
	if err != nil {
		return &ToolResult{Content: "invalid hex: " + err.Error(), IsError: true}, nil
	}
	if !utf8.Valid(decoded) {
		return &ToolResult{Content: "decoded bytes are not valid UTF-8", IsError: true}, nil
	}
	return &ToolResult{Content: string(decoded)}, nil
}

type URLEncodeTool struct{}

func (t *URLEncodeTool) Name() string { return "url_encode" }
func (t *URLEncodeTool) Description() string {
	return "URL-encode text"
}
func (t *URLEncodeTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "Text to encode",
				"default":     "",
			},
		},
	}
}
func (t *URLEncodeTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	return &ToolResult{Content: url.QueryEscape(text)}, nil
}

type URLDecodeTool struct{}

func (t *URLDecodeTool) Name() string { return "url_decode" }
func (t *URLDecodeTool) Description() string {
	return "URL-decode text"
}
func (t *URLDecodeTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"encoded": map[string]any{
				"type":        "string",
				"description": "URL encoded text",
				"default":     "",
			},
		},
	}
}
func (t *URLDecodeTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	encoded, _ := args["encoded"].(string)
	decoded, err := url.QueryUnescape(encoded)
	if err != nil {
		return &ToolResult{Content: "invalid URL encoding: " + err.Error(), IsError: true}, nil
	}
	return &ToolResult{Content: decoded}, nil
}

type HTMLEscapeTool struct{}

func (t *HTMLEscapeTool) Name() string { return "html_escape" }
func (t *HTMLEscapeTool) Description() string {
	return "Escape HTML special characters"
}
func (t *HTMLEscapeTool) InputSchema() any {
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
func (t *HTMLEscapeTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	return &ToolResult{Content: html.EscapeString(text)}, nil
}

type HTMLUnescapeTool struct{}

func (t *HTMLUnescapeTool) Name() string { return "html_unescape" }
func (t *HTMLUnescapeTool) Description() string {
	return "Unescape HTML entities"
}
func (t *HTMLUnescapeTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "HTML text to unescape",
				"default":     "",
			},
		},
	}
}
func (t *HTMLUnescapeTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	return &ToolResult{Content: html.UnescapeString(text)}, nil
}

type Base64URLEncodeTool struct{}

func (t *Base64URLEncodeTool) Name() string { return "base64_url_encode" }
func (t *Base64URLEncodeTool) Description() string {
	return "Encode to base64 URL-safe format"
}
func (t *Base64URLEncodeTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "Text to encode",
				"default":     "",
			},
		},
	}
}
func (t *Base64URLEncodeTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	return &ToolResult{Content: base64.URLEncoding.EncodeToString([]byte(text))}, nil
}

type Base64URLDecodeTool struct{}

func (t *Base64URLDecodeTool) Name() string { return "base64_url_decode" }
func (t *Base64URLDecodeTool) Description() string {
	return "Decode base64 URL-safe format"
}
func (t *Base64URLDecodeTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"encoded": map[string]any{
				"type":        "string",
				"description": "Base64 URL-encoded text",
				"default":     "",
			},
		},
	}
}
func (t *Base64URLDecodeTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	encoded, _ := args["encoded"].(string)
	decoded, err := base64.URLEncoding.DecodeString(encoded)
	if err != nil {
		return &ToolResult{Content: "invalid base64 URL encoding: " + err.Error(), IsError: true}, nil
	}
	if !utf8.Valid(decoded) {
		return &ToolResult{Content: "decoded bytes are not valid UTF-8", IsError: true}, nil
	}
	return &ToolResult{Content: string(decoded)}, nil
}

type JSONEscapeTool struct{}

func (t *JSONEscapeTool) Name() string { return "json_escape" }
func (t *JSONEscapeTool) Description() string {
	return "Escape string for JSON"
}
func (t *JSONEscapeTool) InputSchema() any {
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
func (t *JSONEscapeTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	marshaled, err := json.Marshal(text)
	if err != nil {
		return &ToolResult{Content: "failed to escape: " + err.Error(), IsError: true}, nil
	}
	escaped := string(marshaled[1 : len(marshaled)-1])
	return &ToolResult{Content: escaped}, nil
}

type CSVEscapeTool struct{}

func (t *CSVEscapeTool) Name() string { return "csv_escape" }
func (t *CSVEscapeTool) Description() string {
	return "Escape string for CSV field"
}
func (t *CSVEscapeTool) InputSchema() any {
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
func (t *CSVEscapeTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	if strings.ContainsAny(text, ",\"\n\r") {
		escaped := strings.ReplaceAll(text, "\"", "\"\"")
		return &ToolResult{Content: fmt.Sprintf("\"%s\"", escaped)}, nil
	}
	return &ToolResult{Content: text}, nil
}
