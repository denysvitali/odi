import { Badge } from '@mantine/core'

const COLORS = ['blue', 'green', 'orange', 'grape', 'pink', 'cyan', 'yellow', 'indigo']

export function docTypeColor(type: string) {
  let hash = 0
  for (const ch of type) hash = (hash * 31 + ch.charCodeAt(0)) >>> 0
  return COLORS[hash % COLORS.length]
}

export function DocTypeBadge({ type }: { type?: string }) {
  if (!type?.trim()) return null
  return (
    <Badge size="sm" variant="light" color={docTypeColor(type)} tt="capitalize">
      {type}
    </Badge>
  )
}
