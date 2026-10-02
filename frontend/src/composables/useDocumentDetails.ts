import { ref } from 'vue'
import { api } from '@/api/client'
import { errorMessage } from '@/lib/utils'
import type { DocumentDetails } from '@/types/documents'

export function useDocumentDetails() {
  const details = ref<DocumentDetails | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  let generation = 0

  const fetchDetails = async (documentId: string, { skipCache = false } = {}) => {
    const current = ++generation
    loading.value = true
    error.value = null
    details.value = null
    try {
      const data = await api.getDocumentDetails(documentId, { skipCache })
      if (current === generation) details.value = data
    } catch (err) {
      if (current === generation) error.value = errorMessage(err, 'Failed to load document details')
    } finally {
      if (current === generation) loading.value = false
    }
  }

  const clearDetails = () => {
    generation++
    loading.value = false
    details.value = null
    error.value = null
  }

  return {
    details,
    loading,
    error,
    fetchDetails,
    clearDetails
  }
}
