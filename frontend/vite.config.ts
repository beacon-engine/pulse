import { defineConfig } from 'vite';
import { svelte } from '@sveltejs/vite-plugin-svelte';

export default defineConfig({
  plugins: [svelte()],
  server: {
    port: 5173,
    proxy: {
      '/api': 'http://localhost:9000',
      '/metrics': 'http://localhost:9000',
      '/ws': { target: 'ws://localhost:9000', ws: true },
    },
  },
});
