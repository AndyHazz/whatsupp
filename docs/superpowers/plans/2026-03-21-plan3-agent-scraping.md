# WhatsUpp Plan 3: Agent + Scraping

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** System metrics agent that collects CPU, memory, disk, network, temperature, and Docker container metrics, pushes them to the hub. Plus Prometheus node-exporter scraper on the hub side for backward compatibility.

**Architecture:** Same binary in agent mode (`whatsupp agent`) collects host metrics via /proc, /sys, and Docker API (via socket proxy), buffers locally, and pushes JSON to the hub's `/api/v1/agent/metrics` endpoint. Hub-side scraper pulls from Prometheus endpoints and normalizes to the same metric naming convention.

**Tech Stack:** Go 1.22+, shirou/gopsutil/v4 (system metrics), Docker SDK (github.com/docker/docker/client), prometheus text parser (github.com/prometheus/common/expfmt)

---

## Prerequisites

This plan assumes Plan 1 (store, config) and Plan 2 (API with agent metrics endpoint) are complete. Specifically:

- `internal/store/store.go` exists with SQLite WAL setup, migrations, and `Store` interface
- `internal/config/config.go` exists with YAML parsing including `Agents`, `ScrapeTargets` sections
- `internal/api/router.go` exists with the HTTP router and auth middleware
- `POST /api/v1/agent/metrics` endpoint exists (handler stub from Plan 2 that validates bearer token and returns 200)
- `cmd/whatsupp/main.go` exists with cobra root command and `serve` subcommand
- `go.mod` exists with module `github.com/andyhazz/whatsupp`
- Agent key hashing (SHA-256) and validation are implemented in the auth layer
- `agent_metrics`, `agent_metrics_5min`, `agent_heartbeats` tables exist in the schema migrations

---

## Phase 1: Agent CLI and Config

### Task 1.1: Agent config struct and parser

**File:** `internal/agent/config.go`

- [ ] Write test `internal/agent/config_test.go`:
  - `TestParseAgentConfig` — parses valid YAML with hub_url, agent_key, hostname, interval
  - `TestParseAgentConfig_Defaults` — missing interval defaults to 30s, missing hostname defaults to `os.Hostname()`
  - `TestParseAgentConfig_EnvOverrides` — `WHATSUPP_HUB_URL` and `WHATSUPP_AGENT_KEY` override YAML values
  - `TestParseAgentConfig_Invalid` — missing hub_url returns error
- [ ] Implement `AgentConfig` struct and `ParseAgentConfig(path string) (*AgentConfig, error)`
- [ ] Run: `go test ./internal/agent/ -run TestParseAgentConfig -v`
- [ ] Commit: `feat(agent): add agent config struct and YAML parser`

```go
// AgentConfig is the agent-side configuration loaded from /etc/whatsupp/agent.yml
type AgentConfig struct {
    HubURL    string        `yaml:"hub_url"`
    AgentKey  string        `yaml:"agent_key"`
    Hostname  string        `yaml:"hostname"`
    Interval  time.Duration `yaml:"interval"`
    HostFS    string        `yaml:"host_fs"`    // default: /hostfs
    DockerHost string       `yaml:"docker_host"` // default: from DOCKER_HOST env
}
```

**agent.yml format:**
```yaml
hub_url: "https://monitor.example.com"
agent_key: "sk-abc123..."
hostname: "plexypi"
interval: 30s
host_fs: "/hostfs"
```

### Task 1.2: Agent CLI subcommands

**File:** `cmd/whatsupp/agent.go`

- [ ] Write test `cmd/whatsupp/agent_test.go`:
  - `TestAgentCommand_Exists` — `agent` subcommand registered on root
  - `TestAgentInitCommand_Exists` — `agent init` subcommand registered
  - `TestAgentInitCommand_Flags` — `--hub` and `--key` flags exist and are required
- [ ] Implement `agent` cobra command (runs the agent loop) and `agent init` cobra command (generates config)
- [ ] Run: `go test ./cmd/whatsupp/ -run TestAgent -v`
- [ ] Commit: `feat(cli): add agent and agent init subcommands`

### Task 1.3: Agent init — generate config file

**File:** `internal/agent/init.go`

- [ ] Write test `internal/agent/init_test.go`:
  - `TestGenerateConfig` — given hub URL, key, hostname, writes valid YAML to a temp file
  - `TestGenerateConfig_DetectsHostname` — when hostname is empty, uses `os.Hostname()`
  - `TestGenerateConfig_NoOverwrite` — returns error if file already exists (safety)
- [ ] Implement `GenerateConfig(path, hubURL, key, hostname string) error`
- [ ] Wire into `agent init` command: `whatsupp agent init --hub URL --key KEY [--hostname NAME] [--config /etc/whatsupp/agent.yml]`
- [ ] Run: `go test ./internal/agent/ -run TestGenerateConfig -v`
- [ ] Commit: `feat(agent): implement agent init config generation`

---

## Phase 2: Metric Types and Naming Convention

### Task 2.1: Metric type and naming helpers

**File:** `internal/agent/metric.go`

- [ ] Write test `internal/agent/metric_test.go`:
  - `TestMetric_JSON` — `Metric{Name: "cpu.usage_pct", Value: 23.5}` marshals correctly
  - `TestMetricBatch_JSON` — full batch with host, timestamp, metrics array matches expected JSON
  - `TestMetricName_CPU` — helper `CPUMetric("usage_pct")` returns `"cpu.usage_pct"`
  - `TestMetricName_Disk` — `DiskMetric("/", "usage_pct")` returns `"disk./.usage_pct"`
  - `TestMetricName_Net` — `NetMetric("eth0", "rx_bytes")` returns `"net.eth0.rx_bytes"`
  - `TestMetricName_Docker` — `DockerMetric("plex", "cpu_pct")` returns `"docker.plex.cpu_pct"`
  - `TestMetricName_Temp` — `TempMetric("cpu")` returns `"temp.cpu"`
  - `TestMetricName_Mem` — `MemMetric("used_bytes")` returns `"mem.used_bytes"`
