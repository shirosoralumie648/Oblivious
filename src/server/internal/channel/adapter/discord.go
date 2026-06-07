package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DiscordAdapter implements ChannelAdapter for the Discord platform.
type DiscordAdapter struct {
	client *http.Client
}

// NewDiscordAdapter creates a new Discord adapter with default settings.
func NewDiscordAdapter() *DiscordAdapter {
	return &DiscordAdapter{client: &http.Client{Timeout: 10 * time.Second}}
}

func (a *DiscordAdapter) Type() string {
	return "discord"
}

func (a *DiscordAdapter) TransformInbound(raw json.RawMessage) (InternalMessage, error) {
	var payload discordInboundPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return InternalMessage{}, fmt.Errorf("decode discord payload: %w", err)
	}

	content := []ContentPart{}
	if payload.Content != "" {
		content = append(content, ContentPart{Type: "text", Text: payload.Content})
	}
	for _, embed := range payload.Embeds {
		content = append(content, ContentPart{Type: "card", Metadata: map[string]any{"embed": embed}})
	}
	for _, attachment := range payload.Attachments {
		content = append(content, ContentPart{
			Type: "file",
			URL:  attachment.URL,
			Metadata: map[string]any{
				"attachment_id": attachment.ID,
				"name":          attachment.Filename,
				"mimetype":      attachment.ContentType,
				"size":          attachment.Size,
				"proxy_url":     attachment.ProxyURL,
			},
		})
	}

	role := "user"
	if payload.Author.Bot {
		role = "assistant"
	}

	message := InternalMessage{
		ID:             payload.ID,
		ConversationID: FirstNonEmpty(payload.ChannelID, payload.GuildID),
		Role:           role,
		Content:        content,
		Metadata: map[string]any{
			"adapter":   "discord",
			"author_id": payload.Author.ID,
		},
		Timestamp: time.Now().UTC(),
	}
	if err := ValidateMessage(message); err != nil {
		return InternalMessage{}, err
	}
	return message, nil
}

func (a *DiscordAdapter) TransformOutbound(message InternalMessage) (json.RawMessage, error) {
	if err := ValidateMessage(message); err != nil {
		return nil, err
	}

	payload := discordOutboundPayload{
		ChannelID: message.ConversationID,
		Content:   FirstTextPart(message.Content),
	}
	for _, part := range message.Content {
		if part.Type == "card" && len(part.Metadata) > 0 {
			payload.Embeds = append(payload.Embeds, part.Metadata)
		}
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode discord payload: %w", err)
	}
	return raw, nil
}

func (a *DiscordAdapter) TestConnection(ctx context.Context, config map[string]any) error {
	webhookURL := extractURL(config, "webhook_url", "webhookURL", "url")
	if webhookURL == "" {
		return fmt.Errorf("discord webhook_url is required")
	}
	return probeURL(ctx, a.client, webhookURL)
}

type discordInboundPayload struct {
	ID          string               `json:"id,omitempty"`
	ChannelID   string               `json:"channel_id,omitempty"`
	GuildID     string               `json:"guild_id,omitempty"`
	Content     string               `json:"content,omitempty"`
	Embeds      []map[string]any     `json:"embeds,omitempty"`
	Attachments []discordAttachment  `json:"attachments,omitempty"`
	Author      discordAuthor        `json:"author,omitempty"`
}

type discordAuthor struct {
	ID  string `json:"id,omitempty"`
	Bot bool   `json:"bot,omitempty"`
}

type discordAttachment struct {
	ID          string `json:"id,omitempty"`
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Size        int64  `json:"size,omitempty"`
	URL         string `json:"url,omitempty"`
	ProxyURL    string `json:"proxy_url,omitempty"`
}

type discordOutboundPayload struct {
	ChannelID string           `json:"channel_id,omitempty"`
	Content   string           `json:"content,omitempty"`
	Embeds    []map[string]any `json:"embeds,omitempty"`
}
