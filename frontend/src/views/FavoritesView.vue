<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { Star } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import DocumentGrid from '@/components/documents/DocumentGrid.vue'
import DocumentDetailSheet from '@/components/documents/DocumentDetailSheet.vue'
import PageContainer from '@/components/layout/PageContainer.vue'
import { useDocuments } from '@/composables/useDocuments'
import { useFavorites } from '@/composables/useFavorites'
import { getOpensearchUrl } from '@/lib/config'
import type { Document } from '@/types/documents'

const router = useRouter()
const { documents, loading, loadDocuments } = useDocuments({ initialPageSize: 100 })
const { list, clear } = useFavorites()

const selectedDocument = ref<Document | null>(null)
const sheetOpen = ref(false)

const opensearchUrl = computed(() => getOpensearchUrl())

const favDocs = computed(() => documents.value.filter((d) => list.value.includes(d._id)))

const handleSelect = (doc: Document) => {
  selectedDocument.value = doc
  sheetOpen.value = true
}

onMounted(() => loadDocuments())
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
            Your starred documents · {{ list.length }}
          </p>
        </div>
        <Button v-if="list.length" variant="outline" size="sm" @click="clear">
          Clear all
        </Button>
      </div>

      <DocumentGrid
        :documents="favDocs"
        :loading="loading"
        :opensearch-url="opensearchUrl"
        empty-title="No favorites yet"
        empty-message="Star documents from the grid or detail view to keep them here."
        empty-action="browse"
        @select-document="handleSelect"
        @navigate="(p) => router.push(p)"
      />
    </div>

    <DocumentDetailSheet v-model:open="sheetOpen" :document="selectedDocument" />
  </PageContainer>
</template>
