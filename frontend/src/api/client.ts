import { getApiUrl } from '@/lib/config'
import { STORAGE_KEYS } from '@/lib/constants'
import { logger } from '@/lib/logger'
import type { Document, DocumentDetails, SearchResult } from '@/types/documents'

export interface SearchFilters {
  companies?: string[]
  dateFrom?: string
  dateTo?: string
  hasBarcode?: boolean
  titleFilter?: string
  docTypes?: string[]
  tags?: string[]
}

export interface FacetBucket {
  key: string
  doc_count: number
}

export interface FacetData {
  companies: FacetBucket[]
  dateHistogram: FacetBucket[]
  barcodeCount: number
  totalHits: number
  docTypes?: FacetBucket[]
  tags?: FacetBucket[]
}

export interface ChatRequest {
  question: string
  filters?: SearchFilters
}

export interface ChatResponse {
  answer: string
  citations: string[]
}

export interface KeyFact {
  label: string
  value: string
}

export interface DocumentSummaryResult {
  summary: string
  keyFacts: KeyFact[]
}

export interface CreateShareOptions {
  scanID: string
  sequenceID: number
  expiresInHours?: number
  maxViews?: number
  passphrase?: string
}

export interface CreateShareResult {
  token: string
}

export interface Share {
  token: string
  scanID: string
  sequenceID: number
  expiresAt: number
  maxViews: number
  viewCount: number
  hasPassphrase: boolean
}

export interface ReindexPageError {
  page: string
  error: string
}

export interface ReindexStatus {
  state: 'idle' | 'running' | 'completed' | 'failed'
  startedAt?: string
  finishedAt?: string
  total: number
  processed: number
  duplicates: number
  failed: number
  currentPage?: string
  recentErrors?: ReindexPageError[]
  error?: string
}

export class ApiError extends Error {
  status: number
  retryable: boolean
  code: string
  constructor(message: string, status: number, code = 'API_ERROR') {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.retryable = status === 0 || status >= 500 || status === 408 || status === 429
  }
}

interface RequestOptions extends RequestInit {
  retries?: number
  retryDelayMs?: number
  skipCache?: boolean
}

function getApiToken(): string | null {
  try {
    return localStorage.getItem(STORAGE_KEYS.API_TOKEN)
  } catch (err) {
    logger.warn('api: failed to read API token from localStorage', err)
    return null
  }
}

function buildHeaders(init: RequestInit | undefined): HeadersInit | undefined {
  const token = getApiToken()
  if (!token) return init?.headers
  // Merge existing headers with the Authorization header so callers can still
  // set Content-Type, Accept, etc. without losing the bearer token.
  const headers = new Headers(init?.headers ?? {})
  if (!headers.has('Authorization')) {
    headers.set('Authorization', `Bearer ${token}`)
  }
  return headers
}

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { retries = 2, retryDelayMs = 400, ...init } = options
  const base = getApiUrl()
  const url = path.startsWith('http') ? path : `${base}${path.startsWith('/') ? path : `/${path}`}`

  let lastError: Error | null = null
  for (let attempt = 0; attempt <= retries; attempt++) {
    try {
      const res = await fetch(url, { ...init, headers: buildHeaders(init) })
      if (!res.ok) {
        const err = new ApiError(
          `Request failed: ${res.status} ${res.statusText}`,
          res.status
        )
        if (!err.retryable || attempt === retries) throw err
        lastError = err
      } else {
        if (res.status === 204) return undefined as unknown as T
        const ct = res.headers.get('content-type') || ''
        if (ct.includes('application/json')) return (await res.json()) as T
        return (await res.text()) as unknown as T
      }
    } catch (err) {
      if (err instanceof ApiError && !err.retryable) throw err
      lastError = err as Error
      if (attempt === retries) break
    }
    await new Promise((r) => setTimeout(r, retryDelayMs * Math.pow(2, attempt)))
  }
  if (lastError instanceof ApiError) throw lastError
  throw new ApiError(lastError?.message || 'Network error', 0, 'NETWORK_ERROR')
}

const detailsCache = new Map<string, DocumentDetails>()
const detailsInflight = new Map<string, Promise<DocumentDetails>>()

