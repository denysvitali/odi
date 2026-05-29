import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

// ---------------------------------------------------------------------------
// Mock pdfjs-dist
// ---------------------------------------------------------------------------

// We need to mock the module before importing the subject under test.
// The mock provides a minimal PDFDocumentProxy / PDFPageProxy surface.

const mockGetDocument = vi.fn()
const mockGlobalWorkerOptions = { workerSrc: '' }

vi.mock('pdfjs-dist', () => ({
  getDocument: mockGetDocument,
  GlobalWorkerOptions: mockGlobalWorkerOptions,
}))

vi.mock('pdfjs-dist/build/pdf.worker.min.mjs?url', () => ({
  default: 'mock-worker-url.js',
}))

// ---------------------------------------------------------------------------
// Canvas / OffscreenCanvas mocks
// ---------------------------------------------------------------------------

function createMockCanvas() {
  return {
    width: 0,
    height: 0,
    getContext: vi.fn().mockReturnValue({
      drawImage: vi.fn(),
    }),
    toBlob: vi.fn((cb: BlobCallback) => {
      const blob = new Blob(['fake-jpeg-data'], { type: 'image/jpeg' })
      cb(blob)
    }),
  }
}

// ---------------------------------------------------------------------------
// Helper: create a fake PDF File
// ---------------------------------------------------------------------------

function createFakePdfFile(name: string, content = 'fake-pdf-content'): File {
  const blob = new Blob([content], { type: 'application/pdf' })
  return new File([blob], name, { type: 'application/pdf' })
}

// ---------------------------------------------------------------------------
// Helper: build a mock PDFDocumentProxy with N pages
// ---------------------------------------------------------------------------

function buildMockPage() {
  return {
    getViewport: vi.fn().mockReturnValue({ width: 612, height: 792 }),
    render: vi.fn().mockReturnValue({ promise: Promise.resolve() }),
    cleanup: vi.fn(),
  }
}

function buildMockDocument(pageCount: number) {
  const pages = Array.from({ length: pageCount }, () => buildMockPage())
  return {
    numPages: pageCount,
    getPage: vi.fn().mockImplementation((num: number) => Promise.resolve(pages[num - 1])),
    destroy: vi.fn(),
    _pages: pages,
  }
}

function setupPdfjsMock(pageCount: number) {
  const doc = buildMockDocument(pageCount)
  mockGetDocument.mockReturnValue({
    promise: Promise.resolve(doc),
  })
  return doc
}

