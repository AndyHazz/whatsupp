package hub

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/andyhazz/whatsupp/internal/alerting"
	"github.com/andyhazz/whatsupp/internal/checks"
	"github.com/andyhazz/whatsupp/internal/config"
)

func TestHub_ProcessResult_DownTransition(t *testing.T) {
	s := testStore(t)

	// Capture ntfy messages
	var alertCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		alertCount++
		w.WriteHeader(200)
	}))
	defer srv.Close()

	ntfyClient := alerting.NewNtfyClient(alerting.NtfyConfig{
		URL:              srv.URL,
		Topic:            "test",
		ReminderInterval: time.Hour,
	})

	h := &Hub{
		store:           s,
		alerter:         ntfyClient,
		incidentManager: NewIncidentManager(s),
		monitorStates:   make(map[string]*MonitorState),
		cfg:             &config.Config{},
	}
	h.monitorStates["Plex"] = NewMonitorState("Plex", 2)

	// Two failures should trigger DOWN + incident + alert
	for i := 0; i < 2; i++ {
		h.processResult(checks.Result{Monitor: "Plex", Status: "down", Error: "timeout"})
	}

	if h.monitorStates["Plex"].Status != StatusDown {
		t.Errorf("Status = %v, want DOWN", h.monitorStates["Plex"].Status)
	}
	if alertCount != 1 {
		t.Errorf("alerts sent = %d, want 1", alertCount)
	}

	// Verify incident was created
	inc, _ := s.GetOpenIncident("Plex")
	if inc == nil {
		t.Error("no open incident after DOWN transition")
	}
}

func TestHub_ProcessResult_Recovery(t *testing.T) {
	s := testStore(t)

	var alertCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		alertCount++
		w.WriteHeader(200)
	}))
	defer srv.Close()

	ntfyClient := alerting.NewNtfyClient(alerting.NtfyConfig{
		URL:              srv.URL,
		Topic:            "test",
		ReminderInterval: time.Hour,
	})

	h := &Hub{
		store:           s,
		alerter:         ntfyClient,
		incidentManager: NewIncidentManager(s),
		monitorStates:   make(map[string]*MonitorState),
		cfg:             &config.Config{},
	}
	h.monitorStates["Plex"] = NewMonitorState("Plex", 2)

	// Drive to DOWN
	h.processResult(checks.Result{Monitor: "Plex", Status: "down", Error: "timeout"})
	h.processResult(checks.Result{Monitor: "Plex", Status: "down", Error: "timeout"})

	// Recover
	h.processResult(checks.Result{Monitor: "Plex", Status: "up", LatencyMs: 50})

	if h.monitorStates["Plex"].Status != StatusUp {
		t.Errorf("Status = %v, want UP after recovery", h.monitorStates["Plex"].Status)
	}
	// Should have sent DOWN + RECOVERY = 2 alerts
	if alertCount != 2 {
		t.Errorf("alerts sent = %d, want 2 (down + recovery)", alertCount)
	}

	// Incident should be resolved
	inc, _ := s.GetOpenIncident("Plex")
	if inc != nil {
		t.Error("incident should be resolved after recovery")
	}
}

func TestHub_NewFromConfig(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	cfg := &config.Config{
		Server: config.ServerConfig{
			Listen: ":8080",
			DBPath: dbPath,
		},
		Monitors: []config.Monitor{
			{Name: "Test", Type: "http", URL: "https://example.com", Interval: 60 * time.Second, FailureThreshold: 3},
		},
		Alerting: config.AlertingConfig{
			DefaultFailureThreshold: 3,
			Ntfy: config.NtfyConfig{
				URL:   "https://ntfy.example.com",
				Topic: "test",
			},
			Thresholds: config.ThresholdsConfig{
				DownReminderInterval: time.Hour,
			},
		},
		Retention: config.RetentionConfig{
			CheckResultsRaw: 720 * time.Hour,
			Hourly:          4320 * time.Hour,
		},
	}

	h, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer h.Close()

	if h.store == nil {
		t.Error("store is nil")
	}
	if h.alerter == nil {
		t.Error("alerter is nil")
	}
	if _, ok := h.monitorStates["Test"]; !ok {
		t.Error("monitor state not initialized for 'Test'")
	}
}
