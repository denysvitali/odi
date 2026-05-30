<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { MessageSquare, Send, Loader2, AlertCircle, Sparkles } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from '@/components/ui/card'
import PageContainer from '@/components/layout/PageContainer.vue'
import DocumentCard from '@/components/documents/DocumentCard.vue'
import DocumentDetailSheet from '@/components/documents/DocumentDetailSheet.vue'
import { useChat } from '@/composables/useChat'
import { api } from '@/api/client'
import { getOpensearchUrl } from '@/lib/config'
import { logger } from '@/lib/logger'
import type { Document } from '@/types/documents'

const { question, answer, citations, loading, error, ask } = useChat()

const opensearchUrl = computed(() => getOpensearchUrl())

// Citation doc IDs are resolved into lightweight Document objects so they can
// be rendered with the shared DocumentCard component.
const citedDocuments = ref<Document[]>([])

const selectedDocument = ref<Document | null>(null)
const sheetOpen = ref(false)

async function resolveCitations(ids: string[]) {
  const resolved = await Promise.all(
    ids.map(async (id): Promise<Document> => {
      try {
        const details = await api.getDocumentDetails(id)
        return {
          _id: id,
          _source: {
            text: details.text ?? '',
            title: details.title,
            company: details.company,
            date: details.primaryDate,
            indexedAt: details.indexedAt
          }
        }
      } catch (caught) {
        logger.warn('ChatView: failed to resolve citation', id, caught)
        return { _id: id, _source: { text: '' } }
      }
    })
  )
  citedDocuments.value = resolved
}

watch(citations, (ids) => {
  if (ids.length) {
    void resolveCitations(ids)
  } else {
    citedDocuments.value = []
  }
})

function handleSelect(doc: Document) {
  selectedDocument.value = doc
  sheetOpen.value = true
}

function submit() {
  void ask()
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
    e.preventDefault()
    submit()
  }
}
</script>

<template>
  <PageContainer>
    <div class="space-y-6">
      <div>
        <h1 class="flex items-center gap-2 text-2xl font-bold tracking-tight">
          <MessageSquare class="h-6 w-6 text-primary" aria-hidden="true" />
          Chat with your archive
        </h1>
        <p class="text-muted-foreground">
          Ask a question and get an answer grounded in your indexed documents.
        </p>
      </div>

      <Card>
        <CardContent class="space-y-3 pt-6">
          <textarea
            v-model="question"
            rows="3"
            placeholder="e.g. What was the total amount on my last insurance invoice?"
            class="w-full resize-none rounded-md border border-input bg-background px-3 py-2 text-sm shadow-sm placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
            :disabled="loading"
            @keydown="onKeydown"
          />
          <div class="flex items-center justify-between">
            <span class="text-xs text-muted-foreground">Press ⌘/Ctrl + Enter to send</span>
            <Button :disabled="loading || !question.trim()" @click="submit">
              <Loader2 v-if="loading" class="mr-2 h-4 w-4 animate-spin" aria-hidden="true" />
              <Send v-else class="mr-2 h-4 w-4" aria-hidden="true" />
              Ask
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card v-if="error" class="border-red-500/50">
        <CardHeader>
          <CardTitle class="flex items-center gap-2 text-red-600">
            <AlertCircle class="h-5 w-5" aria-hidden="true" />
            Unable to answer
          </CardTitle>
          <CardDescription>{{ error }}</CardDescription>
        </CardHeader>
      </Card>

      <Card v-if="answer">
        <CardHeader>
          <CardTitle class="flex items-center gap-2">
            <Sparkles class="h-5 w-5 text-primary" aria-hidden="true" />
            Answer
          </CardTitle>
        </CardHeader>
        <CardContent>
          <p class="whitespace-pre-wrap text-sm leading-relaxed">{{ answer }}</p>
        </CardContent>
      </Card>

      <div v-if="citedDocuments.length" class="space-y-3">
        <h2 class="text-sm font-medium text-muted-foreground">
          Sources · {{ citedDocuments.length }}
        </h2>
        <div class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-4">
          <DocumentCard
            v-for="doc in citedDocuments"
            :key="doc._id"
            :document="doc"
            :opensearch-url="opensearchUrl"
            @select="handleSelect"
          />
        </div>
      </div>
    </div>

    <DocumentDetailSheet v-model:open="sheetOpen" :document="selectedDocument" />
  </PageContainer>
</template>
