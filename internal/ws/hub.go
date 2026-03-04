package ws

import (
	"encoding/json"
	"log"
	"sync"
)

type Hub struct {
	clients    map[*Client]bool
	Register   chan *Client
	Unregister chan *Client
	mu         sync.RWMutex

	// Called when a browser sends a message
	OnMessage func(client *Client, msg *WSMessage)
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.mu.Lock()
			h.clients[client] = true
			count := len(h.clients)
			h.mu.Unlock()
			log.Printf("[ws] Client connected (total: %d)", count)

		case client := <-h.Unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			count := len(h.clients)
			h.mu.Unlock()
			log.Printf("[ws] Client disconnected (total: %d)", count)
		}
	}
}

func (h *Hub) Broadcast(msg *WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("[ws] Failed to marshal broadcast: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		select {
		case client.send <- data:
		default:
			// Client too slow, will be cleaned up
			go func(c *Client) {
				h.Unregister <- c
			}(client)
		}
	}
}

func (h *Hub) BroadcastEvent(eventType string, payload interface{}) {
	h.Broadcast(&WSMessage{
		Type:    eventType,
		Payload: payload,
	})
}

func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
