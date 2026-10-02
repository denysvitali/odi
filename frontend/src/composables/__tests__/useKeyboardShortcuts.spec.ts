import { mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { describe, it, expect, vi } from 'vitest'
import { useKeyboardShortcuts } from '../useKeyboardShortcuts'

function setup(handler: (event: KeyboardEvent) => void) {
  return mount(
    defineComponent({
      setup() {
        useKeyboardShortcuts([{ key: 'g', description: 'Navigate', handler }])
        return () => null
      }
    })
  )
}
describe('global keyboard shortcuts', () => {
  it('leaves selects, editable controls, IME and already-handled events alone', () => {
    const handler = vi.fn()
    const wrapper = setup(handler)
    for (const tag of ['input', 'textarea', 'select']) {
      const element = document.createElement(tag)
      document.body.appendChild(element)
      element.dispatchEvent(
        new KeyboardEvent('keydown', { key: 'g', bubbles: true, cancelable: true })
      )
      element.remove()
    }
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'g', isComposing: true }))
    const handled = new KeyboardEvent('keydown', { key: 'g', cancelable: true })
    handled.preventDefault()
    window.dispatchEvent(handled)
    expect(handler).not.toHaveBeenCalled()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'g', cancelable: true }))
    expect(handler).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })
  it('detaches the global handler when its consumer leaves', () => {
    const handler = vi.fn()
    const wrapper = setup(handler)
    wrapper.unmount()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'g' }))
    expect(handler).not.toHaveBeenCalled()
  })
})
