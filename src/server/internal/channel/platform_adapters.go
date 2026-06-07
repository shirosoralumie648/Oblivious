package channel

import (
	"encoding/json"
	"fmt"
	"time"
)

type APIAdapter struct{}

func NewAPIAdapter() *APIAdapter {
	return &APIAdapter{}
}

func (a *APIAdapter) Type() ChannelType {
	return ChannelTypeAPI
}

func (a *APIAdapter) TransformInbound(raw json.RawMessage) (InternalMessage, error) {
	var payload apiPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return InternalMessage{}, fmt.Errorf("decode api payload: %w", err)
	}

	content := append([]ContentPart(nil), payload.Content...)
	if len(content) == 0 && payload.Text != "" {
		content = append(content, ContentPart{Type: ContentTypeText, Text: payload.Text})
	}

	metadata := map[string]any{}
	for key, value := range payload.Metadata {
		metadata[key] = value
	}
	metadata["adapter"] = string(ChannelTypeAPI)
	metadata["raw"] = rawMetadata(raw)

	role := payload.Role
	if role == "" {
		role = RoleUser
	}
	message := InternalMessage{
		ID:             payload.ID,
		ConversationID: payload.ConversationID,
		Role:           role,
		Content:        content,
		Metadata:       metadata,
		Timestamp:      time.Now().UTC(),
	}
	if err := validateInternalMessage(message); err != nil {
		return InternalMessage{}, err
	}
	return message, nil
}

func (a *APIAdapter) TransformOutbound(message InternalMessage) (json.RawMessage, error) {
	if err := validateInternalMessage(message); err != nil {
		return nil, err
	}

	raw, err := json.Marshal(apiPayload{
		APIEvent:       "message",
		ID:             message.ID,
		ConversationID: message.ConversationID,
		Role:           message.Role,
		Text:           firstTextPart(message.Content),
		Content:        message.Content,
		Metadata:       message.Metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("encode api payload: %w", err)
	}
	return raw, nil
}

type FeishuAdapter struct{}

func NewFeishuAdapter() *FeishuAdapter {
	return &FeishuAdapter{}
}

func (a *FeishuAdapter) Type() ChannelType {
	return ChannelTypeFeishu
}

func (a *FeishuAdapter) TransformInbound(raw json.RawMessage) (InternalMessage, error) {
	var payload feishuPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return InternalMessage{}, fmt.Errorf("decode feishu payload: %w", err)
	}

	content := []ContentPart{}
	if payload.Text != "" {
		content = append(content, ContentPart{Type: ContentTypeText, Text: payload.Text})
	}
	if payload.Card != nil {
		content = append(content, ContentPart{Type: ContentTypeCard, Metadata: map[string]any{"card": payload.Card}})
	}
	if len(content) == 0 && payload.Content.Text != "" {
		content = append(content, ContentPart{Type: ContentTypeText, Text: payload.Content.Text})
	}

	message := InternalMessage{
		ID:             firstNonEmpty(payload.MessageID, payload.ID),
		ConversationID: firstNonEmpty(payload.ChatID, payload.OpenChatID, payload.ConversationID),
		Role:           roleFromBot(payload.Sender.Bot),
		Content:        content,
		Metadata: map[string]any{
			"adapter":      string(ChannelTypeFeishu),
			"message_type": payload.MessageType,
			"sender_id":    payload.Sender.ID,
			"raw":          rawMetadata(raw),
		},
		Timestamp: time.Now().UTC(),
	}
	if message.Role == "" {
		message.Role = RoleUser
	}
	if err := validateInternalMessage(message); err != nil {
		return InternalMessage{}, err
	}
	return message, nil
}

func (a *FeishuAdapter) TransformOutbound(message InternalMessage) (json.RawMessage, error) {
	if err := validateInternalMessage(message); err != nil {
		return nil, err
	}

	text := firstTextPart(message.Content)
	card := firstCardMetadata(message.Content)
	payload := feishuPayload{
		ChatID: message.ConversationID,
	}
	if card != nil {
		payload.MessageType = "interactive"
		payload.Card = card
	} else {
		payload.MessageType = "text"
		payload.Content.Text = text
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode feishu payload: %w", err)
	}
	return raw, nil
}

