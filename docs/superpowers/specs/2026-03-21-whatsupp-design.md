# WhatsUpp — Design Specification

**Date:** 2026-03-21
**Status:** Draft
**Replaces:** Uptime Kuma, Beszel

## Overview

WhatsUpp is a lightweight, self-contained network monitoring tool written in Go. It combines uptime monitoring, system metrics collection, and security scanning into a single binary with an embedded web dashboard. Designed for low-resource deployment on an Oracle Cloud free-tier VPS (1GB RAM, 2 cores).

### Goals

- Replace Uptime Kuma (156MB RAM) and Beszel with a single Go process (~40-60MB RSS in practice, including SQLite mmap, goroutine stacks, and HTTP buffers)
- Provide uptime monitoring with response time history, system metrics, and security port scanning
- Alert via ntfy when things go wrong
- Serve a Dracula-themed dashboard behind Caddy reverse proxy
- Support existing node-exporter endpoints without requiring immediate migration

### Non-Goals

- Replacing Victoria Metrics or Grafana (deep ad-hoc analytics)
- Supporting alert channels beyond ntfy
- Multi-user/team features (single admin user is fine)

## Architecture

```
┌────────────────────────────────────────────────────────┐
│                    VPS (Oracle)                          │
│                                                          │
│  ┌────────────────────────────────────────────────────┐ │
│  │               whatsupp serve (hub)                   │ │
│  │                                                      │ │
│  │  Scheduler ─▶ Check Engine ─▶ SQLite (WAL)          │ │
│  │                 │                                    │ │
│  │                 ├─ HTTP checks                       │ │
│  │                 ├─ ICMP ping                         │ │
│  │                 ├─ TCP port checks                   │ │
│  │                 ├─ Security scans (full 65535 TCP)   │ │
│  │                 ├─ Agent metrics receiver             │ │
│  │                 └─ Prometheus scraper                 │ │
│  │                                                      │ │
│  │  HTTP API + WebSocket ──▶ Svelte SPA (embed.FS)     │ │
│  │  ntfy alert client                                   │ │
│  │  Downsampling goroutine                              │ │
│  └────────────────────────────────────────────────────┘ │
│                        ▲                                 │
│           Caddy (:8443)│                                 │
└────────────────────────┼─────────────────────────────────┘
                         │
        ┌────────────────┼────────────────┐
        │                │                │
   whatsupp agent   whatsupp agent   node-exporter
    (plexypi)        (dietpi)        (scraped)
```

### Two modes from the same binary

- `whatsupp serve` — hub: scheduler, check engine, API, dashboard, storage, alerting
- `whatsupp agent` — remote host: collects system metrics, pushes to hub via HTTP

## Check Engine

### Check Types

| Type | Description | Default Interval |
|---|---|---|
| HTTP | GET/HEAD URL, record status code, response time, SSL cert expiry | 60s |
| Ping | ICMP ping (requires `CAP_NET_RAW` — see Deployment), record RTT (round-trip time), packet loss % | 60s |
| Port | TCP connect to host:port, record success + latency | 120s |
| Agent | Receive pushed system metrics from whatsupp agents | 30s |
| Scrape | Pull from Prometheus /metrics endpoint (node-exporter compat) | 30s |
| Security | Full 65535 TCP connect scan, compare against baseline, detect drift | Weekly |

### Monitor State Machine

```
UP ──(N consecutive failures)──▶ DOWN ──(1 success)──▶ UP
                                   │                      │
                                   ▼                      ▼
                              INCIDENT created      INCIDENT resolved
                              ntfy alert sent       ntfy recovery sent
```

- Failure threshold configurable per monitor (global default: 3)
- Each check result stored as: (monitor_id, timestamp, status, latency_ms, metadata_json)
- Status values: `up` or `down` (no intermediate states — keeps the state machine simple)
- metadata_json holds check-specific data (HTTP status codes, cert expiry, ping packet loss, etc.)

### Agent Staleness Detection

