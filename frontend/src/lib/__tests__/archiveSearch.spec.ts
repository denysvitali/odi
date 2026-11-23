import { describe, it, expect } from 'vitest'
import { archiveQuery, countFilters, hasArchiveSearch, parseArchiveQuery } from '../archiveSearch'

describe('archive search links', () => {
  it('round trips every filter, including false and filter-only queries', () => {
    const filters = {
      companies: ['Synthetic company'],
      docTypes: ['invoice'],
      tags: ['paid'],
      dateFrom: '2026-01-01',
      dateTo: '2026-12-31',
      hasBarcode: false,
      titleFilter: 'Invoice'
    }
    expect(parseArchiveQuery(archiveQuery('', filters))).toEqual({ term: '', filters })
    expect(hasArchiveSearch('', { docTypes: ['invoice'] })).toBe(true)
    expect(hasArchiveSearch('', { hasBarcode: false })).toBe(true)
    expect(hasArchiveSearch('  ', {})).toBe(false)
  })

  it('preserves company names and tags containing commas', () => {
    const filters = {
      companies: ['Sample, Inc.', 'Another company'],
      tags: ['[archived]', 'one,two']
    }
    expect(parseArchiveQuery(archiveQuery('', filters)).filters).toEqual(filters)
    expect(parseArchiveQuery(new URLSearchParams({ companies: '[malformed' })).filters).toEqual({})
  })

  it('ignores malformed and unrelated query values', () => {
    const params = new URLSearchParams('q=one&q=two&docTypes=&hasBarcode=invalid&token=synthetic')
    expect(parseArchiveQuery(params)).toEqual({ term: '', filters: {} })
    expect(archiveQuery('  synthetic  ', { companies: [], titleFilter: ' ' }).toString()).toBe(
      'q=synthetic'
    )
  })

  it('counts active filters', () => {
    expect(countFilters({ companies: ['a', 'b'], hasBarcode: false, dateFrom: '2026-01-01' })).toBe(
      4
    )
    expect(countFilters({})).toBe(0)
  })
})