- [ ] Implement types and naming functions:

```go
type Metric struct {
    Name  string  `json:"name"`
    Value float64 `json:"value"`
}

type MetricBatch struct {
    Host      string    `json:"host"`
    Timestamp time.Time `json:"timestamp"`
    Metrics   []Metric  `json:"metrics"`
}

func CPUMetric(name string) string    { return "cpu." + name }
func MemMetric(name string) string    { return "mem." + name }
func DiskMetric(mount, name string) string { return "disk." + mount + "." + name }
func NetMetric(iface, name string) string  { return "net." + iface + "." + name }
func TempMetric(name string) string   { return "temp." + name }
func DockerMetric(container, name string) string { return "docker." + container + "." + name }
```

- [ ] Run: `go test ./internal/agent/ -run TestMetric -v`
- [ ] Commit: `feat(agent): add metric types and naming convention helpers`

---

## Phase 3: System Metrics Collectors

Each collector implements the `Collector` interface and returns `[]Metric`.

### Task 3.0: Collector interface

**File:** `internal/agent/collector.go`

- [ ] Define the `Collector` interface:

```go
type Collector interface {
    Name() string
    Collect(ctx context.Context) ([]Metric, error)
}
```

- [ ] No separate test needed — tested through implementations below.
- [ ] Commit: `feat(agent): add Collector interface`

### Task 3.1: CPU collector

**File:** `internal/agent/collect_cpu.go`

- [ ] Write test `internal/agent/collect_cpu_test.go`:
  - `TestCPUCollector_Collect` — returns metrics with correct names: `cpu.usage_pct`, `cpu.load_1m`, `cpu.load_5m`, `cpu.load_15m`
  - `TestCPUCollector_PerCore` — returns `cpu.core0_pct`, `cpu.core1_pct`, etc. for each logical core
  - `TestCPUCollector_ValueRanges` — usage_pct between 0-100, load averages >= 0
  - `TestCPUCollector_HostFS` — respects HOST_PROC env var for containerized collection
- [ ] Implement `CPUCollector` using `gopsutil/v4`:
  - `cpu.Percent(1*time.Second, false)` for overall usage (1s sample window)
  - `cpu.Percent(0, true)` for per-core (uses delta since last call — zero on first call, skip if so)
  - `load.Avg()` for load averages
- [ ] Run: `go test ./internal/agent/ -run TestCPUCollector -v`
- [ ] Commit: `feat(agent): implement CPU metrics collector`

**Metrics produced:**
- `cpu.usage_pct` — overall CPU usage percentage
- `cpu.load_1m`, `cpu.load_5m`, `cpu.load_15m` — load averages
- `cpu.core{N}_pct` — per-core usage percentage

### Task 3.2: Memory collector

**File:** `internal/agent/collect_mem.go`

- [ ] Write test `internal/agent/collect_mem_test.go`:
  - `TestMemCollector_Collect` — returns `mem.used_bytes`, `mem.available_bytes`, `mem.total_bytes`, `mem.usage_pct`, `mem.swap_used_bytes`, `mem.swap_total_bytes`
  - `TestMemCollector_ValueRanges` — usage_pct 0-100, byte values > 0
  - `TestMemCollector_SwapZero` — handles zero swap gracefully (swap values = 0, no error)
- [ ] Implement `MemCollector` using `gopsutil/v4`:
  - `mem.VirtualMemory()` for RAM stats
  - `mem.SwapMemory()` for swap stats
- [ ] Run: `go test ./internal/agent/ -run TestMemCollector -v`
- [ ] Commit: `feat(agent): implement memory metrics collector`

**Metrics produced:**
- `mem.used_bytes`, `mem.available_bytes`, `mem.total_bytes`, `mem.usage_pct`
- `mem.swap_used_bytes`, `mem.swap_total_bytes`

### Task 3.3: Disk collector

**File:** `internal/agent/collect_disk.go`

- [ ] Write test `internal/agent/collect_disk_test.go`:
  - `TestDiskCollector_Collect` — returns at least `disk./.usage_pct`, `disk./.total_bytes`, `disk./.used_bytes`, `disk./.avail_bytes` for root mount
  - `TestDiskCollector_IOPS` — returns `disk./.read_iops`, `disk./.write_iops`, `disk./.read_bytes`, `disk./.write_bytes` (may be 0 on first call due to delta)
  - `TestDiskCollector_FiltersVirtual` — excludes tmpfs, devtmpfs, sysfs, proc, etc.
  - `TestDiskCollector_MountNames` — mount point used as qualifier (e.g., `/mnt/data` becomes `disk./mnt/data.usage_pct`)
- [ ] Implement `DiskCollector` using `gopsutil/v4`:
  - `disk.Partitions(false)` to list mounts, filter out virtual filesystems
  - `disk.Usage(mountpoint)` for usage stats per mount
  - `disk.IOCounters()` for IOPS and throughput (compute delta from previous call)
  - Store previous IO counters + timestamp for delta calculation
- [ ] Run: `go test ./internal/agent/ -run TestDiskCollector -v`
- [ ] Commit: `feat(agent): implement disk metrics collector`

**Filter list for virtual filesystems:**
```go
var virtualFS = map[string]bool{
    "tmpfs": true, "devtmpfs": true, "sysfs": true, "proc": true,
    "devpts": true, "securityfs": true, "cgroup": true, "cgroup2": true,
    "pstore": true, "efivarfs": true, "bpf": true, "tracefs": true,
    "debugfs": true, "hugetlbfs": true, "mqueue": true, "fusectl": true,
    "overlay": true, "nsfs": true, "fuse.lxcfs": true,
}
```

