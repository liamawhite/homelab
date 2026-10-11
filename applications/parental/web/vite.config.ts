import { fileURLToPath, URL } from 'node:url';

import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// Connect's HTTP paths are all under /<package>.<Service>/, so proxying
// everything under /parental.* to the local Go server (see
// cmd/parental/main.go, PORT defaults to 8080) lets `npm run dev` and
// `go run ./cmd/parental` run side by side against the same browser origin -
// matching the single-binary same-origin setup this app runs as in
// production, so no CORS handling is ever needed.
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    proxy: {
      '/parental.v1.PlannerService': 'http://localhost:8080',
    },
  },
});
