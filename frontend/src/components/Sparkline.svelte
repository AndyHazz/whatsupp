<script>
  export let data = []; // array of numbers (latency_ms values)
  export let statuses = []; // array of 'up'|'down' matching data indices
  export let width = 120;
  export let height = 30;
  export let color = 'var(--cyan)';

  // Find downtime segments (consecutive DOWN statuses)
  $: downtimeRanges = (() => {
    if (!statuses.length || !data.length) return [];
    const step = width / Math.max(data.length - 1, 1);
    const ranges = [];
    let start = null;
    for (let i = 0; i < statuses.length; i++) {
      if (statuses[i] === 'down' && start === null) {
        start = i;
      } else if (statuses[i] !== 'down' && start !== null) {
        ranges.push({ x: start * step, w: (i - start) * step });
        start = null;
      }
    }
    if (start !== null) {
      ranges.push({ x: start * step, w: (statuses.length - start) * step });
    }
    return ranges;
  })();

  $: points = (() => {
    if (!data.length) return '';
    const max = Math.max(...data, 1);
    const min = Math.min(...data, 0);
    const range = max - min || 1;
    const step = width / Math.max(data.length - 1, 1);
    return data
      .map((v, i) => `${i * step},${height - ((v - min) / range) * (height - 4) - 2}`)
      .join(' ');
  })();
</script>

<svg {width} {height} viewBox="0 0 {width} {height}" class="sparkline">
  {#each downtimeRanges as range}
    <rect
      x={range.x}
      y="0"
      width={Math.max(range.w, 2)}
      height={height}
      fill="rgba(255, 85, 85, 0.25)"
      rx="1"
    />
  {/each}
  {#if data.length > 1}
    <polyline
      points={points}
      fill="none"
      stroke={color}
      stroke-width="1.5"
      stroke-linecap="round"
      stroke-linejoin="round"
    />
  {/if}
</svg>

<style>
  .sparkline {
    display: block;
  }
</style>
