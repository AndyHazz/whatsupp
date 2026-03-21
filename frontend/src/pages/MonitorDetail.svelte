<script>
  import { onMount, onDestroy } from 'svelte';
  import { api } from '../lib/api.js';
  import { onMessage } from '../lib/ws.js';
  import Chart from '../components/Chart.svelte';
  import TimeRangeSelector from '../components/TimeRangeSelector.svelte';
  import StatusBadge from '../components/StatusBadge.svelte';

  export let name;

  let monitor = null;
  let incidents = [];
  let chartData = [[], []];
  let loading = true;
  let error = '';
  let rangeSeconds = 86400; // 24h default

  async function loadData() {
    loading = true;
    error = '';
    try {
      const now = Math.floor(Date.now() / 1000);
      const from = now - rangeSeconds;

      const [m, results, inc] = await Promise.all([
        api.getMonitor(name),
        api.getMonitorResults(name, from, now),
        api.getIncidents(from, now),
      ]);

      monitor = m;
      incidents = (inc || []).filter(i => i.monitor === name);

      // Build uPlot data arrays
      const timestamps = (results || []).map(r => r.timestamp);
      const latencies  = (results || []).map(r => r.latency_ms);
      chartData = [timestamps, latencies];
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  onMount(() => loadData());

  // Live updates — append new check results
  const unsub = onMessage('check_result', (data) => {
    if (data.monitor !== name) return;
    if (monitor) {
      monitor = { ...monitor, status: data.status, latency_ms: data.latency_ms };
    }
    if (data.timestamp && data.latency_ms != null) {
      chartData = [
        [...chartData[0], data.timestamp],
        [...chartData[1], data.latency_ms],
      ];
    }
  });

  onDestroy(unsub);

  // Convert incidents to chart bands
  $: chartBands = incidents.map(inc => ({
    from: inc.started_at,
    to: inc.resolved_at || Math.floor(Date.now() / 1000),
  }));

  function formatDuration(seconds) {
    if (!seconds || seconds < 0) return '-';
    if (seconds < 60) return `${seconds}s`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    return `${h}h ${m}m`;
  }

  function formatTime(ts) {
    if (!ts) return '-';
    return new Date(ts * 1000).toLocaleString();
  }
</script>

<div class="monitor-detail">
  {#if loading && !monitor}
    <div class="chart-section">
      <div style="display:flex;justify-content:space-between;margin-bottom:12px;">
        <div class="skel" style="width:40%;height:20px;"></div>
      </div>
      <div class="skel" style="width:100%;height:350px;border-radius:var(--radius);"></div>
    </div>
  {:else if error}
    <p class="error">{error}</p>
  {:else if monitor}
    <div class="header">
      <div>
        <h1>{monitor.name}</h1>
        <span class="meta">{monitor.type} &middot; {monitor.url || monitor.host || ''}</span>
      </div>
      <StatusBadge status={monitor.status} />
    </div>

    <div class="chart-section">
      <div class="chart-controls">
        <h2>Response Time</h2>
        <TimeRangeSelector selected={rangeSeconds} on:change={(e) => { rangeSeconds = e.detail; loadData(); }} />
      </div>
      <Chart data={chartData} label="Latency" unit="ms" height={350} bands={chartBands} />
    </div>

    {#if incidents.length > 0}
      <div class="incidents-section">
        <h2>Recent Incidents</h2>
        <table>
          <thead>
            <tr>
              <th>Started</th>
              <th>Resolved</th>
              <th>Duration</th>
              <th>Cause</th>
            </tr>
          </thead>
          <tbody>
            {#each incidents as inc}
              <tr>
                <td>{formatTime(inc.started_at)}</td>
                <td>{inc.resolved_at ? formatTime(inc.resolved_at) : 'Ongoing'}</td>
                <td>{inc.resolved_at ? formatDuration(inc.resolved_at - inc.started_at) : 'Ongoing'}</td>
                <td class="cause">{inc.cause || '-'}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  {/if}
</div>

<style>
  .header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 24px;
  }
  .header h1 { margin-bottom: 4px; }
  .meta { color: var(--fg-muted); font-size: 0.9rem; }

  .chart-section {
    background: var(--bg-card);
    padding: 16px;
    border-radius: var(--radius);
    margin-bottom: 24px;
    border: 1px solid var(--border-subtle);
    box-shadow: var(--shadow-card);
  }
  .chart-controls {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
    flex-wrap: wrap;
    gap: 8px;
  }
  .chart-controls h2 { font-size: 1.1rem; }

  .incidents-section {
    background: var(--bg-card);
    padding: 16px;
    border-radius: var(--radius);
    border: 1px solid var(--border-subtle);
    box-shadow: var(--shadow-card);
  }
  .incidents-section h2 {
    font-size: 1.1rem;
    margin-bottom: 12px;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.9rem;
  }
  th {
    text-align: left;
    padding: 8px 12px;
    border-bottom: 1px solid var(--border-subtle);
    color: var(--fg-muted);
    font-weight: 600;
  }
  td {
    padding: 8px 12px;
    border-bottom: 1px solid var(--border-subtle);
  }
  tbody tr {
    transition: background-color 0.15s ease;
  }
  tbody tr:hover {
    background-color: rgba(248, 248, 242, 0.04);
  }
  .cause { color: var(--red); }
  .error { color: var(--red); }
</style>
