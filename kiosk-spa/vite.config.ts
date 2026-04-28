import { fileURLToPath, URL } from 'node:url';
import { defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '');
  const apiTarget = env.API_TARGET || 'http://localhost:8089';

  return {
    plugins: [tailwindcss(), react()],
    resolve: {
      alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
    },
    server: {
      port: 5180,
      proxy: {
        '/api': { target: apiTarget, changeOrigin: true },
      },
    },
    build: { outDir: 'dist', emptyOutDir: true, sourcemap: true },
  };
});
