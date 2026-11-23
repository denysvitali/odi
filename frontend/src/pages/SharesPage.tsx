import {
  ActionIcon,
  Alert,
  Anchor,
  Badge,
  Card,
  CopyButton,
  Group,
  Table,
  Text,
  Tooltip
} from '@mantine/core'
import { IconCheck, IconCopy, IconLock, IconRefresh, IconTrash } from '@tabler/icons-react'
import { api } from '@/api/client'
import { PageHeader } from '@/components/layout/PageHeader'
import { useRevokeShare, useShares } from '@/hooks/queries'
import { useDocumentDrawer } from '@/hooks/useDocumentDrawer'
import { formatDateTime } from '@/lib/format'
import { errorMessage } from '@/lib/utils'

export default function SharesPage() {
  const shares = useShares()
  const revoke = useRevokeShare()
  const drawer = useDocumentDrawer()
  const list = shares.data ?? []
  const error = shares.error ?? revoke.error

  return (
    <div style={{ maxWidth: 1000, margin: '0 auto' }}>
      <PageHeader
        title="Shares"
        description="Active share links. Create one from a document’s share button."
        actions={
          <Tooltip label="Refresh">
            <ActionIcon
              variant="default"
              aria-label="Refresh"
              loading={shares.isFetching}
              onClick={() => shares.refetch()}
            >
              <IconRefresh size={16} />
            </ActionIcon>
          </Tooltip>
        }
      />
      {error && (
        <Alert color="red" variant="light" mb="md" role="alert">
          {errorMessage(error, 'Share action failed')}
        </Alert>
      )}
      <Card withBorder padding={0}>
        {shares.isPending ? (
          <Text p="md" c="dimmed" size="sm">
            Loading shares…
          </Text>
        ) : list.length === 0 ? (
          <Text p="xl" c="dimmed" size="sm" ta="center">
            No active share links.
          </Text>
        ) : (
          <Table.ScrollContainer minWidth={560}>
            <Table verticalSpacing="sm" highlightOnHover>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>Document</Table.Th>
                  <Table.Th>Expires</Table.Th>
                  <Table.Th>Views</Table.Th>
                  <Table.Th />
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {list.map((s) => {
                  const id = `${s.scanID}_${s.sequenceID}`
                  return (
                    <Table.Tr key={s.token}>
                      <Table.Td>
                        <Group gap="xs" wrap="nowrap">
                          <Anchor
                            component="button"
                            ff="monospace"
                            size="sm"
                            onClick={() => drawer.open(id)}
                          >
                            {id}
                          </Anchor>
                          {s.hasPassphrase && (
                            <Badge
                              size="xs"
                              variant="light"
                              color="gray"
                              leftSection={<IconLock size={10} />}
                            >
                              Passphrase
                            </Badge>
                          )}
                        </Group>
                      </Table.Td>
                      <Table.Td fz="sm">{formatDateTime(new Date(s.expiresAt * 1000))}</Table.Td>
                      <Table.Td fz="sm">
                        {s.viewCount}
                        {s.maxViews > 0 ? ` / ${s.maxViews}` : ''}
                      </Table.Td>
                      <Table.Td>
                        <Group gap={4} justify="flex-end" wrap="nowrap">
                          <CopyButton value={api.shareUrl(s.token)}>
                            {({ copied, copy }) => (
                              <Tooltip label={copied ? 'Copied' : 'Copy link'}>
                                <ActionIcon
                                  variant="subtle"
                                  color={copied ? 'teal' : 'gray'}
                                  aria-label="Copy link"
                                  onClick={copy}
                                >
                                  {copied ? <IconCheck size={16} /> : <IconCopy size={16} />}
                                </ActionIcon>
                              </Tooltip>
                            )}
                          </CopyButton>
                          <Tooltip label="Revoke">
                            <ActionIcon
                              variant="subtle"
                              color="red"
                              aria-label="Revoke"
                              loading={revoke.isPending && revoke.variables === s.token}
                              onClick={() => revoke.mutate(s.token)}
                            >
                              <IconTrash size={16} />
                            </ActionIcon>
                          </Tooltip>
                        </Group>
                      </Table.Td>
                    </Table.Tr>
                  )
                })}
              </Table.Tbody>
            </Table>
          </Table.ScrollContainer>
        )}
      </Card>
    </div>
  )
}
