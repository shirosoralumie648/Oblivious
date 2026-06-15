package ws

import (
	"encoding/json"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

type Event struct {
	Type      string    `json:"type"`     // 'notification' | 'agent_status' | 'billing' | 'system' | 'chat_messages_synced' | 'chat_message_updated' | 'chat_message_deleted' | 'chat_typing'
	Category  string    `json:"category"` // 'billing' | 'agent' | 'system' | 'mcp' | 'chat'
	Payload   any       `json:"payload"`
	Timestamp time.Time `json:"timestamp"`
}

type ClientMessage struct {
	Type           string `json:"type"`
	ConversationID string `json:"conversationId"`
	IsTyping       bool   `json:"isTyping"`
}

type Client struct {
	hub    *Hub
	userID string
	conn   *websocket.Conn
	send   chan []byte
	rooms  map[string]struct{}
}

type Hub struct {
	mu         sync.RWMutex
	clients    map[string]map[*Client]struct{} // userID -> set of clients
	rooms      map[string]map[*Client]struct{} // conversationID -> set of clients
	register   chan *Client
	unregister chan *Client
	broadcast  chan *BroadcastMessage
	roomEvents chan *ConversationMessage
}

type BroadcastMessage struct {
	UserID string
	Event  Event
}

type ConversationMessage struct {
	ConversationID string
	Sender         *Client
	Event          Event
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]map[*Client]struct{}),
		rooms:      make(map[string]map[*Client]struct{}),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *BroadcastMessage, 256),
		roomEvents: make(chan *ConversationMessage, 256),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			if h.clients[client.userID] == nil {
				h.clients[client.userID] = make(map[*Client]struct{})
			}
			h.clients[client.userID][client] = struct{}{}
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if clients, ok := h.clients[client.userID]; ok {
				if _, exists := clients[client]; exists {
					delete(clients, client)
					close(client.send)
					if len(clients) == 0 {
						delete(h.clients, client.userID)
					}
				}
			}
			h.removeClientFromRoomsLocked(client)
			h.mu.Unlock()

		case msg := <-h.broadcast:
			data, err := json.Marshal(msg.Event)
			if err != nil {
				continue
			}
			h.mu.RLock()
			clients := h.clients[msg.UserID]
			for client := range clients {
				select {
				case client.send <- data:
				default:
					h.mu.RUnlock()
					h.mu.Lock()
					delete(h.clients[msg.UserID], client)
					close(client.send)
					if len(h.clients[msg.UserID]) == 0 {
						delete(h.clients, msg.UserID)
					}
					h.mu.Unlock()
					h.mu.RLock()
				}
			}
			h.mu.RUnlock()

		case msg := <-h.roomEvents:
			data, err := json.Marshal(msg.Event)
			if err != nil {
				continue
			}
			var staleClients []*Client
			h.mu.RLock()
			clients := h.rooms[msg.ConversationID]
			for client := range clients {
				if msg.Sender != nil && client == msg.Sender {
					continue
				}
				select {
				case client.send <- data:
				default:
					staleClients = append(staleClients, client)
				}
			}
			h.mu.RUnlock()
			if len(staleClients) > 0 {
				h.mu.Lock()
				for _, client := range staleClients {
					h.removeClientLocked(client)
				}
				h.mu.Unlock()
			}
		}
	}
}

func (h *Hub) removeClientLocked(client *Client) {
	if clients, ok := h.clients[client.userID]; ok {
		if _, exists := clients[client]; exists {
			delete(clients, client)
			close(client.send)
			if len(clients) == 0 {
				delete(h.clients, client.userID)
			}
		}
	}
	h.removeClientFromRoomsLocked(client)
}

func (h *Hub) removeClientFromRoomsLocked(client *Client) {
	for conversationID := range client.rooms {
		if clients, ok := h.rooms[conversationID]; ok {
			delete(clients, client)
			if len(clients) == 0 {
				delete(h.rooms, conversationID)
			}
		}
		delete(client.rooms, conversationID)
	}
}

func (h *Hub) SendToUser(userID string, event Event) {
	event.Timestamp = time.Now().UTC()
	h.broadcast <- &BroadcastMessage{UserID: userID, Event: event}
}

func (h *Hub) SendToConversation(conversationID string, event Event) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return
	}
	event.Timestamp = time.Now().UTC()
	h.roomEvents <- &ConversationMessage{ConversationID: conversationID, Event: event}
}

func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) ConversationSubscriberCount(conversationID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.rooms[conversationID])
}

func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		c.handleMessage(message)
	}
}

func (c *Client) handleMessage(message []byte) {
	var payload ClientMessage
	if err := json.Unmarshal(message, &payload); err != nil {
		return
	}

	conversationID := strings.TrimSpace(payload.ConversationID)
	if conversationID == "" {
		return
	}

	switch payload.Type {
	case "chat_join":
		c.joinConversation(conversationID)
	case "chat_leave":
		c.leaveConversation(conversationID)
	case "chat_typing":
		c.joinConversation(conversationID)
		c.hub.SendConversationFromClient(conversationID, c, Event{
			Type:     "chat_typing",
			Category: "chat",
			Payload: map[string]any{
				"conversationId": conversationID,
				"isTyping":       payload.IsTyping,
				"userId":         c.userID,
			},
		})
	}
}

func (c *Client) joinConversation(conversationID string) {
	c.hub.mu.Lock()
	defer c.hub.mu.Unlock()
	if c.rooms == nil {
		c.rooms = make(map[string]struct{})
	}
	if _, exists := c.rooms[conversationID]; exists {
		return
	}
	if c.hub.rooms[conversationID] == nil {
		c.hub.rooms[conversationID] = make(map[*Client]struct{})
	}
	c.rooms[conversationID] = struct{}{}
	c.hub.rooms[conversationID][c] = struct{}{}
}

func (c *Client) leaveConversation(conversationID string) {
	c.hub.mu.Lock()
	defer c.hub.mu.Unlock()
	if clients, ok := c.hub.rooms[conversationID]; ok {
		delete(clients, c)
		if len(clients) == 0 {
			delete(c.hub.rooms, conversationID)
		}
	}
	delete(c.rooms, conversationID)
}

func (h *Hub) SendConversationFromClient(conversationID string, sender *Client, event Event) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return
	}
	event.Timestamp = time.Now().UTC()
	h.roomEvents <- &ConversationMessage{ConversationID: conversationID, Sender: sender, Event: event}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

var defaultHub *Hub
var once sync.Once

func DefaultHub() *Hub {
	once.Do(func() {
		defaultHub = NewHub()
		go defaultHub.Run()
	})
	return defaultHub
}

func NotifyUser(userID, eventType, category string, payload any) {
	DefaultHub().SendToUser(userID, Event{
		Type:     eventType,
		Category: category,
		Payload:  payload,
	})
}

func NotifyConversation(conversationID, eventType string, payload any) {
	DefaultHub().SendToConversation(conversationID, Event{
		Type:     eventType,
		Category: "chat",
		Payload:  payload,
	})
}

func init() {
	log.Println("[ws] hub initialized")
}
