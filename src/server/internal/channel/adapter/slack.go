package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SlackAdapter implements ChannelAdapter for the Slack platform.
type SlackAdapter struct {
	client *http.Client
}

// NewSlackAdapter creates a new Slack adapter with default settings.
func NewSlackAdapter() *SlackAdapter {
	return &SlackAdapter{client: &http.Client{Timeout: 10 * time.Second}}
}

func (a *SlackAdapter) Type() string {
	return "slack"
}

func (a *SlackAdapter) TransformInbound(raw json.RawMessage) (InternalMessage, error) {
	var payload slackInboundPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return InternalMessage{}, fmt.Errorf("decode slack payload: %w", err)
	}

	content := []ContentPart{}
	if payload.Event.Text != "" {
		content = append(content, ContentPart{Type: "text", Text: payload.Event.Text})
	}
	files := payload.eventFiles()
	for _, file := range files {
		content = append(content, ContentPart{
			Type: "file",
			URL:  FirstNonEmpty(file.URLPrivate, file.URLPrivateDownload, file.Permalink),
			Metadata: map[string]any{
				"file_id":  file.ID,
				"name":     file.Name,
				"mimetype": file.Mimetype,
				"size":     file.Size,
			},
		})
	}

	role := "user"
	if payload.Event.BotID != "" {
		role = "assistant"
	}

	message := InternalMessage{
		ID:             FirstNonEmpty(payload.Event.ClientMsgID, payload.EventID, payload.Event.TS),
		ConversationID: payload.Event.Channel,
		Role:           role,
		Content:        content,
		Metadata: map[string]any{
			"adapter": "slack",
			"user_id": payload.Event.User,
		},
		Timestamp: time.Now().UTC(),
	}
	if err := ValidateMessage(message); err != nil {
		return InternalMessage{}, err
	}
	return message, nil
}

func (a *SlackAdapter) TransformOutbound(message InternalMessage) (json.RawMessage, error) {
	if err := ValidateMessage(message); err != nil {
		return nil, err
	}

	payload := slackOutboundPayload{
		Channel: message.ConversationID,
		Text:    FirstTextPart(message.Content),
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode slack payload: %w", err)
	}
	return raw, nil
}

func (a *SlackAdapter) TestConnection(ctx context.Context, config map[string]any) error {
	webhookURL := extractURL(config, "webhook_url", "webhookURL", "url")
	if webhookURL == "" {
		return fmt.Errorf("slack webhook_url is required")
	}
	return probeURL(ctx, a.client, webhookURL)
}

type slackInboundPayload struct {
	EventID string     `json:"event_id,omitempty"`
	Event   slackEvent `json:"event,omitempty"`
	Files   []slackFile `json:"files,omitempty"`
}

func (p slackInboundPayload) eventFiles() []slackFile {
	if len(p.Event.Files) > 0 {
		return p.Event.Files
	}
	return p.Files
}

type slackEvent struct {
	BotID       string      `json:"bot_id,omitempty"`
	Channel     string      `json:"channel,omitempty"`
	ClientMsgID string      `json:"client_msg_id,omitempty"`
	Files       []slackFile `json:"files,omitempty"`
	Text        string      `json:"text,omitempty"`
	TS          string      `json:"ts,omitempty"`
	User        string      `json:"user,omitempty"`
}

type slackFile struct {
	ID                 string `json:"id,omitempty"`
	Name               string `json:"name,omitempty"`
	Mimetype           string `json:"mimetype,omitempty"`
	Size               int64  `json:"size,omitempty"`
	URLPrivate         string `json:"url_private,omitempty"`
	URLPrivateDownload string `json:"url_private_download,omitempty"`
	Permalink          string `json:"permalink,omitempty"`
}

type slackOutboundPayload struct {
	Channel string `json:"channel"`
	Text    string `json:"text"`
}
