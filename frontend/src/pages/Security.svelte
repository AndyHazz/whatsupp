<script>
  import { onMount, onDestroy } from 'svelte';
  import { api } from '../lib/api.js';
  import { onMessage } from '../lib/ws.js';
  import Skeleton from '../components/Skeleton.svelte';

  let scans = [];
  let baselines = {};
  let schedules = {};
  let loading = true;
  let error = '';

  // Live scan progress: target -> { scanned, total }
  let activeScans = {};

  async function loadData() {
    try {
      const [s, b, sched] = await Promise.all([
        api.getScans(),
        api.getBaselines(),
        api.getScanSchedules(),
      ]);
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
      // schedules comes as a map of target -> schedule object
      schedules = sched || {};
      // Seed active scans from schedules (in case a scan is already running)
      for (const [target, info] of Object.entries(schedules)) {
        if (info.scanning) {
          activeScans[target] = { scanned: info.scanned, total: info.total };
        }
      }
      activeScans = activeScans;
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  onMount(() => loadData());

  // WebSocket listeners for live scan updates
  const unsubs = [
    onMessage('security_scan_start', (data) => {
      activeScans[data.target] = { scanned: 0, total: data.total };
      activeScans = activeScans;
    }),
    onMessage('security_scan_progress', (data) => {
      activeScans[data.target] = { scanned: data.scanned, total: data.total };
      activeScans = activeScans;
    }),
    onMessage('security_scan_complete', (data) => {
      delete activeScans[data.target];
      activeScans = activeScans;
      // Refresh data to show new results
      loadData();
    }),
    onMessage('security_scan_scheduled', (data) => {
      if (schedules[data.target]) {
        schedules[data.target] = { ...schedules[data.target], next_run: data.next_run };
      } else {
        schedules[data.target] = { target: data.target, next_run: data.next_run };
      }
      schedules = schedules;
    }),
  ];

  onDestroy(() => unsubs.forEach(fn => fn()));

  async function acceptBaseline(target) {
    try {
      await api.updateBaseline(target);
      await loadData();
    } catch (e) {
      error = e.message;
    }
  }

  function formatTime(ts) {
    if (!ts) return 'Never';
    return new Date(ts * 1000).toLocaleString();
  }

  function formatNextRun(ts) {
    if (!ts) return null;
    const now = Date.now() / 1000;
    const diff = ts - now;
    if (diff <= 0) return 'now';
    if (diff < 3600) return `${Math.round(diff / 60)}m`;
    if (diff < 86400) return `${Math.round(diff / 3600)}h`;
    // Show day name + time for longer intervals so the cron schedule is visible
    const d = new Date(ts * 1000);
    const day = d.toLocaleDateString(undefined, { weekday: 'short' });
    const time = d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
    return `${day} ${time}`;
  }

  function getDiff(scan, baseline) {
    if (!baseline) return { newPorts: scan.open_ports || [], missingPorts: [] };
    const expected = new Set(baseline.expected_ports || []);
    const actual   = new Set(scan.open_ports || []);
    const newPorts     = [...actual].filter(p => !expected.has(p));
    const missingPorts = [...expected].filter(p => !actual.has(p));
    return { newPorts, missingPorts };
  }

  function getSchedule(target) {
    return schedules[target] || null;
  }
</script>

<div class="security-page">
  <h1>Security</h1>

  {#if loading}
    <Skeleton variant="security" count={2} />
  {:else if error}
    <p class="error">{error}</p>
  {:else if scans.length === 0 && Object.keys(schedules).length === 0}
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
      {@const active = activeScans[target]}
      {@const sched = getSchedule(target)}
      <div class="scan-card" class:drift={hasDrift} class:scanning={active}>
        <div class="scan-header">
          <div class="scan-title">
            {#if active}
              <span class="status-icon scanning-icon">&#8987;</span>
            {:else}
              <span class="status-icon" class:ok={!hasDrift && baseline} class:warn={hasDrift} class:none={!baseline && !hasDrift}>
                {#if hasDrift}&#9888;{:else if baseline}&#10003;{:else}&#8212;{/if}
              </span>
            {/if}
            <h2>{target}</h2>
          </div>
          <span class="scan-time">{formatTime(scan.timestamp)}</span>
        </div>

        {#if active}
          <div class="progress-section">
            <div class="progress-bar">
              <div class="progress-fill" style="width: {active.total ? (active.scanned / active.total * 100) : 0}%"></div>
            </div>
            <span class="progress-text">
              Scanning... {active.total ? Math.round(active.scanned / active.total * 100) : 0}%
              <span class="progress-detail">{active.scanned.toLocaleString()} / {active.total.toLocaleString()} ports</span>
            </span>
          </div>
        {/if}

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
          <div class="scan-footer-left">
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
          {#if sched && sched.next_run && !active}
            <span class="next-scan" title={formatTime(sched.next_run)}>
              Next: {formatNextRun(sched.next_run)}
            </span>
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
  .scan-card.scanning {
    border-color: rgba(139, 233, 253, 0.3);
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
  .scanning-icon { color: var(--cyan); }

  /* Progress bar */
  .progress-section {
    padding: 12px 16px;
    border-bottom: 1px solid var(--border-subtle);
  }
  .progress-bar {
    height: 4px;
    background: rgba(248, 248, 242, 0.06);
    border-radius: 2px;
    overflow: hidden;
    margin-bottom: 6px;
  }
  .progress-fill {
    height: 100%;
    background: var(--cyan);
    border-radius: 2px;
    transition: width 0.3s ease;
  }
  .progress-text {
    font-size: 0.78rem;
    color: var(--cyan);
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .progress-detail {
    color: var(--fg-muted);
    font-size: 0.72rem;
  }

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
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .next-scan {
    font-size: 0.72rem;
    color: var(--fg-muted);
    cursor: default;
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