The hub tracks the last-seen timestamp for each registered agent. A scheduled goroutine checks every 60 seconds:
- If no metrics received for 5 minutes (configurable): mark agent as `lost`, create incident, send ntfy alert
- When metrics resume: resolve incident, send recovery alert
- Same deduplication rules as monitor alerts (no spam, reminder after 1 hour)

### Security Scanner

Full 65535-port TCP connect scan (no raw sockets / CAP_NET_RAW needed):

- Configurable concurrency (default: 200 for remote targets, 500 for localhost)
- 2-second timeout per port
- ~4-5 minutes for a full scan at 500 concurrency, ~10 minutes at 200
- Compares results against saved baseline
- Alerts on new ports appearing or expected ports disappearing
- Estimated resource usage: ~10MB RAM briefly during scan
- Multiple targets scanned sequentially (not in parallel) to limit resource usage
- Schedule uses cron syntax (e.g. `"0 3 * * 0"` = Sunday 3am), not vague "weekly"

## Agent Metrics

### Collected Metrics

| Category | Metrics |
|---|---|
| CPU | Usage %, load averages, per-core usage |
| Memory | Used/available/swap, usage % |
| Disk | Usage per mount, read/write IOPS, throughput |
| Network | Bytes in/out per interface, error/drop counts |
| Temperature | CPU/GPU temps (from /sys/class/thermal) |
| Docker | Container count, per-container CPU/mem/status |

### Protocol

Push model — agents POST to hub every 30s:

```
POST /api/v1/agent/metrics
Authorization: Bearer <agent-key>
Content-Type: application/json

{
  "host": "plexypi",
  "timestamp": "2026-03-21T12:00:00Z",
  "metrics": [
    {"name": "cpu.usage_pct", "value": 23.5},
    {"name": "cpu.load_1m", "value": 0.45},
    {"name": "mem.used_bytes", "value": 1073741824},
    {"name": "mem.usage_pct", "value": 52.1},
    {"name": "disk./.usage_pct", "value": 67.3},
    {"name": "disk./mnt/data.usage_pct", "value": 45.0},
    {"name": "net.eth0.rx_bytes", "value": 123456789},
    {"name": "temp.cpu", "value": 52.0},
    {"name": "docker.plex.cpu_pct", "value": 5.2},
    {"name": "docker.plex.mem_bytes", "value": 524288000},
    {"name": "docker.plex.status", "value": 1}
  ]
}
```

### Metric Naming Convention

Flat dotted names: `{category}.{qualifier}.{metric}`. This maps directly to `agent_metrics.metric_name`:

| Pattern | Examples |
|---|---|
| `cpu.*` | `cpu.usage_pct`, `cpu.load_1m`, `cpu.load_5m`, `cpu.core0_pct` |
| `mem.*` | `mem.used_bytes`, `mem.available_bytes`, `mem.swap_used_bytes`, `mem.usage_pct` |
| `disk.{mount}.*` | `disk./.usage_pct`, `disk./.read_iops`, `disk./mnt/data.write_bytes` |
| `net.{iface}.*` | `net.eth0.rx_bytes`, `net.eth0.tx_bytes`, `net.eth0.errors` |
| `temp.*` | `temp.cpu`, `temp.gpu` |
| `docker.{name}.*` | `docker.plex.cpu_pct`, `docker.plex.mem_bytes`, `docker.plex.status` (1=running, 0=stopped) |

### Agent Setup

`whatsupp agent init --hub https://monitor.example.com --key <agent-key>` generates `/etc/whatsupp/agent.yml` with hub URL and key. The key must match one defined in the hub's config.

- Agent key generated during setup (`whatsupp agent init`), registered with hub
- Agent keys stored as SHA-256 hashes in the YAML (hub hashes the key on first write via the Settings UI; plaintext key shown once during setup, then discarded)
- Agent buffers up to 5 minutes of metrics locally if hub is unreachable
- Push model avoids NAT traversal issues (agents behind router, hub on public VPS)

### Node-Exporter Compatibility

Hub can also scrape existing Prometheus node-exporter endpoints:

