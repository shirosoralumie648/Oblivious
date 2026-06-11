package mcp

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"strings"
	"time"
)

func init() {
	registerBuiltins(map[string]BuiltinTool{
		"jwt_decode":       &JWTDecodeTool{},
		"jwt_parse_claims": &JWTParseCLaimsTool{},
		"jwt_is_expired":   &JWTIsExpiredTool{},
		"token_generate":   &TokenGenerateTool{},
		"api_key_format":   &APIKeyFormatTool{},
		"api_key_validate": &APIKeyValidateTool{},
	}, map[string]bool{
		"jwt_decode":       true,
		"jwt_parse_claims": true,
		"jwt_is_expired":   true,
		"token_generate":   true,
		"api_key_format":   true,
		"api_key_validate": true,
	})
}

type JWTDecodeTool struct{}

func (t *JWTDecodeTool) Name() string {
	return "jwt_decode"
}

func (t *JWTDecodeTool) Description() string {
	return "Decode JWT without verification"
}

func (t *JWTDecodeTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"token": map[string]any{
				"type":        "string",
				"description": "JWT token to decode",
			},
		},
		"required": []string{"token"},
	}
}

func (t *JWTDecodeTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	token, ok := args["token"].(string)
	if !ok || strings.TrimSpace(token) == "" {
		return &ToolResult{Content: "token is required", IsError: true}, nil
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return &ToolResult{Content: "invalid JWT format: expected 3 parts separated by dots", IsError: true}, nil
	}

	header, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return &ToolResult{Content: "invalid JWT header encoding", IsError: true}, nil
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return &ToolResult{Content: "invalid JWT payload encoding", IsError: true}, nil
	}

	result := map[string]any{
		"header":    json.RawMessage(header),
		"payload":   json.RawMessage(payload),
		"signature": parts[2],
	}

	output, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &ToolResult{Content: "failed to format output", IsError: true}, nil
	}

	return &ToolResult{Content: string(output)}, nil
}

type JWTParseCLaimsTool struct{}

func (t *JWTParseCLaimsTool) Name() string {
	return "jwt_parse_claims"
}

func (t *JWTParseCLaimsTool) Description() string {
	return "Parse JWT claims"
}

func (t *JWTParseCLaimsTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"token": map[string]any{
				"type":        "string",
				"description": "JWT token to parse claims from",
			},
		},
		"required": []string{"token"},
	}
}

func (t *JWTParseCLaimsTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	token, ok := args["token"].(string)
	if !ok || strings.TrimSpace(token) == "" {
		return &ToolResult{Content: "token is required", IsError: true}, nil
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return &ToolResult{Content: "invalid JWT format", IsError: true}, nil
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return &ToolResult{Content: "invalid JWT payload encoding", IsError: true}, nil
	}

	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return &ToolResult{Content: "invalid JWT claims", IsError: true}, nil
	}

	output, err := json.MarshalIndent(claims, "", "  ")
	if err != nil {
		return &ToolResult{Content: "failed to format claims", IsError: true}, nil
	}

	return &ToolResult{Content: string(output)}, nil
}

type JWTIsExpiredTool struct{}

func (t *JWTIsExpiredTool) Name() string {
	return "jwt_is_expired"
}

func (t *JWTIsExpiredTool) Description() string {
	return "Check if JWT is expired"
}

func (t *JWTIsExpiredTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"token": map[string]any{
				"type":        "string",
				"description": "JWT token to check expiration",
			},
		},
		"required": []string{"token"},
	}
}

func (t *JWTIsExpiredTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	token, ok := args["token"].(string)
	if !ok || strings.TrimSpace(token) == "" {
		return &ToolResult{Content: "token is required", IsError: true}, nil
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return &ToolResult{Content: "invalid JWT format", IsError: true}, nil
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return &ToolResult{Content: "invalid JWT payload encoding", IsError: true}, nil
	}

	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return &ToolResult{Content: "invalid JWT claims", IsError: true}, nil
	}

	exp, ok := claims["exp"]
	if !ok {
		return &ToolResult{Content: "false"}, nil
	}

	var expTime int64
	switch v := exp.(type) {
	case float64:
		expTime = int64(v)
	case int64:
		expTime = v
	default:
		return &ToolResult{Content: "invalid exp claim type", IsError: true}, nil
	}

	if time.Now().Unix() > expTime {
		return &ToolResult{Content: "true"}, nil
	}
	return &ToolResult{Content: "false"}, nil
}

