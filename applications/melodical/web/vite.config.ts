import { defineConfig, loadEnv } from 'vite';
export default defineConfig(({ mode }) => ({
  server: {
    proxy: {
      '/api':
        loadEnv(mode, '.', 'MELODICAL_').MELODICAL_API_URL ||
        'http://127.0.0.1:8080',
    },
  },
  build: { outDir: 'dist' },
}));