type WeChatAdapter struct{}

func NewWeChatAdapter() *WeChatAdapter {
	return &WeChatAdapter{}
}

func (a *WeChatAdapter) Type() ChannelType {
	return ChannelTypeWeChat
}

func (a *WeChatAdapter) TransformInbound(raw json.RawMessage) (InternalMessage, error) {
	var payload wechatPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return InternalMessage{}, fmt.Errorf("decode wechat payload: %w", err)
	}

	content := []ContentPart{}
	switch payload.MsgType {
	case "image":
		content = append(content, ContentPart{Type: ContentTypeImage, URL: payload.PicURL, Metadata: map[string]any{"media_id": payload.MediaID}})
	case "link":
		content = append(content, ContentPart{Type: ContentTypeCard, Text: payload.Title, URL: payload.URL, Metadata: map[string]any{"description": payload.Description}})
	default:
		content = append(content, ContentPart{Type: ContentTypeText, Text: payload.Content})
	}

	message := InternalMessage{
		ID:             firstNonEmpty(payload.MsgID, payload.ID),
		ConversationID: firstNonEmpty(payload.FromUserName, payload.ConversationID, payload.ToUserName),
		Role:           RoleUser,
		Content:        content,
		Metadata: map[string]any{
			"adapter":  string(ChannelTypeWeChat),
			"msg_type": payload.MsgType,
			"raw":      rawMetadata(raw),
		},
		Timestamp: time.Now().UTC(),
	}
	if err := validateInternalMessage(message); err != nil {
		return InternalMessage{}, err
	}
	return message, nil
}

