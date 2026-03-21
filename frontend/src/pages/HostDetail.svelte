<script>
  import { onMount, onDestroy } from 'svelte';
  import { api } from '../lib/api.js';
  import { onMessage } from '../lib/ws.js';
  import Chart from '../components/Chart.svelte';
  import TimeRangeSelector from '../components/TimeRangeSelector.svelte';
  import Gauge from '../components/Gauge.svelte';
  import { dracula } from '../lib/theme.js';

  export let name;

  let host = null;
  let loading = true;
  let error = '';
  let rangeSeconds = 86400;

  // Metric categories and their chart configs
  const categories = [
    { key: 'cpu',     label: 'CPU Usage',     names: 'cpu',  unit: '%',  color: dracula.purple },
    { key: 'mem',     label: 'Memory Usage',   names: 'mem',  unit: '%',  color: dracula.pink },
    { key: 'disk',    label: 'Disk Usage',     names: 'disk', unit: '%',  color: dracula.orange },
    { key: 'net',     label: 'Network',        names: 'net',  unit: 'B/s', color: dracula.cyan },
    { key: 'temp',    label: 'Temperature',    names: 'temp', unit: 'C',  color: dracula.red },
    { key: 'docker',  label: 'Docker',         names: 'docker', unit: '%', color: dracula.green },
  ];

  let chartsData = {};

  async function loadData() {
    loading = true;
    error = '';
    try {
      const now = Math.floor(Date.now() / 1000);
      const from = now - rangeSeconds;

      host = await api.getHost(name);

      // Fetch metrics for each category
      const results = await Promise.all(
        categories.map(async (cat) => {
          try {
            const metrics = await api.getHostMetrics(name, from, now, cat.names);
            return { key: cat.key, metrics };
          } catch {
            return { key: cat.key, metrics: [] };
          }
        })
      );

      chartsData = {};
      for (const { key, metrics } of results) {
        if (metrics && metrics.length > 0) {
          // Group by metric_name, use first metric for the chart
          const byName = {};
          for (const m of metrics) {
            if (!byName[m.metric_name]) byName[m.metric_name] = [];
            byName[m.metric_name].push(m);
          }
          // Use the primary metric (first one) for the chart
          const primary = Object.values(byName)[0] || [];
          chartsData[key] = [
            primary.map(m => m.timestamp),
            primary.map(m => m.value),
          ];
        }
      }
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  $: if (name && rangeSeconds) loadData();

  const unsub = onMessage('agent_metric', (data) => {
    if (data.host !== name || !host) return;
    // Update current metrics on host
    host = { ...host };
    for (const m of (data.metrics || [])) {
      if (!host.metrics) host.metrics = {};
      host.metrics[m.name] = m.value;
    }
  });

  onDestroy(unsub);

  function getMetric(metricName) {
    return host?.metrics?.[metricName] ?? null;
  }
</script>

<div class="host-detail">
  {#if loading && !host}
    <p class="muted">Loading...</p>
  {:else if error}
    <p class="error">{error}</p>
  {:else if host}
    <div class="header">
      <h1>{host.name}</h1>
      <TimeRangeSelector selected={rangeSeconds} on:change={(e) => { rangeSeconds = e.detail; }} />
    </div>

    <div class="gauges-row">
      {#if getMetric('cpu.usage_pct') != null}
        <Gauge value={getMetric('cpu.usage_pct')} label="CPU" size={90} />
      {/if}
      {#if getMetric('mem.usage_pct') != null}
        <Gauge value={getMetric('mem.usage_pct')} label="RAM" size={90} />
      {/if}
      {#if getMetric('disk./.usage_pct') != null}
        <Gauge value={getMetric('disk./.usage_pct')} label="Disk /" size={90} />
      {/if}
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
</div>

<style>
  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 20px;
    flex-wrap: wrap;
    gap: 12px;
  }

  .gauges-row {
    display: flex;
    gap: 24px;
    justify-content: center;
    margin-bottom: 24px;
    padding: 16px;
    background: var(--bg-card);
    border-radius: var(--radius);
  }

  .charts {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
    gap: var(--gap);
  }

  .chart-card {
    background: var(--bg-card);
    padding: 16px;
    border-radius: var(--radius);
  }
  .chart-card h3 {
    font-size: 1rem;
    margin-bottom: 8px;
    color: var(--fg-muted);
  }

  .muted { color: var(--fg-muted); }
  .error { color: var(--red); }
</style>
