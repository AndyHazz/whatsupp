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
- **Tiered data retention** — raw → 5min → hourly → daily downsampling
- **REST API** with session auth, rate-limited login, and agent bearer tokens
- **WebSocket** live updates for real-time dashboard
- **Svelte 5 SPA** with Dracula theme, uPlot charts, embedded in the binary
- **SQLite WAL** — no database server needed

## Docker Setup (Recommended)

### 1. Create directories and config

```bash
mkdir -p whatsupp/config && cd whatsupp
```

Copy the example config into `config/config.yml` and edit to suit your setup:

```yaml
server:
  listen: ":8080"
  db_path: "/data/whatsupp.db"

auth:
  initial_username: "admin"
  initial_password: "${WHATSUPP_ADMIN_PASSWORD}"

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

### 2. Create `.env` file

```bash
WHATSUPP_ADMIN_PASSWORD=changeme
NTFY_URL=https://ntfy.example.com
NTFY_TOPIC=whatsupp
NTFY_USERNAME=
NTFY_PASSWORD=
AGENT_KEY_SERVER1=change-this-to-a-random-key
```

### 3. Create `docker-compose.yml`

```yaml
services:
  whatsupp:
    image: ghcr.io/andyhazz/whatsupp:latest
    container_name: whatsupp
    restart: unless-stopped
    command: serve
    cap_add:
      - NET_RAW    # Required for ICMP ping checks
    volumes:
      - ./config:/etc/whatsupp
      - whatsupp-data:/data
    env_file:
      - .env
    ports:
      - "8080:8080"
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/api/v1/health"]
      interval: 30s
      timeout: 5s
      retries: 3

volumes:
  whatsupp-data:
```

### 4. Start

```bash
docker compose up -d
```

Open `http://your-host:8080` and log in with the credentials from your config.

### Building from source

If you prefer to build the image locally instead of using the pre-built one:

```bash
git clone https://github.com/andyhazz/whatsupp.git && cd whatsupp
docker compose up -d --build
```

The multi-stage Dockerfile builds the Svelte frontend and Go binary, producing a minimal Alpine image.

### Updating

```bash
docker compose pull && docker compose up -d
```

The SQLite database is stored in the `whatsupp-data` volume and persists across updates.

## Agent Setup

Deploy the agent on each host you want to collect system metrics from (CPU, memory, disk, network, temperature, Docker containers).

### With Docker

Create `docker-compose.agent.yml` on the target host:

```yaml
services:
  whatsupp-agent:
    image: ghcr.io/andyhazz/whatsupp:latest
    container_name: whatsupp-agent
    restart: unless-stopped
    command: agent
    environment:
      - WHATSUPP_HUB_URL=https://your-hub:8080
      - WHATSUPP_AGENT_KEY=your-agent-key
      - DOCKER_HOST=tcp://docker-proxy:2375
    volumes:
      - /:/hostfs:ro
    pid: host
    depends_on:
      - docker-proxy

  docker-proxy:
    image: tecnativa/docker-socket-proxy
    container_name: docker-proxy
    restart: unless-stopped
    environment:
      - CONTAINERS=1
      - POST=0
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
```

The agent mounts the host filesystem read-only at `/hostfs` for disk/temperature metrics and uses `pid: host` for process visibility. Docker metrics are collected via a socket proxy that only permits read-only container listing.

```bash
docker compose -f docker-compose.agent.yml up -d
```

### Without Docker

```bash
./whatsupp agent init --hub https://your-hub:8080 --key your-agent-key
./whatsupp agent -config /etc/whatsupp/agent.yml
```

### Hub configuration

Add a matching agent entry in the hub's `config.yml`:

```yaml
agents:
  - name: "server1"
    key: "${AGENT_KEY_SERVER1}"
```

## Quick Start (without Docker)

```bash
cd frontend && npm install && npm run build && cd ..
cp -r frontend/dist internal/web/dist
go build -o whatsupp ./cmd/whatsupp/
./whatsupp serve -config config.example.yml
```

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
