<script>
  import { onMount, onDestroy } from 'svelte';
  import { link, navigate } from '../lib/router.js';
  import { api } from '../lib/api.js';
  import { onMessage } from '../lib/ws.js';
  import Gauge from '../components/Gauge.svelte';
  import StatusBadge from '../components/StatusBadge.svelte';
  import Sparkline from '../components/Sparkline.svelte';
  import Skeleton from '../components/Skeleton.svelte';

  let hosts = [];
  let hostMetrics = {}; // host -> { metricName: value }
  let monitors = [];
  let sparklines = {};
  let sparklineStatuses = {};
  let loading = true;
  let error = '';
  let mutedNames = new Set();

  function goToMonitor(name) {
    navigate('/monitors/' + encodeURIComponent(name));
  }

  async function toggleMute(name) {
    try {
      const result = await api.toggleMute(name);
      if (result.muted) {
        mutedNames.add(name);
      } else {
        mutedNames.delete(name);
      }
      mutedNames = mutedNames;
    } catch { /* ignore */ }
  }

  onMount(async () => {
    try {
      const [hostsData, monitorsData, mutes] = await Promise.all([
        api.getHosts(),
        api.getMonitors(),
        api.getMutes(),
      ]);
      hosts = hostsData;
      monitors = monitorsData.sort((a, b) => {
        if (a.status === 'down' && b.status !== 'down') return -1;
        if (a.status !== 'down' && b.status === 'down') return 1;
        return a.name.localeCompare(b.name);
      });
      mutedNames = new Set(mutes || []);

      // Fetch host metrics (last 2 minutes)
      const now = Math.floor(Date.now() / 1000);
      const from = now - 120;
      await Promise.all(hosts.map(async (h) => {
        try {
          const metrics = await api.getHostMetrics(h.host, from, now, null);
          if (metrics && metrics.length > 0) {
            const latest = {};
            for (const m of metrics) {
              latest[m.metric_name] = m.value;
            }
            hostMetrics[h.host] = latest;
          }
        } catch { /* no metrics */ }
      }));
      hostMetrics = hostMetrics;

      // Fetch monitor sparklines (last 1h)
      const oneHourAgo = now - 3600;
      await Promise.all(monitors.map(async (m) => {
        try {
          const results = await api.getMonitorResults(m.name, oneHourAgo, now);
          const filtered = (results || []).filter(r => r.latency_ms != null);
          sparklines[m.name] = filtered.map(r => r.latency_ms);
          sparklineStatuses[m.name] = filtered.map(r => r.status || 'up');
        } catch { /* ignore */ }
      }));
      sparklines = sparklines;
      sparklineStatuses = sparklineStatuses;
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  });

  // Live metric updates
  const unsubMetric = onMessage('agent_metric', (data) => {
    if (!data.host) return;
    if (!hostMetrics[data.host]) hostMetrics[data.host] = {};
    for (const m of (data.metrics || [])) {
      hostMetrics[data.host][m.name] = m.value;
    }
    hostMetrics = hostMetrics;
  });

  // Live monitor updates
  const unsubCheck = onMessage('check_result', (data) => {
    monitors = monitors.map(m => {
      if (m.name === data.monitor) {
        return { ...m, status: data.status, latency_ms: data.latency_ms };
      }
      return m;
    });
    if (sparklines[data.monitor] && data.latency_ms != null) {
      sparklines[data.monitor] = [...sparklines[data.monitor].slice(-59), data.latency_ms];
    }
    if (sparklineStatuses[data.monitor]) {
      sparklineStatuses[data.monitor] = [...sparklineStatuses[data.monitor].slice(-59), data.status || 'up'];
    }
  });

  onDestroy(() => { unsubMetric(); unsubCheck(); });

  function getMetric(hostname, name) {
    return hostMetrics[hostname]?.[name] ?? null;
  }

  function formatLastSeen(ts) {
    if (!ts) return 'Never';
    const diff = Math.floor(Date.now() / 1000) - ts;
    if (diff < 60) return 'Just now';
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
    return `${Math.floor(diff / 3600)}h ago`;
  }

  // Build rows: each host + its grouped monitors
  $: monitorsByGroup = (() => {
    const map = {};
    for (const m of monitors) {
      if (m.group) {
        if (!map[m.group]) map[m.group] = [];
        map[m.group].push(m);
      }
    }
    return map;
  })();

  $: ungroupedMonitors = monitors.filter(m => !m.group);

  // Hosts sorted: those with monitors first, then alphabetical
  $: sortedHosts = [...hosts].sort((a, b) => {
    const aHas = monitorsByGroup[a.host] ? 1 : 0;
    const bHas = monitorsByGroup[b.host] ? 1 : 0;
    if (aHas !== bHas) return bHas - aHas;
    return a.host.localeCompare(b.host);
  });
</script>

<div class="hosts-page">
  <h1>Hosts</h1>

  {#if loading}
    <div class="host-row">
      <div class="row-grid">
        <Skeleton variant="host" count={1} />
        <Skeleton variant="monitor" count={3} />
      </div>
    </div>
    <div class="host-row">
      <div class="row-grid">
        <Skeleton variant="host" count={1} />
        <Skeleton variant="monitor" count={2} />
      </div>
    </div>
  {:else if error}
    <p class="error">{error}</p>
  {:else}
    {#each sortedHosts as h}
      {@const grouped = monitorsByGroup[h.host] || []}
      <div class="host-row">
        <div class="row-grid">
          <!-- Host card -->
          <a href="/hosts/{encodeURIComponent(h.host)}" use:link class="card host-card">
            <div class="card-header">
              <span class="host-name">{h.host}</span>
              <div class="header-right">
                <button
                  class="mute-btn"
                  class:is-muted={mutedNames.has('agent:' + h.host)}
                  title={mutedNames.has('agent:' + h.host) ? 'Unmute notifications' : 'Mute notifications'}
                  on:click|preventDefault|stopPropagation={() => toggleMute('agent:' + h.host)}
                >
                  {#if mutedNames.has('agent:' + h.host)}
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M13.73 21a2 2 0 0 1-3.46 0"/><path d="M18.63 13A17.89 17.89 0 0 1 18 8"/><path d="M6.26 6.26A5.86 5.86 0 0 0 6 8c0 7-3 9-3 9h14"/><path d="M18 8a6 6 0 0 0-9.33-5"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
                  {:else}
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/></svg>
                  {/if}
                </button>
                <span class="last-seen">{formatLastSeen(h.last_seen_at)}</span>
              </div>
            </div>
            {#if getMetric(h.host, 'cpu.usage_pct') != null || getMetric(h.host, 'mem.usage_pct') != null}
              <div class="gauges">
                {#if getMetric(h.host, 'cpu.usage_pct') != null}
                  <Gauge value={getMetric(h.host, 'cpu.usage_pct')} label="CPU" />
                {/if}
                {#if getMetric(h.host, 'mem.usage_pct') != null}
                  <Gauge value={getMetric(h.host, 'mem.usage_pct')} label="RAM" />
                {/if}
              </div>
            {:else}
              <div class="gauges gauges-offline">
                <Gauge value={0} label="CPU" disabled />
                <Gauge value={0} label="RAM" disabled />
              </div>
              <div class="offline-label">Offline</div>
            {/if}
            {#if getMetric(h.host, 'temp.cpu') != null || getMetric(h.host, 'temp.cpu_thermal') != null}
              <div class="temp">
                CPU Temp: <span class="temp-value">{Math.round(getMetric(h.host, 'temp.cpu') ?? getMetric(h.host, 'temp.cpu_thermal') ?? 0)}&deg;C</span>
              </div>
            {/if}
            {#if getMetric(h.host, 'battery.charge_pct') != null}
              {@const chargePct = getMetric(h.host, 'battery.charge_pct')}
              {@const isCharging = getMetric(h.host, 'battery.charging') === 1}
              <div class="battery" class:battery-low={chargePct < 10} class:battery-warn={chargePct >= 10 && chargePct < 20}>
                <span class="battery-icon">{isCharging ? '\u26A1' : '\u{1F50B}'}</span>
                <span class="battery-value">{Math.round(chargePct)}%</span>
              </div>
            {/if}
            {#if h.version}
              <div class="agent-version muted">{h.version}</div>
            {/if}
          </a>

          <!-- Grouped monitor cards -->
          {#each grouped as m}
            <div class="card monitor-card" class:down={m.status === 'down'} on:click={() => goToMonitor(m.name)} role="button" tabindex="0" on:keydown={(e) => e.key === 'Enter' && goToMonitor(m.name)}>
              <div class="card-top">
                <div class="card-title">
                  {#if m.url}
                    <a href={m.url} target="_blank" rel="noopener noreferrer" class="monitor-name service-link" on:click|stopPropagation>{m.name}</a>
                  {:else}
                    <span class="monitor-name">{m.name}</span>
                  {/if}
                </div>
                <div class="card-stats">
                  {#if m.latency_ms != null}
                    <span class="latency">{Math.round(m.latency_ms)}<span class="unit">ms</span></span>
                  {/if}
                  <StatusBadge status={m.status} />
                </div>
              </div>
              <div class="card-meta">
                <div class="card-meta-left">
                  {#if m.uptime_pct != null}
                    <span class="uptime" class:good={m.uptime_pct >= 99} class:warn={m.uptime_pct < 99 && m.uptime_pct >= 95} class:bad={m.uptime_pct < 95}>
                      {m.uptime_pct.toFixed(1)}%
                    </span>
                  {/if}
                  {#if m.cert_days_left != null}
                    <span class="cert-badge" class:cert-warn={m.cert_days_left <= 14} class:cert-danger={m.cert_days_left <= 3}>
                      <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>
                      {m.cert_days_left}d
                    </span>
                  {/if}
                </div>
                <button
                  class="mute-btn"
                  class:is-muted={mutedNames.has(m.name)}
                  title={mutedNames.has(m.name) ? 'Unmute notifications' : 'Mute notifications'}
                  on:click|stopPropagation={() => toggleMute(m.name)}
                >
                  {#if mutedNames.has(m.name)}
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M13.73 21a2 2 0 0 1-3.46 0"/><path d="M18.63 13A17.89 17.89 0 0 1 18 8"/><path d="M6.26 6.26A5.86 5.86 0 0 0 6 8c0 7-3 9-3 9h14"/><path d="M18 8a6 6 0 0 0-9.33-5"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
                  {:else}
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/></svg>
                  {/if}
                </button>
              </div>
              <div class="card-sparkline">
                <Sparkline data={sparklines[m.name] || []} statuses={sparklineStatuses[m.name] || []} width={260} height={36} />
              </div>
            </div>
          {/each}
        </div>
      </div>
    {/each}

    {#if ungroupedMonitors.length > 0}
      <h2 class="section-label">Ungrouped Monitors</h2>
      <div class="row-grid">
        {#each ungroupedMonitors as m}
          <div class="card monitor-card" class:down={m.status === 'down'} on:click={() => goToMonitor(m.name)} role="button" tabindex="0" on:keydown={(e) => e.key === 'Enter' && goToMonitor(m.name)}>
            <div class="card-top">
              <div class="card-title">
                {#if m.url}
                  <a href={m.url} target="_blank" rel="noopener noreferrer" class="monitor-name service-link" on:click|stopPropagation>{m.name}</a>
                {:else}
                  <span class="monitor-name">{m.name}</span>
                {/if}
              </div>
              <div class="card-stats">
                {#if m.latency_ms != null}
                  <span class="latency">{Math.round(m.latency_ms)}<span class="unit">ms</span></span>
                {/if}
                <StatusBadge status={m.status} />
              </div>
            </div>
            <div class="card-meta">
              <div class="card-meta-left">
                {#if m.uptime_pct != null}
                  <span class="uptime" class:good={m.uptime_pct >= 99} class:warn={m.uptime_pct < 99 && m.uptime_pct >= 95} class:bad={m.uptime_pct < 95}>
                    {m.uptime_pct.toFixed(1)}%
                  </span>
                {/if}
              </div>
              <button
                class="mute-btn"
                class:is-muted={mutedNames.has(m.name)}
                title={mutedNames.has(m.name) ? 'Unmute notifications' : 'Mute notifications'}
                on:click|stopPropagation={() => toggleMute(m.name)}
              >
                {#if mutedNames.has(m.name)}
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M13.73 21a2 2 0 0 1-3.46 0"/><path d="M18.63 13A17.89 17.89 0 0 1 18 8"/><path d="M6.26 6.26A5.86 5.86 0 0 0 6 8c0 7-3 9-3 9h14"/><path d="M18 8a6 6 0 0 0-9.33-5"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
                {:else}
                  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/></svg>
                {/if}
              </button>
            </div>
            <div class="card-sparkline">
              <Sparkline data={sparklines[m.name] || []} statuses={sparklineStatuses[m.name] || []} width={260} height={36} />
            </div>
          </div>
        {/each}
      </div>
    {/if}

    {#if hosts.length === 0 && monitors.length === 0}
      <p class="muted">No hosts or monitors configured.</p>
    {/if}
  {/if}
</div>

<style>
  .hosts-page h1 { margin-bottom: var(--gap); }

  .host-row {
    margin-bottom: var(--gap);
  }

  .row-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: var(--gap);
  }

  .section-label {
    font-size: 0.85rem;
    font-weight: 600;
    color: var(--fg-muted);
    text-transform: uppercase;
    letter-spacing: 0.05em;
    margin: 24px 0 12px;
  }

  /* ── Shared card base ──────────────────── */
  .card {
    background: var(--bg-card);
    border-radius: var(--radius);
    padding: 16px;
    color: var(--fg);
    border: 1px solid var(--border-subtle);
    box-shadow: var(--shadow-card);
    transition: transform 0.15s ease, box-shadow 0.15s ease, border-color 0.15s ease, background 0.15s ease;
  }
  .card:hover {
    transform: translateY(-1px);
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3), 0 0 0 1px rgba(189, 147, 249, 0.1);
    border-color: rgba(189, 147, 249, 0.3);
    background: var(--bg-card-hover);
    text-decoration: none;
  }

  /* ── Host card ─────────────────────────── */
  .host-card {
    text-decoration: none;
    border-left: 3px solid var(--purple);
  }
  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
  }
  .host-name { font-weight: 600; font-size: 1.05rem; }
  .header-right { display: flex; align-items: center; gap: 8px; }
  .last-seen { font-size: 0.8rem; color: var(--fg-muted); }

  .gauges {
    display: flex;
    justify-content: space-around;
    margin-bottom: 8px;
  }
  .gauges-offline { opacity: 0.4; }
  .offline-label {
    font-size: 0.8rem;
    text-align: center;
    color: var(--fg-muted);
    opacity: 0.6;
    margin-bottom: 4px;
  }

  .temp {
    font-size: 0.85rem;
    color: var(--fg-muted);
    text-align: center;
  }
  .temp-value { color: var(--orange); font-weight: 600; }

  .battery {
    font-size: 0.85rem;
    color: var(--fg-muted);
    text-align: center;
  }
  .battery-value { font-weight: 600; color: var(--green); }
  .battery-warn .battery-value { color: var(--orange); }
  .battery-low .battery-value { color: var(--red); }
  .battery-icon { margin-right: 2px; }

  .agent-version {
    font-size: 0.75rem;
    text-align: center;
    margin-top: 4px;
  }

  /* ── Monitor card ──────────────────────── */
  .monitor-card {
    cursor: pointer;
  }
  .monitor-card.down {
    border-left: 3px solid var(--red);
  }

  .card-top {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 4px;
  }
  .card-title {
    min-width: 0;
    overflow: hidden;
  }
  .monitor-name {
    font-weight: 600;
    font-size: 1rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    display: block;
  }
  .service-link {
    color: var(--fg);
    text-decoration: none;
    transition: color 0.15s ease;
  }
  .service-link:hover {
    color: var(--purple);
    text-decoration: underline;
  }
  .card-stats {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-shrink: 0;
  }
  .latency {
    color: var(--cyan);
    font-weight: 700;
    font-size: 1.1rem;
    letter-spacing: -0.5px;
  }
  .unit {
    font-size: 0.65rem;
    font-weight: 400;
    opacity: 0.7;
    margin-left: 1px;
  }

  .card-meta {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
  }
  .card-meta-left {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .uptime {
    font-size: 0.8rem;
    font-weight: 500;
  }
  .uptime.good { color: var(--green); }
  .uptime.warn { color: var(--orange); }
  .uptime.bad  { color: var(--red); }

  .cert-badge {
    font-size: 0.72rem;
    color: var(--fg-muted);
    display: flex;
    align-items: center;
    gap: 3px;
    opacity: 0.7;
  }
  .cert-badge.cert-warn { color: var(--orange); opacity: 1; }
  .cert-badge.cert-danger { color: var(--red); opacity: 1; }

  .card-sparkline {
    width: 100%;
    overflow: hidden;
  }
  .card-sparkline :global(.sparkline) {
    width: 100%;
    height: 36px;
  }

  /* ── Mute button ───────────────────────── */
  .mute-btn {
    background: none;
    border: none;
    color: var(--fg-muted);
    cursor: pointer;
    padding: 2px 4px;
    border-radius: var(--radius);
    display: flex;
    align-items: center;
    opacity: 0.4;
    transition: opacity 0.15s ease, color 0.15s ease;
  }
  .mute-btn:hover { opacity: 1; }
  .mute-btn.is-muted { opacity: 0.8; color: var(--orange); }
  .mute-btn.is-muted:hover { opacity: 1; }

  .muted { color: var(--fg-muted); }
  .error { color: var(--red); }
</style>
