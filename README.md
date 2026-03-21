# WhatsUpp

Self-hosted infrastructure monitoring in a single Go binary. Monitors HTTP endpoints, pings hosts, checks TCP ports, scans for open ports, tracks incidents, and sends alerts — all with a Svelte dashboard and zero external dependencies beyond SQLite.

## Features

- **HTTP/Ping/Port monitoring** with configurable intervals and failure thresholds
- **UP/DOWN state machine** with consecutive failure counting before alerting
- **Incident tracking** — automatic create on DOWN, resolve on UP, with duration
- **ntfy alerts** with deduplication and configurable reminder intervals
- **SSL certificate expiry** warnings at configurable day thresholds
- **Security scanning** — full 65535-port TCP connect scan with baseline drift detection
- **Agent mode** — push CPU, memory, disk, network, temperature, and Docker metrics
- **Prometheus scraping** — pull metrics from node-exporter endpoints
- **Tiered data retention** — raw → 5min → hourly → daily downsampling
- **REST API** with session auth, rate-limited login, and agent bearer tokens
- **WebSocket** live updates for real-time dashboard
- **Svelte 5 SPA** with Dracula theme, uPlot charts, embedded in the binary
- **SQLite WAL** — no database server needed

## Quick Start

```bash
# Build
cd frontend && npm install && npm run build && cd ..
cp -r frontend/dist internal/web/dist
go build -o whatsupp ./cmd/whatsupp/

# Run
./whatsupp serve -config config.example.yml
```

Or with Docker:

```bash
docker compose up -d
```

## Configuration

Copy `config.example.yml` and customise:

```yaml
server:
  listen: ":8080"
  db_path: "/data/whatsupp.db"

monitors:
  - name: "Website"
    type: http
    url: "https://example.com"
    interval: 60s
    failure_threshold: 3

  - name: "Gateway"
    type: ping
    host: "192.168.1.1"
    interval: 60s

  - name: "Minecraft"
    type: port
    host: "10.0.0.5"
    port: 25565
    interval: 120s

alerting:
  ntfy:
    url: "${NTFY_URL}"
    topic: "${NTFY_TOPIC}"
  thresholds:
    ssl_expiry_days: [14, 7, 3, 1]
    down_reminder_interval: "1h"
```

Environment variables in `${VAR}` syntax are expanded at load time.

## Agent Mode

Run on each host you want to monitor:

```bash
# Generate config
./whatsupp agent init --hub https://hub:8080 --key your-agent-key

# Run
./whatsupp agent -config /etc/whatsupp/agent.yml
```

Collects CPU, memory, disk, network, temperature, and Docker container metrics every 30s.

## Architecture

```
whatsupp serve                    whatsupp agent
┌─────────────────────┐          ┌──────────────┐
│  Svelte SPA (embed) │          │  gopsutil    │
│  REST API (chi)     │◄─────────│  Docker SDK  │
│  WebSocket hub      │  metrics │  push client │
│  Check scheduler    │          └──────────────┘
│  State machine      │
│  Incident manager   │
│  ntfy alerter       │
│  Downsampler        │
│  SQLite WAL         │
└─────────────────────┘
```

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/v1/auth/login` | Public | Login, returns session cookie |
| POST | `/api/v1/auth/logout` | Session | Logout |
| GET | `/api/v1/health` | Public | Health check |
| GET | `/api/v1/monitors` | Session | List all monitors with status |
| GET | `/api/v1/monitors/:name` | Session | Single monitor detail |
| GET | `/api/v1/monitors/:name/results` | Session | Check results (auto-selects tier) |
| GET | `/api/v1/hosts` | Session | List agent hosts |
| GET | `/api/v1/hosts/:name` | Session | Host detail |
| GET | `/api/v1/hosts/:name/metrics` | Session | Host metrics (auto-selects tier) |
| GET | `/api/v1/incidents` | Session | List incidents |
| GET | `/api/v1/security/scans` | Session | Security scan results |
| GET | `/api/v1/security/baselines` | Session | Port baselines |
| POST | `/api/v1/security/baselines/:target` | Session | Accept current ports as baseline |
| GET | `/api/v1/config` | Session | Get YAML config |
| PUT | `/api/v1/config` | Session | Update config |
| POST | `/api/v1/backup` | Session | Download SQLite backup |
| POST | `/api/v1/agent/metrics` | Bearer | Push agent metrics |
| WS | `/api/v1/ws` | Session | Live updates |

## Data Retention

| Tier | Default | Description |
|------|---------|-------------|
| Raw check results | 30 days | Every check result |
| Raw agent metrics | 48 hours | Every metric push |
| 5-minute agent | 90 days | Aggregated from raw |
| Hourly | 6 months | Aggregated from raw/5min |
| Daily | Forever | Aggregated from hourly |

## Tech Stack

**Backend:** Go 1.22+, SQLite (mattn/go-sqlite3), chi router, gorilla/websocket, bcrypt, gopsutil, pro-bing

**Frontend:** Svelte 5, Vite, uPlot, Dracula theme

**Alerts:** ntfy (self-hosted or ntfy.sh)

## License

MIT
