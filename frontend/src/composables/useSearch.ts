import { ref, computed, getCurrentScope, onScopeDispose } from 'vue'
import { api } from '@/api/client'
import { errorMessage } from '@/lib/utils'
import type { Document } from '@/types/documents'
import type { SearchFilters } from '@/api/client'

export interface UseSearchOptions {
  debounceMs?: number
  pageSize?: number
}

export function useSearch(options: UseSearchOptions = {}) {
  const { debounceMs = 300, pageSize = 100 } = options

  const searchTerm = ref('')
  const results = ref<Document[]>([])
  const loading = ref(false)
  const loadingMore = ref(false)
  const error = ref<string | null>(null)
  const total = ref<number>(0)
  const scrollId = ref<string | null>(null)
  const hasSearched = ref(false)
  const activeFilters = ref<SearchFilters>({})

  let generation = 0
  let debounceTimeout: ReturnType<typeof setTimeout> | null = null

  const activeFilterCount = computed(() => {
    let count = 0
    if (activeFilters.value.companies?.length) count += activeFilters.value.companies.length
    if (activeFilters.value.dateFrom) count++
    if (activeFilters.value.dateTo) count++
    if (activeFilters.value.hasBarcode !== undefined) count++
    if (activeFilters.value.titleFilter?.trim()) count++
    count += activeFilters.value.docTypes?.length || 0
    count += activeFilters.value.tags?.length || 0
    return count
  })

  const search = async (term: string, filters?: SearchFilters) => {
    if (debounceTimeout) {
      clearTimeout(debounceTimeout)
      debounceTimeout = null
    }
    const current = ++generation
    loadingMore.value = false
    scrollId.value = null
    searchTerm.value = term

    if (filters !== undefined) {
      activeFilters.value = filters
    }

    if (!term.trim() && activeFilterCount.value === 0) {
      results.value = []
      total.value = 0
      hasSearched.value = false
      loading.value = false
      error.value = null
      return
    }

    loading.value = true
    error.value = null
    hasSearched.value = true

    try {
      const data = await api.search({
        searchTerm: term,
        size: pageSize,
        filters: activeFilterCount.value > 0 ? activeFilters.value : undefined
      })
      if (current !== generation) return
      if (data.hits) {
        results.value = data.hits.hits
        total.value = data.hits.total?.value || 0
        scrollId.value = data._scroll_id || null
      } else {
        results.value = []
        total.value = 0
      }
    } catch (err) {
      if (current !== generation) return
      error.value = errorMessage(err, 'Search failed')
      results.value = []
      total.value = 0
    } finally {
      if (current === generation) loading.value = false
    }
  }

  const debouncedSearch = (term: string, filters?: SearchFilters) => {
    if (debounceTimeout) clearTimeout(debounceTimeout)
    debounceTimeout = setTimeout(() => search(term, filters), debounceMs)
  }

  const loadMore = async () => {
    if (!scrollId.value || loading.value || loadingMore.value) return
    const current = generation
    loadingMore.value = true
    try {
      const data = await api.search({ scrollId: scrollId.value })
      if (current !== generation) return
      if (data.hits?.hits) {
        results.value.push(...data.hits.hits)
        scrollId.value = data._scroll_id || null
      }
    } catch (err) {
      if (current === generation) error.value = errorMessage(err, 'Unable to load more results')
    } finally {
      if (current === generation) loadingMore.value = false
    }
  }

  const clear = () => {
    generation++
    if (debounceTimeout) clearTimeout(debounceTimeout)
    debounceTimeout = null
    loading.value = false
    loadingMore.value = false
    error.value = null
    searchTerm.value = ''
    results.value = []
    total.value = 0
    hasSearched.value = false
    scrollId.value = null
    activeFilters.value = {}
  }

  const clearFilters = () => {
    activeFilters.value = {}
    return search(searchTerm.value)
  }

  if (getCurrentScope()) onScopeDispose(clear)

  return {
    searchTerm,
    results,
    loading,
    loadingMore,
    error,
    total,
    hasSearched,
    activeFilters,
    activeFilterCount,
    search,
    debouncedSearch,
    loadMore,
    clear,
    clearFilters
  }
}
