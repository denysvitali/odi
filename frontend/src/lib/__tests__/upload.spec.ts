import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { uploadFiles } from '../upload'
import { STORAGE_KEYS } from '../constants'

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
  respond(status = 200) {
    this.status = status
    this.dispatchEvent(new Event('load'))
  }
}

const page = () => new File(['test'], 'page.png')

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

describe('uploadFiles', () => {
  it('sends the bearer token and combines chunk results under one scan', async () => {
    const task = uploadFiles(Array.from({ length: 26 }, page))
    expect(FakeXhr.instances[0].headers.get('authorization')).toBe('Bearer test-token')
    FakeXhr.instances[0].respond()
    await vi.waitFor(() => expect(FakeXhr.instances).toHaveLength(2))
    FakeXhr.instances[1].respond()
    const result = await task
    expect(result.processed).toBe(2)
  })

  it('stops on abort without retrying', async () => {
    const controller = new AbortController()
    const task = uploadFiles([page()], { signal: controller.signal })
    controller.abort()
    await expect(task).rejects.toMatchObject({ name: 'AbortError' })
    expect(FakeXhr.instances).toHaveLength(1)
  })

  it('retries network errors but not client errors', async () => {
    vi.useFakeTimers()
    const task = uploadFiles([page()])
    FakeXhr.instances[0].dispatchEvent(new Event('error'))
    await vi.advanceTimersByTimeAsync(500)
    expect(FakeXhr.instances).toHaveLength(2)
    FakeXhr.instances[1].responseText = '{"error":"Bad file"}'
    FakeXhr.instances[1].respond(400)
    await expect(task).rejects.toThrow('Bad file')
  })

  it('cancels during retry backoff', async () => {
    vi.useFakeTimers()
    const controller = new AbortController()
    const task = uploadFiles([page()], { signal: controller.signal })
    const settled = expect(task).rejects.toMatchObject({ name: 'AbortError' })
    FakeXhr.instances[0].dispatchEvent(new Event('error'))
    await Promise.resolve()
    controller.abort()
    await vi.runAllTimersAsync()
    await settled
    expect(FakeXhr.instances).toHaveLength(1)
  })
})
