import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useDocuments } from '@/composables/useDocuments'

declare global {
  // eslint-disable-next-line no-var
  var _settings: { apiUrl: string; opensearchUrl: string }
}

beforeEach(() => {
  ;(window as unknown as { _settings: unknown })._settings = {
    apiUrl: 'http://api.test',
    opensearchUrl: ''
  }
  vi.restoreAllMocks()
})

const mockDocument = (overrides = {}) => ({
  _id: 'doc-1',
  _source: {
    text: 'Test document',
    date: '2024-01-15T10:00:00Z',
    indexedAt: '2024-01-20T12:00:00Z'
  },
  ...overrides
})

describe('useDocuments', () => {
  it('loads documents successfully', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => ({
        hits: {
          hits: [mockDocument()],
          total: { value: 1, relation: 'eq' }
        },
        _scroll_id: 'scroll-1'
      })
    }) as unknown as typeof fetch

    const docs = useDocuments()
    await docs.loadDocuments()

    expect(docs.documents.value).toHaveLength(1)
    expect(docs.total.value).toBe(1)
    expect(docs.loading.value).toBe(false)
    expect(docs.error.value).toBeNull()
  })

  it('surfaces error on failure', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      statusText: 'Server Error',
      headers: new Headers()
    }) as unknown as typeof fetch

    const docs = useDocuments()
    await docs.loadDocuments()

    expect(docs.error.value).toBeTruthy()
    expect(docs.documents.value).toEqual([])
  })

  it('passes date range to API when set', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => ({
        hits: { hits: [], total: { value: 0, relation: 'eq' } }
      })
    }) as unknown as typeof fetch
    globalThis.fetch = mockFetch

    const docs = useDocuments()
    docs.dateRange.value = { from: '2024-01-01', to: '2024-01-31' }
    await docs.loadDocuments()

    const callArgs = (mockFetch as ReturnType<typeof vi.fn>).mock.calls[0]
    const url = callArgs[0] as string
    expect(url).toContain('date_from=2024-01-01')
    expect(url).toContain('date_to=2024-01-31')
  })

  it('does not include date params when dateRange is null', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => ({
        hits: { hits: [], total: { value: 0, relation: 'eq' } }
      })
    }) as unknown as typeof fetch
    globalThis.fetch = mockFetch

    const docs = useDocuments()
    expect(docs.dateRange.value).toBeNull()
    await docs.loadDocuments()

    const callArgs = (mockFetch as ReturnType<typeof vi.fn>).mock.calls[0]
    const url = callArgs[0] as string
    expect(url).not.toContain('date_from')
    expect(url).not.toContain('date_to')
  })

  it('refresh calls loadDocuments', async () => {
    const mockFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => ({
        hits: { hits: [mockDocument()], total: { value: 1, relation: 'eq' } },
        _scroll_id: 'scroll-new'
      })
    }) as unknown as typeof fetch
    globalThis.fetch = mockFetch

    const docs = useDocuments()
    await docs.refresh()

    expect(mockFetch).toHaveBeenCalled()
    expect(docs.documents.value).toHaveLength(1)
  })

  it('hasMore is computed based on loaded count vs total', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => ({
        hits: {
          hits: Array(5).fill(null).map((_, i) => mockDocument({ _id: `doc-${i}` })),
          total: { value: 10, relation: 'eq' }
        },
        _scroll_id: 'scroll-1'
      })
    }) as unknown as typeof fetch

    const docs = useDocuments()
    await docs.loadDocuments()

    expect(docs.hasMore.value).toBe(true)
    expect(docs.total.value).toBe(10)
  })
})