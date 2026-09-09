import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter, HashRouter } from 'react-router-dom'
import './index.css'
import App from './App.tsx'

// Demo build'i (VITE_DEMO): backend yok. Demo katmanı window.fetch +
// WebSocket'i sentetik veriyle yanıtlar (src/demo/). App mount olmadan ÖNCE
// kurulur ki ilk istekler de yakalansın. Router HashRouter'a düşer — GitHub
// Pages alt-dizininde ("/bazntms/demo/") derin-link SPA fallback'i yok.
const DEMO = import.meta.env.VITE_DEMO === '1'
const Router = DEMO ? HashRouter : BrowserRouter

async function boot() {
  if (DEMO) {
    const { installDemo } = await import('./demo/install')
    installDemo()
  }
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <Router>
        <App />
      </Router>
    </StrictMode>,
  )
}

void boot()
