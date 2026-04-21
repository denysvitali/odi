<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { FileText, Trash2, Sparkles, TrendingUp } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import SearchInput from '@/components/search/SearchInput.vue'
import ResultsCounter from '@/components/search/ResultsCounter.vue'
import DocumentGrid from '@/components/documents/DocumentGrid.vue'
import DocumentDetailSheet from '@/components/documents/DocumentDetailSheet.vue'
import PageContainer from '@/components/layout/PageContainer.vue'
import { useSearch } from '@/composables/useSearch'
import { useDocumentStore } from '@/stores/documents'
import { getOpensearchUrl } from '@/lib/config'
import { formatNumber } from '@/lib/format'
import type { Document } from '@/types/documents'

const route = useRoute()
const router = useRouter()

const { searchTerm, results, loading, total, hasSearched, search } = useSearch({ debounceMs: 300 })

const store = useDocumentStore()
const selectedDocument = ref<Document | null>(null)
const sheetOpen = ref(false)

const opensearchUrl = computed(() => getOpensearchUrl())

const handleSelectDocument = (doc: Document) => {
  selectedDocument.value = doc
  sheetOpen.value = true
}

const handleSearch = () => {
  if (searchTerm.value.trim()) {
    search(searchTerm.value)
    store.addRecentSearch(searchTerm.value)
    router.replace({ path: '/', query: { q: searchTerm.value } })
  }
}

const clearHistory = () => store.clearRecentSearches()

onMounted(() => {
  store.loadRecentSearches()
  const q = route.query.q
  if (typeof q === 'string' && q.trim()) {
    searchTerm.value = q
    search(q)
  }
})

watch(
  () => route.query.q,
  (q) => {
    if (typeof q === 'string' && q !== searchTerm.value) {
      searchTerm.value = q
      if (q) search(q)
    }
  }
)
</script>

<template>
  <PageContainer
    :class="['transition-all duration-500 ease-out', hasSearched ? 'pt-4' : 'pt-[14vh]']"
  >
    <div class="mb-8 text-center">
      <div v-if="!hasSearched" class="relative inline-block">
        <div class="absolute inset-0 -z-10 bg-gradient-to-br from-primary/30 to-apple-purple/30 blur-3xl" aria-hidden="true" />
        <div class="mb-6 inline-flex h-16 w-16 items-center justify-center rounded-2xl bg-gradient-to-br from-primary to-apple-purple text-primary-foreground shadow-lg">
          <FileText class="h-8 w-8" aria-hidden="true" />
        </div>
      </div>

      <h1 v-if="!hasSearched" class="mb-2 text-balance text-3xl font-bold tracking-tight sm:text-5xl">
        Search your
        <span class="text-gradient">documents</span>
      </h1>

      <p v-if="!hasSearched" class="mb-8 text-balance text-muted-foreground">
        Full-text search across every page you've ever scanned.
      </p>

      <SearchInput
        v-model="searchTerm"
        class="mx-auto max-w-2xl"
        @submit="handleSearch"
      />

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
            @click="searchTerm = term; handleSearch()"
          >
            {{ term }}
          </Button>
        </div>
      </div>

      <div v-if="!hasSearched" class="mt-10 flex items-center justify-center gap-6 text-xs text-muted-foreground">
        <span class="inline-flex items-center gap-1.5">
          <Sparkles class="h-3.5 w-3.5 text-primary" aria-hidden="true" />
          Press <kbd class="rounded border bg-background px-1.5 py-0.5 font-medium">?</kbd> for shortcuts
        </span>
      </div>
    </div>

    <div v-if="hasSearched" class="space-y-6">
      <div class="flex items-center justify-between">
        <ResultsCounter :count="results.length" :total="total" :loading="loading" />
        <Button variant="ghost" size="sm" @click="searchTerm = ''; search('')">
          Clear
        </Button>
      </div>

      <DocumentGrid
        :documents="results"
        :loading="loading"
        :search-term="searchTerm"
        :opensearch-url="opensearchUrl"
        empty-action="browse"
        @select-document="handleSelectDocument"
        @navigate="(p) => router.push(p)"
      />

      <p v-if="total > 0" class="text-center text-xs text-muted-foreground">
        {{ formatNumber(total) }} total match{{ total === 1 ? '' : 'es' }}
      </p>
    </div>

    <DocumentDetailSheet v-model:open="sheetOpen" :document="selectedDocument" />
  </PageContainer>
</template>
