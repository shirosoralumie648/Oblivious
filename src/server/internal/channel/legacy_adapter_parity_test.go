package channel

import (
	"encoding/json"
	"testing"
)

func TestLegacyChannelServiceRegistersTelegramAndWebEmbedAdapters(t *testing.T) {
	service := NewChannelService(nil)

	for _, channelType := range []string{"telegram", "web_embed"} {
		t.Run(channelType, func(t *testing.T) {
			adapter, ok := service.GetAdapter(channelType)
			if !ok {
				t.Fatalf("expected default adapter for %q", channelType)
			}
			if adapter.Type() != channelType {
				t.Fatalf("expected adapter type %q, got %q", channelType, adapter.Type())
			}
		})
	}
}

func TestLegacyChannelServiceSendsTelegramTextPayload(t *testing.T) {
	service := NewChannelService(nil)

	log, err := service.SendMessage("telegram", InternalMessage{
		ID:             "msg_telegram_1",
		ConversationID: "12345",
		Role:           RoleAssistant,
		Content:        []ContentPart{{Type: ContentTypeText, Text: "telegram reply"}},
	})
	if err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	if !log.TransformSuccess {
		t.Fatalf("expected transform success, got %q", log.TransformError)
	}
	var payload map[string]any
	if err := json.Unmarshal(log.RawMessage, &payload); err != nil {
		t.Fatalf("raw message is not JSON: %v", err)
	}
	if payload["chat_id"] != "12345" || payload["text"] != "telegram reply" {
		t.Fatalf("unexpected telegram payload: %+v", payload)
	}
}

func TestLegacyChannelServiceSendsWebEmbedPayload(t *testing.T) {
	service := NewChannelService(nil)

	log, err := service.SendMessage("web_embed", InternalMessage{
		ID:             "msg_web_1",
		ConversationID: "visitor_1",
		Role:           RoleAssistant,
		Content:        []ContentPart{{Type: ContentTypeText, Text: "hello web"}},
		Metadata:       map[string]any{"trace_id": "trace_1"},
	})
	if err != nil {
		t.Fatalf("SendMessage returned error: %v", err)
	}
	if !log.TransformSuccess {
		t.Fatalf("expected transform success, got %q", log.TransformError)
	}
	var payload map[string]any
	if err := json.Unmarshal(log.RawMessage, &payload); err != nil {
		t.Fatalf("raw message is not JSON: %v", err)
	}
	if payload["sdk_event"] != "message" ||
		payload["conversation_id"] != "visitor_1" ||
		payload["role"] != "assistant" ||
		payload["text"] != "hello web" {
		t.Fatalf("unexpected web embed payload: %+v", payload)
	}
	metadata, ok := payload["metadata"].(map[string]any)
	if !ok || metadata["trace_id"] != "trace_1" {
		t.Fatalf("expected metadata to be preserved, got %+v", payload["metadata"])
	}
}
