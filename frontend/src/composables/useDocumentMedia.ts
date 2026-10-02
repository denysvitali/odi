import { ref, watch, onScopeDispose, type Ref } from 'vue'
import { api } from '@/api/client'
import { errorMessage } from '@/lib/utils'

// Protected media must use the same bearer-authenticated transport as JSON requests.
export function useDocumentMedia(id: Ref<string>) {
  const thumbnailUrl = ref('')
  const error = ref<string | null>(null)
  let fileUrl = ''
  let fileInflight: Promise<string> | null = null
  let controller: AbortController | null = null
  let generation = 0

  const release = () => {
    controller?.abort()
    if (thumbnailUrl.value) URL.revokeObjectURL(thumbnailUrl.value)
    if (fileUrl) URL.revokeObjectURL(fileUrl)
    thumbnailUrl.value = ''
    fileUrl = ''
    fileInflight = null
  }

  watch(
    id,
    async (value) => {
      const current = ++generation
      release()
      error.value = null
      if (!value) return
      controller = new AbortController()
      try {
        const blob = await api.getThumbnail(value, controller.signal)
        if (current === generation) thumbnailUrl.value = URL.createObjectURL(blob)
      } catch (err) {
        if (current === generation && !controller?.signal.aborted) {
          error.value = errorMessage(err, 'Unable to load document preview')
        }
      }
    },
    { immediate: true }
  )

  const getFileUrl = async () => {
    if (fileUrl) return fileUrl
    if (fileInflight) return fileInflight
    const current = generation
    const signal = controller?.signal
    const promise = api
      .getFile(id.value, signal)
      .then((blob) => {
        if (current !== generation || signal?.aborted) throw new Error('Document changed')
        fileUrl = URL.createObjectURL(blob)
        return fileUrl
      })
      .finally(() => {
        if (current === generation) fileInflight = null
      })
    fileInflight = promise
    return promise
  }

  const openFile = async () => {
    // Create the window within the click gesture to avoid popup blocking.
    const preview = window.open('about:blank', '_blank')
    if (preview) preview.opener = null
    error.value = null
    try {
      const url = await getFileUrl()
      if (preview) preview.location.replace(url)
      else throw new Error('Allow popups to open the document, or download it')
    } catch (err) {
      preview?.close()
      error.value = errorMessage(err, 'Unable to open document')
    }
  }

  const downloadFile = async () => {
    error.value = null
    try {
      const url = await getFileUrl()
      const link = document.createElement('a')
      link.href = url
      link.download = id.value
      document.body.appendChild(link)
      link.click()
      link.remove()
    } catch (err) {
      error.value = errorMessage(err, 'Unable to download document')
    }
  }

  onScopeDispose(() => {
    generation++
    release()
  })

  return { thumbnailUrl, error, openFile, downloadFile }
}
