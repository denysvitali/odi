import { getApiUrl } from '@/lib/config'
import type { Document, DocumentDetails, SearchResult } from '@/types/documents'

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

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { retries = 2, retryDelayMs = 400, ...init } = options
  const base = getApiUrl()
  const url = path.startsWith('http') ? path : `${base}${path.startsWith('/') ? path : `/${path}`}`

  let lastError: Error | null = null
  for (let attempt = 0; attempt <= retries; attempt++) {
    try {
      const res = await fetch(url, init)
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
  listDocuments(params: { size?: number; scrollId?: string } = {}): Promise<SearchResult<Document>> {
    const qs = new URLSearchParams()
    if (params.size) qs.set('size', String(params.size))
    if (params.scrollId) qs.set('scroll_id', params.scrollId)
    return request(`/documents?${qs.toString()}`)
  },

  search(params: { searchTerm?: string; scrollId?: string; size?: number }): Promise<SearchResult<Document>> {
    return request('/search', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(params)
    })
  },

  async getDocumentDetails(id: string, { skipCache = false } = {}): Promise<DocumentDetails> {
    if (!skipCache && detailsCache.has(id)) return detailsCache.get(id)!
    if (detailsInflight.has(id)) return detailsInflight.get(id)!
    const p = request<DocumentDetails>(`/documents/${encodeURIComponent(id)}`)
      .then((d) => {
        detailsCache.set(id, d)
        return d
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
  }
}
