package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

func TestValidateJSON(t *testing.T) {
	tool := &ValidateJSONTool{}
	tests := []struct {
		name  string
		args  map[string]any
		valid bool
	}{
		{"valid object", map[string]any{"json": `{"key":"value"}`}, true},
		{"valid array", map[string]any{"json": `[1,2,3]`}, true},
		{"invalid", map[string]any{"json": `{invalid`}, false},
		{"empty args", map[string]any{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), tt.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var out map[string]any
			json.Unmarshal([]byte(result.Content), &out)
			if out["valid"] != tt.valid {
				t.Errorf("got valid=%v, want %v", out["valid"], tt.valid)
			}
		})
	}
}

func TestValidateCreditCard(t *testing.T) {
	tool := &ValidateCreditCardTool{}
	tests := []struct {
		name     string
		number   string
		valid    bool
		cardType string
	}{
		{"valid visa", "4532015112830366", true, "visa"},
		{"valid mastercard", "5425233430109903", true, "mastercard"},
		{"invalid luhn", "4532015112830367", false, ""},
		{"empty args", "", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), map[string]any{"number": tt.number})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var out map[string]any
			json.Unmarshal([]byte(result.Content), &out)
			if out["valid"] != tt.valid {
				t.Errorf("got valid=%v, want %v", out["valid"], tt.valid)
			}
			if tt.cardType != "" && out["type"] != tt.cardType {
				t.Errorf("got type=%v, want %v", out["type"], tt.cardType)
			}
		})
	}
}

func TestValidateISBN(t *testing.T) {
	tool := &ValidateISBNTool{}
	tests := []struct {
		name    string
		isbn    string
		valid   bool
		version int
	}{
		{"valid isbn10", "0-306-40615-2", true, 10},
		{"valid isbn13", "978-0-306-40615-7", true, 13},
		{"invalid", "123", false, 0},
		{"empty args", "", false, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), map[string]any{"isbn": tt.isbn})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var out map[string]any
			json.Unmarshal([]byte(result.Content), &out)
			if out["valid"] != tt.valid {
				t.Errorf("got valid=%v, want %v", out["valid"], tt.valid)
			}
			if tt.valid {
				if v, ok := out["version"].(float64); !ok || int(v) != tt.version {
					t.Errorf("got version=%v, want %v", out["version"], tt.version)
				}
			}
		})
	}
}

func TestValidateIBAN(t *testing.T) {
	tool := &ValidateIBANTool{}
	tests := []struct {
		name    string
		iban    string
		valid   bool
		country string
	}{
		{"valid DE", "DE89370400440532013000", true, "DE"},
		{"valid GB", "GB82WEST12345698765432", true, "GB"},
		{"invalid", "INVALID", false, ""},
		{"empty args", "", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), map[string]any{"iban": tt.iban})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var out map[string]any
			json.Unmarshal([]byte(result.Content), &out)
			if out["valid"] != tt.valid {
				t.Errorf("got valid=%v, want %v", out["valid"], tt.valid)
			}
			if tt.valid && out["country"] != tt.country {
				t.Errorf("got country=%v, want %v", out["country"], tt.country)
			}
		})
	}
}

func TestValidateMACAddress(t *testing.T) {
	tool := &ValidateMACAddressTool{}
	tests := []struct {
		name   string
		mac    string
		valid  bool
		format string
	}{
		{"colon", "00:1A:2B:3C:4D:5E", true, "colon"},
		{"hyphen", "00-1A-2B-3C-4D-5E", true, "hyphen"},
		{"dot", "001A.2B3C.4D5E", true, "dot"},
		{"invalid", "invalid", false, ""},
		{"empty args", "", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), map[string]any{"mac": tt.mac})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var out map[string]any
			json.Unmarshal([]byte(result.Content), &out)
			if out["valid"] != tt.valid {
				t.Errorf("got valid=%v, want %v", out["valid"], tt.valid)
			}
			if tt.valid && out["format"] != tt.format {
				t.Errorf("got format=%v, want %v", out["format"], tt.format)
			}
		})
	}
}

