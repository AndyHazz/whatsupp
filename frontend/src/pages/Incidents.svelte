<script>
  import { onMount, onDestroy } from 'svelte';
  import { link } from '../lib/router.js';
  import { api } from '../lib/api.js';
  import { onMessage } from '../lib/ws.js';
  import StatusBadge from '../components/StatusBadge.svelte';
  import Skeleton from '../components/Skeleton.svelte';

  let incidents = [];
  let loading = true;
  let error = '';
  let sortKey = 'started_at';
  let sortAsc = false;

  onMount(async () => {
    try {
      incidents = (await api.getIncidents()) || [];
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  });

  // Live incident updates
  const unsub = onMessage('incident', (data) => {
    const idx = incidents.findIndex(i => i.id === data.id);
    if (idx >= 0) {
      incidents[idx] = { ...incidents[idx], ...data };
      incidents = [...incidents];
    } else {
      incidents = [data, ...incidents];
    }
  });

  onDestroy(unsub);

  function sort(key) {
    if (sortKey === key) {
      sortAsc = !sortAsc;
    } else {
      sortKey = key;
      sortAsc = false;
    }
  }

  $: sorted = [...incidents].sort((a, b) => {
    let va = a[sortKey], vb = b[sortKey];
    if (va == null) va = 0;
    if (vb == null) vb = 0;
    if (typeof va === 'string') {
      return sortAsc ? va.localeCompare(vb) : vb.localeCompare(va);
    }
    return sortAsc ? va - vb : vb - va;
  });

  function formatTime(ts) {
    if (!ts) return '-';
    return new Date(ts * 1000).toLocaleString();
  }

  function formatDuration(started, resolved) {
    if (!resolved) return 'Ongoing';
    const secs = resolved - started;
    if (secs < 60) return `${secs}s`;
    if (secs < 3600) return `${Math.floor(secs / 60)}m ${secs % 60}s`;
    const h = Math.floor(secs / 3600);
    const m = Math.floor((secs % 3600) / 60);
    return `${h}h ${m}m`;
  }
</script>

<div class="incidents-page">
  <h1>Incidents</h1>

  {#if loading}
  <div class="table-wrap">
    <table>
      <thead>
        <tr><th>Started</th><th>Monitor</th><th>Status</th><th>Duration</th><th>Cause</th></tr>
      </thead>
      <tbody>
        <Skeleton variant="table-row" count={5} />
      </tbody>
    </table>
  </div>
  {:else if error}
    <p class="error">{error}</p>
  {:else if incidents.length === 0}
    <p class="muted">No incidents recorded.</p>
  {:else}
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th class="sortable" on:click={() => sort('started_at')}>
              Started {sortKey === 'started_at' ? (sortAsc ? '▲' : '▼') : ''}
            </th>
            <th class="sortable" on:click={() => sort('monitor')}>
              Monitor {sortKey === 'monitor' ? (sortAsc ? '▲' : '▼') : ''}
            </th>
            <th>Status</th>
            <th class="sortable" on:click={() => sort('resolved_at')}>
              Duration {sortKey === 'resolved_at' ? (sortAsc ? '▲' : '▼') : ''}
            </th>
            <th>Cause</th>
          </tr>
        </thead>
        <tbody>
          {#each sorted as inc}
            <tr>
              <td>{formatTime(inc.started_at)}</td>
              <td>
                <a href="/monitors/{encodeURIComponent(inc.monitor)}" use:link>
                  {inc.monitor}
                </a>
              </td>
              <td>
                {#if inc.resolved_at}
                  <StatusBadge status="up" />
                {:else}
                  <StatusBadge status="down" />
                {/if}
              </td>
              <td>{formatDuration(inc.started_at, inc.resolved_at)}</td>
              <td class="cause">{inc.cause || '-'}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<style>
  .incidents-page h1 { margin-bottom: var(--gap); }

  .table-wrap {
    background: var(--bg-card);
    border-radius: var(--radius);
    overflow-x: auto;
    border: 1px solid var(--border-subtle);
    box-shadow: var(--shadow-card);
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.9rem;
  }
  th {
    text-align: left;
    padding: 12px;
    border-bottom: 1px solid var(--border-subtle);
    color: var(--fg-muted);
    font-weight: 600;
    white-space: nowrap;
  }
  th.sortable {
    cursor: pointer;
    user-select: none;
  }
  th.sortable:hover { color: var(--fg); }

  td {
    padding: 10px 12px;
    border-bottom: 1px solid var(--border-subtle);
  }
  .cause { color: var(--red); max-width: 300px; }

  tbody tr {
    transition: background-color 0.15s ease;
  }
  tbody tr:hover {
    background-color: rgba(248, 248, 242, 0.04);
  }

  .muted { color: var(--fg-muted); }
  .error { color: var(--red); }
</style>
