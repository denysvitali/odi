import * as pdfjsLib from 'pdfjs-dist'
import type { PDFDocumentProxy } from 'pdfjs-dist'

// Configure the worker for Vite using the ?url import pattern.
// This lets Vite bundle the worker file as a static asset.
import workerUrl from 'pdfjs-dist/build/pdf.worker.min.mjs?url'

pdfjsLib.GlobalWorkerOptions.workerSrc = workerUrl

/** Result of splitting a PDF into individual page images. */
export interface PdfSplitResult {
  files: File[]
  pageCount: number
}

/** Progress callback invoked after each page is rendered. */
export type PdfSplitProgressCallback = (currentPage: number, totalPages: number) => void

/**
 * Render a single PDF page to a JPEG Blob.
 *
 * Uses OffscreenCanvas when available for better performance,
 * falling back to a regular <canvas> element otherwise.
 */
async function renderPageToJpeg(
  page: pdfjsLib.PDFPageProxy,
  scale: number
): Promise<Blob> {
  const viewport = page.getViewport({ scale })

  // Prefer OffscreenCanvas (not available in all browsers / test envs)
  if (typeof OffscreenCanvas !== 'undefined') {
    const canvas = new OffscreenCanvas(viewport.width, viewport.height)
    const ctx = canvas.getContext('2d')!
    await page.render({ canvasContext: ctx as unknown as CanvasRenderingContext2D, viewport }).promise
    return canvas.convertToBlob({ type: 'image/jpeg', quality: 0.85 })
  }

  // Fallback: regular canvas
  const canvas = document.createElement('canvas')
  canvas.width = viewport.width
  canvas.height = viewport.height
  const ctx = canvas.getContext('2d')!
  await page.render({ canvasContext: ctx, viewport }).promise

  return new Promise<Blob>((resolve, reject) => {
    canvas.toBlob(
      (blob) => (blob ? resolve(blob) : reject(new Error('Canvas toBlob returned null'))),
      'image/jpeg',
      0.85
    )
  })
}

/**
 * Split a PDF File into individual JPEG page images.
 *
 * Each page is rendered at 150 DPI (sufficient for OCR, keeps sizes
 * reasonable) and returned as a File named `<original>-p<N>.jpg`.
 *
 * @param file   The PDF File to split.
 * @param onProgress  Optional callback fired after each page.
 * @returns      An object containing the array of JPEG Files and the total page count.
 * @throws       If the PDF is corrupt, password-protected, or cannot be parsed.
 */
export async function splitPdf(
  file: File,
  onProgress?: PdfSplitProgressCallback
): Promise<PdfSplitResult> {
  const arrayBuffer = await file.arrayBuffer()

  let pdf: PDFDocumentProxy
  try {
    pdf = await pdfjsLib.getDocument({ data: arrayBuffer }).promise
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err)
    // pdfjs-dist throws specific errors for encrypted docs
    if (message.includes('password') || message.includes('encrypted')) {
      throw new Error('This PDF is password-protected and cannot be processed.')
    }
    throw new Error(`Failed to read PDF: ${message}`)
  }

  const pageCount = pdf.numPages
  if (pageCount === 0) {
    throw new Error('The PDF contains no pages.')
  }

  // 150 DPI: PDF user-space is 72 units/inch, so scale = 150/72
  const scale = 150 / 72
  const baseName = file.name.replace(/\.pdf$/i, '')
  const files: File[] = []

  for (let i = 1; i <= pageCount; i++) {
    const page = await pdf.getPage(i)
    const blob = await renderPageToJpeg(page, scale)
    const pageFile = new File([blob], `${baseName}-p${i}.jpg`, { type: 'image/jpeg' })
    files.push(pageFile)
    onProgress?.(i, pageCount)
    page.cleanup()
  }

  pdf.destroy()
  return { files, pageCount }
}
