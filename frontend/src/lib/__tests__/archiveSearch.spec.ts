import { describe, it, expect } from 'vitest'
import type { LocationQuery } from 'vue-router'
import { archiveQuery, parseArchiveQuery, hasArchiveSearch } from '../archiveSearch'

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
    expect(parseArchiveQuery(archiveQuery('', filters) as LocationQuery)).toEqual({
      term: '',
      filters
    })
    expect(hasArchiveSearch('', { docTypes: ['invoice'] })).toBe(true)
    expect(hasArchiveSearch('', { hasBarcode: false })).toBe(true)
    expect(hasArchiveSearch('  ', {})).toBe(false)
  })
  it('preserves company names and tags containing commas', () => {
    const filters = {
      companies: ['Sample, Inc.', 'Another company'],
      tags: ['[archived]', 'one,two']
    }
    expect(parseArchiveQuery(archiveQuery('', filters) as LocationQuery).filters).toEqual(filters)
    expect(parseArchiveQuery({ companies: '[malformed' }).filters).toEqual({})
  })
  it('ignores malformed and unrelated query values', () => {
    expect(
      parseArchiveQuery({
        q: ['one', 'two'],
        docTypes: ['', 'invoice'],
        hasBarcode: 'invalid',
        token: 'synthetic'
      })
    ).toEqual({ term: '', filters: {} })
    expect(archiveQuery('  synthetic  ', { companies: [], titleFilter: ' ' })).toEqual({
      q: 'synthetic'
    })
  })
})
