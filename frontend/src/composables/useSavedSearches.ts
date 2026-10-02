import { ref } from 'vue'
import type { SearchFilters } from '@/api/client'
import { archiveQuery, parseArchiveQuery, hasArchiveSearch } from '@/lib/archiveSearch'
import type { LocationQuery } from 'vue-router'

export interface SavedSearch {
  id: string
  name: string
  term: string
  filters: SearchFilters
}

const STORAGE_KEY = 'odi-saved-searches'
const LIMIT = 12

export function useSavedSearches() {
  const searches = ref<SavedSearch[]>([])
  const error = ref('')
  try {
    const raw: unknown = JSON.parse(localStorage.getItem(STORAGE_KEY) || '[]')
    if (Array.isArray(raw)) {
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
          const values: unknown = item.filters[key]
          if (Array.isArray(values))
            filters[key] = values.filter(
              (value): value is string => typeof value === 'string' && !!value.trim()
            )
        }
        for (const key of ['dateFrom', 'dateTo', 'titleFilter'] as const) {
          if (typeof item.filters[key] === 'string') filters[key] = item.filters[key]
        }
        if (typeof item.filters.hasBarcode === 'boolean')
          filters.hasBarcode = item.filters.hasBarcode
        if (hasArchiveSearch(item.term, filters))
          searches.value.push({
            id: item.id,
            name: item.name.slice(0, 80),
            term: item.term,
            filters
          })
        if (searches.value.length >= LIMIT) break
      }
    }
  } catch {
    error.value = 'Saved searches could not be read on this device.'
  }

  function persist(next: SavedSearch[]) {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(next))
      searches.value = next
      error.value = ''
      return true
    } catch {
      error.value = 'Browser storage is unavailable. This search was not saved.'
      return false
    }
  }

  function save(name: string, term: string, filters: SearchFilters) {
    if (!name.trim() || !hasArchiveSearch(term, filters)) return false
    const normalized = parseArchiveQuery(archiveQuery(term, filters) as LocationQuery)
    const signature = JSON.stringify(archiveQuery(normalized.term, normalized.filters))
    const existing = searches.value.find(
      (item) => JSON.stringify(archiveQuery(item.term, item.filters)) === signature
    )
    if (!existing && searches.value.length >= LIMIT) {
      error.value = 'You can save up to 12 searches. Remove one to make room.'
      return false
    }
    return persist([
      {
        id:
          existing?.id ||
          Array.from(crypto.getRandomValues(new Uint8Array(16)), (byte) =>
            byte.toString(16).padStart(2, '0')
          ).join(''),
        name: name.trim().slice(0, 80),
        ...normalized
      },
      ...searches.value.filter((item) => item.id !== existing?.id)
    ])
  }

  function remove(id: string) {
    return persist(searches.value.filter((item) => item.id !== id))
  }

  return { searches, error, save, remove }
}
