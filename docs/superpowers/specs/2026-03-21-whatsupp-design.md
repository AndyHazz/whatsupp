# WhatsUpp — Design Specification

**Date:** 2026-03-21
**Status:** Draft
**Replaces:** Uptime Kuma, Beszel

## Overview

WhatsUpp is a lightweight, self-contained network monitoring tool written in Go. It combines uptime monitoring, system metrics collection, and security scanning into a single binary with an embedded web dashboard. Designed for low-resource deployment on an Oracle Cloud free-tier VPS (1GB RAM, 2 cores).

### Goals

- Replace Uptime Kuma (156MB RAM) and Beszel with a single ~15-20MB Go process
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
| Ping | ICMP ping, record RTT (round-trip time), packet loss % | 60s |
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
- metadata_json holds check-specific data (HTTP status codes, cert expiry, ping packet loss, etc.)

### Security Scanner

Full 65535-port TCP connect scan (no raw sockets / CAP_NET_RAW needed):

- Configurable concurrency (default: 500 concurrent connections)
- 2-second timeout per port
- ~4-5 minutes for a full scan
- Compares results against saved baseline
- Alerts on new ports appearing or expected ports disappearing
- Estimated resource usage: ~10MB RAM briefly during scan

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
  "metrics": { ... }
}
```

- Agent key generated during setup (`whatsupp agent init`), registered with hub
- Agent buffers up to 5 minutes of metrics locally if hub is unreachable
- Push model avoids NAT traversal issues (agents behind router, hub on public VPS)

### Node-Exporter Compatibility

Hub can also scrape existing Prometheus node-exporter endpoints:

- Parses Prometheus text exposition format
- Extracts same metric categories as the native agent
- Allows gradual migration — no need to replace node-exporters immediately

## Storage

### Engine

SQLite in WAL mode. Single file, embedded, zero external dependencies.

### Schema

```sql
-- Monitor definitions
monitors (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  type TEXT NOT NULL,        -- http, ping, port, agent, scrape, security
  config_json TEXT NOT NULL,  -- type-specific config
  interval_s INTEGER NOT NULL,
  enabled BOOLEAN DEFAULT 1,
  failure_threshold INTEGER DEFAULT 3
)

-- Raw check results (retained 1 month)
check_results (
  monitor_id INTEGER REFERENCES monitors(id),
  timestamp INTEGER NOT NULL,  -- unix epoch
  status TEXT NOT NULL,         -- up, down, degraded
  latency_ms REAL,
  metadata_json TEXT
)

-- Hourly summaries (retained 6 months)
check_results_hourly (
  monitor_id INTEGER,
  hour INTEGER,              -- unix epoch truncated to hour
  avg_latency REAL,
  min_latency REAL,
  max_latency REAL,
  success_count INTEGER,
  fail_count INTEGER,
  uptime_pct REAL
)

-- Daily summaries (retained forever)
check_results_daily (
  -- same columns as hourly, truncated to day
)

-- Incidents
incidents (
  id INTEGER PRIMARY KEY,
  monitor_id INTEGER REFERENCES monitors(id),
  started_at INTEGER NOT NULL,
  resolved_at INTEGER,
  cause TEXT
)

-- Raw agent metrics (retained 48 hours)
agent_metrics (
  host_id INTEGER,
  timestamp INTEGER NOT NULL,
  metric_name TEXT NOT NULL,
  value REAL NOT NULL
)

-- 5-minute agent metric summaries (retained 3 months)
agent_metrics_5min (
  host_id INTEGER,
  bucket INTEGER,            -- unix epoch truncated to 5 min
  metric_name TEXT,
  avg REAL, min REAL, max REAL
)

-- Hourly agent metric summaries (retained 6 months)
agent_metrics_hourly (
  host_id INTEGER,
  hour INTEGER,
  metric_name TEXT,
  avg REAL, min REAL, max REAL
)

-- Daily agent metric summaries (retained forever)
agent_metrics_daily (
  -- same as hourly, truncated to day
)

-- Security scan results
security_scans (
  id INTEGER PRIMARY KEY,
  target TEXT NOT NULL,
  timestamp INTEGER NOT NULL,
  open_ports_json TEXT NOT NULL
)

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

### Responsive

Works on mobile for quick status checks. Primarily designed for desktop.

## Alerting

### ntfy Integration

Sole alert channel. Connects to existing ntfy instance at andyhazz.duckdns.org:8444.

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
    url: "https://andyhazz.duckdns.org:8444"
    topic: "alerts"
    username: "AndyHazz"
    password: "${NTFY_PASSWORD}"   # from env var
  thresholds:
    ssl_expiry_days: [14, 7, 3, 1]
    disk_usage_pct: 90
    disk_hysteresis_pct: 5
    down_reminder_interval: "1h"
```

## Configuration

Single YAML config file mounted into the container:

```yaml
server:
  listen: ":8080"
  db_path: "/data/whatsupp.db"

