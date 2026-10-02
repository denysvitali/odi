<script setup lang="ts">
import { computed } from 'vue'
import { cn } from '@/lib/utils'

import type { ClassValue } from 'clsx'

interface Props {
  class?: ClassValue
  fullWidth?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  fullWidth: false
})

const classes = computed(() =>
  cn('mx-auto w-full', props.fullWidth ? 'max-w-none' : 'max-w-7xl', 'min-w-0', props.class)
)
</script>

<template>
  <div :class="classes">
    <slot />
  </div>
</template>
