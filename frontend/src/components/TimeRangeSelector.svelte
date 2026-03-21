<script>
  import { timeRanges } from '../lib/tiers.js';
  export let selected = 86400; // default 24h

  function select(value) {
    selected = value;
  }
</script>

<div class="range-selector">
  {#each timeRanges as range}
    <button
      class:active={selected === range.value}
      on:click={() => select(range.value)}
    >
      {range.label}
    </button>
  {/each}
</div>

<style>
  .range-selector {
    display: flex;
    gap: 4px;
    flex-wrap: wrap;
  }
  button {
    background: var(--bg-card);
    border: 1px solid var(--fg-muted);
    color: var(--fg-muted);
    padding: 4px 10px;
    border-radius: var(--radius);
    font-size: 0.8rem;
    transition: all 0.15s;
  }
  button:hover {
    border-color: var(--purple);
    color: var(--fg);
  }
  button.active {
    background: var(--purple);
    border-color: var(--purple);
    color: var(--bg);
    font-weight: 600;
  }
</style>
