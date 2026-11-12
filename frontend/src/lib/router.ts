import { writable } from 'svelte/store';

export type Route = 'containers' | 'stacks' | 'events' | 'kubernetes' | 'swarm' | 'metrics' | 'resources' | 'users' | 'settings' | 'hosts';

const ROUTES: Route[] = ['containers', 'stacks', 'events', 'kubernetes', 'swarm', 'metrics', 'resources', 'users', 'settings', 'hosts'];

// The hash holds the page and the active environment: #/kubernetes?env=<id>.
// Keeping the environment in the URL makes a refresh, a bookmark or a shared
// link land on the same environment instead of the default one.
function parseHash(): { route: Route; env: string | null } {
  const raw = window.location.hash.replace(/^#\/?/, '');
  const [path, query = ''] = raw.split('?');
  const route = ROUTES.includes(path as Route) ? (path as Route) : 'containers';
  return { route, env: new URLSearchParams(query).get('env') };
}

function buildHash(route: Route, env: string | null): string {
  return `#/${route}${env ? `?env=${encodeURIComponent(env)}` : ''}`;
}

export const currentRoute = writable<Route>(parseHash().route);

/** The environment id named in the URL, if any. */
export const currentEnv = writable<string | null>(parseHash().env);

export function envFromUrl(): string | null {
  return parseHash().env;
}

/** Go to a page, staying on the current environment. */
export function navigate(route: Route) {
  window.location.hash = buildHash(route, parseHash().env);
}

/** Record the active environment in the URL. `replace` rewrites the current
 *  history entry (for corrections on load) instead of adding one. */
export function setEnvInUrl(env: string, replace = false) {
  const hash = buildHash(parseHash().route, env);
  if (hash === window.location.hash) return;
  if (replace) {
    history.replaceState(history.state, '', hash);
    currentEnv.set(env);
  } else {
    window.location.hash = hash;
  }
}

export function initRouter() {
  const update = () => {
    const { route, env } = parseHash();
    currentRoute.set(route);
    currentEnv.set(env);
  };
  window.addEventListener('hashchange', update);
  update();
  return () => window.removeEventListener('hashchange', update);
}
