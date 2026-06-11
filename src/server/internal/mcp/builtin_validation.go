package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func init() {
	registerBuiltins(map[string]BuiltinTool{
		"validate_json":              &ValidateJSONTool{},
		"validate_credit_card":       &ValidateCreditCardTool{},
		"validate_isbn":              &ValidateISBNTool{},
		"validate_iban":              &ValidateIBANTool{},
		"validate_mac_address":       &ValidateMACAddressTool{},
		"validate_hex_color":         &ValidateHexColorTool{},
		"validate_semver":            &ValidateSemverTool{},
		"semver_compare":             &SemverCompareTool{},
		"validate_password_strength": &ValidatePasswordStrengthTool{},
		"validate_phone":             &ValidatePhoneTool{},
		"validate_ssn":               &ValidateSSNTool{},
	}, map[string]bool{
		"validate_json":              true,
		"validate_credit_card":       true,
		"validate_isbn":              true,
		"validate_iban":              true,
		"validate_mac_address":       true,
		"validate_hex_color":         true,
		"validate_semver":            true,
		"semver_compare":             true,
		"validate_password_strength": true,
		"validate_phone":             true,
		"validate_ssn":               true,
	})
}

type ValidateJSONTool struct{}

func (t *ValidateJSONTool) Name() string        { return "validate_json" }
func (t *ValidateJSONTool) Description() string { return "Validate JSON syntax" }
func (t *ValidateJSONTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"json": map[string]any{"type": "string", "description": "JSON string to validate"},
		},
		"required": []string{"json"},
	}
}
func (t *ValidateJSONTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	input, ok := args["json"].(string)
	if !ok {
		input = ""
	}
	var v any
	err := json.Unmarshal([]byte(input), &v)
	result := map[string]any{"valid": err == nil}
	if err != nil {
		result["error"] = err.Error()
	}
	out, _ := json.Marshal(result)
	return &ToolResult{Content: string(out)}, nil
}

type ValidateCreditCardTool struct{}

func (t *ValidateCreditCardTool) Name() string { return "validate_credit_card" }
func (t *ValidateCreditCardTool) Description() string {
	return "Validate credit card number (Luhn algorithm)"
}
func (t *ValidateCreditCardTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"number": map[string]any{"type": "string", "description": "Credit card number"},
		},
		"required": []string{"number"},
	}
}
func (t *ValidateCreditCardTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	number, _ := args["number"].(string)
	number = strings.ReplaceAll(strings.ReplaceAll(number, " ", ""), "-", "")
	valid := luhnCheck(number)
	cardType := ""
	if valid && len(number) > 0 {
		switch number[0] {
		case '4':
			cardType = "visa"
		case '5':
			if len(number) > 1 && number[1] >= '1' && number[1] <= '5' {
				cardType = "mastercard"
			}
		case '3':
			if len(number) > 1 && (number[1] == '4' || number[1] == '7') {
				cardType = "amex"
			}
		case '6':
			cardType = "discover"
		}
	}
	result := map[string]any{"valid": valid}
	if cardType != "" {
		result["type"] = cardType
	}
	out, _ := json.Marshal(result)
	return &ToolResult{Content: string(out)}, nil
}

func luhnCheck(number string) bool {
	if len(number) < 13 || len(number) > 19 {
		return false
	}
	sum := 0
	alt := false
	for i := len(number) - 1; i >= 0; i-- {
		if number[i] < '0' || number[i] > '9' {
			return false
		}
		digit := int(number[i] - '0')
		if alt {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}
		sum += digit
		alt = !alt
	}
	return sum%10 == 0
}

type ValidateISBNTool struct{}

