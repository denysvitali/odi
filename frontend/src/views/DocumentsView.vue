<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { CalendarDays, RefreshCw, X, CheckSquare, Download, Star, Tag as TagIcon } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import DocumentGrid from '@/components/documents/DocumentGrid.vue'
import DocumentDetailSheet from '@/components/documents/DocumentDetailSheet.vue'
import PageContainer from '@/components/layout/PageContainer.vue'
import { useDocuments } from '@/composables/useDocuments'
import { useInfiniteScroll } from '@/composables/useInfiniteScroll'
import { useSelection } from '@/composables/useSelection'
import { useTags } from '@/composables/useTags'
import { getOpensearchUrl } from '@/lib/config'
import { formatNumber } from '@/lib/format'
import type { Document } from '@/types/documents'

const route = useRoute()
const router = useRouter()

const {
  documents,
  loading,
  loadingMore,
  error,
  total,
  hasMore,
  dateRange,
  loadDocuments,
  loadMore,
  refresh
} = useDocuments({ initialPageSize: 12 })

const opensearchUrl = computed(() => getOpensearchUrl())
const selectedDocument = ref<Document | null>(null)
const sheetOpen = ref(false)
const filtersOpen = ref(false)

const dateFrom = ref('')
const dateTo = ref('')
const tagFilter = ref('')

const { allTags, getTags } = useTags()
const selection = useSelection()

const hasActiveFilters = computed(
  () => Boolean(dateFrom.value || dateTo.value || tagFilter.value)
)

const visibleDocuments = computed(() => {
  if (!tagFilter.value) return documents.value
  return documents.value.filter((d) => getTags(d._id).includes(tagFilter.value))
})

const applyDateFilter = () => {
  if (!dateFrom.value && !dateTo.value) dateRange.value = null
  else dateRange.value = { from: dateFrom.value, to: dateTo.value }
  filtersOpen.value = false
}

const clearFilters = () => {
  dateFrom.value = ''
  dateTo.value = ''
  tagFilter.value = ''
  dateRange.value = null
}

const handleSelectDocument = (doc: Document) => {
  if (selection.active.value) {
    selection.toggle(doc._id)
    return
  }
  selectedDocument.value = doc
  sheetOpen.value = true
  router.replace({ path: `/documents/${doc._id}` })
}

const handleToggleSelect = (doc: Document) => {
  if (!selection.active.value) selection.setActive(true)
  selection.toggle(doc._id)
}

const selectAllVisible = () => selection.selectAll(visibleDocuments.value.map((d) => d._id))

