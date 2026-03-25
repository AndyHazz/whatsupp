<script>
  export let variant = 'card'; // 'card' | 'monitor' | 'host' | 'table-row' | 'security'
  export let count = 1;
</script>

{#each Array(count) as _, i}
  {#if variant === 'monitor' || variant === 'card'}
    <!-- Matches Monitor card: title + badge, uptime row, sparkline -->
    <div class="skeleton-card">
      <div class="skel-row">
        <div class="skel skel-text" style="width:50%"></div>
        <div class="skel-row" style="gap:8px">
          <div class="skel skel-text" style="width:48px; height:18px;"></div>
          <div class="skel skel-badge"></div>
        </div>
      </div>
      <div class="skel-row">
        <div class="skel skel-text" style="width:36px; height:12px;"></div>
      </div>
      <div class="skel skel-sparkline"></div>
    </div>
  {:else if variant === 'host'}
    <!-- Matches Host card: name + last-seen, two gauge circles, temp + version -->
    <div class="skeleton-card">
      <div class="skel-row">
        <div class="skel skel-text" style="width:45%"></div>
        <div class="skel skel-text" style="width:60px; height:12px;"></div>
      </div>
      <div class="skel-gauges">
        <div class="skel-gauge-group">
          <div class="skel skel-circle"></div>
          <div class="skel skel-text" style="width:28px; height:10px;"></div>
        </div>
        <div class="skel-gauge-group">
          <div class="skel skel-circle"></div>
          <div class="skel skel-text" style="width:28px; height:10px;"></div>
        </div>
      </div>
      <div class="skel skel-text" style="width:90px; height:12px; align-self:center;"></div>
      <div class="skel skel-text" style="width:40px; height:10px; align-self:center;"></div>
    </div>
  {:else if variant === 'security'}
    <!-- Matches Security card layout -->
    <div class="skeleton-card">
      <div class="skel-row">
        <div class="skel skel-text" style="width:40%"></div>
        <div class="skel skel-badge"></div>
      </div>
      <div class="skel skel-text" style="width:65%; height:12px;"></div>
      <div class="skel skel-text" style="width:50%; height:12px;"></div>
    </div>
  {:else if variant === 'table-row'}
    <tr class="skeleton-row">
      <td><div class="skel skel-text" style="width:70%"></div></td>
      <td><div class="skel skel-text" style="width:60%"></div></td>
      <td><div class="skel skel-badge"></div></td>
      <td><div class="skel skel-text" style="width:40%"></div></td>
      <td><div class="skel skel-text" style="width:50%"></div></td>
    </tr>
  {/if}
{/each}

<style>
  .skel-text {
    height: 14px;
  }

  .skel-badge {
    width: 40px;
    height: 20px;
    border-radius: 10px;
  }

  .skel-sparkline {
    width: 100%;
    height: 36px;
    border-radius: 6px;
  }

  .skel-circle {
    width: 80px;
    height: 80px;
    border-radius: 50%;
  }

  .skeleton-card {
    background: var(--bg-card);
    border-radius: var(--radius);
    border: 1px solid var(--border-subtle);
    padding: 16px;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .skel-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .skel-gauges {
    display: flex;
    justify-content: space-around;
    margin: 4px 0;
  }

  .skel-gauge-group {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
  }

  .skeleton-row td {
    padding: 10px 12px;
    border-bottom: 1px solid var(--border-subtle);
  }
</style>
