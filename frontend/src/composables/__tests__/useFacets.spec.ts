import { afterEach, describe, expect, it, vi } from 'vitest'
import { effectScope, ref } from 'vue'
import { api, type SearchFilters } from '@/api/client'
import { useFacets } from '../useFacets'

afterEach(() => vi.restoreAllMocks())

describe('search facets', () => {
  it('fetches filter-only aggregations including document types and tags', async () => {
    const fetch = vi.spyOn(api, 'searchFacets').mockResolvedValue({
      hits: { total: { value: 2 } },
      aggregations: {
        docTypes: { buckets: [{ key: 'invoice', doc_count: 2 }] },
        tags: { buckets: [{ key: 'tax', doc_count: 1 }] }
      }
    } as unknown as Awaited<ReturnType<typeof api.searchFacets>>)
    const scope = effectScope()
    const state = scope.run(() => useFacets(ref(''), ref<SearchFilters>({ tags: ['tax'] })))!
    await state.fetchFacets()
    expect(fetch).toHaveBeenCalledWith({ searchTerm: '', filters: { tags: ['tax'] } })
    expect(state.facets.value.docTypes).toEqual([{ key: 'invoice', doc_count: 2 }])
    expect(state.facets.value.tags).toEqual([{ key: 'tax', doc_count: 1 }])
    scope.stop()
  })
})
