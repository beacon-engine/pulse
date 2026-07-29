import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte()],
  server: {
    port: 9010,
    proxy: {
      // Gateway HTTP port. 8080 collides with an existing container here, so the
      // gateway is run on 8090 (HTTP_PORT=8090). Change both to match your setup.
      '/api': 'http://localhost:8090',
      '/metrics': 'http://localhost:8090',
      '/ws': { target: 'ws://localhost:8090', ws: true },
    },
  },
});
