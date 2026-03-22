package store

import (
	"fmt"
	"strings"
	"time"
)

// AgentMetricRow represents a raw agent metric data point.
type AgentMetricRow struct {
	Host       string  `json:"host"`
	Timestamp  int64   `json:"timestamp"`
	MetricName string  `json:"metric_name"`
	Value      float64 `json:"value"`
}

// AgentMetricSummary represents an aggregated agent metric.
type AgentMetricSummary struct {
	Host       string  `json:"host"`
	Bucket     int64   `json:"bucket"`
	MetricName string  `json:"metric_name"`
	Avg        float64 `json:"avg"`
	Min        float64 `json:"min"`
	Max        float64 `json:"max"`
}

// AgentHeartbeat represents an agent's last-seen status.
type AgentHeartbeat struct {
	Host       string `json:"host"`
	LastSeenAt int64  `json:"last_seen_at"`
	Version    string `json:"version,omitempty"`
}

// StaleAgent represents an agent that hasn't checked in recently.
type StaleAgent struct {
	Host       string
	LastSeenAt int64
}

// Metric is a simple name/value pair for agent metric insertion.
type Metric struct {
	Name  string
	Value float64
}

// InsertAgentMetricsBatch stores a batch of agent metrics in a single transaction.
func (s *Store) InsertAgentMetricsBatch(host string, ts time.Time, metrics []Metric) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`INSERT INTO agent_metrics (host, timestamp, metric_name, value) VALUES (?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare: %w", err)
	}
	defer stmt.Close()

	tsUnix := ts.Unix()
	for _, m := range metrics {
		if _, err := stmt.Exec(host, tsUnix, m.Name, m.Value); err != nil {
			return fmt.Errorf("insert metric %s: %w", m.Name, err)
		}
	}

	return tx.Commit()
}

// QueryAgentMetricsRaw returns raw agent metrics for a host in a time range.
func (s *Store) QueryAgentMetricsRaw(host string, from, to time.Time, names []string) ([]AgentMetricRow, error) {
	query := `SELECT host, timestamp, metric_name, value FROM agent_metrics WHERE host = ? AND timestamp >= ? AND timestamp <= ?`
	args := []interface{}{host, from.Unix(), to.Unix()}

	if len(names) > 0 {
		placeholders := make([]string, 0, len(names))
		for _, n := range names {
			placeholders = append(placeholders, "metric_name LIKE ?")
			args = append(args, n+"%")
		}
		query += " AND (" + strings.Join(placeholders, " OR ") + ")"
	}

	query += " ORDER BY timestamp"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []AgentMetricRow
	for rows.Next() {
		var r AgentMetricRow
		if err := rows.Scan(&r.Host, &r.Timestamp, &r.MetricName, &r.Value); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// QueryAgentMetrics5Min returns 5-minute aggregated agent metrics.
func (s *Store) QueryAgentMetrics5Min(host string, from, to time.Time, names []string) ([]AgentMetricSummary, error) {
	return s.queryAgentMetricsSummary("agent_metrics_5min", "bucket", host, from, to, names)
}

// QueryAgentMetricsHourly returns hourly aggregated agent metrics.
func (s *Store) QueryAgentMetricsHourly(host string, from, to time.Time, names []string) ([]AgentMetricSummary, error) {
	return s.queryAgentMetricsSummary("agent_metrics_hourly", "hour", host, from, to, names)
}

// QueryAgentMetricsDaily returns daily aggregated agent metrics.
func (s *Store) QueryAgentMetricsDaily(host string, from, to time.Time, names []string) ([]AgentMetricSummary, error) {
	return s.queryAgentMetricsSummary("agent_metrics_daily", "day", host, from, to, names)
}

func (s *Store) queryAgentMetricsSummary(table, bucketCol, host string, from, to time.Time, names []string) ([]AgentMetricSummary, error) {
	query := fmt.Sprintf(`SELECT host, %s, metric_name, avg, min, max FROM %s WHERE host = ? AND %s >= ? AND %s <= ?`,
		bucketCol, table, bucketCol, bucketCol)
	args := []interface{}{host, from.Unix(), to.Unix()}

	if len(names) > 0 {
		placeholders := make([]string, 0, len(names))
		for _, n := range names {
			placeholders = append(placeholders, "metric_name LIKE ?")
			args = append(args, n+"%")
		}
		query += " AND (" + strings.Join(placeholders, " OR ") + ")"
	}

	query += fmt.Sprintf(" ORDER BY %s", bucketCol)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []AgentMetricSummary
	for rows.Next() {
		var r AgentMetricSummary
		if err := rows.Scan(&r.Host, &r.Bucket, &r.MetricName, &r.Avg, &r.Min, &r.Max); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// UpsertHeartbeat creates or updates the heartbeat for an agent host.
func (s *Store) UpsertHeartbeat(host string, ts time.Time) error {
	_, err := s.db.Exec(
		`INSERT INTO agent_heartbeats (host, last_seen_at) VALUES (?, ?)
		 ON CONFLICT(host) DO UPDATE SET last_seen_at = excluded.last_seen_at`,
		host, ts.Unix(),
	)
	return err
}

// GetHeartbeat returns the heartbeat for a specific host, or nil if not found.
func (s *Store) GetHeartbeat(host string) (*AgentHeartbeat, error) {
	row := s.db.QueryRow(`SELECT host, last_seen_at, COALESCE(version, '') FROM agent_heartbeats WHERE host = ?`, host)
	var hb AgentHeartbeat
	err := row.Scan(&hb.Host, &hb.LastSeenAt, &hb.Version)
	if err != nil {
		return nil, nil
	}
	return &hb, nil
}

// GetAllHeartbeats returns all agent heartbeats.
func (s *Store) GetAllHeartbeats() ([]AgentHeartbeat, error) {
	rows, err := s.db.Query(`SELECT host, last_seen_at, COALESCE(version, '') FROM agent_heartbeats ORDER BY host`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []AgentHeartbeat
	for rows.Next() {
		var hb AgentHeartbeat
		if err := rows.Scan(&hb.Host, &hb.LastSeenAt, &hb.Version); err != nil {
			return nil, err
		}
		results = append(results, hb)
	}
	return results, rows.Err()
}

// UpdateAgentVersion updates the version field for an agent host.
func (s *Store) UpdateAgentVersion(host string, version string) error {
	_, err := s.db.Exec(
		`UPDATE agent_heartbeats SET version = ? WHERE host = ?`,
		version, host,
	)
	return err
}

// GetStaleAgents returns agents with last_seen_at before the threshold.
func (s *Store) GetStaleAgents(threshold time.Time) ([]StaleAgent, error) {
	rows, err := s.db.Query(
		`SELECT host, last_seen_at FROM agent_heartbeats WHERE last_seen_at < ? ORDER BY host`,
		threshold.Unix(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []StaleAgent
	for rows.Next() {
		var sa StaleAgent
		if err := rows.Scan(&sa.Host, &sa.LastSeenAt); err != nil {
			return nil, err
		}
		results = append(results, sa)
	}
	return results, rows.Err()
}

// AggregateAgentMetrics5Min aggregates raw agent metrics into 5-minute buckets.
func (s *Store) AggregateAgentMetrics5Min(from, to time.Time) error {
	_, err := s.db.Exec(`
		INSERT INTO agent_metrics_5min (host, bucket, metric_name, avg, min, max)
		SELECT host,
		       (timestamp / 300) * 300 AS bucket,
		       metric_name,
		       AVG(value), MIN(value), MAX(value)
		FROM agent_metrics
		WHERE timestamp >= ? AND timestamp < ?
		GROUP BY host, bucket, metric_name
		ON CONFLICT(host, bucket, metric_name) DO UPDATE SET
		  avg=excluded.avg, min=excluded.min, max=excluded.max
	`, from.Unix(), to.Unix())
	return err
}

// QueryAgentMetrics15Min returns 15-minute aggregated agent metrics.
func (s *Store) QueryAgentMetrics15Min(host string, from, to time.Time, names []string) ([]AgentMetricSummary, error) {
	return s.queryAgentMetricsSummary("agent_metrics_15min", "bucket", host, from, to, names)
}

// AggregateAgentMetrics15Min aggregates 5-min agent metrics into 15-minute buckets.
func (s *Store) AggregateAgentMetrics15Min(from, to time.Time) error {
	_, err := s.db.Exec(`
		INSERT INTO agent_metrics_15min (host, bucket, metric_name, avg, min, max)
		SELECT host,
		       (bucket / 900) * 900 AS bucket,
		       metric_name,
		       AVG(avg), MIN(min), MAX(max)
		FROM agent_metrics_5min
		WHERE bucket >= ? AND bucket < ?
		GROUP BY host, bucket, metric_name
		ON CONFLICT(host, bucket, metric_name) DO UPDATE SET
		  avg=excluded.avg, min=excluded.min, max=excluded.max
	`, from.Unix(), to.Unix())
	return err
}

// DeleteOldAgentMetrics15Min deletes 15-min agent metrics older than cutoff.
func (s *Store) DeleteOldAgentMetrics15Min(cutoff time.Time) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM agent_metrics_15min WHERE bucket < ?`, cutoff.Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// AggregateAgentMetricsHourly aggregates 5-min agent metrics into hourly buckets.
func (s *Store) AggregateAgentMetricsHourly(hourStart, hourEnd int64) error {
	_, err := s.db.Exec(`
		INSERT INTO agent_metrics_hourly (host, hour, metric_name, avg, min, max)
		SELECT host,
		       ? AS hour,
		       metric_name,
		       AVG(avg), MIN(min), MAX(max)
		FROM agent_metrics_5min
		WHERE bucket >= ? AND bucket < ?
		GROUP BY host, metric_name
		ON CONFLICT(host, hour, metric_name) DO UPDATE SET
		  avg=excluded.avg, min=excluded.min, max=excluded.max
	`, hourStart, hourStart, hourEnd)
	return err
}

