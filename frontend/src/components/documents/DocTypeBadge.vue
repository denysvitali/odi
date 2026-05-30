<script setup lang="ts">
import { computed } from 'vue'
import { Badge } from '@/components/ui/badge'
import { cn } from '@/lib/utils'

interface Props {
  docType: string
  class?: string
}

const props = defineProps<Props>()

// Deterministic colour selection: hash the docType to one of a small palette
// so the same type always renders with the same colour across the app.
const palette = [
  'bg-blue-100 text-blue-800 dark:bg-blue-900/40 dark:text-blue-300',
  'bg-green-100 text-green-800 dark:bg-green-900/40 dark:text-green-300',
  'bg-amber-100 text-amber-800 dark:bg-amber-900/40 dark:text-amber-300',
  'bg-purple-100 text-purple-800 dark:bg-purple-900/40 dark:text-purple-300',
  'bg-pink-100 text-pink-800 dark:bg-pink-900/40 dark:text-pink-300',
  'bg-cyan-100 text-cyan-800 dark:bg-cyan-900/40 dark:text-cyan-300',
  'bg-orange-100 text-orange-800 dark:bg-orange-900/40 dark:text-orange-300',
]

const colorClass = computed(() => {
  const key = props.docType ?? ''
  let hash = 0
  for (let i = 0; i < key.length; i++) {
    hash = (hash * 31 + key.charCodeAt(i)) >>> 0
  }
  return palette[hash % palette.length]
})

const label = computed(() => props.docType?.trim() || 'Unknown')
</script>

<template>
  <Badge
    variant="secondary"
    :class="cn('border-transparent', colorClass, props.class)"
  >
    {{ label }}
  </Badge>
</template>
