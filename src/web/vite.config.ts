import { defineConfig, type ProxyOptions } from 'vite';
import react from '@vitejs/plugin-react';
import path from 'path';

const apiProxyTarget = process.env.VITE_API_PROXY_TARGET ?? 'http://127.0.0.1:8080';

function createApiProxy(): ProxyOptions {
  return {
    target: apiProxyTarget,
    changeOrigin: true,
    secure: false,
  };
}

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    proxy: {
      '/api': createApiProxy(),
      '/v1': createApiProxy(),
    },
  },
  preview: {
    proxy: {
      '/api': createApiProxy(),
      '/v1': createApiProxy(),
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    exclude: ['e2e/**', 'node_modules/**', 'dist/**'],
    setupFiles: ['./src/test/setup.ts'],
  },
});
