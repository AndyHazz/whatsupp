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

  onMount(async () => {
    try {
      monitors = await api.getMonitors();
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
    // Append to sparkline
    if (sparklines[data.monitor] && data.latency_ms != null) {
      sparklines[data.monitor] = [...sparklines[data.monitor].slice(-59), data.latency_ms];
    }
  });

  onDestroy(unsub);
</script>

<div class="monitors">
  <h1>Monitors</h1>

  {#if loading}
  <div class="grid">
    <Skeleton variant="card" count={6} />
  </div>
  {:else if error}
    <p class="error">{error}</p>
  {:else if monitors.length === 0}
    <p class="muted">No monitors configured. Add monitors in Settings.</p>
  {:else}
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
  {/if}
</div>

<style>
  .monitors h1 {
    margin-bottom: var(--gap);
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
