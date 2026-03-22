package api

import (
	"testing"
	"time"
)

// mockStore implements the Store interface for testing.
type mockStore struct {
	users             map[string]*User
	sessions          map[string]*Session
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

// UserStore methods
func (m *mockStore) GetUserByUsername(username string) (*User, error) {
	if m.users == nil {
		return nil, nil
	}
	u, ok := m.users[username]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (m *mockStore) CreateUser(username, passwordHash string) error {
	if m.users == nil {
		m.users = make(map[string]*User)
	}
	m.users[username] = &User{ID: int64(len(m.users) + 1), Username: username, PasswordHash: passwordHash}
	return nil
}

func (m *mockStore) GetUserByID(id int64) (*User, error) {
	for _, u := range m.users {
		if u.ID == id {
			return u, nil
		}
	}
	return nil, nil
}

func (m *mockStore) UpdateUserPassword(id int64, passwordHash string) error {
	for _, u := range m.users {
		if u.ID == id {
			u.PasswordHash = passwordHash
			return nil
		}
	}
	return nil
}

func (m *mockStore) UserCount() (int, error) {
	return len(m.users), nil
}

// SessionStoreRW methods
func (m *mockStore) GetSession(token string) (*Session, error) {
	if m.sessions == nil {
		return nil, nil
	}
	s, ok := m.sessions[token]
	if !ok {
		return nil, nil
	}
	return s, nil
}

func (m *mockStore) RenewSession(token string, expiresAt time.Time) error {
	if s, ok := m.sessions[token]; ok {
		s.ExpiresAt = expiresAt
	}
	return nil
}

func (m *mockStore) CreateSession(token string, userID int64, expiresAt time.Time) error {
	if m.sessions == nil {
		m.sessions = make(map[string]*Session)
	}
	m.sessions[token] = &Session{Token: token, UserID: userID, ExpiresAt: expiresAt}
	return nil
}

func (m *mockStore) DeleteSession(token string) error {
	delete(m.sessions, token)
	return nil
}

func (m *mockStore) DeleteExpiredSessions() error { return nil }

// MonitorStore methods
func (m *mockStore) GetCheckResults(monitor string, from, to time.Time) ([]CheckResult, error) {
	return m.checkResults, nil
}
func (m *mockStore) GetCheckResultsHourly(monitor string, from, to time.Time) ([]CheckResultSummary, error) {
	return m.checkSummary, nil
}
func (m *mockStore) GetCheckResultsDaily(monitor string, from, to time.Time) ([]CheckResultSummary, error) {
	return m.checkSummary, nil
}

// HostStore methods
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
func (m *mockStore) GetAgentMetrics15Min(host string, from, to time.Time, names []string) ([]AgentMetricSummary, error) {
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
	m.insertedMetrics = append(m.insertedMetrics, metrics...)
	return nil
}
func (m *mockStore) UpdateAgentHeartbeat(host string, lastSeenAt time.Time) error {
	m.heartbeatUpdated = true
	return nil
}
func (m *mockStore) UpdateAgentVersion(host string, version string) error { return nil }

// IncidentStore methods
func (m *mockStore) GetIncidents(from, to time.Time) ([]Incident, error) { return m.incidents, nil }

// SecurityStore methods
func (m *mockStore) GetSecurityScans() ([]SecurityScan, error)       { return m.scans, nil }
func (m *mockStore) GetSecurityBaselines() ([]SecurityBaseline, error) { return m.baselines, nil }
func (m *mockStore) UpdateSecurityBaseline(target string, portsJSON string, updatedAt time.Time) error {
	m.baselineUpdated = true
	return nil
}

// BackupStore methods
func (m *mockStore) Backup(destPath string) error {
	if m.backupFunc != nil {
		return m.backupFunc(destPath)
	}
	return nil
}

// MuteStore methods
func (m *mockStore) GetMutedNames() (map[string]bool, error) { return nil, nil }
func (m *mockStore) SetMute(name string) error               { return nil }
func (m *mockStore) RemoveMute(name string) error            { return nil }

// mockHubState implements HubState for testing.
type mockHubState struct {
	statuses map[string]MonitorStatus
}

func (m *mockHubState) MonitorStatuses() map[string]MonitorStatus { return m.statuses }
func (m *mockHubState) MonitorStatus(name string) (MonitorStatus, bool) {
	s, ok := m.statuses[name]
	return s, ok
}
func (m *mockHubState) ReloadConfig() error         { return nil }
func (m *mockHubState) SendTestNotification() error  { return nil }
func (m *mockHubState) MuteAlerts(name string)       {}
func (m *mockHubState) UnmuteAlerts(name string)     {}

// newMockStore creates an in-memory mock store with basic functionality.
func newMockStore(t *testing.T) *mockStore {
	t.Helper()
	return &mockStore{
		users:    make(map[string]*User),
		sessions: make(map[string]*Session),
	}
}
