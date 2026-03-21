package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

// --- Mock types ---

type mockHubState struct {
	statuses map[string]MonitorStatus
}

func (m *mockHubState) MonitorStatuses() map[string]MonitorStatus { return m.statuses }
func (m *mockHubState) MonitorStatus(name string) (MonitorStatus, bool) {
	s, ok := m.statuses[name]
	return s, ok
}
func (m *mockHubState) ReloadConfig() error { return nil }

type mockStore struct {
	heartbeats        []AgentHeartbeat
	incidents         []Incident
	scans             []SecurityScan
	baselines         []SecurityBaseline
	baselineUpdated   bool
	insertedMetrics   []AgentMetricPoint
	insertedHost      string
	heartbeatUpdated  bool
	backupFunc        func(destPath string) error
	checkResults      []CheckResult
	checkSummary      []CheckResultSummary
	agentMetrics      []AgentMetric
	agentMetricSumm   []AgentMetricSummary
}

// UserStore
func (m *mockStore) GetUserByUsername(username string) (*User, error) { return nil, nil }
func (m *mockStore) CreateUser(username, passwordHash string) error   { return nil }
func (m *mockStore) UserCount() (int, error)                         { return 0, nil }

// SessionStoreRW
func (m *mockStore) GetSession(token string) (*Session, error)                    { return nil, nil }
func (m *mockStore) RenewSession(token string, expiresAt time.Time) error         { return nil }
func (m *mockStore) CreateSession(token string, userID int64, expiresAt time.Time) error { return nil }
func (m *mockStore) DeleteSession(token string) error                             { return nil }
func (m *mockStore) DeleteExpiredSessions() error                                 { return nil }

// MonitorStore
func (m *mockStore) GetCheckResults(monitor string, from, to time.Time) ([]CheckResult, error) {
	return m.checkResults, nil
}
func (m *mockStore) GetCheckResultsHourly(monitor string, from, to time.Time) ([]CheckResultSummary, error) {
	return m.checkSummary, nil
}
func (m *mockStore) GetCheckResultsDaily(monitor string, from, to time.Time) ([]CheckResultSummary, error) {
	return m.checkSummary, nil
}

// HostStore
func (m *mockStore) GetAgentHeartbeats() ([]AgentHeartbeat, error) { return m.heartbeats, nil }
func (m *mockStore) GetAgentHeartbeat(host string) (*AgentHeartbeat, error) {
	for _, hb := range m.heartbeats {
		if hb.Host == host {
			return &hb, nil
		}
	}
	return nil, nil
}
func (m *mockStore) GetAgentMetrics(host string, from, to time.Time, names []string) ([]AgentMetric, error) {
	return m.agentMetrics, nil
}
func (m *mockStore) GetAgentMetrics5Min(host string, from, to time.Time, names []string) ([]AgentMetricSummary, error) {
	return m.agentMetricSumm, nil
}
func (m *mockStore) GetAgentMetricsHourly(host string, from, to time.Time, names []string) ([]AgentMetricSummary, error) {
	return m.agentMetricSumm, nil
}
func (m *mockStore) GetAgentMetricsDaily(host string, from, to time.Time, names []string) ([]AgentMetricSummary, error) {
	return m.agentMetricSumm, nil
}
func (m *mockStore) InsertAgentMetrics(host string, timestamp time.Time, metrics []AgentMetricPoint) error {
	m.insertedHost = host
	m.insertedMetrics = metrics
	return nil
}
func (m *mockStore) UpdateAgentHeartbeat(host string, lastSeenAt time.Time) error {
	m.heartbeatUpdated = true
	return nil
}

// IncidentStore
func (m *mockStore) GetIncidents(from, to time.Time) ([]Incident, error) { return m.incidents, nil }

// SecurityStore
func (m *mockStore) GetSecurityScans() ([]SecurityScan, error)       { return m.scans, nil }
func (m *mockStore) GetSecurityBaselines() ([]SecurityBaseline, error) { return m.baselines, nil }
func (m *mockStore) UpdateSecurityBaseline(target string, portsJSON string, updatedAt time.Time) error {
	m.baselineUpdated = true
	return nil
}

// BackupStore
func (m *mockStore) Backup(destPath string) error {
	if m.backupFunc != nil {
		return m.backupFunc(destPath)
	}
	return nil
}

// --- Tests ---

