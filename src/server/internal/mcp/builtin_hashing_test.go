package mcp

import (
	"context"
	"strings"
	"testing"
)

func TestMD5Hash(t *testing.T) {
	tool, ok := GetBuiltinTool("md5_hash")
	if !ok {
		t.Fatal("md5_hash builtin not found")
	}

	cases := []struct {
		text string
		want string
	}{
		{"", "d41d8cd98f00b204e9800998ecf8427e"},
		{"hello", "5d41402abc4b2a76b9719d911017c592"},
		{"The quick brown fox jumps over the lazy dog", "9e107d9d372bb6826bd81d3542a419d6"},
	}

	for _, tt := range cases {
		result, err := tool.Execute(context.Background(), map[string]any{"text": tt.text})
		if err != nil {
			t.Fatalf("md5_hash(%q) returned error: %v", tt.text, err)
		}
		if result.Content != tt.want {
			t.Fatalf("md5_hash(%q) = %q, want %q", tt.text, result.Content, tt.want)
		}
	}

	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("md5_hash with empty args returned error: %v", err)
	}
	if result.Content != "d41d8cd98f00b204e9800998ecf8427e" {
		t.Fatalf("md5_hash with empty args = %q, want empty string hash", result.Content)
	}
}

func TestSHA1Hash(t *testing.T) {
	tool, ok := GetBuiltinTool("sha1_hash")
	if !ok {
		t.Fatal("sha1_hash builtin not found")
	}

	cases := []struct {
		text string
		want string
	}{
		{"", "da39a3ee5e6b4b0d3255bfef95601890afd80709"},
		{"hello", "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d"},
	}

	for _, tt := range cases {
		result, err := tool.Execute(context.Background(), map[string]any{"text": tt.text})
		if err != nil {
			t.Fatalf("sha1_hash(%q) returned error: %v", tt.text, err)
		}
		if result.Content != tt.want {
			t.Fatalf("sha1_hash(%q) = %q, want %q", tt.text, result.Content, tt.want)
		}
	}

	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("sha1_hash with empty args returned error: %v", err)
	}
	if len(result.Content) != 40 {
		t.Fatalf("sha1_hash with empty args output length = %d, want 40", len(result.Content))
	}
}

func TestSHA256Hash(t *testing.T) {
	tool, ok := GetBuiltinTool("sha256_hash")
	if !ok {
		t.Fatal("sha256_hash builtin not found")
	}

	cases := []struct {
		text string
		want string
	}{
		{"", "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
		{"hello", "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"},
	}

	for _, tt := range cases {
		result, err := tool.Execute(context.Background(), map[string]any{"text": tt.text})
		if err != nil {
			t.Fatalf("sha256_hash(%q) returned error: %v", tt.text, err)
		}
		if result.Content != tt.want {
			t.Fatalf("sha256_hash(%q) = %q, want %q", tt.text, result.Content, tt.want)
		}
	}

	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("sha256_hash with empty args returned error: %v", err)
	}
	if len(result.Content) != 64 {
		t.Fatalf("sha256_hash with empty args output length = %d, want 64", len(result.Content))
	}
}

func TestSHA512Hash(t *testing.T) {
	tool, ok := GetBuiltinTool("sha512_hash")
	if !ok {
		t.Fatal("sha512_hash builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{"text": "hello"})
	if err != nil {
		t.Fatalf("sha512_hash returned error: %v", err)
	}
	if len(result.Content) != 128 {
		t.Fatalf("sha512_hash output length = %d, want 128", len(result.Content))
	}
	if want := "9b71d224bd62f3785d96d46ad3ea3d73319bfbc2890caadae2dff72519673ca72323c3d99ba5c11d7c7acc6e14b8c5da0c4663475c2e5c3adef46f73bcdec043"; result.Content != want {
		t.Fatalf("sha512_hash(hello) = %q, want %q", result.Content, want)
	}

	result, err = tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("sha512_hash with empty args returned error: %v", err)
	}
	if len(result.Content) != 128 {
		t.Fatalf("sha512_hash with empty args output length = %d, want 128", len(result.Content))
	}
}

func TestCRC32Checksum(t *testing.T) {
	tool, ok := GetBuiltinTool("crc32_checksum")
	if !ok {
		t.Fatal("crc32_checksum builtin not found")
	}

	cases := []struct {
		text string
		want string
	}{
		{"", "0"},
		{"hello", "907060870"},
	}

	for _, tt := range cases {
		result, err := tool.Execute(context.Background(), map[string]any{"text": tt.text})
		if err != nil {
			t.Fatalf("crc32_checksum(%q) returned error: %v", tt.text, err)
		}
		if result.Content != tt.want {
			t.Fatalf("crc32_checksum(%q) = %q, want %q", tt.text, result.Content, tt.want)
		}
	}

	result, err := tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("crc32_checksum with empty args returned error: %v", err)
	}
	if result.Content != "0" {
		t.Fatalf("crc32_checksum with empty args = %q, want 0", result.Content)
	}
}

