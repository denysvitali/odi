import { useEffect, useMemo, useState, type ReactNode } from 'react'
import {
  ActionIcon,
  Alert,
  Button,
  Card,
  Center,
  Checkbox,
  Group,
  Loader,
  Menu,
  SimpleGrid,
  Skeleton,
  Stack,
  Text,
  Tooltip,
  UnstyledButton
} from '@mantine/core'
import { useIntersection } from '@mantine/hooks'
import { IconArrowsSort, IconCheck, IconLayoutGrid, IconList } from '@tabler/icons-react'
import { Link } from 'react-router'
import { useDocumentDrawer } from '@/hooks/useDocumentDrawer'
import { documentCompany, documentDate, documentTitle } from '@/lib/document'
import { formatDate } from '@/lib/format'
import { documentView, tags as tagStore, type DocumentView } from '@/stores/local'
import type { Document } from '@/types/documents'
import { DocTypeBadge } from './DocTypeBadge'
import { DocumentThumbnail } from './DocumentThumbnail'
import { Highlighted } from './Highlighted'
import { StarButton } from './StarButton'
import classes from './DocumentResults.module.css'

type Sort = 'relevance' | 'newest' | 'oldest' | 'title'
const SORTS: { value: Sort; label: string }[] = [
  { value: 'relevance', label: 'Default order' },
  { value: 'newest', label: 'Newest first' },
  { value: 'oldest', label: 'Oldest first' },
  { value: 'title', label: 'Title A–Z' }
]

export interface DocumentResultsProps {
  documents: Document[]
  loading?: boolean
  error?: string | null
  onRetry?: () => void
  hasMore?: boolean
  loadingMore?: boolean
  onLoadMore?: () => void
  /** Load the next page automatically when the end of the list scrolls into view. */
  infinite?: boolean
  empty?: { title: string; message?: string; action?: ReactNode }
  /** Hide sort/view controls (compact embeds). */
  bare?: boolean
  selection?: { selected: Set<string>; toggle: (id: string) => void }
  skeletons?: number
  /** Toolbar content: `title` on the left, `actions` before the sort/view controls. */
  title?: ReactNode
  actions?: ReactNode
}

export function DocumentResults({
  documents,
  loading,
  error,
  onRetry,
  hasMore,
  loadingMore,
  onLoadMore,
  infinite,
  empty = { title: 'No documents found' },
  bare,
  selection,
  skeletons = 6,
  title,
  actions
}: DocumentResultsProps) {
  const view = documentView.use()
  const [sort, setSort] = useState<Sort>('relevance')
  const sorted = useMemo(() => sortDocuments(documents, sort), [documents, sort])

  return (
    <Stack gap="sm">
      {(title || actions || !bare) && (
        <Group justify="space-between" align="flex-end" gap="xs">
          <div>{title}</div>
          <Group gap="xs">
            {actions}
            {!bare && (
              <>
                <Menu position="bottom-end">
                  <Menu.Target>
                    <Tooltip label="Sort loaded documents">
                      <ActionIcon
                        variant="default"
                        aria-label="Sort loaded documents"
                        disabled={documents.length < 2}
                      >
                        <IconArrowsSort size={16} />
                      </ActionIcon>
                    </Tooltip>
                  </Menu.Target>
                  <Menu.Dropdown>
                    {SORTS.map((s) => (
                      <Menu.Item
                        key={s.value}
                        onClick={() => setSort(s.value)}
                        rightSection={sort === s.value ? <IconCheck size={14} /> : null}
                      >
                        {s.label}
                      </Menu.Item>
                    ))}
                  </Menu.Dropdown>
                </Menu>
                <ActionIcon.Group>
                  <ViewButton view="list" current={view} icon={IconList} label="List view" />
                  <ViewButton view="grid" current={view} icon={IconLayoutGrid} label="Grid view" />
                </ActionIcon.Group>
              </>
            )}
          </Group>
        </Group>
      )}

      {error && (
        <Alert color="red" variant="light" role="alert" title={error}>
          {onRetry && (
            <Button size="xs" variant="default" onClick={onRetry}>
              Try again
            </Button>
          )}
        </Alert>
      )}

      {loading ? (
        <Stack gap="xs" aria-busy="true" aria-label="Loading documents">
          {Array.from({ length: skeletons }, (_, i) => (
            <Skeleton key={i} h={56} />
          ))}
        </Stack>
      ) : documents.length === 0 ? (
        !error && <EmptyState {...empty} />
      ) : view === 'grid' && !bare ? (
        <SimpleGrid cols={{ base: 2, sm: 3, lg: 4 }} spacing="sm">
          {sorted.map((doc) => (
            <GridItem key={doc._id} doc={doc} selection={selection} />
          ))}
        </SimpleGrid>
      ) : (
        <Card withBorder padding={0}>
          {sorted.map((doc) => (
            <ListItem key={doc._id} doc={doc} selection={selection} />
          ))}
        </Card>
      )}

      {hasMore && !loading && (
        <LoadMore infinite={infinite} loading={!!loadingMore} onLoadMore={onLoadMore} />
      )}
    </Stack>
  )
}

