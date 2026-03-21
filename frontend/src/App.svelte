<script>
  import { path, matchRoute } from './lib/router.js';
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
  import { connect, disconnect } from './lib/ws.js';

  // Manage WS connection based on auth
  let wasAuth = false;
  $: if ($isAuthenticated && !wasAuth) {
    connect();
    wasAuth = true;
  } else if (!$isAuthenticated && wasAuth) {
    disconnect();
    wasAuth = false;
  }

  // Route matching
  $: currentPath = $path;
  $: monitorMatch = matchRoute('/monitors/:name', currentPath);
  $: hostMatch = matchRoute('/hosts/:name', currentPath);
</script>

{#if !$isAuthenticated}
  <Login />
{:else}
  <Layout>
    {#if currentPath === '/'}
      <Overview />
    {:else if monitorMatch}
      <MonitorDetail name={monitorMatch.name} />
    {:else if currentPath === '/hosts'}
      <Hosts />
    {:else if hostMatch}
      <HostDetail name={hostMatch.name} />
    {:else if currentPath === '/security'}
      <Security />
    {:else if currentPath === '/incidents'}
      <Incidents />
    {:else if currentPath === '/settings'}
      <Settings />
    {:else}
      <Overview />
    {/if}
  </Layout>
{/if}
