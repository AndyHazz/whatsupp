# Container Monitoring & Battery Collector

**Date:** 2026-03-21
**Status:** Approved

## Goal

Add per-container resource visibility (CPU, RAM, network I/O, disk I/O) with a dedicated container table + drill-down on the HostDetail page, and add battery charge reporting for hosts that have one.

## Agent Changes

### New: Battery Collector (`collect_battery.go`)

Reads Linux sysfs at `/sys/class/power_supply/BAT*/`:

| Metric | Source | Description |
|--------|--------|-------------|
| `battery.charge_pct` | `capacity` file | 0-100 charge percentage |
| `battery.charging` | `status` file | 1.0 if Charging/Full, 0.0 if Discharging/Not charging |

- Non-fatal if no battery exists (returns nil, nil)
- New `BatteryMetric(name)` helper in `metric.go`
- Added to collector list in `agent.go`

### Modified: Docker Collector (`collect_docker.go`)

Extract network and block I/O from the existing `types.StatsJSON` response. Add previous-sample state tracking for rate calculation (same pattern as `NetCollector`).

**New metrics per container (totals, summed across all interfaces/devices):**

| Metric | Description |
|--------|-------------|
| `docker.{name}.net_rx_bytes` | Cumulative received bytes |
| `docker.{name}.net_tx_bytes` | Cumulative transmitted bytes |
| `docker.{name}.net_rx_bytes_sec` | Receive rate (bytes/sec) |
| `docker.{name}.net_tx_bytes_sec` | Transmit rate (bytes/sec) |
| `docker.{name}.disk_read_bytes` | Cumulative disk read bytes |
| `docker.{name}.disk_write_bytes` | Cumulative disk write bytes |
| `docker.{name}.disk_read_bytes_sec` | Disk read rate (bytes/sec) |
| `docker.{name}.disk_write_bytes_sec` | Disk write rate (bytes/sec) |

**State tracking additions:**
- `mu sync.Mutex`
- `prevNet map[string]netSnapshot` (rx/tx bytes per container)
- `prevBlkio map[string]blkioSnapshot` (read/write bytes per container)
- `prevTime time.Time`

## Frontend Changes

### HostDetail: Container Table (replaces Docker chart section)

**Table columns:** Status (dot), Name, CPU %, Memory (used/limit), Net I/O (rx/tx rate), Disk I/O (r/w rate)

- Status dot: green = running, red = stopped
- Sorted by name, stopped containers at bottom and dimmed
- Sortable by any column

**Row expansion (accordion):**
- Click a row to expand inline showing 4 small charts:
  - CPU % over time
  - Memory bytes over time (with limit line)
  - Network rx/tx bytes/sec
  - Disk read/write bytes/sec
- Time range follows existing TimeRangeSelector
- Uses existing `Chart.svelte` component

**Real-time updates:** Table values update via existing WebSocket `agent_metric` messages. No new plumbing.

**Data fetching:** Fetch with `names=docker` prefix (already supported). Parse container names from metric keys.

### HostDetail: Battery Gauge

- New gauge alongside CPU/RAM/Disk at top of page
- Only rendered when `battery.charge_pct` exists in metrics
- Charging indicator text below gauge
- Color: green (charging or >20%), yellow (10-20%), red (<10%)
- New `battery` chart category for charge over time

### Hosts Page: Battery Indicator

- Small battery percentage + charging/discharging indicator on host cards
- Only shown for hosts with battery metrics
- No empty space on hosts without batteries

### New Component: `ContainerTable.svelte`

Encapsulates the container table + accordion drill-down logic. Receives `latestMetrics` and chart data as props.

## What Does NOT Change

- API endpoints (metrics flow generically by name)
- Database schema (metric names are strings)
- WebSocket layer (broadcasts all agent metrics)
- Downsampler (aggregates any metric name)
- Retention policies

## Files Changed

| File | Action |
|------|--------|
| `internal/agent/collect_battery.go` | New |
| `internal/agent/collect_docker.go` | Modify — add net/disk I/O + rate tracking |
| `internal/agent/metric.go` | Modify — add `BatteryMetric()` |
| `internal/agent/agent.go` | Modify — add battery collector |
| `frontend/src/pages/HostDetail.svelte` | Modify — battery gauge, container panel |
| `frontend/src/pages/Hosts.svelte` | Modify — battery indicator on cards |
| `frontend/src/components/ContainerTable.svelte` | New |
