<script setup lang="ts">
import { computed } from 'vue'
import type { Reminder } from '@/types/documents'
const props = defineProps<{ reminder: Reminder }>()
const date = computed(() => {
  const parsed = new Date(props.reminder.dueDate)
  if (Number.isNaN(parsed.getTime())) return { month: 'Due', day: '—' }
  return {
    month: parsed.toLocaleDateString(undefined, { month: 'short', timeZone: 'UTC' }),
    day: parsed.toLocaleDateString(undefined, { day: 'numeric', timeZone: 'UTC' })
  }
})
</script>

<template>
  <RouterLink :to="`/documents/${encodeURIComponent(reminder.id)}`" class="reminder-item">
    <span class="reminder-date"
      ><small>{{ date.month }}</small
      ><strong>{{ date.day }}</strong></span
    >
    <div class="min-w-0">
      <h3 class="truncate">{{ reminder.title }}</h3>
      <p>{{ reminder.company || reminder.docType || 'Document deadline' }}</p>
      <p v-if="reminder.amountDue">{{ reminder.amountDue }}</p>
    </div>
  </RouterLink>
</template>
