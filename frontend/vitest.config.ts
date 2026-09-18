import vue from '@vitejs/plugin-vue'
import { defineConfig } from 'vitest/config'

// The Vue plugin is here so component tests can import single-file components.
// They render through `@vue/server-renderer`, which ships with Vue: that gives
// the markup a component really produces without a browser or a DOM package.
export default defineConfig({
  plugins: [vue()],
  test: {
    environment: 'node',
    include: ['src/tests/**/*.test.ts'],
  },
})
