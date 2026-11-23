import { useState } from 'react'
import {
  Alert,
  Badge,
  Button,
  Card,
  Group,
  PasswordInput,
  Progress,
  SimpleGrid,
  Stack,
  Text,
  Title
} from '@mantine/core'
import { notifications } from '@mantine/notifications'
import { useQueryClient } from '@tanstack/react-query'
import { IconRefresh } from '@tabler/icons-react'
import { ApiError, type ReindexStatus } from '@/api/client'
import { PageHeader } from '@/components/layout/PageHeader'
import { useReindex } from '@/hooks/queries'
import { STORAGE_KEYS } from '@/lib/constants'
import { formatDateTime } from '@/lib/format'
import { errorMessage } from '@/lib/utils'

export default function AdminPage() {
  return (
    <Stack maw={860} mx="auto" gap="lg">
      <PageHeader title="Admin" description="Connection settings and index maintenance." />
      <ApiTokenCard />
      <ReindexCard />
    </Stack>
  )
}

function ApiTokenCard() {
  const [token, setToken] = useState(() => localStorage.getItem(STORAGE_KEYS.API_TOKEN) ?? '')
  const client = useQueryClient()
  const save = () => {
    try {
      if (token.trim()) localStorage.setItem(STORAGE_KEYS.API_TOKEN, token.trim())
      else localStorage.removeItem(STORAGE_KEYS.API_TOKEN)
    } catch {
      notifications.show({ color: 'red', message: 'Browser storage is unavailable.' })
      return
    }
    client.resetQueries()
    notifications.show({ message: token.trim() ? 'API token saved.' : 'API token removed.' })
  }
  return (
    <Card withBorder>
      <form
        onSubmit={(e) => {
          e.preventDefault()
          save()
        }}
      >
        <Group align="flex-end" gap="sm">
          <PasswordInput
            style={{ flex: 1 }}
            label="API token"
            description="Sent as a bearer token with every request. Stored in this browser."
            value={token}
            onChange={(e) => setToken(e.currentTarget.value)}
            autoComplete="off"
          />
          <Button type="submit" variant="default">
            Save
          </Button>
        </Group>
      </form>
    </Card>
  )
}

const STATE_COLOR: Record<ReindexStatus['state'], string> = {
  idle: 'gray',
  running: 'blue',
  completed: 'teal',
  failed: 'red'
}

function ReindexCard() {
  const { status, start } = useReindex()
  const s = status.data
  const done = s ? s.processed + s.duplicates + s.failed : 0
  const percent = s?.total ? Math.min(100, Math.round((done / s.total) * 100)) : 0
  const startError =
    start.error instanceof ApiError && start.error.status === 409 ? null : start.error
  const error = status.error ?? startError

  return (
    <Card withBorder>
      <Group justify="space-between" mb="md">
        <Group gap="xs">
          <Title order={4}>Reindex from storage</Title>
          {s && (
            <Badge variant="light" color={STATE_COLOR[s.state]}>
              {s.state}
            </Badge>
          )}
        </Group>
        <Group gap="xs">
          <Button
            size="xs"
            variant="subtle"
            color="gray"
            leftSection={<IconRefresh size={14} />}
            onClick={() => status.refetch()}
            loading={status.isFetching && !start.isPending}
          >
            Refresh
          </Button>
          <Button
            size="xs"
            onClick={() => start.mutate()}
            loading={start.isPending}
            disabled={s?.state === 'running'}
          >
            Start reindex
          </Button>
        </Group>
      </Group>
      {error && (
        <Alert color="red" variant="light" mb="md" role="alert">
          {errorMessage(error, 'Admin action failed')}
        </Alert>
      )}
      <Stack gap="md">
        <div>
          <Group justify="space-between" mb={4}>
            <Text size="sm" c="dimmed" truncate>
              {s?.currentPage ?? 'Progress'}
            </Text>
            <Text size="sm">
              {done} / {s?.total ?? 0}
            </Text>
          </Group>
          <Progress value={percent} animated={s?.state === 'running'} />
        </div>
        <SimpleGrid cols={{ base: 2, sm: 4 }}>
          <Stat label="Processed" value={s?.processed} />
          <Stat label="Duplicates" value={s?.duplicates} />
          <Stat label="Failed" value={s?.failed} />
          <Stat label="Total" value={s?.total} />
        </SimpleGrid>
        <Text size="xs" c="dimmed">
          Started {s?.startedAt ? formatDateTime(s.startedAt) : '—'} · Finished{' '}
          {s?.finishedAt ? formatDateTime(s.finishedAt) : '—'}
        </Text>
        {s?.error && (
          <Alert color="red" variant="light">
            {s.error}
          </Alert>
        )}
        {s?.recentErrors?.length ? (
          <Stack gap={4}>
            <Text size="sm" fw={500}>
              Recent errors
            </Text>
            {s.recentErrors.map((e) => (
              <Text
                key={`${e.page}-${e.error}`}
                size="xs"
                ff="monospace"
                style={{ wordBreak: 'break-word' }}
              >
                {e.page}:{' '}
                <Text span c="dimmed" inherit>
                  {e.error}
                </Text>
              </Text>
            ))}
          </Stack>
        ) : null}
      </Stack>
    </Card>
  )
}

function Stat({ label, value = 0 }: { label: string; value?: number }) {
  return (
    <div>
      <Text size="xs" c="dimmed">
        {label}
      </Text>
      <Text fw={600} size="xl">
        {value}
      </Text>
    </div>
  )
}
