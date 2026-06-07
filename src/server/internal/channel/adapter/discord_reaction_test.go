package adapter

import (
	"encoding/json"
	"testing"
)

func TestDiscordAdapterPreservesReactionMetadataInboundAndOutbound(t *testing.T) {
	adp := NewDiscordAdapter()

	message, err := adp.TransformInbound(json.RawMessage(`{
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

	raw, err := adp.TransformOutbound(InternalMessage{
		ID:             "reply_1",
		ConversationID: "discord_channel_1",
		Role:           "assistant",
		Content:        []ContentPart{{Type: "text", Text: "react with rocket"}},
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
	reactionsPayload, ok := payload["reactions"].([]any)
	if !ok || len(reactionsPayload) != 1 {
		t.Fatalf("expected outbound reactions, got %+v from %s", payload["reactions"], raw)
	}
	reaction, ok := reactionsPayload[0].(map[string]any)
	if !ok {
		t.Fatalf("expected reaction object, got %+v", reactionsPayload[0])
	}
	emojiPayload, ok := reaction["emoji"].(map[string]any)
	if !ok || emojiPayload["name"] != "rocket" {
		t.Fatalf("unexpected outbound reaction payload: %+v", reaction)
	}
}