const exportCSV = () => {
  const rows = visibleDocuments.value
    .filter((d) => selection.selected.value.has(d._id) || !selection.active.value)
    .map((d) => ({
      id: d._id,
      company: d._source.company?.name || '',
      primaryDate: d._source.primaryDate || '',
      indexedAt: d._source.indexedAt || ''
    }))
  const header = Object.keys(rows[0] || { id: '', company: '', primaryDate: '', indexedAt: '' })
  const csv = [
    header.join(','),
    ...rows.map((r) =>
      header
        .map((h) => {
          const v = String((r as Record<string, string>)[h] ?? '').replace(/"/g, '""')
          return `"${v}"`
        })
        .join(',')
    )
  ].join('\n')
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `odi-documents-${new Date().toISOString().slice(0, 10)}.csv`
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

// Infinite scroll
const { targetRef } = useInfiniteScroll(() => {
  if (!loading.value && !loadingMore.value && hasMore.value) loadMore()
})

// Deep-link: /documents/:id opens the sheet
const openDocIdFromRoute = () => {
  const id = route.params.id
  if (typeof id === 'string' && id) {
    const doc = documents.value.find((d) => d._id === id)
    if (doc) {
      selectedDocument.value = doc
      sheetOpen.value = true
    }
  }
}

watch(sheetOpen, (v) => {
  if (!v && route.params.id) router.replace({ path: '/documents' })
})

watch(documents, () => openDocIdFromRoute())

onMounted(async () => {
  await loadDocuments()
  openDocIdFromRoute()
})
</script>

<template>
  <PageContainer>
    <div class="space-y-6">
      <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 class="text-2xl font-bold tracking-tight">Documents</h1>
          <p class="text-muted-foreground">
            Browse all indexed documents
            <span v-if="hasActiveFilters" class="text-primary">· filtered</span>
          </p>
        </div>

        <div class="flex flex-wrap items-center gap-2">
          <Button
            variant="outline"
            size="sm"
            :class="selection.active.value ? 'border-primary text-primary' : ''"
            @click="selection.setActive(!selection.active.value)"
          >
            <CheckSquare class="mr-2 h-4 w-4" aria-hidden="true" />
            {{ selection.active.value ? 'Selecting' : 'Select' }}
          </Button>
          <Button variant="outline" size="sm" @click="exportCSV">
            <Download class="mr-2 h-4 w-4" aria-hidden="true" />
            Export CSV
          </Button>
          <Button
            variant="outline"
            size="sm"
            :class="hasActiveFilters ? 'border-primary text-primary' : ''"
            @click="filtersOpen = !filtersOpen"
          >
            <CalendarDays class="mr-2 h-4 w-4" aria-hidden="true" />
            Filters
          </Button>
          <Button variant="outline" size="sm" :disabled="loading" @click="refresh">
            <RefreshCw class="mr-2 h-4 w-4" :class="{ 'animate-spin': loading }" aria-hidden="true" />
            Refresh
          </Button>
        </div>
      </div>

      <div
        v-if="filtersOpen"
        class="flex flex-col gap-3 rounded-lg border bg-card p-4 sm:flex-row sm:flex-wrap sm:items-end"
      >
        <label class="grid gap-1.5 text-sm font-medium">
          From
          <Input v-model="dateFrom" type="date" class="sm:w-44" />
        </label>
        <label class="grid gap-1.5 text-sm font-medium">
          To
          <Input v-model="dateTo" type="date" class="sm:w-44" />
        </label>

        <div v-if="allTags.length" class="grid gap-1.5 text-sm font-medium">
          <span class="flex items-center gap-1"><TagIcon class="h-3.5 w-3.5" /> Tag</span>
          <div class="flex flex-wrap gap-1">
            <Badge
              v-for="t in allTags"
              :key="t"
              :variant="tagFilter === t ? 'default' : 'outline'"
              class="cursor-pointer"
              @click="tagFilter = tagFilter === t ? '' : t"
            >
              {{ t }}
            </Badge>
          </div>
        </div>

        <div class="flex gap-2">
          <Button size="sm" @click="applyDateFilter">Apply</Button>
          <Button v-if="hasActiveFilters" variant="ghost" size="sm" @click="clearFilters">
            <X class="mr-2 h-4 w-4" aria-hidden="true" />
            Clear
          </Button>
        </div>
      </div>

      <div
        v-if="selection.active.value"
        class="flex flex-wrap items-center gap-3 rounded-lg border bg-primary/5 p-3 text-sm"
      >
        <span class="font-medium">
          <Star v-if="selection.count.value === 0" class="mr-1 inline h-3.5 w-3.5" aria-hidden="true" />
          {{ selection.count.value }} selected
        </span>
        <Button size="sm" variant="ghost" @click="selectAllVisible">Select all visible</Button>
        <Button size="sm" variant="ghost" @click="selection.clear">Clear</Button>
        <div class="flex-1" />
        <Button size="sm" variant="outline" @click="exportCSV">
          <Download class="mr-2 h-3.5 w-3.5" aria-hidden="true" />
          Export selected
        </Button>
      </div>

      <div
        v-if="error"
        class="rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-center text-destructive"
      >
        <p>{{ error }}</p>
        <Button variant="outline" size="sm" class="mt-2" @click="refresh">Try Again</Button>
      </div>

      <div v-if="!error" class="flex items-center justify-between">
        <div class="text-sm text-muted-foreground">
          <span v-if="loading" class="flex items-center gap-2">
            <div class="h-3.5 w-3.5 animate-spin rounded-full border-2 border-current border-t-transparent" />
            Loading documents…
          </span>
          <span v-else-if="total > 0">
            {{ formatNumber(total) }} document{{ total !== 1 ? 's' : '' }} indexed
          </span>
          <span v-else>No documents found</span>
        </div>
      </div>

      <DocumentGrid
        :documents="visibleDocuments"
        :loading="loading"
        :loading-more="loadingMore"
        :has-more="hasMore"
        :opensearch-url="opensearchUrl"
        :selectable="selection.active.value"
        :selected-ids="selection.selected.value"
        empty-action="upload"
        @load-more="loadMore"
        @select-document="handleSelectDocument"
        @toggle-select="handleToggleSelect"
        @navigate="(p) => router.push(p)"
      />

      <div ref="targetRef" class="h-4" />
    </div>

    <DocumentDetailSheet v-model:open="sheetOpen" :document="selectedDocument" />
  </PageContainer>
</template>
