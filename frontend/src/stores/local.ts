import { STORAGE_KEYS } from '@/lib/constants'
import { persistentStore, stringArray } from './persistent'

// ── Favorites ────────────────────────────────────────────────────────────────
export const favorites = persistentStore(STORAGE_KEYS.FAVORITES, stringArray, [])

export function toggleFavorite(id: string) {
  const current = favorites.get()
  favorites.set(current.includes(id) ? current.filter((f) => f !== id) : [...current, id])
}

export function useIsFavorite(id: string) {
  return favorites.use().includes(id)
}

// ── Tags (device-local labels per document) ──────────────────────────────────
type TagMap = Record<string, string[]>

export const tags = persistentStore<TagMap>(
  STORAGE_KEYS.TAGS,
  (raw) => {
    if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return {}
    const out: TagMap = {}
    for (const [id, list] of Object.entries(raw)) {
      const clean = stringArray(list)
      if (clean.length) out[id] = clean
    }
    return out
  },
  {}
)

export function addTag(docId: string, tag: string) {
  const clean = tag.trim()
  const map = tags.get()
  const current = map[docId] || []
  if (!clean || current.includes(clean)) return
  tags.set({ ...map, [docId]: [...current, clean] })
}

export function removeTag(docId: string, tag: string) {
  const map = tags.get()
  const next = (map[docId] || []).filter((t) => t !== tag)
  const rest = { ...map }
  delete rest[docId]
  tags.set(next.length ? { ...rest, [docId]: next } : rest)
}

export function allTags(map: TagMap): string[] {
  return [...new Set(Object.values(map).flat())].sort()
}

// ── Recent searches ──────────────────────────────────────────────────────────
const RECENT_LIMIT = 10
export const recentSearches = persistentStore(STORAGE_KEYS.RECENT_SEARCHES, stringArray, [])

export function addRecentSearch(term: string) {
  const clean = term.trim()
  if (!clean) return
  recentSearches.set(
    [clean, ...recentSearches.get().filter((t) => t !== clean)].slice(0, RECENT_LIMIT)
  )
}

// ── Document list view preference ────────────────────────────────────────────
export type DocumentView = 'list' | 'grid'
export const documentView = persistentStore<DocumentView>(
  STORAGE_KEYS.DOCUMENT_VIEW,
  (raw) => (raw === 'grid' ? 'grid' : 'list'),
  'list'
)
