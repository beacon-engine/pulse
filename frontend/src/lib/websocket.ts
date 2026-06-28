import type { WsEvent } from '../types/containers';

export type { WsEvent };

export function connect(onEvent: (e: WsEvent) => void): () => void {
  const ws = new WebSocket(`ws://${location.host}/ws`);

  ws.onmessage = (msg) => {
    try {
      const data = JSON.parse(msg.data) as WsEvent;
      onEvent(data);
    } catch {
      // ignore malformed messages
    }
  };

  ws.onerror = () => console.warn('Beacon WebSocket error');

  return () => ws.close();
}
