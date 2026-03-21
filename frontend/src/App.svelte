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
  import { connect, disconnect } from './lib/ws.js';

  $: if ($isAuthenticated) {
    connect();
  } else {
    disconnect();
  }
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
