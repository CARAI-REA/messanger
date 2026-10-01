package hub

import (
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"
)

type Client struct {
	UserID int64
	Conn   *websocket.Conn
	Send   chan []byte
	Chats  map[int64]struct{}
}

type Hub struct {
	mu      sync.RWMutex
	clients map[int64]map[*Client]struct{}
}

func New() *Hub {
	return &Hub{clients: make(map[int64]map[*Client]struct{})}
}

func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c.Chats == nil {
		c.Chats = make(map[int64]struct{})
	}
	if h.clients[c.UserID] == nil {
		h.clients[c.UserID] = make(map[*Client]struct{})
	}
	h.clients[c.UserID][c] = struct{}{}
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if set, ok := h.clients[c.UserID]; ok {
		delete(set, c)
		if len(set) == 0 {
			delete(h.clients, c.UserID)
		}
	}
	close(c.Send)
}

func (h *Hub) AddChat(userID, chatID int64) {
	if userID <= 0 || chatID <= 0 {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients[userID] {
		if c.Chats == nil {
			c.Chats = make(map[int64]struct{})
		}
		c.Chats[chatID] = struct{}{}
	}
}

func (h *Hub) RemoveChat(userID, chatID int64) {
	if userID <= 0 || chatID <= 0 {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients[userID] {
		if c.Chats == nil {
			continue
		}
		delete(c.Chats, chatID)
	}
}

func (h *Hub) ConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	n := 0
	for _, set := range h.clients {
		n += len(set)
	}
	return n
}

func (h *Hub) SendToUser(userID int64, payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients[userID] {
		select {
		case c.Send <- payload:
		default:
		}
	}
}

func (h *Hub) SendToChat(chatID int64, payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, set := range h.clients {
		for c := range set {
			if _, ok := c.Chats[chatID]; !ok {
				continue
			}
			select {
			case c.Send <- payload:
			default:
			}
		}
	}
}

func (h *Hub) Broadcast(payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, set := range h.clients {
		for c := range set {
			select {
			case c.Send <- payload:
			default:
			}
		}
	}
}

type Envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}