**Metrics produced per mount:**
- `disk.{mount}.usage_pct`, `disk.{mount}.total_bytes`, `disk.{mount}.used_bytes`, `disk.{mount}.avail_bytes`
- `disk.{mount}.read_iops`, `disk.{mount}.write_iops`, `disk.{mount}.read_bytes`, `disk.{mount}.write_bytes`

### Task 3.4: Network collector

**File:** `internal/agent/collect_net.go`

- [ ] Write test `internal/agent/collect_net_test.go`:
  - `TestNetCollector_Collect` — returns metrics for at least one non-loopback interface
  - `TestNetCollector_MetricNames` — metrics match pattern `net.{iface}.{rx_bytes|tx_bytes|rx_errors|tx_errors|rx_drops|tx_drops}`
  - `TestNetCollector_FiltersLoopback` — `lo` interface excluded
  - `TestNetCollector_FiltersVeth` — `veth*` and `docker*` bridge interfaces excluded
  - `TestNetCollector_Rates` — second collection produces rate values (bytes/sec) from delta
- [ ] Implement `NetCollector` using `gopsutil/v4`:
  - `net.IOCounters(true)` for per-interface counters
  - Filter out `lo`, `veth*`, `docker*`, `br-*` interfaces
  - Compute delta from previous call for rate metrics
  - Report both cumulative totals and per-second rates
- [ ] Run: `go test ./internal/agent/ -run TestNetCollector -v`
- [ ] Commit: `feat(agent): implement network metrics collector`

**Metrics produced per interface:**
- `net.{iface}.rx_bytes`, `net.{iface}.tx_bytes` — cumulative byte counters
- `net.{iface}.rx_bytes_sec`, `net.{iface}.tx_bytes_sec` — bytes per second (delta)
- `net.{iface}.rx_errors`, `net.{iface}.tx_errors` — error counters
- `net.{iface}.rx_drops`, `net.{iface}.tx_drops` — drop counters

### Task 3.5: Temperature collector

**File:** `internal/agent/collect_temp.go`

- [ ] Write test `internal/agent/collect_temp_test.go`:
  - `TestTempCollector_Collect` — on a machine with thermal zones, returns at least one `temp.*` metric
  - `TestTempCollector_NoSensors` — returns empty slice (not error) when no thermal zones found
  - `TestTempCollector_NameSanitization` — sensor names with spaces/special chars are sanitized to safe dotted names
- [ ] Implement `TempCollector` using `gopsutil/v4`:
  - `host.SensorsTemperatures()` — returns all available temperature sensors
  - Map sensor key to metric name: `temp.{sanitized_sensor_key}`
  - Special cases: `coretemp_Package id 0` or `k10temp_Tctl` map to `temp.cpu`
  - If no sensors found (common in VMs/containers), return empty slice — not an error
- [ ] Run: `go test ./internal/agent/ -run TestTempCollector -v`
- [ ] Commit: `feat(agent): implement temperature metrics collector`

**Metrics produced:**
- `temp.cpu` — CPU package temperature (mapped from common sensor names)
- `temp.gpu` — GPU temperature (if available)
- `temp.{sensor}` — any other detected thermal sensor

### Task 3.6: Docker collector

**File:** `internal/agent/collect_docker.go`

- [ ] Write test `internal/agent/collect_docker_test.go`:
  - `TestDockerCollector_NoDocker` — when Docker socket unavailable, returns empty slice (not error) with a logged warning
  - `TestDockerCollector_ParseStats` — given mock stats JSON response, correctly extracts CPU %, memory bytes
  - `TestDockerCollector_ContainerStatus` — running=1, exited/stopped=0
  - `TestDockerCollector_MetricNames` — metrics match `docker.{name}.{cpu_pct|mem_bytes|mem_limit_bytes|mem_usage_pct|status}`
  - `TestDockerCollector_CPUCalc` — verify CPU % calculation from delta of `cpu_stats` fields
- [ ] Implement `DockerCollector` using Docker SDK:
  - `client.NewClientWithOpts(client.FromEnv)` — uses `DOCKER_HOST` env (e.g., `tcp://docker-proxy:2375`)
  - `cli.ContainerList(ctx, ...)` — list running and stopped containers
  - `cli.ContainerStats(ctx, id, false)` — one-shot stats (stream=false)
  - Parse stats JSON to extract CPU delta and memory usage
  - Container name from `container.Names[0]` (strip leading `/`)
  - If Docker unreachable, log warning and return empty — don't fail the entire collection cycle

```go
// CPU % calculation from Docker stats:
// cpuDelta = stats.CPUStats.CPUUsage.TotalUsage - stats.PreCPUStats.CPUUsage.TotalUsage
// systemDelta = stats.CPUStats.SystemUsage - stats.PreCPUStats.SystemUsage
// cpuPercent = (cpuDelta / systemDelta) * numCPUs * 100.0
```

- [ ] Run: `go test ./internal/agent/ -run TestDockerCollector -v`
- [ ] Commit: `feat(agent): implement Docker container metrics collector`

**Metrics produced per container:**
- `docker.{name}.cpu_pct` — container CPU usage percentage
- `docker.{name}.mem_bytes` — container memory usage in bytes
- `docker.{name}.mem_limit_bytes` — container memory limit
- `docker.{name}.mem_usage_pct` — container memory usage as percentage of limit
- `docker.{name}.status` — 1 for running, 0 for stopped/exited

---

## Phase 4: Agent Orchestration and Push Client

### Task 4.1: HTTP push client

**File:** `internal/agent/push.go`

- [ ] Write test `internal/agent/push_test.go`:
  - `TestPushClient_Send` — sends POST to `/api/v1/agent/metrics` with correct JSON body and Authorization header
  - `TestPushClient_BearerToken` — Authorization header is `Bearer <key>`
  - `TestPushClient_Success` — 200 response returns nil error
  - `TestPushClient_Unauthorized` — 401 response returns specific error (bad key)
  - `TestPushClient_ServerError` — 500 response returns retriable error
  - `TestPushClient_Timeout` — request timeout (5s default) returns retriable error
  - `TestPushClient_ConnectionRefused` — hub unreachable returns retriable error
