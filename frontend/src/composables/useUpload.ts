import { ref } from 'vue'
import { getApiUrl } from '@/lib/config'

export interface UploadPageResult {
  sequenceID: number
  status: string
  duplicateOf?: string
  error?: string
}

export interface UploadResult {
  scanID: string
  processed: number
  duplicates: number
  failed: number
  pages: UploadPageResult[]
}

const MAX_ATTEMPTS = 3
const UPLOAD_CHUNK_SIZE = 25

function newScanID(): string {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID()
  }
  return '10000000-1000-4000-8000-100000000000'.replace(/[018]/g, (c) =>
    (Number(c) ^ (Math.random() * 16) >> (Number(c) / 4)).toString(16)
  )
}

export function useUpload() {
  const uploading = ref(false)
  const progress = ref(0)
  const result = ref<UploadResult | null>(null)
  const error = ref<string | null>(null)
  const attempt = ref(0)
  let currentXhr: XMLHttpRequest | null = null

  const attemptOnce = (
    files: File[],
    scanID: string,
    sequenceOffset: number,
    uploadedBytes: number,
    totalBytes: number
  ) =>
    new Promise<UploadResult>((resolve, reject) => {
      const formData = new FormData()
      formData.append('scanID', scanID)
      formData.append('sequenceOffset', String(sequenceOffset))
      for (const file of files) formData.append('files', file)

      const xhr = new XMLHttpRequest()
      currentXhr = xhr

      xhr.upload.addEventListener('progress', (e) => {
        if (e.lengthComputable) {
          progress.value = Math.round(((uploadedBytes + e.loaded) / totalBytes) * 100)
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

    const scanID = newScanID()
    const totalBytes = files.reduce((sum, file) => sum + file.size, 0) || 1
    let uploadedBytes = 0
    const combined: UploadResult = {
      scanID,
      processed: 0,
      duplicates: 0,
      failed: 0,
      pages: []
    }

    try {
      for (let start = 0; start < files.length; start += UPLOAD_CHUNK_SIZE) {
        const chunk = files.slice(start, start + UPLOAD_CHUNK_SIZE)
        const chunkBytes = chunk.reduce((sum, file) => sum + file.size, 0)
        let chunkAttempt = 0

        while (chunkAttempt < MAX_ATTEMPTS) {
          chunkAttempt += 1
          attempt.value += 1
          try {
            const data = await attemptOnce(chunk, scanID, start, uploadedBytes, totalBytes)
            combined.processed += data.processed
            combined.duplicates += data.duplicates
            combined.failed += data.failed
            combined.pages.push(...data.pages)
            uploadedBytes += chunkBytes
            progress.value = Math.round((uploadedBytes / totalBytes) * 100)
            break
          } catch (caught) {
            const err = caught as Error & { status?: number }
            const status = err.status ?? 0
            const retryable = status === 0 || status >= 500
            if (!retryable || chunkAttempt >= MAX_ATTEMPTS) {
              error.value = err.message
              throw err
            }
            await new Promise((r) => setTimeout(r, 500 * Math.pow(2, chunkAttempt - 1)))
          }
        }
      }
      result.value = combined
      return combined
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
