<script setup lang="ts">
import { computed } from 'vue'
import { ExternalLink, Calendar, Building2, Star, Eye, CheckCircle2, Tag as TagIcon } from 'lucide-vue-next'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Tooltip, TooltipTrigger, TooltipContent } from '@/components/ui/tooltip'
import HighlightedText from './HighlightedText.vue'
import { api } from '@/api/client'
import { useFavorites } from '@/composables/useFavorites'
import { useTags } from '@/composables/useTags'
import type { Document } from '@/types/documents'

interface Props {
  document: Document
  searchTerm?: string
  opensearchUrl?: string
  focused?: boolean
  selectable?: boolean
  selected?: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  select: [document: Document]
  toggleSelect: [document: Document]
}>()

const { isFavorite, toggle: toggleFav } = useFavorites()
const { getTags } = useTags()

const docUrl = computed(() => {
  if (!props.opensearchUrl) return '#'
  return `${props.opensearchUrl}/app/discover#/doc/*/odi-*?id=${encodeURIComponent(props.document._id)}`
})

const thumbnailUrl = computed(() => api.thumbnailUrl(props.document._id))
const fullImageUrl = computed(() => api.fileUrl(props.document._id))

const highlightedText = computed(() => {
  return props.document.highlight?.text?.[0] || props.document._source.text || ''
})

const companyName = computed(() => props.document._source.company?.name)
const tags = computed(() => getTags(props.document._id))
const starred = computed(() => isFavorite(props.document._id))

const formatDocId = (id: string) => {
  const match = id.match(/^(\d{4}-\d{2})/)
  if (match) {
    const [year, month] = match[1].split('-')
    return `${month}/${year}`
  }
  return id.slice(0, 10)
}

const handleImageError = (event: Event) => {
  const target = event.target as HTMLImageElement
  if (target) target.style.display = 'none'
}

const openDocument = () => {
  window.open(fullImageUrl.value, '_blank', 'noopener,noreferrer')
}

const openInOpensearch = () => {
  window.open(docUrl.value, '_blank', 'noopener,noreferrer')
}

const handleCardClick = (e: MouseEvent) => {
  if (props.selectable) {
    emit('toggleSelect', props.document)
    return
  }
  if (e.shiftKey || e.metaKey || e.ctrlKey) {
    emit('toggleSelect', props.document)
    return
  }
  emit('select', props.document)
}

const handleKeydown = (e: KeyboardEvent) => {
  if (e.key === 'Enter' || e.key === ' ') {
    e.preventDefault()
    emit('select', props.document)
  } else if (e.key.toLowerCase() === 's') {
    e.preventDefault()
    toggleFav(props.document._id)
  }
}

const onToggleStar = (e: MouseEvent) => {
  e.stopPropagation()
  toggleFav(props.document._id)
}
</script>

<template>
  <Card
    class="group relative overflow-hidden hover-lift cursor-pointer border-border/50 transition-all duration-200 hover:border-primary/40 hover:shadow-lg"
    :class="[
      focused ? 'ring-2 ring-ring ring-offset-2 ring-offset-background shadow-lg' : '',
      selected ? 'ring-2 ring-primary ring-offset-2 ring-offset-background' : ''
    ]"
    tabindex="0"
    role="button"
    :aria-label="`Document ${document._id}${companyName ? ', ' + companyName : ''}`"
    :aria-pressed="selected"
    @click="handleCardClick"
    @keydown="handleKeydown"
  >
    <div v-if="selectable" class="absolute left-2 top-2 z-10">
      <div
        class="flex h-6 w-6 items-center justify-center rounded-full border-2 bg-background/80 backdrop-blur-sm transition-colors"
        :class="selected ? 'border-primary bg-primary text-primary-foreground' : 'border-border'"
        aria-hidden="true"
      >
        <CheckCircle2 v-if="selected" class="h-4 w-4" />
      </div>
    </div>

    <button
      type="button"
      class="absolute right-2 top-2 z-10 flex h-8 w-8 items-center justify-center rounded-full bg-background/70 text-muted-foreground opacity-0 backdrop-blur-sm transition-all duration-200 hover:bg-background hover:text-yellow-500 focus:opacity-100 group-hover:opacity-100"
      :class="starred ? 'opacity-100 text-yellow-500' : ''"
      :aria-label="starred ? 'Unstar document' : 'Star document'"
      @click="onToggleStar"
    >
      <Star class="h-4 w-4" :class="starred ? 'fill-current' : ''" aria-hidden="true" />
    </button>

    <div class="relative aspect-[3/4] overflow-hidden bg-gradient-to-br from-muted to-muted/60">
      <img
        :src="thumbnailUrl"
        :alt="`Thumbnail for document ${document._id}`"
        class="h-full w-full object-cover transition-transform duration-500 ease-out group-hover:scale-105"
        loading="lazy"
        decoding="async"
        @error="handleImageError"
      />

      <div class="absolute inset-0 flex items-center justify-center gap-2 bg-black/0 opacity-0 transition-all duration-300 group-hover:bg-black/40 group-hover:opacity-100">
        <Tooltip>
          <TooltipTrigger as-child>
            <Button
              variant="secondary"
              size="icon"
              aria-label="View full image"
              class="scale-90 transition-transform duration-200 hover:scale-100"
              @click.stop="openDocument"
            >
              <Eye class="h-4 w-4" aria-hidden="true" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>View full image</TooltipContent>
        </Tooltip>

        <Tooltip v-if="opensearchUrl">
          <TooltipTrigger as-child>
            <Button
              variant="secondary"
              size="icon"
              aria-label="Open in OpenSearch"
              class="scale-90 transition-transform duration-200 hover:scale-100"
              @click.stop="openInOpensearch"
            >
              <ExternalLink class="h-4 w-4" aria-hidden="true" />
            </Button>
          </TooltipTrigger>
          <TooltipContent>Open in OpenSearch</TooltipContent>
        </Tooltip>
      </div>

      <div class="absolute bottom-3 left-3">
        <Badge v-if="companyName" variant="secondary" class="glass text-xs">
          <Building2 class="mr-1 h-3 w-3" aria-hidden="true" />
          {{ companyName }}
        </Badge>
      </div>
    </div>

    <CardContent class="p-4">
      <div class="flex items-center gap-2 text-xs text-muted-foreground">
        <Calendar class="h-3 w-3" aria-hidden="true" />
        <span>{{ formatDocId(document._id) }}</span>
      </div>

      <p
        v-if="highlightedText"
        class="mt-2 line-clamp-3 text-sm text-muted-foreground"
      >
        <HighlightedText :text="highlightedText" />
      </p>

      <div v-if="tags.length" class="mt-2 flex flex-wrap gap-1">
        <Badge v-for="t in tags.slice(0, 3)" :key="t" variant="outline" class="text-[10px]">
          <TagIcon class="mr-1 h-2.5 w-2.5" aria-hidden="true" />
          {{ t }}
        </Badge>
      </div>
    </CardContent>
  </Card>
</template>
