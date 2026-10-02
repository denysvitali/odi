import { beforeEach, describe, it, expect, vi } from 'vitest'
import { useSavedSearches } from '../useSavedSearches'

beforeEach(() => {
  localStorage.clear()
  vi.restoreAllMocks()
})
describe('device-local saved searches', () => {
  it('persists filters, restores them, updates duplicates, and deletes them', () => {
    const saved = useSavedSearches()
    expect(saved.save('Invoices', '', { docTypes: ['invoice'], hasBarcode: false })).toBe(true)
    const restored = useSavedSearches()
    expect(restored.searches.value[0]).toMatchObject({
      name: 'Invoices',
      term: '',
      filters: { docTypes: ['invoice'], hasBarcode: false }
    })
    restored.save('Renamed invoices', '', { docTypes: ['invoice'], hasBarcode: false })
    expect(restored.searches.value).toHaveLength(1)
    restored.remove(restored.searches.value[0].id)
    expect(useSavedSearches().searches.value).toEqual([])
  })
  it('handles malformed storage and keeps valid entries with sanitized filters', () => {
    localStorage.setItem(
      'odi-saved-searches',
      JSON.stringify([
        null,
        { id: 'bad', term: 1 },
        {
          id: 'valid',
          name: 'Valid',
          term: 'synthetic',
          filters: {
            tags: 'wrong',
            companies: [null, 2, 'Sample'],
            hasBarcode: 'wrong',
            unexpected: 'drop'
          }
        }
      ])
    )
    expect(useSavedSearches().searches.value).toEqual([
      { id: 'valid', name: 'Valid', term: 'synthetic', filters: { companies: ['Sample'] } }
    ])
    localStorage.setItem('odi-saved-searches', '{')
    expect(useSavedSearches().error.value).toContain('could not be read')
  })
  it('reports failed writes without claiming the search was saved', () => {
    const saved = useSavedSearches()
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('Full')
    })
    expect(saved.save('Sample', 'synthetic', {})).toBe(false)
    expect(saved.searches.value).toEqual([])
    expect(saved.error.value).toContain('not saved')
  })
  it('limits storage and rejects empty searches', () => {
    const saved = useSavedSearches()
    expect(saved.save('Empty', '', {})).toBe(false)
    for (let i = 0; i < 12; i++) expect(saved.save(`Sample ${i}`, `term ${i}`, {})).toBe(true)
    expect(saved.save('Overflow', 'another', {})).toBe(false)
    expect(saved.searches.value).toHaveLength(12)
  })
})
