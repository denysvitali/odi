<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { Star } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import DocumentGrid from '@/components/documents/DocumentGrid.vue'
import DocumentDetailSheet from '@/components/documents/DocumentDetailSheet.vue'
import PageContainer from '@/components/layout/PageContainer.vue'
import { useFavoriteDocuments } from '@/composables/useFavoriteDocuments'
import { useFavorites } from '@/composables/useFavorites'
import { getOpensearchUrl } from '@/lib/config'
import type { Document } from '@/types/documents'

const router = useRouter()
const { list, clear } = useFavorites()
const { documents, loading, failedCount, pendingCount, retry } = useFavoriteDocuments(list)

const selectedDocument = ref<Document | null>(null)
const sheetOpen = ref(false)

const opensearchUrl = computed(() => getOpensearchUrl())

const handleSelect = (doc: Document) => {
  selectedDocument.value = doc
  sheetOpen.value = true
}
</script>

<template>
  <PageContainer>
    <div class="space-y-6">
      <div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 class="flex items-center gap-2 text-2xl font-bold tracking-tight">
            <Star class="h-6 w-6 fill-yellow-500 text-yellow-500" aria-hidden="true" />
            Favorites
          </h1>
          <p class="text-muted-foreground">
            Your starred documents · {{ list.length }} saved on this device
          </p>
        </div>
        <Button v-if="list.length" variant="outline" size="sm" @click="clear"> Clear all </Button>
      </div>

      <div
        v-if="failedCount"
        role="alert"
        class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-destructive/30 bg-destructive/5 p-4"
      >
        <p class="text-sm">
          {{ failedCount }} {{ failedCount === 1 ? 'favorite could' : 'favorites could' }} not be
          loaded. They may be unavailable or your connection may have changed. Your stars are still
          saved.
        </p>
        <Button variant="outline" size="sm" :disabled="loading" @click="retry">
          Retry unavailable favorites
        </Button>
      </div>
      <p v-if="loading" role="status" class="text-sm text-muted-foreground">
        Loading favorites… {{ documents.length }} available, {{ pendingCount }} remaining.
      </p>
      <DocumentGrid
        :documents="documents"
        :loading="loading && documents.length === 0"
        :opensearch-url="opensearchUrl"
        :empty-title="list.length ? 'No favorites available' : 'No favorites yet'"
        :empty-message="
          list.length
            ? 'Retry to load your saved documents.'
            : 'Star documents from the grid or detail view to keep them here.'
        "
        empty-action="browse"
        @select-document="handleSelect"
        @navigate="(p) => router.push(p)"
      />
    </div>

    <DocumentDetailSheet v-model:open="sheetOpen" :document="selectedDocument" />
  </PageContainer>
</template>
