import { getContext, setContext } from 'svelte';
import type { AppState } from '../types/appState';

const KEY = 'beacon-app-state';

export function setAppState(state: AppState) {
  setContext(KEY, state);
}

export function getAppState(): AppState {
  return getContext(KEY) as AppState;
}
