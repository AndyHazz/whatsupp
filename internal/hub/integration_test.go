//go:build integration

package hub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/andyhazz/whatsupp/internal/alerting"
)

func TestStaleness_EndToEnd(t *testing.T) {
	s := testStore(t)

	var mu sync.Mutex
	var alerts []string

	// Mock ntfy server
	ntfySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg struct {
			Title   string `json:"title"`
			Message string `json:"message"`
		}
		json.NewDecoder(r.Body).Decode(&msg)
		mu.Lock()
		alerts = append(alerts, msg.Title)
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer ntfySrv.Close()

	ntfy := alerting.NewNtfyClient(alerting.NtfyConfig{
		URL:   ntfySrv.URL,
		Topic: "test",
	})

	now := time.Now()

	// Insert a stale heartbeat (10 minutes ago)
	s.UpsertHeartbeat("plexypi", now.Add(-10*time.Minute))

	// Run staleness check
	sc := NewStalenessChecker(s, ntfy, 5*time.Minute)
	err := sc.Check(context.Background())
	if err != nil {
		t.Fatalf("Check error: %v", err)
	}

	// Verify incident created
	inc, _ := s.GetOpenIncidentForMonitor("agent:plexypi")
	if inc == nil {
		t.Fatal("expected open incident for stale agent")
	}
	if !strings.Contains(inc.Cause, "no metrics from plexypi") {
		t.Errorf("cause = %q, want to contain 'no metrics from plexypi'", inc.Cause)
	}

	// Verify alert sent
	mu.Lock()
	if len(alerts) != 1 {
		t.Errorf("expected 1 alert, got %d", len(alerts))
	}
	if len(alerts) > 0 && !strings.Contains(alerts[0], "DOWN") {
		t.Errorf("expected DOWN alert, got %q", alerts[0])
	}
	mu.Unlock()

	// Now update heartbeat (agent comes back)
	s.UpsertHeartbeat("plexypi", now)

	// Run staleness check again
	err = sc.Check(context.Background())
	if err != nil {
		t.Fatalf("Check error: %v", err)
	}

	// Verify incident resolved
	inc, _ = s.GetOpenIncidentForMonitor("agent:plexypi")
	if inc != nil {
		t.Error("incident should be resolved after recovery")
	}

	// Verify recovery alert sent
	mu.Lock()
	if len(alerts) != 2 {
		t.Errorf("expected 2 alerts (down + recovery), got %d", len(alerts))
	}
	if len(alerts) > 1 && !strings.Contains(alerts[1], "UP") {
		t.Errorf("expected UP alert, got %q", alerts[1])
	}
	mu.Unlock()
}