- Parses Prometheus text exposition format
- Maps well-known node-exporter metrics to the whatsupp naming convention:
  - `node_cpu_seconds_total` → `cpu.usage_pct` (computed from rate of change)
  - `node_memory_MemAvailable_bytes` → `mem.available_bytes`
  - `node_filesystem_avail_bytes` → `disk.{mount}.avail_bytes`
  - `node_network_receive_bytes_total` → `net.{iface}.rx_bytes`
  - `node_hwmon_temp_celsius` → `temp.cpu`
- Unmapped metrics are ignored (node-exporter exposes hundreds; we only need the core set)
- Allows gradual migration — no need to replace node-exporters immediately

## Storage

### Engine

SQLite in WAL mode. Single file, embedded, zero external dependencies.

**Required PRAGMAs:**
- `PRAGMA journal_mode=WAL` — concurrent reads during writes
- `PRAGMA busy_timeout=5000` — wait up to 5s on write contention
- `PRAGMA synchronous=NORMAL` — safe with WAL, better write performance
- `PRAGMA foreign_keys=ON`

**Backup:** Use SQLite `.backup` API (not filesystem copy) to safely backup while the database is in use. The hub exposes `GET /api/v1/admin/backup` which triggers a `.backup` to a timestamped file.

### Schema

No monitor or host config is stored in the database — the YAML is the sole source. SQLite stores only time-series data, incidents, scans, and auth. Monitor and host **names** (from YAML) are used as keys.

```sql
-- Raw check results (retained 1 month)
check_results (
  monitor TEXT NOT NULL,           -- monitor name from YAML (e.g. "Plex")
  timestamp INTEGER NOT NULL,      -- unix epoch
  status TEXT NOT NULL,             -- up, down
  latency_ms REAL,
  metadata_json TEXT
)
CREATE INDEX idx_check_results_monitor_time ON check_results(monitor, timestamp);

-- Hourly check summaries (retained 6 months)
check_results_hourly (
  monitor TEXT NOT NULL,
  hour INTEGER NOT NULL,           -- unix epoch truncated to hour
  avg_latency REAL,
  min_latency REAL,
  max_latency REAL,
  success_count INTEGER,
  fail_count INTEGER,
  uptime_pct REAL
)
CREATE INDEX idx_check_hourly_monitor_time ON check_results_hourly(monitor, hour);

-- Daily check summaries (retained forever)
check_results_daily (
  monitor TEXT NOT NULL,
  day INTEGER NOT NULL,            -- unix epoch truncated to day
  avg_latency REAL,
  min_latency REAL,
  max_latency REAL,
  success_count INTEGER,
  fail_count INTEGER,
  uptime_pct REAL
)
CREATE INDEX idx_check_daily_monitor_time ON check_results_daily(monitor, day);

-- Incidents
incidents (
  id INTEGER PRIMARY KEY,
  monitor TEXT NOT NULL,            -- monitor name from YAML
  started_at INTEGER NOT NULL,
  resolved_at INTEGER,
  cause TEXT
)
CREATE INDEX idx_incidents_monitor ON incidents(monitor, started_at);

-- Raw agent metrics (retained 48 hours)
agent_metrics (
  host TEXT NOT NULL,               -- host name from YAML (e.g. "plexypi")
  timestamp INTEGER NOT NULL,
  metric_name TEXT NOT NULL,
  value REAL NOT NULL
)
CREATE INDEX idx_agent_metrics_host_time ON agent_metrics(host, timestamp);
CREATE INDEX idx_agent_metrics_name_time ON agent_metrics(host, metric_name, timestamp);

-- 5-minute agent metric summaries (retained 3 months)
agent_metrics_5min (
  host TEXT NOT NULL,
  bucket INTEGER NOT NULL,         -- unix epoch truncated to 5 min
  metric_name TEXT NOT NULL,
  avg REAL, min REAL, max REAL
)
CREATE INDEX idx_agent_5min_host_time ON agent_metrics_5min(host, metric_name, bucket);

-- Hourly agent metric summaries (retained 6 months)
agent_metrics_hourly (
  host TEXT NOT NULL,
  hour INTEGER NOT NULL,
  metric_name TEXT NOT NULL,
  avg REAL, min REAL, max REAL
)
CREATE INDEX idx_agent_hourly_host_time ON agent_metrics_hourly(host, metric_name, hour);

-- Daily agent metric summaries (retained forever)
agent_metrics_daily (
  host TEXT NOT NULL,
  day INTEGER NOT NULL,
  metric_name TEXT NOT NULL,
  avg REAL, min REAL, max REAL
)
CREATE INDEX idx_agent_daily_host_time ON agent_metrics_daily(host, metric_name, day);

-- Security scan results
security_scans (
  id INTEGER PRIMARY KEY,
  target TEXT NOT NULL,
  timestamp INTEGER NOT NULL,
  open_ports_json TEXT NOT NULL
)
CREATE INDEX idx_security_scans_target ON security_scans(target, timestamp);

-- Security baselines
security_baselines (
  target TEXT PRIMARY KEY,
  expected_ports_json TEXT NOT NULL,
  updated_at INTEGER NOT NULL
)

-- Authentication
users (
  id INTEGER PRIMARY KEY,
  username TEXT UNIQUE NOT NULL,
  password_hash TEXT NOT NULL
)

sessions (
  token TEXT PRIMARY KEY,
  user_id INTEGER REFERENCES users(id),
  expires_at INTEGER NOT NULL
)

-- Agent last-seen tracking (for staleness detection)
agent_heartbeats (
  host TEXT PRIMARY KEY,            -- host name from YAML
  last_seen_at INTEGER NOT NULL
)
```

