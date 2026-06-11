package mcp

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"strconv"
	"strings"

	"crypto/hmac"

	"golang.org/x/crypto/bcrypt"
)

func init() {
	registerBuiltins(map[string]BuiltinTool{
		"md5_hash":        &MD5HashTool{},
		"sha1_hash":       &SHA1HashTool{},
		"sha256_hash":     &SHA256HashTool{},
		"sha512_hash":     &SHA512HashTool{},
		"crc32_checksum":  &CRC32ChecksumTool{},
		"hmac_sha256":     &HMACSHA256Tool{},
		"bcrypt_hash":     &BcryptHashTool{},
		"bcrypt_verify":   &BcryptVerifyTool{},
		"checksum_verify": &ChecksumVerifyTool{},
	}, map[string]bool{
		"md5_hash":        true,
		"sha1_hash":       true,
		"sha256_hash":     true,
		"sha512_hash":     true,
		"crc32_checksum":  true,
		"hmac_sha256":     true,
		"bcrypt_hash":     true,
		"bcrypt_verify":   true,
		"checksum_verify": true,
	})
}

type MD5HashTool struct{}

func (t *MD5HashTool) Name() string {
	return "md5_hash"
}

func (t *MD5HashTool) Description() string {
	return "Compute MD5 hash"
}

func (t *MD5HashTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "Text to hash",
				"default":     "",
			},
		},
	}
}

func (t *MD5HashTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	hash := md5.Sum([]byte(text))
	return &ToolResult{Content: hex.EncodeToString(hash[:])}, nil
}

type SHA1HashTool struct{}

func (t *SHA1HashTool) Name() string {
	return "sha1_hash"
}

func (t *SHA1HashTool) Description() string {
	return "Compute SHA-1 hash"
}

func (t *SHA1HashTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "Text to hash",
				"default":     "",
			},
		},
	}
}

func (t *SHA1HashTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	hash := sha1.Sum([]byte(text))
	return &ToolResult{Content: hex.EncodeToString(hash[:])}, nil
}

type SHA256HashTool struct{}

func (t *SHA256HashTool) Name() string {
	return "sha256_hash"
}

func (t *SHA256HashTool) Description() string {
	return "Compute SHA-256 hash"
}

func (t *SHA256HashTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "Text to hash",
				"default":     "",
			},
		},
	}
}

func (t *SHA256HashTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	hash := sha256.Sum256([]byte(text))
	return &ToolResult{Content: hex.EncodeToString(hash[:])}, nil
}

type SHA512HashTool struct{}

func (t *SHA512HashTool) Name() string {
	return "sha512_hash"
}

func (t *SHA512HashTool) Description() string {
	return "Compute SHA-512 hash"
}

func (t *SHA512HashTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "Text to hash",
				"default":     "",
			},
		},
	}
}

func (t *SHA512HashTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	hash := sha512.Sum512([]byte(text))
	return &ToolResult{Content: hex.EncodeToString(hash[:])}, nil
}

type CRC32ChecksumTool struct{}

func (t *CRC32ChecksumTool) Name() string {
	return "crc32_checksum"
}

func (t *CRC32ChecksumTool) Description() string {
	return "Compute CRC32 checksum"
}

func (t *CRC32ChecksumTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "Text to checksum",
				"default":     "",
			},
		},
	}
}

func (t *CRC32ChecksumTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	checksum := crc32.ChecksumIEEE([]byte(text))
	return &ToolResult{Content: strconv.FormatUint(uint64(checksum), 10)}, nil
}

type HMACSHA256Tool struct{}

func (t *HMACSHA256Tool) Name() string {
	return "hmac_sha256"
}

func (t *HMACSHA256Tool) Description() string {
	return "Compute HMAC-SHA256"
}

func (t *HMACSHA256Tool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "Text to hash",
				"default":     "",
			},
			"key": map[string]any{
				"type":        "string",
				"description": "Secret key",
				"default":     "",
			},
		},
	}
}