func (t *ValidateISBNTool) Name() string        { return "validate_isbn" }
func (t *ValidateISBNTool) Description() string { return "Validate ISBN-10 or ISBN-13" }
func (t *ValidateISBNTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"isbn": map[string]any{"type": "string", "description": "ISBN number"},
		},
		"required": []string{"isbn"},
	}
}
func (t *ValidateISBNTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	isbn, _ := args["isbn"].(string)
	isbn = strings.ReplaceAll(strings.ReplaceAll(isbn, "-", ""), " ", "")
	valid, version := false, 0
	if len(isbn) == 10 {
		valid, version = validateISBN10(isbn), 10
	} else if len(isbn) == 13 {
		valid, version = validateISBN13(isbn), 13
	}
	result := map[string]any{"valid": valid}
	if valid {
		result["version"] = version
	}
	out, _ := json.Marshal(result)
	return &ToolResult{Content: string(out)}, nil
}

func validateISBN10(isbn string) bool {
	if len(isbn) != 10 {
		return false
	}
	sum := 0
	for i := 0; i < 9; i++ {
		if isbn[i] < '0' || isbn[i] > '9' {
			return false
		}
		sum += int(isbn[i]-'0') * (10 - i)
	}
	last := isbn[9]
	if last == 'X' || last == 'x' {
		sum += 10
	} else if last >= '0' && last <= '9' {
		sum += int(last - '0')
	} else {
		return false
	}
	return sum%11 == 0
}

func validateISBN13(isbn string) bool {
	if len(isbn) != 13 {
		return false
	}
	sum := 0
	for i := 0; i < 13; i++ {
		if isbn[i] < '0' || isbn[i] > '9' {
			return false
		}
		digit := int(isbn[i] - '0')
		if i%2 == 0 {
			sum += digit
		} else {
			sum += digit * 3
		}
	}
	return sum%10 == 0
}

type ValidateIBANTool struct{}

func (t *ValidateIBANTool) Name() string { return "validate_iban" }
func (t *ValidateIBANTool) Description() string {
	return "Validate IBAN (International Bank Account Number)"
}
func (t *ValidateIBANTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"iban": map[string]any{"type": "string", "description": "IBAN string"},
		},
		"required": []string{"iban"},
	}
}
func (t *ValidateIBANTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	iban, _ := args["iban"].(string)
	iban = strings.ToUpper(strings.ReplaceAll(iban, " ", ""))
	valid := len(iban) >= 15 && len(iban) <= 34 && ibanMod97(iban) == 1
	result := map[string]any{"valid": valid}
	if valid && len(iban) >= 2 {
		result["country"] = iban[:2]
	}
	out, _ := json.Marshal(result)
	return &ToolResult{Content: string(out)}, nil
}

func ibanMod97(iban string) int {
	if len(iban) < 4 {
		return 0
	}
	rearranged := iban[4:] + iban[:4]
	var numStr strings.Builder
	for _, ch := range rearranged {
		if ch >= '0' && ch <= '9' {
			numStr.WriteByte(byte(ch))
		} else if ch >= 'A' && ch <= 'Z' {
			numStr.WriteString(strconv.Itoa(int(ch - 'A' + 10)))
		} else {
			return 0
		}
	}
	mod := 0
	for _, ch := range numStr.String() {
		mod = (mod*10 + int(ch-'0')) % 97
	}
	return mod
}

type ValidateMACAddressTool struct{}

