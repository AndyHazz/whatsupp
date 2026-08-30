package hub

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/andyhazz/whatsupp/internal/alerting"
	"github.com/andyhazz/whatsupp/internal/config"
)

// A permanent, legitimate change (a new port forward) must be reported once,
// not on every scan from then on.
func TestHub_ScanResult_DriftAlertsOnceThenIsAbsorbed(t *testing.T) {
	s := testStore(t)

	var alertCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		alertCount++
		w.WriteHeader(200)
	}))
	defer srv.Close()

	h := &Hub{
		store: s,
		alerter: alerting.NewNtfyClient(alerting.NtfyConfig{
			URL:              srv.URL,
			Topic:            "test",
			ReminderInterval: time.Hour,
		}),
		scanProgress: make(map[string]*ScanState),
		cfg:          &config.Config{},
	}

	// hysteresis 1 = pass-through, so this test isolates baseline absorption
	target := config.SecurityTarget{Host: "example.test", AlertHysteresis: 1}

	// First scan establishes the baseline; nothing to report yet.
	h.processScanResult(target, []int{80, 443}, 1000)
	if alertCount != 0 {
		t.Fatalf("after baseline scan: alerts = %d, want 0", alertCount)
	}

	// A new port appears. Report it once.
	h.processScanResult(target, []int{80, 443, 25566}, 2000)
	if alertCount != 1 {
		t.Fatalf("after new port appears: alerts = %d, want 1", alertCount)
	}

	// Nothing has changed since. It must not be reported again.
	h.processScanResult(target, []int{80, 443, 25566}, 3000)
	if alertCount != 1 {
		t.Errorf("after unchanged rescan: alerts = %d, want 1 (drift re-alerted)", alertCount)
	}
}

// A port that goes away is likewise reported once, then accepted.
func TestHub_ScanResult_GonePortAlertsOnceThenIsAbsorbed(t *testing.T) {
	s := testStore(t)

	var alertCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		alertCount++
		w.WriteHeader(200)
	}))
	defer srv.Close()

	h := &Hub{
		store: s,
		alerter: alerting.NewNtfyClient(alerting.NtfyConfig{
			URL:              srv.URL,
			Topic:            "test",
			ReminderInterval: time.Hour,
		}),
		scanProgress: make(map[string]*ScanState),
		cfg:          &config.Config{},
	}

	target := config.SecurityTarget{Host: "example.test", AlertHysteresis: 1}

	h.processScanResult(target, []int{80, 443, 8080}, 1000)
	h.processScanResult(target, []int{80, 443}, 2000)
	if alertCount != 1 {
		t.Fatalf("after port disappears: alerts = %d, want 1", alertCount)
	}

	h.processScanResult(target, []int{80, 443}, 3000)
	if alertCount != 1 {
		t.Errorf("after unchanged rescan: alerts = %d, want 1 (drift re-alerted)", alertCount)
	}
}

// Drift that hysteresis suppresses must NOT be absorbed, or a transient flap
// would silently become the new normal without ever being reported.
func TestHub_ScanResult_SuppressedDriftIsNotAbsorbed(t *testing.T) {
	s := testStore(t)

	var alertCount int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		alertCount++
		w.WriteHeader(200)
	}))
	defer srv.Close()

	h := &Hub{
		store: s,
		alerter: alerting.NewNtfyClient(alerting.NtfyConfig{
			URL:              srv.URL,
			Topic:            "test",
			ReminderInterval: time.Hour,
		}),
		scanProgress: make(map[string]*ScanState),
		cfg:          &config.Config{},
	}

	// hysteresis 2: a change must show up in two consecutive scans to alert
	target := config.SecurityTarget{Host: "example.test", AlertHysteresis: 2}

	h.processScanResult(target, []int{80, 443}, 1000)

	// 4444 appears in one scan only, then goes away: suppressed, not alerted.
	h.processScanResult(target, []int{80, 443, 4444}, 2000)
	if alertCount != 0 {
		t.Fatalf("single-scan flap: alerts = %d, want 0", alertCount)
	}

	// Baseline must still be the original, so 4444 alerts if it later persists.
	baseline, err := s.GetSecurityBaseline("example.test")
	if err != nil {
		t.Fatalf("GetSecurityBaseline error: %v", err)
	}
	if baseline.ExpectedPortsJSON != "[80,443]" {
		t.Errorf("baseline = %s, want [80,443] (suppressed flap was absorbed)", baseline.ExpectedPortsJSON)
	}
}
