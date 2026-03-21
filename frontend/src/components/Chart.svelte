<script>
  import { onMount, onDestroy, afterUpdate } from 'svelte';
  import uPlot from 'uplot';
  import 'uplot/dist/uPlot.min.css';
  import { dracula } from '../lib/theme.js';

  export let data = [[], []]; // [timestamps[], values[]]
  export let label = 'Value';
  export let unit = 'ms';
  export let color = dracula.cyan;
  export let height = 300;
  export let fillAlpha = 0.1;

  let container;
  let chart = null;

  const opts = () => ({
    width: container?.clientWidth || 800,
    height,
    cursor: {
      drag: { x: true, y: false, setScale: true },
    },
    select: {
      show: true,
    },
    scales: {
      x: { time: true },
      y: { auto: true },
    },
    axes: [
      {
        stroke: dracula.comment,
        grid: { stroke: `${dracula.comment}33`, width: 1 },
        ticks: { stroke: `${dracula.comment}55`, width: 1 },
        font: '11px sans-serif',
      },
      {
        stroke: dracula.comment,
        grid: { stroke: `${dracula.comment}33`, width: 1 },
        ticks: { stroke: `${dracula.comment}55`, width: 1 },
        font: '11px sans-serif',
        values: (u, vals) => vals.map(v => v != null ? `${v}${unit}` : ''),
      },
    ],
    series: [
      {},
      {
        label,
        stroke: color,
        width: 1.5,
        fill: `${color}${Math.round(fillAlpha * 255).toString(16).padStart(2, '0')}`,
        points: { show: false },
      },
    ],
  });

  function create() {
    if (chart) chart.destroy();
    if (!container || !data[0].length) return;
    chart = new uPlot(opts(), data, container);
  }

  function resize() {
    if (chart && container) {
      chart.setSize({ width: container.clientWidth, height });
    }
  }

  onMount(() => {
    create();
    window.addEventListener('resize', resize);
  });

  afterUpdate(() => {
    if (chart && data[0].length) {
      chart.setData(data);
    } else {
      create();
    }
  });

  onDestroy(() => {
    window.removeEventListener('resize', resize);
    if (chart) chart.destroy();
  });
</script>

<div class="chart-wrap" bind:this={container}>
  {#if !data[0].length}
    <div class="no-data">No data available</div>
  {/if}
</div>

<style>
  .chart-wrap {
    width: 100%;
    min-height: 100px;
    position: relative;
  }
  .chart-wrap :global(.u-wrap) {
    background: var(--bg);
    border-radius: var(--radius);
  }
  .chart-wrap :global(.u-legend) {
    font-size: 0.85rem;
    color: var(--fg-muted);
  }
  .chart-wrap :global(.u-select) {
    background: rgba(189, 147, 249, 0.1);
  }
  .no-data {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 200px;
    color: var(--fg-muted);
  }
</style>