func (t *ValidateMACAddressTool) Name() string        { return "validate_mac_address" }
func (t *ValidateMACAddressTool) Description() string { return "Validate MAC address format" }
func (t *ValidateMACAddressTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"mac": map[string]any{"type": "string", "description": "MAC address"},
		},
		"required": []string{"mac"},
	}
}
func (t *ValidateMACAddressTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	mac, _ := args["mac"].(string)
	patterns := map[string]*regexp.Regexp{
		"colon":  regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$`),
		"hyphen": regexp.MustCompile(`^([0-9A-Fa-f]{2}-){5}[0-9A-Fa-f]{2}$`),
		"dot":    regexp.MustCompile(`^([0-9A-Fa-f]{4}\.){2}[0-9A-Fa-f]{4}$`),
	}
	format := ""
	for name, pattern := range patterns {
		if pattern.MatchString(mac) {
			format = name
			break
		}
	}
	result := map[string]any{"valid": format != ""}
	if format != "" {
		result["format"] = format
	}
	out, _ := json.Marshal(result)
	return &ToolResult{Content: string(out)}, nil
}

type ValidateHexColorTool struct{}

func (t *ValidateHexColorTool) Name() string        { return "validate_hex_color" }
func (t *ValidateHexColorTool) Description() string { return "Validate hexadecimal color code" }
func (t *ValidateHexColorTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"color": map[string]any{"type": "string", "description": "Hex color code"},
		},
		"required": []string{"color"},
	}
}
func (t *ValidateHexColorTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	color, _ := args["color"].(string)
	pattern := regexp.MustCompile(`^#?([0-9A-Fa-f]{3}|[0-9A-Fa-f]{6})$`)
	valid := pattern.MatchString(color)
	normalized := color
	if valid {
		if !strings.HasPrefix(normalized, "#") {
			normalized = "#" + normalized
		}
		if len(normalized) == 4 {
			normalized = fmt.Sprintf("#%c%c%c%c%c%c", normalized[1], normalized[1], normalized[2], normalized[2], normalized[3], normalized[3])
		}
	}
	result := map[string]any{"valid": valid}
	if valid {
		result["normalized"] = normalized
	}
	out, _ := json.Marshal(result)
	return &ToolResult{Content: string(out)}, nil
}

type ValidateSemverTool struct{}

func (t *ValidateSemverTool) Name() string        { return "validate_semver" }
func (t *ValidateSemverTool) Description() string { return "Validate semantic version string" }
func (t *ValidateSemverTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"version": map[string]any{"type": "string", "description": "Semver string"},
		},
		"required": []string{"version"},
	}
}
func (t *ValidateSemverTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	version, _ := args["version"].(string)
	pattern := regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([^+]+))?(?:\+(.+))?$`)
	matches := pattern.FindStringSubmatch(version)
	valid := len(matches) > 0
	result := map[string]any{"valid": valid}
	if valid {
		major, _ := strconv.Atoi(matches[1])
		minor, _ := strconv.Atoi(matches[2])
		patch, _ := strconv.Atoi(matches[3])
		result["major"] = major
		result["minor"] = minor
		result["patch"] = patch
	}
	out, _ := json.Marshal(result)
	return &ToolResult{Content: string(out)}, nil
}

type SemverCompareTool struct{}

func (t *SemverCompareTool) Name() string        { return "semver_compare" }
func (t *SemverCompareTool) Description() string { return "Compare two semantic versions" }
func (t *SemverCompareTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"version1": map[string]any{"type": "string", "description": "First version"},
			"version2": map[string]any{"type": "string", "description": "Second version"},
		},
		"required": []string{"version1", "version2"},
	}
}
func (t *SemverCompareTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	v1, _ := args["version1"].(string)
	v2, _ := args["version2"].(string)
	sv1, ok1 := parseSemver(v1)
	sv2, ok2 := parseSemver(v2)
	if !ok1 || !ok2 {
		return &ToolResult{Content: "0", IsError: false}, nil
	}
	cmp := compareSemver(sv1, sv2)
	return &ToolResult{Content: strconv.Itoa(cmp)}, nil
}

type semver struct{ major, minor, patch int }

func parseSemver(v string) (semver, bool) {
	pattern := regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)`)
	matches := pattern.FindStringSubmatch(v)
	if len(matches) < 4 {
		return semver{}, false
	}
	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])
	return semver{major, minor, patch}, true
}

