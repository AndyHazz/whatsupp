package api

import "time"

// Store combines all store interfaces needed by the API.
type Store interface {
	UserStore
	SessionStoreRW
	MonitorStore
	HostStore
	IncidentStore
	SecurityStore
	BackupStore
}

// SessionStore is the interface for session persistence (read).
type SessionStore interface {
	GetSession(token string) (*Session, error)
	RenewSession(token string, expiresAt time.Time) error
}

// SessionStoreRW extends SessionStore with write operations.
type SessionStoreRW interface {
	SessionStore
	CreateSession(token string, userID int64, expiresAt time.Time) error
	DeleteSession(token string) error
	DeleteExpiredSessions() error
}

// UserStore is the interface for user persistence.
type UserStore interface {
	GetUserByUsername(username string) (*User, error)
	CreateUser(username, passwordHash string) error
	UserCount() (int, error)
}

// MonitorStore provides check result queries.
type MonitorStore interface {
	GetCheckResults(monitor string, from, to time.Time) ([]CheckResult, error)
	GetCheckResultsHourly(monitor string, from, to time.Time) ([]CheckResultSummary, error)
	GetCheckResultsDaily(monitor string, from, to time.Time) ([]CheckResultSummary, error)
}

// HostStore provides agent metric queries.
type HostStore interface {
	GetAgentHeartbeats() ([]AgentHeartbeat, error)
	GetAgentHeartbeat(host string) (*AgentHeartbeat, error)
	GetAgentMetrics(host string, from, to time.Time, names []string) ([]AgentMetric, error)
	GetAgentMetrics5Min(host string, from, to time.Time, names []string) ([]AgentMetricSummary, error)
	GetAgentMetricsHourly(host string, from, to time.Time, names []string) ([]AgentMetricSummary, error)
	GetAgentMetricsDaily(host string, from, to time.Time, names []string) ([]AgentMetricSummary, error)
	InsertAgentMetrics(host string, timestamp time.Time, metrics []AgentMetricPoint) error
	UpdateAgentHeartbeat(host string, lastSeenAt time.Time) error
}

// IncidentStore provides incident queries.
type IncidentStore interface {
	GetIncidents(from, to time.Time) ([]Incident, error)
}

// SecurityStore provides security scan and baseline queries.
type SecurityStore interface {
	GetSecurityScans() ([]SecurityScan, error)
	GetSecurityBaselines() ([]SecurityBaseline, error)
	UpdateSecurityBaseline(target string, portsJSON string, updatedAt time.Time) error
}

// BackupStore provides database backup capability.
type BackupStore interface {
	Backup(destPath string) error
}

// HubState provides read-only access to the hub's current state.
type HubState interface {
	// MonitorStatuses returns the current status of all monitors.
	MonitorStatuses() map[string]MonitorStatus
	// MonitorStatus returns the current status of a single monitor.
	MonitorStatus(name string) (MonitorStatus, bool)
	// ReloadConfig triggers a config reload.
	ReloadConfig() error
}

// --- Data types ---

// User represents an authenticated user.
type User struct {
	ID           int64
	Username     string
	PasswordHash string
}

// Session represents an active login session.
type Session struct {
	Token     string
	UserID    int64
	ExpiresAt time.Time
}

type CheckResult struct {
	Monitor      string  `json:"monitor"`
	Timestamp    int64   `json:"timestamp"`
	Status       string  `json:"status"`
	LatencyMs    float64 `json:"latency_ms"`
	MetadataJSON string  `json:"metadata_json,omitempty"`
}

type CheckResultSummary struct {
	Monitor      string  `json:"monitor"`
	Bucket       int64   `json:"bucket"` // hour or day epoch
	AvgLatency   float64 `json:"avg_latency"`
	MinLatency   float64 `json:"min_latency"`
	MaxLatency   float64 `json:"max_latency"`
	SuccessCount int     `json:"success_count"`
	FailCount    int     `json:"fail_count"`
	UptimePct    float64 `json:"uptime_pct"`
}

type MonitorStatus struct {
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	Status    string  `json:"status"` // "up", "down", "unknown"
	LatencyMs float64 `json:"latency_ms"`
	LastCheck int64   `json:"last_check"`
	UptimePct float64 `json:"uptime_pct"` // 24h uptime
}

type AgentHeartbeat struct {
	Host       string `json:"host"`
	LastSeenAt int64  `json:"last_seen_at"`
}

type AgentMetric struct {
	Host       string  `json:"host"`
	Timestamp  int64   `json:"timestamp"`
	MetricName string  `json:"metric_name"`
	Value      float64 `json:"value"`
}

type AgentMetricSummary struct {
	Host       string  `json:"host"`
	Bucket     int64   `json:"bucket"`
	MetricName string  `json:"metric_name"`
	Avg        float64 `json:"avg"`
	Min        float64 `json:"min"`
	Max        float64 `json:"max"`
}

type AgentMetricPoint struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

type Incident struct {
	ID         int64  `json:"id"`
	Monitor    string `json:"monitor"`
	StartedAt  int64  `json:"started_at"`
	ResolvedAt *int64 `json:"resolved_at,omitempty"`
	Cause      string `json:"cause"`
}

type SecurityScan struct {
	ID            int64  `json:"id"`
	Target        string `json:"target"`
	Timestamp     int64  `json:"timestamp"`
	OpenPortsJSON string `json:"open_ports_json"`
}

type SecurityBaseline struct {
	Target            string `json:"target"`
	ExpectedPortsJSON string `json:"expected_ports_json"`
	UpdatedAt         int64  `json:"updated_at"`
}
