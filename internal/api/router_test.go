package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// NewTestRouter creates a router with mock dependencies for testing.
func NewTestRouter(t *testing.T) http.Handler {
	t.Helper()
	result := NewRouter(RouterConfig{
		Store:      &mockStore{},
		Hub:        &mockHubState{statuses: map[string]MonitorStatus{}},
		ConfigPath: "/dev/null",
		AgentKeys:  map[string]string{"test-host": "abc123"},
		BackupDir:  "",
	})
	return result.Handler
}

func TestRouter_HealthIsPublic(t *testing.T) {
	r := NewTestRouter(t)
	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestRouter_MonitorsRequiresAuth(t *testing.T) {
	r := NewTestRouter(t)
	req := httptest.NewRequest("GET", "/api/v1/monitors", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestRouter_LoginIsPublic(t *testing.T) {
	r := NewTestRouter(t)
	body := `{"username":"admin","password":"wrong"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	// Should get 401 (bad creds) not 404 — route exists
	if rr.Code == http.StatusNotFound {
		t.Error("login route should exist")
	}
}

func TestRouter_AgentMetricsRequiresBearerToken(t *testing.T) {
	r := NewTestRouter(t)
	req := httptest.NewRequest("POST", "/api/v1/agent/metrics", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}
