package mcp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"
)

func buildTestJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	header, err := json.Marshal(map[string]any{"alg": "none", "typ": "JWT"})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload) + "." +
		base64.RawURLEncoding.EncodeToString([]byte("sig"))
}

func TestJWTDecodeTool(t *testing.T) {
	tool := &JWTDecodeTool{}
	ctx := context.Background()

	token := buildTestJWT(t, map[string]any{"sub": "user1"})
	result, err := tool.Execute(ctx, map[string]any{"token": token})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error result %q", result.Content)
	}
	for _, want := range []string{`"sub": "user1"`, `"alg": "none"`, `"signature"`} {
		if !strings.Contains(result.Content, want) {
			t.Fatalf("decode output missing %q: %s", want, result.Content)
		}
	}

	result, err = tool.Execute(ctx, map[string]any{"token": "only.two"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected IsError for malformed token, got %q", result.Content)
	}

	result, err = tool.Execute(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError for missing token")
	}
}

func TestJWTParseClaimsTool(t *testing.T) {
	tool := &JWTParseCLaimsTool{}
	ctx := context.Background()

	token := buildTestJWT(t, map[string]any{"sub": "user1", "org": "acme"})
	result, err := tool.Execute(ctx, map[string]any{"token": token})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error result %q", result.Content)
	}
	var claims map[string]any
	if err := json.Unmarshal([]byte(result.Content), &claims); err != nil {
		t.Fatalf("claims output is not JSON: %v", err)
	}
	if claims["sub"] != "user1" || claims["org"] != "acme" {
		t.Fatalf("unexpected claims: %v", claims)
	}

	result, err = tool.Execute(ctx, map[string]any{"token": "a.%%%invalid%%%.c"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.IsError {
		t.Fatalf("expected IsError for bad payload encoding, got %q", result.Content)
	}
}

func TestJWTIsExpiredTool(t *testing.T) {
	tool := &JWTIsExpiredTool{}
	ctx := context.Background()

	expired := buildTestJWT(t, map[string]any{"exp": time.Now().Add(-time.Hour).Unix()})
	result, err := tool.Execute(ctx, map[string]any{"token": expired})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Content != "true" {
		t.Fatalf("expired token: got %q, want \"true\"", result.Content)
	}

	fresh := buildTestJWT(t, map[string]any{"exp": time.Now().Add(time.Hour).Unix()})
	result, err = tool.Execute(ctx, map[string]any{"token": fresh})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Content != "false" {
		t.Fatalf("fresh token: got %q, want \"false\"", result.Content)
	}

	noExp := buildTestJWT(t, map[string]any{"sub": "user1"})
	result, err = tool.Execute(ctx, map[string]any{"token": noExp})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.Content != "false" {
		t.Fatalf("token without exp: got %q, want \"false\"", result.Content)
	}

	result, err = tool.Execute(ctx, map[string]any{"token": "not-a-jwt"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError for malformed token")
	}
}

func TestTokenGenerateTool(t *testing.T) {
	tool := &TokenGenerateTool{}
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error result %q", result.Content)
	}
	decoded, err := base64.URLEncoding.DecodeString(result.Content)
	if err != nil {
		t.Fatalf("token is not URL-safe base64: %v", err)
	}
	if len(decoded) != 32 {
		t.Fatalf("default token length = %d bytes, want 32", len(decoded))
	}

	result, err = tool.Execute(ctx, map[string]any{"length": float64(8)})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	decoded, err = base64.URLEncoding.DecodeString(result.Content)
	if err != nil {
		t.Fatalf("token is not URL-safe base64: %v", err)
	}
	if len(decoded) != 8 {
		t.Fatalf("token length = %d bytes, want 8", len(decoded))
	}

	result, err = tool.Execute(ctx, map[string]any{"length": float64(2048)})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError for out-of-range length")
	}
}

func TestAPIKeyFormatTool(t *testing.T) {
	tool := &APIKeyFormatTool{}
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success, got error result %q", result.Content)
	}
	if !regexp.MustCompile(`^sk_[0-9A-Za-z]+$`).MatchString(result.Content) {
		t.Fatalf("default key %q does not match ^sk_[0-9A-Za-z]+$", result.Content)
	}

	result, err = tool.Execute(ctx, map[string]any{"prefix": "ob", "length": float64(16)})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !strings.HasPrefix(result.Content, "ob_") {
		t.Fatalf("key %q does not use requested prefix", result.Content)
	}

	result, err = tool.Execute(ctx, map[string]any{"length": float64(0)})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError for zero length")
	}
}

func TestAPIKeyValidateTool(t *testing.T) {
	tool := &APIKeyValidateTool{}
	ctx := context.Background()

	longKey := "sk_" + strings.Repeat("a", 40)
	cases := []struct {
		name      string
		args      map[string]any
		wantValid bool
	}{
		{"valid", map[string]any{"key": longKey, "expected_prefix": "sk"}, true},
		{"wrong_prefix", map[string]any{"key": longKey, "expected_prefix": "pk"}, false},
		{"too_short", map[string]any{"key": "sk_abc", "min_length": float64(32)}, false},
		{"bad_characters", map[string]any{"key": "sk_" + strings.Repeat("a", 30) + "-!" + strings.Repeat("b", 10)}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tc.args)
			if err != nil {
				t.Fatalf("Execute failed: %v", err)
			}
			if result.IsError {
				t.Fatalf("expected verdict result, got error %q", result.Content)
			}
			var verdict map[string]any
			if err := json.Unmarshal([]byte(result.Content), &verdict); err != nil {
				t.Fatalf("verdict is not JSON: %v", err)
			}
			if verdict["valid"] != tc.wantValid {
				t.Fatalf("valid = %v, want %v (content %s)", verdict["valid"], tc.wantValid, result.Content)
			}
		})
	}

	result, err := tool.Execute(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError for missing key")
	}
}

func TestJWTTokenToolsRegistered(t *testing.T) {
	for _, name := range []string{"jwt_decode", "jwt_parse_claims", "jwt_is_expired", "token_generate", "api_key_format", "api_key_validate"} {
		if _, ok := GetBuiltinTool(name); !ok {
			t.Fatalf("builtin %s not registered", name)
		}
		if !IsDefaultCommercialBuiltin(name) {
			t.Fatalf("builtin %s should be default commercial enabled (pure local)", name)
		}
	}
}

func TestJWTTokenToolsEmptyArgsDoNotError(t *testing.T) {
	// Cross-cutting contract: default-enabled builtins must not return a Go
	// error for empty args (see TestDefaultEnabledBuiltinsDoNotReturnPlaceholderOutput).
	for _, name := range []string{"jwt_decode", "jwt_parse_claims", "jwt_is_expired", "token_generate", "api_key_format", "api_key_validate"} {
		tool, ok := GetBuiltinTool(name)
		if !ok {
			t.Fatalf("builtin %s not registered", name)
		}
		result, err := tool.Execute(context.Background(), map[string]any{})
		if err != nil {
			t.Fatalf("%s returned error for empty args: %v", name, err)
		}
		if strings.Contains(strings.ToLower(result.Content), "placeholder") {
			t.Fatalf("%s returned placeholder output: %q", name, result.Content)
		}
	}
}
