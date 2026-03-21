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
