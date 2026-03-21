<script>
  import { link } from '../lib/router.js';
  import { logout } from '../lib/auth.js';
  import { wsConnected } from '../lib/ws.js';

  const navItems = [
    { path: '/',          label: 'Overview',   icon: '&#9673;' },
    { path: '/hosts',     label: 'Hosts',      icon: '&#9881;' },
    { path: '/security',  label: 'Security',   icon: '&#9888;' },
    { path: '/incidents', label: 'Incidents',   icon: '&#9889;' },
    { path: '/settings',  label: 'Settings',   icon: '&#9881;' },
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
      <div class="ws-status" class:connected={$wsConnected}>
        <span class="ws-dot"></span>
        {$wsConnected ? 'Live' : 'Disconnected'}
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
