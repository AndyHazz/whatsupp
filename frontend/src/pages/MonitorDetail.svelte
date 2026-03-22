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
  let avgLatency = null;
  let windowUptime = null;

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

      // Build uPlot data arrays + compute stats for the window
      const rows = results || [];
      const timestamps = rows.map(r => r.timestamp);
      const latencies  = rows.map(r => r.latency_ms);
      chartData = [timestamps, latencies];

      // Compute avg latency (exclude zeros/nulls for down results)
      const validLatencies = rows.filter(r => r.status === 'up' && r.latency_ms > 0).map(r => r.latency_ms);
      avgLatency = validLatencies.length > 0
        ? validLatencies.reduce((a, b) => a + b, 0) / validLatencies.length
        : null;

      // Compute uptime % for this window
      const total = rows.length;
      const up = rows.filter(r => r.status === 'up').length;
      windowUptime = total > 0 ? (up / total) * 100 : null;
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
        <div class="chart-stats">
          <h2>Response Time</h2>
          {#if avgLatency != null || windowUptime != null}
            <div class="stat-pills">
              {#if avgLatency != null}
                <span class="stat-pill">Avg <strong>{Math.round(avgLatency)}<span class="stat-unit">ms</span></strong></span>
              {/if}
              {#if windowUptime != null}
                <span class="stat-pill" class:good={windowUptime >= 99} class:warn={windowUptime < 99 && windowUptime >= 95} class:bad={windowUptime < 95}>
                  Uptime <strong>{windowUptime.toFixed(2)}%</strong>
                </span>
              {/if}
              {#if monitor.cert_days_left != null}
                <span class="stat-pill" class:cert-warn={monitor.cert_days_left <= 14} class:cert-danger={monitor.cert_days_left <= 3}>
                  SSL {monitor.cert_days_left}<span class="stat-unit">d</span>
                </span>
              {/if}
            </div>
          {/if}
        </div>
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
  .chart-stats {
    display: flex;
    align-items: center;
    gap: 12px;
    flex-wrap: wrap;
  }
  .chart-stats h2 { font-size: 1.1rem; }
  .stat-pills {
    display: flex;
    gap: 8px;
    flex-wrap: wrap;
  }
  .stat-pill {
    font-size: 0.8rem;
    color: var(--fg-muted);
    background: rgba(248, 248, 242, 0.05);
    padding: 2px 10px;
    border-radius: 12px;
  }
  .stat-pill strong {
    color: var(--cyan);
    font-weight: 600;
  }
  .stat-pill.good strong { color: var(--green); }
  .stat-pill.warn strong { color: var(--orange); }
  .stat-pill.bad strong { color: var(--red); }
  .stat-pill.cert-warn { color: var(--orange); }
  .stat-pill.cert-danger { color: var(--red); }
  .stat-unit {
    font-size: 0.65rem;
    font-weight: 400;
    opacity: 0.7;
  }

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
