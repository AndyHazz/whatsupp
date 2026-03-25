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
    <div class="login-brand">
      <svg viewBox="0 0 64 64" width="48" height="48">
        <rect width="64" height="64" rx="14" fill="#282a36"/>
        <path d="M32 8 C36 8 52 22 52 27 C52 32 43 34 40 34 L37 52 C37 56 27 56 27 52 L24 34 C21 34 12 32 12 27 C12 22 28 8 32 8 Z" fill="#50fa7b"/>
      </svg>
      <h1>WhatsUpp</h1>
    </div>
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
    border: 1px solid var(--border-subtle);
    box-shadow: var(--shadow-card);
  }

  .login-brand {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 12px;
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
    filter: brightness(1.1);
  }
  button:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }
</style>
