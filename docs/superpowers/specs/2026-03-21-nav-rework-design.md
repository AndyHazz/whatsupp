# Navigation Rework + Host Detail Layout

**Date:** 2026-03-21
**Status:** Approved
**Scope:** New Overview dashboard, Monitors page (renamed from Overview), Host Detail layout rework

## Context

The current Overview page shows uptime monitor cards. This is being split into two pages: a real dashboard Overview (at-a-glance status) and a dedicated Monitors page (the uptime cards). The Host Detail page's gauge placement is also being reworked for better space usage.

## 1. New Overview Dashboard

**Route:** `/`

### Summary Tiles

Four clickable tiles in a responsive grid (`repeat(auto-fill, minmax(240px, 1fr))`). Each tile is a card linking to its page.

**Monitors tile** — links to `/monitors`
- Shows count: "7 up" (green) / "0 down" (red)
- If any down, the card gets a subtle red left-border accent (same pattern as monitor cards)
- Data: `api.getMonitors()` → count by status

**Hosts tile** — links to `/hosts`
- Shows count: "3 hosts reporting"
- Shows quick summary: highest CPU/RAM from latest metrics
- Data: `api.getHosts()` → count, plus latest metrics from WebSocket store

**Security tile** — links to `/security`
- Shows: "All clear" (green) or "N new ports detected" (orange/red)
- Data: `api.getScans()` + `api.getBaselines()` → diff new ports across all targets

**Incidents tile** — links to `/incidents`
- Shows: "No active incidents" (green) or "N ongoing" (red)
- Data: `api.getIncidents()` → filter where `resolved_at` is null

### Recent Activity Feed

Below the tiles, a card titled "Recent Activity" showing the last ~10 events in reverse-chronological order.

Each event is a single line: `timestamp — icon — description`

Events come from existing WebSocket dispatches:
- `check_result` → "oracle-http checked — UP 42ms" or "plexypi-ping checked — DOWN"
- `incident` → "Incident started: plexypi-ping" or "Incident resolved: plexypi-ping (2m 30s)"
- `agent_metric` → not shown (too noisy)

The feed subscribes to `onMessage('check_result', ...)` and `onMessage('incident', ...)` and prepends new events. On mount, it shows "Waiting for events..." until the first WebSocket message arrives. No API call needed — it's purely live.

### Styling

- Tiles use the same card depth as other pages (`--border-subtle`, `--shadow-card`, hover lift)
- Status colors: green for all-good, red/orange for issues
- Activity feed uses a simple list layout with muted timestamps and colored status indicators

## 2. Monitors Page

**Route:** `/monitors`

The current `Overview.svelte` content (uptime cards grid with sparklines, hero stats, DOWN accents) moves here with zero functional changes. The component is renamed from `Overview.svelte` to `Monitors.svelte`.

## 3. Navigation Update

Sidebar nav items in `Layout.svelte`:

```js
const navItems = [
  { path: '/',          label: 'Overview',   icon: '&#9673;' },
  { path: '/monitors',  label: 'Monitors',   icon: '&#9672;' },
  { path: '/hosts',     label: 'Hosts',      icon: '&#9881;' },
  { path: '/security',  label: 'Security',   icon: '&#9888;' },
  { path: '/incidents', label: 'Incidents',   icon: '&#9889;' },
  { path: '/settings',  label: 'Settings',   icon: '&#9881;' },
];
```

## 4. Routing Update

In `App.svelte`, add the `/monitors` route and point `/` to the new Overview dashboard:

```svelte
{#if currentPath === '/'}
  <Overview />
{:else if currentPath === '/monitors'}
  <Monitors />
{:else if monitorMatch}
  <MonitorDetail name={monitorMatch.name} />
...
```

The fallback `{:else}` at the bottom also routes to `<Overview />`.

## 5. Host Detail Layout Rework

### Header

Current:
```
[ hostname        ] [ time range buttons ]
[ last seen       ]
```

New:
```
[ hostname        ] [ CPU 78% ] [ RAM 53% ] [ Disk 49% ]
[ last seen       ]
```

- Gauges move from the full-width card into the header, aligned right
- Gauge size shrinks from 80px to 56px for compact fit
- Memory/disk detail text (`2.1 GB / 4.0 GB`) shown below each gauge in smaller text
- The `gauges-row` card is removed entirely

### Time Range Bar

New slim bar between header and charts:

```
[ ─────────────────── 1h  6h  24h  48h  7d  30d  90d  1y ]
```

- Full width, card background with subtle border
- Time range buttons right-aligned
- Compact padding (8px vertical)

### Charts Grid

Unchanged — stays `repeat(auto-fill, minmax(340px, 1fr))` (just fixed in the responsive bug commit).

### Mobile (< 768px)

- Gauges wrap below hostname instead of staying right-aligned
- Time range bar stacks normally

## Files Changed

| File | Change |
|------|--------|
| `pages/Overview.svelte` | **Rewrite** — new dashboard with summary tiles + activity feed |
| `pages/Monitors.svelte` | **New** — current Overview content (rename + move) |
| `pages/HostDetail.svelte` | Header rework (gauges right, slim time-range bar) |
| `components/Layout.svelte` | Add Monitors nav item |
| `App.svelte` | Add `/monitors` route, import Monitors component |

## Out of Scope

- No backend changes
- No new API endpoints
- No changes to Hosts, Security, Incidents, Settings, Login pages
- No changes to MonitorDetail page
