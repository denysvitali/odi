import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { api } from '@/api/client'
import { DocumentDrawer } from '@/components/documents/DocumentDrawer'
import { favorites, tags } from '@/stores/local'
import { renderWithApp } from '@/test/render'

beforeEach(() => {
  vi.spyOn(api, 'getThumbnail').mockRejectedValue(new Error('none'))
  favorites.reload()
  tags.reload()
})
afterEach(() => {
  vi.restoreAllMocks()
  localStorage.clear()
})

describe('DocumentDrawer', () => {
  it('opens from the URL, shows details and the cached summary', async () => {
    vi.spyOn(api, 'getDocumentDetails').mockResolvedValue({
      id: 'scan_1',
      title: 'Electricity bill',
      text: 'Total CHF 12',
      company: { name: 'Power AG' },
      summary: 'Monthly power bill.',
      keyFacts: [{ label: 'Amount', value: 'CHF 12' }]
    })
    renderWithApp(<DocumentDrawer />, '/documents?doc=scan_1')
    expect(await screen.findByText('Electricity bill')).toBeInTheDocument()
    expect(screen.getByText('Power AG')).toBeInTheDocument()
    expect(screen.getByText('Monthly power bill.')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Regenerate summary' })).toBeInTheDocument()
  })

  it('stars and closes, removing the doc param', async () => {
    vi.spyOn(api, 'getDocumentDetails').mockResolvedValue({ id: 'scan_1', title: 'Doc' })
    const { router } = renderWithApp(<DocumentDrawer />, '/?q=x&doc=scan_1')
    await userEvent.click(await screen.findByRole('button', { name: 'Star document' }))
    expect(favorites.get()).toEqual(['scan_1'])
    await userEvent.keyboard('{Escape}')
    await waitFor(() => expect(router.state.location.search).toBe('?q=x'))
  })

  it('shows a retryable error', async () => {
    const details = vi.spyOn(api, 'getDocumentDetails').mockRejectedValueOnce(new Error('Gone'))
    renderWithApp(<DocumentDrawer />, '/?doc=scan_1')
    expect(await screen.findByRole('alert')).toHaveTextContent('Gone')
    details.mockResolvedValue({ id: 'scan_1', title: 'Recovered' })
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(await screen.findByText('Recovered')).toBeInTheDocument()
  })
})
