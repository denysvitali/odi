import { ref, computed } from 'vue'
import type { Document, SearchResult } from '@/types/documents'

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

  const filteredDocuments = computed(() => {
    if (!dateRange.value) return documents.value

    const { from, to } = dateRange.value
    const fromDate = from ? new Date(from) : null
    const toDate = to ? new Date(to + 'T23:59:59') : null

    return documents.value.filter((doc) => {
      const docDate = doc._source?.primaryDate || doc._source?.indexedAt
      if (!docDate) return true

      const docDateObj = new Date(docDate)
      if (fromDate && docDateObj < fromDate) return false
      if (toDate && docDateObj > toDate) return false
      return true
    })
  })

  const filteredTotal = computed(() => filteredDocuments.value.length)

  const hasMore = computed(() => {
    if (total.value === 0) return false
    return documents.value.length < total.value
  })

  const apiUrl = computed(() => window._settings?.apiUrl || '')

  const loadDocuments = async () => {
    if (loading.value) return

    loading.value = true
    error.value = null

    try {
      const url = new URL(`${apiUrl.value}/documents`)
      url.searchParams.set('size', String(pageSize.value))

      const response = await fetch(url.toString())

      if (!response.ok) {
        throw new Error(`Failed to load documents: ${response.statusText}`)
      }

      const data: SearchResult<Document> = await response.json()

      if (data.hits) {
        documents.value = data.hits.hits
        total.value = data.hits.total?.value || 0
        scrollId.value = data._scroll_id || null
      }
    } catch (err) {
      error.value = err instanceof Error ? err.message : 'Failed to load documents'
      console.error('Error loading documents:', err)
    } finally {
      loading.value = false
    }
  }

  const loadMore = async () => {
    if (loadingMore.value || !scrollId.value) return

    loadingMore.value = true

    try {
      const url = new URL(`${apiUrl.value}/documents`)
      url.searchParams.set('scroll_id', scrollId.value)
      url.searchParams.set('size', String(pageSize.value))

      const response = await fetch(url.toString())

      if (!response.ok) {
        throw new Error(`Failed to load more documents: ${response.statusText}`)
      }

      const data: SearchResult<Document> = await response.json()

      if (data.hits?.hits) {
        documents.value.push(...data.hits.hits)
        scrollId.value = data._scroll_id || null
      }
    } catch (err) {
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
