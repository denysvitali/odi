import { useEffect } from 'react'
import type { NavigateFunction } from 'react-router'

export const GO_TARGETS: Record<string, { path: string; label: string }> = {
  h: { path: '/', label: 'Search' },
  d: { path: '/documents', label: 'All documents' },
  f: { path: '/favorites', label: 'Favorites' },
  u: { path: '/upload', label: 'Upload' }
}

function isTyping(target: EventTarget | null) {
  return (
    target instanceof HTMLElement &&
    (['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName) || target.isContentEditable)
  )
}

/** Two-key "g then x" navigation, like GitHub/Gmail. */
export function useGoShortcuts(navigate: NavigateFunction) {
  useEffect(() => {
    let armedUntil = 0
    const onKey = (e: KeyboardEvent) => {
      if (e.defaultPrevented || e.metaKey || e.ctrlKey || e.altKey || isTyping(e.target)) return
      const key = e.key.toLowerCase()
      if (Date.now() < armedUntil && GO_TARGETS[key]) {
        armedUntil = 0
        e.preventDefault()
        navigate(GO_TARGETS[key].path)
      } else armedUntil = key === 'g' ? Date.now() + 1000 : 0
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [navigate])
}
