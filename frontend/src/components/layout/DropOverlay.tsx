import { useEffect, useState } from 'react'
import { Center, Overlay, Stack, Text } from '@mantine/core'
import { IconUpload } from '@tabler/icons-react'
import { useLocation, useNavigate } from 'react-router'
import { isUploadable, stashFiles } from '@/lib/upload'

/** Drop files anywhere in the app to start an upload. The upload page handles its own drops. */
export function DropOverlay() {
  const [dragging, setDragging] = useState(false)
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const active = pathname !== '/upload'

  useEffect(() => {
    if (!active) return
    let depth = 0
    const hasFiles = (e: DragEvent) => !!e.dataTransfer?.types.includes('Files')
    const enter = (e: DragEvent) => hasFiles(e) && (depth++, setDragging(true))
    const over = (e: DragEvent) => hasFiles(e) && e.preventDefault()
    const leave = () => (depth = Math.max(0, depth - 1)) === 0 && setDragging(false)
    const drop = (e: DragEvent) => {
      if (!hasFiles(e)) return
      e.preventDefault()
      depth = 0
      setDragging(false)
      const files = Array.from(e.dataTransfer?.files ?? []).filter(isUploadable)
      if (!files.length) return
      stashFiles(files)
      navigate('/upload')
    }
    window.addEventListener('dragenter', enter)
    window.addEventListener('dragover', over)
    window.addEventListener('dragleave', leave)
    window.addEventListener('drop', drop)
    return () => {
      window.removeEventListener('dragenter', enter)
      window.removeEventListener('dragover', over)
      window.removeEventListener('dragleave', leave)
      window.removeEventListener('drop', drop)
    }
  }, [active, navigate])

  if (!dragging || !active) return null
  return (
    <Overlay fixed blur={2} zIndex={500} aria-hidden>
      <Center h="100%">
        <Stack align="center" gap={4} c="white">
          <IconUpload size={40} />
          <Text fw={600} size="lg">
            Drop to upload
          </Text>
        </Stack>
      </Center>
    </Overlay>
  )
}
