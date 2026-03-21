<script>
  export let data = []; // array of numbers (latency_ms values)
  export let width = 120;
  export let height = 30;
  export let color = 'var(--cyan)';

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
