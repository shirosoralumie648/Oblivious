package observability

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrAlertWebhookEndpointMissing = errors.New("alert webhook endpoint is required")

type AlertWebhookDeliverySinkOptions struct {
	EndpointURL string
	Secret      string
	HTTPClient  *http.Client
}

type WebhookAlertDeliverySink struct {
	endpointURL string
	secret      string
	client      *http.Client
}

func NewWebhookAlertDeliverySink(options AlertWebhookDeliverySinkOptions) *WebhookAlertDeliverySink {
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &WebhookAlertDeliverySink{
		endpointURL: strings.TrimSpace(options.EndpointURL),
		secret:      options.Secret,
		client:      client,
	}
}

func (s *WebhookAlertDeliverySink) Channel() AlertDeliveryChannel {
	return AlertDeliveryChannelThirdParty
}

func (s *WebhookAlertDeliverySink) Deliver(ctx context.Context, event AlertEvent) error {
	if s == nil || s.client == nil {
		return ErrAlertDeliverySinkMissing
	}
	if s.endpointURL == "" {
		return ErrAlertWebhookEndpointMissing
	}

	body, err := json.Marshal(alertWebhookPayload(event))
	if err != nil {
		return fmt.Errorf("encode alert webhook payload: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, s.endpointURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build alert webhook request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if key := alertKey(event); key != "" {
		request.Header.Set("X-Oblivious-Alert-Key", key)
	}
	if s.secret != "" {
		request.Header.Set("X-Oblivious-Alert-Signature", alertWebhookSignature(s.secret, body))
	}

	response, err := s.client.Do(request)
	if err != nil {
		return fmt.Errorf("deliver alert webhook: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return fmt.Errorf("alert webhook returned %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}
	return nil
}

func alertWebhookPayload(event AlertEvent) map[string]any {
	payload := map[string]any{
		"event":    "observability.alert.fired",
		"key":      alertKey(event),
		"severity": string(event.Severity),
		"title":    strings.TrimSpace(event.Title),
		"message":  strings.TrimSpace(event.Message),
	}
	if strings.TrimSpace(event.Component) != "" {
		payload["component"] = strings.TrimSpace(event.Component)
	}
	if !event.OccurredAt.IsZero() {
		payload["occurredAt"] = event.OccurredAt.UTC().Format(time.RFC3339Nano)
	}
	if event.OriginalSeverity != "" {
		payload["originalSeverity"] = string(event.OriginalSeverity)
	}
	if event.Escalated {
		payload["escalated"] = true
	}
	if len(event.Fields) > 0 {
		payload["fields"] = event.Fields
	}
	return payload
}

func alertWebhookSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}
