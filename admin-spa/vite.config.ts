import { fileURLToPath, URL } from 'node:url';
import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

// The admin SPA is served by the Go binary at `/admin/...` in production.
// Vite's `base` is `/admin/` so all built asset URLs include that prefix.
// In dev we proxy /api/* to the Go server on :8089 so cookie auth works.
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');
  const apiTarget = env.API_TARGET || 'http://localhost:8089';

  return {
    base: '/admin/',
    plugins: [tailwindcss(), react()],
    resolve: { alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) } },
    server: {
      port: 5181,
      proxy: {
        '/api': { target: apiTarget, changeOrigin: true },
      },
    },
    build: { outDir: 'dist', emptyOutDir: true, sourcemap: true },
  };
});