- [ ] Implement `PushClient`:

```go
type PushClient struct {
    hubURL  string
    apiKey  string
    client  *http.Client
}

func NewPushClient(hubURL, apiKey string) *PushClient
func (p *PushClient) Send(ctx context.Context, batch MetricBatch) error
```

- [ ] Use `httptest.NewServer` in tests for mock hub
- [ ] Run: `go test ./internal/agent/ -run TestPushClient -v`
- [ ] Commit: `feat(agent): implement HTTP push client for metric delivery`

### Task 4.2: Local metric buffer

**File:** `internal/agent/buffer.go`

- [ ] Write test `internal/agent/buffer_test.go`:
  - `TestBuffer_Add` — adds batch, Len() returns 1
  - `TestBuffer_Drain` — drains all batches in FIFO order, buffer empty after
  - `TestBuffer_MaxAge` — batches older than 5 minutes are dropped on next Add
  - `TestBuffer_MaxSize` — buffer capped at 10 batches (5min / 30s = 10); oldest dropped when full
  - `TestBuffer_Concurrent` — safe for concurrent Add/Drain from different goroutines
- [ ] Implement `MetricBuffer`:

```go
type MetricBuffer struct {
    mu       sync.Mutex
    batches  []MetricBatch
    maxAge   time.Duration  // default: 5 * time.Minute
    maxSize  int            // default: 10
}

func NewMetricBuffer(maxAge time.Duration, maxSize int) *MetricBuffer
func (b *MetricBuffer) Add(batch MetricBatch)
func (b *MetricBuffer) Drain() []MetricBatch
func (b *MetricBuffer) Len() int
```

- [ ] Run: `go test ./internal/agent/ -run TestBuffer -v`
- [ ] Commit: `feat(agent): implement local metric buffer for offline resilience`

### Task 4.3: Agent main loop

**File:** `internal/agent/agent.go`

- [ ] Write test `internal/agent/agent_test.go`:
  - `TestAgent_CollectsAndPushes` — with mock hub, agent collects metrics and sends batch within one interval
  - `TestAgent_BuffersOnFailure` — when hub returns 500, batch buffered; next successful push sends buffered + current
  - `TestAgent_FlushesBuffer` — after hub recovers, all buffered batches are sent in order
  - `TestAgent_GracefulShutdown` — cancelling context stops the agent loop cleanly
  - `TestAgent_SkipsDockerOnError` — if Docker collector fails, other collectors still run
- [ ] Implement `Agent` struct and `Run(ctx context.Context) error`:

```go
type Agent struct {
    config     *AgentConfig
    collectors []Collector
    push       *PushClient
    buffer     *MetricBuffer
    hostname   string
}

func New(cfg *AgentConfig) (*Agent, error)
func (a *Agent) Run(ctx context.Context) error
```

- [ ] Agent loop logic:
  1. Create all collectors (CPU, Mem, Disk, Net, Temp, Docker)
  2. Every `interval` (default 30s), run all collectors concurrently
  3. Combine results into a single `MetricBatch`
  4. Try to flush buffer first (send oldest buffered batches)
  5. Send current batch via PushClient
  6. On send failure, add batch to buffer
  7. On context cancellation, attempt one final flush
- [ ] Wire into `agent` cobra command: parse config, create Agent, run with signal-aware context
- [ ] Run: `go test ./internal/agent/ -run TestAgent -v`
- [ ] Commit: `feat(agent): implement agent main loop with collect-push-buffer cycle`

### Task 4.4: Host filesystem environment setup

**File:** `internal/agent/hostfs.go`

- [ ] Write test `internal/agent/hostfs_test.go`:
  - `TestSetupHostFS_SetsEnv` — given hostFS="/hostfs", sets HOST_PROC, HOST_SYS, HOST_ETC, HOST_VAR, HOST_RUN
  - `TestSetupHostFS_DefaultEmpty` — when hostFS is empty string, does not set env vars (bare-metal mode)
  - `TestSetupHostFS_AlreadySet` — does not override if env vars already set
- [ ] Implement `SetupHostFS(hostFS string)`:

```go
func SetupHostFS(hostFS string) {
    if hostFS == "" {
        return
    }
    envMap := map[string]string{
        "HOST_PROC":       filepath.Join(hostFS, "proc"),
        "HOST_SYS":        filepath.Join(hostFS, "sys"),
        "HOST_ETC":        filepath.Join(hostFS, "etc"),
        "HOST_VAR":        filepath.Join(hostFS, "var"),
        "HOST_RUN":        filepath.Join(hostFS, "run"),
        "HOST_DEV":        filepath.Join(hostFS, "dev"),
        "HOST_ROOT":       hostFS,
    }
    for k, v := range envMap {
        if os.Getenv(k) == "" {
            os.Setenv(k, v)
        }
    }
}
```

- [ ] Call from `Agent.New()` before creating collectors
- [ ] Run: `go test ./internal/agent/ -run TestSetupHostFS -v`
- [ ] Commit: `feat(agent): configure gopsutil host filesystem paths for containerized collection`

---

## Phase 5: Hub-Side Agent Metrics Receiver

### Task 5.1: Agent metrics POST handler (full implementation)

**File:** `internal/api/handlers_agent.go`

- [ ] Write test `internal/api/handlers_agent_test.go`:
  - `TestAgentMetricsHandler_ValidPayload` — valid JSON with bearer token returns 204, metrics stored
  - `TestAgentMetricsHandler_NoAuth` — missing Authorization header returns 401
  - `TestAgentMetricsHandler_BadKey` — invalid key returns 401
  - `TestAgentMetricsHandler_BadJSON` — malformed body returns 400
  - `TestAgentMetricsHandler_EmptyMetrics` — empty metrics array returns 400
  - `TestAgentMetricsHandler_UpdatesHeartbeat` — inserts/updates `agent_heartbeats` with current timestamp
  - `TestAgentMetricsHandler_UnknownHost` — key valid but host not in config returns 403
  - `TestAgentMetricsHandler_BroadcastsWebSocket` — metrics are broadcast to connected WS clients
