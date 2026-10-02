<script setup lang="ts">
import { ref, computed } from 'vue'
import { FileSearch, Upload, LayoutGrid, List } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import DocumentCard from './DocumentCard.vue'
import DocumentList from './DocumentList.vue'
import DocumentSkeleton from './DocumentSkeleton.vue'
import { extractTitleFromText } from '@/lib/documentMetadata'
import type { Document } from '@/types/documents'

interface Props {
  documents: Document[]
  loading?: boolean
  loadingMore?: boolean
  searchTerm?: string
  opensearchUrl?: string
  hasMore?: boolean
  emptyTitle?: string
  emptyMessage?: string
  emptyAction?: 'browse' | 'upload' | 'none'
  selectable?: boolean
  selectedIds?: Set<string>
  initialView?: 'grid' | 'list'
}

const props = withDefaults(defineProps<Props>(), {
  emptyAction: 'none'
})

const emit = defineEmits<{
  loadMore: []
  selectDocument: [document: Document]
  toggleSelect: [document: Document]
  navigate: [path: string]
}>()

const gridRef = ref<HTMLElement>()
const view = ref<'grid' | 'list'>(props.initialView || 'list')
try {
  const saved = localStorage.getItem('odi-document-view')
  if (!props.initialView && (saved === 'grid' || saved === 'list')) view.value = saved
} catch {
  /* View preference is optional when browser storage is unavailable. */
}
const setView = (value: 'grid' | 'list') => {
  view.value = value
  try {
    localStorage.setItem('odi-document-view', value)
  } catch {
    /* Keep the current view. */
  }
}
const sort = ref('original')
const displayedDocuments = computed(() => {
  if (sort.value === 'original') return props.documents
  const docs = [...props.documents]
  if (sort.value === 'title')
    return docs.sort((a, b) =>
      (a._source.title || extractTitleFromText(a._source.text || '')).localeCompare(
        b._source.title || extractTitleFromText(b._source.text || '')
      )
    )
  const timestamp = (doc: Document) => {
    const value = new Date(doc._source.date || doc._source.indexedAt || '').getTime()
    return Number.isNaN(value) ? null : value
  }
  return docs.sort((a, b) => {
    const left = timestamp(a),
      right = timestamp(b)
    if (left === null) return right === null ? 0 : 1
    if (right === null) return -1
    return sort.value === 'oldest' ? left - right : right - left
  })
})
const skeletonCount = 8
const focusedIndex = ref(-1)

const totalColumns = computed(() => {
  if (typeof window === 'undefined') return 4
  const grid = gridRef.value?.querySelector('.grid')
  if (!grid) return 4
  const style = window.getComputedStyle(grid)
  return style.gridTemplateColumns.split(' ').length || 4
})

const isSelected = (id: string) => !!props.selectedIds?.has(id)

const handleKeyDown = (event: KeyboardEvent) => {
  if (view.value !== 'grid' || event.defaultPrevented) return
  const cards = Array.from(
    gridRef.value?.querySelectorAll<HTMLElement>('.grid > [role="button"]') || []
  )
  const index = cards.indexOf(event.target as HTMLElement)
  if (index < 0) return
  let next = index
  if (event.key === 'ArrowRight') next = (index + 1) % cards.length
  else if (event.key === 'ArrowLeft') next = (index - 1 + cards.length) % cards.length
  else if (event.key === 'ArrowDown') next = Math.min(index + totalColumns.value, cards.length - 1)
  else if (event.key === 'ArrowUp') next = Math.max(index - totalColumns.value, 0)
  else return
  event.preventDefault()
  focusedIndex.value = next
  cards[next]?.focus()
}
</script>

<template>
  <div ref="gridRef" class="space-y-6" @keydown="handleKeyDown">
    <div v-if="documents.length || loading" class="archive-grid-toolbar">
      <label
        >Sort loaded documents
        <select v-model="sort" aria-label="Sort loaded documents">
          <option value="original">Original order</option>
          <option value="newest">Newest first</option>
          <option value="oldest">Oldest first</option>
          <option value="title">Title A–Z</option>
        </select>
      </label>
      <div class="view-switch" role="group" aria-label="Document view">
        <button
          type="button"
          aria-label="List view"
          :aria-pressed="view === 'list'"
          @click="setView('list')"
        >
          <List :size="15" aria-hidden="true" />
        </button>
        <button
          type="button"
          aria-label="Grid view"
          :aria-pressed="view === 'grid'"
          @click="setView('grid')"
        >
          <LayoutGrid :size="15" aria-hidden="true" />
        </button>
      </div>
    </div>
    <DocumentList
      v-if="view === 'list' && !loading && documents.length"
      :documents="displayedDocuments"
      :selectable="selectable"
      :selected-ids="selectedIds"
      @select-document="emit('selectDocument', $event)"
      @toggle-select="emit('toggleSelect', $event)"
    />
    <div
      v-else-if="view === 'list' && loading"
      class="document-list p-4 space-y-4"
      aria-label="Loading documents"
      aria-busy="true"
    >
      <div v-for="i in 5" :key="i" class="skeleton h-12 rounded-md" />
    </div>
    <div
      v-else-if="view === 'grid'"
      class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 stagger-children"
      tabindex="-1"
    >
      <template v-if="loading">
        <DocumentSkeleton v-for="i in skeletonCount" :key="`skeleton-${i}`" />
      </template>

      <template v-else>
        <DocumentCard
          v-for="(doc, index) in displayedDocuments"
          :key="doc._id"
          :document="doc"
          :search-term="searchTerm"
          :opensearch-url="opensearchUrl"
          :focused="index === focusedIndex"
          :selectable="selectable"
          :selected="isSelected(doc._id)"
          @select="emit('selectDocument', $event)"
          @toggle-select="emit('toggleSelect', $event)"
        />
      </template>
    </div>

    <div v-if="hasMore || loadingMore" class="flex justify-center py-8">
      <Button v-if="!loadingMore" variant="ghost" @click="emit('loadMore')">
        Load more documents
      </Button>

      <div v-else class="flex items-center gap-2 text-sm text-muted-foreground">
        <div
          class="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent"
        />
        Loading more…
      </div>
    </div>

    <div
      v-if="!loading && documents.length === 0"
      class="flex flex-col items-center justify-center py-20 text-center"
    >
      <div class="relative mb-4">
        <div
          class="absolute inset-0 bg-gradient-to-br from-primary/20 to-apple-purple/20 blur-2xl"
          aria-hidden="true"
        />
        <div class="relative rounded-2xl border bg-background p-5">
          <FileSearch class="h-10 w-10 text-muted-foreground" aria-hidden="true" />
        </div>
      </div>
      <h3 class="text-lg font-semibold">
        {{ emptyTitle || (searchTerm ? `No results for "${searchTerm}"` : 'No documents found') }}
      </h3>
      <p class="mt-2 max-w-sm text-sm text-muted-foreground">
        {{ emptyMessage || 'Try different search terms, remove filters, or upload new documents.' }}
      </p>
      <div v-if="emptyAction !== 'none'" class="mt-6 flex gap-2">
        <Button v-if="emptyAction === 'upload'" @click="emit('navigate', '/upload')">
          <Upload class="mr-2 h-4 w-4" aria-hidden="true" />
          Upload documents
        </Button>
        <Button
          v-if="emptyAction === 'browse'"
          variant="outline"
          @click="emit('navigate', '/documents')"
        >
          Browse all documents
        </Button>
      </div>
    </div>
  </div>
</template>
