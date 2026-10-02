import { mount } from '@vue/test-utils'
import { defineComponent, nextTick } from 'vue'
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { useTheme } from '../useTheme'

let listener: ((event: MediaQueryListEvent) => void) | undefined
let add: ReturnType<typeof vi.fn>
let remove: ReturnType<typeof vi.fn>
beforeEach(() => {
  localStorage.clear()
  localStorage.setItem('odi-theme', 'system')
  document.documentElement.classList.remove('dark')
  add = vi.fn((_, callback) => {
    listener = callback
  })
  remove = vi.fn()
  vi.stubGlobal(
    'matchMedia',
    vi.fn(() => ({ matches: false, addEventListener: add, removeEventListener: remove }))
  )
})
afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})
function setup() {
  let theme!: ReturnType<typeof useTheme>
  const wrapper = mount(
    defineComponent({
      setup() {
        theme = useTheme()
        return () => null
      }
    })
  )
  return { wrapper, theme }
}
describe('theme lifecycle', () => {
  it('removes the system-preference listener when its consumer leaves', () => {
    const { wrapper } = setup()
    expect(add).toHaveBeenCalledWith('change', expect.any(Function))
    wrapper.unmount()
    expect(remove).toHaveBeenCalledWith('change', listener)
  })
  it('responds to system changes while retaining a manually selected theme', async () => {
    const { wrapper, theme } = setup()
    listener!({ matches: true } as MediaQueryListEvent)
    await nextTick()
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    theme.setTheme('light')
    listener!({ matches: true } as MediaQueryListEvent)
    await nextTick()
    expect(document.documentElement.classList.contains('dark')).toBe(false)
    wrapper.unmount()
  })
  it('applies a manual theme even if browser storage rejects it', () => {
    const { wrapper, theme } = setup()
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('Storage unavailable')
    })
    expect(() => theme.setTheme('dark')).not.toThrow()
    expect(document.documentElement.classList.contains('dark')).toBe(true)
    wrapper.unmount()
  })
})
