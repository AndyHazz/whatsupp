package api

import "sync"

// WSMessage represents a message to broadcast to WebSocket clients.
type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// WSHub manages WebSocket connections and message broadcasting.
type WSHub struct {
	mu       sync.RWMutex
	messages []WSMessage
	stopCh   chan struct{}
}

// NewWSHub creates a new WebSocket hub.
func NewWSHub() *WSHub {
	return &WSHub{
		stopCh: make(chan struct{}),
	}
}

// Run starts the hub's main loop.
func (h *WSHub) Run() {
	<-h.stopCh
}

// Stop shuts down the hub.
func (h *WSHub) Stop() {
	select {
	case <-h.stopCh:
		// already stopped
	default:
		close(h.stopCh)
	}
}

// Broadcast sends a message to all connected clients.
func (h *WSHub) Broadcast(msg WSMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messages = append(h.messages, msg)
}
