<script setup lang="ts">
import { computed } from 'vue'
import { useDocumentMedia } from '@/composables/useDocumentMedia'

const props = defineProps<{ id: string; alt: string }>()
const { thumbnailUrl } = useDocumentMedia(computed(() => props.id))
</script>

<template>
  <img v-if="thumbnailUrl" :src="thumbnailUrl" :alt="alt" loading="lazy" decoding="async" />
</template>
