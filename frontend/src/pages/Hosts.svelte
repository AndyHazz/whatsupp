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

  onMount(async () => {
    try {
      hosts = await api.getHosts();

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
            <span class="last-seen">{formatLastSeen(h.last_seen_at)}</span>
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
  .last-seen { font-size: 0.8rem; color: var(--fg-muted); }

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

  .agent-version {
    font-size: 0.75rem;
    text-align: center;
    margin-top: 4px;
  }
  .muted { color: var(--fg-muted); }
  .error { color: var(--red); }
</style>
