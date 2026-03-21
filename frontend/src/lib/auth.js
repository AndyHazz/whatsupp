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
