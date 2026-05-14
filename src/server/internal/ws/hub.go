package ws

import (
	"encoding/json"
	"log"
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
	Type      string    `json:"type"`     // 'notification' | 'agent_status' | 'billing' | 'system'
	Category  string    `json:"category"` // 'billing' | 'agent' | 'system' | 'mcp'
	Payload   any       `json:"payload"`
	Timestamp time.Time `json:"timestamp"`
}

type Client struct {
	hub    *Hub
	userID string
	conn   *websocket.Conn
	send   chan []byte
}

type Hub struct {
	mu         sync.RWMutex
	clients    map[string]map[*Client]struct{} // userID -> set of clients
	register   chan *Client
	unregister chan *Client
	broadcast  chan *BroadcastMessage
}

type BroadcastMessage struct {
	UserID string
	Event  Event
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]map[*Client]struct{}),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *BroadcastMessage, 256),
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
		}
	}
}

func (h *Hub) SendToUser(userID string, event Event) {
	event.Timestamp = time.Now().UTC()
	h.broadcast <- &BroadcastMessage{UserID: userID, Event: event}
}

func (h *Hub) OnlineCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
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
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
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

func init() {
	log.Println("[ws] hub initialized")
}
