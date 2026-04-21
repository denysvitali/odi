<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { Calendar, Building2, FileText, QrCode, AlertCircle, ExternalLink, Star, Tag as TagIcon, Plus, X, Download } from 'lucide-vue-next'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription
} from '@/components/ui/sheet'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Separator } from '@/components/ui/separator'
import DocumentDetailSection from './DocumentDetailSection.vue'
import DocumentCompanyCard from './DocumentCompanyCard.vue'
import DocumentBarcodeCard from './DocumentBarcodeCard.vue'
import DocumentTextContent from './DocumentTextContent.vue'
import DocumentDetailSkeleton from './DocumentDetailSkeleton.vue'
import { useDocumentDetails } from '@/composables/useDocumentDetails'
import { useFavorites } from '@/composables/useFavorites'
import { useTags } from '@/composables/useTags'
import { api } from '@/api/client'
import { formatDate, formatDateTime } from '@/lib/format'
import type { Document } from '@/types/documents'

interface Props {
  document: Document | null
  open: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  'update:open': [value: boolean]
}>()

const { details, loading, error, fetchDetails, clearDetails } = useDocumentDetails()
const { isFavorite, toggle: toggleFav } = useFavorites()
const { getTags, addTag, removeTag } = useTags()

const newTag = ref('')
const findText = ref('')

const thumbnailUrl = computed(() => (props.document ? api.thumbnailUrl(props.document._id) : ''))
const fullImageUrl = computed(() => (props.document ? api.fileUrl(props.document._id) : ''))
const docId = computed(() => props.document?._id || '')
const starred = computed(() => (docId.value ? isFavorite(docId.value) : false))
const docTags = computed(() => (docId.value ? getTags(docId.value) : []))

const hasBarcodes = computed(() => {
  return details.value?.barcodes && details.value.barcodes.length > 0
})

watch([() => props.document, () => props.open], ([newDoc, isOpen]) => {
  if (isOpen && newDoc) {
    fetchDetails(newDoc._id)
  } else if (!isOpen) {
    clearDetails()
    findText.value = ''
  }
}, { immediate: true })

const handleOpenChange = (value: boolean) => {
  emit('update:open', value)
  if (!value) clearDetails()
}

const openFullImage = () => {
  window.open(fullImageUrl.value, '_blank', 'noopener,noreferrer')
}

const downloadImage = () => {
  const a = document.createElement('a')
  a.href = fullImageUrl.value
  a.download = docId.value
  a.rel = 'noopener'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
}

const onAddTag = (e: Event) => {
  e.preventDefault()
  if (!docId.value) return
  addTag(docId.value, newTag.value)
  newTag.value = ''
}
</script>

