import { describe, it, expect } from 'vitest'
import { documentsToCsv, documentTitle, parseDocumentId } from '../document'
import { errorMessage } from '../utils'

describe('document helpers', () => {
  it('parses scan and sequence ids, including underscores in the scan id', () => {
    expect(parseDocumentId('scan_a_3')).toEqual({ scanID: 'scan_a', sequenceID: 3 })
    expect(parseDocumentId('scan')).toBeNull()
    expect(parseDocumentId('scan_')).toBeNull()
    expect(parseDocumentId('_1')).toBeNull()
  })

  it('falls back to an extracted or placeholder title', () => {
    expect(documentTitle({ _source: { title: 'Given', text: '' } })).toBe('Given')
    expect(documentTitle({ _source: { text: '' } })).toBe('Untitled document')
  })

  it('exports quoted CSV', () => {
    const csv = documentsToCsv([{ _id: 'a_1', _source: { title: 'Say "hi"', text: '' } }])
    expect(csv.split('\n')[1]).toBe('"a_1","Say ""hi""","","",""')
  })

  it('extracts error messages', () => {
    expect(errorMessage(new Error('boom'), 'fallback')).toBe('boom')
    expect(errorMessage('nope', 'fallback')).toBe('fallback')
  })
})
