<script setup lang="ts">
import { computed, ref } from 'vue'
import { Copy, Check, BookOpen, Type } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { ScrollArea } from '@/components/ui/scroll-area'

interface Props {
  text: string
  find?: string
}

const props = defineProps<Props>()
const copied = ref(false)
const reading = ref(false)

const copyToClipboard = async () => {
  try {
    await navigator.clipboard.writeText(props.text)
    copied.value = true
    setTimeout(() => (copied.value = false), 2000)
  } catch (err) {
    console.error('Failed to copy:', err)
  }
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function escapeRegex(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
}

// Safe because we escape all content ourselves and only inject <mark> tags.
const renderedText = computed(() => {
  const escaped = escapeHtml(props.text)
  const needle = props.find?.trim()
  if (!needle) return escaped
  const re = new RegExp(escapeRegex(needle), 'gi')
  return escaped.replace(re, (m) => `<mark class="search-highlight">${m}</mark>`)
})

const matchCount = computed(() => {
  const needle = props.find?.trim()
  if (!needle) return 0
  const re = new RegExp(escapeRegex(needle), 'gi')
  return (props.text.match(re) || []).length
})
</script>

<template>
  <div class="space-y-3">
    <div class="flex items-center justify-between gap-2">
      <span class="text-sm text-muted-foreground">
        {{ text.length.toLocaleString() }} characters
        <span v-if="find" class="ml-2">· {{ matchCount }} match{{ matchCount === 1 ? '' : 'es' }}</span>
      </span>
      <div class="flex gap-1">
        <Button
          variant="outline"
          size="sm"
          :aria-pressed="reading"
          :aria-label="reading ? 'Switch to monospace view' : 'Switch to reading view'"
          @click="reading = !reading"
        >
          <BookOpen v-if="!reading" class="h-4 w-4" aria-hidden="true" />
          <Type v-else class="h-4 w-4" aria-hidden="true" />
        </Button>
        <Button variant="outline" size="sm" @click="copyToClipboard">
          <Check v-if="copied" class="mr-2 h-4 w-4 text-green-500" aria-hidden="true" />
          <Copy v-else class="mr-2 h-4 w-4" aria-hidden="true" />
          {{ copied ? 'Copied!' : 'Copy' }}
        </Button>
      </div>
    </div>

    <ScrollArea class="h-[400px] rounded-md border bg-muted/30 p-4">
      <div
        v-if="reading"
        class="mx-auto max-w-prose whitespace-pre-wrap font-serif text-[15px] leading-relaxed"
        v-html="renderedText"
      />
      <pre
        v-else
        class="whitespace-pre-wrap font-mono text-sm"
        v-html="renderedText"
      />
    </ScrollArea>
  </div>
</template>
