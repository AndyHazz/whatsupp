<script>
  import { onMount, onDestroy } from 'svelte';

  export let lastEvent = 0;  // unix timestamp of last check/push
  export let interval = 60;  // seconds between events
  export let color = 'var(--cyan)';

  let dotX = 0;
  let dotY = 0;
  let container;
  let raf;

  function getProgress() {
    const now = Date.now() / 1000;
    const elapsed = now - lastEvent;
    if (interval <= 0) return 0;
    return Math.min((elapsed % interval) / interval, 1);
  }

  // Map progress (0-1) to x,y on the card perimeter, starting top-center clockwise
  function positionOnRect(progress, w, h) {
    const perimeter = 2 * w + 2 * h;
    // Start at top center, go clockwise
    let d = ((progress) * perimeter) % perimeter;

    // Top edge: right half (0 to w/2)
    if (d <= w / 2) {
      return { x: w / 2 + d, y: 0 };
    }
    d -= w / 2;

    // Right edge (0 to h)
    if (d <= h) {
      return { x: w, y: d };
    }
    d -= h;

    // Bottom edge (0 to w)
    if (d <= w) {
      return { x: w - d, y: h };
    }
    d -= w;

    // Left edge (0 to h)
    if (d <= h) {
      return { x: 0, y: h - d };
    }
    d -= h;

    // Top edge: left portion (0 to w/2)
    return { x: d, y: 0 };
  }

  function tick() {
    if (!container) {
      raf = requestAnimationFrame(tick);
      return;
    }
    const w = container.offsetWidth;
    const h = container.offsetHeight;
    const progress = getProgress();
    const pos = positionOnRect(progress, w, h);
    dotX = pos.x;
    dotY = pos.y;
    raf = requestAnimationFrame(tick);
  }

  onMount(() => {
    raf = requestAnimationFrame(tick);
  });

  onDestroy(() => {
    if (raf) cancelAnimationFrame(raf);
  });
</script>

<div class="orbit-container" bind:this={container} style="--dot-color: {color};"  >
  <div
    class="orbit-dot"
    style="left: {dotX}px; top: {dotY}px; --dot-color: {color};"
  ></div>
  <slot />
</div>

<style>
  .orbit-container {
    position: relative;
    min-width: 0;
    display: grid;
  }

  .orbit-dot {
    position: absolute;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--dot-color);
    transform: translate(-50%, -50%);
    box-shadow: 0 0 6px 2px var(--dot-color), 0 0 12px 4px color-mix(in srgb, var(--dot-color) 40%, transparent);
    z-index: 2;
    pointer-events: none;
    will-change: left, top;
  }
</style>
