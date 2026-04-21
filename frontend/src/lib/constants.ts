export const STORAGE_KEYS = {
  THEME: 'odi-theme',
  RECENT_SEARCHES: 'odi-recent-searches',
  FAVORITES: 'odi-favorites',
  TAGS: 'odi-tags',
  LOCALE: 'odi-locale'
} as const

export function getLocale(): string {
  try {
    const stored = localStorage.getItem(STORAGE_KEYS.LOCALE)
    if (stored) return stored
  } catch {}
  if (typeof navigator !== 'undefined' && navigator.language) return navigator.language
  return 'en-US'
}

export const DEFAULT_CURRENCY = 'CHF'
