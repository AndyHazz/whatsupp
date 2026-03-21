# Frontend Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Polish WhatsUpp's frontend for better readability (deeper color depth, card separation) and richer interaction feedback (hover states, transitions, skeletons, gauge animation).

**Architecture:** Pure CSS variable changes at `:root` drive the color shift across every component without touching individual files for basic color updates. Interaction polish is layered on per-component. One new `Skeleton.svelte` component provides shimmer placeholders. No JS logic changes beyond mount animation flags.

**Tech Stack:** Svelte (using `export let`, `$:`, `on:click` patterns), CSS custom properties, SVG (gauges), uPlot (charts — untouched).

**Spec:** `docs/superpowers/specs/2026-03-21-frontend-polish-design.md`

---

## File Map

| File | Action | Responsibility |
|------|--------|----------------|
| `frontend/src/app.css` | Modify | CSS variables, global transitions, keyframes, focus-visible |
| `frontend/src/lib/theme.js` | Modify | Semantic color aliases |
| `frontend/src/components/Skeleton.svelte` | **Create** | Shimmer loading placeholder component |
| `frontend/src/components/Layout.svelte` | Modify | Sidebar colors, borders, nav hover, logout hover |
| `frontend/src/components/StatusBadge.svelte` | Modify | Pulse animation on DOWN |
| `frontend/src/components/Gauge.svelte` | Modify | Arc animation on mount/update |
| `frontend/src/components/TimeRangeSelector.svelte` | Modify | Transition consistency |
| `frontend/src/pages/Overview.svelte` | Modify | Card depth, hover, hero stats, DOWN border, skeleton |
| `frontend/src/pages/Hosts.svelte` | Modify | Card depth, hover, skeleton |
| `frontend/src/pages/HostDetail.svelte` | Modify | Card depth, skeleton |
| `frontend/src/pages/MonitorDetail.svelte` | Modify | Card depth, table hover, table borders, skeleton |
| `frontend/src/pages/Incidents.svelte` | Modify | Table hover, table borders, skeleton |
| `frontend/src/pages/Security.svelte` | Modify | Card depth, skeleton |
| `frontend/src/pages/Settings.svelte` | Modify | Button hover styles, skeleton |
| `frontend/src/pages/Login.svelte` | Modify | Button hover style |

---

### Task 1: CSS Variables & Global Styles

The foundation — every subsequent task depends on these variables existing.

**Files:**
- Modify: `frontend/src/app.css`
- Modify: `frontend/src/lib/theme.js`

- [ ] **Step 1: Update CSS variables in app.css**

In `:root`, change existing and add new variables:

```css
:root {
  --bg: #1e1f29;
  --bg-card: #282a36;
  --bg-card-hover: #323543;
  --fg: #f8f8f2;
  --fg-muted: #a4aecf;
  --green: #50fa7b;
  --red: #ff5555;
  --orange: #ffb86c;
  --cyan: #8be9fd;
  --purple: #bd93f9;
  --pink: #ff79c6;
  --yellow: #f1fa8c;

  --radius: 8px;
  --gap: 16px;
  --sidebar-width: 220px;
  --border-subtle: rgba(248, 248, 242, 0.06);
  --shadow-card: 0 1px 4px rgba(0, 0, 0, 0.3);

  /* ... font-family and color/background stay as-is ... */
}
```

- [ ] **Step 2: Add global transitions to interactive elements**

After the `button { ... }` rule in app.css, add:

```css
a, button, input, textarea, select {
  transition: color 0.15s ease, background-color 0.15s ease,
              border-color 0.15s ease, box-shadow 0.15s ease,
              opacity 0.15s ease;
}
```

- [ ] **Step 3: Add focus-visible rule**

After the new transition rule:

```css
:focus-visible {
  outline: 2px solid var(--purple);
  outline-offset: 2px;
}
```

And change the existing `input:focus, textarea:focus, select:focus` rule — remove `outline: none`, keep border-color:

```css
input:focus, textarea:focus, select:focus {
  border-color: var(--purple);
}
```

- [ ] **Step 4: Add keyframe animations**

At the end of app.css:

```css
@keyframes shimmer {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}

@keyframes pulse-down {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.6; }
}
```

- [ ] **Step 5: Update theme.js**

