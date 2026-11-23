import {
  Alert,
  Button,
  CopyButton,
  Group,
  Stack,
  Text,
  Tooltip,
  UnstyledButton
} from '@mantine/core'
import { useMutation } from '@tanstack/react-query'
import { IconSparkles } from '@tabler/icons-react'
import { api, type KeyFact } from '@/api/client'
import { errorMessage } from '@/lib/utils'

interface Props {
  id: string
  initial: { summary?: string; keyFacts?: KeyFact[] }
}

/** Shows the cached AI summary if the backend has one; otherwise generates on demand. */
export function SummaryPanel({ id, initial }: Props) {
  const generate = useMutation({ mutationFn: () => api.summarizeDocument(id) })
  const summary = generate.data?.summary ?? initial.summary
  const facts = generate.data?.keyFacts ?? initial.keyFacts ?? []
  const hasSummary = !!summary || facts.length > 0

  return (
    <Stack gap="sm">
      {generate.isError && (
        <Alert color="red" variant="light" role="alert">
          {errorMessage(generate.error, 'Failed to generate summary')}
        </Alert>
      )}
      {summary && (
        <Text size="sm" style={{ whiteSpace: 'pre-wrap' }}>
          {summary}
        </Text>
      )}
      {facts.length > 0 && (
        <Stack gap={4}>
          {facts.map((fact, i) => (
            <CopyButton key={`${fact.label}-${i}`} value={fact.value}>
              {({ copied, copy }) => (
                <Tooltip label={copied ? 'Copied' : 'Click to copy'} position="left">
                  <UnstyledButton onClick={copy}>
                    <Group justify="space-between" wrap="nowrap" gap="md">
                      <Text size="sm" c="dimmed">
                        {fact.label}
                      </Text>
                      <Text size="sm" fw={500} ta="right">
                        {fact.value}
                      </Text>
                    </Group>
                  </UnstyledButton>
                </Tooltip>
              )}
            </CopyButton>
          ))}
        </Stack>
      )}
      {generate.isSuccess && !hasSummary && (
        <Text size="sm" c="dimmed">
          No summary available for this document.
        </Text>
      )}
      <Group>
        <Button
          size="xs"
          variant={hasSummary ? 'subtle' : 'light'}
          leftSection={<IconSparkles size={14} />}
          loading={generate.isPending}
          onClick={() => generate.mutate()}
        >
          {hasSummary ? 'Regenerate summary' : 'Summarize with AI'}
        </Button>
      </Group>
    </Stack>
  )
}
