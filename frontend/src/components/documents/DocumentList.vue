<script setup lang="ts">
import { FileText, Star, CheckSquare, Square } from 'lucide-vue-next'
import { useFavorites } from '@/composables/useFavorites'
import { extractTitleFromText } from '@/lib/documentMetadata'
import HighlightedText from './HighlightedText.vue'
import { formatDate } from '@/lib/format'
import type { Document } from '@/types/documents'

defineProps<{
  documents: Document[]
  selectable?: boolean
  selectedIds?: Set<string>
  searchTerm?: string
}>()
const emit = defineEmits<{ selectDocument: [doc: Document]; toggleSelect: [doc: Document] }>()
const { isFavorite, toggle } = useFavorites()
const title = (doc: Document) =>
  doc._source.title || extractTitleFromText(doc._source.text || '') || 'Untitled document'
</script>
<template>
  <div class="document-list" role="list" aria-label="Documents">
    <div class="document-list-head" aria-hidden="true">
      <span>Document</span><span class="document-list-type">Type</span
      ><span class="document-list-date">Document date</span><span />
    </div>
    <div
      v-for="doc in documents"
      :key="doc._id"
      role="listitem"
      class="document-list-row"
      :class="{ 'is-selected': selectedIds?.has(doc._id) }"
    >
      <button
        type="button"
        class="document-list-main"
        :aria-label="`${selectable ? 'Select' : 'Open'} ${title(doc)}`"
        :aria-pressed="selectable ? !!selectedIds?.has(doc._id) : undefined"
        @click="selectable ? emit('toggleSelect', doc) : emit('selectDocument', doc)"
      >
        <span class="document-file-icon"
          ><CheckSquare
            v-if="selectable && selectedIds?.has(doc._id)"
            :size="18"
            aria-hidden="true" /><Square
            v-else-if="selectable"
            :size="18"
            aria-hidden="true" /><FileText v-else :size="18" aria-hidden="true"
        /></span>
        <span class="min-w-0"
          ><span class="document-list-title">{{ title(doc) }}</span
          ><span class="document-list-subtitle">{{
            doc._source.company?.name || 'Personal document'
          }}</span>
          <span
            v-if="searchTerm && doc.highlight?.text?.[0]"
            class="mt-1 block line-clamp-2 text-xs text-muted-foreground"
          >
            <HighlightedText :text="doc.highlight.text[0]" />
          </span>
        </span>
      </button>
      <span
        class="document-list-type capitalize"
        :aria-label="`Type: ${doc._source.docType || 'Unknown'}`"
        >{{ doc._source.docType || '—' }}</span
      >
      <span
        class="document-list-date"
        :aria-label="`Document date: ${formatDate(doc._source.date) || 'Undated'}`"
        >{{ formatDate(doc._source.date) || 'Undated' }}</span
      >
      <button
        type="button"
        class="list-star"
        :aria-label="`${isFavorite(doc._id) ? 'Unstar' : 'Star'} ${title(doc)}`"
        :aria-pressed="isFavorite(doc._id)"
        @click="toggle(doc._id)"
      >
        <Star :size="16" :class="{ 'fill-current': isFavorite(doc._id) }" aria-hidden="true" />
      </button>
    </div>
  </div>
</template>
