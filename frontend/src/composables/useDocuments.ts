import { ref, computed } from 'vue'
import { api, ApiError } from '@/api/client'
import type { Document } from '@/types/documents'

export interface UseDocumentsOptions {
  initialPageSize?: number
}

export interface DateRange {
  from: string
  to: string
}

export function useDocuments(options: UseDocumentsOptions = {}) {
  const { initialPageSize = 12 } = options

  const documents = ref<Document[]>([])
  const loading = ref(false)
  const loadingMore = ref(false)
  const error = ref<string | null>(null)
  const scrollId = ref<string | null>(null)
  const total = ref<number>(0)
  const pageSize = ref(initialPageSize)
  const dateRange = ref<DateRange | null>(null)

  const filteredDocuments = computed(() => documents.value)

  const filteredTotal = computed(() => filteredDocuments.value.length)

  const hasMore = computed(() => {
    if (total.value === 0) return false
    return documents.value.length < total.value
  })

  const loadDocuments = async () => {
    if (loading.value) return
    loading.value = true
    error.value = null
    try {
      const data = await api.listDocuments({
        size: pageSize.value,
        dateFrom: dateRange.value?.from,
        dateTo: dateRange.value?.to
      })
      if (data.hits) {
        documents.value = data.hits.hits
        total.value = data.hits.total?.value || 0
        scrollId.value = data._scroll_id || null
      }
    } catch (err) {
      error.value = err instanceof ApiError ? err.message : err instanceof Error ? err.message : 'Failed to load documents'
    } finally {
      loading.value = false
    }
  }

  const loadMore = async () => {
    if (loadingMore.value || !scrollId.value) return
    loadingMore.value = true
    try {
      const data = await api.listDocuments({ scrollId: scrollId.value, size: pageSize.value })
      if (data.hits?.hits) {
        documents.value.push(...data.hits.hits)
        scrollId.value = data._scroll_id || null
      }
    } catch (err) {
      // Non-fatal; keep previous results
      console.error('Error loading more documents:', err)
    } finally {
      loadingMore.value = false
    }
  }

  const refresh = () => {
    scrollId.value = null
    documents.value = []
    total.value = 0
    return loadDocuments()
  }

  return {
    documents: filteredDocuments,
    loading,
    loadingMore,
    error,
    total: filteredTotal,
    hasMore,
    dateRange,
    loadDocuments,
    loadMore,
    refresh
  }
}
