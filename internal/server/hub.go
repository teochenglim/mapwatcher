package server

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/teochenglim/mapwatch/internal/marker"
)

// WSEvent is a WebSocket message sent to connected clients.
type WSEvent struct {
	Type   string          `json:"type"`
	Marker *marker.Marker  `json:"marker,omitempty"`
	ID     string          `json:"id,omitempty"`
}

// client represents a single WebSocket connection.
type client struct {
	conn *websocket.Conn
	send chan []byte
	done chan struct{}
}

// Hub manages WebSocket client connections and broadcasts events.
type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
	store   *marker.Store
}

// NewHub creates a Hub backed by the given marker store.
func NewHub(store *marker.Store) *Hub {
	return &Hub{
		clients: make(map[*client]struct{}),
		store:   store,
	}
}

// Register adds a new WebSocket client and immediately replays all current markers.
func (h *Hub) Register(conn *websocket.Conn) {
	c := &client{
		conn: conn,
		send: make(chan []byte, 256),
		done: make(chan struct{}),
	}

	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	// Replay current state.
	for _, m := range h.store.All() {
		ev := WSEvent{Type: "marker.add", Marker: m}
		if data, err := json.Marshal(ev); err == nil {
			c.send <- data
		}
	}

	// Write pump.
	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.clients, c)
			h.mu.Unlock()
			conn.Close()
		}()
		for {
			select {
			case msg, ok := <-c.send:
				if !ok {
					return
				}
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					return
				}
			case <-c.done:
				return
			}
		}
	}()

	// Read pump — drain to detect disconnects.
	go func() {
		defer close(c.done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

// BroadcastAdd sends a marker.add event to all connected clients.
func (h *Hub) BroadcastAdd(m *marker.Marker) {
	h.broadcast(WSEvent{Type: "marker.add", Marker: m})
}

// BroadcastUpdate sends a marker.update event to all connected clients.
func (h *Hub) BroadcastUpdate(m *marker.Marker) {
	h.broadcast(WSEvent{Type: "marker.update", Marker: m})
}

// BroadcastRemove sends a marker.remove event to all connected clients.
func (h *Hub) BroadcastRemove(id string) {
	h.broadcast(WSEvent{Type: "marker.remove", ID: id})
}

func (h *Hub) broadcast(ev WSEvent) {
	data, err := json.Marshal(ev)
	if err != nil {
		log.Printf("hub: marshal event: %v", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- data:
		default:
			// Client buffer full — skip this event for this client.
			log.Printf("hub: client send buffer full, dropping event")
		}
	}
}
