import { useState } from 'react'
import { Button, Card, Group, Stack, Text, Title, UnstyledButton } from '@mantine/core'
import { useReminders } from '@/hooks/queries'
import { useDocumentDrawer } from '@/hooks/useDocumentDrawer'
import type { Reminder } from '@/types/documents'

const DAY = 24 * 60 * 60 * 1000
const VISIBLE = 5

function bucket(r: Reminder, now: number) {
  const delta = new Date(r.dueDate).getTime() - now
  if (Number.isNaN(delta) || delta > 30 * DAY) return 'Later'
  return delta <= 7 * DAY ? 'Next 7 days' : 'Next 30 days'
}

/** Deadlines the backend extracted from document dates and key facts. */
export function Upcoming() {
  const { data, isPending, isError, refetch } = useReminders(90)
  const drawer = useDocumentDrawer()
  const reminders = data?.reminders ?? []
  const [now] = useState(Date.now)

  return (
    <Card withBorder padding="md">
      <Title order={5} mb="xs">
        Upcoming
      </Title>
      {isPending ? (
        <Text size="sm" c="dimmed">
          Checking upcoming dates…
        </Text>
      ) : isError ? (
        <Group gap="xs">
          <Text size="sm" c="dimmed">
            Upcoming dates are unavailable.
          </Text>
          <Button size="compact-xs" variant="subtle" onClick={() => refetch()}>
            Retry
          </Button>
        </Group>
      ) : reminders.length === 0 ? (
        <Text size="sm" c="dimmed">
          Nothing due in the next 90 days.
        </Text>
      ) : (
        <Stack gap={6}>
          {reminders.slice(0, VISIBLE).map((r) => (
            <UnstyledButton key={r.id} onClick={() => drawer.open(r.id)}>
              <Group wrap="nowrap" gap="sm" align="flex-start">
                <DueDate date={r.dueDate} />
                <div style={{ minWidth: 0 }}>
                  <Text size="sm" truncate>
                    {r.title}
                  </Text>
                  <Text size="xs" c="dimmed" truncate>
                    {[bucket(r, now), r.company || r.docType, r.amountDue]
                      .filter(Boolean)
                      .join(' · ')}
                  </Text>
                </div>
              </Group>
            </UnstyledButton>
          ))}
          {reminders.length > VISIBLE && (
            <Text size="xs" c="dimmed">
              +{reminders.length - VISIBLE} more
            </Text>
          )}
        </Stack>
      )}
    </Card>
  )
}

function DueDate({ date }: { date: string }) {
  const d = new Date(date)
  const valid = !Number.isNaN(d.getTime())
  return (
    <Stack gap={0} align="center" w={36} style={{ flexShrink: 0 }}>
      <Text size="10px" tt="uppercase" c="dimmed" lh={1.2}>
        {valid ? d.toLocaleDateString(undefined, { month: 'short', timeZone: 'UTC' }) : 'Due'}
      </Text>
      <Text fw={600} lh={1.1}>
        {valid ? d.getUTCDate() : '—'}
      </Text>
    </Stack>
  )
}
