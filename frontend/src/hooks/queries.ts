import {
  keepPreviousData,
  useInfiniteQuery,
  useMutation,
  useQueries,
  useQuery,
  useQueryClient,
  type InfiniteData
} from '@tanstack/react-query'
import { api, type SearchFilters } from '@/api/client'
import { hasArchiveSearch } from '@/lib/archiveSearch'
import { detailsToDocument } from '@/lib/document'
import type { Document, SearchResult } from '@/types/documents'

// OpenSearch scroll cursors are short-lived server state: cached pages would hold
// expired cursors, so paged queries are dropped as soon as nothing renders them
// (gcTime: 0) and a refresh resets them to page one.
type Page = SearchResult<Document>

function flatten(data: InfiniteData<Page> | undefined) {
  const pages = data?.pages ?? []
  return {
    documents: pages.flatMap((p) => p.hits?.hits ?? []),
    total: pages[0]?.hits?.total?.value ?? 0
  }
}

function nextCursor(last: Page, all: Page[]) {
  const loaded = all.reduce((n, p) => n + (p.hits?.hits.length ?? 0), 0)
  const total = all[0]?.hits?.total?.value ?? 0
  return last._scroll_id && (last.hits?.hits.length ?? 0) > 0 && loaded < total
    ? last._scroll_id
    : undefined
}

export interface DateRange {
  from?: string
  to?: string
}

export function useDocumentList(pageSize: number, range: DateRange = {}) {
  const query = useInfiniteQuery({
    queryKey: ['documents', pageSize, range.from ?? '', range.to ?? ''],
    initialPageParam: '',
    queryFn: ({ pageParam }) =>
      pageParam
        ? api.listDocuments({ scrollId: pageParam, size: pageSize })
        : api.listDocuments({ size: pageSize, dateFrom: range.from, dateTo: range.to }),
    getNextPageParam: nextCursor,
    gcTime: 0
  })
  return { ...query, ...flatten(query.data) }
}

export function useSearch(term: string, filters: SearchFilters, size = 24) {
  const query = useInfiniteQuery({
    queryKey: ['search', term, filters, size],
    enabled: hasArchiveSearch(term, filters),
    initialPageParam: '',
    queryFn: ({ pageParam }) =>
      pageParam
        ? api.search({ scrollId: pageParam })
        : api.search({ searchTerm: term, filters, size }),
    getNextPageParam: nextCursor,
    gcTime: 0
  })
  return { ...query, ...flatten(query.data) }
}

export interface Facets {
  companies: Bucket[]
  docTypes: Bucket[]
  tags: Bucket[]
  barcodeCount: number
  totalHits: number
}
export interface Bucket {
  key: string
  count: number
}

interface OsBucket {
  key: string | number
  key_as_string?: string
  doc_count: number
}
interface OsFacetResponse {
  hits?: { total?: { value: number } }
  aggregations?: {
    companies?: { buckets: OsBucket[] }
    docTypes?: { buckets: OsBucket[] }
    tags?: { buckets: OsBucket[] }
    barcode_count?: { doc_count: number }
  }
}

export function parseFacets(response: OsFacetResponse): Facets {
  const aggs = response.aggregations
  const buckets = (b?: { buckets: OsBucket[] }) =>
    b?.buckets.map((x) => ({ key: String(x.key), count: x.doc_count })) ?? []
  return {
    companies: buckets(aggs?.companies),
    docTypes: buckets(aggs?.docTypes),
    tags: buckets(aggs?.tags),
    barcodeCount: aggs?.barcode_count?.doc_count ?? 0,
    totalHits: response.hits?.total?.value ?? 0
  }
}

export function useFacets(term: string, filters: SearchFilters) {
  return useQuery({
    queryKey: ['facets', term, filters],
    enabled: hasArchiveSearch(term, filters),
    placeholderData: keepPreviousData,
    queryFn: async () =>
      parseFacets((await api.searchFacets({ searchTerm: term, filters })) as OsFacetResponse)
  })
}

export function useDocumentDetails(id: string | null) {
  return useQuery({
    queryKey: ['document', id],
    enabled: !!id,
    queryFn: () => api.getDocumentDetails(id!)
  })
}

/** Resolve a list of IDs to documents (favorites, chat citations). */
export function useDocumentsById(ids: string[]) {
  return useQueries({
    queries: ids.map((id) => ({
      queryKey: ['document', id],
      queryFn: () => api.getDocumentDetails(id)
    })),
    combine: (results) => ({
      documents: results.flatMap((r, i) => (r.data ? [detailsToDocument(ids[i], r.data)] : [])),
      pending: results.filter((r) => r.isPending).length,
      failed: results.filter((r) => r.isError).length,
      retry: () => results.forEach((r) => r.isError && r.refetch())
    })
  })
}

export function useReminders(days = 90) {
  return useQuery({ queryKey: ['reminders', days], queryFn: () => api.getReminders(days) })
}

export function useThumbnail(id: string) {
  return useQuery({
    queryKey: ['thumbnail', id],
    enabled: !!id,
    staleTime: Infinity,
    retry: false,
    queryFn: ({ signal }) => api.getThumbnail(id, signal)
  })
}

export function useShares() {
  return useQuery({ queryKey: ['shares'], queryFn: () => api.listShares() })
}

export function useRevokeShare() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: (token: string) => api.revokeShare(token),
    onSuccess: () => client.invalidateQueries({ queryKey: ['shares'] })
  })
}

export function useCreateShare() {
  const client = useQueryClient()
  return useMutation({
    mutationFn: api.createShare,
    onSuccess: () => client.invalidateQueries({ queryKey: ['shares'] })
  })
}

export function useReindex() {
  const status = useQuery({
    queryKey: ['reindex'],
    queryFn: () => api.getReindexStatus(),
    refetchInterval: (q) => (q.state.data?.state === 'running' ? 2000 : false)
  })
  const client = useQueryClient()
  const start = useMutation({
    mutationFn: () => api.startReindex(),
    onSuccess: (data) => client.setQueryData(['reindex'], data),
    // 409: already running — just pick up the live status.
    onError: () => client.invalidateQueries({ queryKey: ['reindex'] })
  })
  return { status, start }
}
