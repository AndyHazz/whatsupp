# Navigation Rework + Host Detail Layout Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split Overview into a real dashboard + dedicated Monitors page, rework Host Detail layout to put gauges in header and time range in a slim bar.

**Architecture:** Rename current Overview.svelte → Monitors.svelte (unchanged content). Write new Overview.svelte as a dashboard with summary tiles + live activity feed. Rework HostDetail.svelte template/CSS to move gauges into header and add time-range bar. Update router and nav.

**Tech Stack:** Svelte (`export let`, `$:`, `on:click` patterns), custom SPA router, WebSocket live events, existing REST API.

**Spec:** `docs/superpowers/specs/2026-03-21-nav-rework-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `frontend/src/pages/Monitors.svelte` | **Create** | Current Overview content (rename/copy) |
| `frontend/src/pages/Overview.svelte` | **Rewrite** | New dashboard: summary tiles + activity feed |
| `frontend/src/pages/HostDetail.svelte` | Modify | Gauges in header, slim time-range bar |
| `frontend/src/components/Layout.svelte` | Modify | Add Monitors nav item |
| `frontend/src/App.svelte` | Modify | Add `/monitors` route |

---

### Task 1: Rename Overview → Monitors + Route Wiring

Move the current uptime cards page to its new home before rewriting Overview.

**Files:**
- Create: `frontend/src/pages/Monitors.svelte`
- Modify: `frontend/src/App.svelte`
- Modify: `frontend/src/components/Layout.svelte`

- [ ] **Step 1: Copy Overview.svelte to Monitors.svelte**

Copy `frontend/src/pages/Overview.svelte` to `frontend/src/pages/Monitors.svelte`. Then change the heading inside from `<h1>Overview</h1>` to `<h1>Monitors</h1>`, and the wrapper class from `class="overview"` to `class="monitors"`, and the CSS selector `.overview h1` to `.monitors h1`.

- [ ] **Step 2: Update App.svelte routing**

Import the new Monitors component and add the `/monitors` route. The full updated `App.svelte`:

```svelte
<script>
  import { path, matchRoute } from './lib/router.js';
  import Layout from './components/Layout.svelte';
  import Login from './pages/Login.svelte';
  import Overview from './pages/Overview.svelte';
  import Monitors from './pages/Monitors.svelte';
  import MonitorDetail from './pages/MonitorDetail.svelte';
  import Hosts from './pages/Hosts.svelte';
  import HostDetail from './pages/HostDetail.svelte';
  import Security from './pages/Security.svelte';
  import Incidents from './pages/Incidents.svelte';
  import Settings from './pages/Settings.svelte';

  import { isAuthenticated } from './lib/auth.js';
  import { connect, disconnect } from './lib/ws.js';

  let wasAuth = false;
  $: if ($isAuthenticated && !wasAuth) {
    connect();
    wasAuth = true;
  } else if (!$isAuthenticated && wasAuth) {
    disconnect();
    wasAuth = false;
  }

  $: currentPath = $path;
  $: monitorMatch = matchRoute('/monitors/:name', currentPath);
  $: hostMatch = matchRoute('/hosts/:name', currentPath);
</script>

