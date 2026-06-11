package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
)

func init() {
	registerBuiltins(map[string]BuiltinTool{
		"uuid_v4":        &UUIDV4Tool{},
		"uuid_parse":     &UUIDParseTool{},
		"random_string":  &RandomStringTool{},
		"random_int":     &RandomIntTool{},
		"random_bytes":   &RandomBytesTool{},
		"random_float":   &RandomFloatTool{},
		"random_choice":  &RandomChoiceTool{},
		"random_shuffle": &RandomShuffleTool{},
	}, map[string]bool{
		"uuid_v4":        true,
		"uuid_parse":     true,
		"random_string":  true,
		"random_int":     true,
		"random_bytes":   true,
		"random_float":   true,
		"random_choice":  true,
		"random_shuffle": true,
	})
}

type UUIDV4Tool struct{}

func (t *UUIDV4Tool) Name() string {
	return "uuid_v4"
}

func (t *UUIDV4Tool) Description() string {
	return "Generate UUID v4"
}

func (t *UUIDV4Tool) InputSchema() any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *UUIDV4Tool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	_ = args
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return &ToolResult{Content: "failed to generate UUID: " + err.Error(), IsError: true}, nil
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	uuid := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
	return &ToolResult{Content: uuid}, nil
}

type UUIDParseTool struct{}

func (t *UUIDParseTool) Name() string {
	return "uuid_parse"
}

func (t *UUIDParseTool) Description() string {
	return "Parse and validate UUID"
}

func (t *UUIDParseTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"uuid": map[string]any{
				"type":        "string",
				"description": "UUID string to parse",
			},
		},
		"required": []string{"uuid"},
	}
}

func (t *UUIDParseTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	uuidStr, ok := args["uuid"].(string)
	if !ok || strings.TrimSpace(uuidStr) == "" {
		return &ToolResult{Content: "uuid is required", IsError: true}, nil
	}
	uuidStr = strings.TrimSpace(uuidStr)
	parts := strings.Split(uuidStr, "-")
	if len(parts) != 5 || len(parts[0]) != 8 || len(parts[1]) != 4 || len(parts[2]) != 4 || len(parts[3]) != 4 || len(parts[4]) != 12 {
		result := map[string]any{"valid": false, "version": nil, "variant": nil}
		data, _ := json.Marshal(result)
		return &ToolResult{Content: string(data)}, nil
	}
	fullHex := strings.ReplaceAll(uuidStr, "-", "")
	b, err := hex.DecodeString(fullHex)
	if err != nil || len(b) != 16 {
		result := map[string]any{"valid": false, "version": nil, "variant": nil}
		data, _ := json.Marshal(result)
		return &ToolResult{Content: string(data)}, nil
	}
	version := int((b[6] >> 4) & 0x0f)
	variantBits := (b[8] >> 6) & 0x03
	variant := "unknown"
	if variantBits == 0x02 {
		variant = "RFC4122"
	}
	result := map[string]any{"valid": true, "version": version, "variant": variant}
	data, _ := json.Marshal(result)
	return &ToolResult{Content: string(data)}, nil
}

type RandomStringTool struct{}

func (t *RandomStringTool) Name() string {
	return "random_string"
}

func (t *RandomStringTool) Description() string {
	return "Generate random alphanumeric string"
}

func (t *RandomStringTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"length": map[string]any{
				"type":        "integer",
				"description": "Length of string",
				"default":     16,
			},
			"charset": map[string]any{
				"type":        "string",
				"description": "Character set (alphanumeric, alpha, numeric, hex)",
				"default":     "alphanumeric",
			},
		},
	}
}

func (t *RandomStringTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	length := 16
	if l, ok := args["length"].(float64); ok {
		length = int(l)
	}
	if length <= 0 || length > 1024 {
		return &ToolResult{Content: "length must be between 1 and 1024", IsError: true}, nil
	}
	charset, _ := args["charset"].(string)
	charset = strings.TrimSpace(strings.ToLower(charset))
	if charset == "" {
		charset = "alphanumeric"
	}
	var chars string
	switch charset {
	case "alphanumeric":
		chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	case "alpha":
		chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	case "numeric":
		chars = "0123456789"
	case "hex":
		chars = "0123456789abcdef"
	default:
		return &ToolResult{Content: "unsupported charset: " + charset, IsError: true}, nil
	}
	result := make([]byte, length)
	max := big.NewInt(int64(len(chars)))
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return &ToolResult{Content: "failed to generate random string: " + err.Error(), IsError: true}, nil
		}
		result[i] = chars[n.Int64()]
	}
	return &ToolResult{Content: string(result)}, nil
}

type RandomIntTool struct{}

func (t *RandomIntTool) Name() string {
	return "random_int"
}

func (t *RandomIntTool) Description() string {
	return "Generate random integer in range"
}

func (t *RandomIntTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"min": map[string]any{
				"type":        "integer",
				"description": "Minimum value (inclusive)",
				"default":     0,
			},
			"max": map[string]any{
				"type":        "integer",
				"description": "Maximum value (inclusive)",
				"default":     100,
			},
		},
	}
}

