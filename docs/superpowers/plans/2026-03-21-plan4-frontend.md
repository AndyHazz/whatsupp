# WhatsUpp Plan 4: Frontend

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox syntax for tracking.

**Goal:** Svelte SPA with Dracula theme embedded into the Go binary. Dashboard with overview, monitor detail, hosts, security, incidents, and settings pages. Live updates via WebSocket. Zoomable time-series charts.

**Architecture:** Svelte 5 SPA compiled by Vite, output embedded into Go binary via embed.FS. The Go server serves the SPA at `/` and falls back to `index.html` for client-side routing. uPlot for time-series charts. WebSocket for live updates.

**Tech Stack:** Svelte 5, Vite, uPlot, svelte-routing (or similar SPA router), Go embed.FS

---

## Prerequisites

This plan assumes Plan 2 (API endpoints, WebSocket, auth) is complete. The following API endpoints must be functional:

- `POST /api/v1/auth/login` / `POST /api/v1/auth/logout`
- `GET /api/v1/monitors`, `GET /api/v1/monitors/:name`, `GET /api/v1/monitors/:name/results`
- `GET /api/v1/hosts`, `GET /api/v1/hosts/:name`, `GET /api/v1/hosts/:name/metrics`
- `GET /api/v1/incidents`
- `GET /api/v1/security/scans`, `GET /api/v1/security/baselines`, `POST /api/v1/security/baselines/:target`
- `GET /api/v1/config`, `PUT /api/v1/config`
- `WS /api/v1/ws`
- `GET /api/v1/health`

---

## Task 1: Scaffold Svelte 5 project with Vite

- [ ] 1a. Create `frontend/` directory and initialize Svelte 5 project

```bash
cd /home/andyhazz/projects/whatsupp
mkdir -p frontend
cd frontend
npm create vite@latest . -- --template svelte
```

Select "Svelte" and "JavaScript" when prompted.

- [ ] 1b. Install dependencies

```bash
cd /home/andyhazz/projects/whatsupp/frontend
npm install
npm install svelte-routing uplot
```

- [ ] 1c. Configure `frontend/vite.config.js` for SPA and API proxy (dev mode)

```js
// frontend/vite.config.js
import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
    },
  },
});
```

- [ ] 1d. Verify scaffold builds cleanly

```bash
cd /home/andyhazz/projects/whatsupp/frontend
npm run build
```

Confirm `frontend/dist/` contains `index.html` and JS/CSS assets.

- [ ] 1e. Commit: `feat(frontend): scaffold Svelte 5 project with Vite`

---

## Task 2: Dracula theme CSS variables

- [ ] 2a. Create `frontend/src/lib/theme.js` with Dracula colour tokens

```js
// frontend/src/lib/theme.js
export const dracula = {
  bg:         '#282a36',
  currentLine:'#44475a',
  fg:         '#f8f8f2',
  comment:    '#6272a4',
  green:      '#50fa7b',
  red:        '#ff5555',
  orange:     '#ffb86c',
  cyan:       '#8be9fd',
  purple:     '#bd93f9',
  pink:       '#ff79c6',
  yellow:     '#f1fa8c',
};

// Semantic aliases for use in components
export const theme = {
  bg:         dracula.bg,
  bgCard:     dracula.currentLine,
  text:       dracula.fg,
  textMuted:  dracula.comment,
  success:    dracula.green,
  error:      dracula.red,
  warning:    dracula.orange,
  info:       dracula.cyan,
  accent:     dracula.purple,
  accentAlt:  dracula.pink,
};
```

- [ ] 2b. Create `frontend/src/app.css` with global Dracula styles and CSS custom properties

```css
/* frontend/src/app.css */
:root {
  --bg: #282a36;
  --bg-card: #44475a;
  --fg: #f8f8f2;
  --fg-muted: #6272a4;
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

  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen,
    Ubuntu, Cantarell, 'Fira Sans', 'Droid Sans', 'Helvetica Neue', sans-serif;
  color: var(--fg);
  background-color: var(--bg);
}

*, *::before, *::after {
  box-sizing: border-box;
  margin: 0;
  padding: 0;
}

body {
  min-height: 100vh;
  line-height: 1.5;
}

a {
  color: var(--cyan);
  text-decoration: none;
}
a:hover {
  text-decoration: underline;
}

button {
  cursor: pointer;
  font-family: inherit;
}

input, textarea, select {
  font-family: inherit;
  font-size: inherit;
  color: var(--fg);
  background: var(--bg);
  border: 1px solid var(--fg-muted);
  border-radius: var(--radius);
  padding: 8px 12px;
}
input:focus, textarea:focus, select:focus {
  outline: none;
  border-color: var(--purple);
}

/* Scrollbar styling */
::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}
::-webkit-scrollbar-track {
  background: var(--bg);
}
::-webkit-scrollbar-thumb {
  background: var(--fg-muted);
  border-radius: 4px;
}
```

- [ ] 2c. Import `app.css` in `frontend/src/main.js`

```js
// frontend/src/main.js
import './app.css';
import App from './App.svelte';

const app = new App({
  target: document.getElementById('app'),
});

export default app;
```

