package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestWSHub_BroadcastReachesClient(t *testing.T) {
	hub := NewWSHub()
	go hub.Run()
	defer hub.Stop()

	// Create a test server with WebSocket handler
	store := &mockStore{
		sessions: map[string]*Session{
			"ws-token": {Token: "ws-token", UserID: 1, ExpiresAt: time.Now().Add(time.Hour)},
		},
	}
	h := &Handlers{wsHub: hub, store: store}

	server := httptest.NewServer(http.HandlerFunc(h.HandleWebSocket))
	defer server.Close()

	// Connect WebSocket client
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	header := http.Header{}
	header.Add("Cookie", "session=ws-token")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Give client time to register
	time.Sleep(50 * time.Millisecond)

	// Broadcast a message
	hub.Broadcast(WSMessage{
		Type: "check_result",
		Data: map[string]interface{}{
			"monitor":    "Plex",
			"status":     "up",
			"latency_ms": 45.2,
			"timestamp":  1711018800,
		},
	})

	// Read the message
	conn.SetReadDeadline(time.Now().Add(time.Second))
	var msg WSMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("read: %v", err)
	}

	if msg.Type != "check_result" {
		t.Errorf("expected type check_result, got %q", msg.Type)
	}
}

func TestWSHub_DisconnectedClientRemoved(t *testing.T) {
	hub := NewWSHub()
	go hub.Run()
	defer hub.Stop()

	store := &mockStore{
		sessions: map[string]*Session{
			"ws-token": {Token: "ws-token", UserID: 1, ExpiresAt: time.Now().Add(time.Hour)},
		},
	}
	h := &Handlers{wsHub: hub, store: store}

	server := httptest.NewServer(http.HandlerFunc(h.HandleWebSocket))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	header := http.Header{}
	header.Add("Cookie", "session=ws-token")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 1 {
		t.Errorf("expected 1 client, got %d", hub.ClientCount())
	}

	conn.Close()
	time.Sleep(100 * time.Millisecond)

	// Broadcast should not panic with no clients
	hub.Broadcast(WSMessage{Type: "test", Data: nil})

	// Client count may take a moment to update
	time.Sleep(100 * time.Millisecond)
	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients after disconnect, got %d", hub.ClientCount())
	}
}
