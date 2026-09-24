// import type { WsEvent } from '../types/containers';

// export type { WsEvent };

// export function connect(onEvent: (e: WsEvent) => void): () => void {
//   const ws = new WebSocket(`ws://${location.host}/ws`);

//   ws.onmessage = (msg) => {
//     try {
//       const data = JSON.parse(msg.data) as WsEvent;
//       onEvent(data);
//     } catch {
//       // ignore malformed messages
//     }
//   };

//   ws.onerror = () => console.warn('Pulse WebSocket error');

//   return () => ws.close();
// }

import type { WsEvent } from '../types/containers';

export type { WsEvent };

const RELEVANT_ACTIONS = new Set([
  'start',
  'die',
  'stop',
  'restart',
  'kill',
  'destroy',
  'create',
  'update',
]);

/**
 * This was rewritten this way to avoid constant
 * but unnecessary polling
 *
 * @param onEvent
 * @returns
 */
export function connect(onEvent: (e: WsEvent) => void): () => void {
  // Same-origin: ws:// on http pages, wss:// on https, always the host:port the
  // app was served from (which is the gateway).
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const ws = new WebSocket(`${protocol}//${location.host}/ws`);

  ws.onmessage = (msg) => {
    try {
      const data = JSON.parse(msg.data) as WsEvent;

      // Ignore ping/pong/heartbeat packets
      if (!data || (data as any).type === 'ping' || (data as any).type === 'pong') {
        return;
      }

      // If the backend sends an action/event field, only trigger on state changes
      const action = (data as any).action ?? (data as any).event ?? (data as any).type;
      if (action && !RELEVANT_ACTIONS.has(action)) {
        return;
      }

      onEvent(data);
    } catch {
      // ignore malformed messages
    }
  };

  ws.onerror = () => console.warn('[Pulse] : WebSocket error');

  return () => ws.close();
}