<template>
  <Sheet :open="open" @update:open="handleOpenChange">
    <SheetContent class="flex w-full flex-col overflow-hidden sm:max-w-lg">
      <SheetHeader class="shrink-0">
        <div class="flex items-center justify-between gap-2">
          <SheetTitle>Document Details</SheetTitle>
          <div class="flex items-center gap-1">
            <Button
              v-if="docId"
              variant="ghost"
              size="icon"
              :aria-label="starred ? 'Unstar document' : 'Star document'"
              @click="toggleFav(docId)"
            >
              <Star class="h-4 w-4" :class="starred ? 'fill-yellow-500 text-yellow-500' : ''" />
            </Button>
            <Button
              v-if="docId"
              variant="ghost"
              size="icon"
              aria-label="Download image"
              @click="downloadImage"
            >
              <Download class="h-4 w-4" />
            </Button>
          </div>
        </div>
        <SheetDescription v-if="document" class="truncate font-mono text-xs">
          {{ document._id }}
        </SheetDescription>
      </SheetHeader>

      <ScrollArea class="-mx-6 flex-1 px-6">
        <div v-if="loading" class="py-6">
          <DocumentDetailSkeleton />
        </div>

        <div v-else-if="error" class="py-6">
          <div class="rounded-lg border border-destructive/50 bg-destructive/10 p-4 text-center">
            <AlertCircle class="mx-auto mb-2 h-8 w-8 text-destructive" aria-hidden="true" />
            <p class="text-sm text-destructive">{{ error }}</p>
            <Button
              variant="outline"
              size="sm"
              class="mt-3"
              @click="document && fetchDetails(document._id, { skipCache: true })"
            >
              Try Again
            </Button>
          </div>
        </div>

        <div v-else-if="details" class="space-y-6 py-6">
          <div
            class="group relative aspect-[3/4] cursor-zoom-in overflow-hidden rounded-lg bg-gradient-to-br from-muted to-muted/60"
            @click="openFullImage"
          >
            <img
              :src="thumbnailUrl"
              :alt="`Preview for document ${document?._id}`"
              class="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105"
              loading="lazy"
              decoding="async"
            />
            <div class="absolute inset-0 flex items-center justify-center bg-black/0 opacity-0 transition-all group-hover:bg-black/40 group-hover:opacity-100">
              <Button variant="secondary" size="sm">
                <ExternalLink class="mr-2 h-4 w-4" aria-hidden="true" />
                View Full Image
              </Button>
            </div>
          </div>

          <div>
            <div class="mb-2 flex items-center gap-2 text-xs text-muted-foreground">
              <TagIcon class="h-3.5 w-3.5" aria-hidden="true" />
              Tags
            </div>
            <div class="flex flex-wrap items-center gap-1.5">
              <Badge
                v-for="t in docTags"
                :key="t"
                variant="secondary"
                class="gap-1"
              >
                {{ t }}
                <button
                  type="button"
                  class="rounded-full p-0.5 hover:bg-background/50"
                  :aria-label="`Remove tag ${t}`"
                  @click="removeTag(docId, t)"
                >
                  <X class="h-3 w-3" aria-hidden="true" />
                </button>
              </Badge>
              <form class="flex items-center gap-1" @submit="onAddTag">
                <Input
                  v-model="newTag"
                  placeholder="Add tag…"
                  class="h-7 w-28 text-xs"
                  aria-label="Add tag"
                />
                <Button type="submit" variant="ghost" size="icon" class="h-7 w-7" aria-label="Add">
                  <Plus class="h-3.5 w-3.5" aria-hidden="true" />
                </Button>
              </form>
            </div>
          </div>

          <Tabs default-value="info" class="w-full">
            <TabsList class="grid w-full grid-cols-3">
              <TabsTrigger value="info">
                <FileText class="mr-2 h-4 w-4" aria-hidden="true" />
                Info
              </TabsTrigger>
              <TabsTrigger value="text">
                <FileText class="mr-2 h-4 w-4" aria-hidden="true" />
                Text
              </TabsTrigger>
              <TabsTrigger value="barcodes" :disabled="!hasBarcodes">
                <QrCode class="mr-2 h-4 w-4" aria-hidden="true" />
                Barcodes
                <Badge v-if="hasBarcodes" variant="secondary" class="ml-2 text-xs">
                  {{ details.barcodes!.length }}
                </Badge>
              </TabsTrigger>
            </TabsList>

            <TabsContent value="info" class="space-y-6 pt-4">
              <DocumentDetailSection title="Dates" :icon="Calendar">
                <div class="space-y-3">
                  <div v-if="details.primaryDate" class="space-y-1">
                    <p class="text-xs text-muted-foreground">Primary Date</p>
                    <p class="font-medium">{{ formatDate(details.primaryDate) }}</p>
                  </div>

                  <div v-if="details.dates && details.dates.length > 0" class="space-y-1">
                    <p class="text-xs text-muted-foreground">All Dates Found</p>
                    <div class="flex flex-wrap gap-2">
                      <Badge v-for="date in details.dates" :key="date" variant="outline">
                        {{ formatDate(date) }}
                      </Badge>
                    </div>
                  </div>

                  <div v-if="details.indexedAt" class="space-y-1">
                    <p class="text-xs text-muted-foreground">Indexed At</p>
                    <p class="text-sm text-muted-foreground">{{ formatDateTime(details.indexedAt) }}</p>
                  </div>
                </div>
              </DocumentDetailSection>

              <Separator />

              <DocumentDetailSection title="Company" :icon="Building2">
                <template v-if="details.company">
                  <DocumentCompanyCard :company="details.company" />
                </template>
                <p v-else class="text-sm text-muted-foreground">
                  No company information available
                </p>
              </DocumentDetailSection>
            </TabsContent>

            <TabsContent value="text" class="space-y-3 pt-4">
              <Input
                v-model="findText"
                type="search"
                placeholder="Find in text…"
                aria-label="Find in text"
              />
              <template v-if="details.text">
                <DocumentTextContent :text="details.text" :find="findText" />
              </template>
              <p v-else class="text-sm text-muted-foreground">
                No text content available
              </p>
            </TabsContent>

            <TabsContent value="barcodes" class="space-y-4 pt-4">
              <template v-if="hasBarcodes">
                <DocumentBarcodeCard
                  v-for="(barcode, index) in details.barcodes"
                  :key="index"
                  :barcode="barcode"
                />
              </template>
              <p v-else class="text-sm text-muted-foreground">
                No barcodes found in this document
              </p>
            </TabsContent>
          </Tabs>
        </div>

        <div v-else class="py-6 text-center text-muted-foreground">
          Select a document to view details
        </div>
      </ScrollArea>
    </SheetContent>
  </Sheet>
</template>
