import { svelte } from '@sveltejs/vite-plugin-svelte'
import { defineConfig } from 'vitest/config'

// The production build goes straight into the Go module so that `go:embed`
// (exporter/web) picks it up and the whole app ships as a single binary.
//
// Development: `npm run dev` serves the UI with hot reload and proxies /api to a
// real exporter, e.g.   RKTOP_API=http://<ip-board>:9888 npm run dev
export default defineConfig({
  plugins: [svelte()],
  // Relative addresses ("./assets/..."), so the same build works at / and under any
  // sub-path behind a reverse proxy (https://host/rktopng/) without a rebuild.
  base: './',
  build: {
    outDir: '../exporter/web/dist',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/api': {
        target: process.env.RKTOP_API ?? 'http://127.0.0.1:9888',
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: 'node',
    include: ['src/**/*.test.ts'],
  },
})
