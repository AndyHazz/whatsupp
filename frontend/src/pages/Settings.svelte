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

  let currentPassword = '';
  let newPassword = '';
  let confirmPassword = '';
  let pwSaving = false;
  let pwError = '';
  let pwSuccess = '';

  async function changePassword() {
    pwError = '';
    pwSuccess = '';
    if (newPassword !== confirmPassword) {
      pwError = 'New passwords do not match.';
      return;
    }
    pwSaving = true;
    try {
      await api.changePassword(currentPassword, newPassword);
      pwSuccess = 'Password updated.';
      currentPassword = '';
      newPassword = '';
      confirmPassword = '';
      setTimeout(() => { pwSuccess = ''; }, 5000);
    } catch (e) {
      pwError = e.message;
    } finally {
      pwSaving = false;
    }
  }

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
      const res = await fetch('/api/v1/test-ntfy', {
        method: 'POST',
        credentials: 'include',
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || `HTTP ${res.status}`);
      }
      success = 'Test notification sent! Check your ntfy topic.';
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
  <div class="section">
    <div class="skel" style="width:60%;height:20px;margin-bottom:12px;"></div>
    <div class="skel" style="width:100%;height:400px;border-radius:var(--radius);"></div>
  </div>
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
      <h2>Change Password</h2>
      {#if pwError}
        <div class="msg error-msg">{pwError}</div>
      {/if}
      {#if pwSuccess}
        <div class="msg success-msg">{pwSuccess}</div>
      {/if}
      <form class="pw-form" on:submit|preventDefault={changePassword}>
        <label>
          Current password
          <input type="password" bind:value={currentPassword} autocomplete="current-password" required />
        </label>
        <label>
          New password
          <input type="password" bind:value={newPassword} autocomplete="new-password" required />
        </label>
        <label>
          Confirm new password
          <input type="password" bind:value={confirmPassword} autocomplete="new-password" required />
        </label>
        <div class="actions">
          <button class="btn-primary" type="submit" disabled={pwSaving || !currentPassword || !newPassword || !confirmPassword}>
            {pwSaving ? 'Updating...' : 'Update Password'}
          </button>
        </div>
      </form>
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
    border: 1px solid var(--border-subtle);
    box-shadow: var(--shadow-card);
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
  .btn-primary:hover:not(:disabled) { filter: brightness(1.1); }
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
    background: rgba(189, 147, 249, 0.08);
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

  .pw-form {
    display: flex;
    flex-direction: column;
    gap: 12px;
    max-width: 360px;
  }
  .pw-form label {
    display: flex;
    flex-direction: column;
    gap: 4px;
    font-size: 0.85rem;
    color: var(--fg-muted);
  }
  .pw-form input {
    font-size: 0.9rem;
  }

</style>
