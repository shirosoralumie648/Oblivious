package ws

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func newTestClient(hub *Hub, userID string) *Client {
	return &Client{
		hub:    hub,
		userID: userID,
		send:   make(chan []byte, 8),
		rooms:  make(map[string]struct{}),
	}
}

func readTestEvent(t *testing.T, ch <-chan []byte) Event {
	t.Helper()
	select {
	case data := <-ch:
		var event Event
		if err := json.Unmarshal(data, &event); err != nil {
			t.Fatalf("unmarshal event: %v", err)
		}
		return event
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
		return Event{}
	}
}

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

func TestHubConversationBroadcastExcludesSenderAndOtherRooms(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	sender := newTestClient(hub, "user-1")
	recipient := newTestClient(hub, "user-2")
	otherRoom := newTestClient(hub, "user-3")
	hub.register <- sender
	hub.register <- recipient
	hub.register <- otherRoom
	sender.joinConversation("conversation_1")
	recipient.joinConversation("conversation_1")
	otherRoom.joinConversation("conversation_2")

	hub.SendConversationFromClient("conversation_1", sender, Event{
		Type:     "chat_typing",
		Category: "chat",
		Payload:  map[string]any{"conversationId": "conversation_1", "isTyping": true},
	})

	event := readTestEvent(t, recipient.send)
	if event.Type != "chat_typing" || event.Category != "chat" {
		t.Fatalf("expected chat typing event, got %+v", event)
	}

	select {
	case data := <-sender.send:
		t.Fatalf("sender should not receive own conversation event, got %s", string(data))
	case data := <-otherRoom.send:
		t.Fatalf("other room should not receive conversation event, got %s", string(data))
	case <-time.After(50 * time.Millisecond):
	}
}

func TestClientTypingMessageJoinsConversationAndBroadcastsPresence(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	sender := newTestClient(hub, "user-1")
	recipient := newTestClient(hub, "user-2")
	hub.register <- sender
	hub.register <- recipient
	recipient.joinConversation("conversation_1")

	sender.handleMessage([]byte(`{"type":"chat_typing","conversationId":"conversation_1","isTyping":true}`))

	event := readTestEvent(t, recipient.send)
	if event.Type != "chat_typing" || event.Category != "chat" {
		t.Fatalf("expected chat typing event, got %+v", event)
	}
	payload, ok := event.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected object payload, got %#v", event.Payload)
	}
	if payload["conversationId"] != "conversation_1" || payload["userId"] != "user-1" || payload["isTyping"] != true {
		t.Fatalf("unexpected typing payload: %+v", payload)
	}
	if count := hub.ConversationSubscriberCount("conversation_1"); count != 2 {
		t.Fatalf("expected sender to join conversation, got subscriber count %d", count)
	}
}

func TestHubUnregisterRemovesConversationSubscriptions(t *testing.T) {
	hub := NewHub()
	go hub.Run()

	client := newTestClient(hub, "user-1")
	hub.register <- client
	client.joinConversation("conversation_1")
	if count := hub.ConversationSubscriberCount("conversation_1"); count != 1 {
		t.Fatalf("expected one subscriber before unregister, got %d", count)
	}

	hub.unregister <- client

	deadline := time.After(time.Second)
	for {
		if count := hub.ConversationSubscriberCount("conversation_1"); count == 0 {
			return
		}
		select {
		case <-deadline:
			t.Fatal("conversation subscription was not removed")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}
}

var _ = websocket.CloseMessage
