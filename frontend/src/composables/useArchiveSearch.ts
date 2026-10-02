import { ref, onScopeDispose } from 'vue'
import { api, type SearchFilters } from '@/api/client'
import type { Document } from '@/types/documents'
import { hasArchiveSearch } from '@/lib/archiveSearch'

export function useArchiveSearch() {
  const results = ref<Document[]>([])
  const total = ref(0)
  const loading = ref(false)
  const loadingMore = ref(false)
  const error = ref('')
  const scrollId = ref('')
  let generation = 0

  async function search(term: string, filters: SearchFilters) {
    const current = ++generation
    results.value = []
    total.value = 0
    scrollId.value = ''
    error.value = ''
    loadingMore.value = false
    loading.value = hasArchiveSearch(term, filters)
    if (!loading.value) return
    try {
      const data = await api.search({ searchTerm: term, filters, size: 24 })
      if (current !== generation) return
      results.value = data.hits?.hits || []
      total.value = data.hits?.total?.value || 0
      scrollId.value = data._scroll_id || ''
    } catch {
      if (current === generation)
        error.value = 'Search could not be loaded. Check your connection and try again.'
    } finally {
      if (current === generation) loading.value = false
    }
  }

  async function loadMore() {
    if (!scrollId.value || loading.value || loadingMore.value) return
    const current = generation
    loadingMore.value = true
    error.value = ''
    try {
      const data = await api.search({ scrollId: scrollId.value })
      if (current !== generation) return
      results.value.push(...(data.hits?.hits || []))
      scrollId.value = data._scroll_id || ''
    } catch {
      if (current === generation) error.value = 'More results could not be loaded. Try again.'
    } finally {
      if (current === generation) loadingMore.value = false
    }
  }

  onScopeDispose(() => {
    generation++
  })
  return { results, total, loading, loadingMore, error, scrollId, search, loadMore }
}
