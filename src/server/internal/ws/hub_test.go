package ws

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestHubBroadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	event := Event{
		Type:     "notification",
		Category: "system",
		Payload:  map[string]string{"message": "test"},
	}

	done := make(chan []byte, 1)

	go func() {
		hub.SendToUser("user-1", event)
	}()

	select {
	case msg := <-done:
		var received Event
		if err := json.Unmarshal(msg, &received); err != nil {
			t.Errorf("failed to unmarshal message: %v", err)
		}
		if received.Type != "notification" {
			t.Errorf("expected type notification, got %s", received.Type)
		}
	case <-time.After(time.Second):
	}
}

func TestEventMarshal(t *testing.T) {
	event := Event{
		Type:      "notification",
		Category:  "billing",
		Payload:   map[string]any{"amount": 10.5, "currency": "USD"},
		Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	data, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if decoded.Type != "notification" {
		t.Errorf("expected type notification, got %s", decoded.Type)
	}
	if decoded.Category != "billing" {
		t.Errorf("expected category billing, got %s", decoded.Category)
	}
}

func TestNotifyUser(t *testing.T) {
	hub := DefaultHub()

	NotifyUser("test-user", "agent_status", "agent", map[string]string{
		"agentId": "agent-1",
		"status":  "running",
	})

	if hub.OnlineCount() != 0 {
		t.Errorf("expected 0 online users, got %d", hub.OnlineCount())
	}
}

func TestHubMultipleUsers(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	event1 := Event{Type: "test1", Category: "system", Payload: "data1"}
	event2 := Event{Type: "test2", Category: "system", Payload: "data2"}

	hub.SendToUser("user-1", event1)
	hub.SendToUser("user-2", event2)
	hub.SendToUser("user-1", event2)

	time.Sleep(10 * time.Millisecond)
}

var _ = websocket.CloseMessage
