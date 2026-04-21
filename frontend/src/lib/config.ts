import type { Settings } from '@/types/settings'

export class ConfigError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'ConfigError'
  }
}

function isValidSettings(value: unknown): value is Settings {
  if (!value || typeof value !== 'object') return false
  const s = value as Record<string, unknown>
  return typeof s.apiUrl === 'string' && s.apiUrl.length > 0
}

export function getSettings(): Settings {
  const value = (window as unknown as { _settings?: unknown })._settings
  if (!isValidSettings(value)) {
    throw new ConfigError('Settings not loaded or invalid: missing apiUrl')
  }
  return value
}

export function getApiUrl(): string {
  try {
    return getSettings().apiUrl.replace(/\/$/, '')
  } catch {
    return ''
  }
}

export function getOpensearchUrl(): string {
  try {
    return getSettings().opensearchUrl || ''
  } catch {
    return ''
  }
}

export function hasValidSettings(): boolean {
  return isValidSettings((window as unknown as { _settings?: unknown })._settings)
}
