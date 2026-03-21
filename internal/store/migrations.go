package store

const schema = `
CREATE TABLE IF NOT EXISTS check_results (
    monitor TEXT NOT NULL,
    timestamp INTEGER NOT NULL,
    status TEXT NOT NULL,
    latency_ms REAL,
    metadata_json TEXT
);
CREATE INDEX IF NOT EXISTS idx_check_results_monitor_time ON check_results(monitor, timestamp);

CREATE TABLE IF NOT EXISTS check_results_hourly (
    monitor TEXT NOT NULL,
    hour INTEGER NOT NULL,
    avg_latency REAL,
    min_latency REAL,
    max_latency REAL,
    success_count INTEGER,
    fail_count INTEGER,
    uptime_pct REAL
);
CREATE INDEX IF NOT EXISTS idx_check_hourly_monitor_time ON check_results_hourly(monitor, hour);

CREATE TABLE IF NOT EXISTS check_results_daily (
    monitor TEXT NOT NULL,
    day INTEGER NOT NULL,
    avg_latency REAL,
    min_latency REAL,
    max_latency REAL,
    success_count INTEGER,
    fail_count INTEGER,
    uptime_pct REAL
);
CREATE INDEX IF NOT EXISTS idx_check_daily_monitor_time ON check_results_daily(monitor, day);

CREATE TABLE IF NOT EXISTS incidents (
    id INTEGER PRIMARY KEY,
    monitor TEXT NOT NULL,
    started_at INTEGER NOT NULL,
    resolved_at INTEGER,
    cause TEXT
);
CREATE INDEX IF NOT EXISTS idx_incidents_monitor ON incidents(monitor, started_at);

CREATE TABLE IF NOT EXISTS agent_metrics (
    host TEXT NOT NULL,
    timestamp INTEGER NOT NULL,
    metric_name TEXT NOT NULL,
    value REAL NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_agent_metrics_host_time ON agent_metrics(host, timestamp);
CREATE INDEX IF NOT EXISTS idx_agent_metrics_name_time ON agent_metrics(host, metric_name, timestamp);

CREATE TABLE IF NOT EXISTS agent_metrics_5min (
    host TEXT NOT NULL,
    bucket INTEGER NOT NULL,
    metric_name TEXT NOT NULL,
    avg REAL,
    min REAL,
    max REAL
);
CREATE INDEX IF NOT EXISTS idx_agent_5min_host_time ON agent_metrics_5min(host, metric_name, bucket);

CREATE TABLE IF NOT EXISTS agent_metrics_hourly (
    host TEXT NOT NULL,
    hour INTEGER NOT NULL,
    metric_name TEXT NOT NULL,
    avg REAL,
    min REAL,
    max REAL
);
CREATE INDEX IF NOT EXISTS idx_agent_hourly_host_time ON agent_metrics_hourly(host, metric_name, hour);

CREATE TABLE IF NOT EXISTS agent_metrics_daily (
    host TEXT NOT NULL,
    day INTEGER NOT NULL,
    metric_name TEXT NOT NULL,
    avg REAL,
    min REAL,
    max REAL
);
CREATE INDEX IF NOT EXISTS idx_agent_daily_host_time ON agent_metrics_daily(host, metric_name, day);

CREATE TABLE IF NOT EXISTS security_scans (
    id INTEGER PRIMARY KEY,
    target TEXT NOT NULL,
    timestamp INTEGER NOT NULL,
    open_ports_json TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_security_scans_target ON security_scans(target, timestamp);

CREATE TABLE IF NOT EXISTS security_baselines (
    target TEXT PRIMARY KEY,
    expected_ports_json TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY,
    username TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
    token TEXT PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    expires_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS agent_heartbeats (
    host TEXT PRIMARY KEY,
    last_seen_at INTEGER NOT NULL
);
`
