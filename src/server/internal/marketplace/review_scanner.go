package marketplace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const defaultReviewScannerName = "marketplace_static_v2"
const defaultReviewPolicyVersion = "marketplace_static_policy_v2"
const reviewRuleToolsValidJSON = "tools.valid_json"
const reviewRuleExternalWebhookEgress = "tools.external_webhook_egress"

type reviewPolicyRule struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Severity string   `json:"severity"`
	Message  string   `json:"message"`
	Needles  []string `json:"needles"`
}

var textReviewRules = []reviewPolicyRule{
	{
		ID:       "text.prompt_injection.override_or_reveal",
		Type:     "prompt_injection",
		Severity: "critical",
		Message:  "Prompt content attempts to override instructions or reveal hidden prompts.",
		Needles: []string{
			"ignore previous instructions",
			"ignore all previous instructions",
			"reveal hidden system",
			"reveal system prompt",
			"developer message",
			"bypass safety",
			"jailbreak",
		},
	},
	{
		ID:       "text.sensitive_api.credential_extraction",
		Type:     "sensitive_api",
		Severity: "high",
		Message:  "Prompt content references credentials or token extraction.",
		Needles: []string{
			"api key",
			"secret key",
			"access token",
			"user token",
			"password",
			"credential",
		},
	},
	{
		ID:       "text.policy_violation.marketplace_blocklist",
		Type:     "policy_violation",
		Severity: "critical",
		Message:  "Content matches a Marketplace policy risk category.",
		Needles: []string{
			"violent extremist",
			"sexual content involving minors",
			"malware",
			"credential exfiltration",
		},
	},
}

var toolReviewRules = []reviewPolicyRule{
	{
		ID:       "tools.sensitive_api.admin_or_credentials",
		Type:     "sensitive_api",
		Severity: "critical",
		Message:  "Tool configuration references sensitive credential or administrative API access.",
		Needles: []string{
			"/oauth/tokens",
			"oauth/tokens",
			"admin:read",
			"admin:write",
			"users.export",
			"credential",
			"secret",
			"api_key",
			"api key",
			"access_token",
			"access token",
			"password",
		},
	},
	{
		ID:       "tools.malicious_code.inline_shell_or_evasion",
		Type:     "malicious_code",
		Severity: "critical",
		Message:  "Tool configuration contains inline code or shell patterns associated with destructive or evasive execution.",
		Needles: []string{
			"child_process",
			"child_process.exec",
			"eval(",
			"function(",
			"subprocess",
			"os.system",
			"rm -rf",
			"base64 -d",
			"curl ",
			"wget ",
		},
	},
	{
		ID:       "tools.unsafe_tool.execution_or_data_movement",
		Type:     "unsafe_tool",
		Severity: "high",
		Message:  "Tool configuration exposes unsafe execution or data movement capabilities.",
		Needles: []string{
			"shell",
			"exec",
			"filesystem",
			"delete_all",
			"webhook",
			"exfiltrate",
		},
	},
}

// StaticReviewScanner is the deterministic first-pass Marketplace scanner.
type StaticReviewScanner struct{}

func NewStaticReviewScanner() StaticReviewScanner {
	return StaticReviewScanner{}
}

func (StaticReviewScanner) ScanAgent(ctx context.Context, agent PublishedAgent) (AutomatedReviewResult, error) {
	select {
	case <-ctx.Done():
		return AutomatedReviewResult{}, ctx.Err()
	default:
	}

	findings := make([]ReviewFinding, 0)
	textFields := []struct {
		field string
		value string
	}{
		{field: "name", value: agent.Name},
		{field: "description", value: agent.Description},
		{field: "system_prompt", value: agent.SystemPrompt},
		{field: "example_conversations", value: agent.ExampleConversations},
	}
	for _, textField := range textFields {
		findings = append(findings, scanTextField(textField.field, textField.value)...)
	}

	toolFindings, err := scanTools(agent.Tools)
	if err != nil {
		findings = append(findings, ReviewFinding{
			RuleID:   reviewRuleToolsValidJSON,
			Type:     "unsafe_tool",
			Severity: "high",
			Field:    "tools",
			Message:  "Tools configuration must be valid structured JSON.",
			Evidence: err.Error(),
		})
	} else {
		findings = append(findings, toolFindings...)
	}

	decision := "pending_manual_review"
	if hasSevereFinding(findings) {
		decision = "rejected"
	}
	return AutomatedReviewResult{
		AgentID:        agent.ID,
		Decision:       decision,
		Scanner:        defaultReviewScannerName,
		PolicyVersion:  defaultReviewPolicyVersion,
		PolicyChecksum: staticReviewPolicyChecksum(),
		Findings:       findings,
		CreatedAt:      time.Now().UTC(),
	}, nil
}

