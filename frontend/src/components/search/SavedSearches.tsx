import { useEffect, useRef, useState } from 'react'
import {
  ActionIcon,
  Card,
  Group,
  Stack,
  Text,
  TextInput,
  Title,
  UnstyledButton
} from '@mantine/core'
import { IconCheck, IconPencil, IconSearch, IconX } from '@tabler/icons-react'
import { removeSearch, renameSearch, savedSearches, type SavedSearch } from '@/stores/savedSearches'

export function SavedSearches({ onSelect }: { onSelect: (s: SavedSearch) => void }) {
  const searches = savedSearches.use()
  const [editing, setEditing] = useState<string | null>(null)
  const [name, setName] = useState('')
  const [error, setError] = useState('')
  // After renaming, return focus to the row's rename button (keyboard users stay in place).
  const [refocus, setRefocus] = useState<string | null>(null)
  const list = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!refocus) return
    list.current?.querySelector<HTMLElement>(`[data-rename-id="${CSS.escape(refocus)}"]`)?.focus()
  }, [refocus])
  const stopEditing = (id: string) => {
    setEditing(null)
    setRefocus(id)
  }

  return (
    <Card withBorder padding="md">
      <Title order={5} mb="xs">
        Saved searches
      </Title>
      {searches.length === 0 && (
        <Text size="sm" c="dimmed">
          Run a search, then save it to find it here.
        </Text>
      )}
      <Stack gap={2} ref={list}>
        {searches.map((s) =>
          editing === s.id ? (
            <form
              key={s.id}
              onSubmit={(e) => {
                e.preventDefault()
                const result = renameSearch(s.id, name)
                if (result.ok) stopEditing(s.id)
                else setError(result.error)
              }}
            >
              <TextInput
                size="xs"
                label="Search name"
                maxLength={80}
                autoFocus
                value={name}
                error={error || undefined}
                onChange={(e) => setName(e.currentTarget.value)}
                onKeyDown={(e) => e.key === 'Escape' && stopEditing(s.id)}
                rightSection={
                  <ActionIcon type="submit" size="sm" variant="subtle" aria-label="Save name">
                    <IconCheck size={14} />
                  </ActionIcon>
                }
              />
            </form>
          ) : (
            <Group key={s.id} gap={4} wrap="nowrap">
              <UnstyledButton onClick={() => onSelect(s)} style={{ flex: 1, minWidth: 0 }} py={4}>
                <Group gap="xs" wrap="nowrap">
                  <IconSearch size={14} color="var(--mantine-color-dimmed)" />
                  <Text size="sm" truncate>
                    {s.name}
                  </Text>
                </Group>
              </UnstyledButton>
              <ActionIcon
                size="sm"
                variant="subtle"
                color="gray"
                aria-label={`Rename saved search ${s.name}`}
                data-rename-id={s.id}
                onClick={() => {
                  setRefocus(null)
                  setEditing(s.id)
                  setName(s.name)
                  setError('')
                }}
              >
                <IconPencil size={14} />
              </ActionIcon>
              <ActionIcon
                size="sm"
                variant="subtle"
                color="gray"
                aria-label={`Remove saved search ${s.name}`}
                onClick={() => removeSearch(s.id)}
              >
                <IconX size={14} />
              </ActionIcon>
            </Group>
          )
        )}
      </Stack>
    </Card>
  )
}
