package store

import (
	"database/sql"
	"fmt"
	"time"
)

// InsertCheckResult stores a single check result.
func (s *Store) InsertCheckResult(monitor string, timestamp int64, status string, latencyMs float64, metadataJSON string) error {
	_, err := s.db.Exec(
		`INSERT INTO check_results (monitor, timestamp, status, latency_ms, metadata_json) VALUES (?, ?, ?, ?, ?)`,
		monitor, timestamp, status, latencyMs, metadataJSON,
	)
	return err
}

// GetCheckResults returns raw check results for a monitor in a time range.
func (s *Store) GetCheckResults(monitor string, from, to int64) ([]CheckResult, error) {
	rows, err := s.db.Query(
		`SELECT monitor, timestamp, status, latency_ms, COALESCE(metadata_json, '') FROM check_results WHERE monitor = ? AND timestamp >= ? AND timestamp <= ? ORDER BY timestamp`,
		monitor, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []CheckResult
	for rows.Next() {
		var r CheckResult
		if err := rows.Scan(&r.Monitor, &r.Timestamp, &r.Status, &r.LatencyMs, &r.MetadataJSON); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// CreateIncident inserts a new open incident and returns its ID.
func (s *Store) CreateIncident(monitor string, startedAt int64, cause string) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO incidents (monitor, started_at, cause) VALUES (?, ?, ?)`,
		monitor, startedAt, cause,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ResolveIncident sets resolved_at on an incident.
func (s *Store) ResolveIncident(id int64, resolvedAt int64) error {
	_, err := s.db.Exec(
		`UPDATE incidents SET resolved_at = ? WHERE id = ?`,
		resolvedAt, id,
	)
	return err
}

// GetOpenIncident returns the open (unresolved) incident for a monitor, or nil.
func (s *Store) GetOpenIncident(monitor string) (*Incident, error) {
	row := s.db.QueryRow(
		`SELECT id, monitor, started_at, resolved_at, cause FROM incidents WHERE monitor = ? AND resolved_at IS NULL ORDER BY started_at DESC LIMIT 1`,
		monitor,
	)
	var inc Incident
	var resolvedAt sql.NullInt64
	err := row.Scan(&inc.ID, &inc.Monitor, &inc.StartedAt, &resolvedAt, &inc.Cause)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if resolvedAt.Valid {
		inc.ResolvedAt = &resolvedAt.Int64
	}
	return &inc, nil
}

// InsertSecurityScan stores a security scan result.
func (s *Store) InsertSecurityScan(target string, timestamp int64, openPortsJSON string) error {
	_, err := s.db.Exec(
		`INSERT INTO security_scans (target, timestamp, open_ports_json) VALUES (?, ?, ?)`,
		target, timestamp, openPortsJSON,
	)
	return err
}

// UpsertSecurityBaseline creates or updates the baseline for a target.
func (s *Store) UpsertSecurityBaseline(target, expectedPortsJSON string, updatedAt int64) error {
	_, err := s.db.Exec(
		`INSERT INTO security_baselines (target, expected_ports_json, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(target) DO UPDATE SET expected_ports_json = excluded.expected_ports_json, updated_at = excluded.updated_at`,
		target, expectedPortsJSON, updatedAt,
	)
	return err
}

// GetAllSecurityScans returns all security scans, most recent first.
func (s *Store) GetAllSecurityScans() ([]SecurityScan, error) {
	rows, err := s.db.Query(
		`SELECT id, target, timestamp, open_ports_json FROM security_scans ORDER BY timestamp DESC LIMIT 100`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var scans []SecurityScan
	for rows.Next() {
		var sc SecurityScan
		if err := rows.Scan(&sc.ID, &sc.Target, &sc.Timestamp, &sc.OpenPortsJSON); err != nil {
			return nil, err
		}
		scans = append(scans, sc)
	}
	return scans, rows.Err()
}

// GetAllSecurityBaselines returns all security baselines.
func (s *Store) GetAllSecurityBaselines() ([]SecurityBaseline, error) {
	rows, err := s.db.Query(
		`SELECT target, expected_ports_json, updated_at FROM security_baselines ORDER BY target`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var baselines []SecurityBaseline
	for rows.Next() {
		var bl SecurityBaseline
		if err := rows.Scan(&bl.Target, &bl.ExpectedPortsJSON, &bl.UpdatedAt); err != nil {
			return nil, err
		}
		baselines = append(baselines, bl)
	}
	return baselines, rows.Err()
}

// GetSecurityBaseline returns the baseline for a target, or nil.
func (s *Store) GetSecurityBaseline(target string) (*SecurityBaseline, error) {
	row := s.db.QueryRow(
		`SELECT target, expected_ports_json, updated_at FROM security_baselines WHERE target = ?`,
		target,
	)
	var bl SecurityBaseline
	err := row.Scan(&bl.Target, &bl.ExpectedPortsJSON, &bl.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &bl, nil
}

// DeleteOldCheckResults deletes check results older than cutoff and returns count deleted.
func (s *Store) DeleteOldCheckResults(cutoff int64) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM check_results WHERE timestamp < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// AggregateCheckResultsHourly aggregates raw check results for the given hour range
// into check_results_hourly. hourStart and hourEnd define the window [hourStart, hourEnd).
func (s *Store) AggregateCheckResultsHourly(hourStart, hourEnd int64) error {
	_, err := s.db.Exec(`
		INSERT INTO check_results_hourly (monitor, hour, avg_latency, min_latency, max_latency, success_count, fail_count, uptime_pct)
		SELECT
			monitor,
			? AS hour,
			AVG(CASE WHEN status = 'up' THEN latency_ms END),
			MIN(CASE WHEN status = 'up' THEN latency_ms END),
			MAX(CASE WHEN status = 'up' THEN latency_ms END),
			SUM(CASE WHEN status = 'up' THEN 1 ELSE 0 END),
			SUM(CASE WHEN status = 'down' THEN 1 ELSE 0 END),
			ROUND(100.0 * SUM(CASE WHEN status = 'up' THEN 1 ELSE 0 END) / COUNT(*), 2)
		FROM check_results
		WHERE timestamp >= ? AND timestamp < ?
		GROUP BY monitor
	`, hourStart, hourStart, hourEnd)
	return err
}

// GetCheckResultsHourly returns hourly aggregated results for a monitor.
func (s *Store) GetCheckResultsHourly(monitor string, from, to int64) ([]CheckResultHourly, error) {
	rows, err := s.db.Query(
		`SELECT monitor, hour, avg_latency, min_latency, max_latency, success_count, fail_count, uptime_pct
		 FROM check_results_hourly WHERE monitor = ? AND hour >= ? AND hour <= ? ORDER BY hour`,
		monitor, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []CheckResultHourly
	for rows.Next() {
		var r CheckResultHourly
		var avgLat, minLat, maxLat sql.NullFloat64
		if err := rows.Scan(&r.Monitor, &r.Hour, &avgLat, &minLat, &maxLat, &r.SuccessCount, &r.FailCount, &r.UptimePct); err != nil {
			return nil, err
		}
		if avgLat.Valid {
			r.AvgLatency = avgLat.Float64
		}
		if minLat.Valid {
			r.MinLatency = minLat.Float64
		}
		if maxLat.Valid {
			r.MaxLatency = maxLat.Float64
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// AggregateCheckResultsDaily aggregates hourly results for the given day range into daily.
func (s *Store) AggregateCheckResultsDaily(dayStart, dayEnd int64) error {
	_, err := s.db.Exec(`
		INSERT INTO check_results_daily (monitor, day, avg_latency, min_latency, max_latency, success_count, fail_count, uptime_pct)
		SELECT
			monitor,
			? AS day,
			AVG(avg_latency),
			MIN(min_latency),
			MAX(max_latency),
			SUM(success_count),
			SUM(fail_count),
			ROUND(100.0 * SUM(success_count) / (SUM(success_count) + SUM(fail_count)), 2)
		FROM check_results_hourly
		WHERE hour >= ? AND hour < ?
		GROUP BY monitor
	`, dayStart, dayStart, dayEnd)
	return err
}

// DeleteOldHourlyCheckResults deletes hourly results older than cutoff.
func (s *Store) DeleteOldHourlyCheckResults(cutoff int64) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM check_results_hourly WHERE hour < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// GetLastNCheckResults returns the last N check results for a monitor (most recent first).
func (s *Store) GetLastNCheckResults(monitor string, n int) ([]CheckResult, error) {
	rows, err := s.db.Query(
		`SELECT monitor, timestamp, status, latency_ms, COALESCE(metadata_json, '')
		 FROM check_results WHERE monitor = ? ORDER BY timestamp DESC LIMIT ?`,
		monitor, n,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []CheckResult
	for rows.Next() {
		var r CheckResult
		if err := rows.Scan(&r.Monitor, &r.Timestamp, &r.Status, &r.LatencyMs, &r.MetadataJSON); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// CountConsecutiveFailures returns the number of consecutive "down" results
// from the most recent check result backwards. Returns 0 if the latest is "up".
func (s *Store) CountConsecutiveFailures(monitor string) (int, error) {
	results, err := s.GetLastNCheckResults(monitor, 100) // enough headroom
	if err != nil {
		return 0, err
	}
	count := 0
	for _, r := range results {
		if r.Status != "down" {
			break
		}
		count++
	}
	return count, nil
}

// GetIncidents returns incidents in a time range.
func (s *Store) GetIncidents(from, to int64) ([]Incident, error) {
	rows, err := s.db.Query(
		`SELECT id, monitor, started_at, resolved_at, cause FROM incidents
		 WHERE started_at >= ? AND started_at <= ? ORDER BY started_at DESC`,
		from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var incidents []Incident
	for rows.Next() {
		var inc Incident
		var resolvedAt sql.NullInt64
		if err := rows.Scan(&inc.ID, &inc.Monitor, &inc.StartedAt, &resolvedAt, &inc.Cause); err != nil {
			return nil, fmt.Errorf("scan incident: %w", err)
		}
		if resolvedAt.Valid {
			inc.ResolvedAt = &resolvedAt.Int64
		}
		incidents = append(incidents, inc)
	}
	return incidents, rows.Err()
}

// CreateUser inserts a new user.
func (s *Store) CreateUser(username, passwordHash string) error {
	_, err := s.db.Exec(
		`INSERT INTO users (username, password_hash) VALUES (?, ?)`,
		username, passwordHash,
	)
	return err
}

// GetUserByUsername returns the user with the given username, or nil if not found.
func (s *Store) GetUserByUsername(username string) (*User, error) {
	row := s.db.QueryRow(
		`SELECT id, username, password_hash FROM users WHERE username = ?`,
		username,
	)
	var u User
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UserCount returns the total number of users.
func (s *Store) UserCount() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	return count, err
}

// CreateSession inserts a new session.
func (s *Store) CreateSession(token string, userID int64, expiresAt time.Time) error {
	_, err := s.db.Exec(
		`INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`,
		token, userID, expiresAt.Unix(),
	)
	return err
}

// GetSession returns the session with the given token, or nil if not found.
func (s *Store) GetSession(token string) (*Session, error) {
	row := s.db.QueryRow(
		`SELECT token, user_id, expires_at FROM sessions WHERE token = ?`,
		token,
	)
	var sess Session
	err := row.Scan(&sess.Token, &sess.UserID, &sess.ExpiresAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

// DeleteSession removes a session by token.
func (s *Store) DeleteSession(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// DeleteExpiredSessions removes all sessions past their expiry.
func (s *Store) DeleteExpiredSessions() error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at < ?`, time.Now().Unix())
	return err
}

// RenewSession updates the expiry time of an existing session.
func (s *Store) RenewSession(token string, expiresAt time.Time) error {
	_, err := s.db.Exec(
		`UPDATE sessions SET expires_at = ? WHERE token = ?`,
		expiresAt.Unix(), token,
	)
	return err
}
