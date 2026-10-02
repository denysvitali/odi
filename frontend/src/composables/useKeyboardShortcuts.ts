import { onMounted, onUnmounted } from 'vue'

type Handler = (e: KeyboardEvent) => void

export interface Shortcut {
  key: string
  meta?: boolean
  ctrl?: boolean
  shift?: boolean
  alt?: boolean
  description: string
  when?: () => boolean
  handler: Handler
  allowInInput?: boolean
}

function isTypingTarget(e: KeyboardEvent): boolean {
  const target = e.target
  if (!(target instanceof HTMLElement)) return false
  const tag = target.tagName
  return (
    tag === 'INPUT' ||
    tag === 'TEXTAREA' ||
    tag === 'SELECT' ||
    target.isContentEditable ||
    !!target.closest('[role=combobox]')
  )
}

export function useKeyboardShortcuts(shortcuts: Shortcut[]) {
  const handler = (e: KeyboardEvent) => {
    if (e.defaultPrevented || e.isComposing) return
    const typing = isTypingTarget(e)
    for (const s of shortcuts) {
      if (typing && !s.allowInInput) continue
      if (s.when && !s.when()) continue
      const needMeta = !!s.meta
      const hasMeta = e.metaKey || e.ctrlKey
      if (needMeta !== hasMeta) continue
      if (!!s.shift !== e.shiftKey) continue
      if (!!s.alt !== e.altKey) continue
      if (e.key.toLowerCase() !== s.key.toLowerCase()) continue
      e.preventDefault()
      s.handler(e)
      break
    }
  }

  onMounted(() => window.addEventListener('keydown', handler))
  onUnmounted(() => window.removeEventListener('keydown', handler))

  return { shortcuts }
}
