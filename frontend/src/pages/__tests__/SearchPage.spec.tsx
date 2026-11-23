import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { api } from '@/api/client'
import { renderWithApp } from '@/test/render'
import SearchPage from '../SearchPage'

const hit = (id: string, title: string) => ({ _id: id, _source: { title, text: '' } })

beforeEach(() => {
  window._settings = { apiUrl: 'https://api.test/api/v1', opensearchUrl: '' }
  vi.spyOn(api, 'getThumbnail').mockRejectedValue(new Error('none'))
  vi.spyOn(api, 'getReminders').mockResolvedValue({ reminders: [], days: 90 })
  vi.spyOn(api, 'searchFacets').mockResolvedValue({} as never)
})
afterEach(() => {
  vi.restoreAllMocks()
  localStorage.clear()
})

describe('SearchPage', () => {
  it('shows recent documents when not searching', async () => {
    vi.spyOn(api, 'listDocuments').mockResolvedValue({
      hits: { hits: [hit('a_1', 'Recent invoice')], total: { value: 1, relation: 'eq' } }
    })
    renderWithApp(<SearchPage />)
    expect(await screen.findByText('Recent invoice')).toBeInTheDocument()
    expect(screen.getByText('Recently added')).toBeInTheDocument()
  })

  it('searches from the URL and paginates with the scroll cursor', async () => {
    const search = vi
      .spyOn(api, 'search')
      .mockResolvedValueOnce({
        hits: { hits: [hit('a_1', 'First')], total: { value: 2, relation: 'eq' } },
        _scroll_id: 'cursor'
      })
      .mockResolvedValueOnce({
        hits: { hits: [hit('a_2', 'Second')], total: { value: 2, relation: 'eq' } }
      })
    renderWithApp(<SearchPage />, '/?q=tax&docTypes=invoice')
    expect(await screen.findByRole('heading', { name: '2 results' })).toBeInTheDocument()
    expect(search).toHaveBeenCalledWith({
      searchTerm: 'tax',
      filters: { docTypes: ['invoice'] },
      size: 24
    })
    await userEvent.click(screen.getByRole('button', { name: 'Load more documents' }))
    expect(await screen.findByText('Second')).toBeInTheDocument()
    expect(search).toHaveBeenLastCalledWith({ scrollId: 'cursor' })
  })

  it('surfaces a retryable error instead of an empty state', async () => {
    const search = vi.spyOn(api, 'search').mockRejectedValue(new Error('down'))
    renderWithApp(<SearchPage />, '/?q=tax')
    expect(await screen.findByRole('alert')).toHaveTextContent('Search could not be loaded')
    expect(screen.queryByText(/No results/)).not.toBeInTheDocument()
    search.mockResolvedValue({
      hits: { hits: [hit('a_1', 'Back')], total: { value: 1, relation: 'eq' } }
    })
    await userEvent.click(screen.getByRole('button', { name: 'Try again' }))
    expect(await screen.findByText('Back')).toBeInTheDocument()
  })

  it('submits a search into the URL and records it as recent', async () => {
    vi.spyOn(api, 'listDocuments').mockResolvedValue({
      hits: { hits: [], total: { value: 0, relation: 'eq' } }
    })
    vi.spyOn(api, 'search').mockResolvedValue({
      hits: { hits: [], total: { value: 0, relation: 'eq' } }
    })
    const { router } = renderWithApp(<SearchPage />)
    await userEvent.type(
      screen.getByRole('searchbox', { name: 'Search documents' }),
      'insurance{Enter}'
    )
    await waitFor(() => expect(router.state.location.search).toBe('?q=insurance'))
    expect(JSON.parse(localStorage.getItem('odi-recent-searches')!)).toEqual(['insurance'])
  })
})
