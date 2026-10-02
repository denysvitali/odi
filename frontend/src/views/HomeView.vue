<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { FileText, Trash2, Sparkles, TrendingUp, X } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import SearchInput from '@/components/search/SearchInput.vue'
import SearchFilters from '@/components/search/SearchFilters.vue'
import ResultsCounter from '@/components/search/ResultsCounter.vue'
import DocumentGrid from '@/components/documents/DocumentGrid.vue'
import DocumentDetailSheet from '@/components/documents/DocumentDetailSheet.vue'
import PageContainer from '@/components/layout/PageContainer.vue'
import { useSearch } from '@/composables/useSearch'
import { useFacets } from '@/composables/useFacets'
import { useDocumentStore } from '@/stores/documents'
import { getOpensearchUrl } from '@/lib/config'
import { formatNumber } from '@/lib/format'
import type { Document } from '@/types/documents'
import type { SearchFilters as SearchFiltersType } from '@/api/client'

const route = useRoute()
const router = useRouter()

const {
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
  loadMore,
  clear,
  clearFilters
} = useSearch({ debounceMs: 300 })

const { facets, loading: facetsLoading } = useFacets(searchTerm, activeFilters)

const store = useDocumentStore()
const selectedDocument = ref<Document | null>(null)
const sheetOpen = ref(false)

const opensearchUrl = computed(() => getOpensearchUrl())

const handleSelectDocument = (doc: Document) => {
  selectedDocument.value = doc
  sheetOpen.value = true
}

const handleSearch = () => {
  if (searchTerm.value.trim() || activeFilterCount.value > 0) {
    search(searchTerm.value)
    if (searchTerm.value.trim()) {
      store.addRecentSearch(searchTerm.value)
    }
    router.replace({
      path: '/',
      query: {
        q: searchTerm.value,
        ...(activeFilters.value.companies?.length
          ? { companies: activeFilters.value.companies.join(',') }
          : {}),
        ...(activeFilters.value.dateFrom ? { dateFrom: activeFilters.value.dateFrom } : {}),
        ...(activeFilters.value.dateTo ? { dateTo: activeFilters.value.dateTo } : {}),
        ...(activeFilters.value.hasBarcode !== undefined
          ? { hasBarcode: String(activeFilters.value.hasBarcode) }
          : {}),
        ...(activeFilters.value.titleFilter ? { title: activeFilters.value.titleFilter } : {}),
        ...(activeFilters.value.docTypes?.length
          ? { docTypes: activeFilters.value.docTypes.join(',') }
          : {}),
        ...(activeFilters.value.tags?.length ? { tags: activeFilters.value.tags.join(',') } : {})
      }
    })
  }
}

const handleFiltersUpdate = (filters: SearchFiltersType) => {
  activeFilters.value = filters
  if (searchTerm.value.trim() || activeFilterCount.value > 0) {
    search(searchTerm.value)
    router.replace({
      path: '/',
      query: {
        q: searchTerm.value,
        ...(filters.companies?.length ? { companies: filters.companies.join(',') } : {}),
        ...(filters.dateFrom ? { dateFrom: filters.dateFrom } : {}),
        ...(filters.dateTo ? { dateTo: filters.dateTo } : {}),
        ...(filters.hasBarcode !== undefined ? { hasBarcode: String(filters.hasBarcode) } : {}),
        ...(filters.titleFilter ? { title: filters.titleFilter } : {}),
        ...(filters.docTypes?.length ? { docTypes: filters.docTypes.join(',') } : {}),
        ...(filters.tags?.length ? { tags: filters.tags.join(',') } : {})
      }
    })
  } else {
    clear()
    router.replace({ path: '/' })
  }
}

const handleClearFilters = () => {
  clearFilters()
  router.replace({ path: '/', query: { q: searchTerm.value } })
}

const clearHistory = () => store.clearRecentSearches()
const selectRecentSearch = (term: string) => {
  searchTerm.value = term
  handleSearch()
}

// Parse filters from URL query
const parseFiltersFromQuery = (): SearchFiltersType => {
  const q = route.query
  const filters: SearchFiltersType = {}

  if (typeof q.companies === 'string' && q.companies) {
    filters.companies = q.companies.split(',').filter(Boolean)
  }
  if (typeof q.dateFrom === 'string' && q.dateFrom) {
    filters.dateFrom = q.dateFrom
  }
  if (typeof q.dateTo === 'string' && q.dateTo) {
    filters.dateTo = q.dateTo
  }
  if (typeof q.hasBarcode === 'string') {
    filters.hasBarcode = q.hasBarcode === 'true'
  }
  if (typeof q.title === 'string' && q.title) {
    filters.titleFilter = q.title
  }

  if (typeof q.docTypes === 'string') filters.docTypes = q.docTypes.split(',').filter(Boolean)
  if (typeof q.tags === 'string') filters.tags = q.tags.split(',').filter(Boolean)
  return filters
}

onMounted(() => store.loadRecentSearches())

