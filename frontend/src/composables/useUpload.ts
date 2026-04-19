import { ref, computed } from 'vue'

export interface UploadPageResult {
  sequenceID: number
  status: string
  error?: string
}

export interface UploadResult {
  scanID: string
  processed: number
  failed: number
  pages: UploadPageResult[]
}

export function useUpload() {
  const uploading = ref(false)
  const progress = ref(0)
  const result = ref<UploadResult | null>(null)
  const error = ref<string | null>(null)

  const apiUrl = computed(() => window._settings?.apiUrl || '')

  const upload = async (files: File[]) => {
    if (uploading.value) return
    if (files.length === 0) return

    uploading.value = true
    progress.value = 0
    result.value = null
    error.value = null

    const formData = new FormData()
    for (const file of files) {
      formData.append('files', file)
    }

    return new Promise<UploadResult>((resolve, reject) => {
      const xhr = new XMLHttpRequest()

      xhr.upload.addEventListener('progress', (e) => {
        if (e.lengthComputable) {
          progress.value = Math.round((e.loaded / e.total) * 100)
        }
      })

      xhr.addEventListener('load', () => {
        uploading.value = false
        if (xhr.status >= 200 && xhr.status < 300) {
          const data: UploadResult = JSON.parse(xhr.responseText)
          result.value = data
          resolve(data)
        } else {
          let msg = `Upload failed (${xhr.status})`
          try {
            const body = JSON.parse(xhr.responseText)
            if (body.error) msg = body.error
          } catch {}
          error.value = msg
          reject(new Error(msg))
        }
      })

      xhr.addEventListener('error', () => {
        uploading.value = false
        error.value = 'Network error'
        reject(new Error('Network error'))
      })

      xhr.addEventListener('abort', () => {
        uploading.value = false
        error.value = 'Upload aborted'
        reject(new Error('Upload aborted'))
      })

      xhr.open('POST', `${apiUrl.value}/upload`)
      xhr.send(formData)
    })
  }

  const reset = () => {
    uploading.value = false
    progress.value = 0
    result.value = null
    error.value = null
  }

  return {
    uploading,
    progress,
    result,
    error,
    upload,
    reset
  }
}
