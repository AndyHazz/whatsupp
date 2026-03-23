<script>
  import { onMount, onDestroy } from 'svelte';
  import { link } from '../lib/router.js';
  import { api } from '../lib/api.js';
  import { onMessage } from '../lib/ws.js';
  import Gauge from '../components/Gauge.svelte';
  import Skeleton from '../components/Skeleton.svelte';

  let hosts = [];
  let hostMetrics = {}; // host -> { metricName: value }
  let loading = true;
  let error = '';
  let mutedNames = new Set();

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
      const [hostsData, mutes] = await Promise.all([
        api.getHosts(),
        api.getMutes(),
      ]);
      hosts = hostsData;
      mutedNames = new Set(mutes || []);

      // Fetch latest metrics for each host (last 2 minutes)
      const now = Math.floor(Date.now() / 1000);
      const from = now - 120;
      await Promise.all(hosts.map(async (h) => {
        try {
          const metrics = await api.getHostMetrics(h.host, from, now, null);
          if (metrics && metrics.length > 0) {
            const latest = {};
            for (const m of metrics) {
              // Keep the latest value for each metric name
              latest[m.metric_name] = m.value;
            }
            hostMetrics[h.host] = latest;
          }
        } catch { /* no metrics */ }
      }));
      hostMetrics = hostMetrics; // trigger reactivity
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  });

  // Live metric updates
  const unsub = onMessage('agent_metric', (data) => {
    if (!data.host) return;
    if (!hostMetrics[data.host]) hostMetrics[data.host] = {};
    for (const m of (data.metrics || [])) {
      hostMetrics[data.host][m.name] = m.value;
    }
    hostMetrics = hostMetrics;
  });

  onDestroy(unsub);

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
</script>

<div class="hosts-page">
  <h1>Hosts</h1>

  {#if loading}
  <div class="grid">
    <Skeleton variant="card" count={4} />
  </div>
  {:else if error}
    <p class="error">{error}</p>
  {:else if hosts.length === 0}
    <p class="muted">No hosts reporting. Configure agents or scrape targets in Settings.</p>
  {:else}
    <div class="grid">
      {#each hosts as h}
        <a href="/hosts/{encodeURIComponent(h.host)}" use:link class="card">
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
          <div class="gauges">
            {#if getMetric(h.host, 'cpu.usage_pct') != null}
              <Gauge value={getMetric(h.host, 'cpu.usage_pct')} label="CPU" />
            {/if}
            {#if getMetric(h.host, 'mem.usage_pct') != null}
              <Gauge value={getMetric(h.host, 'mem.usage_pct')} label="RAM" />
            {/if}
          </div>
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
      {/each}
    </div>
  {/if}
</div>

<style>
  .hosts-page h1 { margin-bottom: var(--gap); }

  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: var(--gap);
  }

  .card {
    background: var(--bg-card);
    border-radius: var(--radius);
    padding: 16px;
    text-decoration: none;
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

  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
  }
  .host-name { font-weight: 600; font-size: 1.05rem; }
  .header-right { display: flex; align-items: center; gap: 8px; }
  .last-seen { font-size: 0.8rem; color: var(--fg-muted); }

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

  .gauges {
    display: flex;
    justify-content: space-around;
    margin-bottom: 8px;
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
  .muted { color: var(--fg-muted); }
  .error { color: var(--red); }
</style>
