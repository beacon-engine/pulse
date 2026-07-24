import { writable } from 'svelte/store';

export type Route = 'containers' | 'stacks' | 'events' | 'kubernetes' | 'swarm' | 'metrics' | 'resources' | 'users';

function readHash(): Route {
  const hash = window.location.hash.replace('#/', '').replace('#', '') as Route;
  const valid: Route[] = ['containers', 'stacks', 'events', 'kubernetes', 'swarm', 'metrics', 'resources', 'users'];
  return valid.indexOf(hash) !== -1 ? hash : 'containers';
}

export const currentRoute = writable<Route>(readHash());

export function navigate(route: Route) {
  window.location.hash = `#/${route}`;
}

export function initRouter() {
  const update = () => currentRoute.set(readHash());
  window.addEventListener('hashchange', update);
  update();
  return () => window.removeEventListener('hashchange', update);
}
