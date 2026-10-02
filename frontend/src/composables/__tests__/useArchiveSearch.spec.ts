import { effectScope } from 'vue'
import { describe, it, expect, vi, afterEach } from 'vitest'
import { api } from '@/api/client'
import { useArchiveSearch } from '../useArchiveSearch'
import type { Document, SearchResult } from '@/types/documents'

const response = (id: string): SearchResult<Document> => ({
  hits: {
    hits: [{ _id: id, _source: { text: 'Synthetic fixture' } }],
    total: { value: 2, relation: 'eq' }
  },
  _scroll_id: 'synthetic-scroll'
})
afterEach(() => vi.restoreAllMocks())
function setup() {
  const scope = effectScope()
  const search = scope.run(useArchiveSearch)!
  return { scope, search }
}

describe('archive search continuity', () => {
  it('searches filters alone and paginates without a text term', async () => {
    const mock = vi
      .spyOn(api, 'search')
      .mockResolvedValueOnce(response('first'))
      .mockResolvedValueOnce(response('second'))
    const { search, scope } = setup()
    await search.search('', { docTypes: ['invoice'] })
    expect(mock).toHaveBeenCalledWith({
      searchTerm: '',
      filters: { docTypes: ['invoice'] },
      size: 24
    })
    await search.loadMore()
    expect(search.results.value.map((d) => d._id)).toEqual(['first', 'second'])
    scope.stop()
  })
  it('discards stale search results and stale pagination after navigating', async () => {
    let finish!: (data: SearchResult<Document>) => void
    const mock = vi
      .spyOn(api, 'search')
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            finish = resolve
          })
      )
      .mockResolvedValueOnce(response('new'))
    const { search, scope } = setup()
    const old = search.search('old', {})
    await search.search('new', {})
    finish(response('old'))
    await old
    expect(search.results.value[0]._id).toBe('new')
    mock.mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finish = resolve
        })
    )
    const page = search.loadMore()
    await search.search('', {})
    finish(response('stale'))
    await page
    expect(search.results.value).toEqual([])
    expect(search.loadingMore.value).toBe(false)
    scope.stop()
  })
  it('retains results after a failed page, allows retry, and hides raw upstream errors', async () => {
    const mock = vi
      .spyOn(api, 'search')
      .mockResolvedValueOnce(response('first'))
      .mockRejectedValueOnce(new Error('sensitive upstream body'))
      .mockResolvedValueOnce(response('second'))
    const { search, scope } = setup()
    await search.search('sample', {})
    await search.loadMore()
    expect(search.results.value).toHaveLength(1)
    expect(search.error.value).not.toContain('sensitive')
    await search.loadMore()
    expect(mock).toHaveBeenCalledTimes(3)
    expect(search.results.value).toHaveLength(2)
    scope.stop()
  })
  it('ignores in-flight results after leaving the scope', async () => {
    let finish!: (data: SearchResult<Document>) => void
    vi.spyOn(api, 'search').mockImplementation(
      () =>
        new Promise((resolve) => {
          finish = resolve
        })
    )
    const { search, scope } = setup()
    const pending = search.search('sample', {})
    scope.stop()
    finish(response('late'))
    await pending
    expect(search.results.value).toEqual([])
  })
})