func TestValidateHexColor(t *testing.T) {
	tool := &ValidateHexColorTool{}
	tests := []struct {
		name       string
		color      string
		valid      bool
		normalized string
	}{
		{"6 digit with hash", "#FF5733", true, "#FF5733"},
		{"6 digit no hash", "FF5733", true, "#FF5733"},
		{"3 digit", "#F73", true, "#FF7733"},
		{"invalid", "ZZZ", false, ""},
		{"empty args", "", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), map[string]any{"color": tt.color})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var out map[string]any
			json.Unmarshal([]byte(result.Content), &out)
			if out["valid"] != tt.valid {
				t.Errorf("got valid=%v, want %v", out["valid"], tt.valid)
			}
			if tt.valid && out["normalized"] != tt.normalized {
				t.Errorf("got normalized=%v, want %v", out["normalized"], tt.normalized)
			}
		})
	}
}

func TestValidateSemver(t *testing.T) {
	tool := &ValidateSemverTool{}
	tests := []struct {
		name    string
		version string
		valid   bool
		major   int
		minor   int
		patch   int
	}{
		{"basic", "1.2.3", true, 1, 2, 3},
		{"with v prefix", "v2.0.1", true, 2, 0, 1},
		{"with prerelease", "1.0.0-alpha", true, 1, 0, 0},
		{"invalid", "1.2", false, 0, 0, 0},
		{"empty args", "", false, 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), map[string]any{"version": tt.version})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var out map[string]any
			json.Unmarshal([]byte(result.Content), &out)
			if out["valid"] != tt.valid {
				t.Errorf("got valid=%v, want %v", out["valid"], tt.valid)
			}
			if tt.valid {
				if int(out["major"].(float64)) != tt.major {
					t.Errorf("got major=%v, want %v", out["major"], tt.major)
				}
			}
		})
	}
}

func TestSemverCompare(t *testing.T) {
	tool := &SemverCompareTool{}
	tests := []struct {
		name string
		v1   string
		v2   string
		want string
	}{
		{"v1 < v2", "1.0.0", "2.0.0", "-1"},
		{"v1 == v2", "1.2.3", "1.2.3", "0"},
		{"v1 > v2", "2.0.0", "1.0.0", "1"},
		{"patch diff", "1.0.1", "1.0.0", "1"},
		{"empty args", "", "", "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), map[string]any{"version1": tt.v1, "version2": tt.v2})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Content != tt.want {
				t.Errorf("got %v, want %v", result.Content, tt.want)
			}
		})
	}
}

func TestValidatePasswordStrength(t *testing.T) {
	tool := &ValidatePasswordStrengthTool{}
	tests := []struct {
		name     string
		password string
		minScore int
	}{
		{"weak", "abc", 0},
		{"medium", "Abc12345", 3},
		{"strong", "Abc123!@#", 4},
		{"empty args", "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), map[string]any{"password": tt.password})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var out map[string]any
			json.Unmarshal([]byte(result.Content), &out)
			score := int(out["score"].(float64))
			if score < tt.minScore {
				t.Errorf("got score=%v, want at least %v", score, tt.minScore)
			}
		})
	}
}

func TestValidatePhone(t *testing.T) {
	tool := &ValidatePhoneTool{}
	tests := []struct {
		name  string
		phone string
		valid bool
	}{
		{"valid US", "555-123-4567", true},
		{"valid intl", "+1234567890", true},
		{"too short", "123", false},
		{"empty args", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), map[string]any{"phone": tt.phone})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var out map[string]any
			json.Unmarshal([]byte(result.Content), &out)
			if out["valid"] != tt.valid {
				t.Errorf("got valid=%v, want %v", out["valid"], tt.valid)
			}
		})
	}
}

func TestValidateSSN(t *testing.T) {
	tool := &ValidateSSNTool{}
	tests := []struct {
		name  string
		ssn   string
		valid bool
	}{
		{"valid", "123-45-6789", true},
		{"invalid area 000", "000-45-6789", false},
		{"invalid area 666", "666-45-6789", false},
		{"invalid format", "12345678", false},
		{"empty args", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tool.Execute(context.Background(), map[string]any{"ssn": tt.ssn})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			var out map[string]any
			json.Unmarshal([]byte(result.Content), &out)
			if out["valid"] != tt.valid {
				t.Errorf("got valid=%v, want %v", out["valid"], tt.valid)
			}
		})
	}
}
