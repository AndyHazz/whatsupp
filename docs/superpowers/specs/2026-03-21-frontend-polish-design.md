# Frontend Polish — Readability & Interaction Design

**Date:** 2026-03-21
**Status:** Approved
**Scope:** Visual polish only — no structural or functional changes

## Context

WhatsUpp is a Svelte 5 SPA with a Dracula dark theme. The app is feature-complete (8 pages, 6 components) and deployed. This spec covers the final frontend polish pass focusing on two areas: readability/contrast and interaction feedback.

## 1. Readability — Refined Depth

### Color Changes

| Token | Current | New | Reason |
|-------|---------|-----|--------|
| `--bg` | `#282a36` | `#1e1f29` | Darker page bg for card separation |
| `--bg-card` | `#44475a` | `#282a36` | Current bg becomes card bg — more contrast |
| `--bg-card-hover` | (none) | `#323543` | Distinct hover state for cards |
| `--border-subtle` | (none) | `rgba(248, 248, 242, 0.06)` | Faint card borders |
| `--shadow-card` | (none) | `0 1px 4px rgba(0, 0, 0, 0.3)` | Subtle depth layering |

The Dracula accent colors (green, red, cyan, purple, etc.) remain unchanged.

### Card Treatment

- Cards get `box-shadow: var(--shadow-card)` and `border: 1px solid var(--border-subtle)`
- **DOWN monitors:** left-border accent `border-left: 3px solid var(--red)` on Overview cards
- Metadata labels (type, interval) become uppercase with `letter-spacing: 0.8px` and smaller font
- Latency becomes a hero stat — larger font-size (~20px), bold weight, unit suffix styled smaller

### Status Badge — Pulsing DOWN

Add a subtle pulse animation to DOWN badges:

```css
@keyframes pulse-down {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.6; }
}
.badge.down { animation: pulse-down 2s infinite; }
```

### Sidebar

- Border-right changes from `var(--fg-muted)` to `var(--border-subtle)` — currently too bright
- Logo divider and footer border also use `--border-subtle`
- Sidebar background becomes a mid-tone between new bg and card bg: `#22232e`

### Tables

- Header border uses `--border-subtle` instead of `--fg-muted`
- Row borders slightly stronger: `rgba(248, 248, 242, 0.06)`

### theme.js

Update `textMuted` alias from `dracula.comment` to the CSS variable value (keep `#6272a4` in dracula const but `textMuted` becomes `'#a4aecf'` to match CSS). Add `bgDeep: '#1e1f29'` alias.

## 2. Interaction Polish

### 2.1 Card & Button Hover States

**Cards (Overview, Hosts, Security):**
```css
.card {
  transition: transform 0.15s ease, box-shadow 0.15s ease, border-color 0.15s ease;
}
.card:hover {
  transform: translateY(-1px);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3), 0 0 0 1px rgba(189, 147, 249, 0.1);
  border-color: rgba(189, 147, 249, 0.3);
  background: var(--bg-card-hover);
}
```

**Primary buttons:** Replace `opacity: 0.9` hover with `filter: brightness(1.1)`.

**Secondary buttons:** On hover, shift border to purple AND add faint purple background `rgba(189, 147, 249, 0.08)`.

**Logout button:** Keep existing red hover color, add background `rgba(255, 85, 85, 0.08)`.

### 2.2 Smooth Transitions

Add to `app.css` global scope:
```css
a, button, input, textarea, select {
  transition: color 0.15s ease, background-color 0.15s ease,
              border-color 0.15s ease, box-shadow 0.15s ease,
              opacity 0.15s ease;
}
```

Component-level transitions already exist on some elements (nav-item, time-range buttons). Ensure consistency at 150ms across all interactive elements. The global rule covers the base; component styles can override duration/properties where needed.

### 2.3 Loading Skeletons

Create a new component `Skeleton.svelte` with variants:

- **card** — matches Overview card layout (header line + sparkline area + meta line)
- **table-row** — matches Incidents table row structure
- **gauge** — circular placeholder matching Gauge component dimensions

Skeleton animation:
```css
@keyframes shimmer {
  0% { background-position: 200% 0; }
  100% { background-position: -200% 0; }
}
.skeleton {
  background: linear-gradient(90deg, #323543 25%, #3a3d4e 50%, #323543 75%);
  background-size: 200% 100%;
  animation: shimmer 1.5s infinite;
  border-radius: 4px;
}
```

Replace `"Loading..."` text in: Overview, Hosts, MonitorDetail, Incidents, Security, Settings.

Each page shows 4-6 skeleton cards (or 5 skeleton table rows) matching its own layout.

### 2.4 Focus & Accessibility

Add to `app.css`:
```css
:focus-visible {
  outline: 2px solid var(--purple);
  outline-offset: 2px;
}
```

Remove `outline: none` from input/textarea focus styles (keep `border-color: var(--purple)` but let the outline show for keyboard users).

### 2.5 Table Row Hover

Add to Incidents and MonitorDetail tables:
```css
tbody tr {
  transition: background-color 0.15s ease;
}
tbody tr:hover {
  background-color: rgba(248, 248, 242, 0.04);
}
```

### 2.6 Gauge Animation

Add CSS transition to the progress circle in `Gauge.svelte`:
```css
circle.progress {
  transition: stroke-dashoffset 0.6s ease-out;
}
```

Initial mount: start with `stroke-dashoffset` at full circumference, then set to computed value after mount (triggers the animation). Use Svelte's `onMount` to flip a `mounted` flag.

## Files Changed

| File | Changes |
|------|---------|
| `app.css` | New CSS variables, global transitions, focus-visible, shimmer keyframes, pulse-down keyframes |
| `theme.js` | Update semantic aliases, add bgDeep |
| `Layout.svelte` | Sidebar border/bg colors, nav hover improvements |
| `Overview.svelte` | Card styles (depth, hover, hero stats, DOWN border), loading skeletons |
| `Hosts.svelte` | Card styles, loading skeletons |
| `HostDetail.svelte` | Card styles |
| `MonitorDetail.svelte` | Card styles, table row hover, loading skeleton |
| `Incidents.svelte` | Table row hover, loading skeleton |
| `Security.svelte` | Card styles, loading skeleton |
| `Settings.svelte` | Button hover styles, loading skeleton |
| `Login.svelte` | Button hover style |
| `StatusBadge.svelte` | Pulse animation on DOWN |
| `Gauge.svelte` | Arc animation on mount/update |
| `TimeRangeSelector.svelte` | Transition consistency |
| `Skeleton.svelte` | **New file** — skeleton loading component |

## Out of Scope

- No new features or pages
- No responsive breakpoint changes
- No chart (uPlot) styling changes — uPlot has its own rendering
- No font changes — system font stack stays
- No JS logic changes beyond skeleton mount flags and gauge animation trigger
