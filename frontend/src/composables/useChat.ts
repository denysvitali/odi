import { ref } from 'vue'
import { api, type SearchFilters } from '@/api/client'

/**
 * useChat drives the "Chat with your archive" RAG flow. It posts a question
 * (optionally scoped by the same SearchFilters used elsewhere) to the backend
 * and exposes the prose answer plus the citation document IDs.
 *
 * Requires `api.chat({ question, filters })` (see handoff notes for shape).
 */
export function useChat() {
  const question = ref('')
  const answer = ref('')
  const citations = ref<string[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function ask(filters?: SearchFilters) {
    const q = question.value.trim()
    if (!q || loading.value) return

    loading.value = true
    error.value = null
    answer.value = ''
    citations.value = []

    try {
      const res = await api.chat({ question: q, filters })
      answer.value = res.answer
      citations.value = res.citations ?? []
    } catch (caught) {
      error.value = caught instanceof Error ? caught.message : 'Unable to get an answer'
    } finally {
      loading.value = false
    }
  }

  function reset() {
    question.value = ''
    answer.value = ''
    citations.value = []
    error.value = null
  }

  return { question, answer, citations, loading, error, ask, reset }
}
