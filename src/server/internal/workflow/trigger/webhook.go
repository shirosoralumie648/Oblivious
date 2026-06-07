package trigger

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

var (
	ErrWebhookSecretRequired  = errors.New("webhook secret is required")
	ErrWebhookPayloadInvalid  = errors.New("webhook payload is invalid")
	ErrWebhookSignatureMismatch = errors.New("webhook signature does not match")
	ErrWebhookMethodNotAllowed = errors.New("webhook HTTP method not allowed")
)

// WebhookTrigger manages webhook-based workflow triggering.
type WebhookTrigger struct {
	ID             string
	URL            string
	Secret         string
	AllowedMethods []string
	Headers        map[string]string
	Definition     map[string]any
}

// NewWebhookTrigger creates a new webhook trigger with a generated URL.
func NewWebhookTrigger(id, baseURL, secret string) (*WebhookTrigger, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("webhook trigger ID is required")
	}
	url, err := GenerateWebhookURL(baseURL, id)
	if err != nil {
		return nil, err
	}
	return &WebhookTrigger{
		ID:             id,
		URL:            url,
		Secret:         strings.TrimSpace(secret),
		AllowedMethods: []string{"POST"},
	}, nil
}

// GenerateWebhookURL creates a webhook URL from a base URL and trigger ID.
func GenerateWebhookURL(baseURL, triggerID string) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	triggerID = strings.TrimSpace(triggerID)
	if baseURL == "" {
		return "", fmt.Errorf("base URL is required")
	}
	if triggerID == "" {
		return "", fmt.Errorf("trigger ID is required")
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf("%s/webhooks/workflow/%s", baseURL, triggerID), nil
}

// VerifySignature validates a webhook payload using HMAC-SHA256.
func VerifySignature(payload []byte, secret string, signature string) error {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return ErrWebhookSecretRequired
	}
	expected := ComputeHMACSHA256(payload, secret)
	provided := normalizeSignature(signature)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(provided)) != 1 {
		return ErrWebhookSignatureMismatch
	}
	return nil
}

// ComputeHMACSHA256 computes the HMAC-SHA256 hex digest of payload with the given secret.
func ComputeHMACSHA256(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// ParseWebhookPayload extracts and validates a webhook HTTP request into a map.
func ParseWebhookPayload(r *http.Request) (map[string]any, error) {
	if r == nil {
		return nil, fmt.Errorf("%w: request is nil", ErrWebhookPayloadInvalid)
	}
	if r.Body == nil {
		return nil, fmt.Errorf("%w: request body is empty", ErrWebhookPayloadInvalid)
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrWebhookPayloadInvalid, err)
	}
	if len(body) == 0 {
		return map[string]any{}, nil
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrWebhookPayloadInvalid, err)
	}
	return payload, nil
}

// ValidateWebhookRequest checks method, signature, and parses the payload.
func ValidateWebhookRequest(r *http.Request, secret string, allowedMethods []string) (map[string]any, error) {
	if r == nil {
		return nil, fmt.Errorf("%w: request is nil", ErrWebhookPayloadInvalid)
	}

	if len(allowedMethods) > 0 {
		methodAllowed := false
		for _, method := range allowedMethods {
			if strings.EqualFold(r.Method, strings.TrimSpace(method)) {
				methodAllowed = true
				break
			}
		}
		if !methodAllowed {
			return nil, fmt.Errorf("%w: %s", ErrWebhookMethodNotAllowed, r.Method)
		}
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrWebhookPayloadInvalid, err)
	}

	if strings.TrimSpace(secret) != "" {
		signature := extractSignature(r)
		if signature == "" {
			return nil, fmt.Errorf("%w: no signature header found", ErrWebhookSignatureMismatch)
		}
		if err := VerifySignature(body, secret, signature); err != nil {
			return nil, err
		}
	}

	if len(body) == 0 {
		return map[string]any{}, nil
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrWebhookPayloadInvalid, err)
	}
	return payload, nil
}

// extractSignature looks for the signature in common webhook header names.
func extractSignature(r *http.Request) string {
	for _, header := range []string{
		"X-Webhook-Signature",
		"X-Signature",
		"X-Hub-Signature-256",
		"X-Hook-Signature",
		"Webhook-Signature",
	} {
		if sig := strings.TrimSpace(r.Header.Get(header)); sig != "" {
			return sig
		}
	}
	return ""
}

func normalizeSignature(sig string) string {
	sig = strings.TrimSpace(sig)
	sig = strings.TrimPrefix(sig, "sha256=")
	sig = strings.TrimPrefix(sig, "SHA256=")
	return strings.ToLower(sig)
}
