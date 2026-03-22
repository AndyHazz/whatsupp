<script>
  import { onMount, onDestroy } from 'svelte';
  import { link, navigate } from '../lib/router.js';
  import { api } from '../lib/api.js';
  import { onMessage } from '../lib/ws.js';
  import StatusBadge from '../components/StatusBadge.svelte';
  import Sparkline from '../components/Sparkline.svelte';
  import Skeleton from '../components/Skeleton.svelte';

  function goToMonitor(name) {
    navigate('/monitors/' + encodeURIComponent(name));
  }

  let monitors = [];
  let loading = true;
  let error = '';
  let mutedNames = new Set();

  // Map of monitor name -> recent latency values for sparkline
  let sparklines = {};
  let sparklineStatuses = {};
  let viewMode = 'cards'; // 'cards' | 'list'

  async function toggleMute(name) {
    try {
      const result = await api.toggleMute(name);
      if (result.muted) {
        mutedNames.add(name);
      } else {
        mutedNames.delete(name);
      }
      mutedNames = mutedNames; // trigger reactivity
    } catch { /* ignore */ }
  }

  onMount(async () => {
    try {
      const [monitorsData, mutes] = await Promise.all([
        api.getMonitors(),
        api.getMutes(),
      ]);
      monitors = monitorsData.sort((a, b) => {
        // DOWN monitors first, then alphabetical
        if (a.status === 'down' && b.status !== 'down') return -1;
        if (a.status !== 'down' && b.status === 'down') return 1;
        return a.name.localeCompare(b.name);
      });
      mutedNames = new Set(mutes || []);
      // Fetch last 1h of results for each monitor for sparklines
      const now = Math.floor(Date.now() / 1000);
      const oneHourAgo = now - 3600;
      await Promise.all(monitors.map(async (m) => {
        try {
          const results = await api.getMonitorResults(m.name, oneHourAgo, now);
          const filtered = (results || []).filter(r => r.latency_ms != null);
          sparklines[m.name] = filtered.map(r => r.latency_ms);
          sparklineStatuses[m.name] = filtered.map(r => r.status || 'up');
        } catch { /* ignore */ }
      }));
      sparklines = sparklines; // trigger reactivity
      sparklineStatuses = sparklineStatuses;
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
    if (sparklineStatuses[data.monitor]) {
      sparklineStatuses[data.monitor] = [...sparklineStatuses[data.monitor].slice(-59), data.status || 'up'];
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
        <div class="card" class:down={m.status === 'down'} on:click={() => goToMonitor(m.name)} role="button" tabindex="0" on:keydown={(e) => e.key === 'Enter' && goToMonitor(m.name)}>
          <div class="card-top">
            <div class="card-title">
              {#if m.url}
                <a href={m.url} target="_blank" rel="noopener noreferrer" class="monitor-name service-link" on:click|stopPropagation>{m.name}</a>
              {:else}
                <span class="monitor-name">{m.name}</span>
              {/if}
            </div>
            <div class="card-stats">
              {#if m.latency_ms != null}
                <span class="latency">{Math.round(m.latency_ms)}<span class="unit">ms</span></span>
              {/if}
              <StatusBadge status={m.status} />
            </div>
          </div>
          <div class="card-meta">
            {#if m.uptime_pct != null}
              <span class="uptime" class:good={m.uptime_pct >= 99} class:warn={m.uptime_pct < 99 && m.uptime_pct >= 95} class:bad={m.uptime_pct < 95}>
                {m.uptime_pct.toFixed(1)}%
              </span>
            {/if}
            <button
              class="mute-btn"
              class:is-muted={mutedNames.has(m.name)}
              title={mutedNames.has(m.name) ? 'Unmute notifications' : 'Mute notifications'}
              on:click|stopPropagation={() => toggleMute(m.name)}
            >
              {#if mutedNames.has(m.name)}
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M13.73 21a2 2 0 0 1-3.46 0"/><path d="M18.63 13A17.89 17.89 0 0 1 18 8"/><path d="M6.26 6.26A5.86 5.86 0 0 0 6 8c0 7-3 9-3 9h14"/><path d="M18 8a6 6 0 0 0-9.33-5"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
              {:else}
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/></svg>
              {/if}
            </button>
          </div>
          <div class="card-sparkline">
            <Sparkline data={sparklines[m.name] || []} statuses={sparklineStatuses[m.name] || []} width={260} height={36} />
          </div>
        </div>
      {/each}
    </div>
    {:else}
    <div class="list">
      {#each monitors as m}
        <div class="list-row" class:down={m.status === 'down'} on:click={() => goToMonitor(m.name)} role="button" tabindex="0" on:keydown={(e) => e.key === 'Enter' && goToMonitor(m.name)}>
          <StatusBadge status={m.status} />
          {#if m.url}
            <a href={m.url} target="_blank" rel="noopener noreferrer" class="list-name service-link" on:click|stopPropagation>{m.name}</a>
          {:else}
            <span class="list-name">{m.name}</span>
          {/if}
          <span class="list-latency">{m.latency_ms != null ? Math.round(m.latency_ms) + 'ms' : '—'}</span>
          <span class="list-uptime" class:good={m.uptime_pct >= 99} class:warn={m.uptime_pct < 99 && m.uptime_pct >= 95} class:bad={m.uptime_pct < 95}>
            {m.uptime_pct != null ? m.uptime_pct.toFixed(1) + '%' : '—'}
          </span>
          <span class="list-type muted">{m.type}</span>
          <button
            class="mute-btn"
            class:is-muted={mutedNames.has(m.name)}
            title={mutedNames.has(m.name) ? 'Unmute notifications' : 'Mute notifications'}
            on:click|stopPropagation={() => toggleMute(m.name)}
          >
            {#if mutedNames.has(m.name)}
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M13.73 21a2 2 0 0 1-3.46 0"/><path d="M18.63 13A17.89 17.89 0 0 1 18 8"/><path d="M6.26 6.26A5.86 5.86 0 0 0 6 8c0 7-3 9-3 9h14"/><path d="M18 8a6 6 0 0 0-9.33-5"/><line x1="1" y1="1" x2="23" y2="23"/></svg>
            {:else}
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/></svg>
            {/if}
          </button>
        </div>
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
    color: var(--fg);
    border-bottom: 1px solid var(--border-subtle);
    cursor: pointer;
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
    color: var(--fg);
    border: 1px solid var(--border-subtle);
    box-shadow: var(--shadow-card);
    cursor: pointer;
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

  .card-top {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 4px;
  }

  .card-title {
    min-width: 0;
    overflow: hidden;
  }

  .monitor-name {
    font-weight: 600;
    font-size: 1rem;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    display: block;
  }

  .service-link {
    color: var(--fg);
    text-decoration: none;
    transition: color 0.15s ease;
  }
  .service-link:hover {
    color: var(--purple);
    text-decoration: underline;
  }

  .card-stats {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-shrink: 0;
  }

  .latency {
    color: var(--cyan);
    font-weight: 700;
    font-size: 1.1rem;
    letter-spacing: -0.5px;
  }
  .unit {
    font-size: 0.65rem;
    font-weight: 400;
    opacity: 0.7;
    margin-left: 1px;
  }

  .card-meta {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 8px;
  }

  .uptime {
    font-size: 0.8rem;
    font-weight: 500;
  }
  .uptime.good { color: var(--green); }
  .uptime.warn { color: var(--orange); }
  .uptime.bad  { color: var(--red); }

  .card-sparkline {
    width: 100%;
    overflow: hidden;
  }
  .card-sparkline :global(.sparkline) {
    width: 100%;
    height: 36px;
  }

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
  .mute-btn:hover {
    opacity: 1;
  }
  .mute-btn.is-muted {
    opacity: 0.8;
    color: var(--orange);
  }
  .mute-btn.is-muted:hover {
    opacity: 1;
  }

  .muted { color: var(--fg-muted); }
  .error { color: var(--red); }
</style>
