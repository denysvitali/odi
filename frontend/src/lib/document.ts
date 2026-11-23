import type { Document, DocumentDetails } from '@/types/documents'
import { extractCompanyFromText, extractTitleFromText } from './documentMetadata'

export function documentTitle(doc: Pick<Document, '_source'>): string {
  return doc._source.title || extractTitleFromText(doc._source.text || '') || 'Untitled document'
}

export function documentCompany(doc: Pick<Document, '_source'>): string {
  return doc._source.company?.name || extractCompanyFromText(doc._source.text || '')
}

export function documentDate(doc: Pick<Document, '_source'>): string | undefined {
  return doc._source.date || doc._source.indexedAt
}

/** Adapt the detail endpoint's shape to the search-hit shape used by lists. */
export function detailsToDocument(id: string, details: DocumentDetails): Document {
  return {
    _id: id,
    _source: {
      title: details.title,
      text: details.text || '',
      date: details.primaryDate,
      indexedAt: details.indexedAt,
      company: details.company,
      docType: details.docType,
      tags: details.tags,
      summary: details.summary,
      keyFacts: details.keyFacts
    }
  }
}

/** Document IDs are `<scanID>_<sequenceID>`; scan IDs may themselves contain underscores. */
export function parseDocumentId(id: string): { scanID: string; sequenceID: number } | null {
  const idx = id.lastIndexOf('_')
  if (idx <= 0) return null
  const sequenceID = Number(id.slice(idx + 1))
  if (!Number.isInteger(sequenceID) || id.slice(idx + 1) === '') return null
  return { scanID: id.slice(0, idx), sequenceID }
}

export function documentsToCsv(docs: Document[]): string {
  const header = ['id', 'title', 'company', 'primaryDate', 'indexedAt']
  const quote = (value: string) => `"${value.replace(/"/g, '""')}"`
  const rows = docs.map((d) =>
    [d._id, documentTitle(d), documentCompany(d), d._source.date || '', d._source.indexedAt || '']
      .map(quote)
      .join(',')
  )
  return [header.join(','), ...rows].join('\n')
}

export function downloadBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = filename
  document.body.appendChild(a)
  a.click()
  a.remove()
  URL.revokeObjectURL(url)
}