```js
export const theme = {
  bg:         '#1e1f29',
  bgCard:     '#282a36',
  bgDeep:     '#1e1f29',
  text:       dracula.fg,
  textMuted:  '#a4aecf',
  success:    dracula.green,
  error:      dracula.red,
  warning:    dracula.orange,
  info:       dracula.cyan,
  accent:     dracula.purple,
  accentAlt:  dracula.pink,
};
```

- [ ] **Step 6: Verify dev server renders with new colors**

Run: `cd frontend && npm run dev` — open in browser, confirm darker background is visible, cards are lighter, scrollbar/inputs still look correct.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/app.css frontend/src/lib/theme.js
git commit -m "style: deeper background, new CSS variables, global transitions, focus-visible"
```

---

### Task 2: Skeleton Component

Create the reusable loading placeholder before any page needs it.

**Files:**
- Create: `frontend/src/components/Skeleton.svelte`

- [ ] **Step 1: Create Skeleton.svelte**

```svelte
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
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/components/Skeleton.svelte
git commit -m "feat: add Skeleton shimmer loading component"
```

---

### Task 3: Layout & Sidebar Polish

**Files:**
- Modify: `frontend/src/components/Layout.svelte`

- [ ] **Step 1: Update sidebar background and borders**

Change `.sidebar` background to `#22232e` and all border references from `var(--fg-muted)` to `var(--border-subtle)`:

- `.sidebar` → `background: #22232e;` and `border-right: 1px solid var(--border-subtle);`
- `.logo` → `border-bottom: 1px solid var(--border-subtle);`
- `.sidebar-footer` → `border-top: 1px solid var(--border-subtle);`

- [ ] **Step 2: Improve nav item hover**

Update `.nav-item:hover`:
```css
.nav-item:hover {
  background: rgba(248, 248, 242, 0.08);
  text-decoration: none;
  color: var(--fg);
}
```

Add nav-item transition (it already has `transition: background 0.15s` — add color):
```css
.nav-item {
  /* existing props */
  transition: background 0.15s ease, color 0.15s ease;
}
```

- [ ] **Step 3: Update logout button hover**

```css
.logout-btn:hover {
  border-color: var(--red);
  color: var(--red);
  background: rgba(255, 85, 85, 0.08);
}
```

- [ ] **Step 4: Update mobile topbar and sidebar borders**

In `@media (max-width: 768px)`:
- `.topbar` → `border-bottom: 1px solid var(--border-subtle);`
- `.sidebar` → `border-right: 1px solid var(--border-subtle);`

- [ ] **Step 5: Verify sidebar looks correct**

Check desktop and mobile (resize browser to <768px). Sidebar should be darker than cards, borders should be subtle.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/components/Layout.svelte
git commit -m "style: sidebar depth, subtle borders, improved nav hover"
```

---

### Task 4: StatusBadge Pulse & Gauge Animation

**Files:**
- Modify: `frontend/src/components/StatusBadge.svelte`
- Modify: `frontend/src/components/Gauge.svelte`
- Modify: `frontend/src/components/TimeRangeSelector.svelte`

- [ ] **Step 1: Add pulse animation to StatusBadge**

In StatusBadge.svelte, update the `.down` style:

```css
.down { background: rgba(255, 85, 85, 0.15); color: var(--red); animation: pulse-down 2s infinite; }
```

The `pulse-down` keyframes are already defined globally in app.css (Task 1).

- [ ] **Step 2: Add gauge arc animation**

In Gauge.svelte, add a `mounted` flag and import `onMount`:

```js
import { onMount } from 'svelte';
let mounted = false;
onMount(() => { mounted = true; });
```

Add a reactive `displayOffset` that starts at full circumference (empty arc) then transitions to the real value:

```js
$: displayOffset = mounted ? offset : circumference;
```

Update the second `<circle>` (the colored arc) — add `class="progress"` and use `displayOffset` instead of `offset`:

```svelte
<circle
  class="progress"
  cx="40" cy="40" r="34" fill="none"
  stroke={color} stroke-width="6"
  stroke-dasharray={circumference}
  stroke-dashoffset={displayOffset}
  stroke-linecap="round"
  transform="rotate(-90 40 40)"
