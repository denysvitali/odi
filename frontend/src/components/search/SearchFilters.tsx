import { useState } from 'react'
import {
  Button,
  Checkbox,
  Chip,
  Group,
  SegmentedControl,
  Stack,
  Text,
  TextInput
} from '@mantine/core'
import { useDebouncedCallback } from '@mantine/hooks'
import type { SearchFilters as Filters } from '@/api/client'
import type { Bucket, Facets } from '@/hooks/queries'
import { countFilters } from '@/lib/archiveSearch'
import { DateRangeFields } from './DateRangeFields'

interface Props {
  filters: Filters
  facets?: Facets
  onChange: (next: Filters) => void
}

/** Keep selected values visible even when the current result set no longer contains them. */
function withSelected(buckets: Bucket[] = [], selected: string[] = []): Bucket[] {
  return [
    ...buckets,
    ...selected.filter((k) => !buckets.some((b) => b.key === k)).map((key) => ({ key, count: 0 }))
  ]
}

export function SearchFilters({ filters, facets, onChange }: Props) {
  const set = (patch: Partial<Filters>) => onChange({ ...filters, ...patch })
  const companies = withSelected(facets?.companies, filters.companies)
  const docTypes = withSelected(facets?.docTypes, filters.docTypes)
  const tags = withSelected(facets?.tags, filters.tags)
  const active = countFilters(filters)

  return (
    <Stack gap="lg">
      <Group justify="space-between">
        <Text fw={600} size="sm">
          Filters
        </Text>
        {active > 0 && (
          <Button size="compact-xs" variant="subtle" onClick={() => onChange({})}>
            Clear all
          </Button>
        )}
      </Group>

      {docTypes.length > 0 && (
        <Section label="Type">
          <ChipList
            buckets={docTypes}
            value={filters.docTypes ?? []}
            onChange={(v) => set({ docTypes: v.length ? v : undefined })}
          />
        </Section>
      )}

      {companies.length > 0 && (
        <Section label="Company">
          <Checkbox.Group
            value={filters.companies ?? []}
            onChange={(v) => set({ companies: v.length ? v : undefined })}
          >
            <Stack gap={6}>
              {companies.map((b) => (
                <Checkbox
                  key={b.key}
                  value={b.key}
                  size="xs"
                  label={
                    <Group gap={6} wrap="nowrap">
                      <Text size="xs" lineClamp={1}>
                        {b.key}
                      </Text>
                      {b.count > 0 && (
                        <Text size="xs" c="dimmed">
                          {b.count}
                        </Text>
                      )}
                    </Group>
                  }
                />
              ))}
            </Stack>
          </Checkbox.Group>
        </Section>
      )}

      <Section label="Date">
        <DateRangeFields
          stacked
          from={filters.dateFrom}
          to={filters.dateTo}
          onChange={({ from, to }) => set({ dateFrom: from, dateTo: to })}
        />
      </Section>

      <Section label="Barcode">
        <SegmentedControl
          size="xs"
          fullWidth
          aria-label="Barcode presence"
          value={filters.hasBarcode === undefined ? 'any' : String(filters.hasBarcode)}
          onChange={(v) => set({ hasBarcode: v === 'any' ? undefined : v === 'true' })}
          data={[
            { value: 'any', label: 'Any' },
            { value: 'true', label: 'With' },
            { value: 'false', label: 'Without' }
          ]}
        />
        {facets && facets.barcodeCount > 0 && (
          <Text size="xs" c="dimmed" mt={4}>
            {facets.barcodeCount} with barcode
          </Text>
        )}
      </Section>

      <Section label="Title">
        <TitleFilter
          value={filters.titleFilter ?? ''}
          onChange={(v) => set({ titleFilter: v || undefined })}
        />
      </Section>

      {tags.length > 0 && (
        <Section label="Tags">
          <ChipList
            buckets={tags}
            value={filters.tags ?? []}
            onChange={(v) => set({ tags: v.length ? v : undefined })}
          />
        </Section>
      )}
    </Stack>
  )
}

function Section({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <Text size="xs" fw={500} c="dimmed" tt="uppercase" mb={6}>
        {label}
      </Text>
      {children}
    </div>
  )
}

function ChipList({
  buckets,
  value,
  onChange
}: {
  buckets: Bucket[]
  value: string[]
  onChange: (v: string[]) => void
}) {
  return (
    <Chip.Group multiple value={value} onChange={onChange}>
      <Group gap={6}>
        {buckets.map((b) => (
          <Chip key={b.key} value={b.key} size="xs" tt="capitalize">
            {b.key}
            {b.count > 0 ? ` ${b.count}` : ''}
          </Chip>
        ))}
      </Group>
    </Chip.Group>
  )
}

/** Debounced so typing doesn't fire a search per keystroke. */
function TitleFilter({ value, onChange }: { value: string; onChange: (v: string) => void }) {
  const [local, setLocal] = useState(value)
  const [synced, setSynced] = useState(value)
  const commit = useDebouncedCallback(onChange, 400)
  // Adopt external changes (e.g. "Clear all") without clobbering in-progress typing.
  if (value !== synced) {
    setSynced(value)
    if (local.trim() !== value) setLocal(value)
  }
  return (
    <TextInput
      size="xs"
      aria-label="Document title"
      placeholder="Title contains…"
      value={local}
      onChange={(e) => {
        setLocal(e.currentTarget.value)
        commit(e.currentTarget.value.trim())
      }}
    />
  )
}
