import type { LocationQuery, LocationQueryRaw } from 'vue-router'
import type { SearchFilters } from '@/api/client'

export function parseArchiveQuery(query: LocationQuery) {
  const filters: SearchFilters = {}
  for (const key of ['companies', 'docTypes', 'tags'] as const) {
    const value = query[key]
    if (typeof value === 'string' && value) {
      if (value.startsWith('[')) {
        try {
          const parsed: unknown = JSON.parse(value)
          if (Array.isArray(parsed))
            filters[key] = parsed.filter(
              (item): item is string => typeof item === 'string' && !!item
            )
        } catch {
          /* Ignore malformed filter arrays. */
        }
      } else filters[key] = value.split(',').filter(Boolean)
    }
  }
  for (const key of ['dateFrom', 'dateTo'] as const) {
    if (typeof query[key] === 'string' && query[key]) filters[key] = query[key]
  }
  if (query.hasBarcode === 'true' || query.hasBarcode === 'false') {
    filters.hasBarcode = query.hasBarcode === 'true'
  }
  if (typeof query.title === 'string' && query.title) filters.titleFilter = query.title
  return { term: typeof query.q === 'string' ? query.q : '', filters }
}

export function archiveQuery(term: string, filters: SearchFilters = {}): LocationQueryRaw {
  const query: LocationQueryRaw = {}
  if (term.trim()) query.q = term.trim()
  for (const key of ['companies', 'docTypes', 'tags'] as const) {
    if (filters[key]?.length) {
      const values = filters[key]
      query[key] = values.some((value) => value.includes(',') || value.startsWith('['))
        ? JSON.stringify(values)
        : values.join(',')
    }
  }
  for (const key of ['dateFrom', 'dateTo'] as const) {
    if (filters[key]) query[key] = filters[key]
  }
  if (filters.hasBarcode !== undefined) query.hasBarcode = String(filters.hasBarcode)
  if (filters.titleFilter?.trim()) query.title = filters.titleFilter.trim()
  return query
}

export function hasArchiveSearch(term: string, filters: SearchFilters) {
  return Object.keys(archiveQuery(term, filters)).length > 0
}
