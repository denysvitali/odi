import { Center, Image, type MantineStyleProp } from '@mantine/core'
import { IconFileText } from '@tabler/icons-react'
import { useThumbnail } from '@/hooks/queries'
import { useObjectUrl } from '@/hooks/useObjectUrl'

interface Props {
  id: string
  w?: number | string
  h?: number | string
  alt?: string
  radius?: string
  style?: MantineStyleProp
}

/** Thumbnails need the bearer token, so they are fetched as blobs rather than via <img src>. */
export function DocumentThumbnail({ id, w, h, alt = '', radius = 'sm', style }: Props) {
  const { data } = useThumbnail(id)
  const url = useObjectUrl(data)
  if (!url)
    return (
      <Center
        w={w}
        h={h}
        bg="var(--mantine-color-default-hover)"
        style={[{ borderRadius: `var(--mantine-radius-${radius})`, flexShrink: 0 }, style]}
      >
        <IconFileText size={18} color="var(--mantine-color-dimmed)" />
      </Center>
    )
  return (
    <Image
      src={url}
      alt={alt}
      w={w}
      h={h}
      radius={radius}
      fit="cover"
      style={[{ flexShrink: 0 }, style]}
    />
  )
}
