import { beforeEach, describe, it, expect, vi } from 'vitest'
import {
  addRecentSearch,
  addTag,
  allTags,
  favorites,
  recentSearches,
  removeTag,
  tags,
  toggleFavorite
} from '../local'
import { removeSearch, renameSearch, savedSearches, saveSearch } from '../savedSearches'

beforeEach(() => {
  vi.restoreAllMocks()
  localStorage.clear()
  for (const store of [favorites, tags, recentSearches, savedSearches]) store.reload()
})

describe('device-local stores', () => {
  it('toggles favorites and persists them', () => {
    toggleFavorite('a_1')
    toggleFavorite('b_2')
    toggleFavorite('a_1')
    expect(favorites.get()).toEqual(['b_2'])
    expect(JSON.parse(localStorage.getItem('odi-favorites')!)).toEqual(['b_2'])
  })

  it('adds, dedupes, and removes tags; drops empty documents', () => {
    addTag('a_1', ' paid ')
    addTag('a_1', 'paid')
    addTag('b_1', 'tax')
    expect(allTags(tags.get())).toEqual(['paid', 'tax'])
    removeTag('a_1', 'paid')
    expect(tags.get()).toEqual({ b_1: ['tax'] })
  })

  it('ignores corrupt storage', () => {
    localStorage.setItem('odi-tags', '{"a":[1,"ok"],"b":"bad"}')
    localStorage.setItem('odi-favorites', '{')
    tags.reload()
    favorites.reload()
    expect(tags.get()).toEqual({ a: ['ok'] })
    expect(favorites.get()).toEqual([])
  })

  it('keeps the ten most recent unique searches', () => {
    for (let i = 0; i < 12; i++) addRecentSearch(`term ${i}`)
    addRecentSearch('term 5')
    expect(recentSearches.get()).toHaveLength(10)
    expect(recentSearches.get()[0]).toBe('term 5')
  })
})

describe('saved searches', () => {
  it('persists filters, restores them, updates duplicates, and deletes them', () => {
    expect(saveSearch('Invoices', '', { docTypes: ['invoice'], hasBarcode: false }).ok).toBe(true)
    savedSearches.reload()
    expect(savedSearches.get()[0]).toMatchObject({
      name: 'Invoices',
      term: '',
      filters: { docTypes: ['invoice'], hasBarcode: false }
    })
    saveSearch('Renamed invoices', '', { docTypes: ['invoice'], hasBarcode: false })
    expect(savedSearches.get()).toHaveLength(1)
    removeSearch(savedSearches.get()[0].id)
    savedSearches.reload()
    expect(savedSearches.get()).toEqual([])
  })

  it('keeps valid entries with sanitized filters', () => {
    localStorage.setItem(
      'odi-saved-searches',
      JSON.stringify([
        null,
        { id: 'bad', term: 1 },
        {
          id: 'valid',
          name: 'Valid',
          term: 'synthetic',
          filters: { tags: 'wrong', companies: [null, 2, 'Sample'], hasBarcode: 'wrong', x: 1 }
        }
      ])
    )
    savedSearches.reload()
    expect(savedSearches.get()).toEqual([
      { id: 'valid', name: 'Valid', term: 'synthetic', filters: { companies: ['Sample'] } }
    ])
  })

  it('reports failed writes without claiming the search was saved', () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('Full')
    })
    const result = saveSearch('Sample', 'synthetic', {})
    expect(result.ok).toBe(false)
    expect(!result.ok && result.error).toContain('not saved')
    expect(savedSearches.get()).toEqual([])
  })

  it('renames without changing filters, order, or identifiers', () => {
    saveSearch('Invoices', '', { docTypes: ['invoice'], companies: ['Sample'] })
    saveSearch('Contracts', 'agreement', {})
    const before = savedSearches.get()
    expect(renameSearch(before[1].id, '  Household bills  ').ok).toBe(true)
    expect(savedSearches.get()).toEqual([before[0], { ...before[1], name: 'Household bills' }])
    expect(renameSearch(before[1].id, '   ').ok).toBe(false)
    expect(renameSearch('missing', 'Name').ok).toBe(false)
  })

  it('limits storage and rejects empty searches', () => {
    expect(saveSearch('Empty', '', {}).ok).toBe(false)
    for (let i = 0; i < 12; i++) expect(saveSearch(`Sample ${i}`, `term ${i}`, {}).ok).toBe(true)
    expect(saveSearch('Overflow', 'another', {}).ok).toBe(false)
    expect(savedSearches.get()).toHaveLength(12)
  })
})
