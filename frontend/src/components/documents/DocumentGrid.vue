<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { FileSearch, Upload } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import DocumentCard from './DocumentCard.vue'
import DocumentSkeleton from './DocumentSkeleton.vue'
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
  const docs = props.documents
  if (docs.length === 0) return
  const target = event.target as HTMLElement | null
  if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable)) return

  switch (event.key) {
    case 'ArrowRight':
      event.preventDefault()
      focusedIndex.value = focusedIndex.value < docs.length - 1 ? focusedIndex.value + 1 : 0
      break
    case 'ArrowLeft':
      event.preventDefault()
      focusedIndex.value = focusedIndex.value > 0 ? focusedIndex.value - 1 : docs.length - 1
      break
    case 'ArrowDown': {
      event.preventDefault()
      const ni = focusedIndex.value + totalColumns.value
      if (ni < docs.length) focusedIndex.value = ni
      break
    }
    case 'ArrowUp': {
      event.preventDefault()
      const ni = focusedIndex.value - totalColumns.value
      if (ni >= 0) focusedIndex.value = ni
      break
    }
    case 'Enter':
      event.preventDefault()
      if (focusedIndex.value >= 0 && focusedIndex.value < docs.length) {
        emit('selectDocument', docs[focusedIndex.value])
      }
      break
    case 'Escape':
      event.preventDefault()
      focusedIndex.value = -1
      break
  }
}

onMounted(() => window.addEventListener('keydown', handleKeyDown))
onUnmounted(() => window.removeEventListener('keydown', handleKeyDown))
</script>

<template>
  <div ref="gridRef" class="space-y-6">
    <div
      class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 stagger-children"
      tabindex="-1"
    >
      <template v-if="loading">
        <DocumentSkeleton v-for="i in skeletonCount" :key="`skeleton-${i}`" />
      </template>

      <template v-else>
        <DocumentCard
          v-for="(doc, index) in documents"
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
      <Button
        v-if="!loadingMore"
        variant="ghost"
        @click="emit('loadMore')"
      >
        Load more documents
      </Button>

      <div v-else class="flex items-center gap-2 text-sm text-muted-foreground">
        <div class="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
        Loading more…
      </div>
    </div>

    <div
      v-if="!loading && documents.length === 0"
      class="flex flex-col items-center justify-center py-20 text-center"
    >
      <div class="relative mb-4">
        <div class="absolute inset-0 bg-gradient-to-br from-primary/20 to-apple-purple/20 blur-2xl" aria-hidden="true" />
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
        <Button v-if="emptyAction === 'browse'" variant="outline" @click="emit('navigate', '/documents')">
          Browse all documents
        </Button>
      </div>
    </div>
  </div>
</template>