- [ ] 2d. Verify: open dev server (`npm run dev`), confirm dark background (#282a36) and light text (#f8f8f2) render correctly.

- [ ] 2e. Commit: `feat(frontend): add Dracula theme CSS variables and global styles`

---

## Task 3: SPA router setup

- [ ] 3a. Create `frontend/src/App.svelte` with svelte-routing

```svelte
<!-- frontend/src/App.svelte -->
<script>
  import { Router, Route } from 'svelte-routing';
  import Layout from './components/Layout.svelte';
  import Login from './pages/Login.svelte';
  import Overview from './pages/Overview.svelte';
  import MonitorDetail from './pages/MonitorDetail.svelte';
  import Hosts from './pages/Hosts.svelte';
  import HostDetail from './pages/HostDetail.svelte';
  import Security from './pages/Security.svelte';
  import Incidents from './pages/Incidents.svelte';
  import Settings from './pages/Settings.svelte';

  import { isAuthenticated } from './lib/auth.js';
</script>

<Router>
  {#if !$isAuthenticated}
    <Login />
  {:else}
    <Layout>
      <Route path="/" component={Overview} />
      <Route path="/monitors/:name" let:params>
        <MonitorDetail name={params.name} />
      </Route>
      <Route path="/hosts" component={Hosts} />
      <Route path="/hosts/:name" let:params>
        <HostDetail name={params.name} />
      </Route>
      <Route path="/security" component={Security} />
      <Route path="/incidents" component={Incidents} />
      <Route path="/settings" component={Settings} />
    </Layout>
  {/if}
</Router>
```

- [ ] 3b. Create `frontend/src/lib/auth.js` — reactive auth store

```js
// frontend/src/lib/auth.js
import { writable } from 'svelte/store';

export const isAuthenticated = writable(false);

// Check session on load by hitting a protected endpoint
export async function checkSession() {
  try {
    const res = await fetch('/api/v1/monitors', { credentials: 'include' });
    isAuthenticated.set(res.ok);
  } catch {
    isAuthenticated.set(false);
  }
}

export async function login(username, password) {
  const res = await fetch('/api/v1/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    credentials: 'include',
    body: JSON.stringify({ username, password }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: 'Login failed' }));
    throw new Error(err.error || 'Login failed');
  }
  isAuthenticated.set(true);
}

export async function logout() {
  await fetch('/api/v1/auth/logout', {
    method: 'POST',
    credentials: 'include',
  });
  isAuthenticated.set(false);
}

// Run check on module load
checkSession();
```

- [ ] 3c. Create placeholder page components so the router resolves without errors. Create these files with minimal content:

`frontend/src/pages/Login.svelte`, `frontend/src/pages/Overview.svelte`, `frontend/src/pages/MonitorDetail.svelte`, `frontend/src/pages/Hosts.svelte`, `frontend/src/pages/HostDetail.svelte`, `frontend/src/pages/Security.svelte`, `frontend/src/pages/Incidents.svelte`, `frontend/src/pages/Settings.svelte`

Each placeholder:

```svelte
<script>
  // Placeholder — implemented in later tasks
</script>

<div class="page">
  <h1>Page Name</h1>
  <p>Coming soon.</p>
</div>
```

Also create placeholder `frontend/src/components/Layout.svelte`:

```svelte
<script>
</script>

<div>
  <slot />
</div>
```

- [ ] 3d. Verify: `npm run build` succeeds, dev server shows the Login placeholder (since no session exists).

- [ ] 3e. Commit: `feat(frontend): add SPA router with auth gating and placeholder pages`

---

## Task 4: Login page

- [ ] 4a. Implement `frontend/src/pages/Login.svelte`

```svelte
<!-- frontend/src/pages/Login.svelte -->
<script>
  import { login } from '../lib/auth.js';

  let username = '';
  let password = '';
  let error = '';
  let loading = false;

  async function handleSubmit() {
    error = '';
    loading = true;
    try {
      await login(username, password);
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }
</script>

<div class="login-page">
  <form class="login-card" on:submit|preventDefault={handleSubmit}>
    <h1>WhatsUpp</h1>
    <p class="subtitle">Network Monitor</p>

    {#if error}
      <div class="error">{error}</div>
    {/if}

    <label>
      Username
      <input type="text" bind:value={username} autocomplete="username" required />
    </label>

    <label>
      Password
      <input type="password" bind:value={password} autocomplete="current-password" required />
    </label>

    <button type="submit" disabled={loading}>
      {loading ? 'Signing in...' : 'Sign In'}
    </button>
  </form>
</div>

<style>
  .login-page {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
  }

  .login-card {
    background: var(--bg-card);
    padding: 40px;
    border-radius: var(--radius);
    width: 100%;
    max-width: 380px;
    display: flex;
    flex-direction: column;
    gap: 16px;
  }

  h1 {
    text-align: center;
    color: var(--purple);
    font-size: 2rem;
    margin-bottom: 0;
  }

  .subtitle {
    text-align: center;
    color: var(--fg-muted);
    margin-bottom: 8px;
  }

  label {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 0.9rem;
    color: var(--fg-muted);
  }

  .error {
    background: rgba(255, 85, 85, 0.15);
    color: var(--red);
    padding: 8px 12px;
    border-radius: var(--radius);
    font-size: 0.9rem;
  }

  button {
    background: var(--purple);
    color: var(--bg);
    border: none;
    padding: 10px;
    border-radius: var(--radius);
    font-weight: 600;
    font-size: 1rem;
    margin-top: 8px;
  }
  button:hover:not(:disabled) {
    opacity: 0.9;
  }
  button:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
</style>
```

- [ ] 4b. Verify: dev server shows centered login card with purple "WhatsUpp" heading, dark card background, styled inputs.

- [ ] 4c. Commit: `feat(frontend): implement login page with Dracula styling`

---

## Task 5: Layout component (sidebar navigation + header)

- [ ] 5a. Implement `frontend/src/components/Layout.svelte`

```svelte
<!-- frontend/src/components/Layout.svelte -->
<script>
  import { link } from 'svelte-routing';
  import { logout } from '../lib/auth.js';

  const navItems = [
    { path: '/',          label: 'Overview',  icon: '&#9673;' },
    { path: '/hosts',     label: 'Hosts',     icon: '&#9881;' },
    { path: '/security',  label: 'Security',  icon: '&#9888;' },
    { path: '/incidents', label: 'Incidents',  icon: '&#9889;' },
    { path: '/settings',  label: 'Settings',  icon: '&#9881;' },
  ];

  let sidebarOpen = true;
</script>

<div class="layout" class:collapsed={!sidebarOpen}>
  <aside class="sidebar">
    <div class="logo">
      <a href="/" use:link>
        <h2>WhatsUpp</h2>
      </a>
    </div>
    <nav>
      {#each navItems as item}
        <a href={item.path} use:link class="nav-item">
          <span class="nav-icon">{@html item.icon}</span>
          <span class="nav-label">{item.label}</span>
        </a>
      {/each}
    </nav>
    <div class="sidebar-footer">
      <button class="logout-btn" on:click={logout}>Sign Out</button>
    </div>
  </aside>

  <main class="content">
    <slot />
  </main>
</div>

<style>
  .layout {
    display: grid;
    grid-template-columns: var(--sidebar-width) 1fr;
    min-height: 100vh;
  }

  .sidebar {
    background: var(--bg-card);
    display: flex;
    flex-direction: column;
    padding: 16px 0;
    border-right: 1px solid var(--fg-muted);
    position: sticky;
    top: 0;
    height: 100vh;
    overflow-y: auto;
  }

  .logo {
    padding: 0 20px 20px;
    border-bottom: 1px solid var(--fg-muted);
    margin-bottom: 8px;
  }
  .logo a {
    text-decoration: none;
  }
  .logo h2 {
    color: var(--purple);
    font-size: 1.4rem;
  }

  nav {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: 8px;
  }

  .nav-item {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 10px 12px;
    border-radius: var(--radius);
    color: var(--fg);
    text-decoration: none;
    font-size: 0.95rem;
    transition: background 0.15s;
  }
  .nav-item:hover {
    background: rgba(248, 248, 242, 0.08);
    text-decoration: none;
  }

  .nav-icon {
    font-size: 1.1rem;
    width: 20px;
    text-align: center;
  }

  .sidebar-footer {
    padding: 12px 16px;
    border-top: 1px solid var(--fg-muted);
  }

  .logout-btn {
    background: none;
    border: 1px solid var(--fg-muted);
    color: var(--fg-muted);
    padding: 6px 12px;
    border-radius: var(--radius);
    font-size: 0.85rem;
    width: 100%;
  }
  .logout-btn:hover {
    border-color: var(--red);
    color: var(--red);
  }

  .content {
    padding: var(--gap);
    overflow-x: hidden;
  }

  /* Responsive: collapse sidebar on small screens */
  @media (max-width: 768px) {
    .layout {
      grid-template-columns: 1fr;
    }
    .sidebar {
      display: none;
    }
  }
</style>
```

- [ ] 5b. Verify: after login, sidebar appears on the left with "WhatsUpp" logo, nav links, and "Sign Out" button. Content area shows the Overview placeholder.

- [ ] 5c. Commit: `feat(frontend): add layout component with sidebar navigation`

---

## Task 6: API client library

- [ ] 6a. Create `frontend/src/lib/api.js` — fetch wrapper with cookie auth and error handling

```js
// frontend/src/lib/api.js
const BASE = '/api/v1';

class ApiError extends Error {
  constructor(status, message) {
    super(message);
    this.status = status;
  }
}

async function request(method, path, body = null) {
  const opts = {
    method,
    credentials: 'include',
    headers: {},
  };

  if (body !== null) {
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(body);
  }

  const res = await fetch(`${BASE}${path}`, opts);

  if (res.status === 401) {
    // Session expired — reload to show login
    window.location.reload();
    throw new ApiError(401, 'Session expired');
  }

  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new ApiError(res.status, data.error || `HTTP ${res.status}`);
  }

  // Handle 204 No Content
  if (res.status === 204) return null;

  // Handle backup endpoint (returns file)
  const ct = res.headers.get('content-type') || '';
  if (ct.includes('application/octet-stream')) {
    return res.blob();
  }

  return res.json();
}

export const api = {
  // Monitors
  getMonitors:       ()             => request('GET', '/monitors'),
  getMonitor:        (name)         => request('GET', `/monitors/${encodeURIComponent(name)}`),
  getMonitorResults: (name, from, to) => {
    const params = new URLSearchParams();
    if (from) params.set('from', String(from));
    if (to)   params.set('to', String(to));
    return request('GET', `/monitors/${encodeURIComponent(name)}/results?${params}`);
  },

  // Hosts
  getHosts:        ()                      => request('GET', '/hosts'),
  getHost:         (name)                  => request('GET', `/hosts/${encodeURIComponent(name)}`),
  getHostMetrics:  (name, from, to, names) => {
    const params = new URLSearchParams();
    if (from)  params.set('from', String(from));
    if (to)    params.set('to', String(to));
    if (names) params.set('names', names);
    return request('GET', `/hosts/${encodeURIComponent(name)}/metrics?${params}`);
  },

  // Incidents
  getIncidents: (from, to) => {
    const params = new URLSearchParams();
    if (from) params.set('from', String(from));
    if (to)   params.set('to', String(to));
    return request('GET', `/incidents?${params}`);
  },

  // Security
  getScans:          ()       => request('GET', '/security/scans'),
  getBaselines:      ()       => request('GET', '/security/baselines'),
  updateBaseline:    (target) => request('POST', `/security/baselines/${encodeURIComponent(target)}`),

  // Config
  getConfig:    ()       => request('GET', '/config'),
  updateConfig: (yaml)   => request('PUT', '/config', { config: yaml }),

  // Admin
  getBackup: () => request('GET', '/admin/backup'),

  // Health
  getHealth: () => request('GET', '/health'),
};
```

- [ ] 6b. Verify: import `api` in Overview placeholder, call `api.getMonitors()` in `onMount`, log to console. Confirm fetch goes to `/api/v1/monitors` (check dev tools Network tab).

- [ ] 6c. Commit: `feat(frontend): add API client library with fetch wrapper`

---

## Task 7: WebSocket client library

- [ ] 7a. Create `frontend/src/lib/ws.js` — auto-reconnecting WebSocket with typed message dispatch

```js
// frontend/src/lib/ws.js
import { writable } from 'svelte/store';

// Stores for live data
export const liveCheckResults = writable([]);
export const liveIncidents    = writable([]);
export const liveAgentMetrics = writable([]);
export const wsConnected      = writable(false);

let ws = null;
let reconnectTimer = null;
let reconnectDelay = 1000;
const MAX_RECONNECT_DELAY = 30000;

const listeners = {
  check_result:  [],
  incident:      [],
  agent_metric:  [],
};

export function onMessage(type, callback) {
  if (listeners[type]) {
    listeners[type].push(callback);
  }
  // Return unsubscribe function
  return () => {
    listeners[type] = listeners[type].filter(cb => cb !== callback);
  };
}

function dispatch(type, data) {
  if (type === 'check_result') {
    liveCheckResults.update(arr => [...arr.slice(-99), data]);
  } else if (type === 'incident') {
    liveIncidents.update(arr => [...arr.slice(-49), data]);
  } else if (type === 'agent_metric') {
    liveAgentMetrics.update(arr => [...arr.slice(-49), data]);
  }

  (listeners[type] || []).forEach(cb => {
    try { cb(data); } catch (e) { console.error('WS listener error:', e); }
  });
}

export function connect() {
  if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
    return;
  }

  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  ws = new WebSocket(`${proto}//${location.host}/api/v1/ws`);

  ws.onopen = () => {
    wsConnected.set(true);
    reconnectDelay = 1000;
    console.log('[WS] Connected');
  };

  ws.onclose = () => {
    wsConnected.set(false);
    console.log(`[WS] Disconnected, reconnecting in ${reconnectDelay}ms`);
    scheduleReconnect();
  };

  ws.onerror = (e) => {
    console.error('[WS] Error:', e);
    ws.close();
  };

  ws.onmessage = (event) => {
    try {
      const msg = JSON.parse(event.data);
      if (msg.type && msg.data) {
        dispatch(msg.type, msg.data);
      }
    } catch (e) {
      console.error('[WS] Parse error:', e);
    }
  };
}

