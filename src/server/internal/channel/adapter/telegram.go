package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// TelegramAdapter implements ChannelAdapter for Telegram bot messages.
type TelegramAdapter struct {
	client *http.Client
}

// NewTelegramAdapter creates a new Telegram adapter with default settings.
func NewTelegramAdapter() *TelegramAdapter {
	return &TelegramAdapter{client: &http.Client{Timeout: 10 * time.Second}}
}

func (a *TelegramAdapter) Type() string {
	return "telegram"
}

func (a *TelegramAdapter) TransformInbound(raw json.RawMessage) (InternalMessage, error) {
	var payload telegramInboundPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return InternalMessage{}, fmt.Errorf("decode telegram payload: %w", err)
	}

	content := []ContentPart{}
	if payload.Message.Text != "" {
		content = append(content, ContentPart{Type: "text", Text: payload.Message.Text})
	}
	if payload.Message.Caption != "" {
		content = append(content, ContentPart{Type: "text", Text: payload.Message.Caption})
	}
	if payload.Message.Document.FileID != "" {
		content = append(content, ContentPart{
			Type: "file",
			URL:  "telegram://file/" + payload.Message.Document.FileID,
			Metadata: map[string]any{
				"file_id":        payload.Message.Document.FileID,
				"file_unique_id": payload.Message.Document.FileUniqueID,
				"name":           payload.Message.Document.FileName,
				"mimetype":       payload.Message.Document.MimeType,
				"size":           payload.Message.Document.FileSize,
			},
		})
	}

	role := "user"
	if payload.Message.From.IsBot {
		role = "assistant"
	}
	message := InternalMessage{
		ID:             fmt.Sprint(payload.Message.MessageID),
		ConversationID: fmt.Sprint(payload.Message.Chat.ID),
		Role:           role,
		Content:        content,
		Metadata: map[string]any{
			"adapter": "telegram",
			"from_id": fmt.Sprint(payload.Message.From.ID),
		},
		Timestamp: time.Now().UTC(),
	}
	if err := ValidateMessage(message); err != nil {
		return InternalMessage{}, err
	}
	return message, nil
}

func (a *TelegramAdapter) TransformOutbound(message InternalMessage) (json.RawMessage, error) {
	if err := ValidateMessage(message); err != nil {
		return nil, err
	}

	raw, err := json.Marshal(telegramOutboundPayload{
		ChatID: message.ConversationID,
		Text:   FirstTextPart(message.Content),
	})
	if err != nil {
		return nil, fmt.Errorf("encode telegram payload: %w", err)
	}
	return raw, nil
}

func (a *TelegramAdapter) TestConnection(ctx context.Context, config map[string]any) error {
	webhookURL := extractURL(config, "webhook_url", "webhookURL", "url")
	if webhookURL == "" {
		return fmt.Errorf("telegram webhook_url is required")
	}
	return probeURL(ctx, a.client, webhookURL)
}

type telegramInboundPayload struct {
	Message telegramMessage `json:"message,omitempty"`
}

type telegramMessage struct {
	MessageID int64            `json:"message_id,omitempty"`
	Chat      telegramChat     `json:"chat,omitempty"`
	From      telegramSender   `json:"from,omitempty"`
	Text      string           `json:"text,omitempty"`
	Caption   string           `json:"caption,omitempty"`
	Document  telegramDocument `json:"document,omitempty"`
}

type telegramChat struct {
	ID int64 `json:"id,omitempty"`
}

type telegramSender struct {
	ID    int64 `json:"id,omitempty"`
	IsBot bool  `json:"is_bot,omitempty"`
}

type telegramDocument struct {
	FileID       string `json:"file_id,omitempty"`
	FileUniqueID string `json:"file_unique_id,omitempty"`
	FileName     string `json:"file_name,omitempty"`
	MimeType     string `json:"mime_type,omitempty"`
	FileSize     int64  `json:"file_size,omitempty"`
}

type telegramOutboundPayload struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}