- [ ] Implement full handler (replace Plan 2 stub):
  1. Validate `Authorization: Bearer <key>` against hashed keys in config
  2. Parse JSON body into `MetricBatch`
  3. Validate host matches the agent key's configured host name
  4. Insert metrics into `agent_metrics` table
  5. Update `agent_heartbeats` (upsert last_seen_at)
  6. Broadcast to WebSocket clients
  7. Return 204 No Content
- [ ] Run: `go test ./internal/api/ -run TestAgentMetricsHandler -v`
- [ ] Commit: `feat(api): implement full agent metrics POST handler`

### Task 5.2: Store methods for agent metrics

**File:** `internal/store/agent_metrics.go`

- [ ] Write test `internal/store/agent_metrics_test.go`:
  - `TestInsertAgentMetrics` — inserts batch of metrics, queryable afterward
  - `TestInsertAgentMetrics_BulkInsert` — 50 metrics in one call, all stored
  - `TestQueryAgentMetrics_TimeRange` — returns only metrics within from/to range
  - `TestQueryAgentMetrics_NameFilter` — filters by metric name prefix (e.g., "cpu" matches "cpu.usage_pct")
  - `TestQueryAgentMetrics_TierSelection` — <=48h uses raw, <=7d uses 5min, <=90d uses hourly, >90d uses daily
  - `TestUpsertHeartbeat` — inserts new host, updates existing host's last_seen_at
  - `TestGetStaleAgents` — returns agents with last_seen_at > threshold
- [ ] Implement store methods:

```go
func (s *Store) InsertAgentMetrics(host string, ts time.Time, metrics []Metric) error
func (s *Store) QueryAgentMetrics(host string, from, to time.Time, namePrefix string) ([]AgentMetricRow, error)
func (s *Store) UpsertHeartbeat(host string, ts time.Time) error
func (s *Store) GetStaleAgents(threshold time.Time) ([]StaleAgent, error)
```

- [ ] Use batch INSERT for `InsertAgentMetrics` (single transaction, prepared statement):
```sql
INSERT INTO agent_metrics (host, timestamp, metric_name, value)
VALUES (?, ?, ?, ?)
```

- [ ] `QueryAgentMetrics` tier selection logic:
```go
rangeDur := to.Sub(from)
switch {
case rangeDur <= 48*time.Hour:
    // query agent_metrics (raw)
case rangeDur <= 7*24*time.Hour:
    // query agent_metrics_5min
case rangeDur <= 90*24*time.Hour:
    // query agent_metrics_hourly
default:
    // query agent_metrics_daily
}
```

- [ ] Run: `go test ./internal/store/ -run TestInsertAgentMetrics -v && go test ./internal/store/ -run TestQueryAgentMetrics -v && go test ./internal/store/ -run TestUpsertHeartbeat -v && go test ./internal/store/ -run TestGetStaleAgents -v`
- [ ] Commit: `feat(store): add agent metrics insert, query with tier selection, and heartbeat methods`

### Task 5.3: Agent metrics 5-minute downsampling

**File:** `internal/hub/downsampler.go` (extend existing)

- [ ] Write test `internal/hub/downsampler_test.go` (add to existing):
  - `TestDownsample5Min` — given raw metrics spanning 10 minutes, produces 2 five-minute buckets with correct avg/min/max
  - `TestDownsample5Min_MultipleMetrics` — different metric names aggregated independently
  - `TestDownsample5Min_MultipleHosts` — different hosts aggregated independently
  - `TestDownsample5Min_Idempotent` — running twice doesn't duplicate rows
  - `TestPurgeRawAgentMetrics` — deletes raw metrics older than 48h, keeps newer ones
- [ ] Implement downsampling:

```go
func (d *Downsampler) AggregateAgentMetrics5Min(ctx context.Context) error
func (d *Downsampler) PurgeRawAgentMetrics(ctx context.Context, olderThan time.Time) error
```

- [ ] SQL for 5-min aggregation:
```sql
INSERT OR REPLACE INTO agent_metrics_5min (host, bucket, metric_name, avg, min, max)
SELECT host,
       (timestamp / 300) * 300 AS bucket,
       metric_name,
       AVG(value), MIN(value), MAX(value)
FROM agent_metrics
WHERE timestamp >= ? AND timestamp < ?
GROUP BY host, bucket, metric_name
```

- [ ] Register in hub scheduler: run every 5 minutes
- [ ] Run: `go test ./internal/hub/ -run TestDownsample5Min -v`
- [ ] Commit: `feat(hub): implement 5-minute agent metrics downsampling and raw purge`

---

## Phase 6: Prometheus Scraper (Hub-Side)

### Task 6.1: Prometheus text format parser

**File:** `internal/checks/scrape.go`

- [ ] Write test `internal/checks/scrape_test.go`:
  - `TestParsePrometheusText_Counter` — parses `node_cpu_seconds_total{cpu="0",mode="idle"} 12345.67` correctly
  - `TestParsePrometheusText_Gauge` — parses `node_memory_MemAvailable_bytes 1073741824` correctly
  - `TestParsePrometheusText_MultipleLines` — parses full node-exporter output with HELP/TYPE comments
  - `TestParsePrometheusText_Empty` — empty input returns empty map
  - `TestParsePrometheusText_Malformed` — malformed lines skipped, valid lines still parsed
- [ ] Implement using `github.com/prometheus/common/expfmt`:

```go
type PrometheusMetric struct {
    Name   string
    Labels map[string]string
    Value  float64
}

func ParsePrometheusText(reader io.Reader) ([]PrometheusMetric, error)
```

