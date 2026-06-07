package adapter

import (
	"encoding/json"
	"testing"
)

func TestFeiShuAdapterTransformOutboundCardUsesInteractivePayload(t *testing.T) {
	adp := NewFeiShuAdapter()

	raw, err := adp.TransformOutbound(InternalMessage{
		ConversationID: "oc_123",
		Role:           "assistant",
		Content: []ContentPart{{
			Type: "card",
			Metadata: map[string]any{
				"schema": "2.0",
				"header": map[string]any{
					"title": map[string]any{"content": "Incident resolved"},
				},
				"elements": []any{
					map[string]any{"tag": "button", "text": map[string]any{"content": "Open runbook"}},
				},
			},
		}},
	})
	if err != nil {
		t.Fatalf("TransformOutbound returned error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("outbound payload is not JSON: %v", err)
	}
	if payload["chat_id"] != "oc_123" {
		t.Fatalf("expected chat_id oc_123, got %#v", payload["chat_id"])
	}
	if payload["msg_type"] != "interactive" {
		t.Fatalf("expected interactive message type, got %#v in payload %s", payload["msg_type"], raw)
	}
	card, ok := payload["card"].(map[string]any)
	if !ok {
		t.Fatalf("expected card object, got %#v in payload %s", payload["card"], raw)
	}
	if card["schema"] != "2.0" {
		t.Fatalf("expected card metadata to be preserved, got %#v", card)
	}
	if _, exists := payload["content"]; exists {
		t.Fatalf("interactive card payload should not include text content, got %s", raw)
	}
}

func TestWeChatAdapterTransformOutboundCardUsesNewsPayload(t *testing.T) {
	adp := NewWeChatAdapter()

	raw, err := adp.TransformOutbound(InternalMessage{
		ConversationID: "openid_123",
		Role:           "assistant",
		Content: []ContentPart{{
			Type: "card",
			Text: "Release notes",
			URL:  "https://example.com/releases/2026-06-07",
			Metadata: map[string]any{
				"description": "June release summary",
				"picurl":      "https://example.com/release.png",
			},
		}},
	})
	if err != nil {
		t.Fatalf("TransformOutbound returned error: %v", err)
	}

	var payload struct {
		ToUser  string `json:"touser"`
		MsgType string `json:"msgtype"`
		News    struct {
			Articles []struct {
				Title       string `json:"title"`
				Description string `json:"description"`
				URL         string `json:"url"`
				PicURL      string `json:"picurl"`
			} `json:"articles"`
		} `json:"news"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("outbound payload is not JSON: %v", err)
	}
	if payload.ToUser != "openid_123" || payload.MsgType != "news" {
		t.Fatalf("expected news payload for openid_123, got %+v from %s", payload, raw)
	}
	if len(payload.News.Articles) != 1 {
		t.Fatalf("expected one news article, got %+v from %s", payload.News.Articles, raw)
	}
	article := payload.News.Articles[0]
	if article.Title != "Release notes" ||
		article.Description != "June release summary" ||
		article.URL != "https://example.com/releases/2026-06-07" ||
		article.PicURL != "https://example.com/release.png" {
		t.Fatalf("unexpected news article: %+v", article)
	}
}

func TestWeChatAdapterTransformOutboundImageUsesMediaIDPayload(t *testing.T) {
	adp := NewWeChatAdapter()

	raw, err := adp.TransformOutbound(InternalMessage{
		ConversationID: "openid_123",
		Role:           "assistant",
		Content: []ContentPart{{
			Type:     "image",
			Metadata: map[string]any{"media_id": "media_456"},
		}},
	})
	if err != nil {
		t.Fatalf("TransformOutbound returned error: %v", err)
	}

	var payload struct {
		ToUser  string            `json:"touser"`
		MsgType string            `json:"msgtype"`
		Image   map[string]string `json:"image"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("outbound payload is not JSON: %v", err)
	}
	if payload.ToUser != "openid_123" || payload.MsgType != "image" {
		t.Fatalf("expected image payload for openid_123, got %+v from %s", payload, raw)
	}
	if payload.Image["media_id"] != "media_456" {
		t.Fatalf("expected image media_id media_456, got %+v from %s", payload.Image, raw)
	}
}
