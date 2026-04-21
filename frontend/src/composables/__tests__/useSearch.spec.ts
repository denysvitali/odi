import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useSearch } from '@/composables/useSearch'

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