- [ ] Run: `go test ./internal/checks/ -run TestParsePrometheusText -v`
- [ ] Commit: `feat(scrape): implement Prometheus text exposition format parser`

### Task 6.2: Node-exporter metric mapper

**File:** `internal/checks/scrape_mapper.go`

- [ ] Write test `internal/checks/scrape_mapper_test.go`:
  - `TestMapCPU` — `node_cpu_seconds_total` with mode labels mapped to `cpu.usage_pct` (computed from rate)
  - `TestMapMemory` — `node_memory_MemAvailable_bytes` → `mem.available_bytes`, `node_memory_MemTotal_bytes` → `mem.total_bytes`
  - `TestMapDisk` — `node_filesystem_avail_bytes{mountpoint="/"}` → `disk./.avail_bytes`
  - `TestMapDisk_FiltersMounts` — excludes `/sys`, `/proc`, `/dev` mountpoints
  - `TestMapNetwork` — `node_network_receive_bytes_total{device="eth0"}` → `net.eth0.rx_bytes`
  - `TestMapNetwork_FiltersInterfaces` — excludes `lo`, `veth*`, `docker*`
  - `TestMapTemperature` — `node_hwmon_temp_celsius{chip="coretemp",sensor="temp1"}` → `temp.cpu`
  - `TestMapUnknown_Ignored` — unmapped metrics (e.g., `node_scrape_collector_duration_seconds`) produce no output
  - `TestMapCPURate` — second scrape produces correct CPU % from delta of cumulative seconds
- [ ] Implement `NodeExporterMapper`:

```go
type NodeExporterMapper struct {
    prevCPU map[string]float64  // previous cpu_seconds_total per cpu+mode for rate calc
    prevNet map[string]float64  // previous net bytes per iface for rate calc
    prevTS  time.Time
}

func NewNodeExporterMapper() *NodeExporterMapper
func (m *NodeExporterMapper) Map(metrics []PrometheusMetric) []Metric
```

- [ ] CPU rate calculation:
  - Sum all `node_cpu_seconds_total` by mode per CPU
  - `idle_delta = current_idle - prev_idle`
  - `total_delta = current_total - prev_total`
  - `usage_pct = (1 - idle_delta/total_delta) * 100`

- [ ] Mapping table:

| Prometheus Metric | Labels Used | WhatsUpp Metric |
|---|---|---|
| `node_cpu_seconds_total` | (aggregated) | `cpu.usage_pct` (rate) |
| `node_load1` | | `cpu.load_1m` |
| `node_load5` | | `cpu.load_5m` |
| `node_load15` | | `cpu.load_15m` |
| `node_memory_MemTotal_bytes` | | `mem.total_bytes` |
| `node_memory_MemAvailable_bytes` | | `mem.available_bytes` |
| `node_memory_MemFree_bytes` | | `mem.free_bytes` |
| `node_memory_SwapTotal_bytes` | | `mem.swap_total_bytes` |
| `node_memory_SwapFree_bytes` | | `mem.swap_used_bytes` (total - free) |
| `node_filesystem_size_bytes` | `mountpoint` | `disk.{mount}.total_bytes` |
| `node_filesystem_avail_bytes` | `mountpoint` | `disk.{mount}.avail_bytes` |
| `node_filesystem_free_bytes` | `mountpoint` | `disk.{mount}.free_bytes` |
| `node_network_receive_bytes_total` | `device` | `net.{iface}.rx_bytes` |
| `node_network_transmit_bytes_total` | `device` | `net.{iface}.tx_bytes` |
| `node_network_receive_errs_total` | `device` | `net.{iface}.rx_errors` |
| `node_network_transmit_errs_total` | `device` | `net.{iface}.tx_errors` |
| `node_hwmon_temp_celsius` | `chip`, `sensor` | `temp.cpu` (mapped) |

- [ ] Run: `go test ./internal/checks/ -run TestMap -v`
- [ ] Commit: `feat(scrape): implement node-exporter metric mapper to whatsupp naming`

### Task 6.3: Scrape check integration

**File:** `internal/checks/scrape.go` (extend)

- [ ] Write test `internal/checks/scrape_test.go` (add to existing):
  - `TestScrapeCheck_Execute` — given mock HTTP server returning node-exporter text, produces mapped metrics
  - `TestScrapeCheck_Timeout` — slow target returns error after timeout
  - `TestScrapeCheck_BadStatus` — non-200 response returns error
  - `TestScrapeCheck_StoresMetrics` — scraped and mapped metrics are inserted into agent_metrics table
  - `TestScrapeCheck_UpdatesHeartbeat` — scrape updates agent_heartbeats for the target's host name
- [ ] Implement `ScrapeCheck`:

```go
type ScrapeCheck struct {
    name    string
    url     string
    mapper  *NodeExporterMapper
    store   *store.Store
    client  *http.Client
}

func NewScrapeCheck(name, url string, store *store.Store) *ScrapeCheck
func (s *ScrapeCheck) Execute(ctx context.Context) error
```

- [ ] Scrape flow:
  1. HTTP GET to target URL (5s timeout)
  2. Parse Prometheus text response
  3. Map to whatsupp metrics via `NodeExporterMapper`
  4. Insert into `agent_metrics` table (same table as agent-pushed metrics)
  5. Update `agent_heartbeats`
- [ ] Wire into hub scheduler: create ScrapeCheck for each `scrape_targets` entry in config
- [ ] Run: `go test ./internal/checks/ -run TestScrapeCheck -v`
- [ ] Commit: `feat(scrape): integrate Prometheus scraper with hub scheduler and store`

---

## Phase 7: Agent Staleness Detection

### Task 7.1: Staleness checker

**File:** `internal/hub/staleness.go`