function setupPdfjsError(err: Error) {
  mockGetDocument.mockReturnValue({
    promise: Promise.reject(err),
  })
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('pdfSplitter', () => {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let originalDocument: any
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let originalOffscreenCanvas: any

  beforeEach(() => {
    vi.clearAllMocks()

    // Save originals
    originalDocument = globalThis.document
    originalOffscreenCanvas = (globalThis as Record<string, unknown>).OffscreenCanvas

    // Force use of the document.createElement('canvas') path by removing
    // OffscreenCanvas. This is simpler to mock.
    ;(globalThis as Record<string, unknown>).OffscreenCanvas = undefined

    // Mock document.createElement for canvas
    globalThis.document = {
      ...globalThis.document,
      createElement: vi.fn().mockImplementation((tag: string) => {
        if (tag === 'canvas') return createMockCanvas()
        return originalDocument.createElement(tag)
      }),
    } as unknown as Document
  })

  afterEach(() => {
    globalThis.document = originalDocument
    ;(globalThis as Record<string, unknown>).OffscreenCanvas = originalOffscreenCanvas
    vi.restoreAllMocks()
  })

  // We import the module inside each test (or in a beforeAll) to pick up the
  // mocks. Dynamic import is needed because vi.mock is hoisted.
  async function loadModule() {
    return await import('@/lib/pdfSplitter')
  }

  // -----------------------------------------------------------------------
  // Single-page PDF
  // -----------------------------------------------------------------------

  it('splits a 1-page PDF into a single JPEG file', async () => {
    setupPdfjsMock(1)
    const { splitPdf } = await loadModule()

    const file = createFakePdfFile('test.pdf')
    const result = await splitPdf(file)

    expect(result.pageCount).toBe(1)
    expect(result.files).toHaveLength(1)
    expect(result.files[0].name).toBe('test-p1.jpg')
    expect(result.files[0].type).toBe('image/jpeg')
  })

  // -----------------------------------------------------------------------
  // Multi-page PDF
  // -----------------------------------------------------------------------

  it('splits a multi-page PDF into the correct number of files', async () => {
    setupPdfjsMock(4)
    const { splitPdf } = await loadModule()

    const file = createFakePdfFile('multipage.pdf')
    const result = await splitPdf(file)

    expect(result.pageCount).toBe(4)
    expect(result.files).toHaveLength(4)
  })

  // -----------------------------------------------------------------------
  // Output file naming: p1, p2, etc.
  // -----------------------------------------------------------------------

  it('names output files correctly as p1, p2, p3...', async () => {
    setupPdfjsMock(3)
    const { splitPdf } = await loadModule()

    const file = createFakePdfFile('invoice-2024.pdf')
    const result = await splitPdf(file)

    expect(result.files[0].name).toBe('invoice-2024-p1.jpg')
    expect(result.files[1].name).toBe('invoice-2024-p2.jpg')
    expect(result.files[2].name).toBe('invoice-2024-p3.jpg')
  })

  it('strips the .pdf extension from the base name', async () => {
    setupPdfjsMock(1)
    const { splitPdf } = await loadModule()

    const file = createFakePdfFile('document.PDF')
    const result = await splitPdf(file)

    expect(result.files[0].name).toBe('document-p1.jpg')
  })

  // -----------------------------------------------------------------------
  // Error handling for corrupt PDFs
  // -----------------------------------------------------------------------

  it('throws a descriptive error for corrupt PDFs', async () => {
    setupPdfjsError(new Error('Invalid PDF structure'))
    const { splitPdf } = await loadModule()

    const file = createFakePdfFile('corrupt.pdf', 'not-a-real-pdf')
    await expect(splitPdf(file)).rejects.toThrow('Failed to read PDF')
  })

  it('throws a specific error for password-protected PDFs', async () => {
    setupPdfjsError(new Error('password required'))
    const { splitPdf } = await loadModule()

    const file = createFakePdfFile('protected.pdf')
    await expect(splitPdf(file)).rejects.toThrow('password-protected')
  })

  it('throws for encrypted PDFs', async () => {
    setupPdfjsError(new Error('document is encrypted'))
    const { splitPdf } = await loadModule()

    const file = createFakePdfFile('encrypted.pdf')
    await expect(splitPdf(file)).rejects.toThrow('password-protected')
  })

  it('throws for a PDF with zero pages', async () => {
    setupPdfjsMock(0)
    const { splitPdf } = await loadModule()

    const file = createFakePdfFile('empty.pdf')
    await expect(splitPdf(file)).rejects.toThrow('no pages')
  })

  // -----------------------------------------------------------------------
  // Progress callback
  // -----------------------------------------------------------------------

  it('calls the progress callback for each page', async () => {
    setupPdfjsMock(3)
    const { splitPdf } = await loadModule()

    const onProgress = vi.fn()
    const file = createFakePdfFile('progress.pdf')
    await splitPdf(file, onProgress)

    expect(onProgress).toHaveBeenCalledTimes(3)
    expect(onProgress).toHaveBeenCalledWith(1, 3)
    expect(onProgress).toHaveBeenCalledWith(2, 3)
    expect(onProgress).toHaveBeenCalledWith(3, 3)
  })

  it('does not throw when progress callback is not provided', async () => {
    setupPdfjsMock(2)
    const { splitPdf } = await loadModule()

    const file = createFakePdfFile('noprogress.pdf')
    await expect(splitPdf(file)).resolves.toBeDefined()
  })

  // -----------------------------------------------------------------------
  // getDocument is called with the correct arguments
  // -----------------------------------------------------------------------

  it('passes the file ArrayBuffer to pdfjs.getDocument', async () => {
    setupPdfjsMock(1)
    const { splitPdf } = await loadModule()

    const file = createFakePdfFile('test.pdf')
    await splitPdf(file)

    expect(mockGetDocument).toHaveBeenCalledTimes(1)
    const callArgs = mockGetDocument.mock.calls[0][0]
    expect(callArgs.data).toBeInstanceOf(ArrayBuffer)
  })

  // -----------------------------------------------------------------------
  // Non-Error exceptions from pdfjs are also handled
  // -----------------------------------------------------------------------

  it('handles non-Error rejections from pdfjs gracefully', async () => {
    setupPdfjsError('string error' as unknown as Error)
    const { splitPdf } = await loadModule()

    const file = createFakePdfFile('bad.pdf')
    await expect(splitPdf(file)).rejects.toThrow('Failed to read PDF')
  })

  // -----------------------------------------------------------------------
  // Page cleanup and document destroy
  // -----------------------------------------------------------------------

  it('calls cleanup on each page after rendering', async () => {
    const doc = setupPdfjsMock(2)
    const { splitPdf } = await loadModule()

    const file = createFakePdfFile('cleanup.pdf')
    await splitPdf(file)

    for (const page of doc._pages) {
      expect(page.cleanup).toHaveBeenCalled()
    }
  })

  it('calls destroy on the PDF document after processing', async () => {
    const doc = setupPdfjsMock(2)
    const { splitPdf } = await loadModule()

    const file = createFakePdfFile('destroy.pdf')
    await splitPdf(file)

    expect(doc.destroy).toHaveBeenCalledTimes(1)
  })
})