{#if !$isAuthenticated}
  <Login />
{:else}
  <Layout>
    {#if currentPath === '/'}
      <Overview />
    {:else if currentPath === '/monitors'}
      <Monitors />
    {:else if monitorMatch}
      <MonitorDetail name={monitorMatch.name} />
    {:else if currentPath === '/hosts'}
      <Hosts />
    {:else if hostMatch}
      <HostDetail name={hostMatch.name} />
    {:else if currentPath === '/security'}
      <Security />
    {:else if currentPath === '/incidents'}
      <Incidents />
    {:else if currentPath === '/settings'}
      <Settings />
    {:else}
      <Overview />
    {/if}
  </Layout>
{/if}
```

Note: `/monitors` must come before `monitorMatch` (which matches `/monitors/:name`). This is safe because `matchRoute('/monitors/:name', '/monitors')` returns `null` (different segment count).

- [ ] **Step 3: Update Layout.svelte nav items**

Replace the `navItems` array in `Layout.svelte`:

```js
const navItems = [
  { path: '/',          label: 'Overview',   icon: '&#9673;' },
  { path: '/monitors',  label: 'Monitors',   icon: '&#9672;' },
  { path: '/hosts',     label: 'Hosts',      icon: '&#9881;' },
  { path: '/security',  label: 'Security',   icon: '&#9888;' },
  { path: '/incidents', label: 'Incidents',   icon: '&#9889;' },
  { path: '/settings',  label: 'Settings',   icon: '&#9881;' },
];
```

- [ ] **Step 4: Verify routing works**

Open the app. Click "Overview" → should show the old overview content (temporarily, until Task 2 rewrites it). Click "Monitors" → should show the same uptime cards. Click a monitor card → should navigate to `/monitors/:name` detail page.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/Monitors.svelte frontend/src/App.svelte frontend/src/components/Layout.svelte
git commit -m "feat: add Monitors page, update routing and nav"
```

---

### Task 2: New Overview Dashboard

Rewrite Overview.svelte as a dashboard with summary tiles and live activity feed.

**Files:**
- Rewrite: `frontend/src/pages/Overview.svelte`

- [ ] **Step 1: Write the new Overview.svelte**

Complete replacement:

```svelte
<script>
  import { onMount, onDestroy } from 'svelte';
  import { link } from '../lib/router.js';
  import { api } from '../lib/api.js';
  import { onMessage } from '../lib/ws.js';

  let monitors = [];
  let hosts = [];
  let newPortCount = 0;
  let activeIncidents = 0;
  let loading = true;
  let error = '';

  // Live activity feed
  let events = [];
  const MAX_EVENTS = 10;

  // Hosts metrics from WS
  let hostMetrics = {}; // { hostname: { 'cpu.usage_pct': val, ... } }

  onMount(async () => {
    try {
      const [m, h, scans, baselines, incidents] = await Promise.all([
        api.getMonitors(),
        api.getHosts(),
        api.getScans(),
        api.getBaselines(),
        api.getIncidents(),
      ]);

      monitors = m || [];
      hosts = h || [];
      activeIncidents = (incidents || []).filter(i => !i.resolved_at).length;

      // Security diff — count new ports across all targets
      const baselineMap = {};
      for (const bl of (baselines || [])) {
        const ports = typeof bl.expected_ports_json === 'string'
          ? JSON.parse(bl.expected_ports_json || '[]')
          : (bl.expected_ports_json || []);
        baselineMap[bl.target] = new Set(ports);
      }
      // Get latest scan per target
      const latestScans = {};
      for (const s of (scans || [])) {
        if (!latestScans[s.target] || s.timestamp > latestScans[s.target].timestamp) {
          latestScans[s.target] = s;
        }
      }
      let portCount = 0;
      for (const [target, scan] of Object.entries(latestScans)) {
        const openPorts = typeof scan.open_ports_json === 'string'
          ? JSON.parse(scan.open_ports_json || '[]')
          : (scan.open_ports_json || []);
        const expected = baselineMap[target] || new Set();
        portCount += openPorts.filter(p => !expected.has(p)).length;
      }
      newPortCount = portCount;
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  });

  // Live check results → update monitor counts + activity feed
  const unsub1 = onMessage('check_result', (data) => {
    // Update monitor status in our list
    monitors = monitors.map(m => {
      if (m.name === data.monitor) return { ...m, status: data.status, latency_ms: data.latency_ms };
      return m;
    });

    const status = data.status === 'up' ? 'UP' : 'DOWN';
    const latency = data.latency_ms != null ? ` ${Math.round(data.latency_ms)}ms` : '';
    addEvent(status === 'UP' ? 'check' : 'alert', `${data.monitor} — ${status}${latency}`, status);
  });

  // Live incidents → update count + activity feed
  const unsub2 = onMessage('incident', (data) => {
    if (data.resolved_at) {
      activeIncidents = Math.max(0, activeIncidents - 1);
      const dur = formatDuration(data.resolved_at - data.started_at);
      addEvent('resolve', `Incident resolved: ${data.monitor} (${dur})`, 'resolved');
    } else {
      activeIncidents++;
      addEvent('alert', `Incident started: ${data.monitor}`, 'down');
    }
  });

  // Live agent metrics → update host summary
  const unsub3 = onMessage('agent_metric', (data) => {
    if (!data.host) return;
    if (!hostMetrics[data.host]) hostMetrics[data.host] = {};
    for (const m of (data.metrics || [])) {
      hostMetrics[data.host][m.name] = m.value;
    }
    hostMetrics = hostMetrics;
  });

  onDestroy(() => { unsub1(); unsub2(); unsub3(); });

  function addEvent(icon, text, status) {
    const now = new Date();
    const time = now.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
    events = [{ icon, text, status, time }, ...events.slice(0, MAX_EVENTS - 1)];
  }

  function formatDuration(secs) {
    if (secs < 60) return `${secs}s`;
    if (secs < 3600) return `${Math.floor(secs / 60)}m ${secs % 60}s`;
    return `${Math.floor(secs / 3600)}h ${Math.floor((secs % 3600) / 60)}m`;
  }

  $: upCount = monitors.filter(m => m.status === 'up').length;
  $: downCount = monitors.filter(m => m.status === 'down').length;

  // Find highest CPU across all hosts from WS data
  $: maxCpu = Object.values(hostMetrics).reduce((max, m) => {
    const v = m['cpu.usage_pct'];
    return v != null && v > max ? v : max;
  }, 0);
  $: maxRam = Object.values(hostMetrics).reduce((max, m) => {
    const v = m['mem.usage_pct'];
    return v != null && v > max ? v : max;
  }, 0);
</script>

<div class="overview">
  <h1>Overview</h1>

  {#if loading}
    <div class="tiles">
      {#each Array(4) as _}
        <div class="tile skeleton-tile">
          <div class="skel" style="width:60%;height:18px;"></div>
          <div class="skel" style="width:40%;height:28px;margin-top:8px;"></div>
        </div>
      {/each}
    </div>
  {:else if error}
    <p class="error">{error}</p>
  {:else}
    <div class="tiles">
      <a href="/monitors" use:link class="tile" class:alert={downCount > 0}>
        <div class="tile-label">Monitors</div>
        <div class="tile-value">
          <span class="up">{upCount} up</span>
          {#if downCount > 0}
            <span class="separator">/</span>
            <span class="down">{downCount} down</span>
          {/if}
        </div>
      </a>

      <a href="/hosts" use:link class="tile">
        <div class="tile-label">Hosts</div>
        <div class="tile-value">{hosts.length} reporting</div>
        {#if maxCpu > 0}
          <div class="tile-detail">CPU {Math.round(maxCpu)}% &middot; RAM {Math.round(maxRam)}%</div>
        {/if}
      </a>

      <a href="/security" use:link class="tile" class:alert={newPortCount > 0}>
        <div class="tile-label">Security</div>
        <div class="tile-value">
          {#if newPortCount > 0}
            <span class="warn">{newPortCount} new port{newPortCount !== 1 ? 's' : ''}</span>
          {:else}
            <span class="ok">All clear</span>
          {/if}
        </div>
      </a>

      <a href="/incidents" use:link class="tile" class:alert={activeIncidents > 0}>
        <div class="tile-label">Incidents</div>
        <div class="tile-value">
          {#if activeIncidents > 0}
            <span class="down">{activeIncidents} ongoing</span>
          {:else}
            <span class="ok">No active incidents</span>
          {/if}
        </div>
      </a>
    </div>

    <div class="activity-card">
      <h2>Recent Activity</h2>
      {#if events.length === 0}
        <p class="muted">Waiting for events...</p>
      {:else}
        <div class="event-list">
          {#each events as evt}
            <div class="event" class:event-up={evt.status === 'UP'} class:event-down={evt.status === 'down' || evt.status === 'DOWN'} class:event-resolved={evt.status === 'resolved'}>
              <span class="event-time">{evt.time}</span>
              <span class="event-text">{evt.text}</span>
            </div>
          {/each}
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .overview h1 { margin-bottom: var(--gap); }

  .tiles {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
    gap: var(--gap);
    margin-bottom: var(--gap);
  }

  .tile {
    background: var(--bg-card);
    border-radius: var(--radius);
    padding: 20px;
    text-decoration: none;
    color: var(--fg);
    border: 1px solid var(--border-subtle);
    box-shadow: var(--shadow-card);
    transition: transform 0.15s ease, box-shadow 0.15s ease, border-color 0.15s ease, background 0.15s ease;
  }
  .tile:hover {
    transform: translateY(-1px);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3), 0 0 0 1px rgba(189, 147, 249, 0.1);
    border-color: rgba(189, 147, 249, 0.3);
    background: var(--bg-card-hover);
    text-decoration: none;
  }
  .tile.alert {
    border-left: 3px solid var(--red);
  }

  .tile-label {
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.8px;
    color: var(--fg-muted);
    margin-bottom: 6px;
  }
  .tile-value {
    font-size: 1.3rem;
    font-weight: 700;
  }
  .tile-detail {
    font-size: 0.8rem;
    color: var(--fg-muted);
    margin-top: 4px;
  }

  .up { color: var(--green); }
  .down { color: var(--red); }
  .warn { color: var(--orange); }
  .ok { color: var(--green); }
  .separator { color: var(--fg-muted); margin: 0 4px; }

  .skeleton-tile {
    min-height: 80px;
  }

  .activity-card {
    background: var(--bg-card);
    border-radius: var(--radius);
    padding: 16px;
    border: 1px solid var(--border-subtle);
    box-shadow: var(--shadow-card);
  }
  .activity-card h2 {
    font-size: 1.1rem;
    margin-bottom: 12px;
  }

  .event-list {
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .event {
    display: flex;
    gap: 12px;
    padding: 6px 8px;
    border-radius: 4px;
    font-size: 0.85rem;
  }
  .event:hover {
    background: rgba(248, 248, 242, 0.04);
  }
  .event-time {
    color: var(--fg-muted);
    font-size: 0.8rem;
    white-space: nowrap;
  }
  .event-up .event-text { color: var(--green); }
  .event-down .event-text { color: var(--red); }
  .event-resolved .event-text { color: var(--cyan); }

  .muted { color: var(--fg-muted); }
  .error { color: var(--red); }
</style>
```

- [ ] **Step 2: Verify the dashboard**

Open `/` — should show 4 summary tiles and "Waiting for events..." activity feed. Tiles should be clickable. Wait ~15s for WS data to populate hosts CPU/RAM and activity feed.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/Overview.svelte
git commit -m "feat: new Overview dashboard with summary tiles and live activity feed"
```

---

### Task 3: Host Detail Layout Rework

Move gauges into header, add slim time-range bar.

**Files:**
- Modify: `frontend/src/pages/HostDetail.svelte`

- [ ] **Step 1: Rework the template**

Replace the content between `{:else if host}` and the closing `{/if}` (lines 139–187) with:

```svelte
  {:else if host}
    <div class="header">
      <div class="header-left">
        <h1>{host.host}</h1>
        <span class="last-seen">Last seen: {formatLastSeen(host.last_seen_at)}</span>
      </div>
      <div class="header-gauges">
        {#if latestMetrics['cpu.usage_pct'] != null}
          <Gauge value={latestMetrics['cpu.usage_pct']} label="CPU" size={56} />
        {/if}
        {#if latestMetrics['mem.usage_pct'] != null}
          <div class="gauge-with-detail">
            <Gauge value={latestMetrics['mem.usage_pct']} label="RAM" size={56} />
            {#if latestMetrics['mem.total_bytes']}
              <span class="detail">{fmtBytes(latestMetrics['mem.used_bytes'] || 0)} / {fmtBytes(latestMetrics['mem.total_bytes'])}</span>
            {/if}
          </div>
        {/if}
        {#each Object.entries(latestMetrics).filter(([k, v]) => k.match(/^disk\..*usage_pct$/) && !k.includes('/snap/') && !k.includes('/boot/') && !k.includes('/mnt/data') && v < 99.5) as [mname, val]}
          {@const mount = mname.match(/^disk\.(.+)\.usage_pct$/)?.[1] || '?'}
          <div class="gauge-with-detail">
            <Gauge value={val} label="Disk {mount}" size={56} />
            {#if latestMetrics[`disk.${mount}.total_bytes`]}
              <span class="detail">{fmtBytes(latestMetrics[`disk.${mount}.used_bytes`] || 0)} / {fmtBytes(latestMetrics[`disk.${mount}.total_bytes`])}</span>
            {/if}
          </div>
        {/each}
      </div>
    </div>

    <div class="time-bar">
      <TimeRangeSelector selected={rangeSeconds} on:change={(e) => { rangeSeconds = e.detail; loadData(); }} />
    </div>

    <div class="charts">
      {#each categories as cat}
        {#if chartsData[cat.key]}
          <div class="chart-card">
            <h3>{cat.label}</h3>
            <Chart
              data={chartsData[cat.key]}
              label={cat.label}
              unit={cat.unit}
              color={cat.color}
              height={250}
            />
          </div>
        {/if}
      {/each}
    </div>
  {/if}
```

- [ ] **Step 2: Replace the `<style>` block**

Replace the entire `<style>` block with:

```css
<style>
  .header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 12px;
    flex-wrap: wrap;
    gap: 12px;
  }
  .header-left h1 { margin-bottom: 2px; }
  .last-seen { font-size: 0.85rem; color: var(--fg-muted); }

  .header-gauges {
    display: flex;
    gap: 16px;
    align-items: flex-start;
    flex-wrap: wrap;
  }

  .time-bar {
    display: flex;
    justify-content: flex-end;
    padding: 8px 16px;
    margin-bottom: var(--gap);
    background: var(--bg-card);
    border-radius: var(--radius);
    border: 1px solid var(--border-subtle);
    box-shadow: var(--shadow-card);
  }

  .charts {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
    gap: var(--gap);
  }

  .chart-card {
    background: var(--bg-card);
    padding: 16px;
    border-radius: var(--radius);
    border: 1px solid var(--border-subtle);
    box-shadow: var(--shadow-card);
  }
  .chart-card h3 {
    font-size: 1rem;
    margin-bottom: 8px;
    color: var(--fg-muted);
  }

  .gauge-with-detail {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 2px;
  }
  .gauge-with-detail .detail {
    font-size: 0.65rem;
    color: var(--fg-muted);
  }

  .error { color: var(--red); }

  @media (max-width: 768px) {
    .header {
      flex-direction: column;
    }
    .header-gauges {
      justify-content: center;
      width: 100%;
    }
  }
</style>
```

- [ ] **Step 3: Update loading skeleton**

The loading state (lines 133–136) currently shows a `.gauges-row` skeleton. Replace with a simpler skeleton since the gauges are now in the header:

```svelte
  {#if loading && !host}
    <div class="time-bar">
      <div class="skel" style="width:200px;height:28px;"></div>
    </div>
    <div class="charts">
      {#each Array(4) as _}
        <div class="chart-card">
          <div class="skel" style="width:40%;height:16px;margin-bottom:8px;"></div>
          <div class="skel" style="width:100%;height:250px;border-radius:var(--radius);"></div>
        </div>
      {/each}
    </div>
```

Remove the `Skeleton` import since it's no longer used. Add `.skel` to the style block:

```css
.skel {
  background: linear-gradient(90deg, #323543 25%, #3a3d4e 50%, #323543 75%);
  background-size: 200% 100%;
  animation: shimmer 1.5s infinite;
  border-radius: 4px;
}
```

Wait — `.skel` is already defined globally in `app.css`. So no local `.skel` needed. Just remove the Skeleton import and use `class="skel"` directly.

- [ ] **Step 4: Verify Host Detail**

Open a host detail page. Gauges should be top-right in the header at 56px size. Time range bar should be a slim row below the header. Charts should be responsive. On mobile width, gauges should wrap below hostname.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/HostDetail.svelte
git commit -m "feat: host detail layout — gauges in header, slim time-range bar"
```

---

### Task 4: Build & Deploy

**Files:**
- None — verification and deploy

- [ ] **Step 1: Run production build**

```bash
cd frontend && npm run build
```

Expected: Build succeeds with no errors.

- [ ] **Step 2: Visual check**

1. `/` — dashboard with 4 tiles, activity feed populates live
2. `/monitors` — uptime cards (former Overview)
3. Click monitor card → `/monitors/:name` detail page works
4. `/hosts` → host cards
5. `/hosts/:name` → gauges in header, time-range bar, responsive charts
6. All other pages unchanged
7. Nav sidebar has 6 items: Overview, Monitors, Hosts, Security, Incidents, Settings

- [ ] **Step 3: Push, tag, deploy**

```bash
git push origin master
git tag v0.2.5 && git push origin v0.2.5
# Wait for CI to complete
ssh oracle "cd /home/ubuntu/whatsupp && docker compose pull && docker compose up -d"
```
