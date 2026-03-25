<script>
  import { onMount } from 'svelte';
  import { link } from '../lib/router.js';
  import { logout } from '../lib/auth.js';
  import { wsConnected } from '../lib/ws.js';
  import { api } from '../lib/api.js';

  let serverVersion = '';

  onMount(async () => {
    try {
      const h = await api.getHealth();
      serverVersion = h.version || '';
    } catch {}
  });

  const navItems = [
    { path: '/',          label: 'Overview',   icon: '&#9673;' },
    { path: '/monitors',  label: 'Monitors',   icon: '&#9672;' },
    { path: '/hosts',     label: 'Hosts',      icon: '&#9881;' },
    { path: '/security',  label: 'Security',   icon: '&#9888;' },
    { path: '/incidents', label: 'Incidents',   icon: '&#9889;' },
    { path: '/settings',  label: 'Settings',   icon: '&#9881;' },
  ];

  let sidebarOpen = false;

  function closeSidebar() {
    sidebarOpen = false;
  }

  function navClick(e) {
    // Close sidebar on mobile after navigation
    if (window.innerWidth <= 768) {
      sidebarOpen = false;
    }
  }
</script>

<div class="layout">
  <!-- Mobile top bar -->
  <header class="topbar">
    <button class="menu-btn" on:click={() => sidebarOpen = !sidebarOpen}>
      <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
        <line x1="3" y1="6" x2="21" y2="6"/>
        <line x1="3" y1="12" x2="21" y2="12"/>
        <line x1="3" y1="18" x2="21" y2="18"/>
      </svg>
    </button>
    <a href="/" use:link class="topbar-logo">
      <svg class="logo-icon" viewBox="0 0 64 64" width="28" height="28">
        <rect width="64" height="64" rx="16" fill="#282a36"/>
        <path d="M32 8 C36 8 52 22 52 27 C52 32 43 34 40 34 L37 52 C37 56 27 56 27 52 L24 34 C21 34 12 32 12 27 C12 22 28 8 32 8 Z" fill="#50fa7b"/>
      </svg>
      <span>WhatsUpp</span>
    </a>
  </header>

  <!-- Overlay for mobile -->
  {#if sidebarOpen}
    <div class="overlay" on:click={closeSidebar}></div>
  {/if}

  <aside class="sidebar" class:open={sidebarOpen}>
    <div class="logo">
      <a href="/" use:link on:click={navClick}>
        <svg class="logo-icon" viewBox="0 0 64 64" width="32" height="32">
          <rect width="64" height="64" rx="14" fill="#282a36"/>
          <path d="M32 8 C36 8 52 22 52 27 C52 32 43 34 40 34 L37 52 C37 56 27 56 27 52 L24 34 C21 34 12 32 12 27 C12 22 28 8 32 8 Z" fill="#50fa7b"/>
        </svg>
        <h2>WhatsUpp</h2>
      </a>
    </div>
    <nav>
      {#each navItems as item}
        <a href={item.path} use:link class="nav-item" on:click={navClick}>
          <span class="nav-icon">{@html item.icon}</span>
          <span class="nav-label">{item.label}</span>
        </a>
      {/each}
    </nav>
    <div class="sidebar-footer">
      <div class="ws-status" class:connected={$wsConnected}>
        <span class="ws-dot"></span>
        {$wsConnected ? 'Live' : 'Disconnected'}
        {#if serverVersion}
          <a href="https://github.com/andyhazz/whatsupp" target="_blank" rel="noopener noreferrer" class="version">
            <svg viewBox="0 0 19 19" width="12" height="12" fill="currentColor"><use href="/icons.svg#github-icon"/></svg>
            {serverVersion}
          </a>
        {/if}
      </div>
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

  /* ── Top bar (mobile only) ────────────── */
  .topbar {
    display: none;
  }

  .overlay {
    display: none;
  }

  /* ── Sidebar ──────────────────────────── */
  .sidebar {
    background: #22232e;
    display: flex;
    flex-direction: column;
    padding: 16px 0;
    border-right: 1px solid var(--border-subtle);
    position: sticky;
    top: 0;
    height: 100vh;
    overflow-y: auto;
  }

  .logo {
    padding: 0 20px 20px;
    border-bottom: 1px solid var(--border-subtle);
    margin-bottom: 8px;
  }
  .logo a {
    text-decoration: none;
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .logo h2 {
    color: var(--purple);
    font-size: 1.4rem;
  }
  .logo-icon {
    flex-shrink: 0;
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
    transition: background 0.15s ease, color 0.15s ease;
  }
  .nav-item:hover {
    background: rgba(248, 248, 242, 0.08);
    text-decoration: none;
    color: var(--fg);
  }

  .nav-icon {
    font-size: 1.1rem;
    width: 20px;
    text-align: center;
  }

  .sidebar-footer {
    padding: 12px 16px;
    border-top: 1px solid var(--border-subtle);
    display: flex;
    flex-direction: column;
    gap: 8px;
  }

  .ws-status {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 0.75rem;
    color: var(--fg-muted);
  }
  .ws-status.connected {
    color: var(--green);
  }
  .ws-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--fg-muted);
  }
  .ws-status.connected .ws-dot {
    background: var(--green);
  }
  .version {
    margin-left: auto;
    font-size: 0.7rem;
    opacity: 0.7;
    color: var(--fg-muted);
    text-decoration: none;
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .version:hover {
    color: var(--purple);
    opacity: 1;
    text-decoration: none;
  }

  .logout-btn {
    background: none;
    border: 1px solid var(--fg-muted);
    color: var(--fg-muted);
    padding: 6px 12px;
    border-radius: var(--radius);
    font-size: 0.85rem;
    width: 100%;
    cursor: pointer;
  }
  .logout-btn:hover {
    border-color: var(--red);
    color: var(--red);
    background: rgba(255, 85, 85, 0.08);
  }

  .content {
    padding: 20px 24px 24px;
    overflow-x: hidden;
  }

  /* ── Mobile ───────────────────────────── */
  @media (max-width: 768px) {
    .layout {
      grid-template-columns: 1fr;
      grid-template-rows: auto 1fr;
    }

    .topbar {
      display: flex;
      align-items: center;
      gap: 12px;
      padding: 10px 16px;
      background: var(--bg-card);
      border-bottom: 1px solid var(--border-subtle);
      position: sticky;
      top: 0;
      z-index: 100;
    }
    .topbar-logo {
      display: flex;
      align-items: center;
      gap: 8px;
      text-decoration: none;
      color: var(--purple);
      font-weight: 700;
      font-size: 1.1rem;
    }
    .menu-btn {
      background: none;
      border: none;
      color: var(--fg);
      padding: 4px;
      cursor: pointer;
      display: flex;
      align-items: center;
    }

    .sidebar {
      position: fixed;
      top: 0;
      left: -280px;
      width: 260px;
      height: 100vh;
      z-index: 200;
      transition: left 0.25s ease;
      border-right: 1px solid var(--border-subtle);
    }
    .sidebar.open {
      left: 0;
    }

    .overlay {
      display: block;
      position: fixed;
      inset: 0;
      background: rgba(0, 0, 0, 0.5);
      z-index: 150;
    }
  }
</style>