type TokenGenerateTool struct{}

func (t *TokenGenerateTool) Name() string {
	return "token_generate"
}

func (t *TokenGenerateTool) Description() string {
	return "Generate random secure token"
}

func (t *TokenGenerateTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"length": map[string]any{
				"type":        "integer",
				"description": "Token length in bytes",
				"default":     32,
			},
		},
	}
}

func (t *TokenGenerateTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	length := 32
	if l, ok := args["length"]; ok {
		switch v := l.(type) {
		case float64:
			length = int(v)
		case int:
			length = v
		}
	}

	if length <= 0 || length > 1024 {
		return &ToolResult{Content: "length must be between 1 and 1024", IsError: true}, nil
	}

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return &ToolResult{Content: "failed to generate random bytes", IsError: true}, nil
	}

	token := base64.URLEncoding.EncodeToString(bytes)
	return &ToolResult{Content: token}, nil
}

type APIKeyFormatTool struct{}

func (t *APIKeyFormatTool) Name() string {
	return "api_key_format"
}

func (t *APIKeyFormatTool) Description() string {
	return "Format API key with prefix"
}

func (t *APIKeyFormatTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prefix": map[string]any{
				"type":        "string",
				"description": "API key prefix (e.g., 'sk')",
				"default":     "sk",
			},
			"length": map[string]any{
				"type":        "integer",
				"description": "Key length in bytes",
				"default":     32,
			},
		},
	}
}

func (t *APIKeyFormatTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	prefix := "sk"
	if p, ok := args["prefix"].(string); ok && p != "" {
		prefix = p
	}

	length := 32
	if l, ok := args["length"]; ok {
		switch v := l.(type) {
		case float64:
			length = int(v)
		case int:
			length = v
		}
	}

	if length <= 0 || length > 1024 {
		return &ToolResult{Content: "length must be between 1 and 1024", IsError: true}, nil
	}

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return &ToolResult{Content: "failed to generate random bytes", IsError: true}, nil
	}

	key := base62Encode(bytes)
	return &ToolResult{Content: prefix + "_" + key}, nil
}

type APIKeyValidateTool struct{}

func (t *APIKeyValidateTool) Name() string {
	return "api_key_validate"
}

func (t *APIKeyValidateTool) Description() string {
	return "Validate API key format"
}

func (t *APIKeyValidateTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"key": map[string]any{
				"type":        "string",
				"description": "API key to validate",
			},
			"expected_prefix": map[string]any{
				"type":        "string",
				"description": "Expected prefix (e.g., 'sk')",
				"default":     "",
			},
			"min_length": map[string]any{
				"type":        "integer",
				"description": "Minimum key length",
				"default":     32,
			},
		},
		"required": []string{"key"},
	}
}

func (t *APIKeyValidateTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	key, ok := args["key"].(string)
	if !ok {
		return &ToolResult{Content: "key is required", IsError: true}, nil
	}

	expectedPrefix := ""
	if p, ok := args["expected_prefix"].(string); ok {
		expectedPrefix = p
	}

	minLength := 32
	if l, ok := args["min_length"]; ok {
		switch v := l.(type) {
		case float64:
			minLength = int(v)
		case int:
			minLength = v
		}
	}

	parts := strings.Split(key, "_")
	valid := true
	prefix := ""
	keyLength := len(key)

	if len(parts) >= 2 {
		prefix = parts[0]
		keyLength = len(strings.Join(parts[1:], "_"))
	}

	if expectedPrefix != "" && prefix != expectedPrefix {
		valid = false
	}

	if keyLength < minLength {
		valid = false
	}

	for _, r := range key {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_') {
			valid = false
			break
		}
	}

	result := map[string]any{
		"valid":  valid,
		"prefix": prefix,
		"length": keyLength,
	}

	output, err := json.Marshal(result)
	if err != nil {
		return &ToolResult{Content: "failed to format result", IsError: true}, nil
	}

	return &ToolResult{Content: string(output)}, nil
}

func base62Encode(data []byte) string {
	const base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	num := new(big.Int).SetBytes(data)
	base := big.NewInt(62)
	zero := big.NewInt(0)
	mod := new(big.Int)

	var result []byte
	for num.Cmp(zero) > 0 {
		num.DivMod(num, base, mod)
		result = append([]byte{base62Chars[mod.Int64()]}, result...)
	}

	if len(result) == 0 {
		return "0"
	}
	return string(result)
}
