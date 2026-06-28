// SPDX-FileCopyrightText: 2026 Skaphos
// SPDX-License-Identifier: MIT
import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    port: 5173,
  },
  test: {
    globals: true,
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    css: true,
    // Playwright specs live under e2e/ and run with their own runner.
    exclude: ['**/node_modules/**', '**/e2e/**', '**/dist/**'],
  },
})
