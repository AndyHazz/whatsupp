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