### Downsampling Schedule

| Tier | Source | Aggregation | Retention |
|---|---|---|---|
| Raw check results | Check engine | None | 1 month |
| Raw agent metrics | Agent push / scrape | None | 48 hours |
| 5-min agent metrics | Raw agent metrics | Avg/min/max | 3 months |
| Hourly (all) | Raw / 5-min | Avg/min/max/uptime% | 6 months |
| Daily (all) | Hourly | Avg/min/max/uptime% | Forever |

Downsampling runs as scheduled goroutines:
- Every 5 minutes: aggregate raw agent metrics → 5-min, delete agent raw older than 48h
- Every hour: aggregate → hourly, delete raw checks older than 1 month, delete 5-min older than 3 months
- Every midnight: aggregate hourly → daily, delete hourly older than 6 months

### Storage Estimates

- Raw check results (1 month): ~15MB
- Raw agent metrics (48h): ~44MB
- 5-min agent metrics (3 months): ~26MB
- Hourly summaries (6 months): ~3MB
- Daily summaries (years): negligible
- **Total steady state: ~90MB**

## Frontend

### Technology

- Svelte (compiles to ~15KB gzipped vanilla JS)
- uPlot for time-series charts (12KB, GPU-accelerated)
- Dracula theme throughout

### Dracula Colour Palette

| Role | Colour | Hex |
|---|---|---|
| Background | Dark | #282a36 |
| Current Line | Slightly lighter | #44475a |
| Foreground | Light | #f8f8f2 |
| Comment/muted | Grey | #6272a4 |
| UP / success | Green | #50fa7b |
| DOWN / error | Red | #ff5555 |
| Warning | Orange | #ffb86c |
| Info / links | Cyan | #8be9fd |
| Accent | Purple | #bd93f9 |
| Secondary accent | Pink | #ff79c6 |

### Pages

| Page | Content |
|---|---|
| Overview | Grid of all monitors: current status, response time sparklines, uptime % |
| Monitor Detail | Zoomable response time chart (auto-selects raw/5min/hourly/daily), incident history, config |
| Hosts | Per-host system metrics: CPU, RAM, disk, network, temps, Docker. Same zoomable time range |
| Security | Last scan per target, baseline diff, port change history, SSL cert expiry countdown |
| Incidents | Timeline of all incidents, duration, cause |
| Settings | Monitor CRUD, agent registration, ntfy config, user management, scan targets, baselines |

### Live Updates

WebSocket connection from frontend to hub. Check results broadcast to connected clients — status changes and sparklines update without polling.

**WebSocket auth:** The WS upgrade request at `/api/v1/ws` authenticates via the session cookie (browsers cannot set custom headers on WS upgrade). The session cookie uses `SameSite=Strict`, `HttpOnly`, `Secure` flags.

