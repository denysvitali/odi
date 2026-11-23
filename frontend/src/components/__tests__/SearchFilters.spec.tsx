import { describe, expect, it, vi } from 'vitest'
import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { SearchFilters } from '@/components/search/SearchFilters'
import { renderWithApp } from '@/test/render'

const facets = {
  companies: [{ key: 'Sample AG', count: 3 }],
  docTypes: [{ key: 'invoice', count: 2 }],
  tags: [],
  barcodeCount: 1,
  totalHits: 3
}

describe('SearchFilters', () => {
  it('toggles companies and keeps selected values without a facet bucket', async () => {
    const onChange = vi.fn()
    renderWithApp(
      <SearchFilters filters={{ companies: ['Gone GmbH'] }} facets={facets} onChange={onChange} />
    )
    expect(screen.getByLabelText('Gone GmbH')).toBeChecked()
    await userEvent.click(screen.getByLabelText(/Sample AG/))
    expect(onChange).toHaveBeenLastCalledWith({ companies: ['Gone GmbH', 'Sample AG'] })
  })

  it('sets date bounds and clears all filters', async () => {
    const onChange = vi.fn()
    renderWithApp(
      <SearchFilters filters={{ hasBarcode: true }} facets={facets} onChange={onChange} />
    )
    await userEvent.type(screen.getByLabelText('From'), '2026-01-01')
    expect(onChange).toHaveBeenLastCalledWith({
      hasBarcode: true,
      dateFrom: '2026-01-01',
      dateTo: undefined
    })
    await userEvent.click(screen.getByRole('button', { name: 'Clear all' }))
    expect(onChange).toHaveBeenLastCalledWith({})
  })
})
