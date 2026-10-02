import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { effectScope, nextTick, ref } from 'vue'
import { api } from '@/api/client'
import { useDocumentMedia } from '../useDocumentMedia'

beforeEach(() => {
  class MediaURL extends URL {
    static createObjectURL = vi.fn()
    static revokeObjectURL = vi.fn()
  }
  vi.stubGlobal('URL', MediaURL)
})
afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

describe('protected media lifecycle', () => {
  it('revokes previous previews and ignores late responses after navigation', async () => {
    let finish!: (value: Blob) => void
    vi.spyOn(api, 'getThumbnail')
      .mockResolvedValueOnce(new Blob(['one']))
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            finish = resolve
          })
      )
    const create = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:preview')
    const revoke = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
    const scope = effectScope()
    const id = ref('first')
    const media = scope.run(() => useDocumentMedia(id))!
    await Promise.resolve()
    expect(media.thumbnailUrl.value).toBe('blob:preview')
    id.value = 'second'
    await nextTick()
    expect(revoke).toHaveBeenCalledWith('blob:preview')
    scope.stop()
    finish(new Blob(['late']))
    await Promise.resolve()
    expect(create).toHaveBeenCalledTimes(1)
  })

  it('shares one file fetch and blob URL between concurrent actions', async () => {
    vi.spyOn(api, 'getThumbnail').mockResolvedValue(new Blob(['preview']))
    let finish!: (value: Blob) => void
    const fetch = vi.spyOn(api, 'getFile').mockImplementation(
      () =>
        new Promise((resolve) => {
          finish = resolve
        })
    )
    const create = vi.spyOn(URL, 'createObjectURL').mockReturnValue('blob:media')
    const revoke = vi.spyOn(URL, 'revokeObjectURL').mockImplementation(() => {})
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})
    const scope = effectScope()
    const media = scope.run(() => useDocumentMedia(ref('scan_1')))!
    await Promise.resolve()
    const first = media.downloadFile()
    const second = media.downloadFile()
    expect(fetch).toHaveBeenCalledTimes(1)
    finish(new Blob(['file']))
    await Promise.all([first, second])
    expect(create).toHaveBeenCalledTimes(2) // thumbnail + shared file
    scope.stop()
    expect(revoke).toHaveBeenCalledTimes(2)
  })
})
