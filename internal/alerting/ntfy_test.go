package alerting

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

type capturedMessage struct {
	Topic    string `json:"topic"`
	Title    string `json:"title"`
	Message  string `json:"message"`
	Priority int    `json:"priority"`
	Tags     string `json:"tags"`
}

func captureServer(t *testing.T) (*httptest.Server, *[]capturedMessage, *sync.Mutex) {
	t.Helper()
	var msgs []capturedMessage
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var msg capturedMessage
		json.Unmarshal(body, &msg)
		mu.Lock()
		msgs = append(msgs, msg)
		mu.Unlock()
		w.WriteHeader(200)
	}))
	t.Cleanup(srv.Close)
	return srv, &msgs, &mu
}

func TestNtfyClient_SendDownAlert(t *testing.T) {
	srv, msgs, mu := captureServer(t)

	client := NewNtfyClient(NtfyConfig{
		URL:              srv.URL,
		Topic:            "test",
		ReminderInterval: time.Hour,
	})

	err := client.SendDown("Plex", "connection refused (3/3 failures)")
	if err != nil {
		t.Fatalf("SendDown() error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(*msgs) != 1 {
		t.Fatalf("messages sent = %d, want 1", len(*msgs))
	}
	if (*msgs)[0].Priority != 4 {
		t.Errorf("Priority = %d, want 4", (*msgs)[0].Priority)
	}
}

func TestNtfyClient_SendRecovery(t *testing.T) {
	srv, msgs, mu := captureServer(t)

	client := NewNtfyClient(NtfyConfig{
		URL:              srv.URL,
		Topic:            "test",
		ReminderInterval: time.Hour,
	})

	err := client.SendRecovery("Plex", "4m 32s")
	if err != nil {
		t.Fatalf("SendRecovery() error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(*msgs) != 1 {
		t.Fatalf("messages sent = %d, want 1", len(*msgs))
	}
	if (*msgs)[0].Priority != 3 {
		t.Errorf("Priority = %d, want 3", (*msgs)[0].Priority)
	}
}

func TestNtfyClient_Deduplication(t *testing.T) {
	srv, msgs, mu := captureServer(t)

	client := NewNtfyClient(NtfyConfig{
		URL:              srv.URL,
		Topic:            "test",
		ReminderInterval: time.Hour,
	})

	client.SendDown("Plex", "timeout")
	client.SendDown("Plex", "timeout")
	client.SendDown("Plex", "timeout")

	mu.Lock()
	defer mu.Unlock()
	if len(*msgs) != 1 {
		t.Errorf("messages sent = %d, want 1 (dedup should suppress duplicates)", len(*msgs))
	}
}

func TestNtfyClient_ReminderAfterInterval(t *testing.T) {
	srv, msgs, mu := captureServer(t)

	client := NewNtfyClient(NtfyConfig{
		URL:              srv.URL,
		Topic:            "test",
		ReminderInterval: 100 * time.Millisecond,
	})

	client.SendDown("Plex", "timeout")
	time.Sleep(150 * time.Millisecond)
	client.SendDown("Plex", "timeout")

	mu.Lock()
	defer mu.Unlock()
	if len(*msgs) != 2 {
		t.Errorf("messages sent = %d, want 2 (initial + reminder)", len(*msgs))
	}
}

func TestNtfyClient_RecoveryClearsDedup(t *testing.T) {
	srv, msgs, mu := captureServer(t)

	client := NewNtfyClient(NtfyConfig{
		URL:              srv.URL,
		Topic:            "test",
		ReminderInterval: time.Hour,
	})

	client.SendDown("Plex", "timeout")
	client.SendRecovery("Plex", "5m")
	client.SendDown("Plex", "timeout")

	mu.Lock()
	defer mu.Unlock()
	if len(*msgs) != 3 {
		t.Errorf("messages sent = %d, want 3 (down + recovery + new down)", len(*msgs))
	}
}

func TestNtfyClient_SecurityAlerts(t *testing.T) {
	srv, msgs, mu := captureServer(t)

	client := NewNtfyClient(NtfyConfig{
		URL:              srv.URL,
		Topic:            "test",
		ReminderInterval: time.Hour,
	})

	err := client.SendNewPort("84.18.245.85", 4444)
	if err != nil {
		t.Fatalf("SendNewPort() error: %v", err)
	}

	err = client.SendPortGone("84.18.245.85", 443)
	if err != nil {
		t.Fatalf("SendPortGone() error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(*msgs) != 2 {
		t.Fatalf("messages sent = %d, want 2", len(*msgs))
	}
	if (*msgs)[0].Priority != 5 {
		t.Errorf("NewPort priority = %d, want 5", (*msgs)[0].Priority)
	}
	if (*msgs)[1].Priority != 4 {
		t.Errorf("PortGone priority = %d, want 4", (*msgs)[1].Priority)
	}
}

func TestNtfyClient_SSLExpiryAlert(t *testing.T) {
	srv, msgs, mu := captureServer(t)

	client := NewNtfyClient(NtfyConfig{
		URL:              srv.URL,
		Topic:            "test",
		ReminderInterval: time.Hour,
	})

	err := client.SendSSLExpiry("example.com", 7)
	if err != nil {
		t.Fatalf("SendSSLExpiry() error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(*msgs) != 1 {
		t.Fatalf("messages sent = %d, want 1", len(*msgs))
	}
	if (*msgs)[0].Priority != 4 {
		t.Errorf("Priority = %d, want 4", (*msgs)[0].Priority)
	}
}

func TestNtfyClient_BasicAuth(t *testing.T) {
	var authHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := NewNtfyClient(NtfyConfig{
		URL:              srv.URL,
		Topic:            "test",
		Username:         "user",
		Password:         "pass",
		ReminderInterval: time.Hour,
	})

	client.SendDown("Test", "err")
	if authHeader == "" {
		t.Error("Authorization header should be set when username/password configured")
	}
}
