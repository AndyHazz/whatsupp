<script>
  export let status; // 'up' | 'down' | 'unknown'
</script>

<span class="badge" class:up={status === 'up'} class:down={status === 'down'} class:unknown={status !== 'up' && status !== 'down'}>
  {status?.toUpperCase() || 'UNKNOWN'}
</span>

<style>
  .badge {
    display: inline-block;
    padding: 2px 10px;
    border-radius: 12px;
    font-size: 0.75rem;
    font-weight: 700;
    letter-spacing: 0.5px;
  }
  .up      { background: rgba(80, 250, 123, 0.15); color: var(--green); }
  .down    { background: rgba(255, 85, 85, 0.15); color: var(--red); animation: pulse-down 2s infinite; }
  .unknown { background: rgba(98, 114, 164, 0.15); color: var(--fg-muted); }
</style>