func (t *HMACSHA256Tool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	key, _ := args["key"].(string)
	h := hmac.New(sha256.New, []byte(key))
	h.Write([]byte(text))
	return &ToolResult{Content: hex.EncodeToString(h.Sum(nil))}, nil
}

type BcryptHashTool struct{}

func (t *BcryptHashTool) Name() string {
	return "bcrypt_hash"
}

func (t *BcryptHashTool) Description() string {
	return "Hash password with bcrypt"
}

func (t *BcryptHashTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"password": map[string]any{
				"type":        "string",
				"description": "Password to hash",
				"default":     "",
			},
			"cost": map[string]any{
				"type":        "integer",
				"description": "Bcrypt cost factor (4-31)",
				"default":     10,
			},
		},
	}
}

func (t *BcryptHashTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	password, _ := args["password"].(string)
	cost := 10
	if c, ok := args["cost"].(float64); ok {
		cost = int(c)
	} else if c, ok := args["cost"].(int); ok {
		cost = c
	}
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		return &ToolResult{Content: fmt.Sprintf("cost must be between %d and %d", bcrypt.MinCost, bcrypt.MaxCost), IsError: true}, nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return &ToolResult{Content: err.Error(), IsError: true}, nil
	}
	return &ToolResult{Content: string(hash)}, nil
}

type BcryptVerifyTool struct{}

func (t *BcryptVerifyTool) Name() string {
	return "bcrypt_verify"
}

func (t *BcryptVerifyTool) Description() string {
	return "Verify bcrypt password hash"
}

func (t *BcryptVerifyTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"password": map[string]any{
				"type":        "string",
				"description": "Password to verify",
				"default":     "",
			},
			"hash": map[string]any{
				"type":        "string",
				"description": "Bcrypt hash to verify against",
				"default":     "",
			},
		},
	}
}

func (t *BcryptVerifyTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	password, _ := args["password"].(string)
	hash, _ := args["hash"].(string)
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err == nil {
		return &ToolResult{Content: "match"}, nil
	}
	return &ToolResult{Content: "no match"}, nil
}

type ChecksumVerifyTool struct{}

func (t *ChecksumVerifyTool) Name() string {
	return "checksum_verify"
}

func (t *ChecksumVerifyTool) Description() string {
	return "Verify checksum matches text"
}

func (t *ChecksumVerifyTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"text": map[string]any{
				"type":        "string",
				"description": "Text to verify",
				"default":     "",
			},
			"algorithm": map[string]any{
				"type":        "string",
				"description": "Hash algorithm",
				"enum":        []string{"md5", "sha1", "sha256", "sha512", "crc32"},
				"default":     "sha256",
			},
			"expected": map[string]any{
				"type":        "string",
				"description": "Expected checksum value",
				"default":     "",
			},
		},
	}
}

func (t *ChecksumVerifyTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	text, _ := args["text"].(string)
	algorithm, _ := args["algorithm"].(string)
	expected, _ := args["expected"].(string)

	if algorithm == "" {
		algorithm = "sha256"
	}
	algorithm = strings.ToLower(strings.TrimSpace(algorithm))
	expected = strings.ToLower(strings.TrimSpace(expected))

	var computed string
	switch algorithm {
	case "md5":
		hash := md5.Sum([]byte(text))
		computed = hex.EncodeToString(hash[:])
	case "sha1":
		hash := sha1.Sum([]byte(text))
		computed = hex.EncodeToString(hash[:])
	case "sha256":
		hash := sha256.Sum256([]byte(text))
		computed = hex.EncodeToString(hash[:])
	case "sha512":
		hash := sha512.Sum512([]byte(text))
		computed = hex.EncodeToString(hash[:])
	case "crc32":
		checksum := crc32.ChecksumIEEE([]byte(text))
		computed = strconv.FormatUint(uint64(checksum), 10)
	default:
		return &ToolResult{Content: fmt.Sprintf("unsupported algorithm %q", algorithm), IsError: true}, nil
	}

	if computed == expected {
		return &ToolResult{Content: "valid"}, nil
	}
	return &ToolResult{Content: "invalid"}, nil
}
