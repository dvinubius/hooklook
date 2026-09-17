import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// During development the browser stays on the Vite origin and Vite proxies
// every application route to Go. That keeps one origin for the owner cookie,
// for the `Origin` header Go checks on mutations, and for EventSource — while
// the bin page is still served (and authorized) by Go, not by Vite.
//
// Run Go with PUBLIC_BASE_URL set to this origin so its capture URLs and
// same-origin check agree with what the browser is actually using.
const toGo = {
  target: 'http://127.0.0.1:8080',
  // Keep the browser's Host/Origin so Go's same-origin check sees the real one.
  changeOrigin: false,
}

export default defineConfig({
  plugins: [vue()],
  server: {
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
    proxy: {
      '^/$': toGo, // home: Go's resolve-or-create redirect
      '/bins': toGo, // the authorized page shell
      '/api': toGo, // metadata, list, detail, SSE, owner mutations
      '/b/': toGo, // public capture
      '/health': toGo,
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    assetsDir: 'assets',
    sourcemap: false,
  },
})
