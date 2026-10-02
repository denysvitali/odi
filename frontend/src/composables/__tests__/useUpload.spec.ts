import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { effectScope } from 'vue'
import { useUpload } from '../useUpload'
import { STORAGE_KEYS } from '@/lib/constants'

class FakeXhr extends EventTarget {
  static instances: FakeXhr[] = []
  upload = new EventTarget()
  headers = new Map<string, string>()
  status = 200
  responseText = JSON.stringify({ processed: 1, duplicates: 0, failed: 0, pages: [] })
  constructor() {
    super()
    FakeXhr.instances.push(this)
  }
  open = vi.fn()
  send = vi.fn()
  setRequestHeader(name: string, value: string) {
    this.headers.set(name.toLowerCase(), value)
  }
  abort() {
    this.dispatchEvent(new Event('abort'))
  }
}

beforeEach(() => {
  FakeXhr.instances = []
  vi.stubGlobal('XMLHttpRequest', FakeXhr)
  window._settings = { apiUrl: 'https://api.test/api/v1', opensearchUrl: '' }
  localStorage.setItem(STORAGE_KEYS.API_TOKEN, 'test-token')
})
afterEach(() => {
  vi.unstubAllGlobals()
  vi.useRealTimers()
  localStorage.clear()
})

describe('upload lifecycle', () => {
  it('sends the configured bearer token and stops canceled uploads without retries', async () => {
    vi.useFakeTimers()
    const state = useUpload()
    const task = state.upload([new File(['test'], 'page.png')])
    expect(FakeXhr.instances[0].headers.get('authorization')).toBe('Bearer test-token')
    state.abort()
    // Busy remains true until the active upload actually settles.
    expect(state.uploading.value).toBe(true)
    await task
    await vi.runAllTimersAsync()
    expect(FakeXhr.instances).toHaveLength(1)
    expect(state.uploading.value).toBe(false)
    expect(state.error.value).toBeNull()
  })

  it('stops the active upload when its view scope is disposed', async () => {
    const scope = effectScope()
    const state = scope.run(() => useUpload())!
    const task = state.upload([new File(['test'], 'page.png')])
    scope.stop()
    await task
    expect(FakeXhr.instances).toHaveLength(1)
    expect(state.uploading.value).toBe(false)
    expect(state.error.value).toBeNull()
  })

  it('cancels during retry backoff and does not send subsequent chunks', async () => {
    vi.useFakeTimers()
    const state = useUpload()
    const task = state.upload(Array.from({ length: 26 }, () => new File(['test'], 'page.png')))
    FakeXhr.instances[0].dispatchEvent(new Event('error'))
    await Promise.resolve()
    state.abort()
    await vi.runAllTimersAsync()
    await task
    expect(FakeXhr.instances).toHaveLength(1)
  })
})
