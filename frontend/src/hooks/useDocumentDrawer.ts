import { useCallback } from 'react'
import { useSearchParams } from 'react-router'

/**
 * The open document lives in the URL (`?doc=<id>`) so any page can open it,
 * links are shareable, and Back closes the drawer.
 */
export function useDocumentDrawer() {
  const [params, setParams] = useSearchParams()
  const openId = params.get('doc')
  const open = useCallback(
    (id: string) =>
      setParams((prev) => {
        const next = new URLSearchParams(prev)
        next.set('doc', id)
        return next
      }),
    [setParams]
  )
  const close = useCallback(
    () =>
      setParams(
        (prev) => {
          const next = new URLSearchParams(prev)
          next.delete('doc')
          return next
        },
        { replace: true }
      ),
    [setParams]
  )
  return { openId, open, close }
}
