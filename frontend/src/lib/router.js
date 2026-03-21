import { writable, derived } from 'svelte/store';

export const path = writable(window.location.pathname);

// Listen for popstate (back/forward)
window.addEventListener('popstate', () => {
  path.set(window.location.pathname);
});

// Navigate programmatically
export function navigate(to) {
  window.history.pushState({}, '', to);
  path.set(to);
}

// Click handler for links
export function link(node) {
  function handleClick(e) {
    const href = node.getAttribute('href');
    if (!href || href.startsWith('http') || e.ctrlKey || e.metaKey || e.shiftKey) return;
    e.preventDefault();
    navigate(href);
  }
  node.addEventListener('click', handleClick);
  return {
    destroy() {
      node.removeEventListener('click', handleClick);
    }
  };
}

// Route matching helper
export function matchRoute(pattern, pathname) {
  if (pattern === pathname) return {};

  const patternParts = pattern.split('/');
  const pathParts = pathname.split('/');

  if (patternParts.length !== pathParts.length) return null;

  const params = {};
  for (let i = 0; i < patternParts.length; i++) {
    if (patternParts[i].startsWith(':')) {
      params[patternParts[i].slice(1)] = decodeURIComponent(pathParts[i]);
    } else if (patternParts[i] !== pathParts[i]) {
      return null;
    }
  }
  return params;
}
