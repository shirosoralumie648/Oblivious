package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// FeiShuAdapter implements ChannelAdapter for the Feishu (Lark) platform.
type FeiShuAdapter struct {
	client *http.Client
}

// NewFeiShuAdapter creates a new Feishu adapter with default settings.
func NewFeiShuAdapter() *FeiShuAdapter {
	return &FeiShuAdapter{client: &http.Client{Timeout: 10 * time.Second}}
}

func (a *FeiShuAdapter) Type() string {
	return "feishu"
}

func (a *FeiShuAdapter) TransformInbound(raw json.RawMessage) (InternalMessage, error) {
	var payload feishuInboundPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return InternalMessage{}, fmt.Errorf("decode feishu payload: %w", err)
	}

	content := []ContentPart{}
	if payload.Text != "" {
		content = append(content, ContentPart{Type: "text", Text: payload.Text})
	}
	if payload.Card != nil {
		content = append(content, ContentPart{Type: "card", Metadata: map[string]any{"card": payload.Card}})
	}
	if len(content) == 0 && payload.Content.Text != "" {
		content = append(content, ContentPart{Type: "text", Text: payload.Content.Text})
	}

	role := "user"
	if payload.Sender.Bot {
		role = "assistant"
	}

	message := InternalMessage{
		ID:             FirstNonEmpty(payload.MessageID, payload.ID),
		ConversationID: FirstNonEmpty(payload.ChatID, payload.OpenChatID, payload.ConversationID),
		Role:           role,
		Content:        content,
		Metadata: map[string]any{
			"adapter":      "feishu",
			"message_type": payload.MessageType,
			"sender_id":    payload.Sender.ID,
		},
		Timestamp: time.Now().UTC(),
	}
	if err := ValidateMessage(message); err != nil {
		return InternalMessage{}, err
	}
	return message, nil
}

func (a *FeiShuAdapter) TransformOutbound(message InternalMessage) (json.RawMessage, error) {
	if err := ValidateMessage(message); err != nil {
		return nil, err
	}

	payload := feishuOutboundPayload{
		ChatID: message.ConversationID,
	}
	if card := FirstCardMetadata(message.Content); card != nil {
		payload.MsgType = "interactive"
		payload.Card = card
	} else {
		payload.MsgType = "text"
		payload.Content = &feishuContent{Text: FirstTextPart(message.Content)}
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode feishu payload: %w", err)
	}
	return raw, nil
}

func (a *FeiShuAdapter) TestConnection(ctx context.Context, config map[string]any) error {
	webhookURL := extractURL(config, "webhook_url", "webhookURL", "url")
	if webhookURL == "" {
		return fmt.Errorf("feishu webhook_url is required")
	}
	return probeURL(ctx, a.client, webhookURL)
}

type feishuInboundPayload struct {
	ID             string         `json:"id,omitempty"`
	MessageID      string         `json:"message_id,omitempty"`
	ConversationID string         `json:"conversation_id,omitempty"`
	ChatID         string         `json:"chat_id,omitempty"`
	OpenChatID     string         `json:"open_chat_id,omitempty"`
	MessageType    string         `json:"message_type,omitempty"`
	Text           string         `json:"text,omitempty"`
	Content        feishuContent  `json:"content,omitempty"`
	Card           map[string]any `json:"card,omitempty"`
	Sender         feishuSender   `json:"sender,omitempty"`
}

type feishuContent struct {
	Text string `json:"text,omitempty"`
}

type feishuSender struct {
	ID  string `json:"id,omitempty"`
	Bot bool   `json:"bot,omitempty"`
}

type feishuOutboundPayload struct {
	ChatID  string         `json:"chat_id,omitempty"`
	MsgType string         `json:"msg_type,omitempty"`
	Content *feishuContent `json:"content,omitempty"`
	Card    map[string]any `json:"card,omitempty"`
}

func extractURL(config map[string]any, keys ...string) string {
	if config == nil {
		return ""
	}
	for _, key := range keys {
		if value, ok := config[key].(string); ok {
			if trimmed := value; trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func probeURL(ctx context.Context, client *http.Client, url string) error {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return fmt.Errorf("invalid url: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("upstream returned status %d", resp.StatusCode)
	}
	return nil
}
