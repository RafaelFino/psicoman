import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: 'dist',
    emptyOutDir: true,
  },
  server: {
    // Proxies /api/* to the Go backend during local development (npm run dev).
    // The Go server must be running on :8080 (make run or make run-dev).
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
})
