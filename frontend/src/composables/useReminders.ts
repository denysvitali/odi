import { computed, ref } from 'vue'
import { api } from '@/api/client'
import type { Reminder } from '@/types/documents'

/**
 * useReminders drives the "Upcoming" deadlines view. It fetches reminders
 * derived from the due dates / renewal deadlines already extracted into each
 * document and buckets them into "This week / This month / Later" for display.
 *
 * Requires `api.getReminders(days?: number): Promise<{ reminders: Reminder[]; days: number }>`
 * (see handoff notes for shape). No LLM call is involved on the backend.
 */
export interface ReminderBucket {
  key: 'thisWeek' | 'thisMonth' | 'later'
  label: string
  reminders: Reminder[]
}

const WEEK_MS = 7 * 24 * 60 * 60 * 1000
const MONTH_MS = 30 * 24 * 60 * 60 * 1000

export function useReminders() {
  const reminders = ref<Reminder[]>([])
  const days = ref(90)
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function load(windowDays?: number): Promise<void> {
    loading.value = true
    error.value = null
    try {
      const res = await api.getReminders(windowDays)
      reminders.value = res.reminders ?? []
      days.value = res.days ?? windowDays ?? 90
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : 'Unable to load reminders'
      reminders.value = []
    } finally {
      loading.value = false
    }
  }

  // buckets groups the (already ascending) reminders into time horizons relative
  // to now, so the view can render distinct "This week / This month / Later"
  // sections without re-sorting.
  const buckets = computed<ReminderBucket[]>(() => {
    const now = Date.now()
    const thisWeek: Reminder[] = []
    const thisMonth: Reminder[] = []
    const later: Reminder[] = []

    for (const r of reminders.value) {
      const due = new Date(r.dueDate).getTime()
      const delta = due - now
      if (Number.isNaN(due) || delta > MONTH_MS) {
        later.push(r)
      } else if (delta <= WEEK_MS) {
        thisWeek.push(r)
      } else {
        thisMonth.push(r)
      }
    }

    return [
      { key: 'thisWeek', label: 'This week', reminders: thisWeek },
      { key: 'thisMonth', label: 'This month', reminders: thisMonth },
      { key: 'later', label: 'Later', reminders: later }
    ]
  })

  const total = computed(() => reminders.value.length)
  const hasReminders = computed(() => total.value > 0)

  return { reminders, days, loading, error, load, buckets, total, hasReminders }
}
