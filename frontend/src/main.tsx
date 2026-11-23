import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'
import type { Settings } from '@/types/settings'
import { hasValidSettings } from '@/lib/config'
import { logger } from '@/lib/logger'

declare global {
  interface Window {
    _settings?: Settings
  }
}

async function loadSettings(): Promise<Settings> {
  const base = import.meta.env.BASE_URL || '/'
  const file = window.location.hostname === 'odi.denv.it' ? 'settings-mock.json' : 'settings.json'
  const res = await fetch(new URL(file, window.location.origin + base))
  if (!res.ok) throw new Error(`Failed to load settings (${res.status})`)
  return res.json()
}

// Rendered with DOM APIs only (no innerHTML): settings failed, so nothing else is trusted yet.
function renderFatal(root: HTMLElement, message: string) {
  root.replaceChildren()
  const box = document.createElement('div')
  box.style.cssText =
    'min-height:100vh;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:12px;font-family:system-ui,sans-serif;padding:24px;text-align:center'
  const heading = document.createElement('h1')
  heading.style.fontSize = '20px'
  heading.textContent = 'Configuration error'
  const body = document.createElement('p')
  body.style.color = '#666'
  body.textContent = message
  const button = document.createElement('button')
  button.type = 'button'
  button.textContent = 'Reload'
  button.addEventListener('click', () => window.location.reload())
  box.append(heading, body, button)
  root.append(box)
}

const root = document.getElementById('root')!
loadSettings()
  .then((settings) => {
    window._settings = settings
    if (!hasValidSettings())
      throw new Error('Invalid settings: apiUrl and opensearchUrl must be valid http(s) URLs')
    createRoot(root).render(
      <StrictMode>
        <App />
      </StrictMode>
    )
  })
  .catch((err: Error) => {
    logger.error('Failed to bootstrap app:', err)
    renderFatal(root, err.message)
  })