/>
```

Add to `<style>`:
```css
.progress {
  transition: stroke-dashoffset 0.6s ease-out;
}
```

- [ ] **Step 3: Ensure TimeRangeSelector has consistent transitions**

The existing TimeRangeSelector already has `transition: all 0.15s;` on buttons — this is fine. No change needed, but verify it matches the 150ms timing.

- [ ] **Step 4: Verify in browser**

- DOWN status badges should pulse
- Gauge arcs should animate from 0 to their value on page load
- Time range buttons should transition smoothly

- [ ] **Step 5: Commit**

```bash
git add frontend/src/components/StatusBadge.svelte frontend/src/components/Gauge.svelte
git commit -m "style: pulsing DOWN badge, animated gauge arcs"
```

---

### Task 5: Overview Page Polish

**Files:**
- Modify: `frontend/src/pages/Overview.svelte`

- [ ] **Step 1: Add Skeleton import and loading state**

In `<script>`, add:
```js
import Skeleton from '../components/Skeleton.svelte';
```

Replace the loading block:
```svelte
{#if loading}
  <div class="grid">
    <Skeleton variant="card" count={6} />
  </div>
```

Note: Skeleton's `count` prop renders multiple cards, but each is its own `<div>` — the grid will auto-flow them.

- [ ] **Step 2: Update card styles for depth**

Replace the `.card` styles:

```css
.card {
  background: var(--bg-card);
  border-radius: var(--radius);
  padding: 16px;
  text-decoration: none;
  color: var(--fg);
  border: 1px solid var(--border-subtle);
  box-shadow: var(--shadow-card);
  transition: transform 0.15s ease, box-shadow 0.15s ease, border-color 0.15s ease, background 0.15s ease;
}
.card:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3), 0 0 0 1px rgba(189, 147, 249, 0.1);
  border-color: rgba(189, 147, 249, 0.3);
  background: var(--bg-card-hover);
  text-decoration: none;
}
```

- [ ] **Step 3: Add DOWN monitor left-border accent**

Add a new class for down status. In the template, add `class:down={m.status === 'down'}` to the card `<a>`:

```svelte
<a href="/monitors/{encodeURIComponent(m.name)}" use:link class="card" class:down={m.status === 'down'}>
```

Add CSS:
```css
.card.down {
  border-left: 3px solid var(--red);
}
```

- [ ] **Step 4: Hero stat for latency**

Update the `.latency` style to make it a hero number:

```css
.latency {
  color: var(--cyan);
  font-weight: 700;
  font-size: 1.25rem;
  letter-spacing: -0.5px;
}
```

Wrap the unit suffix in a `<span>` in the template:

```svelte
{#if m.latency_ms != null}
  <span class="latency">{Math.round(m.latency_ms)}<span class="unit">ms</span></span>
{/if}
```

Add unit style:
```css
.unit {
  font-size: 0.7rem;
  font-weight: 400;
  opacity: 0.7;
  margin-left: 1px;
}
```

- [ ] **Step 5: Uppercase metadata**

Update `.card-footer` style:

```css
.card-footer {
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.8px;
}
```

- [ ] **Step 6: Verify Overview page**

- Cards should float above the darker background
- DOWN monitors should have a red left border and pulsing badge
- Latency number should be large with smaller "ms" suffix
- Hover should lift cards with purple glow
- Loading should show skeleton cards

- [ ] **Step 7: Commit**

```bash
git add frontend/src/pages/Overview.svelte
git commit -m "style: overview cards with depth, hero stats, DOWN accent, skeletons"
```

---

### Task 6: Hosts & HostDetail Page Polish

**Files:**
- Modify: `frontend/src/pages/Hosts.svelte`
- Modify: `frontend/src/pages/HostDetail.svelte`

- [ ] **Step 1: Update Hosts.svelte card styles and add skeleton**

Add Skeleton import:
```js
import Skeleton from '../components/Skeleton.svelte';
```

Replace loading block:
```svelte
{#if loading}
  <div class="grid">
    <Skeleton variant="card" count={4} />
  </div>
```

Update `.card` and `.card:hover` to match Overview card styles (depth, shadow, hover lift):

```css
.card {
  background: var(--bg-card);
  border-radius: var(--radius);
  padding: 16px;
  text-decoration: none;
  color: var(--fg);
  border: 1px solid var(--border-subtle);
  box-shadow: var(--shadow-card);
  transition: transform 0.15s ease, box-shadow 0.15s ease, border-color 0.15s ease, background 0.15s ease;
}
.card:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3), 0 0 0 1px rgba(189, 147, 249, 0.1);
  border-color: rgba(189, 147, 249, 0.3);
  background: var(--bg-card-hover);
  text-decoration: none;
}
```

- [ ] **Step 2: Update HostDetail.svelte card styles and add skeleton**

Add Skeleton import:
```js
import Skeleton from '../components/Skeleton.svelte';
```

Replace loading block:
```svelte
{#if loading && !host}
  <div class="gauges-row">
    <Skeleton variant="gauge" count={3} />
  </div>
```

Add depth to `.gauges-row` and `.chart-card`:

```css
.gauges-row {
  /* existing props */
  border: 1px solid var(--border-subtle);
  box-shadow: var(--shadow-card);
}

.chart-card {
  /* existing props */
  border: 1px solid var(--border-subtle);
  box-shadow: var(--shadow-card);
}
```

- [ ] **Step 3: Verify both pages**

- Host cards should have depth and hover lift
- HostDetail gauges should animate on load
- Chart cards should have subtle borders and shadows

- [ ] **Step 4: Commit**

```bash
git add frontend/src/pages/Hosts.svelte frontend/src/pages/HostDetail.svelte
git commit -m "style: hosts cards with depth, skeletons, chart card borders"
```

---

### Task 7: MonitorDetail Page Polish

**Files:**
- Modify: `frontend/src/pages/MonitorDetail.svelte`

- [ ] **Step 1: Add skeleton and card depth**

Replace loading block with a skeleton that matches the chart section layout:
```svelte
{#if loading && !monitor}
  <div class="chart-section">
    <div style="display:flex;justify-content:space-between;margin-bottom:12px;">
      <div class="skel" style="width:40%;height:20px;"></div>
    </div>
    <div class="skel" style="width:100%;height:350px;border-radius:var(--radius);"></div>
  </div>
```

Add the `.skel` class to the component's `<style>` block (since MonitorDetail doesn't import Skeleton — the chart layout doesn't match any Skeleton variant):
```css
.skel {
  background: linear-gradient(90deg, #323543 25%, #3a3d4e 50%, #323543 75%);
  background-size: 200% 100%;
  animation: shimmer 1.5s infinite;
  border-radius: 4px;
}
```

Add depth to `.chart-section` and `.incidents-section`:

```css
.chart-section {
  /* existing props */
  border: 1px solid var(--border-subtle);
  box-shadow: var(--shadow-card);
}
.incidents-section {
  /* existing props */
  border: 1px solid var(--border-subtle);
  box-shadow: var(--shadow-card);
}
```

- [ ] **Step 2: Add table row hover and update borders**

Update `th` border:
```css
th {
  /* existing props */
  border-bottom: 1px solid var(--border-subtle);
}
```

Update `td` border:
```css
td {
  /* existing props */
  border-bottom: 1px solid var(--border-subtle);
}
```

Add table row hover:
```css
tbody tr {
  transition: background-color 0.15s ease;
}
tbody tr:hover {
  background-color: rgba(248, 248, 242, 0.04);
}
```

- [ ] **Step 3: Verify MonitorDetail**

- Chart section and incidents table should have depth
- Table rows should highlight on hover
- Loading should show skeleton

- [ ] **Step 4: Commit**

```bash
git add frontend/src/pages/MonitorDetail.svelte
git commit -m "style: monitor detail depth, table hover, skeleton loading"
```

---

### Task 8: Incidents Page Polish

**Files:**
- Modify: `frontend/src/pages/Incidents.svelte`

- [ ] **Step 1: Add skeleton loading**

Add Skeleton import:
```js
import Skeleton from '../components/Skeleton.svelte';
```

Replace loading block:
```svelte
{#if loading}
  <div class="table-wrap">
    <table>
      <thead>
        <tr><th>Started</th><th>Monitor</th><th>Status</th><th>Duration</th><th>Cause</th></tr>
      </thead>
      <tbody>
        <Skeleton variant="table-row" count={5} />
      </tbody>
    </table>
  </div>
```

- [ ] **Step 2: Add depth to table wrapper and update borders**

```css
.table-wrap {
  /* existing props */
  border: 1px solid var(--border-subtle);
  box-shadow: var(--shadow-card);
}
```

Update `th` border:
```css
th {
  /* existing props */
  border-bottom: 1px solid var(--border-subtle);
}
```

Update `td` border:
```css
td {
  /* existing props */
  border-bottom: 1px solid var(--border-subtle);
}
```

Add table row hover:
```css
tbody tr {
  transition: background-color 0.15s ease;
}
tbody tr:hover {
  background-color: rgba(248, 248, 242, 0.04);
}
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/Incidents.svelte
git commit -m "style: incidents table depth, row hover, skeleton loading"
```

---

### Task 9: Security Page Polish

**Files:**
- Modify: `frontend/src/pages/Security.svelte`

- [ ] **Step 1: Add skeleton and card depth**

Add Skeleton import:
```js
import Skeleton from '../components/Skeleton.svelte';
```

Replace loading block:
```svelte
{#if loading}
  <Skeleton variant="card" count={2} />
```

Update `.scan-card`:

```css
.scan-card {
  /* existing props */
  border: 1px solid var(--border-subtle);
  box-shadow: var(--shadow-card);
}
```

- [ ] **Step 2: Update button hover**

Replace `.accept-btn:hover`:
```css
.accept-btn:hover {
  filter: brightness(1.1);
}
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/Security.svelte
git commit -m "style: security cards with depth, button hover, skeleton"
```

---

### Task 10: Settings & Login Page Polish

**Files:**
- Modify: `frontend/src/pages/Settings.svelte`
- Modify: `frontend/src/pages/Login.svelte`

- [ ] **Step 1: Settings — add skeleton and card depth**

Replace loading block with a skeleton matching the config editor layout (no Skeleton import needed — the editor layout doesn't match any Skeleton variant):
```svelte
{#if loading}
  <div class="section">
    <div class="skel" style="width:60%;height:20px;margin-bottom:12px;"></div>
    <div class="skel" style="width:100%;height:400px;border-radius:var(--radius);"></div>
  </div>
```

Add the `.skel` class to the component's `<style>` block:
```css
.skel {
  background: linear-gradient(90deg, #323543 25%, #3a3d4e 50%, #323543 75%);
  background-size: 200% 100%;
  animation: shimmer 1.5s infinite;
  border-radius: 4px;
}
```

Update `.section`:
```css
.section {
  /* existing props */
  border: 1px solid var(--border-subtle);
  box-shadow: var(--shadow-card);
}
```

Update button hovers:
```css
.btn-primary:hover:not(:disabled) { filter: brightness(1.1); }

.btn-secondary:hover:not(:disabled) {
  border-color: var(--purple);
  background: rgba(189, 147, 249, 0.08);
}
```

- [ ] **Step 2: Login — update button hover and card depth**

Add depth to `.login-card`:
```css
.login-card {
  /* existing props */
  border: 1px solid var(--border-subtle);
  box-shadow: var(--shadow-card);
}
```

Update button hover:
```css
button:hover:not(:disabled) {
  filter: brightness(1.1);
}
```

- [ ] **Step 3: Verify both pages**

- Settings sections should have subtle borders
- Primary buttons should brighten on hover (not dim with opacity)
- Secondary buttons should get purple tint on hover
- Login card should float above the darker background

- [ ] **Step 4: Commit**

```bash
git add frontend/src/pages/Settings.svelte frontend/src/pages/Login.svelte
git commit -m "style: settings and login depth, button hover polish"
```

---

### Task 11: Final Build & Verify

**Files:**
- None — this is a verification task

- [ ] **Step 1: Run production build**

```bash
cd frontend && npm run build
```

Expected: Build succeeds with no errors. Output in `dist/`.

- [ ] **Step 2: Visual check all pages**

Open the dev server and visit each page:
1. `/` (Overview) — dark bg, cards pop, hover lifts, DOWN border, hero stats, skeletons
2. `/hosts` — card depth, hover, gauge animation
3. `/hosts/:name` — chart cards have borders, gauges animate
4. `/monitors/:name` — chart section depth, table row hover
5. `/incidents` — table depth, row hover, skeleton
6. `/security` — card depth, button hover
7. `/settings` — section borders, button hovers
8. Login page (sign out first) — card floats, button hover

- [ ] **Step 3: Check mobile layout**

Resize to <768px width. Verify:
- Hamburger menu still works
- Sidebar borders are subtle
- Cards stack properly
- No horizontal overflow

- [ ] **Step 4: Commit production build**

```bash
git add frontend/dist/
git commit -m "build: production build with frontend polish"
```
