<script>
  import { onMount, onDestroy } from 'svelte';
  import { link } from '../lib/router.js';
  import { api } from '../lib/api.js';
  import { onMessage } from '../lib/ws.js';
  import StatusBadge from '../components/StatusBadge.svelte';
  import Sparkline from '../components/Sparkline.svelte';
  import Skeleton from '../components/Skeleton.svelte';

  let monitors = [];
  let loading = true;
  let error = '';

  // Map of monitor name -> recent latency values for sparkline
  let sparklines = {};
  let viewMode = 'cards'; // 'cards' | 'list'

  onMount(async () => {
    try {
      monitors = (await api.getMonitors()).sort((a, b) => {
        // DOWN monitors first, then alphabetical
        if (a.status === 'down' && b.status !== 'down') return -1;
        if (a.status !== 'down' && b.status === 'down') return 1;
        return a.name.localeCompare(b.name);
      });
      // Fetch last 1h of results for each monitor for sparklines
      const now = Math.floor(Date.now() / 1000);
      const oneHourAgo = now - 3600;
      await Promise.all(monitors.map(async (m) => {
        try {
          const results = await api.getMonitorResults(m.name, oneHourAgo, now);
          sparklines[m.name] = (results || []).map(r => r.latency_ms).filter(v => v != null);
        } catch { /* ignore */ }
      }));
      sparklines = sparklines; // trigger reactivity
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  });

  // Live updates
  const unsub = onMessage('check_result', (data) => {
    // Update monitor status
    monitors = monitors.map(m => {
      if (m.name === data.monitor) {
        return { ...m, status: data.status, latency_ms: data.latency_ms };
      }
      return m;
    });
    // Re-sort: DOWN first, then alphabetical
    monitors = monitors.sort((a, b) => {
      if (a.status === 'down' && b.status !== 'down') return -1;
      if (a.status !== 'down' && b.status === 'down') return 1;
      return a.name.localeCompare(b.name);
    });
    // Append to sparkline
    if (sparklines[data.monitor] && data.latency_ms != null) {
      sparklines[data.monitor] = [...sparklines[data.monitor].slice(-59), data.latency_ms];
    }
  });

  onDestroy(unsub);
</script>

<div class="monitors">
  <div class="page-header">
    <h1>Monitors</h1>
    <div class="view-toggle">
      <button class:active={viewMode === 'cards'} on:click={() => viewMode = 'cards'} title="Card view">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><rect x="1" y="1" width="6" height="6" rx="1"/><rect x="9" y="1" width="6" height="6" rx="1"/><rect x="1" y="9" width="6" height="6" rx="1"/><rect x="9" y="9" width="6" height="6" rx="1"/></svg>
      </button>
      <button class:active={viewMode === 'list'} on:click={() => viewMode = 'list'} title="List view">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor"><rect x="1" y="2" width="14" height="2.5" rx="1"/><rect x="1" y="6.75" width="14" height="2.5" rx="1"/><rect x="1" y="11.5" width="14" height="2.5" rx="1"/></svg>
      </button>
    </div>
  </div>

  {#if loading}
  <div class="grid">
    <Skeleton variant="card" count={6} />
  </div>
  {:else if error}
    <p class="error">{error}</p>
  {:else if monitors.length === 0}
    <p class="muted">No monitors configured. Add monitors in Settings.</p>
  {:else}
    {#if viewMode === 'cards'}
    <div class="grid">
      {#each monitors as m}
        <a href="/monitors/{encodeURIComponent(m.name)}" use:link class="card" class:down={m.status === 'down'}>
          <div class="card-header">
            <span class="monitor-name">{m.name}</span>
            <StatusBadge status={m.status} />
          </div>
          <div class="card-body">
            <Sparkline data={sparklines[m.name] || []} />
            <div class="meta">
              {#if m.latency_ms != null}
                <span class="latency">{Math.round(m.latency_ms)}<span class="unit">ms</span></span>
              {/if}
              {#if m.uptime_pct != null}
                <span class="uptime" class:good={m.uptime_pct >= 99} class:warn={m.uptime_pct < 99 && m.uptime_pct >= 95} class:bad={m.uptime_pct < 95}>
                  {m.uptime_pct.toFixed(1)}%
                </span>
              {/if}
            </div>
          </div>
          <div class="card-footer muted">
            {m.type} &middot; {m.interval || '60s'}
          </div>
        </a>
      {/each}
    </div>
    {:else}
    <div class="list">
      {#each monitors as m}
        <a href="/monitors/{encodeURIComponent(m.name)}" use:link class="list-row" class:down={m.status === 'down'}>
          <StatusBadge status={m.status} />
          <span class="list-name">{m.name}</span>
          <span class="list-latency">{m.latency_ms != null ? Math.round(m.latency_ms) + 'ms' : '—'}</span>
          <span class="list-uptime" class:good={m.uptime_pct >= 99} class:warn={m.uptime_pct < 99 && m.uptime_pct >= 95} class:bad={m.uptime_pct < 95}>
            {m.uptime_pct != null ? m.uptime_pct.toFixed(1) + '%' : '—'}
          </span>
          <span class="list-type muted">{m.type}</span>
        </a>
      {/each}
    </div>
    {/if}
  {/if}
</div>

<style>
  .page-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: var(--gap);
  }
  .page-header h1 { margin-bottom: 0; }

  .view-toggle {
    display: flex;
    gap: 2px;
  }
  .view-toggle button {
    background: var(--bg-card);
    border: 1px solid var(--border-subtle);
    color: var(--fg-muted);
    padding: 6px 8px;
    border-radius: var(--radius);
    display: flex;
    align-items: center;
  }
  .view-toggle button.active {
    background: var(--purple);
    border-color: var(--purple);
    color: var(--bg);
  }

  .list {
    display: flex;
    flex-direction: column;
    background: var(--bg-card);
    border-radius: var(--radius);
    border: 1px solid var(--border-subtle);
    box-shadow: var(--shadow-card);
    overflow: hidden;
  }
  .list-row {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px 16px;
    text-decoration: none;
    color: var(--fg);
    border-bottom: 1px solid var(--border-subtle);
    transition: background-color 0.15s ease;
  }
  .list-row:last-child { border-bottom: none; }
  .list-row:hover {
    background-color: rgba(248, 248, 242, 0.04);
    text-decoration: none;
  }
  .list-row.down {
    border-left: 3px solid var(--red);
  }
  .list-name {
    font-weight: 600;
    flex: 1;
  }
  .list-latency {
    color: var(--cyan);
    font-weight: 600;
    min-width: 60px;
    text-align: right;
  }
  .list-uptime {
    min-width: 50px;
    text-align: right;
  }
  .list-uptime.good { color: var(--green); }
  .list-uptime.warn { color: var(--orange); }
  .list-uptime.bad { color: var(--red); }
  .list-type {
    min-width: 40px;
    text-align: right;
    font-size: 0.8rem;
    text-transform: uppercase;
  }

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
  .card.down {
    border-left: 3px solid var(--red);
  }

  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
  }

  .monitor-name {
    font-weight: 600;
    font-size: 1.05rem;
  }

  .card-body {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 8px;
  }

  .meta {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 2px;
    font-size: 0.85rem;
  }

  .latency {
    color: var(--cyan);
    font-weight: 700;
    font-size: 1.25rem;
    letter-spacing: -0.5px;
  }
  .unit {
    font-size: 0.7rem;
    font-weight: 400;
    opacity: 0.7;
    margin-left: 1px;
  }

  .uptime.good { color: var(--green); }
  .uptime.warn { color: var(--orange); }
  .uptime.bad  { color: var(--red); }

  .card-footer {
    font-size: 0.75rem;
    text-transform: uppercase;
    letter-spacing: 0.8px;
  }

  .muted { color: var(--fg-muted); }
  .error { color: var(--red); }
</style>
