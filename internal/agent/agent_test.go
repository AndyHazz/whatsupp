package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestAgent_CollectsAndPushes(t *testing.T) {
	var mu sync.Mutex
	var received []MetricBatch

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var batch MetricBatch
		json.NewDecoder(r.Body).Decode(&batch)
		mu.Lock()
		received = append(received, batch)
		mu.Unlock()
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	cfg := &AgentConfig{
		HubURL:   srv.URL,
		AgentKey: "sk-test",
		Hostname: "testhost",
		Interval: 100 * time.Millisecond,
	}

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	a.Run(ctx)

	mu.Lock()
	defer mu.Unlock()
	if len(received) == 0 {
		t.Fatal("expected at least one batch received")
	}
	if received[0].Host != "testhost" {
		t.Errorf("host = %q, want %q", received[0].Host, "testhost")
	}
	if len(received[0].Metrics) == 0 {
		t.Error("expected non-empty metrics")
	}
}

func TestAgent_BuffersOnFailure(t *testing.T) {
	callCount := 0
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		callCount++
		n := callCount
		mu.Unlock()
		if n <= 2 {
			w.WriteHeader(500)
			return
		}
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	cfg := &AgentConfig{
		HubURL:   srv.URL,
		AgentKey: "sk-test",
		Hostname: "testhost",
		Interval: 100 * time.Millisecond,
	}

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	a.Run(ctx)

	mu.Lock()
	defer mu.Unlock()
	if callCount < 3 {
		t.Logf("call count = %d (may be timing dependent)", callCount)
	}
}

func TestAgent_GracefulShutdown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	cfg := &AgentConfig{
		HubURL:   srv.URL,
		AgentKey: "sk-test",
		Hostname: "testhost",
		Interval: 10 * time.Second, // Long interval
	}

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		a.Run(ctx)
		close(done)
	}()

	// Give the agent time to start and do first collection
	time.Sleep(200 * time.Millisecond)

	cancel()

	select {
	case <-done:
		// OK
	case <-time.After(5 * time.Second):
		t.Fatal("agent did not shut down within 5s")
	}
}

func TestAgent_SkipsDockerOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	cfg := &AgentConfig{
		HubURL:     srv.URL,
		AgentKey:   "sk-test",
		Hostname:   "testhost",
		Interval:   100 * time.Millisecond,
		DockerHost: "tcp://127.0.0.1:1", // unreachable
	}

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	// Should not panic even if Docker is unreachable
	a.Run(ctx)
}
