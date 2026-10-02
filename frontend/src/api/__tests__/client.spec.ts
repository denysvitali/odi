import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '../client'
import { STORAGE_KEYS } from '@/lib/constants'

beforeEach(() => {
  window._settings = { apiUrl: 'https://api.test/api/v1', opensearchUrl: '' }
  localStorage.setItem(STORAGE_KEYS.API_TOKEN, 'test-token')
})
afterEach(() => {
  vi.restoreAllMocks()
  localStorage.clear()
})

describe('authenticated API transport', () => {
  it('loads binary media with authorization and rejects redirect following', async () => {
    const blob = new Blob(['synthetic'], { type: 'image/png' })
    const fetch = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValue({ ok: true, status: 200, blob: async () => blob } as Response)
    expect(await api.getThumbnail('scan_1')).toBe(blob)
    expect(await api.getFile('scan_1')).toBe(blob)
    expect(fetch.mock.calls[0][0]).toBe('https://api.test/api/v1/thumbnails/scan_1')
    expect(fetch.mock.calls[1][0]).toBe('https://api.test/api/v1/files/scan/1')
    expect(new Headers(fetch.mock.calls[0][1]?.headers).get('Authorization')).toBe(
      'Bearer test-token'
    )
    expect(fetch.mock.calls[0][1]?.redirect).toBe('error')
  })

  it('does not repeat mutations after upstream errors', async () => {
    const fetch = vi
      .spyOn(globalThis, 'fetch')
      .mockResolvedValue({ ok: false, status: 503, statusText: 'Unavailable' } as Response)
    await expect(api.createShare({ scanID: 'synthetic', sequenceID: 0 })).rejects.toThrow('503')
    expect(fetch).toHaveBeenCalledTimes(1)
  })

  it('does not retry an aborted media request', async () => {
    const controller = new AbortController()
    controller.abort()
    const fetch = vi
      .spyOn(globalThis, 'fetch')
      .mockRejectedValue(new DOMException('Aborted', 'AbortError'))
    await expect(api.getFile('scan_1', controller.signal)).rejects.toThrow('Aborted')
    expect(fetch).toHaveBeenCalledTimes(1)
  })
})
