import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  base: process.env.UI_BASE_PATH || '/',
  plugins: [react()],
  // React + Mantine form one ~600 kB vendor chunk; pdf.js loads lazily with the upload page.
  build: { chunkSizeWarningLimit: 800 },
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) }
  }
})
