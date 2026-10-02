<script setup lang="ts">
import { computed } from 'vue'
import { CalendarDays, CheckCircle2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import DeadlineLink from './DeadlineLink.vue'
import type { ReminderBucket } from '@/composables/useReminders'
const props = defineProps<{ buckets: ReminderBucket[]; loading: boolean; error: string | null }>()
defineEmits<{ retry: [] }>()
const groups = computed(() => props.buckets.filter((bucket) => bucket.reminders.length))
const labels = { thisWeek: 'Next 7 days', thisMonth: 'Next 30 days', later: 'Later' }
</script>

<template>
  <section id="upcoming" class="workspace-panel scroll-mt-24">
    <h2 class="panel-title">
      <CalendarDays :size="16" class="text-primary" aria-hidden="true" />On the horizon
    </h2>
    <p v-if="loading" class="panel-caption" role="status">Checking upcoming dates…</p>
    <div v-else-if="error" role="alert">
      <p class="panel-caption">Upcoming dates are unavailable.</p>
      <Button variant="ghost" size="sm" @click="$emit('retry')">Try again</Button>
    </div>
    <template v-else-if="groups.length">
      <div v-for="group in groups" :key="group.key" class="mt-4">
        <h3 class="text-xs font-medium text-muted-foreground">{{ labels[group.key] }}</h3>
        <DeadlineLink v-for="item in group.reminders.slice(0, 3)" :key="item.id" :reminder="item" />
        <details v-if="group.reminders.length > 3" class="mt-3">
          <summary class="text-action cursor-pointer">
            Show {{ group.reminders.length - 3 }} more
            {{ group.reminders.length - 3 === 1 ? 'deadline' : 'deadlines' }}
          </summary>
          <DeadlineLink v-for="item in group.reminders.slice(3)" :key="item.id" :reminder="item" />
        </details>
      </div>
    </template>
    <div v-else class="py-3">
      <CheckCircle2 :size="24" class="mb-3 text-primary" aria-hidden="true" />
      <p class="text-sm font-medium">Nothing coming up.</p>
      <p class="panel-caption mt-1">No extracted deadlines in the next 90 days.</p>
    </div>
    <p class="reminder-footer">
      Dates extracted from your documents. Check the original before acting.
    </p>
  </section>
</template>
