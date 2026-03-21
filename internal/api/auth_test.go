package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/andyhazz/whatsupp/internal/store"
)

// testStoreAdapter wraps *store.Store to satisfy the api.Store interface.
// It adapts between store.Session (int64 timestamps) and api.Session (time.Time).
type testStoreAdapter struct {
	s *store.Store
}

func newTestStore(t *testing.T) *testStoreAdapter {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() {
		s.Close()
		os.Remove(dbPath)
	})
	return &testStoreAdapter{s: s}
}

// UserStore methods
func (a *testStoreAdapter) CreateUser(username, passwordHash string) error {
	return a.s.CreateUser(username, passwordHash)
}

func (a *testStoreAdapter) GetUserByUsername(username string) (*User, error) {
	u, err := a.s.GetUserByUsername(username)
	if err != nil || u == nil {
		return nil, err
	}
	return &User{ID: u.ID, Username: u.Username, PasswordHash: u.PasswordHash}, nil
}

func (a *testStoreAdapter) GetUserByID(id int64) (*User, error) {
	u, err := a.s.GetUserByID(id)
	if err != nil || u == nil {
		return nil, err
	}
	return &User{ID: u.ID, Username: u.Username, PasswordHash: u.PasswordHash}, nil
}

func (a *testStoreAdapter) UpdateUserPassword(id int64, passwordHash string) error {
	return a.s.UpdateUserPassword(id, passwordHash)
}

func (a *testStoreAdapter) UserCount() (int, error) {
	return a.s.UserCount()
}

// SessionStore methods
func (a *testStoreAdapter) GetSession(token string) (*Session, error) {
	sess, err := a.s.GetSession(token)
	if err != nil || sess == nil {
		return nil, err
	}
	return &Session{
		Token:     sess.Token,
		UserID:    sess.UserID,
		ExpiresAt: time.Unix(sess.ExpiresAt, 0),
	}, nil
}

func (a *testStoreAdapter) RenewSession(token string, expiresAt time.Time) error {
	return a.s.RenewSession(token, expiresAt)
}

func (a *testStoreAdapter) CreateSession(token string, userID int64, expiresAt time.Time) error {
	return a.s.CreateSession(token, userID, expiresAt)
}

func (a *testStoreAdapter) DeleteSession(token string) error {
	return a.s.DeleteSession(token)
}

func (a *testStoreAdapter) DeleteExpiredSessions() error {
	return a.s.DeleteExpiredSessions()
}

// Stub methods for interfaces not needed in auth tests
func (a *testStoreAdapter) GetCheckResults(monitor string, from, to time.Time) ([]CheckResult, error) {
	return nil, nil
}
func (a *testStoreAdapter) GetCheckResultsHourly(monitor string, from, to time.Time) ([]CheckResultSummary, error) {
	return nil, nil
}
func (a *testStoreAdapter) GetCheckResultsDaily(monitor string, from, to time.Time) ([]CheckResultSummary, error) {
	return nil, nil
}
func (a *testStoreAdapter) GetAgentHeartbeats() ([]AgentHeartbeat, error) { return nil, nil }
func (a *testStoreAdapter) GetAgentHeartbeat(host string) (*AgentHeartbeat, error) {
	return nil, nil
}
func (a *testStoreAdapter) GetAgentMetrics(host string, from, to time.Time, names []string) ([]AgentMetric, error) {
	return nil, nil
}
func (a *testStoreAdapter) GetAgentMetrics5Min(host string, from, to time.Time, names []string) ([]AgentMetricSummary, error) {
	return nil, nil
}
func (a *testStoreAdapter) GetAgentMetricsHourly(host string, from, to time.Time, names []string) ([]AgentMetricSummary, error) {
	return nil, nil
}
func (a *testStoreAdapter) GetAgentMetricsDaily(host string, from, to time.Time, names []string) ([]AgentMetricSummary, error) {
	return nil, nil
}
func (a *testStoreAdapter) InsertAgentMetrics(host string, timestamp time.Time, metrics []AgentMetricPoint) error {
	return nil
}
func (a *testStoreAdapter) UpdateAgentHeartbeat(host string, lastSeenAt time.Time) error {
	return nil
}
func (a *testStoreAdapter) UpdateAgentVersion(host string, version string) error { return nil }
func (a *testStoreAdapter) GetIncidents(from, to time.Time) ([]Incident, error) { return nil, nil }
func (a *testStoreAdapter) GetSecurityScans() ([]SecurityScan, error)           { return nil, nil }
func (a *testStoreAdapter) GetSecurityBaselines() ([]SecurityBaseline, error)    { return nil, nil }
func (a *testStoreAdapter) UpdateSecurityBaseline(target string, portsJSON string, updatedAt time.Time) error {
	return nil
}
func (a *testStoreAdapter) Backup(destPath string) error { return nil }

// --- Password hashing tests ---

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("testpass123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if hash == "testpass123" {
		t.Fatal("hash should not equal plaintext")
	}
}

func TestCheckPassword(t *testing.T) {
	hash, _ := HashPassword("correct-horse")
	if !CheckPassword(hash, "correct-horse") {
		t.Error("expected true for correct password")
	}
	if CheckPassword(hash, "wrong-password") {
		t.Error("expected false for wrong password")
	}
}

