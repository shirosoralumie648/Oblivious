package marketplace

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const defaultReviewScannerName = "marketplace_static_v1"

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
		AgentID:   agent.ID,
		Decision:  decision,
		Scanner:   defaultReviewScannerName,
		Findings:  findings,
		CreatedAt: time.Now().UTC(),
	}, nil
}

func scanTextField(field, value string) []ReviewFinding {
	normalized := strings.ToLower(value)
	findings := make([]ReviewFinding, 0)
	if containsAny(normalized, []string{
		"ignore previous instructions",
		"ignore all previous instructions",
		"reveal hidden system",
		"reveal system prompt",
		"developer message",
		"bypass safety",
		"jailbreak",
	}) {
		findings = append(findings, ReviewFinding{
			Type:     "prompt_injection",
			Severity: "critical",
			Field:    field,
			Message:  "Prompt content attempts to override instructions or reveal hidden prompts.",
			Evidence: summarizeEvidence(value),
		})
	}
	if containsAny(normalized, []string{
		"api key",
		"secret key",
		"access token",
		"user token",
		"password",
		"credential",
	}) {
		findings = append(findings, ReviewFinding{
			Type:     "sensitive_api",
			Severity: "high",
			Field:    field,
			Message:  "Prompt content references credentials or token extraction.",
			Evidence: summarizeEvidence(value),
		})
	}
	if containsAny(normalized, []string{
		"violent extremist",
		"sexual content involving minors",
		"malware",
		"credential exfiltration",
	}) {
		findings = append(findings, ReviewFinding{
			Type:     "policy_violation",
			Severity: "critical",
			Field:    field,
			Message:  "Content matches a Marketplace policy risk category.",
			Evidence: summarizeEvidence(value),
		})
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
	if containsAny(toolText, []string{
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
	}) {
		findings = append(findings, ReviewFinding{
			Type:     "sensitive_api",
			Severity: "critical",
			Field:    "tools",
			Message:  "Tool configuration references sensitive credential or administrative API access.",
			Evidence: summarizeEvidence(flattenJSONText(payload)),
		})
	}
	if containsAny(toolText, []string{
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
	}) {
		findings = append(findings, ReviewFinding{
			Type:     "malicious_code",
			Severity: "critical",
			Field:    "tools",
			Message:  "Tool configuration contains inline code or shell patterns associated with destructive or evasive execution.",
			Evidence: summarizeEvidence(flattenJSONText(payload)),
		})
	}
	if containsAny(toolText, []string{
		"shell",
		"exec",
		"filesystem",
		"delete_all",
		"webhook",
		"exfiltrate",
	}) {
		findings = append(findings, ReviewFinding{
			Type:     "unsafe_tool",
			Severity: "high",
			Field:    "tools",
			Message:  "Tool configuration exposes unsafe execution or data movement capabilities.",
			Evidence: summarizeEvidence(flattenJSONText(payload)),
		})
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
			Type:     "unsafe_tool",
			Severity: "high",
			Field:    "tools",
			Message:  "Tool configuration defines an external webhook data egress path.",
			Evidence: summarizeEvidence(parsed.Hostname() + " " + rawURL),
		})
	}
	return findings
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
