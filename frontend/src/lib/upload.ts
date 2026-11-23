import { buildHeaders } from '@/api/client'
import { getApiUrl } from './config'

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

export interface UploadOptions {
  signal?: AbortSignal
  onProgress?: (percent: number) => void
  onAttempt?: (attempt: number) => void
}

const MAX_ATTEMPTS = 3
const CHUNK_SIZE = 25

class UploadError extends Error {
  constructor(
    message: string,
    readonly status = 0
  ) {
    super(message)
  }
}

function sendChunk(
  files: File[],
  scanID: string,
  sequenceOffset: number,
  onBytes: (loaded: number) => void,
  signal?: AbortSignal
) {
  return new Promise<UploadResult>((resolve, reject) => {
    const form = new FormData()
    form.append('scanID', scanID)
    form.append('sequenceOffset', String(sequenceOffset))
    for (const file of files) form.append('files', file)

    const xhr = new XMLHttpRequest()
    const onAbort = () => xhr.abort()
    signal?.addEventListener('abort', onAbort, { once: true })
    const settle = () => signal?.removeEventListener('abort', onAbort)

    xhr.upload.addEventListener('progress', (e) => e.lengthComputable && onBytes(e.loaded))
    xhr.addEventListener('load', () => {
      settle()
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          resolve(JSON.parse(xhr.responseText) as UploadResult)
        } catch {
          reject(new UploadError('Invalid response from server', xhr.status))
        }
        return
      }
      let message = `Upload failed (${xhr.status})`
      try {
        message = JSON.parse(xhr.responseText).error || message
      } catch {
        /* keep the status message */
      }
      reject(new UploadError(message, xhr.status))
    })
    xhr.addEventListener('error', () => (settle(), reject(new UploadError('Network error'))))
    xhr.addEventListener(
      'abort',
      () => (settle(), reject(new DOMException('Aborted', 'AbortError')))
    )
    xhr.open('POST', `${getApiUrl()}/upload`)
    new Headers(buildHeaders(undefined)).forEach((value, name) => xhr.setRequestHeader(name, value))
    xhr.send(form)
  })
}

const wait = (ms: number, signal?: AbortSignal) =>
  new Promise<void>((resolve, reject) => {
    const timer = setTimeout(resolve, ms)
    signal?.addEventListener(
      'abort',
      () => (clearTimeout(timer), reject(new DOMException('Aborted', 'AbortError'))),
      { once: true }
    )
  })

/**
 * Upload pages as one scan, in chunks, retrying network/5xx failures with backoff.
 * Rejects with an AbortError when `signal` fires.
 */
export async function uploadFiles(files: File[], opts: UploadOptions = {}): Promise<UploadResult> {
  const { signal, onProgress, onAttempt } = opts
  const scanID = crypto.randomUUID()
  const totalBytes = files.reduce((n, f) => n + f.size, 0) || 1
  const combined: UploadResult = { scanID, processed: 0, duplicates: 0, failed: 0, pages: [] }
  let uploaded = 0
  let attempts = 0

  for (let start = 0; start < files.length; start += CHUNK_SIZE) {
    const chunk = files.slice(start, start + CHUNK_SIZE)
    for (let attempt = 1; ; attempt++) {
      signal?.throwIfAborted()
      onAttempt?.(++attempts)
      try {
        const data = await sendChunk(
          chunk,
          scanID,
          start,
          (loaded) => onProgress?.(Math.round(((uploaded + loaded) / totalBytes) * 100)),
          signal
        )
        combined.processed += data.processed
        combined.duplicates += data.duplicates
        combined.failed += data.failed
        combined.pages.push(...data.pages)
        uploaded += chunk.reduce((n, f) => n + f.size, 0)
        onProgress?.(Math.round((uploaded / totalBytes) * 100))
        break
      } catch (err) {
        const retryable = err instanceof UploadError && (err.status === 0 || err.status >= 500)
        if (!retryable || attempt >= MAX_ATTEMPTS) throw err
        await wait(500 * 2 ** (attempt - 1), signal)
      }
    }
  }
  return combined
}

export const isPdf = (f: File) => f.type === 'application/pdf' || /\.pdf$/i.test(f.name)
export const isUploadable = (f: File) => f.type.startsWith('image/') || isPdf(f)

// Hand-off for files dropped outside the upload page.
let stashed: File[] = []
export function stashFiles(files: File[]) {
  stashed = files
}
export function takeStashedFiles(): File[] {
  const files = stashed
  stashed = []
  return files
}