- [ ] Write test `internal/hub/staleness_test.go`:
  - `TestStalenessCheck_NoStaleAgents` — all agents recently seen, no incidents created
  - `TestStalenessCheck_OneStale` — one agent not seen for >5 min, incident created with cause "no metrics from {host} for 5 minutes"
  - `TestStalenessCheck_AlreadyIncident` — existing open incident for stale agent, no duplicate created
  - `TestStalenessCheck_Recovery` — previously stale agent sends metrics, open incident resolved
  - `TestStalenessCheck_NtfyAlert` — stale detection triggers ntfy alert with priority 4 (high)
  - `TestStalenessCheck_NtfyRecovery` — recovery triggers ntfy alert with priority 3 (default)
  - `TestStalenessCheck_ConfigurableThreshold` — uses config value for staleness threshold (default 5m)
- [ ] Implement `StalenessChecker`:

```go
type StalenessChecker struct {
    store     *store.Store
    alerter   *alerting.Alerter
    config    *config.Config
    threshold time.Duration  // default: 5 * time.Minute
}

func NewStalenessChecker(store *store.Store, alerter *alerting.Alerter, cfg *config.Config) *StalenessChecker
func (s *StalenessChecker) Check(ctx context.Context) error
```

- [ ] Check logic:
  1. `store.GetStaleAgents(time.Now().Add(-threshold))` — get agents with old heartbeats
  2. For each stale agent:
     a. Check if open incident already exists for this host (monitor = "agent:{host}")
     b. If not, create incident and send ntfy alert
  3. For each agent with recent heartbeat that has an open incident:
     a. Resolve the incident
     b. Send ntfy recovery alert
- [ ] Register in hub scheduler: run every 60 seconds
- [ ] Run: `go test ./internal/hub/ -run TestStalenessCheck -v`
- [ ] Commit: `feat(hub): implement agent staleness detection with incident creation and ntfy alerts`

### Task 7.2: Store methods for staleness incidents

**File:** `internal/store/incidents.go` (extend existing)

- [ ] Write test `internal/store/incidents_test.go` (add to existing):
  - `TestGetOpenIncidentForMonitor` — returns open incident (resolved_at IS NULL) for given monitor name
  - `TestGetOpenIncidentForMonitor_None` — returns nil when no open incident
  - `TestResolveIncident` — sets resolved_at on the incident
  - `TestCreateIncident_Agent` — creates incident with monitor name "agent:plexypi"
- [ ] Implement:

```go
func (s *Store) GetOpenIncidentForMonitor(monitor string) (*Incident, error)
func (s *Store) CreateIncident(monitor string, startedAt time.Time, cause string) (int64, error)
func (s *Store) ResolveIncident(id int64, resolvedAt time.Time) error
```

- [ ] Run: `go test ./internal/store/ -run TestGetOpenIncident -v && go test ./internal/store/ -run TestResolveIncident -v`
- [ ] Commit: `feat(store): add incident query and resolution methods for staleness detection`

---

## Phase 8: API Endpoints for Host Metrics

### Task 8.1: GET /api/v1/hosts endpoint

**File:** `internal/api/handlers_hosts.go`

- [ ] Write test `internal/api/handlers_hosts_test.go`:
  - `TestHostsListHandler` — returns list of configured agents and scrape targets with last-seen timestamps
  - `TestHostsListHandler_WithStatus` — includes "ok" or "stale" status based on heartbeat age
  - `TestHostsListHandler_Empty` — no agents configured returns empty array
  - `TestHostsListHandler_RequiresAuth` — unauthenticated request returns 401
- [ ] Implement: query `agent_heartbeats`, merge with config agents/scrape_targets, return JSON
- [ ] Register route in router
- [ ] Run: `go test ./internal/api/ -run TestHostsListHandler -v`
- [ ] Commit: `feat(api): implement GET /api/v1/hosts endpoint`

### Task 8.2: GET /api/v1/hosts/:name endpoint

**File:** `internal/api/handlers_hosts.go` (extend)

- [ ] Write test `internal/api/handlers_hosts_test.go` (add):
  - `TestHostDetailHandler` — returns host detail with last-seen, status, and latest values for top-level metrics
  - `TestHostDetailHandler_NotFound` — unknown host returns 404
  - `TestHostDetailHandler_RequiresAuth` — returns 401
- [ ] Implement: query latest metric values for host, return JSON
- [ ] Register route
- [ ] Run: `go test ./internal/api/ -run TestHostDetailHandler -v`
- [ ] Commit: `feat(api): implement GET /api/v1/hosts/:name endpoint`

### Task 8.3: GET /api/v1/hosts/:name/metrics endpoint

**File:** `internal/api/handlers_hosts.go` (extend)

- [ ] Write test `internal/api/handlers_hosts_test.go` (add):
  - `TestHostMetricsHandler_RawTier` — from/to within 48h returns raw data points
  - `TestHostMetricsHandler_5MinTier` — from/to within 7d returns 5-min aggregates
  - `TestHostMetricsHandler_HourlyTier` — from/to within 90d returns hourly aggregates
  - `TestHostMetricsHandler_DailyTier` — from/to beyond 90d returns daily aggregates
  - `TestHostMetricsHandler_NameFilter` — `?names=cpu,mem` filters to only cpu.* and mem.* metrics
  - `TestHostMetricsHandler_DefaultTimeRange` — missing from/to defaults to last 1 hour
  - `TestHostMetricsHandler_RequiresAuth` — returns 401
- [ ] Implement: parse query params, call `store.QueryAgentMetrics`, return JSON
- [ ] Register route
- [ ] Run: `go test ./internal/api/ -run TestHostMetricsHandler -v`
- [ ] Commit: `feat(api): implement GET /api/v1/hosts/:name/metrics with automatic tier selection`

---

## Phase 9: Integration Tests

### Task 9.1: Agent end-to-end test

**File:** `internal/agent/integration_test.go`