func compareSemver(v1, v2 semver) int {
	if v1.major != v2.major {
		if v1.major < v2.major {
			return -1
		}
		return 1
	}
	if v1.minor != v2.minor {
		if v1.minor < v2.minor {
			return -1
		}
		return 1
	}
	if v1.patch != v2.patch {
		if v1.patch < v2.patch {
			return -1
		}
		return 1
	}
	return 0
}

type ValidatePasswordStrengthTool struct{}

func (t *ValidatePasswordStrengthTool) Name() string        { return "validate_password_strength" }
func (t *ValidatePasswordStrengthTool) Description() string { return "Check password strength" }
func (t *ValidatePasswordStrengthTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"password": map[string]any{"type": "string", "description": "Password to check"},
		},
		"required": []string{"password"},
	}
}
func (t *ValidatePasswordStrengthTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	password, _ := args["password"].(string)
	score := 0
	feedback := []string{}
	if len(password) >= 8 {
		score++
	} else {
		feedback = append(feedback, "Add more characters (minimum 8)")
	}
	if regexp.MustCompile(`[A-Z]`).MatchString(password) {
		score++
	} else {
		feedback = append(feedback, "Add uppercase letters")
	}
	if regexp.MustCompile(`[a-z]`).MatchString(password) {
		score++
	} else {
		feedback = append(feedback, "Add lowercase letters")
	}
	if regexp.MustCompile(`[0-9]`).MatchString(password) {
		score++
	} else {
		feedback = append(feedback, "Add numbers")
	}
	if regexp.MustCompile(`[^A-Za-z0-9]`).MatchString(password) {
		score++
	}
	if score == 5 {
		feedback = []string{}
	}
	finalScore := score - 1
	if finalScore < 0 {
		finalScore = 0
	}
	result := map[string]any{"score": finalScore, "feedback": feedback}
	out, _ := json.Marshal(result)
	return &ToolResult{Content: string(out)}, nil
}

type ValidatePhoneTool struct{}

func (t *ValidatePhoneTool) Name() string { return "validate_phone" }
func (t *ValidatePhoneTool) Description() string {
	return "Validate phone number format (basic)"
}
func (t *ValidatePhoneTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"phone":        map[string]any{"type": "string", "description": "Phone number"},
			"country_code": map[string]any{"type": "string", "description": "Country code (optional)"},
		},
		"required": []string{"phone"},
	}
}
func (t *ValidatePhoneTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	phone, _ := args["phone"].(string)
	digits := regexp.MustCompile(`[^0-9]`).ReplaceAllString(phone, "")
	valid := len(digits) >= 10 && len(digits) <= 15
	normalized := ""
	if valid {
		if strings.HasPrefix(phone, "+") {
			normalized = "+" + digits
		} else {
			normalized = digits
		}
	}
	result := map[string]any{"valid": valid}
	if valid {
		result["normalized"] = normalized
	}
	out, _ := json.Marshal(result)
	return &ToolResult{Content: string(out)}, nil
}

type ValidateSSNTool struct{}

func (t *ValidateSSNTool) Name() string { return "validate_ssn" }
func (t *ValidateSSNTool) Description() string {
	return "Validate US Social Security Number format"
}
func (t *ValidateSSNTool) InputSchema() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"ssn": map[string]any{"type": "string", "description": "SSN"},
		},
		"required": []string{"ssn"},
	}
}
func (t *ValidateSSNTool) Execute(ctx context.Context, args map[string]any) (*ToolResult, error) {
	ssn, _ := args["ssn"].(string)
	pattern := regexp.MustCompile(`^(\d{3})-(\d{2})-(\d{4})$`)
	matches := pattern.FindStringSubmatch(ssn)
	valid := len(matches) > 0
	if valid {
		area, _ := strconv.Atoi(matches[1])
		if area == 0 || area == 666 || area >= 900 {
			valid = false
		}
	}
	result := map[string]any{"valid": valid}
	if valid {
		result["area"] = matches[1]
	}
	out, _ := json.Marshal(result)
	return &ToolResult{Content: string(out)}, nil
}