func TestHealthHandler_ReturnsOK(t *testing.T) {
	h := &Handlers{}
	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	rr := httptest.NewRecorder()

	h.Health(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["status"] != "ok" {
		t.Errorf("expected status=ok, got %q", resp["status"])
	}
}

// Step 9: Monitor list and detail endpoints

func TestMonitorsHandler_ReturnsList(t *testing.T) {
	hub := &mockHubState{
		statuses: map[string]MonitorStatus{
			"Plex": {Name: "Plex", Type: "http", Status: "up", LatencyMs: 45.2, LastCheck: 1711018800},
			"VPN":  {Name: "VPN", Type: "ping", Status: "down", LatencyMs: 0, LastCheck: 1711018800},
		},
	}
	h := &Handlers{hub: hub}

	req := httptest.NewRequest("GET", "/api/v1/monitors", nil)
	rr := httptest.NewRecorder()
	h.ListMonitors(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var monitors []MonitorStatus
	json.NewDecoder(rr.Body).Decode(&monitors)
	if len(monitors) != 2 {
		t.Errorf("expected 2 monitors, got %d", len(monitors))
	}
}

func TestMonitorDetailHandler_Found(t *testing.T) {
	hub := &mockHubState{
		statuses: map[string]MonitorStatus{
			"Plex": {Name: "Plex", Type: "http", Status: "up", LatencyMs: 45.2},
		},
	}
	h := &Handlers{hub: hub}

	// Using chi URL param — in test, set via chi context
	req := httptest.NewRequest("GET", "/api/v1/monitors/Plex", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "Plex")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.GetMonitor(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestMonitorDetailHandler_NotFound(t *testing.T) {
	hub := &mockHubState{statuses: map[string]MonitorStatus{}}
	h := &Handlers{hub: hub}

	req := httptest.NewRequest("GET", "/api/v1/monitors/NonExistent", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("name", "NonExistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.GetMonitor(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

// Step 10: Tier selection tests

func TestSelectCheckResultTier(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"1 hour → raw", 1 * time.Hour, "raw"},
		{"48 hours → raw", 48 * time.Hour, "raw"},
		{"49 hours → hourly", 49 * time.Hour, "hourly"},
		{"30 days → hourly", 30 * 24 * time.Hour, "hourly"},
		{"31 days → daily", 31 * 24 * time.Hour, "daily"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectCheckResultTier(tt.duration)
			if got != tt.want {
				t.Errorf("selectCheckResultTier(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

func TestSelectAgentMetricTier(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"1 hour → raw", 1 * time.Hour, "raw"},
		{"48 hours → raw", 48 * time.Hour, "raw"},
		{"3 days → 5min", 3 * 24 * time.Hour, "5min"},
		{"7 days → 5min", 7 * 24 * time.Hour, "5min"},
		{"8 days → hourly", 8 * 24 * time.Hour, "hourly"},
		{"90 days → hourly", 90 * 24 * time.Hour, "hourly"},
		{"91 days → daily", 91 * 24 * time.Hour, "daily"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectAgentMetricTier(tt.duration)
			if got != tt.want {
				t.Errorf("selectAgentMetricTier(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

// Step 11: Host endpoints

func TestListHosts_ReturnsHeartbeats(t *testing.T) {
	store := &mockStore{
		heartbeats: []AgentHeartbeat{
			{Host: "plexypi", LastSeenAt: 1711018800},
			{Host: "dietpi", LastSeenAt: 1711018700},
		},
	}
	h := &Handlers{store: store}

	req := httptest.NewRequest("GET", "/api/v1/hosts", nil)
	rr := httptest.NewRecorder()
	h.ListHosts(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var hosts []AgentHeartbeat
	json.NewDecoder(rr.Body).Decode(&hosts)
	if len(hosts) != 2 {
		t.Errorf("expected 2 hosts, got %d", len(hosts))
	}
}

// Step 12: Incident list endpoint

func TestListIncidents_ReturnsList(t *testing.T) {
	store := &mockStore{
		incidents: []Incident{
			{ID: 1, Monitor: "Plex", StartedAt: 1711018800, Cause: "connection refused"},
		},
	}
	h := &Handlers{store: store}

	req := httptest.NewRequest("GET", "/api/v1/incidents?from=1711000000&to=1711099999", nil)
	rr := httptest.NewRecorder()
	h.ListIncidents(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// Step 13: Security scan and baseline endpoints

func TestListSecurityScans(t *testing.T) {
	store := &mockStore{
		scans: []SecurityScan{
			{ID: 1, Target: "84.18.245.85", Timestamp: 1711018800, OpenPortsJSON: "[443,8443]"},
		},
	}
	h := &Handlers{store: store}

	req := httptest.NewRequest("GET", "/api/v1/security/scans", nil)
	rr := httptest.NewRecorder()
	h.ListSecurityScans(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestUpdateSecurityBaseline(t *testing.T) {
	store := &mockStore{
		scans: []SecurityScan{
			{ID: 1, Target: "84.18.245.85", Timestamp: 1711018800, OpenPortsJSON: "[443,8443]"},
		},
	}
	h := &Handlers{store: store}

	req := httptest.NewRequest("POST", "/api/v1/security/baselines/84.18.245.85", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("target", "84.18.245.85")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	h.AcceptBaseline(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

// Step 14: Config GET/PUT endpoints

func TestGetConfig_ReturnsYAML(t *testing.T) {
	// Create a temp config file
	tmpFile, err := os.CreateTemp("", "whatsupp-config-*.yml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("server:\n  listen: ':8080'\n")
	tmpFile.Close()

	h := &Handlers{configPath: tmpFile.Name()}

	req := httptest.NewRequest("GET", "/api/v1/config", nil)
	rr := httptest.NewRecorder()
	h.GetConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	ct := rr.Header().Get("Content-Type")
	if ct != "application/x-yaml" {
		t.Errorf("expected Content-Type application/x-yaml, got %q", ct)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "listen") {
		t.Errorf("expected config content, got %q", body)
	}
}

func TestPutConfig_WritesAndReloads(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "whatsupp-config-*.yml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString("server:\n  listen: ':8080'\n")
	tmpFile.Close()

	hub := &mockHubState{statuses: map[string]MonitorStatus{}}
	h := &Handlers{configPath: tmpFile.Name(), hub: hub}

	newConfig := "server:\n  listen: ':9090'\n"
	req := httptest.NewRequest("PUT", "/api/v1/config", strings.NewReader(newConfig))
	req.Header.Set("Content-Type", "application/x-yaml")
	rr := httptest.NewRecorder()
	h.PutConfig(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	// Verify file was written
	data, _ := os.ReadFile(tmpFile.Name())
	if string(data) != newConfig {
		t.Errorf("config file not updated: got %q", string(data))
	}
}

// Step 15: Admin backup endpoint

func TestBackupHandler_ReturnsFile(t *testing.T) {
	tmpDir := t.TempDir()
	store := &mockStore{
		backupFunc: func(destPath string) error {
			// Simulate backup by creating a file
			return os.WriteFile(destPath, []byte("fake-backup-data"), 0644)
		},
	}
	h := &Handlers{store: store}
	h.backupDir = tmpDir

	req := httptest.NewRequest("GET", "/api/v1/admin/backup", nil)
	rr := httptest.NewRecorder()
	h.Backup(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	ct := rr.Header().Get("Content-Type")
	if ct != "application/octet-stream" {
		t.Errorf("expected application/octet-stream, got %q", ct)
	}

	cd := rr.Header().Get("Content-Disposition")
	if !strings.HasPrefix(cd, "attachment; filename=") {
		t.Errorf("expected Content-Disposition attachment, got %q", cd)
	}
}

// Step 16: Agent metrics POST endpoint

func TestAgentMetrics_ValidPayload(t *testing.T) {
	store := &mockStore{}
	wsHub := NewWSHub()
	go wsHub.Run()
	defer wsHub.Stop()

	h := &Handlers{store: store, wsHub: wsHub}

	payload := `{
		"host": "plexypi",
		"timestamp": "2026-03-21T12:00:00Z",
		"metrics": [
			{"name": "cpu.usage_pct", "value": 23.5},
			{"name": "mem.usage_pct", "value": 52.1}
		]
	}`
	req := httptest.NewRequest("POST", "/api/v1/agent/metrics", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.PostAgentMetrics(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d; body: %s", rr.Code, rr.Body.String())
	}
}

func TestAgentMetrics_EmptyBody_Returns400(t *testing.T) {
	h := &Handlers{store: &mockStore{}}

	req := httptest.NewRequest("POST", "/api/v1/agent/metrics", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	h.PostAgentMetrics(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}
