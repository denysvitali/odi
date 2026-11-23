import { useEffect, useState } from 'react'

/** Create (and revoke) an object URL for a blob for as long as the component uses it. */
export function useObjectUrl(blob: Blob | undefined) {
  const [url, setUrl] = useState<string>()
  useEffect(() => {
    // Object URLs are an external resource: created and revoked with the effect lifecycle.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (!blob) return setUrl(undefined)
    const next = URL.createObjectURL(blob)
    setUrl(next)
    return () => URL.revokeObjectURL(next)
  }, [blob])
  return url
}
