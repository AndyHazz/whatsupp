<script>
  import Chart from './Chart.svelte';
  import { dracula } from '../lib/theme.js';

  export let latestMetrics = {};
  export let allMetrics = [];  // raw metric points for charts

  let expandedContainer = null;
  let sortKey = 'name';
  let sortAsc = true;

  // Parse container data from latestMetrics
  $: containers = (() => {
    const map = {};
    for (const [key, val] of Object.entries(latestMetrics)) {
      if (!key.startsWith('docker.')) continue;
      const parts = key.split('.');
      if (parts.length !== 3) continue;
      const cname = parts[1];
      const stat = parts[2];
      if (!map[cname]) map[cname] = { name: cname };
      map[cname][stat] = val;
    }
    return Object.values(map);
  })();

  $: sorted = (() => {
    const list = [...containers];
    list.sort((a, b) => {
      // Stopped containers always at bottom
      const aRunning = a.status === 1;
      const bRunning = b.status === 1;
      if (aRunning !== bRunning) return bRunning - aRunning;

      let av, bv;
      if (sortKey === 'name') {
        av = a.name.toLowerCase();
        bv = b.name.toLowerCase();
        return sortAsc ? av.localeCompare(bv) : bv.localeCompare(av);
      }
      av = a[sortKey] ?? 0;
      bv = b[sortKey] ?? 0;
      return sortAsc ? av - bv : bv - av;
    });
    return list;
  })();

  function toggleSort(key) {
    if (sortKey === key) {
      sortAsc = !sortAsc;
    } else {
      sortKey = key;
      sortAsc = key === 'name';
    }
  }

  function toggleExpand(name) {
    expandedContainer = expandedContainer === name ? null : name;
  }

  function fmtBytes(b) {
    if (b == null) return '—';
    if (b >= 1e12) return (b / 1e12).toFixed(1) + ' TB';
    if (b >= 1e9) return (b / 1e9).toFixed(1) + ' GB';
    if (b >= 1e6) return (b / 1e6).toFixed(0) + ' MB';
    if (b >= 1e3) return (b / 1e3).toFixed(0) + ' KB';
    return Math.round(b) + ' B';
  }

  function fmtRate(b) {
    if (b == null) return '—';
    return fmtBytes(b) + '/s';
  }

  function fmtPct(v) {
    if (v == null) return '—';
    return v.toFixed(1) + '%';
  }

  function sortArrow(key) {
    if (sortKey !== key) return '';
    return sortAsc ? ' \u25B2' : ' \u25BC';
  }

  // Build chart data for an expanded container
  function containerChartData(cname, suffix) {
    const metricName = `docker.${cname}.${suffix}`;
    const byName = {};
    for (const m of allMetrics) {
      if (m.metric_name === metricName) {
        if (!byName[metricName]) byName[metricName] = [];
        byName[metricName].push(m);
      }
    }
    const points = byName[metricName] || [];
    if (!points.length) return null;
    return [
      points.map(p => p.timestamp),
      points.map(p => p.value),
    ];
  }

  // Build multi-series chart (e.g. rx + tx)
  function containerDualChart(cname, suffix1, suffix2) {
    const name1 = `docker.${cname}.${suffix1}`;
    const name2 = `docker.${cname}.${suffix2}`;
    const pts1 = allMetrics.filter(m => m.metric_name === name1);
    const pts2 = allMetrics.filter(m => m.metric_name === name2);
    if (!pts1.length && !pts2.length) return null;
    // Use timestamps from whichever has data
    const source = pts1.length >= pts2.length ? pts1 : pts2;
    const ts = source.map(p => p.timestamp);
    const tsSet = new Set(ts);
    // Build value maps
    const map1 = {};
    for (const p of pts1) map1[p.timestamp] = p.value;
    const map2 = {};
    for (const p of pts2) map2[p.timestamp] = p.value;
    return [
      ts,
      ts.map(t => map1[t] ?? null),
      ts.map(t => map2[t] ?? null),
    ];
  }
</script>

