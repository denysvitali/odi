import type { ReactNode } from 'react'
import { Group, Text, Title } from '@mantine/core'
import { useDocumentTitle } from '@mantine/hooks'

export function PageHeader({
  title,
  description,
  actions
}: {
  title: string
  description?: ReactNode
  actions?: ReactNode
}) {
  useDocumentTitle(`${title} · ODI`)
  return (
    <Group justify="space-between" align="flex-end" mb="md" gap="sm">
      <div>
        <Title order={2}>{title}</Title>
        {description && (
          <Text c="dimmed" size="sm">
            {description}
          </Text>
        )}
      </div>
      {actions && <Group gap="xs">{actions}</Group>}
    </Group>
  )
}
