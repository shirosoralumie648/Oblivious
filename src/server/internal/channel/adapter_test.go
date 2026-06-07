package channel

import (
	"encoding/json"
	"testing"
	"time"
)

func TestWebhookAdapterTransformOutboundSerializesRoleAndTextContent(t *testing.T) {
	adapter := NewWebhookAdapter()
	message := InternalMessage{
		ID:             "msg_2",
		ConversationID: "conversation_1",
		Role:           RoleAssistant,
		Content: []ContentPart{
			{Type: ContentTypeText, Text: "hello back"},
		},
		Timestamp: time.Date(2026, 6, 4, 12, 5, 0, 0, time.UTC),
	}

	raw, err := adapter.TransformOutbound(message)
	if err != nil {
		t.Fatalf("TransformOutbound returned error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("outbound payload is not JSON: %v", err)
	}
	if payload["role"] != "assistant" {
		t.Fatalf("expected assistant role, got %#v", payload["role"])
	}
	if payload["text"] != "hello back" {
		t.Fatalf("expected text content, got %#v", payload["text"])
	}
	if payload["conversation_id"] != "conversation_1" {
		t.Fatalf("expected conversation_id, got %#v", payload["conversation_id"])
	}
}

func TestAdapterRegistryRegistersPublishingAdaptersByDefault(t *testing.T) {
	registry := NewAdapterRegistry(nil)

	for _, channelType := range []ChannelType{
		ChannelType("api"),
		ChannelType("feishu"),
		ChannelType("wechat"),
		ChannelType("discord"),
		ChannelType("slack"),
		ChannelType("telegram"),
		ChannelType("web_embed"),
	} {
		adapter, err := registry.Adapter(channelType)
		if err != nil {
			t.Fatalf("expected default adapter for %q: %v", channelType, err)
		}
		if adapter.Type() != channelType {
			t.Fatalf("expected adapter type %q, got %q", channelType, adapter.Type())
		}
	}
}

func TestAPIAdapterTransformsRESTMessageInboundAndOutbound(t *testing.T) {
	adapter, err := NewAdapterRegistry(nil).Adapter(ChannelType("api"))
	if err != nil {
		t.Fatalf("expected api adapter: %v", err)
	}

	message, err := adapter.TransformInbound(json.RawMessage(`{
		"id": "api_msg_1",
		"conversation_id": "conversation_api",
		"role": "user",
		"text": "hello through rest",
		"metadata": {"request_id": "req_1"}
	}`))
	if err != nil {
		t.Fatalf("TransformInbound returned error: %v", err)
	}
	if message.ID != "api_msg_1" || message.ConversationID != "conversation_api" || message.Role != RoleUser {
		t.Fatalf("unexpected api inbound message: %+v", message)
	}
	if len(message.Content) != 1 || message.Content[0].Type != ContentTypeText || message.Content[0].Text != "hello through rest" {
		t.Fatalf("unexpected api inbound content: %+v", message.Content)
	}
	if message.Metadata["adapter"] != "api" || message.Metadata["request_id"] != "req_1" {
		t.Fatalf("expected api metadata, got %+v", message.Metadata)
	}

	raw, err := adapter.TransformOutbound(InternalMessage{
		ID:             "reply_1",
		ConversationID: "conversation_api",
		Role:           RoleAssistant,
		Content:        []ContentPart{{Type: ContentTypeText, Text: "api reply"}},
		Metadata:       map[string]any{"trace_id": "trace_1"},
	})
	if err != nil {
		t.Fatalf("TransformOutbound returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("outbound payload is not JSON: %v", err)
	}
	if payload["api_event"] != "message" ||
		payload["conversation_id"] != "conversation_api" ||
		payload["role"] != "assistant" ||
		payload["text"] != "api reply" {
		t.Fatalf("unexpected api outbound payload: %+v", payload)
	}
	metadata, ok := payload["metadata"].(map[string]any)
	if !ok || metadata["trace_id"] != "trace_1" {
		t.Fatalf("expected api outbound metadata, got %+v", payload["metadata"])
	}
}

func TestSlackAdapterTransformsEventInboundAndTextOutbound(t *testing.T) {
	adapter, err := NewAdapterRegistry(nil).Adapter(ChannelType("slack"))
	if err != nil {
		t.Fatalf("expected slack adapter: %v", err)
	}

	message, err := adapter.TransformInbound(json.RawMessage(`{
		"event_id": "Ev123",
		"event": {
			"channel": "C123",
			"user": "U123",
			"text": "hello slack"
		}
	}`))
	if err != nil {
		t.Fatalf("TransformInbound returned error: %v", err)
	}
	if message.ID != "Ev123" || message.ConversationID != "C123" || message.Role != RoleUser {
		t.Fatalf("unexpected slack inbound message: %+v", message)
	}
	if len(message.Content) != 1 || message.Content[0].Text != "hello slack" {
		t.Fatalf("unexpected slack content: %+v", message.Content)
	}
	if message.Metadata["adapter"] != "slack" || message.Metadata["user_id"] != "U123" {
		t.Fatalf("expected slack metadata, got %+v", message.Metadata)
	}

	raw, err := adapter.TransformOutbound(InternalMessage{
		ID:             "reply_1",
		ConversationID: "C123",
		Role:           RoleAssistant,
		Content:        []ContentPart{{Type: ContentTypeText, Text: "slack reply"}},
	})
	if err != nil {
		t.Fatalf("TransformOutbound returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("outbound payload is not JSON: %v", err)
	}
	if payload["channel"] != "C123" || payload["text"] != "slack reply" {
		t.Fatalf("unexpected slack outbound payload: %+v", payload)
	}
}

func TestSlackAdapterTransformsFileInbound(t *testing.T) {
	adapter, err := NewAdapterRegistry(nil).Adapter(ChannelType("slack"))
	if err != nil {
		t.Fatalf("expected slack adapter: %v", err)
	}

	message, err := adapter.TransformInbound(json.RawMessage(`{
		"event_id": "EvFile123",
		"event": {
			"channel": "C123",
			"user": "U123",
			"text": "please review",
			"files": [
				{
					"id": "F123",
					"name": "report.pdf",
					"mimetype": "application/pdf",
					"size": 4096,
					"url_private": "https://files.slack.com/files-pri/T123-F123/report.pdf",
					"url_private_download": "https://files.slack.com/files-pri-download/T123-F123/report.pdf",
					"permalink": "https://workspace.slack.com/files/U123/F123/report.pdf"
				}
			]
		}
	}`))
	if err != nil {
		t.Fatalf("TransformInbound returned error: %v", err)
	}

	if len(message.Content) != 2 {
		t.Fatalf("expected text and file content, got %+v", message.Content)
	}
	if message.Content[0].Type != ContentTypeText || message.Content[0].Text != "please review" {
		t.Fatalf("expected text content first, got %+v", message.Content[0])
	}
	file := message.Content[1]
	if file.Type != ContentTypeFile {
		t.Fatalf("expected file content, got %+v", file)
	}
	if file.URL != "https://files.slack.com/files-pri/T123-F123/report.pdf" {
		t.Fatalf("expected url_private file URL, got %q", file.URL)
	}
	if file.Metadata["file_id"] != "F123" ||
		file.Metadata["name"] != "report.pdf" ||
		file.Metadata["mimetype"] != "application/pdf" ||
		file.Metadata["size"] != int64(4096) {
		t.Fatalf("unexpected file metadata: %+v", file.Metadata)
	}
}

func TestTelegramAdapterTransformsMessageInboundAndTextOutbound(t *testing.T) {
	adapter, err := NewAdapterRegistry(nil).Adapter(ChannelType("telegram"))
	if err != nil {
		t.Fatalf("expected telegram adapter: %v", err)
	}

	message, err := adapter.TransformInbound(json.RawMessage(`{
		"message": {
			"message_id": 42,
			"chat": {"id": 12345},
			"from": {"id": 678, "is_bot": false},
			"text": "hello telegram"
		}
	}`))
	if err != nil {
		t.Fatalf("TransformInbound returned error: %v", err)
	}
	if message.ID != "42" || message.ConversationID != "12345" || message.Role != RoleUser {
		t.Fatalf("unexpected telegram inbound message: %+v", message)
	}
	if len(message.Content) != 1 || message.Content[0].Text != "hello telegram" {
		t.Fatalf("unexpected telegram content: %+v", message.Content)
	}

	raw, err := adapter.TransformOutbound(InternalMessage{
		ID:             "reply_1",
		ConversationID: "12345",
		Role:           RoleAssistant,
		Content:        []ContentPart{{Type: ContentTypeText, Text: "telegram reply"}},
	})
	if err != nil {
		t.Fatalf("TransformOutbound returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("outbound payload is not JSON: %v", err)
	}
	if payload["chat_id"] != "12345" || payload["text"] != "telegram reply" {
		t.Fatalf("unexpected telegram outbound payload: %+v", payload)
	}
}

func TestTelegramAdapterTransformsDocumentInbound(t *testing.T) {
	adapter, err := NewAdapterRegistry(nil).Adapter(ChannelType("telegram"))
	if err != nil {
		t.Fatalf("expected telegram adapter: %v", err)
	}

	message, err := adapter.TransformInbound(json.RawMessage(`{
		"message": {
			"message_id": 43,
			"chat": {"id": 12345},
			"from": {"id": 678, "is_bot": false},
			"caption": "please review",
			"document": {
				"file_id": "BQACAgIAAxkBAAIB",
				"file_unique_id": "AgAD123",
				"file_name": "brief.pdf",
				"mime_type": "application/pdf",
				"file_size": 8192
			}
		}
	}`))
	if err != nil {
		t.Fatalf("TransformInbound returned error: %v", err)
	}

	if len(message.Content) != 2 {
		t.Fatalf("expected caption and document content, got %+v", message.Content)
	}
	if message.Content[0].Type != ContentTypeText || message.Content[0].Text != "please review" {
		t.Fatalf("expected caption text first, got %+v", message.Content[0])
	}
	file := message.Content[1]
	if file.Type != ContentTypeFile {
		t.Fatalf("expected file content, got %+v", file)
	}
	if file.URL != "telegram://file/BQACAgIAAxkBAAIB" {
		t.Fatalf("expected telegram file URL, got %q", file.URL)
	}
	if file.Metadata["file_id"] != "BQACAgIAAxkBAAIB" ||
		file.Metadata["file_unique_id"] != "AgAD123" ||
		file.Metadata["name"] != "brief.pdf" ||
		file.Metadata["mimetype"] != "application/pdf" ||
		file.Metadata["size"] != int64(8192) {
		t.Fatalf("unexpected file metadata: %+v", file.Metadata)
	}
}

func TestWebEmbedAdapterTransformsSdkMessageInboundAndOutbound(t *testing.T) {
	adapter, err := NewAdapterRegistry(nil).Adapter(ChannelType("web_embed"))
	if err != nil {
		t.Fatalf("expected web embed adapter: %v", err)
	}

	message, err := adapter.TransformInbound(json.RawMessage(`{
		"id": "web_msg_1",
		"conversation_id": "visitor_1",
		"role": "user",
		"text": "hello from iframe",
		"embed_origin": "https://app.example"
	}`))
	if err != nil {
		t.Fatalf("TransformInbound returned error: %v", err)
	}
	if message.ID != "web_msg_1" || message.ConversationID != "visitor_1" || message.Role != RoleUser {
		t.Fatalf("unexpected web embed inbound message: %+v", message)
	}
	if len(message.Content) != 1 || message.Content[0].Text != "hello from iframe" {
		t.Fatalf("unexpected web embed content: %+v", message.Content)
	}
	if message.Metadata["adapter"] != "web_embed" || message.Metadata["embed_origin"] != "https://app.example" {
		t.Fatalf("expected web embed metadata, got %+v", message.Metadata)
	}

	raw, err := adapter.TransformOutbound(InternalMessage{
		ID:             "reply_1",
		ConversationID: "visitor_1",
		Role:           RoleAssistant,
		Content:        []ContentPart{{Type: ContentTypeText, Text: "web reply"}},
	})
	if err != nil {
		t.Fatalf("TransformOutbound returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("outbound payload is not JSON: %v", err)
	}
	if payload["conversation_id"] != "visitor_1" || payload["text"] != "web reply" || payload["sdk_event"] != "message" {
		t.Fatalf("unexpected web embed outbound payload: %+v", payload)
	}
}

func TestFeishuAdapterTransformsCardInboundAndTextOutbound(t *testing.T) {
	adapter, err := NewAdapterRegistry(nil).Adapter(ChannelType("feishu"))
	if err != nil {
		t.Fatalf("expected feishu adapter: %v", err)
	}

	message, err := adapter.TransformInbound(json.RawMessage(`{
		"message_id": "feishu_msg_1",
		"chat_id": "chat_1",
		"message_type": "interactive",
		"card": {"header":{"title":{"content":"Deploy report"}},"elements":[{"tag":"div","text":{"content":"Phase done"}}]}
	}`))
	if err != nil {
		t.Fatalf("TransformInbound returned error: %v", err)
	}
	if message.ID != "feishu_msg_1" || message.ConversationID != "chat_1" || message.Role != RoleUser {
		t.Fatalf("unexpected inbound message: %+v", message)
	}
	if len(message.Content) != 1 || message.Content[0].Type != ContentTypeCard {
		t.Fatalf("expected card content, got %+v", message.Content)
	}
	if message.Metadata["message_type"] != "interactive" || message.Metadata["raw"] == nil {
		t.Fatalf("expected feishu raw metadata, got %+v", message.Metadata)
	}

	raw, err := adapter.TransformOutbound(InternalMessage{
		ID:             "reply_1",
		ConversationID: "chat_1",
		Role:           RoleAssistant,
		Content:        []ContentPart{{Type: ContentTypeText, Text: "reply text"}},
	})
	if err != nil {
		t.Fatalf("TransformOutbound returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("outbound payload is not JSON: %v", err)
	}
	if payload["msg_type"] != "text" || payload["chat_id"] != "chat_1" {
		t.Fatalf("unexpected feishu outbound payload: %+v", payload)
	}
	content, ok := payload["content"].(map[string]any)
	if !ok || content["text"] != "reply text" {
		t.Fatalf("expected feishu text content, got %+v", payload["content"])
	}
}

func TestWeChatAdapterTransformsJSONTextInboundAndOutbound(t *testing.T) {
	adapter, err := NewAdapterRegistry(nil).Adapter(ChannelType("wechat"))
	if err != nil {
		t.Fatalf("expected wechat adapter: %v", err)
	}

	message, err := adapter.TransformInbound(json.RawMessage(`{
		"MsgId": "wechat_msg_1",
		"FromUserName": "user_openid",
		"MsgType": "text",
		"Content": "hello from wechat"
	}`))
	if err != nil {
		t.Fatalf("TransformInbound returned error: %v", err)
	}
	if message.ID != "wechat_msg_1" || message.ConversationID != "user_openid" || message.Role != RoleUser {
		t.Fatalf("unexpected inbound message: %+v", message)
	}
	if len(message.Content) != 1 || message.Content[0].Type != ContentTypeText || message.Content[0].Text != "hello from wechat" {
		t.Fatalf("unexpected inbound content: %+v", message.Content)
	}
	if message.Metadata["msg_type"] != "text" {
		t.Fatalf("expected wechat msg_type metadata, got %+v", message.Metadata)
	}

	raw, err := adapter.TransformOutbound(InternalMessage{
		ID:             "reply_1",
		ConversationID: "user_openid",
		Role:           RoleAssistant,
		Content:        []ContentPart{{Type: ContentTypeText, Text: "wechat reply"}},
	})
	if err != nil {
		t.Fatalf("TransformOutbound returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("outbound payload is not JSON: %v", err)
	}
	if payload["msgtype"] != "text" || payload["touser"] != "user_openid" {
		t.Fatalf("unexpected wechat outbound payload: %+v", payload)
	}
	text, ok := payload["text"].(map[string]any)
	if !ok || text["content"] != "wechat reply" {
		t.Fatalf("expected wechat text content, got %+v", payload["text"])
	}
}

func TestWeChatAdapterTransformsCardOutboundToNewsPayload(t *testing.T) {
	adapter, err := NewAdapterRegistry(nil).Adapter(ChannelType("wechat"))
	if err != nil {
		t.Fatalf("expected wechat adapter: %v", err)
	}

	raw, err := adapter.TransformOutbound(InternalMessage{
		ID:             "reply_2",
		ConversationID: "user_openid",
		Role:           RoleAssistant,
		Content: []ContentPart{
			{
				Type: ContentTypeCard,
				Text: "Launch notes",
				URL:  "https://example.test/launch",
				Metadata: map[string]any{
					"description": "Read the rollout plan",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("TransformOutbound returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("outbound payload is not JSON: %v", err)
	}
	if payload["msgtype"] != "news" || payload["touser"] != "user_openid" {
		t.Fatalf("unexpected wechat outbound payload: %+v", payload)
	}
	news, ok := payload["news"].(map[string]any)
	if !ok {
		t.Fatalf("expected wechat news payload, got %+v", payload["news"])
	}
	articles, ok := news["articles"].([]any)
	if !ok || len(articles) != 1 {
		t.Fatalf("expected one wechat news article, got %+v", news["articles"])
	}
	article, ok := articles[0].(map[string]any)
	if !ok {
		t.Fatalf("expected article object, got %+v", articles[0])
	}
	if article["title"] != "Launch notes" ||
		article["description"] != "Read the rollout plan" ||
		article["url"] != "https://example.test/launch" {
		t.Fatalf("unexpected article payload: %+v", article)
	}
}

func TestWeChatAdapterTransformsImageOutboundToImagePayload(t *testing.T) {
	adapter, err := NewAdapterRegistry(nil).Adapter(ChannelType("wechat"))
	if err != nil {
		t.Fatalf("expected wechat adapter: %v", err)
	}

	raw, err := adapter.TransformOutbound(InternalMessage{
		ID:             "reply_3",
		ConversationID: "user_openid",
		Role:           RoleAssistant,
		Content: []ContentPart{
			{
				Type: ContentTypeImage,
				URL:  "https://cdn.example.test/chart.png",
				Metadata: map[string]any{
					"media_id": "media_123",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("TransformOutbound returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("outbound payload is not JSON: %v", err)
	}
	if payload["msgtype"] != "image" || payload["touser"] != "user_openid" {
		t.Fatalf("unexpected wechat outbound payload: %+v", payload)
	}
	image, ok := payload["image"].(map[string]any)
	if !ok || image["media_id"] != "media_123" {
		t.Fatalf("expected wechat image media_id, got %+v", payload["image"])
	}
}

func TestDiscordAdapterTransformsContentEmbedsInboundAndOutbound(t *testing.T) {
	adapter, err := NewAdapterRegistry(nil).Adapter(ChannelType("discord"))
	if err != nil {
		t.Fatalf("expected discord adapter: %v", err)
	}

	message, err := adapter.TransformInbound(json.RawMessage(`{
		"id": "discord_msg_1",
		"channel_id": "discord_channel_1",
		"author": {"id":"user_1","bot":false},
		"content": "hello discord",
		"embeds": [{"title":"Incident","description":"resolved"}]
	}`))
	if err != nil {
		t.Fatalf("TransformInbound returned error: %v", err)
	}
	if message.ID != "discord_msg_1" || message.ConversationID != "discord_channel_1" || message.Role != RoleUser {
		t.Fatalf("unexpected inbound message: %+v", message)
	}
	if len(message.Content) != 2 ||
		message.Content[0].Type != ContentTypeText ||
		message.Content[0].Text != "hello discord" ||
		message.Content[1].Type != ContentTypeCard {
		t.Fatalf("expected discord text and embed card content, got %+v", message.Content)
	}

	raw, err := adapter.TransformOutbound(InternalMessage{
		ID:             "reply_1",
		ConversationID: "discord_channel_1",
		Role:           RoleAssistant,
		Content: []ContentPart{
			{Type: ContentTypeText, Text: "discord reply"},
			{Type: ContentTypeCard, Metadata: map[string]any{"title": "Summary", "description": "sent"}},
		},
	})
	if err != nil {
		t.Fatalf("TransformOutbound returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("outbound payload is not JSON: %v", err)
	}
	if payload["content"] != "discord reply" || payload["channel_id"] != "discord_channel_1" {
		t.Fatalf("unexpected discord outbound payload: %+v", payload)
	}
	embeds, ok := payload["embeds"].([]any)
	if !ok || len(embeds) != 1 {
		t.Fatalf("expected one discord embed, got %+v", payload["embeds"])
	}
}

func TestDiscordAdapterTransformsAttachmentInbound(t *testing.T) {
	adapter, err := NewAdapterRegistry(nil).Adapter(ChannelType("discord"))
	if err != nil {
		t.Fatalf("expected discord adapter: %v", err)
	}

	message, err := adapter.TransformInbound(json.RawMessage(`{
		"id": "discord_msg_file_1",
		"channel_id": "discord_channel_1",
		"author": {"id":"user_1","bot":false},
		"content": "please review",
		"attachments": [
			{
				"id": "att_1",
				"filename": "incident.csv",
				"content_type": "text/csv",
				"size": 2048,
				"url": "https://cdn.discordapp.com/attachments/incident.csv",
				"proxy_url": "https://media.discordapp.net/attachments/incident.csv"
			}
		]
	}`))
	if err != nil {
		t.Fatalf("TransformInbound returned error: %v", err)
	}

	if len(message.Content) != 2 {
		t.Fatalf("expected text and attachment content, got %+v", message.Content)
	}
	if message.Content[0].Type != ContentTypeText || message.Content[0].Text != "please review" {
		t.Fatalf("expected text content first, got %+v", message.Content[0])
	}
	file := message.Content[1]
	if file.Type != ContentTypeFile {
		t.Fatalf("expected file content, got %+v", file)
	}
	if file.URL != "https://cdn.discordapp.com/attachments/incident.csv" {
		t.Fatalf("expected attachment url, got %q", file.URL)
	}
	if file.Metadata["attachment_id"] != "att_1" ||
		file.Metadata["name"] != "incident.csv" ||
		file.Metadata["mimetype"] != "text/csv" ||
		file.Metadata["size"] != int64(2048) ||
		file.Metadata["proxy_url"] != "https://media.discordapp.net/attachments/incident.csv" {
		t.Fatalf("unexpected attachment metadata: %+v", file.Metadata)
	}
}

func TestDiscordAdapterPreservesReactionMetadataInboundAndOutbound(t *testing.T) {
	adapter, err := NewAdapterRegistry(nil).Adapter(ChannelType("discord"))
	if err != nil {
		t.Fatalf("expected discord adapter: %v", err)
	}

	message, err := adapter.TransformInbound(json.RawMessage(`{
		"id": "discord_msg_react_1",
		"channel_id": "discord_channel_1",
		"author": {"id":"user_1","bot":false},
		"content": "vote now",
		"reactions": [
			{"count": 3, "me": true, "emoji": {"id":"emoji_1", "name":"rocket", "animated": false}}
		]
	}`))
	if err != nil {
		t.Fatalf("TransformInbound returned error: %v", err)
	}
	reactions, ok := message.Metadata["reactions"].([]map[string]any)
	if !ok || len(reactions) != 1 {
		t.Fatalf("expected reaction metadata, got %+v", message.Metadata["reactions"])
	}
	emoji, ok := reactions[0]["emoji"].(map[string]any)
	if !ok || emoji["name"] != "rocket" || reactions[0]["count"] != float64(3) || reactions[0]["me"] != true {
		t.Fatalf("unexpected reaction metadata: %+v", reactions)
	}

	raw, err := adapter.TransformOutbound(InternalMessage{
		ID:             "reply_1",
		ConversationID: "discord_channel_1",
		Role:           RoleAssistant,
		Content:        []ContentPart{{Type: ContentTypeText, Text: "react with rocket"}},
		Metadata: map[string]any{
			"reactions": []map[string]any{{
				"emoji": map[string]any{"name": "rocket"},
				"count": int64(1),
			}},
		},
	})
	if err != nil {
		t.Fatalf("TransformOutbound returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("outbound payload is not JSON: %v", err)
	}
	outboundReactions, ok := payload["reactions"].([]any)
	if !ok || len(outboundReactions) != 1 {
		t.Fatalf("expected outbound reactions, got %+v from %s", payload["reactions"], raw)
	}
	outboundReaction, ok := outboundReactions[0].(map[string]any)
	if !ok {
		t.Fatalf("expected reaction object, got %+v", outboundReactions[0])
	}
	outboundEmoji, ok := outboundReaction["emoji"].(map[string]any)
	if !ok || outboundEmoji["name"] != "rocket" {
		t.Fatalf("unexpected outbound reaction payload: %+v", outboundReaction)
	}
}
