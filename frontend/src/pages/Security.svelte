<script>
  import { onMount } from 'svelte';
  import { api } from '../lib/api.js';
  import Skeleton from '../components/Skeleton.svelte';

  let scans = [];
  let baselines = {};
  let loading = true;
  let error = '';

  onMount(async () => {
    try {
      const [s, b] = await Promise.all([
        api.getScans(),
        api.getBaselines(),
      ]);
      // Parse JSON port strings into arrays
      scans = (s || []).map(sc => ({
        ...sc,
        open_ports: typeof sc.open_ports_json === 'string' ? JSON.parse(sc.open_ports_json || '[]') : (sc.open_ports_json || []),
      }));
      baselines = {};
      for (const bl of (b || [])) {
        baselines[bl.target] = {
          ...bl,
          expected_ports: typeof bl.expected_ports_json === 'string' ? JSON.parse(bl.expected_ports_json || '[]') : (bl.expected_ports_json || []),
        };
      }
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  });

  async function acceptBaseline(target) {
    try {
      await api.updateBaseline(target);
      // Refresh data
      const [s, b] = await Promise.all([api.getScans(), api.getBaselines()]);
      scans = (s || []).map(sc => ({
        ...sc,
        open_ports: typeof sc.open_ports_json === 'string' ? JSON.parse(sc.open_ports_json || '[]') : (sc.open_ports_json || []),
      }));
      baselines = {};
      for (const bl of (b || [])) baselines[bl.target] = {
        ...bl,
        expected_ports: typeof bl.expected_ports_json === 'string' ? JSON.parse(bl.expected_ports_json || '[]') : (bl.expected_ports_json || []),
      };
    } catch (e) {
      error = e.message;
    }
  }

  function formatTime(ts) {
    if (!ts) return 'Never';
    return new Date(ts * 1000).toLocaleString();
  }

  function getDiff(scan, baseline) {
    if (!baseline) return { newPorts: scan.open_ports || [], missingPorts: [] };
    const expected = new Set(baseline.expected_ports || []);
    const actual   = new Set(scan.open_ports || []);
    const newPorts     = [...actual].filter(p => !expected.has(p));
    const missingPorts = [...expected].filter(p => !actual.has(p));
    return { newPorts, missingPorts };
  }

  function daysUntil(ts) {
    if (!ts) return null;
    const diff = ts - Math.floor(Date.now() / 1000);
    return Math.floor(diff / 86400);
  }
</script>

<div class="security-page">
  <h1>Security</h1>

  {#if loading}
    <Skeleton variant="card" count={2} />
  {:else if error}
    <p class="error">{error}</p>
  {:else if scans.length === 0}
    <p class="muted">No security scans have been run yet. Configure scan targets in Settings.</p>
  {:else}
    <!-- Group scans by target, show latest per target -->
    {#each Object.entries(
      scans.reduce((acc, s) => {
        if (!acc[s.target] || s.timestamp > acc[s.target].timestamp) acc[s.target] = s;
        return acc;
      }, {})
    ) as [target, scan]}
      {@const baseline = baselines[target]}
      {@const diff = getDiff(scan, baseline)}
      <div class="scan-card">
        <div class="scan-header">
          <h2>{target}</h2>
          <span class="scan-time">Last scan: {formatTime(scan.timestamp)}</span>
        </div>

        <div class="ports">
          <h3>Open Ports ({(scan.open_ports || []).length})</h3>
          <div class="port-list">
            {#each (scan.open_ports || []) as port}
              {@const isNew = diff.newPorts.includes(port)}
              <span class="port-badge" class:new-port={isNew}>
                {port}
                {#if isNew}<span class="new-tag">NEW</span>{/if}
              </span>
            {/each}
          </div>
        </div>

        {#if diff.missingPorts.length > 0}
          <div class="missing">
            <h3>Missing from Baseline</h3>
            <div class="port-list">
              {#each diff.missingPorts as port}
                <span class="port-badge missing-port">{port}</span>
              {/each}
            </div>
          </div>
        {/if}

        {#if diff.newPorts.length > 0 || diff.missingPorts.length > 0}
          <button class="accept-btn" on:click={() => acceptBaseline(target)}>
            Accept as New Baseline
          </button>
        {:else if !baseline}
          <button class="accept-btn" on:click={() => acceptBaseline(target)}>
            Set Baseline
          </button>
        {:else}
          <p class="baseline-match">Matches baseline</p>
        {/if}
      </div>
    {/each}
  {/if}
</div>

<style>
  .security-page h1 { margin-bottom: var(--gap); }

  .scan-card {
    background: var(--bg-card);
    padding: 16px;
    border-radius: var(--radius);
    margin-bottom: var(--gap);
    border: 1px solid var(--border-subtle);
    box-shadow: var(--shadow-card);
  }
  .scan-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
  }
  .scan-header h2 { font-size: 1.1rem; }
  .scan-time { font-size: 0.8rem; color: var(--fg-muted); }

  .ports, .missing {
    margin-bottom: 12px;
  }
  .ports h3, .missing h3 {
    font-size: 0.9rem;
    color: var(--fg-muted);
    margin-bottom: 8px;
  }

  .port-list {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .port-badge {
    background: rgba(248, 248, 242, 0.08);
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 0.85rem;
    font-family: monospace;
  }
  .port-badge.new-port {
    background: rgba(255, 85, 85, 0.15);
    color: var(--red);
  }
  .port-badge.missing-port {
    background: rgba(255, 184, 108, 0.15);
    color: var(--orange);
  }
  .new-tag {
    font-size: 0.65rem;
    font-weight: 700;
    margin-left: 4px;
    text-transform: uppercase;
  }

  .accept-btn {
    background: var(--purple);
    color: var(--bg);
    border: none;
    padding: 6px 14px;
    border-radius: var(--radius);
    font-size: 0.85rem;
    font-weight: 600;
    margin-top: 8px;
  }
  .accept-btn:hover { filter: brightness(1.1); }

  .baseline-match {
    color: var(--green);
    font-size: 0.85rem;
    margin-top: 8px;
  }

  .muted { color: var(--fg-muted); }
  .error { color: var(--red); }
</style>
