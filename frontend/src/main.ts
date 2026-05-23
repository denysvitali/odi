import { createApp } from 'vue'
import { createPinia } from 'pinia'

import App from './App.vue'
import router from './router'

import './assets/styles/tailwind.css'
import './assets/styles/design-tokens.css'
import type { Settings } from '@/types/settings'
import { logger } from '@/lib/logger'

declare global {
  interface Window {
    _settings?: Settings
  }
}

function isValidUrl(s: string): boolean {
  try {
    const u = new URL(s)
    return u.protocol === 'http:' || u.protocol === 'https:'
  } catch {
    return false
  }
}

function isValidSettings(value: unknown): value is Settings {
  if (!value || typeof value !== 'object') return false
  const s = value as Record<string, unknown>
  if (typeof s.apiUrl !== 'string' || s.apiUrl.length === 0) return false
  if (!isValidUrl(s.apiUrl)) return false
  // opensearchUrl is optional, but if present must be a valid http(s) URL too.
  if (s.opensearchUrl !== undefined) {
    if (typeof s.opensearchUrl !== 'string') return false
    if (s.opensearchUrl.length > 0 && !isValidUrl(s.opensearchUrl)) return false
  }
  return true
}

async function loadSettings(): Promise<Settings> {
  const basePath = import.meta.env.BASE_URL || '/'
  const settingsFile =
    typeof window !== 'undefined' && window.location.hostname === 'odi.denv.it'
      ? 'settings-mock.json'
      : 'settings.json'
  const path = new URL(settingsFile, window.location.origin + basePath).toString()
  const res = await fetch(path)
  if (!res.ok) throw new Error(`Failed to load settings (${res.status})`)
  const data = await res.json()
  if (!isValidSettings(data)) {
    throw new Error('Invalid settings: apiUrl and opensearchUrl must be valid http(s) URLs')
  }
  return data
}

function renderFatal(message: string) {
  const root = document.getElementById('app')
  if (!root) return

  // Build the error UI with DOM APIs only — no innerHTML, no raw HTML strings.
  // textContent assignments are inert and immune to injection.
  while (root.firstChild) root.removeChild(root.firstChild)

  const wrapper = document.createElement('div')
  wrapper.style.minHeight = '100vh'
  wrapper.style.display = 'flex'
  wrapper.style.alignItems = 'center'
  wrapper.style.justifyContent = 'center'
  wrapper.style.padding = '24px'
  wrapper.style.fontFamily = 'system-ui, sans-serif'

  const inner = document.createElement('div')
  inner.style.maxWidth = '420px'
  inner.style.textAlign = 'center'

  const heading = document.createElement('h1')
  heading.style.fontSize = '20px'
  heading.style.fontWeight = '600'
  heading.style.marginBottom = '8px'
  heading.textContent = 'Configuration error'

  const body = document.createElement('p')
  body.style.color = '#666'
  body.style.marginBottom = '16px'
  body.textContent = message

  const button = document.createElement('button')
  button.type = 'button'
  button.style.padding = '8px 16px'
  button.style.borderRadius = '8px'
  button.style.border = '1px solid #ccc'
  button.style.background = '#fff'
  button.style.cursor = 'pointer'
  button.textContent = 'Reload'
  button.addEventListener('click', () => window.location.reload())

  inner.appendChild(heading)
  inner.appendChild(body)
  inner.appendChild(button)
  wrapper.appendChild(inner)
  root.appendChild(wrapper)
}

loadSettings()
  .then((settings) => {
    window._settings = settings
    const app = createApp(App)
    app.use(createPinia())
    app.use(router)
    app.mount('#app')
  })
  .catch((err: Error) => {
    logger.error('Failed to bootstrap app:', err)
    renderFatal(err.message)
  })