func TestHMACSHA256(t *testing.T) {
	tool, ok := GetBuiltinTool("hmac_sha256")
	if !ok {
		t.Fatal("hmac_sha256 builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{"text": "message", "key": "secret"})
	if err != nil {
		t.Fatalf("hmac_sha256 returned error: %v", err)
	}
	if want := "8b5f48702995c1598c573db1e21866a9b825d4a794d169d7060a03605796360b"; result.Content != want {
		t.Fatalf("hmac_sha256(message, secret) = %q, want %q", result.Content, want)
	}

	result, err = tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("hmac_sha256 with empty args returned error: %v", err)
	}
	if len(result.Content) != 64 {
		t.Fatalf("hmac_sha256 with empty args output length = %d, want 64", len(result.Content))
	}
}

func TestBcryptHash(t *testing.T) {
	tool, ok := GetBuiltinTool("bcrypt_hash")
	if !ok {
		t.Fatal("bcrypt_hash builtin not found")
	}

	result, err := tool.Execute(context.Background(), map[string]any{"password": "secret", "cost": 4})
	if err != nil {
		t.Fatalf("bcrypt_hash returned error: %v", err)
	}
	if !strings.HasPrefix(result.Content, "$2a$04$") && !strings.HasPrefix(result.Content, "$2b$04$") {
		t.Fatalf("bcrypt_hash output = %q, want bcrypt hash format", result.Content)
	}

	result, err = tool.Execute(context.Background(), map[string]any{"password": "secret", "cost": 100})
	if err == nil && !result.IsError {
		t.Fatal("bcrypt_hash with invalid cost should return error")
	}

	result, err = tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("bcrypt_hash with empty args returned error: %v", err)
	}
	if result.IsError {
		t.Fatalf("bcrypt_hash with empty args returned error result: %q", result.Content)
	}
	if !strings.HasPrefix(result.Content, "$2a$") && !strings.HasPrefix(result.Content, "$2b$") {
		t.Fatalf("bcrypt_hash with empty args = %q, want bcrypt hash format", result.Content)
	}
}

func TestBcryptVerify(t *testing.T) {
	tool, ok := GetBuiltinTool("bcrypt_verify")
	if !ok {
		t.Fatal("bcrypt_verify builtin not found")
	}

	hash := "$2a$10$0dlasoxseuXXL6z4qf9IQ..eOZZ7/g9SqEDXQo7BZq7v0XWI1Xkse"

	result, err := tool.Execute(context.Background(), map[string]any{"password": "secret", "hash": hash})
	if err != nil {
		t.Fatalf("bcrypt_verify returned error: %v", err)
	}
	if result.Content != "match" {
		t.Fatalf("bcrypt_verify(correct password) = %q, want match", result.Content)
	}

	result, err = tool.Execute(context.Background(), map[string]any{"password": "wrong", "hash": hash})
	if err != nil {
		t.Fatalf("bcrypt_verify returned error: %v", err)
	}
	if result.Content != "no match" {
		t.Fatalf("bcrypt_verify(wrong password) = %q, want no match", result.Content)
	}

	result, err = tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("bcrypt_verify with empty args returned error: %v", err)
	}
	if result.Content != "no match" {
		t.Fatalf("bcrypt_verify with empty args = %q, want no match", result.Content)
	}
}

func TestChecksumVerify(t *testing.T) {
	tool, ok := GetBuiltinTool("checksum_verify")
	if !ok {
		t.Fatal("checksum_verify builtin not found")
	}

	cases := []struct {
		name      string
		args      map[string]any
		wantValid bool
	}{
		{
			name: "md5 valid",
			args: map[string]any{
				"text":      "hello",
				"algorithm": "md5",
				"expected":  "5d41402abc4b2a76b9719d911017c592",
			},
			wantValid: true,
		},
		{
			name: "md5 invalid",
			args: map[string]any{
				"text":      "hello",
				"algorithm": "md5",
				"expected":  "wrong",
			},
			wantValid: false,
		},
		{
			name: "sha256 valid",
			args: map[string]any{
				"text":      "hello",
				"algorithm": "sha256",
				"expected":  "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
			},
			wantValid: true,
		},
		{
			name: "crc32 valid",
			args: map[string]any{
				"text":      "hello",
				"algorithm": "crc32",
				"expected":  "907060870",
			},
			wantValid: true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), tt.args)
			if err != nil {
				t.Fatalf("checksum_verify returned error: %v", err)
			}
			want := "invalid"
			if tt.wantValid {
				want = "valid"
			}
			if result.Content != want {
				t.Fatalf("checksum_verify = %q, want %q", result.Content, want)
			}
		})
	}

	result, err := tool.Execute(context.Background(), map[string]any{"algorithm": "invalid"})
	if err == nil && !result.IsError {
		t.Fatal("checksum_verify with invalid algorithm should return error")
	}

	result, err = tool.Execute(context.Background(), map[string]any{})
	if err != nil {
		t.Fatalf("checksum_verify with empty args returned error: %v", err)
	}
	if result.Content != "invalid" {
		t.Fatalf("checksum_verify with empty args = %q, want invalid", result.Content)
	}
}

func TestHashingToolsRegistered(t *testing.T) {
	tools := []string{
		"md5_hash", "sha1_hash", "sha256_hash", "sha512_hash",
		"crc32_checksum", "hmac_sha256", "bcrypt_hash",
		"bcrypt_verify", "checksum_verify",
	}

	for _, name := range tools {
		if _, ok := GetBuiltinTool(name); !ok {
			t.Fatalf("hashing tool %s not registered", name)
		}
		if !IsDefaultCommercialBuiltin(name) {
			t.Fatalf("hashing tool %s should be default commercial enabled", name)
		}
	}
}
