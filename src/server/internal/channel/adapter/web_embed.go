package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// WebEmbedAdapter implements ChannelAdapter for iframe and Web SDK embeds.
type WebEmbedAdapter struct {
	client *http.Client
}

// NewWebEmbedAdapter creates a new web embed adapter with default settings.
func NewWebEmbedAdapter() *WebEmbedAdapter {
	return &WebEmbedAdapter{client: &http.Client{Timeout: 10 * time.Second}}
}

func (a *WebEmbedAdapter) Type() string {
	return "web_embed"
}

func (a *WebEmbedAdapter) TransformInbound(raw json.RawMessage) (InternalMessage, error) {
	var payload webEmbedInboundPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return InternalMessage{}, fmt.Errorf("decode web embed payload: %w", err)
	}

	content := append([]ContentPart(nil), payload.Content...)
	if len(content) == 0 && payload.Text != "" {
		content = append(content, ContentPart{Type: "text", Text: payload.Text})
	}

	metadata := map[string]any{}
	for key, value := range payload.Metadata {
		metadata[key] = value
	}
	metadata["adapter"] = "web_embed"
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

func (a *WebEmbedAdapter) TransformOutbound(message InternalMessage) (json.RawMessage, error) {
	if err := ValidateMessage(message); err != nil {
		return nil, err
	}

	raw, err := json.Marshal(webEmbedOutboundPayload{
		SDKEvent:       "message",
		ID:             message.ID,
		ConversationID: message.ConversationID,
		Role:           message.Role,
		Text:           FirstTextPart(message.Content),
		Content:        message.Content,
		Metadata:       message.Metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("encode web embed payload: %w", err)
	}
	return raw, nil
}

func (a *WebEmbedAdapter) TestConnection(ctx context.Context, config map[string]any) error {
	origin := ""
	if config != nil {
		if value, ok := config["origin"].(string); ok {
			origin = value
		}
	}
	if origin == "" {
		origin = extractURL(config, "webhook_url", "webhookURL", "url")
	}
	if origin == "" {
		return fmt.Errorf("web_embed origin or webhook_url is required")
	}
	return probeURL(ctx, a.client, strings.TrimRight(origin, "/")+"/health")
}

type webEmbedInboundPayload struct {
	SDKEvent       string         `json:"sdk_event,omitempty"`
	ID             string         `json:"id,omitempty"`
	ConversationID string         `json:"conversation_id"`
	Role           string         `json:"role"`
	Text           string         `json:"text,omitempty"`
	Content        []ContentPart  `json:"content,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	EmbedOrigin    string         `json:"embed_origin,omitempty"`
}

type webEmbedOutboundPayload struct {
	SDKEvent       string         `json:"sdk_event,omitempty"`
	ID             string         `json:"id,omitempty"`
	ConversationID string         `json:"conversation_id"`
	Role           string         `json:"role"`
	Text           string         `json:"text,omitempty"`
	Content        []ContentPart  `json:"content,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}
