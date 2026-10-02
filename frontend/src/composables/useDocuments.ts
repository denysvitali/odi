import { ref, computed } from 'vue'
import { api } from '@/api/client'
import { errorMessage } from '@/lib/utils'
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

  let generation = 0

  const filteredDocuments = computed(() => documents.value)

  const filteredTotal = computed(() => total.value)

  const hasMore = computed(() => {
    if (total.value === 0) return false
    return documents.value.length < total.value
  })

  const loadDocuments = async () => {
    const current = ++generation
    loadingMore.value = false
    loading.value = true
    error.value = null
    try {
      const data = await api.listDocuments({
        size: pageSize.value,
        dateFrom: dateRange.value?.from,
        dateTo: dateRange.value?.to
      })
      if (current !== generation) return
      documents.value = []
      total.value = 0
      scrollId.value = null
      if (data.hits) {
        documents.value = data.hits.hits
        total.value = data.hits.total?.value || 0
        scrollId.value = data._scroll_id || null
      }
    } catch (err) {
      if (current === generation) error.value = errorMessage(err, 'Failed to load documents')
    } finally {
      if (current === generation) loading.value = false
    }
  }

  const loadMore = async () => {
    if (loading.value || loadingMore.value || !scrollId.value) return
    const current = generation
    loadingMore.value = true
    try {
      const data = await api.listDocuments({ scrollId: scrollId.value, size: pageSize.value })
      if (current !== generation) return
      if (data.hits?.hits) {
        documents.value.push(...data.hits.hits)
        scrollId.value = data._scroll_id || null
      }
    } catch (err) {
      if (current === generation) error.value = errorMessage(err, 'Unable to load more documents')
    } finally {
      if (current === generation) loadingMore.value = false
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