func staticReviewPolicyChecksum() string {
	payload, err := json.Marshal(reviewPolicyManifest())
	if err != nil {
		payload = []byte(defaultReviewPolicyVersion)
	}
	sum := sha256.Sum256(payload)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func reviewPolicyManifest() struct {
	Version string             `json:"version"`
	Rules   []reviewPolicyRule `json:"rules"`
} {
	rules := make([]reviewPolicyRule, 0, len(textReviewRules)+len(toolReviewRules)+2)
	rules = append(rules, textReviewRules...)
	rules = append(rules, toolReviewRules...)
	rules = append(rules, reviewPolicyRule{
		ID:       reviewRuleExternalWebhookEgress,
		Type:     "unsafe_tool",
		Severity: "high",
		Message:  "Tool configuration defines an external webhook data egress path.",
		Needles: []string{
			"{{user.",
			"{{conversation.",
			"conversation.transcript",
			"user.email",
			"collector",
			"ingest",
			"exfiltrate",
		},
	})
	rules = append(rules, reviewPolicyRule{
		ID:       reviewRuleToolsValidJSON,
		Type:     "unsafe_tool",
		Severity: "high",
		Message:  "Tools configuration must be valid structured JSON.",
	})
	return struct {
		Version string             `json:"version"`
		Rules   []reviewPolicyRule `json:"rules"`
	}{
		Version: defaultReviewPolicyVersion,
		Rules:   rules,
	}
}

func scanTextField(field, value string) []ReviewFinding {
	normalized := strings.ToLower(value)
	findings := make([]ReviewFinding, 0)
	for _, rule := range textReviewRules {
		if containsAny(normalized, rule.Needles) {
			findings = append(findings, reviewFindingForRule(rule, field, value))
		}
	}
	return findings
}

func scanTools(rawTools string) ([]ReviewFinding, error) {
	var payload any
	if err := json.Unmarshal([]byte(rawTools), &payload); err != nil {
		return nil, err
	}
	toolText := strings.ToLower(flattenJSONText(payload))
	findings := make([]ReviewFinding, 0)
	findings = append(findings, scanExternalWebhookEgress(payload, toolText)...)
	for _, rule := range toolReviewRules {
		if containsAny(toolText, rule.Needles) {
			findings = append(findings, reviewFindingForRule(rule, "tools", flattenJSONText(payload)))
		}
	}
	return findings, nil
}

func scanExternalWebhookEgress(payload any, toolText string) []ReviewFinding {
	if !containsAny(toolText, []string{
		"{{user.",
		"{{conversation.",
		"conversation.transcript",
		"user.email",
		"collector",
		"ingest",
		"exfiltrate",
	}) {
		return nil
	}

	var findings []ReviewFinding
	for _, rawURL := range extractHTTPURLs(payload) {
		parsed, err := url.Parse(rawURL)
		if err != nil || parsed.Hostname() == "" {
			continue
		}
		findings = append(findings, ReviewFinding{
			RuleID:   reviewRuleExternalWebhookEgress,
			Type:     "unsafe_tool",
			Severity: "high",
			Field:    "tools",
			Message:  "Tool configuration defines an external webhook data egress path.",
			Evidence: summarizeEvidence(parsed.Hostname() + " " + rawURL),
		})
	}
	return findings
}

func reviewFindingForRule(rule reviewPolicyRule, field, evidence string) ReviewFinding {
	return ReviewFinding{
		RuleID:   rule.ID,
		Type:     rule.Type,
		Severity: rule.Severity,
		Field:    field,
		Message:  rule.Message,
		Evidence: summarizeEvidence(evidence),
	}
}

func extractHTTPURLs(value any) []string {
	switch typed := value.(type) {
	case map[string]any:
		urls := make([]string, 0, len(typed))
		for _, value := range typed {
			urls = append(urls, extractHTTPURLs(value)...)
		}
		return urls
	case []any:
		urls := make([]string, 0, len(typed))
		for _, value := range typed {
			urls = append(urls, extractHTTPURLs(value)...)
		}
		return urls
	case string:
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(typed)), "http://") ||
			strings.HasPrefix(strings.ToLower(strings.TrimSpace(typed)), "https://") {
			return []string{strings.TrimSpace(typed)}
		}
	}
	return nil
}

func flattenJSONText(value any) string {
	switch typed := value.(type) {
	case map[string]any:
		parts := make([]string, 0, len(typed)*2)
		for key, value := range typed {
			parts = append(parts, key, flattenJSONText(value))
		}
		return strings.Join(parts, " ")
	case []any:
		parts := make([]string, 0, len(typed))
		for _, value := range typed {
			parts = append(parts, flattenJSONText(value))
		}
		return strings.Join(parts, " ")
	case string:
		return typed
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

func containsAny(value string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}

func summarizeEvidence(value string) string {
	trimmed := strings.Join(strings.Fields(value), " ")
	if len(trimmed) <= 160 {
		return trimmed
	}
	return trimmed[:160]
}

func hasSevereFinding(findings []ReviewFinding) bool {
	for _, finding := range findings {
		if finding.Severity == "high" || finding.Severity == "critical" {
			return true
		}
	}
	return false
}
