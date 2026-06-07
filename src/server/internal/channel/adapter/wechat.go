package adapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// WeChatAdapter implements ChannelAdapter for the WeChat Official Account platform.
type WeChatAdapter struct {
	client *http.Client
}

// NewWeChatAdapter creates a new WeChat adapter with default settings.
func NewWeChatAdapter() *WeChatAdapter {
	return &WeChatAdapter{client: &http.Client{Timeout: 10 * time.Second}}
}

func (a *WeChatAdapter) Type() string {
	return "wechat"
}

func (a *WeChatAdapter) TransformInbound(raw json.RawMessage) (InternalMessage, error) {
	var payload wechatInboundPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return InternalMessage{}, fmt.Errorf("decode wechat payload: %w", err)
	}

	content := []ContentPart{}
	switch payload.MsgType {
	case "image":
		content = append(content, ContentPart{
			Type:     "image",
			URL:      payload.PicURL,
			Metadata: map[string]any{"media_id": payload.MediaID},
		})
	case "link":
		content = append(content, ContentPart{
			Type: "card",
			Text: payload.Title,
			URL:  payload.URL,
			Metadata: map[string]any{
				"description": payload.Description,
			},
		})
	default:
		content = append(content, ContentPart{Type: "text", Text: payload.Content})
	}

	message := InternalMessage{
		ID:             FirstNonEmpty(payload.MsgID, payload.ID),
		ConversationID: FirstNonEmpty(payload.FromUserName, payload.ConversationID, payload.ToUserName),
		Role:           "user",
		Content:        content,
		Metadata: map[string]any{
			"adapter":  "wechat",
			"msg_type": payload.MsgType,
		},
		Timestamp: time.Now().UTC(),
	}
	if err := ValidateMessage(message); err != nil {
		return InternalMessage{}, err
	}
	return message, nil
}

func (a *WeChatAdapter) TransformOutbound(message InternalMessage) (json.RawMessage, error) {
	if err := ValidateMessage(message); err != nil {
		return nil, err
	}

	text := FirstTextPart(message.Content)
	payload := wechatOutboundPayload{
		ToUser:  message.ConversationID,
		MsgType: "text",
		Text:    map[string]string{"content": text},
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode wechat payload: %w", err)
	}
	return raw, nil
}

func (a *WeChatAdapter) TestConnection(ctx context.Context, config map[string]any) error {
	webhookURL := extractURL(config, "webhook_url", "webhookURL", "url")
	if webhookURL == "" {
		return fmt.Errorf("wechat webhook_url is required")
	}
	return probeURL(ctx, a.client, webhookURL)
}

type wechatInboundPayload struct {
	ID             string `json:"id,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	MsgID          string `json:"MsgId,omitempty"`
	FromUserName   string `json:"FromUserName,omitempty"`
	ToUserName     string `json:"ToUserName,omitempty"`
	MsgType        string `json:"MsgType,omitempty"`
	Content        string `json:"Content,omitempty"`
	PicURL         string `json:"PicUrl,omitempty"`
	MediaID        string `json:"MediaId,omitempty"`
	Title          string `json:"Title,omitempty"`
	Description    string `json:"Description,omitempty"`
	URL            string `json:"Url,omitempty"`
}

type wechatOutboundPayload struct {
	ToUser  string            `json:"touser,omitempty"`
	MsgType string            `json:"msgtype"`
	Text    map[string]string `json:"text,omitempty"`
}