watch(
  () => route.query,
  () => {
    const term = typeof route.query.q === 'string' ? route.query.q : ''
    const filters = parseFiltersFromQuery()
    if (
      hasSearched.value &&
      term === searchTerm.value &&
      JSON.stringify(filters) === JSON.stringify(activeFilters.value)
    )
      return
    search(term, filters)
  },
  { immediate: true }
)

const handleClearSearch = () => {
  clear()
  router.replace({ path: '/' })
}
</script>

<template>
  <PageContainer
    :class="['transition-all duration-500 ease-out', hasSearched ? 'pt-4' : 'pt-[14vh]']"
  >
    <div class="mb-8 text-center">
      <div v-if="!hasSearched" class="relative inline-block">
        <div
          class="absolute inset-0 -z-10 bg-gradient-to-br from-primary/30 to-apple-purple/30 blur-3xl"
          aria-hidden="true"
        />
        <div
          class="mb-6 inline-flex h-16 w-16 items-center justify-center rounded-2xl bg-gradient-to-br from-primary to-apple-purple text-primary-foreground shadow-lg"
        >
          <FileText class="h-8 w-8" aria-hidden="true" />
        </div>
      </div>

      <h1
        v-if="!hasSearched"
        class="mb-2 text-balance text-3xl font-bold tracking-tight sm:text-5xl"
      >
        Search your
        <span class="text-gradient">documents</span>
      </h1>

      <p v-if="!hasSearched" class="mb-8 text-balance text-muted-foreground">
        Full-text search across every page you've ever scanned.
      </p>

      <SearchInput v-model="searchTerm" class="mx-auto max-w-2xl" @submit="handleSearch" />

      <div v-if="!hasSearched && store.recentSearches.length > 0" class="mt-6">
        <div class="mb-3 flex items-center justify-center gap-3 text-sm text-muted-foreground">
          <TrendingUp class="h-3.5 w-3.5" aria-hidden="true" />
          <span>Recent searches</span>
          <button
            type="button"
            class="inline-flex items-center gap-1 text-xs hover:text-foreground"
            @click="clearHistory"
          >
            <Trash2 class="h-3 w-3" aria-hidden="true" />
            Clear
          </button>
        </div>
        <div class="flex flex-wrap justify-center gap-2">
          <Button
            v-for="term in store.recentSearches.slice(0, 8)"
            :key="term"
            variant="secondary"
            size="sm"
            class="text-muted-foreground"
            @click="selectRecentSearch(term)"
          >
            {{ term }}
          </Button>
        </div>
      </div>

      <div
        v-if="!hasSearched"
        class="mt-10 flex items-center justify-center gap-6 text-xs text-muted-foreground"
      >
        <span class="inline-flex items-center gap-1.5">
          <Sparkles class="h-3.5 w-3.5 text-primary" aria-hidden="true" />
          Press <kbd class="rounded border bg-background px-1.5 py-0.5 font-medium">?</kbd> for
          shortcuts
        </span>
      </div>
    </div>

    <div v-if="hasSearched" class="space-y-6">
      <div class="flex items-center justify-between">
        <div class="flex items-center gap-3">
          <ResultsCounter :count="results.length" :total="total" :loading="loading" />
          <Badge v-if="activeFilterCount > 0" variant="secondary" class="gap-1">
            {{ activeFilterCount }} filter{{ activeFilterCount === 1 ? '' : 's' }}
            <button
              type="button"
              class="ml-1 rounded-full hover:bg-muted-foreground/20"
              aria-label="Clear search filters"
              @click="handleClearFilters"
            >
              <X class="h-3 w-3" />
            </button>
          </Badge>
        </div>
        <Button variant="ghost" size="sm" @click="handleClearSearch"> Clear </Button>
      </div>

      <div class="flex gap-6">
        <!-- Filters sidebar (mobile toggle + desktop sidebar) -->
        <SearchFilters
          :filters="activeFilters"
          :facets="facets"
          :loading="facetsLoading"
          :active-count="activeFilterCount"
          @update:filters="handleFiltersUpdate"
          @clear="handleClearFilters"
        />

        <!-- Results grid -->
        <div class="flex-1 min-w-0">
          <div
            v-if="error"
            role="alert"
            class="mb-4 rounded-lg border border-destructive/50 p-4 text-destructive"
          >
            <p>{{ error }}</p>
            <Button variant="outline" size="sm" class="mt-2" @click="search(searchTerm)"
              >Try again</Button
            >
          </div>
          <DocumentGrid
            :documents="results"
            :loading="loading"
            :loading-more="loadingMore"
            :has-more="results.length < total"
            :search-term="searchTerm"
            :opensearch-url="opensearchUrl"
            empty-action="browse"
            @select-document="handleSelectDocument"
            @load-more="loadMore"
            @navigate="(p) => router.push(p)"
          />
        </div>
      </div>

      <p v-if="total > 0" class="text-center text-xs text-muted-foreground">
        {{ formatNumber(total) }} total match{{ total === 1 ? '' : 'es' }}
      </p>
    </div>

    <DocumentDetailSheet v-model:open="sheetOpen" :document="selectedDocument" />
  </PageContainer>
</template>
