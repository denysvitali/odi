import { useSyncExternalStore } from 'react'
import { logger } from '@/lib/logger'

export interface PersistentStore<T> {
  get(): T
  /** Persists and publishes `next`; returns false (and keeps the old value) if storage fails. */
  set(next: T): boolean
  use(): T
  /** Re-read from storage (tests, cross-tab updates). */
  reload(): void
}

/**
 * A tiny localStorage-backed store. `parse` must validate untrusted storage
 * content and return a safe value; it may throw, in which case `fallback` is used.
 */
export function persistentStore<T>(key: string, parse: (raw: unknown) => T, fallback: T) {
  const listeners = new Set<() => void>()
  let value: T | undefined

  const read = (): T => {
    try {
      const raw = localStorage.getItem(key)
      return raw === null ? fallback : parse(JSON.parse(raw))
    } catch (err) {
      logger.warn(`store ${key}: unreadable, using defaults`, err)
      return fallback
    }
  }
  const publish = (next: T) => {
    value = next
    listeners.forEach((l) => l())
  }

  const store: PersistentStore<T> = {
    get: () => (value === undefined ? (value = read()) : value),
    set(next) {
      try {
        localStorage.setItem(key, JSON.stringify(next))
      } catch (err) {
        logger.warn(`store ${key}: failed to persist`, err)
        return false
      }
      publish(next)
      return true
    },
    use: () => useSyncExternalStore(subscribe, store.get),
    reload: () => publish(read())
  }

  function subscribe(listener: () => void) {
    listeners.add(listener)
    const onStorage = (e: StorageEvent) => e.key === key && store.reload()
    window.addEventListener('storage', onStorage)
    return () => {
      listeners.delete(listener)
      window.removeEventListener('storage', onStorage)
    }
  }

  return store
}

export const stringArray = (raw: unknown): string[] =>
  Array.isArray(raw) ? raw.filter((v): v is string => typeof v === 'string' && !!v.trim()) : []
