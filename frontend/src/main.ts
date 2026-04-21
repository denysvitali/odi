import { createApp } from 'vue'
import { createPinia } from 'pinia'

import App from './App.vue'
import router from './router'

import './assets/styles/tailwind.css'
import './assets/styles/design-tokens.css'
import type { Settings } from '@/types/settings'

declare global {
  interface Window {
    _settings: Settings
  }
}

function isValidSettings(value: unknown): value is Settings {
  if (!value || typeof value !== 'object') return false
  const s = value as Record<string, unknown>
  return typeof s.apiUrl === 'string' && s.apiUrl.length > 0
}

async function loadSettings(): Promise<Settings> {
  const path =
    typeof window !== 'undefined' && window.location.hostname === 'odi.denv.it'
      ? '/settings-mock.json'
      : '/settings.json'
  const res = await fetch(path)
  if (!res.ok) throw new Error(`Failed to load settings (${res.status})`)
  const data = await res.json()
  if (!isValidSettings(data)) throw new Error('Invalid settings: missing apiUrl')
  return data
}

function renderFatal(message: string) {
  const root = document.getElementById('app')
  if (!root) return
  root.innerHTML = `
    <div style="min-height:100vh;display:flex;align-items:center;justify-content:center;padding:24px;font-family:system-ui,sans-serif;">
      <div style="max-width:420px;text-align:center;">
        <h1 style="font-size:20px;font-weight:600;margin-bottom:8px;">Configuration error</h1>
        <p style="color:#666;margin-bottom:16px;">${message.replace(/</g, '&lt;')}</p>
        <button onclick="window.location.reload()" style="padding:8px 16px;border-radius:8px;border:1px solid #ccc;background:#fff;cursor:pointer;">Reload</button>
      </div>
    </div>`
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
    console.error('Failed to bootstrap app:', err)
    renderFatal(err.message)
  })
