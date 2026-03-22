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
    <div class="scan-grid">
    <!-- Group scans by target, show latest per target -->
    {#each Object.entries(
      scans.reduce((acc, s) => {
        if (!acc[s.target] || s.timestamp > acc[s.target].timestamp) acc[s.target] = s;
        return acc;
      }, {})
    ) as [target, scan]}
      {@const baseline = baselines[target]}
      {@const diff = getDiff(scan, baseline)}
      {@const hasDrift = diff.newPorts.length > 0 || diff.missingPorts.length > 0}
      <div class="scan-card" class:drift={hasDrift}>
        <div class="scan-header">
          <div class="scan-title">
            <span class="status-icon" class:ok={!hasDrift && baseline} class:warn={hasDrift} class:none={!baseline && !hasDrift}>
              {#if hasDrift}&#9888;{:else if baseline}&#10003;{:else}&#8212;{/if}
            </span>
            <h2>{target}</h2>
          </div>
          <span class="scan-time">{formatTime(scan.timestamp)}</span>
        </div>

        <div class="scan-body">
          <div class="port-section">
            <h3>Open Ports <span class="port-count">{(scan.open_ports || []).length}</span></h3>
            <div class="port-list">
              {#each (scan.open_ports || []) as port}
                {@const isNew = diff.newPorts.includes(port)}
                <span class="port-badge" class:new-port={isNew}>
                  {port}
                  {#if isNew}<span class="new-tag">new</span>{/if}
                </span>
              {/each}
            </div>
          </div>

          {#if diff.missingPorts.length > 0}
            <div class="port-section missing">
              <h3>Missing from Baseline</h3>
              <div class="port-list">
                {#each diff.missingPorts as port}
                  <span class="port-badge missing-port">{port}</span>
                {/each}
              </div>
            </div>
          {/if}
        </div>

        <div class="scan-footer">
          {#if hasDrift}
            <button class="accept-btn" on:click={() => acceptBaseline(target)}>
              Accept as New Baseline
            </button>
          {:else if !baseline}
            <button class="accept-btn secondary" on:click={() => acceptBaseline(target)}>
              Set Baseline
            </button>
          {:else}
            <span class="baseline-match">&#10003; Matches baseline</span>
          {/if}
        </div>
      </div>
    {/each}
    </div>
  {/if}
</div>

<style>
  .security-page h1 { margin-bottom: 20px; }

  .scan-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(420px, 1fr));
    gap: var(--gap);
  }

  .scan-card {
    background: var(--bg-card);
    border-radius: var(--radius);
    border: 1px solid var(--border-subtle);
    box-shadow: var(--shadow-card);
    display: flex;
    flex-direction: column;
    overflow: hidden;
  }
  .scan-card.drift {
    border-color: rgba(255, 184, 108, 0.3);
  }

  .scan-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 14px 16px;
    border-bottom: 1px solid var(--border-subtle);
  }
  .scan-title {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .scan-header h2 { font-size: 1.05rem; }
  .scan-time { font-size: 0.75rem; color: var(--fg-muted); white-space: nowrap; }

  .status-icon {
    font-size: 1rem;
    line-height: 1;
  }
  .status-icon.ok { color: var(--green); }
  .status-icon.warn { color: var(--orange); }
  .status-icon.none { color: var(--fg-muted); }

  .scan-body {
    padding: 14px 16px;
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }

  .port-section h3 {
    font-size: 0.8rem;
    font-weight: 500;
    color: var(--fg-muted);
    margin-bottom: 8px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .port-count {
    font-weight: 600;
    color: var(--fg);
  }

  .port-list {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .port-badge {
    background: rgba(248, 248, 242, 0.06);
    padding: 3px 10px;
    border-radius: 4px;
    font-size: 0.8rem;
    font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', monospace;
    border: 1px solid rgba(248, 248, 242, 0.06);
  }
  .port-badge.new-port {
    background: rgba(255, 85, 85, 0.12);
    border-color: rgba(255, 85, 85, 0.25);
    color: var(--red);
  }
  .port-badge.missing-port {
    background: rgba(255, 184, 108, 0.12);
    border-color: rgba(255, 184, 108, 0.25);
    color: var(--orange);
  }
  .new-tag {
    font-size: 0.6rem;
    font-weight: 700;
    margin-left: 4px;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }

  .scan-footer {
    padding: 12px 16px;
    border-top: 1px solid var(--border-subtle);
  }

  .accept-btn {
    background: var(--purple);
    color: var(--bg);
    border: none;
    padding: 6px 14px;
    border-radius: var(--radius);
    font-size: 0.82rem;
    font-weight: 600;
  }
  .accept-btn.secondary {
    background: transparent;
    color: var(--purple);
    border: 1px solid var(--purple);
  }
  .accept-btn:hover { filter: brightness(1.1); }

  .baseline-match {
    color: var(--green);
    font-size: 0.82rem;
  }

  .muted { color: var(--fg-muted); }
  .error { color: var(--red); }

  @media (max-width: 520px) {
    .scan-grid {
      grid-template-columns: 1fr;
    }
  }
</style>
