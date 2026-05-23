import type { Settings } from '@/types/settings'
import { logger } from '@/lib/logger'

export class ConfigError extends Error {
  constructor(message: string) {
    super(message)
    this.name = 'ConfigError'
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
  if (typeof s.apiUrl !== 'string' || s.apiUrl.length === 0) {
    logger.error('config: apiUrl must be a non-empty string')
    return false
  }
  if (!isValidUrl(s.apiUrl)) {
    logger.error('config: apiUrl must be a valid http(s) URL', s.apiUrl)
    return false
  }
  if (s.opensearchUrl !== undefined) {
    if (typeof s.opensearchUrl !== 'string') {
      logger.error('config: opensearchUrl must be a string when set')
      return false
    }
    if (s.opensearchUrl.length > 0 && !isValidUrl(s.opensearchUrl)) {
      logger.error('config: opensearchUrl must be a valid http(s) URL', s.opensearchUrl)
      return false
    }
  }
  return true
}

export function getSettings(): Settings {
  const value = window._settings
  if (!isValidSettings(value)) {
    // Fail loud — a bad config could otherwise let `javascript:` / `data:` URLs
    // through. Refusing to operate is safer than falling back silently.
    throw new ConfigError('Settings not loaded or invalid: apiUrl/opensearchUrl must be http(s) URLs')
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
  return isValidSettings(window._settings)
}
