import { useState } from 'react'
import { Alert, Button, Card, Group, Kbd, Stack, Text, Textarea, Title } from '@mantine/core'
import { useMutation } from '@tanstack/react-query'
import { IconSend } from '@tabler/icons-react'
import { api } from '@/api/client'
import { DocumentResults } from '@/components/documents/DocumentResults'
import { PageHeader } from '@/components/layout/PageHeader'
import { useDocumentsById } from '@/hooks/queries'
import { errorMessage } from '@/lib/utils'

export default function ChatPage() {
  const [question, setQuestion] = useState('')
  const chat = useMutation({ mutationFn: (q: string) => api.chat({ question: q }) })
  const sources = useDocumentsById(chat.data?.citations ?? [])
  const ask = () => question.trim() && !chat.isPending && chat.mutate(question.trim())

  return (
    <Stack maw={860} mx="auto" gap="md">
      <PageHeader
        title="Ask your archive"
        description="Answers are grounded in your indexed documents."
      />
      <form
        onSubmit={(e) => {
          e.preventDefault()
          ask()
        }}
      >
        <Textarea
          aria-label="Question"
          placeholder="e.g. What was the total on my last insurance invoice?"
          autosize
          minRows={2}
          maxRows={8}
          value={question}
          onChange={(e) => setQuestion(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
              e.preventDefault()
              ask()
            }
          }}
        />
        <Group justify="space-between" mt="xs">
          <Text size="xs" c="dimmed">
            <Kbd size="xs">Ctrl/⌘</Kbd> + <Kbd size="xs">Enter</Kbd> to send
          </Text>
          <Button
            type="submit"
            leftSection={<IconSend size={16} />}
            loading={chat.isPending}
            disabled={!question.trim()}
          >
            Ask
          </Button>
        </Group>
      </form>

      {chat.isError && (
        <Alert color="red" variant="light" title="Unable to answer" role="alert">
          {errorMessage(chat.error, 'Unable to get an answer')}
        </Alert>
      )}
      {chat.data && (
        <Card withBorder>
          <Text size="sm" style={{ whiteSpace: 'pre-wrap' }} lh={1.6}>
            {chat.data.answer}
          </Text>
        </Card>
      )}
      {(chat.data?.citations.length ?? 0) > 0 && (
        <div>
          <Title order={5} mb="xs">
            Sources
          </Title>
          <DocumentResults
            bare
            documents={sources.documents}
            loading={sources.pending > 0 && !sources.documents.length}
            skeletons={2}
          />
        </div>
      )}
    </Stack>
  )
}
