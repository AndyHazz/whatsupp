package hub

import (
	"time"

	"github.com/andyhazz/whatsupp/internal/api"
	"github.com/andyhazz/whatsupp/internal/store"
)

// StoreAdapter wraps *store.Store to satisfy the api.Store interface.
// It adapts between store types (int64 timestamps) and api types (time.Time).
type StoreAdapter struct {
	s *store.Store
}

// NewStoreAdapter creates a new StoreAdapter.
func NewStoreAdapter(s *store.Store) *StoreAdapter {
	return &StoreAdapter{s: s}
}

// --- UserStore ---

func (a *StoreAdapter) GetUserByUsername(username string) (*api.User, error) {
	u, err := a.s.GetUserByUsername(username)
	if err != nil || u == nil {
		return nil, err
	}
	return &api.User{ID: u.ID, Username: u.Username, PasswordHash: u.PasswordHash}, nil
}

func (a *StoreAdapter) CreateUser(username, passwordHash string) error {
	return a.s.CreateUser(username, passwordHash)
}

func (a *StoreAdapter) UserCount() (int, error) {
	return a.s.UserCount()
}

// --- SessionStoreRW ---

func (a *StoreAdapter) GetSession(token string) (*api.Session, error) {
	sess, err := a.s.GetSession(token)
	if err != nil || sess == nil {
		return nil, err
	}
	return &api.Session{
		Token:     sess.Token,
		UserID:    sess.UserID,
		ExpiresAt: time.Unix(sess.ExpiresAt, 0),
	}, nil
}

func (a *StoreAdapter) RenewSession(token string, expiresAt time.Time) error {
	return a.s.RenewSession(token, expiresAt)
}

func (a *StoreAdapter) CreateSession(token string, userID int64, expiresAt time.Time) error {
	return a.s.CreateSession(token, userID, expiresAt)
}

func (a *StoreAdapter) DeleteSession(token string) error {
	return a.s.DeleteSession(token)
}

func (a *StoreAdapter) DeleteExpiredSessions() error {
	return a.s.DeleteExpiredSessions()
}

// --- MonitorStore ---

func (a *StoreAdapter) GetCheckResults(monitor string, from, to time.Time) ([]api.CheckResult, error) {
	results, err := a.s.GetCheckResults(monitor, from.Unix(), to.Unix())
	if err != nil {
		return nil, err
	}
	out := make([]api.CheckResult, len(results))
	for i, r := range results {
		out[i] = api.CheckResult{
			Monitor:      r.Monitor,
			Timestamp:    r.Timestamp,
			Status:       r.Status,
			LatencyMs:    r.LatencyMs,
			MetadataJSON: r.MetadataJSON,
		}
	}
	return out, nil
}

func (a *StoreAdapter) GetCheckResultsHourly(monitor string, from, to time.Time) ([]api.CheckResultSummary, error) {
	results, err := a.s.GetCheckResultsHourly(monitor, from.Unix(), to.Unix())
	if err != nil {
		return nil, err
	}
	out := make([]api.CheckResultSummary, len(results))
	for i, r := range results {
		out[i] = api.CheckResultSummary{
			Monitor:      r.Monitor,
			Bucket:       r.Hour,
			AvgLatency:   r.AvgLatency,
			MinLatency:   r.MinLatency,
			MaxLatency:   r.MaxLatency,
			SuccessCount: r.SuccessCount,
			FailCount:    r.FailCount,
			UptimePct:    r.UptimePct,
		}
	}
	return out, nil
}

func (a *StoreAdapter) GetCheckResultsDaily(monitor string, from, to time.Time) ([]api.CheckResultSummary, error) {
	// Daily aggregation — reuse hourly for now if daily not yet available
	return nil, nil
}

// --- HostStore ---

func (a *StoreAdapter) GetAgentHeartbeats() ([]api.AgentHeartbeat, error) {
	// Agent heartbeats not yet implemented in store
	return nil, nil
}

func (a *StoreAdapter) GetAgentHeartbeat(host string) (*api.AgentHeartbeat, error) {
	return nil, nil
}

func (a *StoreAdapter) GetAgentMetrics(host string, from, to time.Time, names []string) ([]api.AgentMetric, error) {
	return nil, nil
}

func (a *StoreAdapter) GetAgentMetrics5Min(host string, from, to time.Time, names []string) ([]api.AgentMetricSummary, error) {
	return nil, nil
}

func (a *StoreAdapter) GetAgentMetricsHourly(host string, from, to time.Time, names []string) ([]api.AgentMetricSummary, error) {
	return nil, nil
}

func (a *StoreAdapter) GetAgentMetricsDaily(host string, from, to time.Time, names []string) ([]api.AgentMetricSummary, error) {
	return nil, nil
}

func (a *StoreAdapter) InsertAgentMetrics(host string, timestamp time.Time, metrics []api.AgentMetricPoint) error {
	return nil
}

func (a *StoreAdapter) UpdateAgentHeartbeat(host string, lastSeenAt time.Time) error {
	return nil
}

// --- IncidentStore ---

func (a *StoreAdapter) GetIncidents(from, to time.Time) ([]api.Incident, error) {
	results, err := a.s.GetIncidents(from.Unix(), to.Unix())
	if err != nil {
		return nil, err
	}
	out := make([]api.Incident, len(results))
	for i, r := range results {
		out[i] = api.Incident{
			ID:        r.ID,
			Monitor:   r.Monitor,
			StartedAt: r.StartedAt,
			Cause:     r.Cause,
		}
		if r.ResolvedAt != nil {
			out[i].ResolvedAt = r.ResolvedAt
		}
	}
	return out, nil
}

// --- SecurityStore ---

func (a *StoreAdapter) GetSecurityScans() ([]api.SecurityScan, error) {
	return nil, nil
}

func (a *StoreAdapter) GetSecurityBaselines() ([]api.SecurityBaseline, error) {
	return nil, nil
}

func (a *StoreAdapter) UpdateSecurityBaseline(target string, portsJSON string, updatedAt time.Time) error {
	return a.s.UpsertSecurityBaseline(target, portsJSON, updatedAt.Unix())
}

// --- BackupStore ---

func (a *StoreAdapter) Backup(destPath string) error {
	// SQLite backup not yet implemented
	return nil
}
