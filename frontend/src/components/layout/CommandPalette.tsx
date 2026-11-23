import { useState } from 'react'
import { Group, Loader, Text, useComputedColorScheme, useMantineColorScheme } from '@mantine/core'
import { useDebouncedValue } from '@mantine/hooks'
import { Spotlight, spotlight } from '@mantine/spotlight'
import {
  IconFiles,
  IconHistory,
  IconKeyboard,
  IconMessageCircle,
  IconMoon,
  IconSearch,
  IconSettings,
  IconStar,
  IconUpload
} from '@tabler/icons-react'
import { useNavigate } from 'react-router'
import { useSearch } from '@/hooks/queries'
import { useDocumentDrawer } from '@/hooks/useDocumentDrawer'
import { archiveQuery } from '@/lib/archiveSearch'
import { documentCompany, documentTitle } from '@/lib/document'
import { formatDate } from '@/lib/format'
import { recentSearches } from '@/stores/local'
import { DocumentThumbnail } from '@/components/documents/DocumentThumbnail'

const icon = (Icon: typeof IconSearch) => <Icon size={18} stroke={1.6} />

export function CommandPalette({ onShowShortcuts }: { onShowShortcuts: () => void }) {
  const [query, setQuery] = useState('')
  const [debounced] = useDebouncedValue(query.trim(), 200)
  const navigate = useNavigate()
  const drawer = useDocumentDrawer()
  const recent = recentSearches.use()
  const { setColorScheme } = useMantineColorScheme()
  const dark = useComputedColorScheme('light') === 'dark'
  const docs = useSearch(debounced, {}, 5)

  const commands = [
    { label: 'Search', icon: IconSearch, run: () => navigate('/') },
    { label: 'All documents', icon: IconFiles, run: () => navigate('/documents') },
    { label: 'Favorites', icon: IconStar, run: () => navigate('/favorites') },
    { label: 'Upload documents', icon: IconUpload, run: () => navigate('/upload') },
    { label: 'Ask your archive', icon: IconMessageCircle, run: () => navigate('/chat') },
    { label: 'Admin', icon: IconSettings, run: () => navigate('/admin') },
    {
      label: dark ? 'Switch to light mode' : 'Switch to dark mode',
      icon: IconMoon,
      run: () => setColorScheme(dark ? 'light' : 'dark')
    },
    { label: 'Keyboard shortcuts', icon: IconKeyboard, run: onShowShortcuts },
    ...recent.slice(0, 5).map((term) => ({
      label: `Search “${term}”`,
      icon: IconHistory,
      run: () => navigate({ pathname: '/', search: archiveQuery(term).toString() })
    }))
  ]
  const q = query.trim().toLowerCase()
  const matching = commands.filter((c) => c.label.toLowerCase().includes(q))

  return (
    <Spotlight.Root query={query} onQueryChange={setQuery} scrollable maxHeight="60vh">
      <Spotlight.Search
        placeholder="Search documents or type a command…"
        aria-label="Command palette input"
        leftSection={<IconSearch size={18} />}
      />
      <Spotlight.ActionsList>
        {q && (
          <Spotlight.Action
            label={`Search all documents for “${query.trim()}”`}
            leftSection={icon(IconSearch)}
            onClick={() => navigate({ pathname: '/', search: archiveQuery(query).toString() })}
          />
        )}
        {matching.length > 0 && (
          <Spotlight.ActionsGroup label="Commands">
            {matching.map((c) => (
              <Spotlight.Action
                key={c.label}
                label={c.label}
                leftSection={icon(c.icon)}
                onClick={c.run}
              />
            ))}
          </Spotlight.ActionsGroup>
        )}
        {debounced && (
          <Spotlight.ActionsGroup label="Documents">
            {docs.isFetching && !docs.documents.length && (
              <Group p="sm" gap="xs">
                <Loader size="xs" />
                <Text size="sm" c="dimmed">
                  Searching…
                </Text>
              </Group>
            )}
            {docs.documents.slice(0, 5).map((doc) => (
              <Spotlight.Action
                key={doc._id}
                onClick={() => {
                  spotlight.close()
                  drawer.open(doc._id)
                }}
              >
                <Group wrap="nowrap" gap="sm" w="100%">
                  <DocumentThumbnail id={doc._id} w={32} h={42} />
                  <div style={{ minWidth: 0 }}>
                    <Text size="sm" truncate>
                      {documentTitle(doc)}
                    </Text>
                    <Text size="xs" c="dimmed" truncate>
                      {[documentCompany(doc), formatDate(doc._source.date)]
                        .filter(Boolean)
                        .join(' · ')}
                    </Text>
                  </div>
                </Group>
              </Spotlight.Action>
            ))}
          </Spotlight.ActionsGroup>
        )}
        {!q && !matching.length && <Spotlight.Empty>Nothing found</Spotlight.Empty>}
      </Spotlight.ActionsList>
    </Spotlight.Root>
  )
}
