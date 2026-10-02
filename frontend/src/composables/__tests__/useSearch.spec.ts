import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useSearch } from '@/composables/useSearch'

declare global {
  var _settings: { apiUrl: string; opensearchUrl: string }
}

beforeEach(() => {
  ;(window as unknown as { _settings: unknown })._settings = {
    apiUrl: 'http://api.test',
    opensearchUrl: ''
  }
  vi.restoreAllMocks()
})

describe('useSearch', () => {
  it('clears results when the term is empty', async () => {
    const s = useSearch()
    await s.search('')
    expect(s.results.value).toEqual([])
    expect(s.total.value).toBe(0)
    expect(s.hasSearched.value).toBe(false)
  })

  it('populates results on a successful response', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => ({
        hits: {
          hits: [{ _id: 'a', _source: { text: 'x' } }],
          total: { value: 1, relation: 'eq' }
        }
      })
    }) as unknown as typeof fetch

    const s = useSearch()
    await s.search('hello')
    expect(s.results.value).toHaveLength(1)
    expect(s.total.value).toBe(1)
    expect(s.hasSearched.value).toBe(true)
  })

  it('surfaces a user-facing error on failure', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 500,
      statusText: 'Server Error',
      headers: new Headers()
    }) as unknown as typeof fetch

    const s = useSearch()
    await s.search('boom')
    expect(s.error.value).toBeTruthy()
    expect(s.results.value).toEqual([])
  })
})

describe('search continuity', () => {
  it('keeps the newest result when requests finish out of order', async () => {
    let finishOld!: (value: unknown) => void
    const response = (id: string) => ({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => ({
        hits: { hits: [{ _id: id, _source: { text: id } }], total: { value: 1 } }
      })
    })
    globalThis.fetch = vi
      .fn()
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            finishOld = resolve
          })
      )
      .mockResolvedValueOnce(response('new'))
    const s = useSearch()
    const old = s.search('old')
    await s.search('new')
    finishOld(response('old'))
    await old
    expect(s.results.value[0]._id).toBe('new')
    expect(s.searchTerm.value).toBe('new')
  })

  it('supports filter-only pagination', async () => {
    globalThis.fetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => ({
        hits: { hits: [{ _id: 'a', _source: { text: 'x' } }], total: { value: 3 } },
        _scroll_id: 'next'
      })
    })
    const s = useSearch()
    await s.search('', { tags: ['invoice'] })
    await s.loadMore()
    expect(globalThis.fetch).toHaveBeenCalledTimes(2)
    expect(s.results.value).toHaveLength(2)
    await s.clearFilters()
    expect(s.results.value).toHaveLength(0)
    expect(s.hasSearched.value).toBe(false)
  })

  it('does not resurrect results after clearing an in-flight request', async () => {
    let finish!: (value: unknown) => void
    globalThis.fetch = vi.fn(
      () =>
        new Promise((resolve) => {
          finish = resolve
        })
    ) as typeof fetch
    const s = useSearch()
    const request = s.search('old')
    s.clear()
    finish({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => ({ hits: { hits: [{ _id: 'old' }], total: { value: 1 } } })
    })
    await request
    expect(s.results.value).toEqual([])
    expect(s.hasSearched.value).toBe(false)
  })
})