**WebSocket message format:**
```json
{"type": "check_result", "data": {"monitor_id": 1, "status": "up", "latency_ms": 45.2, "timestamp": 1711018800}}
{"type": "incident", "data": {"id": 5, "monitor_id": 1, "started_at": 1711018800, "cause": "connection refused"}}
{"type": "agent_metric", "data": {"host_id": 1, "metrics": [{"name": "cpu.usage_pct", "value": 23.5}], "timestamp": 1711018800}}
```

### Responsive

Works on mobile for quick status checks. Primarily designed for desktop.

## Alerting

### ntfy Integration

Sole alert channel. Connects to any [ntfy](https://ntfy.sh) instance (self-hosted or ntfy.sh).

### Alert Types

| Event | ntfy Priority | Example |
|---|---|---|
| Monitor DOWN | High (4) | "Plex is DOWN - connection refused (3/3 failures)" |
| Monitor RECOVERED | Default (3) | "Plex is UP - was down for 4m 32s" |
| New port detected | Urgent (5) | "Security: new port 4444/tcp on 84.18.245.85 (not in baseline)" |
| Port disappeared | High (4) | "Security: port 443/tcp no longer open on 84.18.245.85" |
| SSL cert expiring | High (4) | "SSL cert for n8n8n8n.duckdns.org expires in 7 days" |
| Agent lost | High (4) | "No metrics from plexypi for 5 minutes" |
| Disk critical | High (4) | "plexypi / is 92% full" |

### Deduplication

- DOWN alert sent once, not repeated every check interval
- Configurable reminder interval if still down (default: 1 hour)
- SSL expiry alerts at configurable day thresholds (default: 14, 7, 3, 1)
- Disk alerts use hysteresis: alert at threshold, don't re-alert until usage drops below (threshold - hysteresis) and rises again

### Configuration

```yaml
alerting:
  default_failure_threshold: 3
  ntfy:
    url: "${NTFY_URL}"
    topic: "${NTFY_TOPIC}"
    username: "${NTFY_USERNAME}"      # optional, for authenticated ntfy
    password: "${NTFY_PASSWORD}"
  thresholds:
    ssl_expiry_days: [14, 7, 3, 1]
    disk_usage_pct: 90
    disk_hysteresis_pct: 5
    down_reminder_interval: "1h"
```

## Configuration

Single YAML config file mounted into the container.

### YAML as Sole Configuration Source

The YAML config file is the single source of truth for all configuration — monitors, agents, hosts, security targets, alerting. There is no config stored in the database.

- Hub reads `config.yml` on startup and watches for changes (fsnotify). Config changes take effect without restart.
- The Settings UI reads and writes the YAML file directly via the API (`GET/PUT /api/v1/config`).
- SQLite stores only time-series data (check results, metrics, incidents, scans) and auth (users, sessions).
- Monitor and host names from the YAML are used as keys in time-series tables (no integer ID mapping needed).
- The config file must be mounted read-write in the container for UI edits to persist.

This eliminates config drift, sync logic, and an entire class of "which source wins?" bugs.

```yaml
server:
  listen: ":8080"
  db_path: "/data/whatsupp.db"

auth:
  # Initial admin created on first run if no users exist
  initial_username: "admin"
  initial_password: "${WHATSUPP_ADMIN_PASSWORD}"

monitors:
  - name: "Web App"
    type: http
    url: "https://example.com"
    interval: 60s
    failure_threshold: 3

  - name: "API Server"
    type: http
    url: "http://10.0.0.5:8080/health"
    interval: 60s

  - name: "Gateway"
    type: ping
    host: "10.0.0.1"
    interval: 60s

  - name: "VPN Tunnel"
    type: ping
    host: "10.7.0.2"
    interval: 120s

  - name: "Game Server"
    type: port
    host: "game.example.com"
    port: 25565
    interval: 120s

agents:
  - name: "server-1"
    key: "${AGENT_KEY_SERVER1}"
  - name: "server-2"
    key: "${AGENT_KEY_SERVER2}"

scrape_targets:
  - name: "server-1-node"
    url: "http://10.0.0.5:9100/metrics"
    interval: 30s

security:
  targets:
    - host: "203.0.113.1"           # your public IP
      schedule: "0 3 * * 0"          # Sunday 3am
      scan_concurrency: 200
      timeout: "2s"
    - host: "198.51.100.1"           # your VPS
      schedule: "0 4 * * 0"          # Sunday 4am (sequential, after first)
      scan_concurrency: 500
      timeout: "2s"

alerting:
  default_failure_threshold: 3
  ntfy:
    url: "${NTFY_URL}"               # e.g. https://ntfy.example.com
    topic: "${NTFY_TOPIC}"
    username: "${NTFY_USERNAME}"
    password: "${NTFY_PASSWORD}"
  thresholds:
    ssl_expiry_days: [14, 7, 3, 1]
    disk_usage_pct: 90
    disk_hysteresis_pct: 5
    down_reminder_interval: "1h"

retention:
  check_results_raw: "720h"       # 30 days
  agent_metrics_raw: "48h"
  agent_metrics_5min: "2160h"     # 90 days
  hourly: "4320h"                  # 180 days
  daily: "0"                       # forever
```

## Deployment

### Hub (VPS)

```yaml
# docker-compose.yml
services:
  whatsupp:
    image: ghcr.io/andyhazz/whatsupp:latest  # update to your registry
    container_name: whatsupp
    restart: unless-stopped
    command: serve
    cap_add:
      - NET_RAW              # required for ICMP ping checks
    volumes:
      - ./config:/etc/whatsupp          # config.yml lives here, writable for UI edits
      - whatsupp-data:/data
    env_file:
      - .env                  # WHATSUPP_ADMIN_PASSWORD, NTFY_*, AGENT_KEY_*
    networks:
      - proxy-net
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/api/v1/health"]
      interval: 30s
      timeout: 5s
      retries: 3

volumes:
  whatsupp-data:

networks:
  proxy-net:
    external: true
```

Reverse proxy routes to `whatsupp:8080` (e.g., Caddy, Traefik, nginx).

### Agent

Uses a Docker socket proxy (`tecnativa/docker-socket-proxy`) to safely expose only container stats — no access to environment variables, exec, or other sensitive Docker API endpoints.

```yaml
services:
  whatsupp-agent:
    image: ghcr.io/andyhazz/whatsupp:latest  # update to your registry
    container_name: whatsupp-agent
    restart: unless-stopped
    command: agent
    environment:
      - WHATSUPP_HUB_URL=https://monitor.example.com
      - WHATSUPP_AGENT_KEY=${AGENT_KEY}
      - DOCKER_HOST=tcp://docker-proxy:2375
    volumes:
      - /:/hostfs:ro                       # for system metrics
    pid: host                              # for accurate process/CPU metrics
    depends_on:
      - docker-proxy

  docker-proxy:
    image: tecnativa/docker-socket-proxy
    container_name: docker-proxy
    restart: unless-stopped
    environment:
      - CONTAINERS=1                       # allow listing containers + stats
      - POST=0                             # read-only
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
```

### Migration Path

1. Deploy whatsupp alongside existing monitoring (it listens on a different internal port)
2. Add a temporary reverse proxy route (e.g., `:8448`) pointing to whatsupp for testing
3. Verify all monitors and dashboards work correctly
4. Switch the primary route to whatsupp
5. Remove old monitoring containers (Uptime Kuma, Beszel, etc.)
6. Deploy agents to monitored hosts
7. Optionally remove node-exporters once agents are confirmed working

## API

RESTful JSON API, all endpoints under `/api/v1/`:

### Public (no auth)

- `POST /api/v1/auth/login` — sets session cookie (24h expiry, renewed on activity). Rate limited: 5 attempts per minute per IP, lockout for 15 minutes after 10 failures.
- `POST /api/v1/auth/logout` — clears session
- `GET /api/v1/health` — healthcheck (for Caddy/Docker healthchecks)

### Authenticated (session token in cookie or Authorization header)

- `GET /api/v1/config` — read current YAML config (for Settings UI)
- `PUT /api/v1/config` — write updated YAML config (Settings UI edits). Hub reloads automatically.
- `GET /api/v1/monitors` — list all monitors with current status
- `GET /api/v1/monitors/:name` — monitor detail
- `GET /api/v1/monitors/:name/results?from=&to=` — check results (auto-selects storage tier based on time range: <=48h→raw, <=30d→hourly, >30d→daily)
- `GET /api/v1/hosts` — list agent/scrape hosts with current metrics and last-seen
- `GET /api/v1/hosts/:name` — host detail (last seen, current top-level metrics)
- `GET /api/v1/hosts/:name/metrics?from=&to=&names=` — host metrics (auto-selects tier: <=48h→raw, <=7d→5min, <=90d→hourly, >90d→daily). `names` filters by metric name prefix (e.g. `cpu,mem`)
- `GET /api/v1/incidents?from=&to=` — incident list
- `GET /api/v1/security/scans` — recent scan results
- `GET /api/v1/security/baselines` — current baselines
- `POST /api/v1/security/baselines/:target` — update baseline (accept current as new baseline)
- `GET /api/v1/admin/backup` — trigger SQLite backup, returns backup file
- `WS /api/v1/ws` — WebSocket for live updates

### Agent (agent key auth)

- `POST /api/v1/agent/metrics` — push metrics from agent

## Project Structure

```
whatsupp/
├── cmd/
│   └── whatsupp/
│       └── main.go              # CLI: serve / agent subcommands
├── internal/
│   ├── hub/
│   │   ├── hub.go               # Hub orchestration
│   │   ├── scheduler.go         # Check scheduling
│   │   └── downsampler.go       # Retention & aggregation
│   ├── checks/
│   │   ├── http.go
│   │   ├── ping.go
│   │   ├── port.go
│   │   ├── scrape.go            # Prometheus scraper
│   │   └── security.go          # TCP port scanner
│   ├── agent/
│   │   ├── agent.go             # Agent mode orchestration
│   │   ├── collector.go         # System metrics collection
│   │   └── docker.go            # Docker metrics
│   ├── store/
│   │   ├── store.go             # SQLite interface
│   │   ├── migrations.go        # Schema migrations
│   │   └── queries.go           # Prepared queries
│   ├── alerting/
│   │   └── ntfy.go              # ntfy client + deduplication
│   ├── api/
│   │   ├── router.go            # HTTP router
│   │   ├── handlers.go          # API handlers
│   │   ├── auth.go              # Session auth
│   │   └── websocket.go         # WebSocket hub
│   └── config/
│       ├── config.go            # YAML config parsing + validation
│       └── writer.go            # Write config back to YAML (for Settings UI)
├── frontend/
│   ├── src/
│   │   ├── App.svelte
│   │   ├── pages/
│   │   │   ├── Overview.svelte
│   │   │   ├── MonitorDetail.svelte
│   │   │   ├── Hosts.svelte
│   │   │   ├── Security.svelte
│   │   │   ├── Incidents.svelte
│   │   │   └── Settings.svelte
│   │   ├── components/
│   │   │   ├── Chart.svelte      # uPlot wrapper
│   │   │   ├── StatusGrid.svelte
│   │   │   ├── Sparkline.svelte
│   │   │   └── ...
│   │   └── lib/
│   │       ├── api.js            # API client
│   │       ├── ws.js             # WebSocket client
│   │       └── theme.js          # Dracula tokens
│   ├── package.json
│   └── vite.config.js
├── Dockerfile                     # Multi-stage: build frontend, build Go, minimal runtime
├── docker-compose.yml
├── config.example.yml
├── go.mod
├── go.sum
└── README.md
```

## Build

Multi-stage Dockerfile:

1. **Stage 1 (Node):** Build Svelte frontend → `dist/`
2. **Stage 2 (Go):** Copy `dist/` into `embed.FS`, build Go binary
3. **Stage 3 (Scratch/Alpine):** Copy binary + ca-certificates only

Result: single container image, ~15-20MB compressed.