func (t *RandomIntTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	min := 0
	if m, ok := args["min"].(float64); ok {
		min = int(m)
	}
	max := 100
	if m, ok := args["max"].(float64); ok {
		max = int(m)
	}
	if min > max {
		return &ToolResult{Content: "min must be less than or equal to max", IsError: true}, nil
	}
	rangeSize := int64(max - min + 1)
	n, err := rand.Int(rand.Reader, big.NewInt(rangeSize))
	if err != nil {
		return &ToolResult{Content: "failed to generate random int: " + err.Error(), IsError: true}, nil
	}
	result := int(n.Int64()) + min
	return &ToolResult{Content: fmt.Sprintf("%d", result)}, nil
}

type RandomBytesTool struct{}

func (t *RandomBytesTool) Name() string {
	return "random_bytes"
}

func (t *RandomBytesTool) Description() string {
	return "Generate random bytes (hex-encoded)"
}

func (t *RandomBytesTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"length": map[string]any{
				"type":        "integer",
				"description": "Number of bytes to generate",
				"default":     16,
			},
		},
	}
}

func (t *RandomBytesTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	length := 16
	if l, ok := args["length"].(float64); ok {
		length = int(l)
	}
	if length <= 0 || length > 1024 {
		return &ToolResult{Content: "length must be between 1 and 1024", IsError: true}, nil
	}
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return &ToolResult{Content: "failed to generate random bytes: " + err.Error(), IsError: true}, nil
	}
	return &ToolResult{Content: hex.EncodeToString(b)}, nil
}

type RandomFloatTool struct{}

func (t *RandomFloatTool) Name() string {
	return "random_float"
}

func (t *RandomFloatTool) Description() string {
	return "Generate random float in range"
}

func (t *RandomFloatTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"min": map[string]any{
				"type":        "number",
				"description": "Minimum value (inclusive)",
				"default":     0.0,
			},
			"max": map[string]any{
				"type":        "number",
				"description": "Maximum value (exclusive)",
				"default":     1.0,
			},
		},
	}
}

func (t *RandomFloatTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	min := 0.0
	if m, ok := args["min"].(float64); ok {
		min = m
	}
	max := 1.0
	if m, ok := args["max"].(float64); ok {
		max = m
	}
	if min >= max {
		return &ToolResult{Content: "min must be less than max", IsError: true}, nil
	}
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return &ToolResult{Content: "failed to generate random float: " + err.Error(), IsError: true}, nil
	}
	var n uint64
	for i := 0; i < 8; i++ {
		n = (n << 8) | uint64(b[i])
	}
	const maxUint64 = float64(^uint64(0))
	fraction := float64(n) / maxUint64
	result := min + fraction*(max-min)
	return &ToolResult{Content: fmt.Sprintf("%.6f", result)}, nil
}

type RandomChoiceTool struct{}

func (t *RandomChoiceTool) Name() string {
	return "random_choice"
}

func (t *RandomChoiceTool) Description() string {
	return "Pick random element from array"
}

func (t *RandomChoiceTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"choices": map[string]any{
				"type":        "array",
				"description": "Array of choices",
				"items": map[string]any{
					"type": "string",
				},
			},
		},
		"required": []string{"choices"},
	}
}

func (t *RandomChoiceTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	choicesRaw, ok := args["choices"].([]any)
	if !ok || len(choicesRaw) == 0 {
		return &ToolResult{Content: "choices array is required and must not be empty", IsError: true}, nil
	}
	choices := make([]string, 0, len(choicesRaw))
	for _, c := range choicesRaw {
		if s, ok := c.(string); ok {
			choices = append(choices, s)
		}
	}
	if len(choices) == 0 {
		return &ToolResult{Content: "choices array must contain at least one string", IsError: true}, nil
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(choices))))
	if err != nil {
		return &ToolResult{Content: "failed to pick random choice: " + err.Error(), IsError: true}, nil
	}
	return &ToolResult{Content: choices[n.Int64()]}, nil
}

type RandomShuffleTool struct{}

func (t *RandomShuffleTool) Name() string {
	return "random_shuffle"
}

func (t *RandomShuffleTool) Description() string {
	return "Shuffle array randomly"
}

func (t *RandomShuffleTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"items": map[string]any{
				"type":        "array",
				"description": "Array to shuffle",
				"items": map[string]any{
					"type": "string",
				},
			},
		},
		"required": []string{"items"},
	}
}

func (t *RandomShuffleTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	_ = ctx
	itemsRaw, ok := args["items"].([]any)
	if !ok {
		return &ToolResult{Content: "items array is required", IsError: true}, nil
	}
	items := make([]string, 0, len(itemsRaw))
	for _, item := range itemsRaw {
		if s, ok := item.(string); ok {
			items = append(items, s)
		}
	}
	result := make([]string, len(items))
	copy(result, items)
	for i := len(result) - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return &ToolResult{Content: "failed to shuffle: " + err.Error(), IsError: true}, nil
		}
		jInt := int(j.Int64())
		result[i], result[jInt] = result[jInt], result[i]
	}
	data, _ := json.Marshal(result)
	return &ToolResult{Content: string(data)}, nil
}
