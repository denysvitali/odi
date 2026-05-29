import { ref } from 'vue'
import { logger } from '@/lib/logger'

/**
 * Copy-to-clipboard helper with a transient "copied" indicator.
 *
 * Returns a reactive `copied` flag that flips to true on a successful copy and
 * back to false after `resetMs`. Falls back to a hidden textarea +
 * `execCommand('copy')` for older browsers and insecure (non-HTTPS) contexts
 * where the async Clipboard API is unavailable.
 */
export function useClipboard(resetMs = 2000) {
  const copied = ref(false)
  let timer: ReturnType<typeof setTimeout> | null = null

  const markCopied = () => {
    copied.value = true
    if (timer) clearTimeout(timer)
    timer = setTimeout(() => (copied.value = false), resetMs)
  }

  const copy = async (text: string, context = 'clipboard'): Promise<boolean> => {
    try {
      await navigator.clipboard.writeText(text)
      markCopied()
      return true
    } catch {
      // Fallback for older browsers / insecure contexts.
      try {
        const textarea = document.createElement('textarea')
        textarea.value = text
        document.body.appendChild(textarea)
        textarea.select()
        document.execCommand('copy')
        document.body.removeChild(textarea)
        markCopied()
        return true
      } catch (err) {
        logger.warn(`failed to copy to ${context}`, err)
        return false
      }
    }
  }

  return { copied, copy }
}
