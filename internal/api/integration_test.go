package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestIntegration_LoginAndAccessMonitors(t *testing.T) {
	// Setup: create store with admin user
	store := newTestStore(t)
	hash, _ := HashPassword("admin123")
	store.CreateUser("admin", hash)

	hub := &mockHubState{
		statuses: map[string]MonitorStatus{
			"Plex": {Name: "Plex", Type: "http", Status: "up", LatencyMs: 45.2},
		},
	}

	result := NewRouter(RouterConfig{
		Store:      store,
		Hub:        hub,
		ConfigPath: "/dev/null",
		AgentKeys:  map[string]string{},
	})

	server := httptest.NewServer(result.Handler)
	defer server.Close()

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Step 1: Try monitors without auth -> 401
	resp, err := client.Get(server.URL + "/api/v1/monitors")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}

	// Step 2: Login
	loginBody := `{"username":"admin","password":"admin123"}`
	resp, err = client.Post(server.URL+"/api/v1/auth/login", "application/json", strings.NewReader(loginBody))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login expected 200, got %d", resp.StatusCode)
	}

	// Extract session cookie
	var sessionCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatal("expected session cookie after login")
	}

	// Step 3: Access monitors with session cookie -> 200
	req, _ := http.NewRequest("GET", server.URL+"/api/v1/monitors", nil)
	req.AddCookie(sessionCookie)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with session, got %d", resp.StatusCode)
	}

	var monitors []MonitorStatus
	json.NewDecoder(resp.Body).Decode(&monitors)
	if len(monitors) != 1 {
		t.Fatalf("expected 1 monitor, got %d", len(monitors))
	}
	if monitors[0].Name != "Plex" {
		t.Errorf("expected Plex, got %q", monitors[0].Name)
	}

	// Step 4: Health is public
	resp, err = client.Get(server.URL + "/api/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("health expected 200, got %d", resp.StatusCode)
	}

	// Step 5: Logout
	req, _ = http.NewRequest("POST", server.URL+"/api/v1/auth/logout", nil)
	req.AddCookie(sessionCookie)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("logout expected 200, got %d", resp.StatusCode)
	}

	// Step 6: Session should be invalid after logout
	req, _ = http.NewRequest("GET", server.URL+"/api/v1/monitors", nil)
	req.AddCookie(sessionCookie)
	resp, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d", resp.StatusCode)
	}
}

func TestIntegration_WebSocketReceivesBroadcast(t *testing.T) {
	store := newTestStore(t)
	hash, _ := HashPassword("admin123")
	store.CreateUser("admin", hash)

	hub := &mockHubState{statuses: map[string]MonitorStatus{}}

	result := NewRouter(RouterConfig{
		Store:      store,
		Hub:        hub,
		ConfigPath: "/dev/null",
		AgentKeys:  map[string]string{},
	})

	server := httptest.NewServer(result.Handler)
	defer server.Close()

	// Login first to get session
	client := &http.Client{}
	loginBody := `{"username":"admin","password":"admin123"}`
	resp, _ := client.Post(server.URL+"/api/v1/auth/login", "application/json", strings.NewReader(loginBody))
	var sessionCookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "session" {
			sessionCookie = c
		}
	}

	if sessionCookie == nil {
		t.Fatal("no session cookie after login")
	}

	// Connect WebSocket with session cookie
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/ws"
	header := http.Header{}
	header.Add("Cookie", sessionCookie.String())
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, header)
	if err != nil {
		t.Fatalf("ws dial: %v", err)
	}
	defer conn.Close()

	// Give time to register
	time.Sleep(50 * time.Millisecond)

	// Broadcast via the WSHub exposed from the router
	result.WSHub.Broadcast(WSMessage{
		Type: "check_result",
		Data: map[string]interface{}{
			"monitor":    "Plex",
			"status":     "up",
			"latency_ms": 42.0,
		},
	})

	// Read the broadcast message
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	var msg WSMessage
	if err := conn.ReadJSON(&msg); err != nil {
		t.Fatalf("ws read: %v", err)
	}

	if msg.Type != "check_result" {
		t.Errorf("expected type check_result, got %q", msg.Type)
	}
	t.Log("WebSocket connection established with session auth and received broadcast")
}
