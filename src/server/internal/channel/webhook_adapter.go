package channel

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type WebhookAdapter struct {
	client *http.Client
}

func NewWebhookAdapter() *WebhookAdapter {
	return &WebhookAdapter{client: &http.Client{Timeout: 10 * time.Second}}
}

func (a *WebhookAdapter) Type() ChannelType {
	return ChannelTypeWebhook
}

func (a *WebhookAdapter) TransformInbound(raw json.RawMessage) (InternalMessage, error) {
	var payload webhookPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return InternalMessage{}, fmt.Errorf("decode webhook payload: %w", err)
	}

	message := InternalMessage{
		ID:             payload.ID,
		ConversationID: payload.ConversationID,
		Role:           payload.Role,
		Content:        payload.Content,
		Metadata:       payload.Metadata,
		Timestamp:      payload.Timestamp,
	}
	if len(message.Content) == 0 && payload.Text != "" {
		message.Content = []ContentPart{{Type: ContentTypeText, Text: payload.Text}}
	}
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now().UTC()
	}
	if err := validateInternalMessage(message); err != nil {
		return InternalMessage{}, err
	}
	return message, nil
}

func (a *WebhookAdapter) TransformOutbound(message InternalMessage) (json.RawMessage, error) {
	if err := validateInternalMessage(message); err != nil {
		return nil, err
	}

	payload := webhookPayload{
		ID:             message.ID,
		ConversationID: message.ConversationID,
		Role:           message.Role,
		Content:        message.Content,
		Metadata:       message.Metadata,
		Timestamp:      message.Timestamp,
	}
	for _, part := range message.Content {
		if part.Type == ContentTypeText && part.Text != "" {
			payload.Text = part.Text
			break
		}
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode webhook payload: %w", err)
	}
	return raw, nil
}

func (a *WebhookAdapter) DeliverOutbound(ctx context.Context, config map[string]any, raw json.RawMessage) error {
	url := webhookOutboundURL(config)
	if url == "" {
		return nil
	}

	client := a.client
	if client == nil {
		client = http.DefaultClient
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return fmt.Errorf("webhook delivery timeout: %w", ctx.Err())
		}
		return fmt.Errorf("webhook delivery failed: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		_, _ = io.Copy(io.Discard, response.Body)
		return nil
	}

	body, _ := io.ReadAll(io.LimitReader(response.Body, 512))
	detail := strings.TrimSpace(string(body))
	if detail != "" {
		return fmt.Errorf("webhook delivery failed with status %d: %s", response.StatusCode, detail)
	}
	return fmt.Errorf("webhook delivery failed with status %d", response.StatusCode)
}

func webhookOutboundURL(config map[string]any) string {
	if config == nil {
		return ""
	}
	for _, key := range []string{"webhook_url", "webhookURL", "url"} {
		if value, ok := config[key].(string); ok {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

type webhookPayload struct {
	ID             string         `json:"id"`
	ConversationID string         `json:"conversation_id"`
	Role           Role           `json:"role"`
	Text           string         `json:"text,omitempty"`
	Content        []ContentPart  `json:"content,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	Timestamp      time.Time      `json:"timestamp,omitempty"`
}

func validateInternalMessage(message InternalMessage) error {
	if message.ConversationID == "" {
		return fmt.Errorf("conversation_id is required")
	}
	if !validRole(message.Role) {
		return fmt.Errorf("role %q is not supported", message.Role)
	}
	if len(message.Content) == 0 {
		return fmt.Errorf("content is required")
	}
	for _, part := range message.Content {
		if !validContentType(part.Type) {
			return fmt.Errorf("content type %q is not supported", part.Type)
		}
	}
	return nil
}