func (a *WeChatAdapter) TransformOutbound(message InternalMessage) (json.RawMessage, error) {
	if err := validateInternalMessage(message); err != nil {
		return nil, err
	}

	if card, ok := firstCardPart(message.Content); ok && card.URL != "" {
		payload := wechatOutboundPayload{
			ToUser:  message.ConversationID,
			MsgType: "news",
			News: &wechatNewsPayload{
				Articles: []wechatNewsArticle{{
					Title:       firstNonEmpty(card.Text, stringMetadata(card.Metadata, "title")),
					Description: stringMetadata(card.Metadata, "description"),
					URL:         card.URL,
					PicURL:      firstNonEmpty(stringMetadata(card.Metadata, "picurl"), stringMetadata(card.Metadata, "pic_url")),
				}},
			},
		}
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode wechat payload: %w", err)
		}
		return raw, nil
	}

	if image, ok := firstContentPart(message.Content, ContentTypeImage); ok {
		if mediaID := stringMetadata(image.Metadata, "media_id"); mediaID != "" {
			payload := wechatOutboundPayload{
				ToUser:  message.ConversationID,
				MsgType: "image",
				Image: map[string]string{
					"media_id": mediaID,
				},
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
		Text: map[string]string{
			"content": firstTextPart(message.Content),
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode wechat payload: %w", err)
	}
	return raw, nil
}

type DiscordAdapter struct{}

func NewDiscordAdapter() *DiscordAdapter {
	return &DiscordAdapter{}
}

func (a *DiscordAdapter) Type() ChannelType {
	return ChannelTypeDiscord
}

func (a *DiscordAdapter) TransformInbound(raw json.RawMessage) (InternalMessage, error) {
	var payload discordPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return InternalMessage{}, fmt.Errorf("decode discord payload: %w", err)
	}

	content := []ContentPart{}
	if payload.Content != "" {
		content = append(content, ContentPart{Type: ContentTypeText, Text: payload.Content})
	}
	for _, embed := range payload.Embeds {
		content = append(content, ContentPart{Type: ContentTypeCard, Metadata: map[string]any{"embed": embed}})
	}
	for _, attachment := range payload.Attachments {
		content = append(content, ContentPart{
			Type: ContentTypeFile,
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

	message := InternalMessage{
		ID:             payload.ID,
		ConversationID: firstNonEmpty(payload.ChannelID, payload.GuildID),
		Role:           roleFromBot(payload.Author.Bot),
		Content:        content,
		Metadata: map[string]any{
			"adapter":   string(ChannelTypeDiscord),
			"author_id": payload.Author.ID,
			"raw":       rawMetadata(raw),
		},
		Timestamp: time.Now().UTC(),
	}
	if len(payload.Reactions) > 0 {
		message.Metadata["reactions"] = payload.Reactions
	}
	if message.Role == "" {
		message.Role = RoleUser
	}
	if err := validateInternalMessage(message); err != nil {
		return InternalMessage{}, err
	}
	return message, nil
}

func (a *DiscordAdapter) TransformOutbound(message InternalMessage) (json.RawMessage, error) {
	if err := validateInternalMessage(message); err != nil {
		return nil, err
	}

	payload := discordPayload{
		ChannelID: message.ConversationID,
		Content:   firstTextPart(message.Content),
	}
	for _, part := range message.Content {
		if part.Type == ContentTypeCard && len(part.Metadata) > 0 {
			payload.Embeds = append(payload.Embeds, part.Metadata)
		}
	}
	payload.Reactions = discordReactionsFromMetadata(message.Metadata)

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode discord payload: %w", err)
	}
	return raw, nil
}

type SlackAdapter struct{}

func NewSlackAdapter() *SlackAdapter {
	return &SlackAdapter{}
}

func (a *SlackAdapter) Type() ChannelType {
	return ChannelTypeSlack
}

func (a *SlackAdapter) TransformInbound(raw json.RawMessage) (InternalMessage, error) {
	var payload slackPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return InternalMessage{}, fmt.Errorf("decode slack payload: %w", err)
	}

	content := []ContentPart{}
	if payload.Event.Text != "" {
		content = append(content, ContentPart{Type: ContentTypeText, Text: payload.Event.Text})
	}
	for _, file := range payload.slackFiles() {
		content = append(content, ContentPart{
			Type: ContentTypeFile,
			URL:  firstNonEmpty(file.URLPrivate, file.URLPrivateDownload, file.Permalink),
			Metadata: map[string]any{
				"file_id":  file.ID,
				"name":     file.Name,
				"mimetype": file.Mimetype,
				"size":     file.Size,
			},
		})
	}

	message := InternalMessage{
		ID:             firstNonEmpty(payload.Event.ClientMsgID, payload.EventID, payload.Event.TS),
		ConversationID: payload.Event.Channel,
		Role:           RoleUser,
		Content:        content,
		Metadata: map[string]any{
			"adapter": string(ChannelTypeSlack),
			"user_id": payload.Event.User,
			"raw":     rawMetadata(raw),
		},
		Timestamp: time.Now().UTC(),
	}
	if payload.Event.BotID != "" {
		message.Role = RoleAssistant
	}
	if err := validateInternalMessage(message); err != nil {
		return InternalMessage{}, err
	}
	return message, nil
}

func (a *SlackAdapter) TransformOutbound(message InternalMessage) (json.RawMessage, error) {
	if err := validateInternalMessage(message); err != nil {
		return nil, err
	}

	raw, err := json.Marshal(slackOutboundPayload{
		Channel: message.ConversationID,
		Text:    firstTextPart(message.Content),
	})
	if err != nil {
		return nil, fmt.Errorf("encode slack payload: %w", err)
	}
	return raw, nil
}

type TelegramAdapter struct{}

func NewTelegramAdapter() *TelegramAdapter {
	return &TelegramAdapter{}
}

func (a *TelegramAdapter) Type() ChannelType {
	return ChannelTypeTelegram
}

func (a *TelegramAdapter) TransformInbound(raw json.RawMessage) (InternalMessage, error) {
	var payload telegramPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return InternalMessage{}, fmt.Errorf("decode telegram payload: %w", err)
	}

	content := []ContentPart{}
	if payload.Message.Text != "" {
		content = append(content, ContentPart{Type: ContentTypeText, Text: payload.Message.Text})
	}
	if payload.Message.Caption != "" {
		content = append(content, ContentPart{Type: ContentTypeText, Text: payload.Message.Caption})
	}
	if payload.Message.Document.FileID != "" {
		content = append(content, ContentPart{
			Type: ContentTypeFile,
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

	message := InternalMessage{
		ID:             fmt.Sprint(payload.Message.MessageID),
		ConversationID: fmt.Sprint(payload.Message.Chat.ID),
		Role:           roleFromBot(payload.Message.From.IsBot),
		Content:        content,
		Metadata: map[string]any{
			"adapter": string(ChannelTypeTelegram),
			"from_id": fmt.Sprint(payload.Message.From.ID),
			"raw":     rawMetadata(raw),
		},
		Timestamp: time.Now().UTC(),
	}
	if err := validateInternalMessage(message); err != nil {
		return InternalMessage{}, err
	}
	return message, nil
}

func (a *TelegramAdapter) TransformOutbound(message InternalMessage) (json.RawMessage, error) {
	if err := validateInternalMessage(message); err != nil {
		return nil, err
	}

	raw, err := json.Marshal(telegramOutboundPayload{
		ChatID: message.ConversationID,
		Text:   firstTextPart(message.Content),
	})
	if err != nil {
		return nil, fmt.Errorf("encode telegram payload: %w", err)
	}
	return raw, nil
}

type WebEmbedAdapter struct{}

func NewWebEmbedAdapter() *WebEmbedAdapter {
	return &WebEmbedAdapter{}
}

func (a *WebEmbedAdapter) Type() ChannelType {
	return ChannelTypeWebEmbed
}

func (a *WebEmbedAdapter) TransformInbound(raw json.RawMessage) (InternalMessage, error) {
	var payload webEmbedPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return InternalMessage{}, fmt.Errorf("decode web embed payload: %w", err)
	}

	content := append([]ContentPart(nil), payload.Content...)
	if len(content) == 0 && payload.Text != "" {
		content = append(content, ContentPart{Type: ContentTypeText, Text: payload.Text})
	}
	metadata := map[string]any{}
	for key, value := range payload.Metadata {
		metadata[key] = value
	}
	metadata["adapter"] = string(ChannelTypeWebEmbed)
	metadata["raw"] = rawMetadata(raw)
	if payload.EmbedOrigin != "" {
		metadata["embed_origin"] = payload.EmbedOrigin
	}

	role := payload.Role
	if role == "" {
		role = RoleUser
	}
	message := InternalMessage{
		ID:             payload.ID,
		ConversationID: payload.ConversationID,
		Role:           role,
		Content:        content,
		Metadata:       metadata,
		Timestamp:      time.Now().UTC(),
	}
	if err := validateInternalMessage(message); err != nil {
		return InternalMessage{}, err
	}
	return message, nil
}

func (a *WebEmbedAdapter) TransformOutbound(message InternalMessage) (json.RawMessage, error) {
	if err := validateInternalMessage(message); err != nil {
		return nil, err
	}

	raw, err := json.Marshal(webEmbedPayload{
		SDKEvent:       "message",
		ID:             message.ID,
		ConversationID: message.ConversationID,
		Role:           message.Role,
		Text:           firstTextPart(message.Content),
		Content:        message.Content,
		Metadata:       message.Metadata,
	})
	if err != nil {
		return nil, fmt.Errorf("encode web embed payload: %w", err)
	}
	return raw, nil
}

type feishuPayload struct {
	ID             string         `json:"id,omitempty"`
	MessageID      string         `json:"message_id,omitempty"`
	ConversationID string         `json:"conversation_id,omitempty"`
	ChatID         string         `json:"chat_id,omitempty"`
	OpenChatID     string         `json:"open_chat_id,omitempty"`
	MessageType    string         `json:"message_type,omitempty"`
	Text           string         `json:"text,omitempty"`
	Content        feishuContent  `json:"content,omitempty"`
	Card           map[string]any `json:"card,omitempty"`
	Sender         platformSender `json:"sender,omitempty"`
}

type feishuContent struct {
	Text string `json:"text,omitempty"`
}

func (p feishuPayload) MarshalJSON() ([]byte, error) {
	type outbound struct {
		ChatID  string         `json:"chat_id,omitempty"`
		MsgType string         `json:"msg_type,omitempty"`
		Content feishuContent  `json:"content,omitempty"`
		Card    map[string]any `json:"card,omitempty"`
	}
	return json.Marshal(outbound{
		ChatID:  p.ChatID,
		MsgType: p.MessageType,
		Content: p.Content,
		Card:    p.Card,
	})
}

type wechatPayload struct {
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

type discordPayload struct {
	ID          string              `json:"id,omitempty"`
	ChannelID   string              `json:"channel_id,omitempty"`
	GuildID     string              `json:"guild_id,omitempty"`
	Content     string              `json:"content,omitempty"`
	Embeds      []map[string]any    `json:"embeds,omitempty"`
	Attachments []discordAttachment `json:"attachments,omitempty"`
	Reactions   []map[string]any    `json:"reactions,omitempty"`
	Author      platformSender      `json:"author,omitempty"`
}

type discordAttachment struct {
	ID          string `json:"id,omitempty"`
	Filename    string `json:"filename,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	Size        int64  `json:"size,omitempty"`
	URL         string `json:"url,omitempty"`
	ProxyURL    string `json:"proxy_url,omitempty"`
}

type slackPayload struct {
	EventID string      `json:"event_id,omitempty"`
	Event   slackEvent  `json:"event,omitempty"`
	Files   []slackFile `json:"files,omitempty"`
}

func (p slackPayload) slackFiles() []slackFile {
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

type telegramPayload struct {
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

type webEmbedPayload struct {
	SDKEvent       string         `json:"sdk_event,omitempty"`
	ID             string         `json:"id,omitempty"`
	ConversationID string         `json:"conversation_id"`
	Role           Role           `json:"role"`
	Text           string         `json:"text,omitempty"`
	Content        []ContentPart  `json:"content,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	EmbedOrigin    string         `json:"embed_origin,omitempty"`
}

type apiPayload struct {
	APIEvent       string         `json:"api_event,omitempty"`
	ID             string         `json:"id,omitempty"`
	ConversationID string         `json:"conversation_id"`
	Role           Role           `json:"role"`
	Text           string         `json:"text,omitempty"`
	Content        []ContentPart  `json:"content,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

type platformSender struct {
	ID  string `json:"id,omitempty"`
	Bot bool   `json:"bot,omitempty"`
}

func roleFromBot(bot bool) Role {
	if bot {
		return RoleAssistant
	}
	return RoleUser
}

func firstTextPart(content []ContentPart) string {
	for _, part := range content {
		if part.Type == ContentTypeText && part.Text != "" {
			return part.Text
		}
	}
	return ""
}

func firstCardMetadata(content []ContentPart) map[string]any {
	for _, part := range content {
		if part.Type == ContentTypeCard && len(part.Metadata) > 0 {
			return part.Metadata
		}
	}
	return nil
}

func firstCardPart(content []ContentPart) (ContentPart, bool) {
	for _, part := range content {
		if part.Type == ContentTypeCard {
			return part, true
		}
	}
	return ContentPart{}, false
}

func firstContentPart(content []ContentPart, contentType ContentType) (ContentPart, bool) {
	for _, part := range content {
		if part.Type == contentType {
			return part, true
		}
	}
	return ContentPart{}, false
}

func stringMetadata(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key].(string)
	if !ok {
		return ""
	}
	return value
}

func discordReactionsFromMetadata(metadata map[string]any) []map[string]any {
	if metadata == nil {
		return nil
	}
	switch reactions := metadata["reactions"].(type) {
	case []map[string]any:
		return reactions
	case []any:
		normalized := make([]map[string]any, 0, len(reactions))
		for _, reaction := range reactions {
			if reactionMap, ok := reaction.(map[string]any); ok {
				normalized = append(normalized, reactionMap)
			}
		}
		if len(normalized) > 0 {
			return normalized
		}
	}
	if emoji, ok := metadata["emoji"].(string); ok && emoji != "" {
		return []map[string]any{{"emoji": map[string]any{"name": emoji}}}
	}
	if emoji, ok := metadata["emoji"].(map[string]any); ok && len(emoji) > 0 {
		return []map[string]any{{"emoji": emoji}}
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func rawMetadata(raw json.RawMessage) map[string]any {
	var metadata map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		return map[string]any{"raw": string(raw)}
	}
	return metadata
}