auth:
  # Initial admin created on first run if no users exist
  initial_username: "admin"
  initial_password: "${WHATSUPP_ADMIN_PASSWORD}"

monitors:
  - name: "Plex"
    type: http
    url: "http://84.18.245.85:32400/identity"
    interval: 60s
    failure_threshold: 3

  - name: "Traefik"
    type: http
    url: "http://192.168.50.5"
    interval: 60s

  - name: "N8N"
    type: http
    url: "https://n8n8n8n.duckdns.org"
    interval: 120s

  - name: "Router WebUI"
    type: http
    url: "https://192.168.50.1:8443"
    interval: 120s

  - name: "Home Connection"
    type: ping
    host: "192.168.50.1"
    interval: 60s

  - name: "WireGuard VPN"
    type: ping
    host: "10.7.0.2"
    interval: 120s

  - name: "Minecraft"
    type: port
    host: "andyhazz.duckdns.org"
    port: 25565
    interval: 120s

agents:
  - name: "plexypi"
    key: "${AGENT_KEY_PLEXYPI}"
  - name: "dietpi"
    key: "${AGENT_KEY_DIETPI}"

scrape_targets:
  - name: "plexypi-node"
    url: "http://192.168.50.5:9100/metrics"
    interval: 30s
  - name: "dietpi-node"
    url: "http://192.168.50.50:9100/metrics"
    interval: 30s

security:
  targets:
    - host: "84.18.245.85"
      schedule: "weekly"
      scan_concurrency: 500
      timeout: "2s"
    - host: "145.241.217.231"
      schedule: "weekly"
      scan_concurrency: 500
      timeout: "2s"

alerting:
  default_failure_threshold: 3
  ntfy:
    url: "https://andyhazz.duckdns.org:8444"
    topic: "alerts"
    username: "AndyHazz"
    password: "${NTFY_PASSWORD}"
  thresholds:
    ssl_expiry_days: [14, 7, 3, 1]
    disk_usage_pct: 90
    disk_hysteresis_pct: 5
    down_reminder_interval: "1h"

retention:
  check_results_raw: "720h"       # 1 month
  agent_metrics_raw: "48h"
  agent_metrics_5min: "2160h"     # 3 months
  hourly: "4320h"                  # 6 months
  daily: "0"                       # forever
```

## Deployment

### Hub (VPS)

```yaml
# docker-compose.yml
services:
  whatsupp:
    image: ghcr.io/andyhazz/whatsupp:latest
    container_name: whatsupp
    restart: unless-stopped
    command: serve
    volumes:
      - ./config.yml:/etc/whatsupp/config.yml:ro
      - whatsupp-data:/data
    environment:
      - WHATSUPP_ADMIN_PASSWORD
      - NTFY_PASSWORD
      - AGENT_KEY_PLEXYPI
      - AGENT_KEY_DIETPI
    networks:
      - proxy-net

volumes:
  whatsupp-data:

networks:
  proxy-net:
    external: true
```

Caddy routes `andyhazz.duckdns.org:8443` to `whatsupp:8080`.

### Agent (plexypi / dietpi)

```yaml
services:
  whatsupp-agent:
    image: ghcr.io/andyhazz/whatsupp:latest
    container_name: whatsupp-agent
    restart: unless-stopped
    command: agent
    environment:
      - WHATSUPP_HUB_URL=https://andyhazz.duckdns.org:8443
      - WHATSUPP_AGENT_KEY=${AGENT_KEY}
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro  # for Docker metrics
      - /:/hostfs:ro                                    # for system metrics
    pid: host    # for accurate process/CPU metrics
```

### Migration Path

1. Deploy whatsupp alongside Uptime Kuma
2. Verify all monitors report correctly
3. Update Caddy to point :8443 at whatsupp
4. Remove Uptime Kuma and Beszel containers
5. Deploy agents to plexypi and dietpi
6. Optionally remove node-exporters once agents are confirmed working

## API

RESTful JSON API, all endpoints under `/api/v1/`:

### Public (no auth)

- `POST /api/v1/auth/login` — returns session token

### Authenticated (session token in cookie or Authorization header)

- `GET /api/v1/monitors` — list all monitors with current status
- `GET /api/v1/monitors/:id` — monitor detail
- `GET /api/v1/monitors/:id/results?from=&to=&resolution=` — check results (auto-selects tier)
- `POST /api/v1/monitors` — create monitor
- `PUT /api/v1/monitors/:id` — update monitor
- `DELETE /api/v1/monitors/:id` — delete monitor
- `GET /api/v1/hosts` — list agent hosts with current metrics
- `GET /api/v1/hosts/:id/metrics?from=&to=&resolution=` — host metrics (auto-selects tier)
- `GET /api/v1/incidents?from=&to=` — incident list
- `GET /api/v1/security/scans` — recent scan results
- `GET /api/v1/security/baselines` — current baselines
- `POST /api/v1/security/baselines/:target` — update baseline (accept current as new baseline)
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
│       └── config.go            # YAML config parsing
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