{#if containers.length === 0}
  <p class="no-containers">No containers detected</p>
{:else}
  <div class="container-table-wrap">
    <table class="container-table">
      <thead>
        <tr>
          <th class="th-status"></th>
          <th class="th-name sortable" on:click={() => toggleSort('name')}>Name{sortArrow('name')}</th>
          <th class="th-num sortable" on:click={() => toggleSort('cpu_pct')}>CPU{sortArrow('cpu_pct')}</th>
          <th class="th-num sortable" on:click={() => toggleSort('mem_bytes')}>Memory{sortArrow('mem_bytes')}</th>
          <th class="th-num sortable" on:click={() => toggleSort('net_rx_bytes_sec')}>Net RX{sortArrow('net_rx_bytes_sec')}</th>
          <th class="th-num sortable" on:click={() => toggleSort('net_tx_bytes_sec')}>Net TX{sortArrow('net_tx_bytes_sec')}</th>
          <th class="th-num sortable" on:click={() => toggleSort('disk_read_bytes_sec')}>Disk R{sortArrow('disk_read_bytes_sec')}</th>
          <th class="th-num sortable" on:click={() => toggleSort('disk_write_bytes_sec')}>Disk W{sortArrow('disk_write_bytes_sec')}</th>
        </tr>
      </thead>
      <tbody>
        {#each sorted as c (c.name)}
          <tr
            class="container-row"
            class:stopped={c.status !== 1}
            class:expanded={expandedContainer === c.name}
            on:click={() => c.status === 1 && toggleExpand(c.name)}
          >
            <td class="td-status">
              <span class="status-dot" class:running={c.status === 1} class:down={c.status !== 1}></span>
            </td>
            <td class="td-name">{c.name}</td>
            <td class="td-num">{c.status === 1 ? fmtPct(c.cpu_pct) : '—'}</td>
            <td class="td-num">
              {#if c.status === 1 && c.mem_bytes != null}
                {fmtBytes(c.mem_bytes)}{#if c.mem_limit_bytes} / {fmtBytes(c.mem_limit_bytes)}{/if}
              {:else}
                —
              {/if}
            </td>
            <td class="td-num">{c.status === 1 ? fmtRate(c.net_rx_bytes_sec) : '—'}</td>
            <td class="td-num">{c.status === 1 ? fmtRate(c.net_tx_bytes_sec) : '—'}</td>
            <td class="td-num">{c.status === 1 ? fmtRate(c.disk_read_bytes_sec) : '—'}</td>
            <td class="td-num">{c.status === 1 ? fmtRate(c.disk_write_bytes_sec) : '—'}</td>
          </tr>
          {#if expandedContainer === c.name}
            <tr class="expanded-row">
              <td colspan="8">
                <div class="expanded-charts">
                  {#if containerChartData(c.name, 'cpu_pct')}
                    <div class="mini-chart">
                      <h4>CPU</h4>
                      <Chart data={containerChartData(c.name, 'cpu_pct')} label="CPU" unit="%" color={dracula.purple} height={160} />
                    </div>
                  {/if}
                  {#if containerChartData(c.name, 'mem_bytes')}
                    <div class="mini-chart">
                      <h4>Memory</h4>
                      <Chart data={containerChartData(c.name, 'mem_bytes')} label="Memory" unit=" B" color={dracula.pink} height={160} />
                    </div>
                  {/if}
                  {#if containerChartData(c.name, 'net_rx_bytes_sec') || containerChartData(c.name, 'net_tx_bytes_sec')}
                    <div class="mini-chart">
                      <h4>Network</h4>
                      {#if containerChartData(c.name, 'net_rx_bytes_sec')}
                        <Chart data={containerChartData(c.name, 'net_rx_bytes_sec')} label="RX" unit=" B/s" color={dracula.cyan} height={160} />
                      {/if}
                    </div>
                  {/if}
                  {#if containerChartData(c.name, 'disk_read_bytes_sec') || containerChartData(c.name, 'disk_write_bytes_sec')}
                    <div class="mini-chart">
                      <h4>Disk I/O</h4>
                      {#if containerChartData(c.name, 'disk_read_bytes_sec')}
                        <Chart data={containerChartData(c.name, 'disk_read_bytes_sec')} label="Read" unit=" B/s" color={dracula.orange} height={160} />
                      {/if}
                    </div>
                  {/if}
                </div>
              </td>
            </tr>
          {/if}
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<style>
  .container-table-wrap {
    overflow-x: auto;
  }

  .container-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.85rem;
  }

  .container-table th {
    text-align: left;
    padding: 8px 10px;
    color: var(--fg-muted);
    font-weight: 600;
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    border-bottom: 1px solid var(--border-subtle);
    white-space: nowrap;
    user-select: none;
  }

  .sortable {
    cursor: pointer;
  }
  .sortable:hover {
    color: var(--fg);
  }

  .th-status { width: 24px; }
  .th-num { text-align: right; }

  .container-table td {
    padding: 6px 10px;
    border-bottom: 1px solid var(--border-subtle);
  }

  .td-num { text-align: right; font-variant-numeric: tabular-nums; }
  .td-name { font-weight: 500; }

  .container-row {
    cursor: pointer;
    transition: background 0.1s ease;
  }
  .container-row:hover {
    background: rgba(255, 255, 255, 0.03);
  }
  .container-row.stopped {
    opacity: 0.45;
    cursor: default;
  }
  .container-row.expanded {
    background: rgba(189, 147, 249, 0.05);
  }

  .status-dot {
    display: inline-block;
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--fg-muted);
  }
  .status-dot.running { background: var(--green); }
  .status-dot.down { background: var(--red); }

  .expanded-row td {
    padding: 12px 10px;
    background: rgba(189, 147, 249, 0.03);
    border-bottom: 1px solid var(--border-subtle);
  }

  .expanded-charts {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 12px;
  }

  .mini-chart {
    background: var(--bg-card);
    padding: 10px;
    border-radius: var(--radius);
    border: 1px solid var(--border-subtle);
  }
  .mini-chart h4 {
    font-size: 0.8rem;
    color: var(--fg-muted);
    margin-bottom: 6px;
  }

  .no-containers {
    color: var(--fg-muted);
    padding: 16px;
  }
</style>
