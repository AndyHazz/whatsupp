package hub

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/andyhazz/whatsupp/internal/alerting"
)

func TestStalenessCheck_NoStaleAgents(t *testing.T) {
	s := testStore(t)

	now := time.Now()
	s.UpsertHeartbeat("host1", now)
	s.UpsertHeartbeat("host2", now)

	sc := NewStalenessChecker(s, nil, 5*time.Minute)
	err := sc.Check(context.Background())
	if err != nil {
		t.Fatalf("Check error: %v", err)
	}

	// No incidents should be created
	incidents, _ := s.GetIncidents(now.Add(-time.Hour).Unix(), now.Add(time.Hour).Unix())
	if len(incidents) != 0 {
		t.Errorf("expected 0 incidents, got %d", len(incidents))
	}
}

func TestStalenessCheck_OneStale(t *testing.T) {
	s := testStore(t)

	now := time.Now()
	s.UpsertHeartbeat("fresh", now)
	s.UpsertHeartbeat("stale", now.Add(-10*time.Minute))

	var mu sync.Mutex
	var alertCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		alertCount++
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	ntfy := alerting.NewNtfyClient(alerting.NtfyConfig{URL: srv.URL, Topic: "test"})
	sc := NewStalenessChecker(s, ntfy, 5*time.Minute)
	sc.Check(context.Background())

	// Should have one incident for "agent:stale"
	inc, _ := s.GetOpenIncidentForMonitor("agent:stale")
	if inc == nil {
		t.Fatal("expected open incident for stale agent")
	}

	mu.Lock()
	if alertCount != 1 {
		t.Errorf("alert count = %d, want 1", alertCount)
	}
	mu.Unlock()
}

func TestStalenessCheck_AlreadyIncident(t *testing.T) {
	s := testStore(t)

	now := time.Now()
	s.UpsertHeartbeat("stale", now.Add(-10*time.Minute))

	// Pre-create incident
	s.CreateIncident("agent:stale", now.Add(-5*time.Minute).Unix(), "already stale")

	sc := NewStalenessChecker(s, nil, 5*time.Minute)
	sc.Check(context.Background())

	// Should not create a duplicate incident
	incidents, _ := s.GetIncidents(now.Add(-time.Hour).Unix(), now.Add(time.Hour).Unix())
	count := 0
	for _, inc := range incidents {
		if inc.Monitor == "agent:stale" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 incident for agent:stale, got %d", count)
	}
}

func TestStalenessCheck_Recovery(t *testing.T) {
	s := testStore(t)

	now := time.Now()

	// Create stale heartbeat and incident
	s.UpsertHeartbeat("host1", now.Add(-10*time.Minute))
	s.CreateIncident("agent:host1", now.Add(-5*time.Minute).Unix(), "stale")

	// Now update heartbeat to current
	s.UpsertHeartbeat("host1", now)

	var mu sync.Mutex
	var alertCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		alertCount++
		mu.Unlock()
		w.WriteHeader(200)
	}))
	defer srv.Close()

	ntfy := alerting.NewNtfyClient(alerting.NtfyConfig{URL: srv.URL, Topic: "test"})
	sc := NewStalenessChecker(s, ntfy, 5*time.Minute)
	sc.Check(context.Background())

	// Incident should be resolved
	inc, _ := s.GetOpenIncidentForMonitor("agent:host1")
	if inc != nil {
		t.Error("incident should be resolved after recovery")
	}

	mu.Lock()
	if alertCount != 1 {
		t.Errorf("alert count = %d, want 1 (recovery)", alertCount)
	}
	mu.Unlock()
}

func TestStalenessCheck_ConfigurableThreshold(t *testing.T) {
	s := testStore(t)

	now := time.Now()
	// 3 minutes ago - would be stale with 2min threshold but not with 5min
	s.UpsertHeartbeat("host1", now.Add(-3*time.Minute))

	sc := NewStalenessChecker(s, nil, 2*time.Minute)
	sc.Check(context.Background())

	inc, _ := s.GetOpenIncidentForMonitor("agent:host1")
	if inc == nil {
		t.Error("expected incident with 2-minute threshold")
	}

	// With 5-minute threshold, should not be stale
	s2 := testStore(t)
	s2.UpsertHeartbeat("host1", now.Add(-3*time.Minute))
	sc2 := NewStalenessChecker(s2, nil, 5*time.Minute)
	sc2.Check(context.Background())

	inc2, _ := s2.GetOpenIncidentForMonitor("agent:host1")
	if inc2 != nil {
		t.Error("expected no incident with 5-minute threshold")
	}
}
