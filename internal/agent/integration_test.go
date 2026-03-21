//go:build integration

package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAgent_EndToEnd(t *testing.T) {
	var mu sync.Mutex
	var received []MetricBatch

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/agent/metrics" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Error("missing bearer token")
		}

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
		AgentKey: "sk-integration-test",
		Hostname: "testhost",
		Interval: 200 * time.Millisecond,
	}

	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	a.Run(ctx)

	mu.Lock()
	defer mu.Unlock()

	if len(received) == 0 {
		t.Fatal("no batches received")
	}

	batch := received[0]
	if batch.Host != "testhost" {
		t.Errorf("host = %q, want %q", batch.Host, "testhost")
	}
	if batch.Timestamp.IsZero() {
		t.Error("timestamp is zero")
	}
	if len(batch.Metrics) == 0 {
		t.Error("metrics array is empty")
	}

	// Check for expected metrics
	names := make(map[string]bool)
	for _, m := range batch.Metrics {
		names[m.Name] = true
	}

	if !names["cpu.usage_pct"] {
		t.Error("missing cpu.usage_pct")
	}
	if !names["mem.usage_pct"] {
		t.Error("missing mem.usage_pct")
	}

	hasDisk := false
	for name := range names {
		if strings.HasPrefix(name, "disk.") {
			hasDisk = true
			break
		}
	}
	if !hasDisk {
		t.Error("missing disk metrics")
	}
}
