import { useState } from 'react'
import {
  ActionIcon,
  Anchor,
  Badge,
  Box,
  Button,
  Card,
  CloseButton,
  Drawer,
  Grid,
  Group,
  Indicator,
  Popover,
  Stack,
  Text,
  TextInput,
  Title,
  Tooltip
} from '@mantine/core'
import { useDisclosure, useDocumentTitle, useMediaQuery } from '@mantine/hooks'
import {
  IconAdjustments,
  IconArrowRight,
  IconBookmark,
  IconHistory,
  IconSearch,
  IconX
} from '@tabler/icons-react'
import { Link, useSearchParams } from 'react-router'
import type { SearchFilters as Filters } from '@/api/client'
import { DocumentResults, emptyActions } from '@/components/documents/DocumentResults'
import { SearchFilters } from '@/components/search/SearchFilters'
import { SavedSearches } from '@/components/search/SavedSearches'
import { Upcoming } from '@/components/search/Upcoming'
import { useDocumentList, useFacets, useSearch } from '@/hooks/queries'
import {
  archiveQuery,
  countFilters,
  hasArchiveSearch,
  parseArchiveQuery
} from '@/lib/archiveSearch'
import { formatNumber } from '@/lib/format'
import { addRecentSearch, recentSearches } from '@/stores/local'
import { saveSearch } from '@/stores/savedSearches'

const CATEGORIES = [
  { label: 'Invoices', type: 'invoice' },
  { label: 'Contracts', type: 'contract' },
  { label: 'Banking', type: 'bank' },
  { label: 'Medical', type: 'medical' }
]

export default function SearchPage() {
  const [params, setParams] = useSearchParams()
  const { term, filters } = parseArchiveQuery(params)
  const searching = hasArchiveSearch(term, filters)
  useDocumentTitle(term ? `${term} · ODI` : 'ODI')

  const go = (nextTerm: string, nextFilters: Filters) => {
    if (nextTerm.trim()) addRecentSearch(nextTerm)
    setParams(archiveQuery(nextTerm, nextFilters))
  }

  return (
    <Stack gap="lg" maw={1200} mx="auto">
      <SearchBar term={term} onSubmit={(t) => go(t, filters)} />
      {searching ? (
        <Results term={term} filters={filters} onChange={go} />
      ) : (
        <Overview onSearch={go} />
      )}
    </Stack>
  )
}

function SearchBar({ term, onSubmit }: { term: string; onSubmit: (term: string) => void }) {
  const [value, setValue] = useState(term)
  const [synced, setSynced] = useState(term)
  if (term !== synced) {
    setSynced(term)
    setValue(term)
  }
  return (
    <form
      role="search"
      onSubmit={(e) => {
        e.preventDefault()
        onSubmit(value)
      }}
    >
      <TextInput
        size="lg"
        type="search"
        aria-label="Search documents"
        data-global-search
        placeholder="Search your archive…"
        autoComplete="off"
        leftSection={<IconSearch size={20} />}
        rightSectionWidth={48}
        rightSection={
          <ActionIcon type="submit" size="lg" variant="filled" aria-label="Search">
            <IconArrowRight size={18} />
          </ActionIcon>
        }
        value={value}
        onChange={(e) => setValue(e.currentTarget.value)}
      />
    </form>
  )
}

function Overview({ onSearch }: { onSearch: (term: string, filters: Filters) => void }) {
  const recent = useDocumentList(8)
  const history = recentSearches.use()

  return (
    <>
      <Group gap="xs">
        {CATEGORIES.map((c) => (
          <Button
            key={c.type}
            size="compact-sm"
            radius="xl"
            variant="light"
            onClick={() => onSearch('', { docTypes: [c.type] })}
          >
            {c.label}
          </Button>
        ))}
        {history.slice(0, 4).map((t) => (
          <Button
            key={t}
            size="compact-sm"
            radius="xl"
            variant="default"
            fw={400}
            leftSection={<IconHistory size={14} />}
            onClick={() => onSearch(t, {})}
          >
            {t}
          </Button>
        ))}
        {history.length > 0 && (
          <CloseButton
            size="sm"
            aria-label="Clear recent searches"
            onClick={() => recentSearches.set([])}
          />
        )}
      </Group>

      <Grid gap="lg">
        <Grid.Col span={{ base: 12, md: 8 }}>
          <Group justify="space-between" mb="xs">
            <Title order={4}>Recently added</Title>
            <Anchor component={Link} to="/documents" size="sm">
              View all{recent.total ? ` ${formatNumber(recent.total)}` : ''}
            </Anchor>
          </Group>
          <DocumentResults
            bare
            documents={recent.documents.slice(0, 8)}
            loading={recent.isPending}
            error={recent.isError ? 'Documents could not be loaded.' : null}
            onRetry={() => recent.refetch()}
            empty={{
              title: 'Your archive starts here',
              message: 'Add your first scan or PDF — every page becomes searchable.',
              action: emptyActions.upload
            }}
          />
        </Grid.Col>
        <Grid.Col span={{ base: 12, md: 4 }}>
          <Stack gap="lg">
            <Upcoming />
            <SavedSearches onSelect={(s) => onSearch(s.term, s.filters)} />
          </Stack>
        </Grid.Col>
      </Grid>
    </>
  )
}

