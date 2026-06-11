package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestURLParseTool(t *testing.T) {
	tool := &URLParseTool{}
	ctx := context.Background()

	tests := []struct {
		name    string
		args    map[string]any
		wantErr bool
		check   func(string) bool
	}{
		{
			name: "valid URL",
			args: map[string]any{"url": "https://example.com:8080/path?key=value#frag"},
			check: func(s string) bool {
				var result map[string]string
				json.Unmarshal([]byte(s), &result)
				return result["scheme"] == "https" && result["host"] == "example.com:8080" && result["path"] == "/path"
			},
		},
		{
			name:  "empty args uses default",
			args:  map[string]any{},
			check: func(s string) bool { return strings.Contains(s, "example.com") },
		},
		{
			name:    "invalid URL",
			args:    map[string]any{"url": "ht!tp://bad url"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantErr {
				if !result.IsError {
					t.Errorf("expected error result")
				}
			} else if tt.check != nil && !tt.check(result.Content) {
				t.Errorf("check failed, got: %s", result.Content)
			}
		})
	}
}

func TestURLBuildTool(t *testing.T) {
	tool := &URLBuildTool{}
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]any{
		"scheme": "https",
		"host":   "test.com",
		"path":   "/api",
		"query":  map[string]any{"foo": "bar"},
	})
	if err != nil || result.IsError {
		t.Fatalf("unexpected error")
	}
	if !strings.Contains(result.Content, "https://test.com/api") || !strings.Contains(result.Content, "foo=bar") {
		t.Errorf("unexpected URL: %s", result.Content)
	}

	result, err = tool.Execute(ctx, map[string]any{})
	if err != nil || result.IsError {
		t.Fatalf("empty args should use defaults")
	}
	if !strings.Contains(result.Content, "example.com") {
		t.Errorf("expected default URL, got: %s", result.Content)
	}
}

func TestURLQueryAddTool(t *testing.T) {
	tool := &URLQueryAddTool{}
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]any{
		"url":   "https://example.com",
		"key":   "search",
		"value": "test",
	})
	if err != nil || result.IsError {
		t.Fatalf("unexpected error")
	}
	if !strings.Contains(result.Content, "search=test") {
		t.Errorf("query not added: %s", result.Content)
	}

	result, err = tool.Execute(ctx, map[string]any{})
	if err != nil || result.IsError {
		t.Fatalf("empty args should use defaults")
	}
}

func TestURLQueryRemoveTool(t *testing.T) {
	tool := &URLQueryRemoveTool{}
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]any{
		"url": "https://example.com?foo=bar&baz=qux",
		"key": "foo",
	})
	if err != nil || result.IsError {
		t.Fatalf("unexpected error")
	}
	if strings.Contains(result.Content, "foo=") {
		t.Errorf("query not removed: %s", result.Content)
	}

	result, err = tool.Execute(ctx, map[string]any{})
	if err != nil || result.IsError {
		t.Fatalf("empty args should use defaults")
	}
}

func TestURLQueryGetTool(t *testing.T) {
	tool := &URLQueryGetTool{}
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]any{
		"url": "https://example.com?search=golang",
		"key": "search",
	})
	if err != nil || result.IsError {
		t.Fatalf("unexpected error")
	}
	if result.Content != "golang" {
		t.Errorf("expected 'golang', got: %s", result.Content)
	}

	result, err = tool.Execute(ctx, map[string]any{})
	if err != nil || result.IsError {
		t.Fatalf("empty args should use defaults")
	}
	if result.Content != "test" {
		t.Errorf("expected default 'test', got: %s", result.Content)
	}
}

func TestURLNormalizeTool(t *testing.T) {
	tool := &URLNormalizeTool{}
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]any{
		"url": "HTTP://Example.COM:80/path?z=1&a=2",
	})
	if err != nil || result.IsError {
		t.Fatalf("unexpected error")
	}
	if !strings.HasPrefix(result.Content, "http://example.com/") {
		t.Errorf("not normalized: %s", result.Content)
	}
	if strings.Contains(result.Content, ":80") {
		t.Errorf("default port not removed: %s", result.Content)
	}

	result, err = tool.Execute(ctx, map[string]any{})
	if err != nil || result.IsError {
		t.Fatalf("empty args should use defaults")
	}
}

