import { logger } from '@/lib/logger'

export const STORAGE_KEYS = {
  THEME: 'odi-theme',
  RECENT_SEARCHES: 'odi-recent-searches',
  FAVORITES: 'odi-favorites',
  TAGS: 'odi-tags',
  LOCALE: 'odi-locale',
  API_TOKEN: 'odi.apiToken'
} as const

export function getLocale(): string {
  try {
    const stored = localStorage.getItem(STORAGE_KEYS.LOCALE)
    if (stored) return stored
  } catch (err) {
    logger.warn('constants: failed to read locale from localStorage', err)
  }
  if (typeof navigator !== 'undefined' && navigator.language) return navigator.language
  return 'en-US'
}

export const DEFAULT_CURRENCY = 'CHF'
