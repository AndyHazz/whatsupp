package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// Validate that the Origin header matches the request Host to prevent
		// cross-site WebSocket hijacking. If no Origin is sent (non-browser
		// clients), allow the connection since cookie auth still applies.
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		// The Origin must match the Host header (same-origin)
		host := r.Host
		return strings.Contains(origin, host)
	},
}

// WSMessage is the JSON envelope for all WebSocket messages.
type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// wsClient represents a single WebSocket connection.
type wsClient struct {
	conn *websocket.Conn
	send chan []byte
}

// WSHub manages WebSocket connections and broadcasts messages.
type WSHub struct {
	mu         sync.RWMutex
	clients    map[*wsClient]bool
	register   chan *wsClient
	unregister chan *wsClient
	broadcast  chan []byte
	quit       chan struct{}
}

// NewWSHub creates a new WebSocket hub.
func NewWSHub() *WSHub {
	return &WSHub{
		clients:    make(map[*wsClient]bool),
		register:   make(chan *wsClient),
		unregister: make(chan *wsClient),
		broadcast:  make(chan []byte, 256),
		quit:       make(chan struct{}),
	}
}

// Run starts the hub event loop. Call in a goroutine.
func (h *WSHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			// Snapshot client list to avoid lock inversion during eviction
			h.mu.RLock()
			snapshot := make([]*wsClient, 0, len(h.clients))
			for c := range h.clients {
				snapshot = append(snapshot, c)
			}
			h.mu.RUnlock()

			for _, client := range snapshot {
				select {
				case client.send <- message:
				default:
					// Client too slow — schedule disconnect
					h.unregister <- client
				}
			}

		case <-h.quit:
			h.mu.Lock()
			for client := range h.clients {
				close(client.send)
				delete(h.clients, client)
			}
			h.mu.Unlock()
			return
		}
	}
}

// Stop shuts down the hub.
func (h *WSHub) Stop() {
	select {
	case <-h.quit:
		// already stopped
	default:
		close(h.quit)
	}
}

// Broadcast sends a message to all connected clients.
func (h *WSHub) Broadcast(msg WSMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("ws broadcast marshal error: %v", err)
		return
	}
	select {
	case h.broadcast <- data:
	default:
		log.Printf("ws broadcast channel full, dropping message")
	}
}

// ClientCount returns the number of connected clients.
func (h *WSHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// HandleWebSocket handles WS /api/v1/ws.
// Auth is via session cookie (browsers can't set headers on WS upgrade).
func (h *Handlers) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Authenticate via session cookie
	cookie, err := r.Cookie("session")
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	session, err := h.store.GetSession(cookie.Value)
	if err != nil || session == nil || time.Now().After(session.ExpiresAt) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade error: %v", err)
		return
	}

	client := &wsClient{
		conn: conn,
		send: make(chan []byte, 256),
	}

	h.wsHub.register <- client

	// Writer goroutine with periodic ping
	go func() {
		pingTicker := time.NewTicker(30 * time.Second)
		defer func() {
			pingTicker.Stop()
			conn.Close()
		}()
		for {
			select {
			case msg, ok := <-client.send:
				if !ok {
					conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
					return
				}
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					return
				}
			case <-pingTicker.C:
				conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					return
				}
			}
		}
	}()

	// Reader goroutine (handles pings/pongs and detects disconnects)
	go func() {
		defer func() {
			h.wsHub.unregister <- client
			conn.Close()
		}()
		conn.SetReadLimit(512) // We don't expect large messages from clients
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		conn.SetPongHandler(func(string) error {
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()
}
