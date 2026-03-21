<script>
  export let variant = 'card'; // 'card' | 'table-row' | 'gauge'
  export let count = 1;
</script>

{#each Array(count) as _, i}
  {#if variant === 'card'}
    <div class="skeleton-card">
      <div class="skel-row">
        <div class="skel skel-text" style="width:55%"></div>
        <div class="skel skel-badge"></div>
      </div>
      <div class="skel skel-chart"></div>
      <div class="skel skel-text" style="width:35%"></div>
    </div>
  {:else if variant === 'table-row'}
    <tr class="skeleton-row">
      <td><div class="skel skel-text" style="width:70%"></div></td>
      <td><div class="skel skel-text" style="width:60%"></div></td>
      <td><div class="skel skel-badge"></div></td>
      <td><div class="skel skel-text" style="width:40%"></div></td>
      <td><div class="skel skel-text" style="width:50%"></div></td>
    </tr>
  {:else if variant === 'gauge'}
    <div class="skeleton-gauge">
      <div class="skel skel-circle"></div>
      <div class="skel skel-text" style="width:30px; height:10px;"></div>
    </div>
  {/if}
{/each}

<style>
  .skel {
    background: linear-gradient(90deg, #323543 25%, #3a3d4e 50%, #323543 75%);
    background-size: 200% 100%;
    animation: shimmer 1.5s infinite;
    border-radius: 4px;
  }

  .skel-text {
    height: 14px;
  }

  .skel-badge {
    width: 40px;
    height: 20px;
    border-radius: 10px;
  }

  .skel-chart {
    width: 100%;
    height: 32px;
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
    gap: 12px;
  }

  .skel-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .skeleton-row td {
    padding: 10px 12px;
    border-bottom: 1px solid var(--border-subtle);
  }

  .skeleton-gauge {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
  }
</style>
