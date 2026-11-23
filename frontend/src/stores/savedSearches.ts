import type { SearchFilters } from '@/api/client'
import { archiveQuery, hasArchiveSearch, parseArchiveQuery } from '@/lib/archiveSearch'
import { STORAGE_KEYS } from '@/lib/constants'
import { persistentStore, stringArray } from './persistent'

export interface SavedSearch {
  id: string
  name: string
  term: string
  filters: SearchFilters
}

export const SAVED_SEARCH_LIMIT = 12
const NAME_LIMIT = 80

function sanitize(raw: unknown): SavedSearch[] {
  if (!Array.isArray(raw)) throw new Error('saved searches must be an array')
  const out: SavedSearch[] = []
  for (const item of raw) {
    if (
      !item ||
      typeof item !== 'object' ||
      typeof item.id !== 'string' ||
      typeof item.name !== 'string' ||
      !item.name.trim() ||
      typeof item.term !== 'string' ||
      !item.filters ||
      typeof item.filters !== 'object'
    )
      continue
    const filters: SearchFilters = {}
    for (const key of ['companies', 'docTypes', 'tags'] as const) {
      if (Array.isArray(item.filters[key])) filters[key] = stringArray(item.filters[key])
    }
    for (const key of ['dateFrom', 'dateTo', 'titleFilter'] as const) {
      if (typeof item.filters[key] === 'string') filters[key] = item.filters[key]
    }
    if (typeof item.filters.hasBarcode === 'boolean') filters.hasBarcode = item.filters.hasBarcode
    if (hasArchiveSearch(item.term, filters))
      out.push({ id: item.id, name: item.name.slice(0, NAME_LIMIT), term: item.term, filters })
    if (out.length >= SAVED_SEARCH_LIMIT) break
  }
  return out
}

export const savedSearches = persistentStore(STORAGE_KEYS.SAVED_SEARCHES, sanitize, [])

export type SaveResult = { ok: true } | { ok: false; error: string }
const STORAGE_ERROR = 'Browser storage is unavailable. Your changes were not saved.'

const signature = (term: string, filters: SearchFilters) => archiveQuery(term, filters).toString()

export function saveSearch(name: string, term: string, filters: SearchFilters): SaveResult {
  if (!name.trim()) return { ok: false, error: 'Enter a name for this search.' }
  if (!hasArchiveSearch(term, filters)) return { ok: false, error: 'There is nothing to save yet.' }
  const normalized = parseArchiveQuery(archiveQuery(term, filters))
  const sig = signature(normalized.term, normalized.filters)
  const current = savedSearches.get()
  const existing = current.find((s) => signature(s.term, s.filters) === sig)
  if (!existing && current.length >= SAVED_SEARCH_LIMIT)
    return {
      ok: false,
      error: `You can save up to ${SAVED_SEARCH_LIMIT} searches. Remove one first.`
    }
  const entry: SavedSearch = {
    id: existing?.id || crypto.randomUUID(),
    name: name.trim().slice(0, NAME_LIMIT),
    ...normalized
  }
  const ok = savedSearches.set([entry, ...current.filter((s) => s.id !== existing?.id)])
  return ok ? { ok } : { ok, error: STORAGE_ERROR }
}

export function renameSearch(id: string, name: string): SaveResult {
  const trimmed = name.trim()
  if (!trimmed) return { ok: false, error: 'Enter a name for this saved search.' }
  const current = savedSearches.get()
  if (!current.some((s) => s.id === id))
    return { ok: false, error: 'This saved search is no longer available.' }
  const ok = savedSearches.set(
    current.map((s) => (s.id === id ? { ...s, name: trimmed.slice(0, NAME_LIMIT) } : s))
  )
  return ok ? { ok } : { ok, error: STORAGE_ERROR }
}

export function removeSearch(id: string): SaveResult {
  const ok = savedSearches.set(savedSearches.get().filter((s) => s.id !== id))
  return ok ? { ok } : { ok, error: STORAGE_ERROR }
}