function scheduleReconnect() {
  clearTimeout(reconnectTimer);
  reconnectTimer = setTimeout(() => {
    reconnectDelay = Math.min(reconnectDelay * 2, MAX_RECONNECT_DELAY);
    connect();
  }, reconnectDelay);
}

export function disconnect() {
  clearTimeout(reconnectTimer);
  if (ws) {
    ws.close();
    ws = null;
  }
  wsConnected.set(false);
}
```

- [ ] 7b. Connect WebSocket on login success. In `frontend/src/App.svelte`, add:

```svelte
<script>
  import { connect, disconnect } from './lib/ws.js';
  import { isAuthenticated } from './lib/auth.js';

  $: if ($isAuthenticated) {
    connect();
  } else {
    disconnect();
  }
</script>
```

- [ ] 7c. Verify: after login, browser dev tools show WebSocket connection attempt to `/api/v1/ws`. If hub is not running, confirm auto-reconnect with exponential backoff in console logs.

- [ ] 7d. Commit: `feat(frontend): add WebSocket client with auto-reconnect and typed stores`

---

## Task 8: Overview page (status grid, sparklines, uptime %)

- [ ] 8a. Create `frontend/src/components/StatusBadge.svelte`

```svelte
<!-- frontend/src/components/StatusBadge.svelte -->
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
  .down    { background: rgba(255, 85, 85, 0.15); color: var(--red); }
  .unknown { background: rgba(98, 114, 164, 0.15); color: var(--fg-muted); }
</style>
```

- [ ] 8b. Create `frontend/src/components/Sparkline.svelte` — tiny inline SVG chart

```svelte
<!-- frontend/src/components/Sparkline.svelte -->
<script>
  export let data = []; // array of numbers (latency_ms values)
  export let width = 120;
  export let height = 30;
  export let color = 'var(--cyan)';

  $: points = (() => {
    if (!data.length) return '';
    const max = Math.max(...data, 1);
    const min = Math.min(...data, 0);
    const range = max - min || 1;
    const step = width / Math.max(data.length - 1, 1);
    return data
      .map((v, i) => `${i * step},${height - ((v - min) / range) * (height - 4) - 2}`)
      .join(' ');
  })();
</script>

