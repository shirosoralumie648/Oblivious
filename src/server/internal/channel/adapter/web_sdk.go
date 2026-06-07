package adapter

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

// WebSDKAdapter implements ChannelAdapter for the Web SDK / web embed platform.
// It handles messages from embedded web widgets (iframes, SDK clients).
type WebSDKAdapter struct {
	client *http.Client
}

// NewWebSDKAdapter creates a new Web SDK adapter with default settings.
func NewWebSDKAdapter() *WebSDKAdapter {
	return &WebSDKAdapter{client: &http.Client{Timeout: 10 * time.Second}}
}

func (a *WebSDKAdapter) Type() string {
	return "web_sdk"
}

func (a *WebSDKAdapter) TransformInbound(raw json.RawMessage) (InternalMessage, error) {
	var payload webSDKInboundPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return InternalMessage{}, fmt.Errorf("decode web sdk payload: %w", err)
	}

	content := append([]ContentPart(nil), payload.Content...)
	if len(content) == 0 && payload.Text != "" {
		content = append(content, ContentPart{Type: "text", Text: payload.Text})
	}

	metadata := map[string]any{}
	for key, value := range payload.Metadata {
		metadata[key] = value
	}
	metadata["adapter"] = "web_sdk"
	if payload.EmbedOrigin != "" {
		metadata["embed_origin"] = payload.EmbedOrigin
	}

	role := payload.Role
	if role == "" {
		role = "user"
	}

	message := InternalMessage{
		ID:             payload.ID,
		ConversationID: payload.ConversationID,
		Role:           role,
		Content:        content,
		Metadata:       metadata,
		Timestamp:      time.Now().UTC(),
	}
	if err := ValidateMessage(message); err != nil {
		return InternalMessage{}, err
	}
	return message, nil
}

func (a *WebSDKAdapter) TransformOutbound(message InternalMessage) (json.RawMessage, error) {
	if err := ValidateMessage(message); err != nil {
		return nil, err
	}

	payload := webSDKOutboundPayload{
		SDKEvent:       "message",
		ID:             message.ID,
		ConversationID: message.ConversationID,
		Role:           message.Role,
		Text:           FirstTextPart(message.Content),
		Content:        message.Content,
		Metadata:       message.Metadata,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode web sdk payload: %w", err)
	}
	return raw, nil
}

func (a *WebSDKAdapter) TestConnection(ctx context.Context, config map[string]any) error {
	origin := ""
	if config != nil {
		if v, ok := config["origin"].(string); ok {
			origin = v
		}
	}
	if origin == "" {
		origin = extractURL(config, "webhook_url", "webhookURL", "url")
	}
	if origin == "" {
		return fmt.Errorf("web_sdk origin or webhook_url is required")
	}

	origin = strings.TrimRight(origin, "/")
	healthURL := origin + "/health"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		return fmt.Errorf("invalid health url: %w", err)
	}

	client := a.client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}
	return nil
}

// DeliverMessage sends a message to the web SDK webhook endpoint.
func (a *WebSDKAdapter) DeliverMessage(ctx context.Context, config map[string]any, raw json.RawMessage) error {
	webhookURL := extractURL(config, "webhook_url", "webhookURL", "url")
	if webhookURL == "" {
		return nil
	}

	client := a.client
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(raw))
	if err != nil {
		return fmt.Errorf("build web sdk request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("web sdk delivery failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	detail := strings.TrimSpace(string(body))
	if detail != "" {
		return fmt.Errorf("web sdk delivery failed with status %d: %s", resp.StatusCode, detail)
	}
	return fmt.Errorf("web sdk delivery failed with status %d", resp.StatusCode)
}

type webSDKInboundPayload struct {
	SDKEvent       string         `json:"sdk_event,omitempty"`
	ID             string         `json:"id,omitempty"`
	ConversationID string         `json:"conversation_id"`
	Role           string         `json:"role"`
	Text           string         `json:"text,omitempty"`
	Content        []ContentPart  `json:"content,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	EmbedOrigin    string         `json:"embed_origin,omitempty"`
}

type webSDKOutboundPayload struct {
	SDKEvent       string         `json:"sdk_event,omitempty"`
	ID             string         `json:"id,omitempty"`
	ConversationID string         `json:"conversation_id"`
	Role           string         `json:"role"`
	Text           string         `json:"text,omitempty"`
	Content        []ContentPart  `json:"content,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}
