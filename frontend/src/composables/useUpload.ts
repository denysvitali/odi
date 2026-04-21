import { ref } from 'vue'
import { getApiUrl } from '@/lib/config'

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

const MAX_ATTEMPTS = 3

export function useUpload() {
  const uploading = ref(false)
  const progress = ref(0)
  const result = ref<UploadResult | null>(null)
  const error = ref<string | null>(null)
  const attempt = ref(0)
  let currentXhr: XMLHttpRequest | null = null

  const attemptOnce = (files: File[]) =>
    new Promise<UploadResult>((resolve, reject) => {
      const formData = new FormData()
      for (const file of files) formData.append('files', file)

      const xhr = new XMLHttpRequest()
      currentXhr = xhr

      xhr.upload.addEventListener('progress', (e) => {
        if (e.lengthComputable) {
          progress.value = Math.round((e.loaded / e.total) * 100)
        }
      })
      xhr.addEventListener('load', () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          try {
            resolve(JSON.parse(xhr.responseText) as UploadResult)
          } catch {
            reject(new Error('Invalid response'))
          }
        } else {
          let msg = `Upload failed (${xhr.status})`
          try {
            const body = JSON.parse(xhr.responseText)
            if (body.error) msg = body.error
          } catch {}
          const err = new Error(msg) as Error & { status?: number }
          err.status = xhr.status
          reject(err)
        }
      })
      xhr.addEventListener('error', () => reject(new Error('Network error')))
      xhr.addEventListener('abort', () => reject(new Error('Upload aborted')))
      xhr.open('POST', `${getApiUrl()}/upload`)
      xhr.send(formData)
    })

  const upload = async (files: File[]) => {
    if (uploading.value) return
    if (files.length === 0) return

    uploading.value = true
    progress.value = 0
    result.value = null
    error.value = null
    attempt.value = 0

    try {
      while (attempt.value < MAX_ATTEMPTS) {
        attempt.value += 1
        try {
          const data = await attemptOnce(files)
          result.value = data
          return data
        } catch (caught) {
          const err = caught as Error & { status?: number }
          const status = err.status ?? 0
          const retryable = status === 0 || status >= 500
          if (!retryable || attempt.value >= MAX_ATTEMPTS) {
            error.value = err.message
            throw err
          }
          await new Promise((r) => setTimeout(r, 500 * Math.pow(2, attempt.value - 1)))
        }
      }
    } finally {
      uploading.value = false
      currentXhr = null
    }
  }

  const abort = () => {
    currentXhr?.abort()
    currentXhr = null
    uploading.value = false
  }

  const reset = () => {
    uploading.value = false
    progress.value = 0
    result.value = null
    error.value = null
    attempt.value = 0
  }

  return {
    uploading,
    progress,
    result,
    error,
    attempt,
    upload,
    abort,
    reset
  }
}