// AggregateAgentMetricsDaily aggregates hourly agent metrics into daily buckets.
func (s *Store) AggregateAgentMetricsDaily(dayStart, dayEnd int64) error {
	_, err := s.db.Exec(`
		INSERT INTO agent_metrics_daily (host, day, metric_name, avg, min, max)
		SELECT host,
		       ? AS day,
		       metric_name,
		       AVG(avg), MIN(min), MAX(max)
		FROM agent_metrics_hourly
		WHERE hour >= ? AND hour < ?
		GROUP BY host, metric_name
		ON CONFLICT(host, day, metric_name) DO UPDATE SET
		  avg=excluded.avg, min=excluded.min, max=excluded.max
	`, dayStart, dayStart, dayEnd)
	return err
}

// DeleteOldAgentMetrics deletes raw agent metrics older than cutoff.
func (s *Store) DeleteOldAgentMetrics(cutoff time.Time) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM agent_metrics WHERE timestamp < ?`, cutoff.Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteOldAgentMetrics5Min deletes 5-min agent metrics older than cutoff.
func (s *Store) DeleteOldAgentMetrics5Min(cutoff time.Time) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM agent_metrics_5min WHERE bucket < ?`, cutoff.Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// GetOpenIncidentForMonitor returns the open incident for a given monitor name.
func (s *Store) GetOpenIncidentForMonitor(monitor string) (*Incident, error) {
	return s.GetOpenIncident(monitor)
}

// CreateIncidentWithTime creates an incident using time.Time.
func (s *Store) CreateIncidentWithTime(monitor string, startedAt time.Time, cause string) (int64, error) {
	return s.CreateIncident(monitor, startedAt.Unix(), cause)
}

// ResolveIncidentWithTime resolves an incident using time.Time.
func (s *Store) ResolveIncidentWithTime(id int64, resolvedAt time.Time) error {
	return s.ResolveIncident(id, resolvedAt.Unix())
}
