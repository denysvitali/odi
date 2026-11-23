import '@testing-library/jest-dom/vitest'
import { cleanup } from '@testing-library/react'
import { afterEach, vi } from 'vitest'

afterEach(() => cleanup())

// jsdom lacks these browser APIs that Mantine relies on.
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    addListener: vi.fn(),
    removeListener: vi.fn(),
    dispatchEvent: vi.fn()
  }))
})
class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}
class IntersectionObserverStub extends ResizeObserverStub {
  takeRecords() {
    return []
  }
}
window.ResizeObserver = ResizeObserverStub as unknown as typeof ResizeObserver
window.IntersectionObserver = IntersectionObserverStub as unknown as typeof IntersectionObserver
window.HTMLElement.prototype.scrollIntoView = () => {}
