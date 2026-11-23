import { useCallback, useState } from 'react'
import { api } from '@/api/client'
import { downloadBlob } from '@/lib/document'
import { errorMessage } from '@/lib/utils'

/** Open or download the full-resolution file through the authenticated transport. */
export function useDocumentFile(id: string) {
  const [error, setError] = useState<string | null>(null)

  const open = useCallback(async () => {
    // Open the window within the click gesture so popup blockers allow it.
    const preview = window.open('about:blank', '_blank')
    if (preview) preview.opener = null
    setError(null)
    try {
      if (!preview) throw new Error('Allow popups to open the document, or download it')
      const url = URL.createObjectURL(await api.getFile(id))
      preview.location.replace(url)
      // The new tab keeps its own reference once loaded.
      setTimeout(() => URL.revokeObjectURL(url), 60_000)
    } catch (err) {
      preview?.close()
      setError(errorMessage(err, 'Unable to open document'))
    }
  }, [id])

  const download = useCallback(async () => {
    setError(null)
    try {
      downloadBlob(await api.getFile(id), id)
    } catch (err) {
      setError(errorMessage(err, 'Unable to download document'))
    }
  }, [id])

  return { open, download, error }
}
