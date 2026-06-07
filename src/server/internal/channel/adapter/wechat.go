package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
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
	rawFormat := "json"
	if IsXMLPayload(raw) {
		if err := xml.Unmarshal(raw, &payload); err != nil {
			return InternalMessage{}, fmt.Errorf("decode wechat xml payload: %w", err)
		}
		rawFormat = "xml"
	} else if err := json.Unmarshal(raw, &payload); err != nil {
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
			"adapter":    "wechat",
			"msg_type":   payload.MsgType,
			"raw_format": rawFormat,
		},
		Timestamp: time.Now().UTC(),
	}
	if payload.ToUserName != "" {
		message.Metadata["to_user_name"] = payload.ToUserName
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
	if StringMetadata(message.Metadata, "wechat_format") == "xml" {
		raw, err := xml.Marshal(wechatXMLPayload{
			ToUserName:   message.ConversationID,
			FromUserName: StringMetadata(message.Metadata, "from_user_name"),
			CreateTime:   IntMetadata(message.Metadata, "create_time"),
			MsgType:      "text",
			Content:      FirstTextPart(message.Content),
		})
		if err != nil {
			return nil, fmt.Errorf("encode wechat xml payload: %w", err)
		}
		return raw, nil
	}

	if card, ok := FirstCardPart(message.Content); ok && card.URL != "" {
		payload := wechatOutboundPayload{
			ToUser:  message.ConversationID,
			MsgType: "news",
			News: &wechatNewsPayload{
				Articles: []wechatNewsArticle{{
					Title:       FirstNonEmpty(card.Text, StringMetadata(card.Metadata, "title")),
					Description: StringMetadata(card.Metadata, "description"),
					URL:         card.URL,
					PicURL:      FirstNonEmpty(StringMetadata(card.Metadata, "picurl"), StringMetadata(card.Metadata, "pic_url")),
				}},
			},
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode wechat payload: %w", err)
		}
		return raw, nil
	}

	if image, ok := FirstContentPart(message.Content, "image"); ok {
		if mediaID := StringMetadata(image.Metadata, "media_id"); mediaID != "" {
			payload := wechatOutboundPayload{
				ToUser:  message.ConversationID,
				MsgType: "image",
				Image:   map[string]string{"media_id": mediaID},
			}
			raw, err := json.Marshal(payload)
			if err != nil {
				return nil, fmt.Errorf("encode wechat payload: %w", err)
			}
			return raw, nil
		}
	}

	payload := wechatOutboundPayload{
		ToUser:  message.ConversationID,
		MsgType: "text",
		Text:    map[string]string{"content": FirstTextPart(message.Content)},
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
	XMLName        xml.Name `json:"-" xml:"xml"`
	ID             string   `json:"id,omitempty"`
	ConversationID string   `json:"conversation_id,omitempty"`
	MsgID          string   `json:"MsgId,omitempty" xml:"MsgId"`
	FromUserName   string   `json:"FromUserName,omitempty" xml:"FromUserName"`
	ToUserName     string   `json:"ToUserName,omitempty" xml:"ToUserName"`
	MsgType        string   `json:"MsgType,omitempty" xml:"MsgType"`
	Content        string   `json:"Content,omitempty" xml:"Content"`
	PicURL         string   `json:"PicUrl,omitempty" xml:"PicUrl"`
	MediaID        string   `json:"MediaId,omitempty" xml:"MediaId"`
	Title          string   `json:"Title,omitempty" xml:"Title"`
	Description    string   `json:"Description,omitempty" xml:"Description"`
	URL            string   `json:"Url,omitempty" xml:"Url"`
}

type wechatXMLPayload struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content,omitempty"`
}

type wechatOutboundPayload struct {
	ToUser  string             `json:"touser,omitempty"`
	MsgType string             `json:"msgtype"`
	Text    map[string]string  `json:"text,omitempty"`
	News    *wechatNewsPayload `json:"news,omitempty"`
	Image   map[string]string  `json:"image,omitempty"`
}

type wechatNewsPayload struct {
	Articles []wechatNewsArticle `json:"articles"`
}

type wechatNewsArticle struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url"`
	PicURL      string `json:"picurl,omitempty"`
}

// IntMetadata extracts an integer value from a metadata map.
func IntMetadata(metadata map[string]any, key string) int64 {
	if metadata == nil {
		return 0
	}
	switch value := metadata[key].(type) {
	case int64:
		return value
	case int:
		return int64(value)
	case float64:
		return int64(value)
	default:
		return 0
	}
}

// IsXMLPayload reports whether a raw payload appears to be XML.
func IsXMLPayload(raw json.RawMessage) bool {
	return bytes.HasPrefix(bytes.TrimSpace(raw), []byte("<"))
}
