import { ref, watch, onScopeDispose, type Ref } from 'vue'
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
  docTypes?: { buckets: OsBucket[] }
  tags?: { buckets: OsBucket[] }
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
    totalHits: 0
  })
  const loading = ref(false)
  const error = ref<string | null>(null)

  let generation = 0
  let debounceTimeout: ReturnType<typeof setTimeout> | null = null

  const parseAggregations = (response: OsSearchResponse): FacetData => {
    const aggs = response.aggregations
    const totalHits = response.hits?.total?.value || 0

    const companies: FacetBucket[] =
      aggs?.companies?.buckets.map((b) => ({
        key: String(b.key),
        doc_count: b.doc_count
      })) || []

    const dateHistogram: FacetBucket[] =
      aggs?.date_histogram?.buckets.map((b) => ({
        key: b.key_as_string || String(b.key),
        doc_count: b.doc_count
      })) || []

    const barcodeCount = aggs?.barcode_count?.doc_count || 0

    const docTypes =
      aggs?.docTypes?.buckets.map((b) => ({ key: String(b.key), doc_count: b.doc_count })) || []
    const tags =
      aggs?.tags?.buckets.map((b) => ({ key: String(b.key), doc_count: b.doc_count })) || []
    return { companies, dateHistogram, barcodeCount, totalHits, docTypes, tags }
  }

  const fetchFacets = async () => {
    const current = ++generation
    if (
      !searchTerm.value.trim() &&
      !Object.values(activeFilters.value).some((value) =>
        Array.isArray(value) ? value.length > 0 : value !== undefined && value !== ''
      )
    ) {
      facets.value = {
        companies: [],
        dateHistogram: [],
        barcodeCount: 0,
        totalHits: 0
      }
      loading.value = false
      error.value = null
      return
    }

    loading.value = true
    error.value = null

    try {
      const data = (await api.searchFacets({
        searchTerm: searchTerm.value,
        filters: activeFilters.value
      })) as unknown as OsSearchResponse
      if (current === generation) facets.value = parseAggregations(data)
    } catch (err) {
      if (current === generation) error.value = errorMessage(err, 'Failed to load facets')
    } finally {
      if (current === generation) loading.value = false
    }
  }

  const debouncedFetch = () => {
    generation++
    if (debounceTimeout) clearTimeout(debounceTimeout)
    debounceTimeout = setTimeout(fetchFacets, debounceMs)
  }

  // Auto-refresh when search term or filters change
  watch([searchTerm, activeFilters], debouncedFetch, { deep: true })

  onScopeDispose(() => {
    generation++
    if (debounceTimeout) clearTimeout(debounceTimeout)
  })

  return {
    facets,
    loading,
    error,
    fetchFacets
  }
}
