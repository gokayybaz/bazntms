import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// `--mode demo` (npm run build:demo): backend'siz statik demo build'i.
// base = GitHub Pages alt-dizini, çıktı ayrı klasöre (web/dist'e dokunmaz).
// Router demo'da HashRouter'a düşer (Pages alt-dizininde deep-link 404'ü yok).
export default defineConfig(({ mode }) => {
  const demo = mode === 'demo'
  return {
    plugins: [react(), tailwindcss()],
    base: demo ? '/bazntms/demo/' : '/',
    build: {
      outDir: demo ? '../web/demo' : '../web/dist',
      emptyOutDir: true,
    },
    server: {
      proxy: {
        '/api': 'http://localhost:8080',
        '/ws': { target: 'ws://localhost:8080', ws: true },
      },
    },
  }
})
