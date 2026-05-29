import { ref, watch, type Ref } from 'vue'
import { api } from '@/api/client'
import { errorMessage } from '@/lib/utils'
import type { FacetData, FacetBucket, SearchFilters } from '@/api/client'

// Raw OpenSearch aggregation response types
interface OsBucket {
  key: string | number
  key_as_string?: string
  doc_count: number
}

interface OsAggregations {
  companies?: { buckets: OsBucket[] }
  date_histogram?: { buckets: OsBucket[] }
  barcode_count?: { doc_count: number }
}

interface OsSearchResponse {
  hits?: {
    total?: { value: number }
  }
  aggregations?: OsAggregations
}

export interface UseFacetsOptions {
  debounceMs?: number
}

export function useFacets(
  searchTerm: Ref<string>,
  activeFilters: Ref<SearchFilters>,
  options: UseFacetsOptions = {}
) {
  const { debounceMs = 300 } = options

  const facets = ref<FacetData>({
    companies: [],
    dateHistogram: [],
    barcodeCount: 0,
    totalHits: 0,
  })
  const loading = ref(false)
  const error = ref<string | null>(null)

  let debounceTimeout: ReturnType<typeof setTimeout> | null = null

  const parseAggregations = (response: OsSearchResponse): FacetData => {
    const aggs = response.aggregations
    const totalHits = response.hits?.total?.value || 0

    const companies: FacetBucket[] = aggs?.companies?.buckets.map((b) => ({
      key: String(b.key),
      doc_count: b.doc_count,
    })) || []

    const dateHistogram: FacetBucket[] = aggs?.date_histogram?.buckets.map((b) => ({
      key: b.key_as_string || String(b.key),
      doc_count: b.doc_count,
    })) || []

    const barcodeCount = aggs?.barcode_count?.doc_count || 0

    return { companies, dateHistogram, barcodeCount, totalHits }
  }

  const fetchFacets = async () => {
    if (!searchTerm.value.trim()) {
      facets.value = {
        companies: [],
        dateHistogram: [],
        barcodeCount: 0,
        totalHits: 0,
      }
      return
    }

    loading.value = true
    error.value = null

    try {
      const data = await api.searchFacets({
        searchTerm: searchTerm.value,
        filters: activeFilters.value,
      }) as unknown as OsSearchResponse
      facets.value = parseAggregations(data)
    } catch (err) {
      error.value = errorMessage(err, 'Failed to load facets')
    } finally {
      loading.value = false
    }
  }

  const debouncedFetch = () => {
    if (debounceTimeout) clearTimeout(debounceTimeout)
    debounceTimeout = setTimeout(fetchFacets, debounceMs)
  }

  // Auto-refresh when search term or filters change
  watch([searchTerm, activeFilters], debouncedFetch, { deep: true })

  return {
    facets,
    loading,
    error,
    fetchFacets,
  }
}
