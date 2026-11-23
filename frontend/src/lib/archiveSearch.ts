import type { SearchFilters } from '@/api/client'

const LIST_KEYS = ['companies', 'docTypes', 'tags'] as const
const DATE_KEYS = ['dateFrom', 'dateTo'] as const

/** Parse the search term and filters encoded in a URL query string. */
export function parseArchiveQuery(params: URLSearchParams) {
  const filters: SearchFilters = {}
  for (const key of LIST_KEYS) {
    const value = single(params, key)
    if (!value) continue
    if (value.startsWith('[')) {
      try {
        const parsed: unknown = JSON.parse(value)
        if (Array.isArray(parsed))
          filters[key] = parsed.filter((item): item is string => typeof item === 'string' && !!item)
      } catch {
        /* Ignore malformed filter arrays. */
      }
    } else filters[key] = value.split(',').filter(Boolean)
  }
  for (const key of DATE_KEYS) {
    const value = single(params, key)
    if (value) filters[key] = value
  }
  const barcode = single(params, 'hasBarcode')
  if (barcode === 'true' || barcode === 'false') filters.hasBarcode = barcode === 'true'
  const title = single(params, 'title')
  if (title) filters.titleFilter = title
  return { term: single(params, 'q') ?? '', filters }
}

/** Encode a search term and filters as URL query params (empty values are dropped). */
export function archiveQuery(term: string, filters: SearchFilters = {}): URLSearchParams {
  const query = new URLSearchParams()
  if (term.trim()) query.set('q', term.trim())
  for (const key of LIST_KEYS) {
    const values = filters[key]
    if (!values?.length) continue
    query.set(
      key,
      values.some((value) => value.includes(',') || value.startsWith('['))
        ? JSON.stringify(values)
        : values.join(',')
    )
  }
  for (const key of DATE_KEYS) {
    const value = filters[key]
    if (value) query.set(key, value)
  }
  if (filters.hasBarcode !== undefined) query.set('hasBarcode', String(filters.hasBarcode))
  if (filters.titleFilter?.trim()) query.set('title', filters.titleFilter.trim())
  return query
}

export function hasArchiveSearch(term: string, filters: SearchFilters) {
  return archiveQuery(term, filters).size > 0
}

export function countFilters(filters: SearchFilters) {
  return Object.values(filters).reduce<number>(
    (n, value) => n + (Array.isArray(value) ? value.length : value === undefined ? 0 : 1),
    0
  )
}

/** Repeated keys are treated as malformed rather than silently picking one. */
function single(params: URLSearchParams, key: string): string | undefined {
  const values = params.getAll(key)
  return values.length === 1 && values[0] ? values[0] : undefined
}
