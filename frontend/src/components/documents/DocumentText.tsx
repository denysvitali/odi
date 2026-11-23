import { useState } from 'react'
import { Button, CopyButton, Group, ScrollArea, Text, TextInput } from '@mantine/core'
import { IconSearch } from '@tabler/icons-react'
import { countMatches, FindHighlight } from './Highlighted'

export function DocumentText({ text }: { text: string }) {
  const [find, setFind] = useState('')
  if (!text)
    return (
      <Text size="sm" c="dimmed">
        No text content available.
      </Text>
    )
  const matches = countMatches(text, find)
  return (
    <>
      <Group gap="xs" mb="xs" wrap="nowrap">
        <TextInput
          size="xs"
          style={{ flex: 1 }}
          type="search"
          placeholder="Find in text…"
          aria-label="Find in text"
          leftSection={<IconSearch size={14} />}
          rightSection={
            find.trim() ? (
              <Text size="xs" c="dimmed">
                {matches}
              </Text>
            ) : undefined
          }
          value={find}
          onChange={(e) => setFind(e.currentTarget.value)}
        />
        <CopyButton value={text}>
          {({ copied, copy }) => (
            <Button size="xs" variant="default" onClick={copy}>
              {copied ? 'Copied' : 'Copy'}
            </Button>
          )}
        </CopyButton>
      </Group>
      <ScrollArea.Autosize mah={420} type="auto">
        <Text size="sm" style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-word' }}>
          <FindHighlight text={text} needle={find} />
        </Text>
      </ScrollArea.Autosize>
    </>
  )
}