- [ ] Write integration test (build tag `//go:build integration`):
  - `TestAgent_EndToEnd` — starts mock hub server, creates agent with all collectors, runs one collection cycle, verifies:
    - HTTP POST received at `/api/v1/agent/metrics`
    - JSON body contains host, timestamp, and non-empty metrics array
    - At least cpu.usage_pct, mem.usage_pct, and one disk metric present
    - Bearer token in Authorization header
- [ ] Run: `go test ./internal/agent/ -tags integration -run TestAgent_EndToEnd -v`
- [ ] Commit: `test(agent): add end-to-end integration test`

### Task 9.2: Scrape end-to-end test

**File:** `internal/checks/integration_test.go`

- [ ] Write integration test (build tag `//go:build integration`):
  - `TestScrape_EndToEnd` — starts mock node-exporter serving real sample output, runs scrape check, verifies:
    - Metrics stored in SQLite
    - Correct metric names (cpu.usage_pct, mem.available_bytes, etc.)
    - Heartbeat updated
- [ ] Include sample node-exporter output as test fixture in `testdata/node_exporter_sample.txt`
- [ ] Run: `go test ./internal/checks/ -tags integration -run TestScrape_EndToEnd -v`
- [ ] Commit: `test(scrape): add end-to-end integration test with sample node-exporter output`

### Task 9.3: Agent staleness end-to-end test

**File:** `internal/hub/integration_test.go`

- [ ] Write integration test (build tag `//go:build integration`):
  - `TestStaleness_EndToEnd` — insert heartbeat with old timestamp, run staleness check, verify:
    - Incident created with correct cause text
    - Mock ntfy server received alert
    - Update heartbeat to now, run check again, incident resolved
    - Mock ntfy server received recovery
- [ ] Run: `go test ./internal/hub/ -tags integration -run TestStaleness_EndToEnd -v`
- [ ] Commit: `test(hub): add staleness detection end-to-end integration test`

---

## Phase 10: Go Module Dependencies

### Task 10.1: Add new dependencies

- [ ] Run:
```bash
cd /home/andyhazz/projects/whatsupp
go get github.com/shirou/gopsutil/v4@latest
go get github.com/docker/docker/client@latest
go get github.com/docker/docker/api/types@latest
go get github.com/prometheus/common/expfmt@latest
go mod tidy
```
- [ ] Verify no conflicting transitive dependencies
- [ ] Commit: `chore(deps): add gopsutil, docker sdk, and prometheus expfmt`

**Note:** Run this task first if implementing sequentially, so imports resolve. Listed last in the plan because it's a mechanical step — the plan is ordered by logical dependency, not execution order.

---

## File Summary

| File | Purpose |
|---|---|
| `internal/agent/config.go` | Agent YAML config parsing |
| `internal/agent/init.go` | `whatsupp agent init` config generation |
| `internal/agent/metric.go` | Metric types and naming convention |
| `internal/agent/collector.go` | Collector interface |
| `internal/agent/collect_cpu.go` | CPU metrics via gopsutil |
| `internal/agent/collect_mem.go` | Memory metrics via gopsutil |
| `internal/agent/collect_disk.go` | Disk metrics via gopsutil |
| `internal/agent/collect_net.go` | Network metrics via gopsutil |
| `internal/agent/collect_temp.go` | Temperature metrics via gopsutil |
| `internal/agent/collect_docker.go` | Docker container metrics via Docker SDK |
| `internal/agent/push.go` | HTTP push client |
| `internal/agent/buffer.go` | Local metric buffer |
| `internal/agent/agent.go` | Agent orchestration loop |
| `internal/agent/hostfs.go` | Host filesystem env setup for containers |
| `internal/checks/scrape.go` | Prometheus text parser + scrape check |
| `internal/checks/scrape_mapper.go` | Node-exporter to whatsupp metric mapper |
| `internal/api/handlers_agent.go` | POST /api/v1/agent/metrics handler |
| `internal/api/handlers_hosts.go` | GET /api/v1/hosts endpoints |
| `internal/store/agent_metrics.go` | Agent metrics store methods |
| `internal/store/incidents.go` | Incident store methods (extended) |
| `internal/hub/downsampler.go` | 5-min downsampling (extended) |
| `internal/hub/staleness.go` | Agent staleness checker |
| `cmd/whatsupp/agent.go` | Agent CLI subcommands |

## Dependency Graph

```
Phase 10 (deps) ─── can run first or in parallel with Phase 1
    │
Phase 1 (config, CLI)
    │
Phase 2 (metric types) ─── no external deps
    │
    ├─── Phase 3 (collectors) ─── each collector independent, can parallelize
    │        │
    │    Phase 4 (push, buffer, agent loop)
    │
    ├─── Phase 5 (hub receiver, store, downsampling)
    │        │
    │    Phase 6 (scraper) ─── depends on store methods from 5.2
    │        │
    │    Phase 7 (staleness) ─── depends on store from 5.2 and incidents from 7.2
    │
    └─── Phase 8 (host API endpoints) ─── depends on store from 5.2
              │
         Phase 9 (integration tests) ─── depends on all above
```

## Verification Checklist

After all tasks complete:

- [ ] `go test ./internal/agent/... -v` — all agent tests pass
- [ ] `go test ./internal/checks/... -v` — all scrape tests pass
- [ ] `go test ./internal/hub/... -v` — all downsampling and staleness tests pass
- [ ] `go test ./internal/store/... -v` — all store tests pass
- [ ] `go test ./internal/api/... -v` — all API handler tests pass
- [ ] `go test ./cmd/whatsupp/... -v` — CLI tests pass
- [ ] `go test ./... -tags integration -v` — integration tests pass
- [ ] `go vet ./...` — no vet issues
- [ ] `go build ./cmd/whatsupp` — binary builds successfully
- [ ] Manual: `./whatsupp agent init --hub http://localhost:8080 --key test123` creates valid config
- [ ] Manual: `./whatsupp agent` starts collecting and pushing metrics