func TestIPValidateTool(t *testing.T) {
	tool := &IPValidateTool{}
	ctx := context.Background()

	tests := []struct {
		ip      string
		valid   bool
		version int
	}{
		{"192.168.1.1", true, 4},
		{"2001:db8::1", true, 6},
		{"invalid", false, 0},
	}

	for _, tt := range tests {
		result, err := tool.Execute(ctx, map[string]any{"ip": tt.ip})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var res map[string]any
		json.Unmarshal([]byte(result.Content), &res)
		if res["valid"] != tt.valid {
			t.Errorf("ip %s: expected valid=%v, got %v", tt.ip, tt.valid, res["valid"])
		}
		if tt.valid {
			if int(res["version"].(float64)) != tt.version {
				t.Errorf("ip %s: expected version %d, got %v", tt.ip, tt.version, res["version"])
			}
		}
	}

	result, err := tool.Execute(ctx, map[string]any{})
	if err != nil || result.IsError {
		t.Fatalf("empty args should use defaults")
	}
}

func TestIPInCIDRTool(t *testing.T) {
	tool := &IPInCIDRTool{}
	ctx := context.Background()

	tests := []struct {
		ip     string
		cidr   string
		expect string
	}{
		{"192.168.1.10", "192.168.1.0/24", "true"},
		{"192.168.2.10", "192.168.1.0/24", "false"},
	}

	for _, tt := range tests {
		result, err := tool.Execute(ctx, map[string]any{"ip": tt.ip, "cidr": tt.cidr})
		if err != nil || result.IsError {
			t.Fatalf("unexpected error")
		}
		if result.Content != tt.expect {
			t.Errorf("ip %s in %s: expected %s, got %s", tt.ip, tt.cidr, tt.expect, result.Content)
		}
	}

	result, err := tool.Execute(ctx, map[string]any{})
	if err != nil || result.IsError {
		t.Fatalf("empty args should use defaults")
	}
	if result.Content != "true" {
		t.Errorf("default should be true, got: %s", result.Content)
	}
}

func TestCIDRParseTool(t *testing.T) {
	tool := &CIDRParseTool{}
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]any{"cidr": "192.168.1.0/24"})
	if err != nil || result.IsError {
		t.Fatalf("unexpected error")
	}
	var res map[string]any
	json.Unmarshal([]byte(result.Content), &res)
	if res["network"] != "192.168.1.0" {
		t.Errorf("unexpected network: %v", res["network"])
	}
	if res["total"].(float64) != 256 {
		t.Errorf("unexpected total: %v", res["total"])
	}

	result, err = tool.Execute(ctx, map[string]any{})
	if err != nil || result.IsError {
		t.Fatalf("empty args should use defaults")
	}
}

func TestDomainExtractTool(t *testing.T) {
	tool := &DomainExtractTool{}
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]any{"hostname": "api.staging.example.com"})
	if err != nil || result.IsError {
		t.Fatalf("unexpected error")
	}
	var res map[string]string
	json.Unmarshal([]byte(result.Content), &res)
	if res["domain"] != "example" || res["tld"] != "com" {
		t.Errorf("unexpected parse: %+v", res)
	}

	result, err = tool.Execute(ctx, map[string]any{})
	if err != nil || result.IsError {
		t.Fatalf("empty args should use defaults")
	}
}

func TestEmailValidateTool(t *testing.T) {
	tool := &EmailValidateTool{}
	ctx := context.Background()

	tests := []struct {
		email string
		valid bool
	}{
		{"user@example.com", true},
		{"invalid-email", false},
		{"@example.com", false},
	}

	for _, tt := range tests {
		result, err := tool.Execute(ctx, map[string]any{"email": tt.email})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var res map[string]any
		json.Unmarshal([]byte(result.Content), &res)
		if res["valid"] != tt.valid {
			t.Errorf("email %s: expected valid=%v, got %v", tt.email, tt.valid, res["valid"])
		}
	}

	result, err := tool.Execute(ctx, map[string]any{})
	if err != nil || result.IsError {
		t.Fatalf("empty args should use defaults")
	}
}

func TestUserAgentParseTool(t *testing.T) {
	tool := &UserAgentParseTool{}
	ctx := context.Background()

	result, err := tool.Execute(ctx, map[string]any{
		"user_agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
	})
	if err != nil || result.IsError {
		t.Fatalf("unexpected error")
	}
	var res map[string]string
	json.Unmarshal([]byte(result.Content), &res)
	if res["browser"] != "Chrome" {
		t.Errorf("expected Chrome, got: %s", res["browser"])
	}
	if res["os"] != "Windows" {
		t.Errorf("expected Windows, got: %s", res["os"])
	}

	result, err = tool.Execute(ctx, map[string]any{})
	if err != nil || result.IsError {
		t.Fatalf("empty args should use defaults")
	}
}
