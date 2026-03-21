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

  const ct = res.headers.get('content-type') || '';

  // Handle backup endpoint (returns file)
  if (ct.includes('application/octet-stream')) {
    return res.blob();
  }

  // Handle YAML responses (config endpoint)
  if (ct.includes('yaml') || ct.includes('text/plain')) {
    const text = await res.text();
    return { config: text };
  }

  return res.json();
}

async function requestRaw(method, path, body, contentType) {
  const res = await fetch(`${BASE}${path}`, {
    method,
    credentials: 'include',
    headers: { 'Content-Type': contentType },
    body,
  });
  if (res.status === 401) {
    window.location.reload();
    throw new ApiError(401, 'Session expired');
  }
  if (!res.ok) {
    const data = await res.json().catch(() => ({}));
    throw new ApiError(res.status, data.error || `HTTP ${res.status}`);
  }
  if (res.status === 204) return null;
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

  // Account
  changePassword: (currentPassword, newPassword) =>
    request('POST', '/auth/change-password', { current_password: currentPassword, new_password: newPassword }),

  // Config
  getConfig:    ()       => request('GET', '/config'),
  updateConfig: (yaml)   => requestRaw('PUT', '/config', yaml, 'application/x-yaml'),

  // Admin
  getBackup: () => request('GET', '/admin/backup'),

  // Health
  getHealth: () => request('GET', '/health'),
};
