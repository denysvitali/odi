import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { useClipboard } from '@/composables/useClipboard'

beforeEach(() => {
  vi.restoreAllMocks()
  vi.useFakeTimers()
})

afterEach(() => {
  vi.useRealTimers()
})

describe('useClipboard', () => {
  it('writes text and flips copied true then back to false after resetMs', async () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    vi.stubGlobal('navigator', { clipboard: { writeText } })

    const { copied, copy } = useClipboard(2000)
    expect(copied.value).toBe(false)

    const ok = await copy('hello')
    expect(ok).toBe(true)
    expect(writeText).toHaveBeenCalledWith('hello')
    expect(copied.value).toBe(true)

    vi.advanceTimersByTime(2000)
    expect(copied.value).toBe(false)
  })

  it('falls back to execCommand when the clipboard API throws', async () => {
    const writeText = vi.fn().mockRejectedValue(new Error('denied'))
    vi.stubGlobal('navigator', { clipboard: { writeText } })
    const execCommand = vi.fn().mockReturnValue(true)
    document.execCommand = execCommand as unknown as typeof document.execCommand

    const { copied, copy } = useClipboard()
    const ok = await copy('fallback text')

    expect(ok).toBe(true)
    expect(execCommand).toHaveBeenCalledWith('copy')
    expect(copied.value).toBe(true)
  })

  it('returns false when both the clipboard API and the fallback fail', async () => {
    vi.stubGlobal('navigator', {
      clipboard: { writeText: vi.fn().mockRejectedValue(new Error('denied')) }
    })
    document.execCommand = vi.fn(() => {
      throw new Error('no execCommand')
    }) as unknown as typeof document.execCommand

    const { copied, copy } = useClipboard()
    const ok = await copy('nope')

    expect(ok).toBe(false)
    expect(copied.value).toBe(false)
  })
})
