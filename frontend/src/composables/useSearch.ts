import { ref } from 'vue'
import { api, ApiError } from '@/api/client'
import type { Document } from '@/types/documents'

export interface UseSearchOptions {
  debounceMs?: number
  pageSize?: number
}

export function useSearch(options: UseSearchOptions = {}) {
  const { debounceMs = 300, pageSize = 12 } = options

  const searchTerm = ref('')
  const results = ref<Document[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)
  const total = ref<number>(0)
  const scrollId = ref<string | null>(null)
  const hasSearched = ref(false)

  let debounceTimeout: ReturnType<typeof setTimeout> | null = null

  const search = async (term: string) => {
    if (debounceTimeout) {
      clearTimeout(debounceTimeout)
      debounceTimeout = null
    }
    searchTerm.value = term

    if (!term.trim()) {
      results.value = []
      total.value = 0
      hasSearched.value = false
      return
    }

    loading.value = true
    error.value = null
    hasSearched.value = true

    try {
      const data = await api.search({ searchTerm: term, size: pageSize })
      if (data.hits) {
        results.value = data.hits.hits
        total.value = data.hits.total?.value || 0
        scrollId.value = data._scroll_id || null
      } else {
        results.value = []
        total.value = 0
      }
    } catch (err) {
      error.value = err instanceof ApiError ? err.message : err instanceof Error ? err.message : 'Search failed'
      results.value = []
      total.value = 0
    } finally {
      loading.value = false
    }
  }

  const debouncedSearch = (term: string) => {
    if (debounceTimeout) clearTimeout(debounceTimeout)
    debounceTimeout = setTimeout(() => search(term), debounceMs)
  }

  const loadMore = async () => {
    if (!scrollId.value || !searchTerm.value) return
    loading.value = true
    try {
      const data = await api.search({ scrollId: scrollId.value })
      if (data.hits?.hits) {
        results.value.push(...data.hits.hits)
        scrollId.value = data._scroll_id || null
      }
    } catch (err) {
      console.error('Error loading more results:', err)
    } finally {
      loading.value = false
    }
  }

  const clear = () => {
    searchTerm.value = ''
    results.value = []
    total.value = 0
    hasSearched.value = false
    scrollId.value = null
  }

  return {
    searchTerm,
    results,
    loading,
    error,
    total,
    hasSearched,
    search,
    debouncedSearch,
    loadMore,
    clear
  }
}