function ViewButton(props: {
  view: DocumentView
  current: DocumentView
  icon: typeof IconList
  label: string
}) {
  const { view, current, icon: Icon, label } = props
  return (
    <ActionIcon
      variant={view === current ? 'light' : 'default'}
      aria-label={label}
      aria-pressed={view === current}
      onClick={() => documentView.set(view)}
    >
      <Icon size={16} />
    </ActionIcon>
  )
}

function LoadMore({
  infinite,
  loading,
  onLoadMore
}: {
  infinite?: boolean
  loading: boolean
  onLoadMore?: () => void
}) {
  const { ref, entry } = useIntersection({ rootMargin: '200px' })
  const visible = !!entry?.isIntersecting
  useEffect(() => {
    if (infinite && visible && !loading) onLoadMore?.()
  }, [infinite, visible, loading, onLoadMore])
  return (
    <Center ref={ref} py="sm">
      {loading ? (
        <Loader size="sm" />
      ) : (
        <Button variant="subtle" onClick={onLoadMore}>
          Load more documents
        </Button>
      )}
    </Center>
  )
}

function EmptyState({ title, message, action }: NonNullable<DocumentResultsProps['empty']>) {
  return (
    <Stack align="center" gap={6} py="xl" ta="center">
      <Text fw={500}>{title}</Text>
      {message && (
        <Text size="sm" c="dimmed" maw={380}>
          {message}
        </Text>
      )}
      {action}
    </Stack>
  )
}

export const emptyActions = {
  upload: (
    <Button component={Link} to="/upload" size="xs" mt="xs">
      Upload documents
    </Button>
  ),
  browse: (
    <Button component={Link} to="/documents" size="xs" variant="default" mt="xs">
      Browse all documents
    </Button>
  )
}

interface ItemProps {
  doc: Document
  selection?: DocumentResultsProps['selection']
}

function useItem({ doc, selection }: ItemProps) {
  const drawer = useDocumentDrawer()
  const localTags = tagStore.use()[doc._id] ?? []
  const selected = !!selection?.selected.has(doc._id)
  const onClick = () => (selection ? selection.toggle(doc._id) : drawer.open(doc._id))
  const meta = [documentCompany(doc), formatDate(documentDate(doc))].filter(Boolean).join(' · ')
  return { selected, onClick, meta, title: documentTitle(doc), tags: localTags }
}

function ListItem(props: ItemProps) {
  const { doc, selection } = props
  const { selected, onClick, meta, title, tags } = useItem(props)
  const snippet = doc.highlight?.text?.[0]
  return (
    <Group
      className={classes.row}
      data-selected={selected || undefined}
      wrap="nowrap"
      gap="sm"
      px="sm"
      py={8}
    >
      {selection && (
        <Checkbox
          checked={selected}
          onChange={() => selection.toggle(doc._id)}
          aria-label={`Select ${title}`}
        />
      )}
      <UnstyledButton onClick={onClick} className={classes.main}>
        <Group wrap="nowrap" gap="sm">
          <DocumentThumbnail id={doc._id} w={36} h={48} />
          <div style={{ minWidth: 0, flex: 1 }}>
            <Text size="sm" fw={500} truncate className="document-title">
              {title}
            </Text>
            {meta && (
              <Text size="xs" c="dimmed" truncate>
                {meta}
              </Text>
            )}
            {snippet && (
              <Text size="xs" c="dimmed" lineClamp={2} mt={2}>
                <Highlighted text={snippet} />
              </Text>
            )}
          </div>
        </Group>
      </UnstyledButton>
      <Group gap={6} wrap="nowrap" visibleFrom="xs">
        {tags.slice(0, 2).map((t) => (
          <Text key={t} size="xs" c="dimmed">
            #{t}
          </Text>
        ))}
        <DocTypeBadge type={doc._source.docType} />
      </Group>
      <StarButton id={doc._id} />
    </Group>
  )
}

function GridItem(props: ItemProps) {
  const { doc } = props
  const { selected, onClick, meta, title } = useItem(props)
  return (
    <Card withBorder padding="xs" className={classes.card} data-selected={selected || undefined}>
      <Card.Section>
        <UnstyledButton onClick={onClick} w="100%" aria-label={title}>
          <DocumentThumbnail id={doc._id} w="100%" h={180} radius="0" />
        </UnstyledButton>
      </Card.Section>
      <Group wrap="nowrap" gap={4} mt="xs" align="flex-start">
        <div style={{ minWidth: 0, flex: 1 }}>
          <Text size="sm" fw={500} lineClamp={2} className="document-title">
            {title}
          </Text>
          <Text size="xs" c="dimmed" truncate>
            {meta}
          </Text>
        </div>
        <StarButton id={doc._id} size="sm" />
      </Group>
    </Card>
  )
}

function sortDocuments(docs: Document[], sort: Sort) {
  if (sort === 'relevance') return docs
  if (sort === 'title')
    return [...docs].sort((a, b) => documentTitle(a).localeCompare(documentTitle(b)))
  const time = (d: Document) => {
    const t = new Date(documentDate(d) || '').getTime()
    return Number.isNaN(t) ? null : t
  }
  return [...docs].sort((a, b) => {
    const l = time(a)
    const r = time(b)
    if (l === null) return r === null ? 0 : 1
    if (r === null) return -1
    return sort === 'oldest' ? l - r : r - l
  })
}
