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
