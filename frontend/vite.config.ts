import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

// Dev-server proxy target: the running gateway. Override for your setup, e.g.
//   PULSE_GATEWAY_TARGET=http://localhost:8090 npm run dev
// This only affects `vite dev` — in production the gateway serves the built SPA
// and the browser talks to it same-origin (see src/lib/api.ts, src/lib/websocket.ts).
const gatewayTarget = process.env.PULSE_GATEWAY_TARGET ?? 'http://localhost:9080';
// http -> ws, https -> wss (so the /ws proxy tracks the same host/scheme).
const wsTarget = gatewayTarget.replace(/^http/, 'ws');

export default defineConfig({
  plugins: [svelte()],
  server: {
    port: 9010,
    proxy: {
      '/api': gatewayTarget,
      '/metrics': gatewayTarget,
      '/ws': { target: wsTarget, ws: true },
    },
  },
});
