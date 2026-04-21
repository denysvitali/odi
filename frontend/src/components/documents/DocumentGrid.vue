<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
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
}

const props = defineProps<Props>()

const emit = defineEmits<{
  loadMore: []
  selectDocument: [document: Document]
}>()

const gridRef = ref<HTMLElement>()

// Show 6 skeleton items when loading
const skeletonCount = 6

// Keyboard navigation state
const focusedIndex = ref(-1)

const totalColumns = computed(() => {
  if (typeof window === 'undefined') return 4
  const grid = gridRef.value?.querySelector('.grid')
  if (!grid) return 4
  const style = window.getComputedStyle(grid)
  return style.gridTemplateColumns.split(' ').length || 4
})

const handleKeyDown = (event: KeyboardEvent) => {
  const docs = props.documents
  if (docs.length === 0) return

  switch (event.key) {
    case 'ArrowRight': {
      event.preventDefault()
      if (focusedIndex.value < docs.length - 1) {
        focusedIndex.value++
      } else {
        focusedIndex.value = 0
      }
      break
    }
    case 'ArrowLeft': {
      event.preventDefault()
      if (focusedIndex.value > 0) {
        focusedIndex.value--
      } else {
        focusedIndex.value = docs.length - 1
      }
      break
    }
    case 'ArrowDown': {
      event.preventDefault()
      const newIndex = focusedIndex.value + totalColumns.value
      if (newIndex < docs.length) {
        focusedIndex.value = newIndex
      }
      break
    }
    case 'ArrowUp': {
      event.preventDefault()
      const newIndex = focusedIndex.value - totalColumns.value
      if (newIndex >= 0) {
        focusedIndex.value = newIndex
      }
      break
    }
    case 'Enter': {
      event.preventDefault()
      if (focusedIndex.value >= 0 && focusedIndex.value < docs.length) {
        emit('selectDocument', docs[focusedIndex.value])
      }
      break
    }
    case 'Escape': {
      event.preventDefault()
      focusedIndex.value = -1
      break
    }
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeyDown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeyDown)
})
</script>

<template>
  <div ref="gridRef" class="space-y-6">
    <!-- Document Grid -->
    <div
      class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4"
      tabindex="-1"
    >
      <template v-if="loading">
        <DocumentSkeleton
          v-for="i in skeletonCount"
          :key="`skeleton-${i}`"
        />
      </template>

      <template v-else>
        <DocumentCard
          v-for="(doc, index) in documents"
          :key="doc._id"
          :document="doc"
          :search-term="searchTerm"
          :opensearch-url="opensearchUrl"
          :focused="index === focusedIndex"
          v-motion="{
            initial: { opacity: 0, y: 20 },
            enter: { opacity: 1, y: 0 }
          }"
          @select="emit('selectDocument', $event)"
        />
      </template>
    </div>

    <!-- Load More -->
    <div v-if="hasMore || loadingMore" class="flex justify-center py-8">
      <button
        v-if="!loadingMore"
        class="rounded-lg px-6 py-3 text-sm font-medium text-muted-foreground transition-colors hover:bg-secondary hover:text-foreground"
        @click="emit('loadMore')"
      >
        Load more documents
      </button>

      <div v-else class="flex items-center gap-2 text-sm text-muted-foreground">
        <div class="h-4 w-4 animate-spin rounded-full border-2 border-current border-t-transparent" />
        Loading more...
      </div>
    </div>

    <!-- Empty State -->
    <div
      v-if="!loading && documents.length === 0"
      class="flex flex-col items-center justify-center py-20 text-center"
    >
      <div class="rounded-full bg-secondary p-4">
        <svg
          class="h-8 w-8 text-muted-foreground"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
          />
        </svg>
      </div>
      <h3 class="mt-4 text-lg font-semibold">No documents found</h3>
      <p class="mt-2 max-w-sm text-sm text-muted-foreground">
        Try adjusting your search terms or browse all documents.
      </p>
    </div>
  </div>
</template>
