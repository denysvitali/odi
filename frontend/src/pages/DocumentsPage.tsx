import { useMemo, useState } from 'react'
import { ActionIcon, Affix, Button, Card, Group, Select, Stack, Text, Tooltip } from '@mantine/core'
import { useQueryClient } from '@tanstack/react-query'
import { IconDownload, IconListCheck, IconRefresh } from '@tabler/icons-react'
import { DocumentResults, emptyActions } from '@/components/documents/DocumentResults'
import { PageHeader } from '@/components/layout/PageHeader'
import { DateRangeFields } from '@/components/search/DateRangeFields'
import { useDocumentList, type DateRange } from '@/hooks/queries'
import { documentsToCsv, downloadBlob } from '@/lib/document'
import { formatNumber } from '@/lib/format'
import { allTags, tags as tagStore } from '@/stores/local'
import type { Document } from '@/types/documents'

export default function DocumentsPage() {
  const [range, setRange] = useState<DateRange>({})
  const [tag, setTag] = useState<string | null>(null)
  const [selected, setSelected] = useState<Set<string> | null>(null)
  const list = useDocumentList(24, range)
  const client = useQueryClient()
  const tagMap = tagStore.use()
  const tagOptions = useMemo(() => allTags(tagMap), [tagMap])

  const visible = tag ? list.documents.filter((d) => tagMap[d._id]?.includes(tag)) : list.documents
  const toggle = (id: string) =>
    setSelected((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  const exportCsv = (docs: Document[]) =>
    downloadBlob(
      new Blob([documentsToCsv(docs)], { type: 'text/csv;charset=utf-8' }),
      `odi-documents-${new Date().toISOString().slice(0, 10)}.csv`
    )

  return (
    <Stack gap={0} maw={1200} mx="auto">
      <PageHeader
        title="Documents"
        description={list.isPending ? 'Loading…' : `${formatNumber(list.total)} indexed`}
        actions={
          <>
            <Button
              size="xs"
              variant={selected ? 'light' : 'default'}
              leftSection={<IconListCheck size={14} />}
              onClick={() => setSelected(selected ? null : new Set())}
            >
              {selected ? 'Done' : 'Select'}
            </Button>
            <Button
              size="xs"
              variant="default"
              leftSection={<IconDownload size={14} />}
              onClick={() => exportCsv(visible)}
            >
              Export CSV
            </Button>
            <Tooltip label="Refresh">
              <ActionIcon
                variant="default"
                size="md"
                aria-label="Refresh"
                loading={list.isFetching && !list.isFetchingNextPage}
                onClick={() => client.resetQueries({ queryKey: ['documents'] })}
              >
                <IconRefresh size={16} />
              </ActionIcon>
            </Tooltip>
          </>
        }
      />

      <DocumentResults
        title={
          <Group align="flex-end" gap="sm">
            <DateRangeFields from={range.from} to={range.to} onChange={setRange} />
            {tagOptions.length > 0 && (
              <Select
                size="xs"
                label="Tag"
                placeholder="Any"
                clearable
                data={tagOptions}
                value={tag}
                onChange={setTag}
                w={160}
              />
            )}
            {(range.from || range.to || tag) && (
              <Button
                size="xs"
                variant="subtle"
                onClick={() => {
                  setRange({})
                  setTag(null)
                }}
              >
                Clear filters
              </Button>
            )}
          </Group>
        }
        documents={visible}
        loading={list.isPending}
        error={list.isError ? 'Documents could not be loaded.' : null}
        onRetry={() => (list.documents.length ? list.fetchNextPage() : list.refetch())}
        hasMore={list.hasNextPage}
        loadingMore={list.isFetchingNextPage}
        onLoadMore={() => list.fetchNextPage()}
        infinite
        selection={selected ? { selected, toggle } : undefined}
        empty={
          range.from || range.to || tag
            ? { title: 'No documents match these filters' }
            : {
                title: 'No documents yet',
                message: 'Upload scans or PDFs to get started.',
                action: emptyActions.upload
              }
        }
      />

      {selected && (
        <Affix position={{ bottom: 20, left: '50%' }} style={{ transform: 'translateX(-50%)' }}>
          <Card shadow="lg" withBorder padding="xs">
            <Group gap="xs" wrap="nowrap">
              <Text size="sm" fw={500} px="xs">
                {selected.size} selected
              </Text>
              <Button
                size="xs"
                variant="subtle"
                onClick={() => setSelected(new Set(visible.map((d) => d._id)))}
              >
                Select all loaded
              </Button>
              <Button
                size="xs"
                variant="subtle"
                color="gray"
                onClick={() => setSelected(new Set())}
              >
                Clear
              </Button>
              <Button
                size="xs"
                disabled={!selected.size}
                leftSection={<IconDownload size={14} />}
                onClick={() => exportCsv(visible.filter((d) => selected.has(d._id)))}
              >
                Export selected
              </Button>
            </Group>
          </Card>
        </Affix>
      )}
    </Stack>
  )
}