export const api = {
  listDocuments(params: { size?: number; scrollId?: string; dateFrom?: string; dateTo?: string } = {}): Promise<SearchResult<Document>> {
    const qs = new URLSearchParams()
    if (params.size) qs.set('size', String(params.size))
    if (params.scrollId) qs.set('scroll_id', params.scrollId)
    if (params.dateFrom) qs.set('date_from', params.dateFrom)
    if (params.dateTo) qs.set('date_to', params.dateTo)
    return request(`/documents?${qs.toString()}`)
  },

  search(params: { searchTerm?: string; scrollId?: string; size?: number; filters?: SearchFilters }): Promise<SearchResult<Document>> {
    // Flatten filters to top-level fields to match backend SearchRequest struct
    const body: Record<string, unknown> = {
      searchTerm: params.searchTerm,
      scrollId: params.scrollId,
      size: params.size,
    }
    if (params.filters) {
      if (params.filters.companies?.length) body.companies = params.filters.companies
      if (params.filters.dateFrom) body.dateFrom = params.filters.dateFrom
      if (params.filters.dateTo) body.dateTo = params.filters.dateTo
      if (params.filters.hasBarcode !== undefined) body.hasBarcode = params.filters.hasBarcode
      if (params.filters.titleFilter) body.title = params.filters.titleFilter
      if (params.filters.docTypes?.length) body.docTypes = params.filters.docTypes
      if (params.filters.tags?.length) body.tags = params.filters.tags
    }
    return request('/search', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    })
  },

  searchFacets(params: { searchTerm?: string; filters?: SearchFilters }): Promise<FacetData> {
    // Flatten filters to top-level fields to match backend SearchFacetsRequest struct
    const body: Record<string, unknown> = {
      searchTerm: params.searchTerm,
    }
    if (params.filters) {
      if (params.filters.companies?.length) body.companies = params.filters.companies
      if (params.filters.dateFrom) body.dateFrom = params.filters.dateFrom
      if (params.filters.dateTo) body.dateTo = params.filters.dateTo
      if (params.filters.hasBarcode !== undefined) body.hasBarcode = params.filters.hasBarcode
      if (params.filters.titleFilter) body.title = params.filters.titleFilter
      if (params.filters.docTypes?.length) body.docTypes = params.filters.docTypes
      if (params.filters.tags?.length) body.tags = params.filters.tags
    }
    return request('/search/facets', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    })
  },

  async getDocumentDetails(id: string, { skipCache = false } = {}): Promise<DocumentDetails> {
    if (!skipCache && detailsCache.has(id)) return detailsCache.get(id)!
    const existing = detailsInflight.get(id)
    if (existing) return existing
    // On error we eagerly drop the inflight entry *before* re-throwing so the
    // next caller can issue a fresh fetch instead of awaiting the rejected
    // promise (which would otherwise be cached forever via the Map).
    const p = request<DocumentDetails>(`/documents/${encodeURIComponent(id)}`)
      .then((d) => {
        detailsCache.set(id, d)
        return d
      })
      .catch((err) => {
        detailsInflight.delete(id)
        throw err
      })
      .finally(() => detailsInflight.delete(id))
    detailsInflight.set(id, p)
    return p
  },

  invalidateDocument(id: string): void {
    detailsCache.delete(id)
  },

  clearCaches(): void {
    detailsCache.clear()
  },

  thumbnailUrl(id: string): string {
    return `${getApiUrl()}/thumbnails/${encodeURIComponent(id)}`
  },

  fileUrl(id: string): string {
    return `${getApiUrl()}/files/${encodeURIComponent(id).replace(/_/g, '/')}`
  },

  getReindexStatus(): Promise<ReindexStatus> {
    return request('/admin/reindex')
  },

  startReindex(): Promise<ReindexStatus> {
    return request('/admin/reindex', { method: 'POST' })
  },

  chat(params: ChatRequest): Promise<ChatResponse> {
    const body: Record<string, unknown> = { question: params.question }
    if (params.filters) {
      const f = params.filters
      if (f.companies?.length) body.companies = f.companies
      if (f.dateFrom) body.dateFrom = f.dateFrom
      if (f.dateTo) body.dateTo = f.dateTo
      if (f.hasBarcode !== undefined) body.hasBarcode = f.hasBarcode
      if (f.titleFilter) body.title = f.titleFilter
      if (f.docTypes?.length) body.docTypes = f.docTypes
      if (f.tags?.length) body.tags = f.tags
    }
    return request('/chat', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body)
    })
  },

  summarizeDocument(id: string): Promise<DocumentSummaryResult> {
    return request(`/documents/${encodeURIComponent(id)}/summary`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' }
    })
  },

  createShare(opts: CreateShareOptions): Promise<CreateShareResult> {
    return request('/shares', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(opts)
    })
  },

  listShares(): Promise<Share[]> {
    return request('/shares')
  },

  revokeShare(token: string): Promise<void> {
    return request(`/shares/${encodeURIComponent(token)}`, { method: 'DELETE' })
  },

  // The public, unauthenticated share page lives at the server root (NOT under
  // /api/v1). Derive the server origin by stripping the API path segment from
  // getApiUrl(), then append /share/<token>.
  shareUrl(token: string): string {
    const apiUrl = getApiUrl()
    const root = apiUrl.replace(/\/api\/v\d+\/?$/, '').replace(/\/$/, '')
    return `${root}/share/${encodeURIComponent(token)}`
  }
}
