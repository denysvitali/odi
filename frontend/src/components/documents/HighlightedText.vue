<script setup lang="ts">
import { computed } from 'vue'
import { parseHighlight } from '@/lib/highlight'

interface Props {
  text: string
}

const props = defineProps<Props>()

const segments = computed(() => parseHighlight(props.text))
</script>

<template>
  <span>
    <template v-for="(seg, i) in segments" :key="i">
      <mark v-if="seg.match" class="search-highlight">{{ seg.text }}</mark>
      <template v-else>{{ seg.text }}</template>
    </template>
  </span>
</template>