function Results({
  term,
  filters,
  onChange
}: {
  term: string
  filters: Filters
  onChange: (term: string, filters: Filters) => void
}) {
  const search = useSearch(term, filters)
  const facets = useFacets(term, filters)
  const [filtersOpen, filtersDrawer] = useDisclosure(false)
  // The sidebar replaces the drawer from the md breakpoint up.
  const wide = useMediaQuery('(min-width: 62em)')
  if (wide && filtersOpen) filtersDrawer.close()
  const active = countFilters(filters)
  const setFilters = (next: Filters) => onChange(term, next)
  const panel = <SearchFilters filters={filters} facets={facets.data} onChange={setFilters} />

  return (
    <Grid gap="lg">
      <Grid.Col span={{ base: 12, md: 3 }} visibleFrom="md">
        <Card withBorder padding="md" pos="sticky" top={72}>
          {panel}
        </Card>
      </Grid.Col>
      <Grid.Col span={{ base: 12, md: 9 }}>
        <DocumentResults
          title={
            <Group gap="xs">
              <Title order={4} aria-live="polite">
                {search.isPending
                  ? 'Searching…'
                  : `${formatNumber(search.total)} result${search.total === 1 ? '' : 's'}`}
              </Title>
              {active > 0 && (
                <Badge
                  variant="light"
                  rightSection={
                    <CloseButton
                      size="xs"
                      aria-label="Clear search filters"
                      onClick={() => setFilters({})}
                    />
                  }
                >
                  {active} filter{active === 1 ? '' : 's'}
                </Badge>
              )}
            </Group>
          }
          actions={
            <>
              <Indicator disabled={!active} size={8} hiddenFrom="md">
                <ActionIcon
                  variant="default"
                  aria-label="Open search filters"
                  onClick={filtersDrawer.open}
                >
                  <IconAdjustments size={16} />
                </ActionIcon>
              </Indicator>
              <SaveSearchButton term={term} filters={filters} />
              <Tooltip label="Clear search">
                <ActionIcon
                  variant="default"
                  aria-label="Clear search"
                  onClick={() => onChange('', {})}
                >
                  <IconX size={16} />
                </ActionIcon>
              </Tooltip>
            </>
          }
          documents={search.documents}
          loading={search.isPending}
          error={
            search.isError
              ? search.documents.length
                ? 'More results could not be loaded.'
                : 'Search could not be loaded. Check your connection and try again.'
              : null
          }
          onRetry={() => (search.documents.length ? search.fetchNextPage() : search.refetch())}
          hasMore={search.hasNextPage}
          loadingMore={search.isFetchingNextPage}
          onLoadMore={() => search.fetchNextPage()}
          empty={{
            title: term ? `No results for “${term}”` : 'No matching documents',
            message: 'Try different words or remove some filters.',
            action: emptyActions.browse
          }}
        />
      </Grid.Col>
      <Drawer
        opened={filtersOpen}
        onClose={filtersDrawer.close}
        title="Search filters"
        position="bottom"
        size="85%"
      >
        <Box pb="md">{panel}</Box>
      </Drawer>
    </Grid>
  )
}

function SaveSearchButton({ term, filters }: { term: string; filters: Filters }) {
  const [opened, popover] = useDisclosure(false)
  const [name, setName] = useState('')
  const [error, setError] = useState('')
  return (
    <Popover
      opened={opened}
      onChange={(o) => (o ? popover.open() : popover.close())}
      position="bottom-end"
      trapFocus
      withArrow
    >
      <Popover.Target>
        <Tooltip label="Save search">
          <ActionIcon
            variant="default"
            aria-label="Save search"
            aria-expanded={opened}
            onClick={() => {
              setName(term || 'Filtered documents')
              setError('')
              popover.toggle()
            }}
          >
            <IconBookmark size={16} />
          </ActionIcon>
        </Tooltip>
      </Popover.Target>
      <Popover.Dropdown>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            const result = saveSearch(name, term, filters)
            if (result.ok) popover.close()
            else setError(result.error)
          }}
        >
          <Stack gap="xs" w={240}>
            <TextInput
              size="xs"
              label="Search name"
              maxLength={80}
              required
              data-autofocus
              value={name}
              error={error || undefined}
              onChange={(e) => setName(e.currentTarget.value)}
            />
            <Text size="xs" c="dimmed">
              Saved in this browser only.
            </Text>
            <Button size="xs" type="submit">
              Save
            </Button>
          </Stack>
        </form>
      </Popover.Dropdown>
    </Popover>
  )
}
