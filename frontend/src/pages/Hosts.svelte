<script>
  import { onMount, onDestroy } from 'svelte';
  import { link } from '../lib/router.js';
  import { api } from '../lib/api.js';
  import { onMessage } from '../lib/ws.js';
  import Gauge from '../components/Gauge.svelte';

  let hosts = [];
  let loading = true;
  let error = '';

  onMount(async () => {
    try {
      hosts = await api.getHosts();
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  });

  // Live metric updates
  const unsub = onMessage('agent_metric', (data) => {
    hosts = hosts.map(h => {
      if (h.name !== data.host) return h;
      const updated = { ...h, metrics: { ...h.metrics } };
      for (const m of (data.metrics || [])) {
        updated.metrics[m.name] = m.value;
      }
      return updated;
    });
  });

  onDestroy(unsub);

  function getMetric(host, name) {
    return host.metrics?.[name] ?? null;
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
    <p class="muted">Loading hosts...</p>
  {:else if error}
    <p class="error">{error}</p>
  {:else if hosts.length === 0}
    <p class="muted">No hosts reporting. Configure agents or scrape targets in Settings.</p>
  {:else}
    <div class="grid">
      {#each hosts as host}
        <a href="/hosts/{encodeURIComponent(host.host)}" use:link class="card">
          <div class="card-header">
            <span class="host-name">{host.host}</span>
            <span class="last-seen">{formatLastSeen(host.last_seen_at)}</span>
          </div>
          <div class="gauges">
            {#if getMetric(host, 'cpu.usage_pct') != null}
              <Gauge value={getMetric(host, 'cpu.usage_pct')} label="CPU" />
            {/if}
            {#if getMetric(host, 'mem.usage_pct') != null}
              <Gauge value={getMetric(host, 'mem.usage_pct')} label="RAM" />
            {/if}
            {#if getMetric(host, 'disk./.usage_pct') != null}
              <Gauge value={getMetric(host, 'disk./.usage_pct')} label="Disk" />
            {/if}
          </div>
          {#if getMetric(host, 'temp.cpu') != null}
            <div class="temp">
              CPU Temp: <span class="temp-value">{Math.round(getMetric(host, 'temp.cpu'))}&deg;C</span>
            </div>
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
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: var(--gap);
  }

  .card {
    background: var(--bg-card);
    border-radius: var(--radius);
    padding: 16px;
    text-decoration: none;
    color: var(--fg);
    border: 1px solid transparent;
    transition: border-color 0.15s;
  }
  .card:hover {
    border-color: var(--purple);
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

  .muted { color: var(--fg-muted); }
  .error { color: var(--red); }
</style>