func TestGenerateSessionToken(t *testing.T) {
	tok1, err := GenerateSessionToken()
	if err != nil {
		t.Fatalf("GenerateSessionToken: %v", err)
	}
	if len(tok1) != 64 { // 32 bytes = 64 hex chars
		t.Fatalf("expected 64 char token, got %d", len(tok1))
	}
	tok2, _ := GenerateSessionToken()
	if tok1 == tok2 {
		t.Error("tokens should be unique")
	}
}

// --- Rate limiter tests ---

func TestRateLimiter_AllowsUnderLimit(t *testing.T) {
	rl := NewLoginRateLimiter()
	for i := 0; i < 5; i++ {
		if !rl.Allow("1.2.3.4") {
			t.Fatalf("attempt %d should be allowed", i+1)
		}
	}
}

func TestRateLimiter_BlocksOverMinuteLimit(t *testing.T) {
	rl := NewLoginRateLimiter()
	for i := 0; i < 5; i++ {
		rl.Allow("1.2.3.4")
	}
	if rl.Allow("1.2.3.4") {
		t.Error("6th attempt within a minute should be blocked")
	}
}

func TestRateLimiter_LocksOutAfterTenFailures(t *testing.T) {
	rl := NewLoginRateLimiter()
	for i := 0; i < 10; i++ {
		rl.RecordFailure("1.2.3.4")
	}
	if rl.Allow("1.2.3.4") {
		t.Error("should be locked out after 10 failures")
	}
}

func TestRateLimiter_DifferentIPsAreIndependent(t *testing.T) {
	rl := NewLoginRateLimiter()
	for i := 0; i < 10; i++ {
		rl.RecordFailure("1.2.3.4")
	}
	if !rl.Allow("5.6.7.8") {
		t.Error("different IP should not be affected")
	}
}

// --- Login handler tests ---

func TestLoginHandler_ValidCredentials_SetsCookie(t *testing.T) {
	st := newTestStore(t)
	hash, _ := HashPassword("admin123")
	st.CreateUser("admin", hash)

	rl := NewLoginRateLimiter()
	h := &Handlers{store: st, rateLimiter: rl}

	body := `{"username":"admin","password":"admin123"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Login(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	cookies := rr.Result().Cookies()
	found := false
	for _, c := range cookies {
		if c.Name == "session" && c.Value != "" {
			found = true
			if !c.HttpOnly {
				t.Error("session cookie must be HttpOnly")
			}
			if c.SameSite != http.SameSiteStrictMode {
				t.Error("session cookie must be SameSite=Strict")
			}
		}
	}
	if !found {
		t.Error("expected session cookie to be set")
	}
}

func TestLoginHandler_BadPassword_Returns401(t *testing.T) {
	st := newTestStore(t)
	hash, _ := HashPassword("admin123")
	st.CreateUser("admin", hash)

	rl := NewLoginRateLimiter()
	h := &Handlers{store: st, rateLimiter: rl}

	body := `{"username":"admin","password":"wrong"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	h.Login(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestLoginHandler_RateLimited_Returns429(t *testing.T) {
	st := newTestStore(t)
	hash, _ := HashPassword("admin123")
	st.CreateUser("admin", hash)

	rl := NewLoginRateLimiter()
	h := &Handlers{store: st, rateLimiter: rl}

	// Exhaust rate limit
	for i := 0; i < 5; i++ {
		body := `{"username":"admin","password":"wrong"}`
		req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "1.2.3.4:12345"
		rr := httptest.NewRecorder()
		h.Login(rr, req)
	}

	// 6th attempt should be rate limited
	body := `{"username":"admin","password":"admin123"}`
	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "1.2.3.4:12345"
	rr := httptest.NewRecorder()
	h.Login(rr, req)

	if rr.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rr.Code)
	}
}

// --- EnsureAdminUser tests ---

func TestEnsureAdminUser_CreatesUserWhenNoneExist(t *testing.T) {
	st := newTestStore(t)

	err := EnsureAdminUser(st, "admin", "secretpass")
	if err != nil {
		t.Fatalf("EnsureAdminUser: %v", err)
	}

	user, err := st.GetUserByUsername("admin")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if user == nil {
		t.Fatal("expected user to be created")
	}
	if !CheckPassword(user.PasswordHash, "secretpass") {
		t.Error("password should match")
	}
}

func TestEnsureAdminUser_SkipsWhenUsersExist(t *testing.T) {
	st := newTestStore(t)
	hash, _ := HashPassword("existing")
	st.CreateUser("existing-admin", hash)

	err := EnsureAdminUser(st, "admin", "secretpass")
	if err != nil {
		t.Fatalf("EnsureAdminUser: %v", err)
	}

	// Should NOT have created a second user
	user, _ := st.GetUserByUsername("admin")
	if user != nil {
		t.Error("should not create admin when users already exist")
	}
}

func TestSessionCleanup_DeletesExpired(t *testing.T) {
	store := newTestStore(t)
	// Create a user first (sessions reference users via FK)
	hash, _ := HashPassword("test")
	store.CreateUser("testuser", hash)
	// Create an expired session
	store.CreateSession("expired", 1, time.Now().Add(-time.Hour))
	// Create a valid session
	store.CreateSession("valid", 1, time.Now().Add(time.Hour))

	store.DeleteExpiredSessions()

	// Verify expired is gone
	sess, err := store.GetSession("expired")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if sess != nil {
		t.Error("expired session should have been deleted")
	}

	// Verify valid session still exists
	sess, err = store.GetSession("valid")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if sess == nil {
		t.Error("valid session should still exist")
	}
}