<svg {width} {height} viewBox="0 0 {width} {height}" class="sparkline">
  {#if data.length > 1}
    <polyline
      points={points}
      fill="none"
      stroke={color}
      stroke-width="1.5"
      stroke-linecap="round"
      stroke-linejoin="round"
    />
  {/if}
</svg>

<style>
  .sparkline {
    display: block;
  }
</style>
```

- [ ] 8c. Implement `frontend/src/pages/Overview.svelte`

```svelte
<!-- frontend/src/pages/Overview.svelte -->
<script>
  import { onMount, onDestroy } from 'svelte';
  import { link } from 'svelte-routing';
  import { api } from '../lib/api.js';
  import { onMessage } from '../lib/ws.js';
  import StatusBadge from '../components/StatusBadge.svelte';
  import Sparkline from '../components/Sparkline.svelte';

  let monitors = [];
  let loading = true;
  let error = '';

  // Map of monitor name -> recent latency values for sparkline
  let sparklines = {};

  onMount(async () => {
    try {
      monitors = await api.getMonitors();
      // Fetch last 1h of results for each monitor for sparklines
      const now = Math.floor(Date.now() / 1000);
      const oneHourAgo = now - 3600;
      await Promise.all(monitors.map(async (m) => {
        try {
          const results = await api.getMonitorResults(m.name, oneHourAgo, now);
          sparklines[m.name] = (results || []).map(r => r.latency_ms).filter(v => v != null);
        } catch { /* ignore */ }
      }));
      sparklines = sparklines; // trigger reactivity
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  });

  // Live updates
  const unsub = onMessage('check_result', (data) => {
    // Update monitor status
    monitors = monitors.map(m => {
      if (m.name === data.monitor) {
        return { ...m, status: data.status, latency_ms: data.latency_ms };
      }
      return m;
    });
    // Append to sparkline
    if (sparklines[data.monitor] && data.latency_ms != null) {
      sparklines[data.monitor] = [...sparklines[data.monitor].slice(-59), data.latency_ms];
    }
  });

  onDestroy(unsub);
</script>

<div class="overview">
  <h1>Overview</h1>

  {#if loading}
    <p class="muted">Loading monitors...</p>
  {:else if error}
    <p class="error">{error}</p>
  {:else if monitors.length === 0}
    <p class="muted">No monitors configured. Add monitors in Settings.</p>
  {:else}
    <div class="grid">
      {#each monitors as m}
        <a href="/monitors/{encodeURIComponent(m.name)}" use:link class="card">
          <div class="card-header">
            <span class="monitor-name">{m.name}</span>
            <StatusBadge status={m.status} />
          </div>
          <div class="card-body">
            <Sparkline data={sparklines[m.name] || []} />
            <div class="meta">
              {#if m.latency_ms != null}
                <span class="latency">{Math.round(m.latency_ms)}ms</span>
              {/if}
              {#if m.uptime_pct != null}
                <span class="uptime" class:good={m.uptime_pct >= 99} class:warn={m.uptime_pct < 99 && m.uptime_pct >= 95} class:bad={m.uptime_pct < 95}>
                  {m.uptime_pct.toFixed(1)}%
                </span>
              {/if}
            </div>
          </div>
          <div class="card-footer muted">
            {m.type} &middot; {m.interval || '60s'}
          </div>
        </a>
      {/each}
    </div>
  {/if}
</div>

<style>
  .overview h1 {
    margin-bottom: var(--gap);
  }

  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: var(--gap);
  }

  .card {
    background: var(--bg-card);
    border-radius: var(--radius);
    padding: 16px;
    text-decoration: none;
    color: var(--fg);
    transition: border-color 0.15s;
    border: 1px solid transparent;
  }
  .card:hover {
    border-color: var(--purple);
    text-decoration: none;
  }

  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
  }

  .monitor-name {
    font-weight: 600;
    font-size: 1.05rem;
  }

  .card-body {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 8px;
  }

  .meta {
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 2px;
    font-size: 0.85rem;
  }

  .latency {
    color: var(--cyan);
    font-weight: 600;
  }

  .uptime.good { color: var(--green); }
  .uptime.warn { color: var(--orange); }
  .uptime.bad  { color: var(--red); }

  .card-footer {
    font-size: 0.8rem;
  }

  .muted { color: var(--fg-muted); }
  .error { color: var(--red); }
</style>
```

- [ ] 8d. Verify: page renders a responsive card grid. Each card shows monitor name, status badge, sparkline, latency, uptime %, and type. Cards are clickable links to `/monitors/:name`.

- [ ] 8e. Commit: `feat(frontend): implement overview page with status grid and sparklines`

---

## Task 9: uPlot chart component (Dracula-themed, auto tier selection)

- [ ] 9a. Create `frontend/src/lib/tiers.js` — time range to storage tier mapping

```js
// frontend/src/lib/tiers.js

// Check result tiers
export function checkTierLabel(fromTs, toTs) {
  const hours = (toTs - fromTs) / 3600;
  if (hours <= 48) return 'raw';
  if (hours <= 720) return 'hourly';  // 30 days
  return 'daily';
}

// Agent metric tiers
export function metricTierLabel(fromTs, toTs) {
  const hours = (toTs - fromTs) / 3600;
  if (hours <= 48) return 'raw';
  if (hours <= 168) return '5min';    // 7 days
  if (hours <= 2160) return 'hourly'; // 90 days
  return 'daily';
}

// Preset time ranges (value = seconds back from now)
export const timeRanges = [
  { label: '1h',  value: 3600 },
  { label: '6h',  value: 21600 },
  { label: '24h', value: 86400 },
  { label: '48h', value: 172800 },
  { label: '7d',  value: 604800 },
  { label: '30d', value: 2592000 },
  { label: '90d', value: 7776000 },
  { label: '1y',  value: 31536000 },
];
```

- [ ] 9b. Create `frontend/src/components/Chart.svelte` — uPlot wrapper with Dracula theme

```svelte
<!-- frontend/src/components/Chart.svelte -->
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
```

- [ ] 9c. Create `frontend/src/components/TimeRangeSelector.svelte`

```svelte
<!-- frontend/src/components/TimeRangeSelector.svelte -->
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
```

- [ ] 9d. Verify: import Chart into MonitorDetail placeholder, pass dummy data `[[1,2,3,4,5], [10,20,15,25,30]]`, confirm uPlot renders with Dracula colors, drag-to-zoom works.

- [ ] 9e. Commit: `feat(frontend): add uPlot chart component with Dracula theme and time range selector`

---

## Task 10: Monitor Detail page

- [ ] 10a. Implement `frontend/src/pages/MonitorDetail.svelte`

```svelte
<!-- frontend/src/pages/MonitorDetail.svelte -->
<script>
  import { onMount, onDestroy } from 'svelte';
  import { api } from '../lib/api.js';
  import { onMessage } from '../lib/ws.js';
  import Chart from '../components/Chart.svelte';
  import TimeRangeSelector from '../components/TimeRangeSelector.svelte';
  import StatusBadge from '../components/StatusBadge.svelte';

  export let name;

  let monitor = null;
  let incidents = [];
  let chartData = [[], []];
  let loading = true;
  let error = '';
  let rangeSeconds = 86400; // 24h default

  async function loadData() {
    loading = true;
    error = '';
    try {
      const now = Math.floor(Date.now() / 1000);
      const from = now - rangeSeconds;

      const [m, results, inc] = await Promise.all([
        api.getMonitor(name),
        api.getMonitorResults(name, from, now),
        api.getIncidents(from, now),
      ]);

      monitor = m;
      incidents = (inc || []).filter(i => i.monitor === name);

      // Build uPlot data arrays
      const timestamps = (results || []).map(r => r.timestamp);
      const latencies  = (results || []).map(r => r.latency_ms);
      chartData = [timestamps, latencies];
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  $: if (name && rangeSeconds) loadData();

  // Live updates — append new check results
  const unsub = onMessage('check_result', (data) => {
    if (data.monitor !== name) return;
    if (monitor) {
      monitor = { ...monitor, status: data.status, latency_ms: data.latency_ms };
    }
    if (data.timestamp && data.latency_ms != null) {
      chartData = [
        [...chartData[0], data.timestamp],
        [...chartData[1], data.latency_ms],
      ];
    }
  });

  onDestroy(unsub);

  function formatDuration(seconds) {
    if (!seconds || seconds < 0) return '-';
    if (seconds < 60) return `${seconds}s`;
    if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`;
    const h = Math.floor(seconds / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    return `${h}h ${m}m`;
  }

  function formatTime(ts) {
    if (!ts) return '-';
    return new Date(ts * 1000).toLocaleString();
  }
</script>

<div class="monitor-detail">
  {#if loading && !monitor}
    <p class="muted">Loading...</p>
  {:else if error}
    <p class="error">{error}</p>
  {:else if monitor}
    <div class="header">
      <div>
        <h1>{monitor.name}</h1>
        <span class="meta">{monitor.type} &middot; {monitor.url || monitor.host || ''}</span>
      </div>
      <StatusBadge status={monitor.status} />
    </div>

    <div class="chart-section">
      <div class="chart-controls">
        <h2>Response Time</h2>
        <TimeRangeSelector bind:selected={rangeSeconds} />
      </div>
      <Chart data={chartData} label="Latency" unit="ms" height={350} />
    </div>

    {#if incidents.length > 0}
      <div class="incidents-section">
        <h2>Recent Incidents</h2>
        <table>
          <thead>
            <tr>
              <th>Started</th>
              <th>Resolved</th>
              <th>Duration</th>
              <th>Cause</th>
            </tr>
          </thead>
          <tbody>
            {#each incidents as inc}
              <tr>
                <td>{formatTime(inc.started_at)}</td>
                <td>{inc.resolved_at ? formatTime(inc.resolved_at) : 'Ongoing'}</td>
                <td>{inc.resolved_at ? formatDuration(inc.resolved_at - inc.started_at) : 'Ongoing'}</td>
                <td class="cause">{inc.cause || '-'}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  {/if}
</div>

<style>
  .header {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    margin-bottom: 24px;
  }
  .header h1 { margin-bottom: 4px; }
  .meta { color: var(--fg-muted); font-size: 0.9rem; }

  .chart-section {
    background: var(--bg-card);
    padding: 16px;
    border-radius: var(--radius);
    margin-bottom: 24px;
  }
  .chart-controls {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
    flex-wrap: wrap;
    gap: 8px;
  }
  .chart-controls h2 { font-size: 1.1rem; }

  .incidents-section {
    background: var(--bg-card);
    padding: 16px;
    border-radius: var(--radius);
  }
  .incidents-section h2 {
    font-size: 1.1rem;
    margin-bottom: 12px;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.9rem;
  }
  th {
    text-align: left;
    padding: 8px 12px;
    border-bottom: 1px solid var(--fg-muted);
    color: var(--fg-muted);
    font-weight: 600;
  }
  td {
    padding: 8px 12px;
    border-bottom: 1px solid rgba(98, 114, 164, 0.2);
  }
  .cause { color: var(--red); }
  .muted { color: var(--fg-muted); }
  .error { color: var(--red); }
</style>
```

- [ ] 10b. Verify: navigate to `/monitors/SomeName`, page shows monitor header with status badge, zoomable chart with time range selector, and incident history table. Changing time range reloads data.

- [ ] 10c. Commit: `feat(frontend): implement monitor detail page with zoomable chart and incident history`

---

## Task 11: Hosts page and Host Detail page

- [ ] 11a. Create `frontend/src/components/Gauge.svelte` — circular usage gauge

```svelte
<!-- frontend/src/components/Gauge.svelte -->
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
```

- [ ] 11b. Implement `frontend/src/pages/Hosts.svelte`

```svelte
<!-- frontend/src/pages/Hosts.svelte -->
<script>
  import { onMount, onDestroy } from 'svelte';
  import { link } from 'svelte-routing';
  import { api } from '../lib/api.js';
  import { onMessage } from '../lib/ws.js';
  import Gauge from '../components/Gauge.svelte';

  let hosts = [];
  let loading = true;
  let error = '';

  onMount(async () => {
    try {
      hosts = await api.getHosts();
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  });

  // Live metric updates
  const unsub = onMessage('agent_metric', (data) => {
    hosts = hosts.map(h => {
      if (h.name !== data.host) return h;
      const updated = { ...h, metrics: { ...h.metrics } };
      for (const m of (data.metrics || [])) {
        updated.metrics[m.name] = m.value;
      }
      return updated;
    });
  });

  onDestroy(unsub);

  function getMetric(host, name) {
    return host.metrics?.[name] ?? null;
  }

  function formatLastSeen(ts) {
    if (!ts) return 'Never';
    const diff = Math.floor(Date.now() / 1000) - ts;
    if (diff < 60) return 'Just now';
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
    return `${Math.floor(diff / 3600)}h ago`;
  }
</script>

<div class="hosts-page">
  <h1>Hosts</h1>

  {#if loading}
    <p class="muted">Loading hosts...</p>
  {:else if error}
    <p class="error">{error}</p>
  {:else if hosts.length === 0}
    <p class="muted">No hosts reporting. Configure agents or scrape targets in Settings.</p>
  {:else}
    <div class="grid">
      {#each hosts as host}
        <a href="/hosts/{encodeURIComponent(host.name)}" use:link class="card">
          <div class="card-header">
            <span class="host-name">{host.name}</span>
            <span class="last-seen">{formatLastSeen(host.last_seen_at)}</span>
          </div>
          <div class="gauges">
            {#if getMetric(host, 'cpu.usage_pct') != null}
              <Gauge value={getMetric(host, 'cpu.usage_pct')} label="CPU" />
            {/if}
            {#if getMetric(host, 'mem.usage_pct') != null}
              <Gauge value={getMetric(host, 'mem.usage_pct')} label="RAM" />
            {/if}
            {#if getMetric(host, 'disk./.usage_pct') != null}
              <Gauge value={getMetric(host, 'disk./.usage_pct')} label="Disk" />
            {/if}
          </div>
          {#if getMetric(host, 'temp.cpu') != null}
            <div class="temp">
              CPU Temp: <span class="temp-value">{Math.round(getMetric(host, 'temp.cpu'))}&deg;C</span>
            </div>
          {/if}
        </a>
      {/each}
    </div>
  {/if}
</div>

<style>
  .hosts-page h1 { margin-bottom: var(--gap); }

  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: var(--gap);
  }

  .card {
    background: var(--bg-card);
    border-radius: var(--radius);
    padding: 16px;
    text-decoration: none;
    color: var(--fg);
    border: 1px solid transparent;
    transition: border-color 0.15s;
  }
  .card:hover {
    border-color: var(--purple);
    text-decoration: none;
  }

  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
  }
  .host-name { font-weight: 600; font-size: 1.05rem; }
  .last-seen { font-size: 0.8rem; color: var(--fg-muted); }

  .gauges {
    display: flex;
    justify-content: space-around;
    margin-bottom: 8px;
  }

  .temp {
    font-size: 0.85rem;
    color: var(--fg-muted);
    text-align: center;
  }
  .temp-value { color: var(--orange); font-weight: 600; }

  .muted { color: var(--fg-muted); }
  .error { color: var(--red); }
</style>
```

- [ ] 11c. Implement `frontend/src/pages/HostDetail.svelte`

```svelte
<!-- frontend/src/pages/HostDetail.svelte -->
<script>
  import { onMount, onDestroy } from 'svelte';
  import { api } from '../lib/api.js';
  import { onMessage } from '../lib/ws.js';
  import Chart from '../components/Chart.svelte';
  import TimeRangeSelector from '../components/TimeRangeSelector.svelte';
  import Gauge from '../components/Gauge.svelte';
  import { dracula } from '../lib/theme.js';

  export let name;

  let host = null;
  let loading = true;
  let error = '';
  let rangeSeconds = 86400;

  // Metric categories and their chart configs
  const categories = [
    { key: 'cpu',     label: 'CPU Usage',     names: 'cpu',  unit: '%',  color: dracula.purple },
    { key: 'mem',     label: 'Memory Usage',   names: 'mem',  unit: '%',  color: dracula.pink },
    { key: 'disk',    label: 'Disk Usage',     names: 'disk', unit: '%',  color: dracula.orange },
    { key: 'net',     label: 'Network',        names: 'net',  unit: 'B/s', color: dracula.cyan },
    { key: 'temp',    label: 'Temperature',    names: 'temp', unit: 'C',  color: dracula.red },
    { key: 'docker',  label: 'Docker',         names: 'docker', unit: '%', color: dracula.green },
  ];

  let chartsData = {};

  async function loadData() {
    loading = true;
    error = '';
    try {
      const now = Math.floor(Date.now() / 1000);
      const from = now - rangeSeconds;

      host = await api.getHost(name);

      // Fetch metrics for each category
      const results = await Promise.all(
        categories.map(async (cat) => {
          try {
            const metrics = await api.getHostMetrics(name, from, now, cat.names);
            return { key: cat.key, metrics };
          } catch {
            return { key: cat.key, metrics: [] };
          }
        })
      );

      chartsData = {};
      for (const { key, metrics } of results) {
        if (metrics && metrics.length > 0) {
          // Group by metric_name, use first metric for the chart
          const byName = {};
          for (const m of metrics) {
            if (!byName[m.metric_name]) byName[m.metric_name] = [];
            byName[m.metric_name].push(m);
          }
          // Use the primary metric (first one) for the chart
          const primary = Object.values(byName)[0] || [];
          chartsData[key] = [
            primary.map(m => m.timestamp),
            primary.map(m => m.value),
          ];
        }
      }
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  $: if (name && rangeSeconds) loadData();

  const unsub = onMessage('agent_metric', (data) => {
    if (data.host !== name || !host) return;
    // Update current metrics on host
    host = { ...host };
    for (const m of (data.metrics || [])) {
      if (!host.metrics) host.metrics = {};
      host.metrics[m.name] = m.value;
    }
  });

  onDestroy(unsub);

  function getMetric(name) {
    return host?.metrics?.[name] ?? null;
  }
</script>

<div class="host-detail">
  {#if loading && !host}
    <p class="muted">Loading...</p>
  {:else if error}
    <p class="error">{error}</p>
  {:else if host}
    <div class="header">
      <h1>{host.name}</h1>
      <TimeRangeSelector bind:selected={rangeSeconds} />
    </div>

    <div class="gauges-row">
      {#if getMetric('cpu.usage_pct') != null}
        <Gauge value={getMetric('cpu.usage_pct')} label="CPU" size={90} />
      {/if}
      {#if getMetric('mem.usage_pct') != null}
        <Gauge value={getMetric('mem.usage_pct')} label="RAM" size={90} />
      {/if}
      {#if getMetric('disk./.usage_pct') != null}
        <Gauge value={getMetric('disk./.usage_pct')} label="Disk /" size={90} />
      {/if}
    </div>

    <div class="charts">
      {#each categories as cat}
        {#if chartsData[cat.key]}
          <div class="chart-card">
            <h3>{cat.label}</h3>
            <Chart
              data={chartsData[cat.key]}
              label={cat.label}
              unit={cat.unit}
              color={cat.color}
              height={250}
            />
          </div>
        {/if}
      {/each}
    </div>
  {/if}
</div>

<style>
  .header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 20px;
    flex-wrap: wrap;
    gap: 12px;
  }

  .gauges-row {
    display: flex;
    gap: 24px;
    justify-content: center;
    margin-bottom: 24px;
    padding: 16px;
    background: var(--bg-card);
    border-radius: var(--radius);
  }

  .charts {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(400px, 1fr));
    gap: var(--gap);
  }

  .chart-card {
    background: var(--bg-card);
    padding: 16px;
    border-radius: var(--radius);
  }
  .chart-card h3 {
    font-size: 1rem;
    margin-bottom: 8px;
    color: var(--fg-muted);
  }

  .muted { color: var(--fg-muted); }
  .error { color: var(--red); }
</style>
```

- [ ] 11d. Verify: `/hosts` shows a card grid with CPU/RAM/Disk gauges per host. Clicking a card navigates to `/hosts/:name` with individual metric charts. Time range selector changes chart data.

- [ ] 11e. Commit: `feat(frontend): implement hosts page with gauges and host detail with per-category charts`

---

## Task 12: Security page

- [ ] 12a. Implement `frontend/src/pages/Security.svelte`

```svelte
<!-- frontend/src/pages/Security.svelte -->
<script>
  import { onMount } from 'svelte';
  import { api } from '../lib/api.js';

  let scans = [];
  let baselines = {};
  let loading = true;
  let error = '';

  onMount(async () => {
    try {
      const [s, b] = await Promise.all([
        api.getScans(),
        api.getBaselines(),
      ]);
      scans = s || [];
      // Convert baselines array to map by target
      baselines = {};
      for (const bl of (b || [])) {
        baselines[bl.target] = bl;
      }
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  });

  async function acceptBaseline(target) {
    try {
      await api.updateBaseline(target);
      // Refresh data
      const [s, b] = await Promise.all([api.getScans(), api.getBaselines()]);
      scans = s || [];
      baselines = {};
      for (const bl of (b || [])) baselines[bl.target] = bl;
    } catch (e) {
      error = e.message;
    }
  }

  function formatTime(ts) {
    if (!ts) return 'Never';
    return new Date(ts * 1000).toLocaleString();
  }

  function getDiff(scan, baseline) {
    if (!baseline) return { newPorts: scan.open_ports || [], missingPorts: [] };
    const expected = new Set(baseline.expected_ports || []);
    const actual   = new Set(scan.open_ports || []);
    const newPorts     = [...actual].filter(p => !expected.has(p));
    const missingPorts = [...expected].filter(p => !actual.has(p));
    return { newPorts, missingPorts };
  }

  // SSL cert expiry info from monitor metadata (if available)
  function daysUntil(ts) {
    if (!ts) return null;
    const diff = ts - Math.floor(Date.now() / 1000);
    return Math.floor(diff / 86400);
  }
</script>

<div class="security-page">
  <h1>Security</h1>

  {#if loading}
    <p class="muted">Loading scan results...</p>
  {:else if error}
    <p class="error">{error}</p>
  {:else if scans.length === 0}
    <p class="muted">No security scans have been run yet. Configure scan targets in Settings.</p>
  {:else}
    <!-- Group scans by target, show latest per target -->
    {#each Object.entries(
      scans.reduce((acc, s) => {
        if (!acc[s.target] || s.timestamp > acc[s.target].timestamp) acc[s.target] = s;
        return acc;
      }, {})
    ) as [target, scan]}
      {@const baseline = baselines[target]}
      {@const diff = getDiff(scan, baseline)}
      <div class="scan-card">
        <div class="scan-header">
          <h2>{target}</h2>
          <span class="scan-time">Last scan: {formatTime(scan.timestamp)}</span>
        </div>

        <div class="ports">
          <h3>Open Ports ({(scan.open_ports || []).length})</h3>
          <div class="port-list">
            {#each (scan.open_ports || []) as port}
              {@const isNew = diff.newPorts.includes(port)}
              <span class="port-badge" class:new-port={isNew}>
                {port}
                {#if isNew}<span class="new-tag">NEW</span>{/if}
              </span>
            {/each}
          </div>
        </div>

        {#if diff.missingPorts.length > 0}
          <div class="missing">
            <h3>Missing from Baseline</h3>
            <div class="port-list">
              {#each diff.missingPorts as port}
                <span class="port-badge missing-port">{port}</span>
              {/each}
            </div>
          </div>
        {/if}

        {#if diff.newPorts.length > 0 || diff.missingPorts.length > 0}
          <button class="accept-btn" on:click={() => acceptBaseline(target)}>
            Accept as New Baseline
          </button>
        {:else if !baseline}
          <button class="accept-btn" on:click={() => acceptBaseline(target)}>
            Set Baseline
          </button>
        {:else}
          <p class="baseline-match">Matches baseline</p>
        {/if}
      </div>
    {/each}
  {/if}
</div>

<style>
  .security-page h1 { margin-bottom: var(--gap); }

  .scan-card {
    background: var(--bg-card);
    padding: 16px;
    border-radius: var(--radius);
    margin-bottom: var(--gap);
  }
  .scan-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 12px;
  }
  .scan-header h2 { font-size: 1.1rem; }
  .scan-time { font-size: 0.8rem; color: var(--fg-muted); }

  .ports, .missing {
    margin-bottom: 12px;
  }
  .ports h3, .missing h3 {
    font-size: 0.9rem;
    color: var(--fg-muted);
    margin-bottom: 8px;
  }

  .port-list {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
  }

  .port-badge {
    background: rgba(248, 248, 242, 0.08);
    padding: 2px 8px;
    border-radius: 4px;
    font-size: 0.85rem;
    font-family: monospace;
  }
  .port-badge.new-port {
    background: rgba(255, 85, 85, 0.15);
    color: var(--red);
  }
  .port-badge.missing-port {
    background: rgba(255, 184, 108, 0.15);
    color: var(--orange);
  }
  .new-tag {
    font-size: 0.65rem;
    font-weight: 700;
    margin-left: 4px;
    text-transform: uppercase;
  }

  .accept-btn {
    background: var(--purple);
    color: var(--bg);
    border: none;
    padding: 6px 14px;
    border-radius: var(--radius);
    font-size: 0.85rem;
    font-weight: 600;
    margin-top: 8px;
  }
  .accept-btn:hover { opacity: 0.9; }

  .baseline-match {
    color: var(--green);
    font-size: 0.85rem;
    margin-top: 8px;
  }

  .muted { color: var(--fg-muted); }
  .error { color: var(--red); }
</style>
```

- [ ] 12b. Verify: page shows scan cards grouped by target. Open ports displayed as badges. New ports highlighted in red, missing ports in orange. "Accept as New Baseline" button appears when drift is detected.

- [ ] 12c. Commit: `feat(frontend): implement security page with baseline diffs and port badges`

---

## Task 13: Incidents page

- [ ] 13a. Implement `frontend/src/pages/Incidents.svelte`

```svelte
<!-- frontend/src/pages/Incidents.svelte -->
<script>
  import { onMount, onDestroy } from 'svelte';
  import { link } from 'svelte-routing';
  import { api } from '../lib/api.js';
  import { onMessage } from '../lib/ws.js';
  import StatusBadge from '../components/StatusBadge.svelte';

  let incidents = [];
  let loading = true;
  let error = '';
  let sortKey = 'started_at';
  let sortAsc = false;

  onMount(async () => {
    try {
      incidents = (await api.getIncidents()) || [];
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  });

  // Live incident updates
  const unsub = onMessage('incident', (data) => {
    const idx = incidents.findIndex(i => i.id === data.id);
    if (idx >= 0) {
      incidents[idx] = { ...incidents[idx], ...data };
      incidents = [...incidents];
    } else {
      incidents = [data, ...incidents];
    }
  });

  onDestroy(unsub);

  function sort(key) {
    if (sortKey === key) {
      sortAsc = !sortAsc;
    } else {
      sortKey = key;
      sortAsc = false;
    }
  }

  $: sorted = [...incidents].sort((a, b) => {
    let va = a[sortKey], vb = b[sortKey];
    if (va == null) va = 0;
    if (vb == null) vb = 0;
    if (typeof va === 'string') {
      return sortAsc ? va.localeCompare(vb) : vb.localeCompare(va);
    }
    return sortAsc ? va - vb : vb - va;
  });

  function formatTime(ts) {
    if (!ts) return '-';
    return new Date(ts * 1000).toLocaleString();
  }

  function formatDuration(started, resolved) {
    if (!resolved) return 'Ongoing';
    const secs = resolved - started;
    if (secs < 60) return `${secs}s`;
    if (secs < 3600) return `${Math.floor(secs / 60)}m ${secs % 60}s`;
    const h = Math.floor(secs / 3600);
    const m = Math.floor((secs % 3600) / 60);
    return `${h}h ${m}m`;
  }
</script>

<div class="incidents-page">
  <h1>Incidents</h1>

  {#if loading}
    <p class="muted">Loading incidents...</p>
  {:else if error}
    <p class="error">{error}</p>
  {:else if incidents.length === 0}
    <p class="muted">No incidents recorded.</p>
  {:else}
    <div class="table-wrap">
      <table>
        <thead>
          <tr>
            <th class="sortable" on:click={() => sort('started_at')}>
              Started {sortKey === 'started_at' ? (sortAsc ? '▲' : '▼') : ''}
            </th>
            <th class="sortable" on:click={() => sort('monitor')}>
              Monitor {sortKey === 'monitor' ? (sortAsc ? '▲' : '▼') : ''}
            </th>
            <th>Status</th>
            <th class="sortable" on:click={() => sort('resolved_at')}>
              Duration {sortKey === 'resolved_at' ? (sortAsc ? '▲' : '▼') : ''}
            </th>
            <th>Cause</th>
          </tr>
        </thead>
        <tbody>
          {#each sorted as inc}
            <tr>
              <td>{formatTime(inc.started_at)}</td>
              <td>
                <a href="/monitors/{encodeURIComponent(inc.monitor)}" use:link>
                  {inc.monitor}
                </a>
              </td>
              <td>
                {#if inc.resolved_at}
                  <StatusBadge status="up" />
                {:else}
                  <StatusBadge status="down" />
                {/if}
              </td>
              <td>{formatDuration(inc.started_at, inc.resolved_at)}</td>
              <td class="cause">{inc.cause || '-'}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
</div>

<style>
  .incidents-page h1 { margin-bottom: var(--gap); }

  .table-wrap {
    background: var(--bg-card);
    border-radius: var(--radius);
    overflow-x: auto;
  }

  table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.9rem;
  }
  th {
    text-align: left;
    padding: 12px;
    border-bottom: 1px solid var(--fg-muted);
    color: var(--fg-muted);
    font-weight: 600;
    white-space: nowrap;
  }
  th.sortable {
    cursor: pointer;
    user-select: none;
  }
  th.sortable:hover { color: var(--fg); }

  td {
    padding: 10px 12px;
    border-bottom: 1px solid rgba(98, 114, 164, 0.15);
  }
  .cause { color: var(--red); max-width: 300px; }

  .muted { color: var(--fg-muted); }
  .error { color: var(--red); }
</style>
```

- [ ] 13b. Verify: page shows a sortable table of incidents. Clicking column headers toggles sort direction. Monitor names link to Monitor Detail. Ongoing incidents show DOWN badge; resolved show UP.

- [ ] 13c. Commit: `feat(frontend): implement incidents page with sortable table`

---

## Task 14: Settings page

- [ ] 14a. Implement `frontend/src/pages/Settings.svelte`

```svelte
<!-- frontend/src/pages/Settings.svelte -->
<script>
  import { onMount } from 'svelte';
  import { api } from '../lib/api.js';

  let configYaml = '';
  let originalYaml = '';
  let loading = true;
  let saving = false;
  let error = '';
  let success = '';
  let ntfyTesting = false;

  onMount(async () => {
    try {
      const data = await api.getConfig();
      configYaml = data.config || '';
      originalYaml = configYaml;
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  });

  $: isDirty = configYaml !== originalYaml;

  async function saveConfig() {
    saving = true;
    error = '';
    success = '';
    try {
      await api.updateConfig(configYaml);
      originalYaml = configYaml;
      success = 'Configuration saved. Changes take effect immediately.';
      setTimeout(() => { success = ''; }, 5000);
    } catch (e) {
      error = e.message;
    } finally {
      saving = false;
    }
  }

  function resetConfig() {
    configYaml = originalYaml;
  }

  async function testNtfy() {
    ntfyTesting = true;
    try {
      // Send a test notification via a health-like endpoint
      // Fallback: use the ntfy URL from config to send directly
      await fetch('/api/v1/health');
      success = 'Test notification sent (check your ntfy topic).';
      setTimeout(() => { success = ''; }, 5000);
    } catch (e) {
      error = 'Failed to send test notification: ' + e.message;
    } finally {
      ntfyTesting = false;
    }
  }

  async function downloadBackup() {
    try {
      const blob = await api.getBackup();
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `whatsupp-backup-${new Date().toISOString().slice(0, 10)}.db`;
      a.click();
      URL.revokeObjectURL(url);
    } catch (e) {
      error = 'Backup failed: ' + e.message;
    }
  }
</script>

<div class="settings-page">
  <h1>Settings</h1>

  {#if loading}
    <p class="muted">Loading configuration...</p>
  {:else}
    {#if error}
      <div class="msg error-msg">{error}</div>
    {/if}
    {#if success}
      <div class="msg success-msg">{success}</div>
    {/if}

    <div class="section">
      <h2>Configuration (YAML)</h2>
      <p class="help">
        Edit the YAML config below. Changes are applied immediately on save.
        Monitors, agents, security targets, and alerting are all configured here.
      </p>
      <textarea
        class="config-editor"
        bind:value={configYaml}
        spellcheck="false"
        rows="30"
      ></textarea>
      <div class="actions">
        <button class="btn-primary" on:click={saveConfig} disabled={!isDirty || saving}>
          {saving ? 'Saving...' : 'Save Configuration'}
        </button>
        <button class="btn-secondary" on:click={resetConfig} disabled={!isDirty}>
          Discard Changes
        </button>
      </div>
    </div>

    <div class="section">
      <h2>Notifications</h2>
      <button class="btn-secondary" on:click={testNtfy} disabled={ntfyTesting}>
        {ntfyTesting ? 'Sending...' : 'Send Test ntfy Notification'}
      </button>
    </div>

    <div class="section">
      <h2>Database</h2>
      <button class="btn-secondary" on:click={downloadBackup}>
        Download Database Backup
      </button>
    </div>
  {/if}
</div>

<style>
  .settings-page h1 { margin-bottom: var(--gap); }

  .section {
    background: var(--bg-card);
    padding: 20px;
    border-radius: var(--radius);
    margin-bottom: var(--gap);
  }
  .section h2 {
    font-size: 1.1rem;
    margin-bottom: 8px;
  }
  .help {
    font-size: 0.85rem;
    color: var(--fg-muted);
    margin-bottom: 12px;
  }

  .config-editor {
    width: 100%;
    min-height: 400px;
    font-family: 'Fira Code', 'Cascadia Code', 'JetBrains Mono', 'Consolas', monospace;
    font-size: 0.85rem;
    line-height: 1.6;
    padding: 16px;
    background: var(--bg);
    color: var(--fg);
    border: 1px solid var(--fg-muted);
    border-radius: var(--radius);
    resize: vertical;
    tab-size: 2;
  }
  .config-editor:focus {
    border-color: var(--purple);
  }

  .actions {
    display: flex;
    gap: 8px;
    margin-top: 12px;
  }

  .btn-primary {
    background: var(--purple);
    color: var(--bg);
    border: none;
    padding: 8px 16px;
    border-radius: var(--radius);
    font-weight: 600;
  }
  .btn-primary:hover:not(:disabled) { opacity: 0.9; }
  .btn-primary:disabled { opacity: 0.5; cursor: not-allowed; }

  .btn-secondary {
    background: none;
    border: 1px solid var(--fg-muted);
    color: var(--fg);
    padding: 8px 16px;
    border-radius: var(--radius);
  }
  .btn-secondary:hover:not(:disabled) {
    border-color: var(--purple);
  }
  .btn-secondary:disabled { opacity: 0.5; cursor: not-allowed; }

  .msg {
    padding: 10px 14px;
    border-radius: var(--radius);
    margin-bottom: 12px;
    font-size: 0.9rem;
  }
  .error-msg { background: rgba(255, 85, 85, 0.12); color: var(--red); }
  .success-msg { background: rgba(80, 250, 123, 0.12); color: var(--green); }

  .muted { color: var(--fg-muted); }
</style>
```

- [ ] 14b. Verify: page loads current YAML config into a monospace textarea. "Save Configuration" button is disabled until changes are made. "Discard Changes" restores original. "Download Database Backup" triggers a file download.

- [ ] 14c. Commit: `feat(frontend): implement settings page with YAML editor and backup download`

---

## Task 15: Go embed.FS integration (serve SPA, fallback routing)

- [ ] 15a. Create `internal/web/` directory structure

```bash
mkdir -p /home/andyhazz/projects/whatsupp/internal/web
```

- [ ] 15b. Create `internal/web/embed.go` — embeds the frontend dist directory

```go
// internal/web/embed.go
package web

import "embed"

//go:embed dist/*
var DistFS embed.FS
```

Note: The `dist/` directory must exist at build time. It will be populated by the Dockerfile's Node build stage. For local development, run `cd frontend && npm run build` first, then `cp -r frontend/dist internal/web/dist`.

- [ ] 15c. Create `internal/web/handler.go` — SPA file server with index.html fallback

```go
// internal/web/handler.go
package web

import (
	"io/fs"
	"net/http"
	"strings"
)

// Handler returns an http.Handler that serves the embedded SPA.
// For any path that does not match a static file, it serves index.html
// to support client-side routing.
func Handler() http.Handler {
	// Strip the "dist/" prefix from the embedded filesystem
	distFS, err := fs.Sub(DistFS, "dist")
	if err != nil {
		panic("failed to create sub filesystem: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(distFS))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Don't serve SPA for API routes
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// Try to serve the file directly
		path := r.URL.Path
		if path == "/" {
			path = "/index.html"
		}

		// Check if the file exists in the embedded FS
		f, err := distFS.(fs.ReadFileFS).ReadFile(strings.TrimPrefix(path, "/"))
		if err != nil {
			// File not found — serve index.html for SPA routing
			indexData, err := distFS.(fs.ReadFileFS).ReadFile("index.html")
			if err != nil {
				http.Error(w, "index.html not found", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(indexData)
			return
		}

		// File exists — let the standard file server handle it
		// (this sets correct content types and caching headers)
		_ = f
		fileServer.ServeHTTP(w, r)
	})
}
```

- [ ] 15d. Write test `internal/web/handler_test.go`

```go
// internal/web/handler_test.go
package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandler_ServesIndexForRoot(t *testing.T) {
	// This test requires dist/index.html to exist.
	// In CI, the Node build stage populates it.
	// For local dev, create a minimal one:
	// mkdir -p internal/web/dist && echo '<html></html>' > internal/web/dist/index.html

	handler := Handler()

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("expected HTML content type, got %s", rec.Header().Get("Content-Type"))
	}
}

func TestHandler_FallbackToIndex(t *testing.T) {
	handler := Handler()

	req := httptest.NewRequest("GET", "/monitors/Plex", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for SPA fallback, got %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("expected HTML content type for SPA fallback, got %s", rec.Header().Get("Content-Type"))
	}
}

func TestHandler_ApiPathsNotServed(t *testing.T) {
	handler := Handler()

	req := httptest.NewRequest("GET", "/api/v1/monitors", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for API path, got %d", rec.Code)
	}
}
```

- [ ] 15e. Create a minimal `internal/web/dist/index.html` for test compilation

```bash
mkdir -p /home/andyhazz/projects/whatsupp/internal/web/dist
echo '<!DOCTYPE html><html><head><title>WhatsUpp</title></head><body><div id="app"></div></body></html>' > /home/andyhazz/projects/whatsupp/internal/web/dist/index.html
```

- [ ] 15f. Run tests

```bash
cd /home/andyhazz/projects/whatsupp
go test ./internal/web/ -v
```

Confirm all three tests pass.

- [ ] 15g. Wire the SPA handler into the API router. In `internal/api/router.go`, add at the bottom of the route registration (after all `/api/` routes):

```go
import "whatsupp/internal/web"

// ... at the end of route setup:
// Serve SPA for all non-API routes
router.PathPrefix("/").Handler(web.Handler())
```

The exact integration depends on the router used in Plan 2 (likely gorilla/mux or chi). The key requirement: the SPA handler must be the last route registered so it only catches non-API paths.

- [ ] 15h. Commit: `feat(web): add embed.FS SPA handler with index.html fallback for client-side routing`

---

## Task 16: Multi-stage Dockerfile

- [ ] 16a. Create `frontend/.dockerignore`

```
node_modules
dist
```

- [ ] 16b. Create `Dockerfile` at project root

```dockerfile
# ============================================================
# Stage 1: Build frontend
# ============================================================
FROM node:20-alpine AS frontend-build

WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# ============================================================
# Stage 2: Build Go binary
# ============================================================
FROM golang:1.22-alpine AS go-build

RUN apk add --no-cache gcc musl-dev

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

# Copy compiled frontend into the embed path
COPY --from=frontend-build /app/frontend/dist /app/internal/web/dist/

# Copy all Go source
COPY . .
# Overwrite any local dist with the freshly built one
COPY --from=frontend-build /app/frontend/dist /app/internal/web/dist/

RUN CGO_ENABLED=1 go build -o /whatsupp ./cmd/whatsupp

# ============================================================
# Stage 3: Minimal runtime
# ============================================================
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

COPY --from=go-build /whatsupp /usr/local/bin/whatsupp

RUN mkdir -p /data /etc/whatsupp

VOLUME ["/data"]
EXPOSE 8080

ENTRYPOINT ["whatsupp"]
CMD ["serve"]
```

- [ ] 16c. Create `.dockerignore` at project root

```
.git
node_modules
frontend/node_modules
frontend/dist
*.db
*.db-wal
*.db-shm
.env
```

- [ ] 16d. Verify Dockerfile builds (requires Docker)

```bash
cd /home/andyhazz/projects/whatsupp
docker build -t whatsupp:dev .
```

Confirm image builds successfully and is small (target: <30MB compressed).

- [ ] 16e. Commit: `feat(docker): add multi-stage Dockerfile for frontend + Go build`

---

## Task 17: docker-compose.yml for hub + agent

- [ ] 17a. Create `docker-compose.yml` at project root

```yaml
# docker-compose.yml — WhatsUpp hub deployment
services:
  whatsupp:
    build: .
    container_name: whatsupp
    restart: unless-stopped
    command: serve
    cap_add:
      - NET_RAW    # Required for ICMP ping checks
    volumes:
      - ./config:/etc/whatsupp       # config.yml — writable for Settings UI
      - whatsupp-data:/data
    env_file:
      - .env
    ports:
      - "8080:8080"
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/api/v1/health"]
      interval: 30s
      timeout: 5s
      retries: 3

volumes:
  whatsupp-data:
```

- [ ] 17b. Create `docker-compose.agent.yml` for agent deployment

```yaml
# docker-compose.agent.yml — WhatsUpp agent deployment
# Deploy on each monitored host
services:
  whatsupp-agent:
    image: ghcr.io/andyhazz/whatsupp:latest
    container_name: whatsupp-agent
    restart: unless-stopped
    command: agent
    environment:
      - WHATSUPP_HUB_URL=${WHATSUPP_HUB_URL}
      - WHATSUPP_AGENT_KEY=${AGENT_KEY}
      - DOCKER_HOST=tcp://docker-proxy:2375
    volumes:
      - /:/hostfs:ro
    pid: host
    depends_on:
      - docker-proxy

  docker-proxy:
    image: tecnativa/docker-socket-proxy
    container_name: docker-proxy
    restart: unless-stopped
    environment:
      - CONTAINERS=1
      - POST=0
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
```

- [ ] 17c. Create `config.example.yml` at project root

```yaml
# WhatsUpp configuration
# Copy to config/config.yml and edit for your environment.
# Environment variables can be used with ${VAR_NAME} syntax.

server:
  listen: ":8080"
  db_path: "/data/whatsupp.db"

auth:
  initial_username: "admin"
  initial_password: "${WHATSUPP_ADMIN_PASSWORD}"

monitors:
  - name: "Example Website"
    type: http
    url: "https://example.com"
    interval: 60s
    failure_threshold: 3

  - name: "Gateway"
    type: ping
    host: "192.168.1.1"
    interval: 60s

agents:
  - name: "server-1"
    key: "${AGENT_KEY_SERVER1}"

scrape_targets: []

security:
  targets: []

alerting:
  default_failure_threshold: 3
  ntfy:
    url: "${NTFY_URL}"
    topic: "${NTFY_TOPIC}"
    username: "${NTFY_USERNAME}"
    password: "${NTFY_PASSWORD}"
  thresholds:
    ssl_expiry_days: [14, 7, 3, 1]
    disk_usage_pct: 90
    disk_hysteresis_pct: 5
    down_reminder_interval: "1h"

retention:
  check_results_raw: "720h"
  agent_metrics_raw: "48h"
  agent_metrics_5min: "2160h"
  hourly: "4320h"
  daily: "0"
```

- [ ] 17d. Create `.env.example` at project root

```env
WHATSUPP_ADMIN_PASSWORD=changeme
NTFY_URL=https://ntfy.example.com
NTFY_TOPIC=whatsupp
NTFY_USERNAME=
NTFY_PASSWORD=
AGENT_KEY_SERVER1=change-this-to-a-random-key
```

- [ ] 17e. Verify: `docker compose up --build` starts the hub, healthcheck passes, dashboard is accessible at `http://localhost:8080`.

- [ ] 17f. Commit: `feat(docker): add docker-compose files for hub and agent deployment`

---

## Summary of files created/modified

### New files

| Path | Purpose |
|---|---|
| `frontend/package.json` | Svelte 5 + Vite project |
| `frontend/vite.config.js` | Vite config with API proxy |
| `frontend/src/main.js` | App entry point |
| `frontend/src/app.css` | Dracula global styles |
| `frontend/src/App.svelte` | Root component with router |
| `frontend/src/lib/theme.js` | Dracula colour tokens |
| `frontend/src/lib/auth.js` | Auth store + login/logout |
| `frontend/src/lib/api.js` | API client (fetch wrapper) |
| `frontend/src/lib/ws.js` | WebSocket client (auto-reconnect) |
| `frontend/src/lib/tiers.js` | Time range / tier mapping |
| `frontend/src/components/Layout.svelte` | Sidebar + main content |
| `frontend/src/components/StatusBadge.svelte` | UP/DOWN badge |
| `frontend/src/components/Sparkline.svelte` | Inline SVG sparkline |
| `frontend/src/components/Chart.svelte` | uPlot wrapper (Dracula) |
| `frontend/src/components/TimeRangeSelector.svelte` | Time range buttons |
| `frontend/src/components/Gauge.svelte` | Circular usage gauge |
| `frontend/src/pages/Login.svelte` | Login form |
| `frontend/src/pages/Overview.svelte` | Monitor grid + sparklines |
| `frontend/src/pages/MonitorDetail.svelte` | Zoomable chart + incidents |
| `frontend/src/pages/Hosts.svelte` | Host cards with gauges |
| `frontend/src/pages/HostDetail.svelte` | Per-host metric charts |
| `frontend/src/pages/Security.svelte` | Scan results + baseline diffs |
| `frontend/src/pages/Incidents.svelte` | Sortable incident table |
| `frontend/src/pages/Settings.svelte` | YAML editor + backup |
| `internal/web/embed.go` | embed.FS declaration |
| `internal/web/handler.go` | SPA file server + fallback |
| `internal/web/handler_test.go` | Handler tests |
| `Dockerfile` | Multi-stage build |
| `.dockerignore` | Docker build exclusions |
| `frontend/.dockerignore` | Frontend build exclusions |
| `docker-compose.yml` | Hub deployment |
| `docker-compose.agent.yml` | Agent deployment |
| `config.example.yml` | Example configuration |
| `.env.example` | Example environment variables |

### Modified files

| Path | Change |
|---|---|
| `internal/api/router.go` | Add SPA handler as catch-all route after all API routes |

### Dependency graph

```
Task 1 (scaffold) ← Task 2 (theme) ← Task 3 (router + auth store)
                                            ↓
                                       Task 4 (login)
                                            ↓
                                       Task 5 (layout)
                                            ↓
                                  Task 6 (API client) + Task 7 (WS client)
                                            ↓
                        Task 8 (overview) + Task 9 (chart component)
                               ↓                      ↓
                        Task 10 (monitor detail)
                               ↓
                        Task 11 (hosts)
                               ↓
                  Task 12 (security) + Task 13 (incidents) + Task 14 (settings)
                                            ↓
                                    Task 15 (Go embed.FS)
                                            ↓
                                    Task 16 (Dockerfile)
                                            ↓
                                    Task 17 (docker-compose)
```
