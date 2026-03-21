<script>
  import { onMount, onDestroy } from 'svelte';
  import { api } from '../lib/api.js';
  import { onMessage } from '../lib/ws.js';
  import Chart from '../components/Chart.svelte';
  import TimeRangeSelector from '../components/TimeRangeSelector.svelte';
  import Gauge from '../components/Gauge.svelte';
  import { dracula } from '../lib/theme.js';
  import Skeleton from '../components/Skeleton.svelte';

  export let name;

  let host = null;
  let loading = true;
  let error = '';
  let rangeSeconds = 3600; // default 1h since we have ~30min data so far

  const categories = [
    { key: 'cpu',     label: 'CPU Usage',     names: 'cpu.usage_pct',        unit: '%',  color: dracula.purple },
    { key: 'mem',     label: 'Memory Usage',  names: 'mem.usage_pct',        unit: '%',  color: dracula.pink },
    { key: 'disk',    label: 'Disk Usage',    names: 'disk',                 unit: '%',  color: dracula.orange },
    { key: 'net',     label: 'Network',       names: 'net',                  unit: 'B/s', color: dracula.cyan },
    { key: 'temp',    label: 'Temperature',   names: 'temp',                 unit: '°C', color: dracula.red },
    { key: 'docker',  label: 'Docker',        names: 'docker',               unit: '%',  color: dracula.green },
  ];

  let chartsData = {};
  let latestMetrics = {}; // Latest value per metric_name

  async function loadData() {
    loading = true;
    error = '';
    try {
      const now = Math.floor(Date.now() / 1000);
      const from = now - rangeSeconds;

      host = await api.getHost(name);

      // Fetch all metrics at once
      let allMetrics = [];
      try {
        allMetrics = await api.getHostMetrics(name, from, now, null) || [];
      } catch { /* no metrics yet */ }

      // Group by metric_name
      const byName = {};
      for (const m of allMetrics) {
        if (!byName[m.metric_name]) byName[m.metric_name] = [];
        byName[m.metric_name].push(m);
      }

      // Track latest value of each metric
      latestMetrics = {};
      for (const [mname, points] of Object.entries(byName)) {
        if (points.length > 0) {
          latestMetrics[mname] = points[points.length - 1].value;
        }
      }

      // Build chart data per category
      chartsData = {};
      for (const cat of categories) {
        // Find the primary metric matching this category
        let primaryName = null;
        for (const mname of Object.keys(byName)) {
          if (mname === cat.names || mname.startsWith(cat.names)) {
            // For specific metrics like cpu.usage_pct, use exact match
            // For broad categories like 'net', pick the first rate metric
            if (cat.key === 'net') {
              if (mname.endsWith('_bytes_sec') && !mname.includes('lo.') && !primaryName) {
                primaryName = mname;
              }
            } else if (cat.key === 'disk') {
              if (mname.endsWith('usage_pct') && !primaryName) {
                primaryName = mname;
              }
            } else if (cat.key === 'temp') {
              if (!primaryName) primaryName = mname;
            } else if (cat.key === 'docker') {
              if (mname.endsWith('cpu_pct') && !primaryName) {
                primaryName = mname;
              }
            } else {
              primaryName = cat.names;
            }
          }
        }

        if (primaryName && byName[primaryName]) {
          const points = byName[primaryName];
          chartsData[cat.key] = [
            points.map(m => m.timestamp),
            points.map(m => m.value),
          ];
        }
      }
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  onMount(() => loadData());

  const unsub = onMessage('agent_metric', (data) => {
    if (data.host !== name) return;
    for (const m of (data.metrics || [])) {
      latestMetrics[m.name] = m.value;
    }
    latestMetrics = latestMetrics; // trigger reactivity
  });

  onDestroy(unsub);

  function fmtBytes(b) {
    if (b >= 1e12) return (b / 1e12).toFixed(1) + ' TB';
    if (b >= 1e9) return (b / 1e9).toFixed(1) + ' GB';
    if (b >= 1e6) return (b / 1e6).toFixed(0) + ' MB';
    return (b / 1e3).toFixed(0) + ' KB';
  }

  function formatLastSeen(ts) {
    if (!ts) return 'Never';
    const diff = Math.floor(Date.now() / 1000) - ts;
    if (diff < 60) return 'Just now';
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
    return `${Math.floor(diff / 3600)}h ago`;
  }
</script>

<div class="host-detail">
  {#if loading && !host}
  <div class="gauges-row">
    <Skeleton variant="gauge" count={3} />
  </div>
  {:else if error}
    <p class="error">{error}</p>
  {:else if host}
    <div class="header">
      <div class="header-left">
        <h1>{host.host}</h1>
        <span class="last-seen">Last seen: {formatLastSeen(host.last_seen_at)}</span>
      </div>
      <TimeRangeSelector selected={rangeSeconds} on:change={(e) => { rangeSeconds = e.detail; loadData(); }} />
    </div>

    <div class="gauges-row">
      {#if latestMetrics['cpu.usage_pct'] != null}
        <Gauge value={latestMetrics['cpu.usage_pct']} label="CPU" />
      {/if}
      {#if latestMetrics['mem.usage_pct'] != null}
        <div class="gauge-with-detail">
          <Gauge value={latestMetrics['mem.usage_pct']} label="RAM" />
          {#if latestMetrics['mem.total_bytes']}
            <span class="detail">{fmtBytes(latestMetrics['mem.used_bytes'] || 0)} / {fmtBytes(latestMetrics['mem.total_bytes'])}</span>
          {/if}
        </div>
      {/if}
      {#each Object.entries(latestMetrics).filter(([k, v]) => k.match(/^disk\..*usage_pct$/) && !k.includes('/snap/') && !k.includes('/boot/') && !k.includes('/mnt/data') && v < 99.5) as [mname, val]}
        {@const mount = mname.match(/^disk\.(.+)\.usage_pct$/)?.[1] || '?'}
        <div class="gauge-with-detail">
          <Gauge value={val} label="Disk {mount}" />
          {#if latestMetrics[`disk.${mount}.total_bytes`]}
            <span class="detail">{fmtBytes(latestMetrics[`disk.${mount}.used_bytes`] || 0)} / {fmtBytes(latestMetrics[`disk.${mount}.total_bytes`])}</span>
          {/if}
        </div>
      {/each}
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
    align-items: flex-start;
    margin-bottom: 20px;
    flex-wrap: wrap;
    gap: 12px;
  }
  .header-left h1 { margin-bottom: 2px; }
  .last-seen { font-size: 0.85rem; color: var(--fg-muted); }

  .gauges-row {
    display: flex;
    gap: 24px;
    justify-content: center;
    margin-bottom: 24px;
    padding: 16px;
    background: var(--bg-card);
    border-radius: var(--radius);
    flex-wrap: wrap;
    border: 1px solid var(--border-subtle);
    box-shadow: var(--shadow-card);
  }

  .charts {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
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
    gap: 4px;
  }
  .gauge-with-detail .detail {
    font-size: 0.7rem;
    color: var(--fg-muted);
  }

  .error { color: var(--red); }

  @media (max-width: 1200px) {
    .charts { grid-template-columns: repeat(2, 1fr); }
  }
  @media (max-width: 768px) {
    .charts { grid-template-columns: 1fr; }
  }
</style>
