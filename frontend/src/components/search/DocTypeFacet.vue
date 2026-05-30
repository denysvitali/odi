<script setup lang="ts">
import { FileType } from 'lucide-vue-next'
import { cn } from '@/lib/utils'

interface Bucket {
  key: string
  doc_count: number
}

interface Props {
  buckets: Bucket[]
  selected?: string[]
}

const props = withDefaults(defineProps<Props>(), {
  selected: () => [],
})

const emit = defineEmits<{
  update: [keys: string[]]
}>()

const isSelected = (key: string) => props.selected.includes(key)

const toggle = (key: string) => {
  const next = [...props.selected]
  const idx = next.indexOf(key)
  if (idx >= 0) {
    next.splice(idx, 1)
  } else {
    next.push(key)
  }
  emit('update', next)
}
</script>

<template>
  <div v-if="buckets.length > 0">
    <div class="mb-2 flex items-center gap-2 text-sm font-medium">
      <FileType class="h-4 w-4 text-muted-foreground" />
      Document Type
    </div>
    <div class="flex flex-wrap gap-1.5">
      <button
        v-for="bucket in buckets"
        :key="bucket.key"
        type="button"
        :class="cn(
          'inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium transition-colors',
          isSelected(bucket.key)
            ? 'border-primary bg-primary/10 text-primary'
            : 'border-input hover:bg-accent'
        )"
        @click="toggle(bucket.key)"
      >
        <span class="truncate">{{ bucket.key }}</span>
        <span class="text-muted-foreground">{{ bucket.doc_count }}</span>
      </button>
    </div>
  </div>
</template>
