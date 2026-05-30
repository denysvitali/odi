import { ref } from 'vue'
import { api } from '@/api/client'

/**
 * A single extracted label/value pair (e.g. amount due, due date, IBAN).
 */
export interface KeyFact {
  label: string
  value: string
}

/**
 * Shape returned by `api.summarizeDocument(id)`.
 * See HANDOFF NOTES for the exact client.ts signature this composable expects.
 */
export interface DocumentSummary {
  summary: string
  keyFacts: KeyFact[]
}

/**
 * useDocumentSummary provides on-demand AI summarization for a single document.
 * `summarize(id)` calls the backend, which lazily generates + caches the
 * summary, and exposes reactive summary / keyFacts / loading / error refs.
 */
export function useDocumentSummary() {
  const summary = ref<string>('')
  const keyFacts = ref<KeyFact[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  const summarize = async (id: string) => {
    if (!id) {
      return
    }
    loading.value = true
    error.value = null
    try {
      const result = await api.summarizeDocument(id)
      summary.value = result.summary ?? ''
      keyFacts.value = result.keyFacts ?? []
    } catch (e) {
      error.value = e instanceof Error ? e.message : 'Failed to generate summary'
      summary.value = ''
      keyFacts.value = []
    } finally {
      loading.value = false
    }
  }

  const reset = () => {
    summary.value = ''
    keyFacts.value = []
    error.value = null
    loading.value = false
  }

  return {
    summary,
    keyFacts,
    loading,
    error,
    summarize,
    reset,
  }
}
