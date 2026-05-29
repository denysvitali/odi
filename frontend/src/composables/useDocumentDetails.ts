import { ref } from 'vue'
import { api } from '@/api/client'
import { errorMessage } from '@/lib/utils'
import type { DocumentDetails } from '@/types/documents'

export function useDocumentDetails() {
  const details = ref<DocumentDetails | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  const fetchDetails = async (documentId: string, { skipCache = false } = {}) => {
    if (loading.value) return
    loading.value = true
    error.value = null
    details.value = null
    try {
      details.value = await api.getDocumentDetails(documentId, { skipCache })
    } catch (err) {
      error.value = errorMessage(err, 'Failed to load document details')
    } finally {
      loading.value = false
    }
  }

  const clearDetails = () => {
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
