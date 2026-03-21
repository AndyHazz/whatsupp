<script>
  export let value = 0;   // 0-100
  export let label = '';
  export let size = 80;

  $: pct = Math.min(Math.max(value, 0), 100);
  $: color = pct >= 90 ? 'var(--red)' : pct >= 75 ? 'var(--orange)' : 'var(--green)';
  $: circumference = 2 * Math.PI * 34;
  $: offset = circumference * (1 - pct / 100);
</script>

<div class="gauge" style="width:{size}px">
  <svg viewBox="0 0 80 80" width={size} height={size}>
    <circle cx="40" cy="40" r="34" fill="none" stroke="var(--bg)" stroke-width="6" />
    <circle
      cx="40" cy="40" r="34" fill="none"
      stroke={color} stroke-width="6"
      stroke-dasharray={circumference}
      stroke-dashoffset={offset}
      stroke-linecap="round"
      transform="rotate(-90 40 40)"
    />
    <text x="40" y="40" text-anchor="middle" dominant-baseline="central"
      fill="var(--fg)" font-size="14" font-weight="600">
      {Math.round(pct)}%
    </text>
  </svg>
  {#if label}
    <span class="gauge-label">{label}</span>
  {/if}
</div>

<style>
  .gauge {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
  }
  .gauge-label {
    font-size: 0.75rem;
    color: var(--fg-muted);
  }
</style>
