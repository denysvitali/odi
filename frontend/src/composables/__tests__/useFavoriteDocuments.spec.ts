import { effectScope, nextTick, ref, type EffectScope } from 'vue'
import { flushPromises } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import { useFavoriteDocuments } from '../useFavoriteDocuments'
import type { DocumentDetails } from '@/types/documents'

vi.mock('@/api/client', () => ({ api: { getDocumentDetails: vi.fn() } }))
const fetchDetails = vi.mocked(api.getDocumentDetails)
let scope: EffectScope
const setup = (saved: string[]) => {
  scope = effectScope()
  const ids = ref(saved)
  const favorites = scope.run(() => useFavoriteDocuments(ids))!
  return { ids, favorites }
}
function deferred() {
  let resolve!: (details: DocumentDetails) => void
  const promise = new Promise<DocumentDetails>((res) => {
    resolve = res
  })
  return { promise, resolve }
}
beforeEach(() => {
  fetchDetails.mockReset()
})
afterEach(() => scope?.stop())

describe('favorite document resolution', () => {
  it('fetches saved IDs directly in saved order and maps detail metadata', async () => {
    fetchDetails.mockImplementation(async (id) => ({
      id,
      title: `Saved ${id}`,
      text: 'Synthetic document',
      primaryDate: '2026-01-01',
      tags: ['tax']
    }))
    const { favorites } = setup(['outside-first-page', 'recent'])
    await flushPromises()
    expect(fetchDetails.mock.calls.map(([id]) => id)).toEqual(['outside-first-page', 'recent'])
    expect(favorites.documents.value.map((doc) => doc._id)).toEqual([
      'outside-first-page',
      'recent'
    ])
    expect(favorites.documents.value[0]._source).toMatchObject({
      date: '2026-01-01',
      tags: ['tax']
    })
    expect(favorites.loading.value).toBe(false)
  })

  it('bounds the request pool even when favorites change while requests are in flight', async () => {
    const requests = Array.from({ length: 8 }, deferred)
    let cursor = 0
    fetchDetails.mockImplementation(() => requests[cursor++].promise)
    const { ids, favorites } = setup(['a', 'b', 'c', 'd', 'e', 'f'])
    expect(fetchDetails).toHaveBeenCalledTimes(4)
    ids.value = ['new-a', 'new-b', 'new-c', 'new-d']
    await nextTick()
    expect(fetchDetails).toHaveBeenCalledTimes(4)
    for (let i = 0; i < 4; i++) requests[i].resolve({ id: 'old', text: 'stale' })
    await flushPromises()
    expect(fetchDetails).toHaveBeenCalledTimes(8)
    expect(favorites.documents.value).toEqual([])
    expect(fetchDetails.mock.calls.slice(4).map(([id]) => id)).toEqual(ids.value)
    for (let i = 4; i < 8; i++) requests[i].resolve({ id: 'new', text: 'current' })
    await flushPromises()
    expect(favorites.documents.value.map((doc) => doc._id)).toEqual(ids.value)
  })

  it('retains successful documents and retries only failures with a fresh fetch', async () => {
    fetchDetails.mockImplementation(async (id) => {
      if (id === 'unavailable') throw new Error('synthetic failure')
      return { id, text: '' }
    })
    const { favorites } = setup(['available', 'unavailable'])
    await flushPromises()
    expect(favorites.failedCount.value).toBe(1)
    expect(favorites.documents.value.map((doc) => doc._id)).toEqual(['available'])
    fetchDetails.mockResolvedValue({ id: 'unavailable', text: '' })
    favorites.retry()
    await flushPromises()
    expect(fetchDetails).toHaveBeenLastCalledWith('unavailable', { skipCache: true })
    expect(fetchDetails).toHaveBeenCalledTimes(3)
    expect(favorites.failedCount.value).toBe(0)
    expect(favorites.documents.value).toHaveLength(2)
  })

  it('ignores an old result after removing and re-adding the same saved ID', async () => {
    const old = deferred()
    const current = deferred()
    fetchDetails.mockReturnValueOnce(old.promise).mockReturnValueOnce(current.promise)
    const { ids, favorites } = setup(['same'])
    ids.value = []
    await nextTick()
    ids.value = ['same']
    await nextTick()
    old.resolve({ id: 'same', title: 'Old', text: '' })
    await flushPromises()
    expect(favorites.documents.value).toEqual([])
    current.resolve({ id: 'same', title: 'Current', text: '' })
    await flushPromises()
    expect(favorites.documents.value[0]._source.title).toBe('Current')
  })

  it('does not launch queued work after the view is disposed', async () => {
    const request = deferred()
    fetchDetails.mockReturnValue(request.promise)
    const { favorites } = setup(['a', 'b', 'c', 'd', 'e'])
    scope.stop()
    request.resolve({ id: 'a', text: '' })
    await flushPromises()
    expect(fetchDetails).toHaveBeenCalledTimes(4)
    expect(favorites.documents.value).toEqual([])
  })
})
