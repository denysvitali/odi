import { ActionIcon, type ActionIconProps } from '@mantine/core'
import { IconStar, IconStarFilled } from '@tabler/icons-react'
import { toggleFavorite, useIsFavorite } from '@/stores/local'

export function StarButton({ id, ...props }: { id: string } & ActionIconProps) {
  const starred = useIsFavorite(id)
  return (
    <ActionIcon
      variant="subtle"
      color={starred ? 'yellow' : 'gray'}
      aria-label={starred ? 'Unstar document' : 'Star document'}
      aria-pressed={starred}
      onClick={(e) => {
        e.stopPropagation()
        toggleFavorite(id)
      }}
      {...props}
    >
      {starred ? <IconStarFilled size={16} /> : <IconStar size={16} />}
    </ActionIcon>
  )
}
