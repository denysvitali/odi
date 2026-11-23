import { useState } from 'react'
import {
  ActionIcon,
  Alert,
  Anchor,
  Badge,
  Button,
  CopyButton,
  Drawer,
  Group,
  Skeleton,
  Stack,
  Tabs,
  TagsInput,
  Text,
  Tooltip,
  UnstyledButton
} from '@mantine/core'
import { IconCheck, IconDownload, IconExternalLink, IconLink, IconShare } from '@tabler/icons-react'
import { useDocumentDetails } from '@/hooks/queries'
import { useDocumentDrawer } from '@/hooks/useDocumentDrawer'
import { useDocumentFile } from '@/hooks/useDocumentFile'
import { getOpensearchUrl } from '@/lib/config'
import { extractCompanyFromText, extractTitleFromText } from '@/lib/documentMetadata'
import { formatDate, formatDateTime } from '@/lib/format'
import { errorMessage } from '@/lib/utils'
import { addTag, removeTag, tags as tagStore } from '@/stores/local'
import type { DocumentDetails } from '@/types/documents'
import { BarcodeCard } from './BarcodeCard'
import { DocTypeBadge } from './DocTypeBadge'
import { DocumentText } from './DocumentText'
import { DocumentThumbnail } from './DocumentThumbnail'
import { ShareModal } from './ShareModal'
import { StarButton } from './StarButton'
import { SummaryPanel } from './SummaryPanel'

export function DocumentDrawer() {
  const { openId, close } = useDocumentDrawer()
  const details = useDocumentDetails(openId)
  const title = details.data
    ? details.data.title || extractTitleFromText(details.data.text || '') || 'Untitled document'
    : 'Document'

  return (
    <Drawer
      opened={!!openId}
      onClose={close}
      position="right"
      size="lg"
      title={
        <Text fw={600} lineClamp={2}>
          {title}
        </Text>
      }
    >
      {openId &&
        (details.isPending ? (
          <Stack>
            <Skeleton h={320} />
            <Skeleton h={20} w="60%" />
            <Skeleton h={20} w="40%" />
          </Stack>
        ) : details.isError ? (
          <Alert color="red" variant="light" title="Unable to open document" role="alert">
            <Text size="sm" mb="xs">
              {errorMessage(details.error, 'Failed to load document details')}
            </Text>
            <Button size="xs" variant="default" onClick={() => details.refetch()}>
              Try again
            </Button>
          </Alert>
        ) : (
          <DocumentBody id={openId} details={details.data} />
        ))}
    </Drawer>
  )
}

function DocumentBody({ id, details }: { id: string; details: DocumentDetails }) {
  const file = useDocumentFile(id)
  const [shareOpen, setShareOpen] = useState(false)
  const docTags = tagStore.use()[id] ?? []
  const company = details.company?.name || extractCompanyFromText(details.text || '')
  const barcodes = details.barcodes ?? []

  return (
    <Stack gap="md">
      <Group gap={4}>
        <StarButton id={id} size="lg" />
        <Tooltip label="Download">
          <ActionIcon
            variant="subtle"
            color="gray"
            size="lg"
            aria-label="Download document"
            onClick={file.download}
          >
            <IconDownload size={18} />
          </ActionIcon>
        </Tooltip>
        <Tooltip label="Create share link">
          <ActionIcon
            variant="subtle"
            color="gray"
            size="lg"
            aria-label="Share document"
            onClick={() => setShareOpen(true)}
          >
            <IconShare size={18} />
          </ActionIcon>
        </Tooltip>
        <CopyButton value={window.location.href}>
          {({ copied, copy }) => (
            <Tooltip label={copied ? 'Copied' : 'Copy link to this document'}>
              <ActionIcon
                variant="subtle"
                color={copied ? 'teal' : 'gray'}
                size="lg"
                aria-label="Copy link"
                onClick={copy}
              >
                {copied ? <IconCheck size={18} /> : <IconLink size={18} />}
              </ActionIcon>
            </Tooltip>
          )}
        </CopyButton>
        {getOpensearchUrl() && (
          <Tooltip label="Open in OpenSearch Dashboards">
            <ActionIcon
              component="a"
              href={`${getOpensearchUrl()}/app/discover#/doc/*/odi-*?id=${encodeURIComponent(id)}`}
              target="_blank"
              rel="noopener noreferrer"
              variant="subtle"
              color="gray"
              size="lg"
              aria-label="Open in OpenSearch"
            >
              <IconExternalLink size={18} />
            </ActionIcon>
          </Tooltip>
        )}
        <Text size="xs" c="dimmed" ff="monospace" ml="auto" truncate maw={180} title={id}>
          {id}
        </Text>
      </Group>

      {file.error && (
        <Alert color="red" variant="light" role="alert">
          {file.error}
        </Alert>
      )}

      <UnstyledButton onClick={file.open} aria-label="Open document image" title="Open full image">
        <DocumentThumbnail
          id={id}
          w="100%"
          h={360}
          alt={`Preview for document ${id}`}
          style={{ objectFit: 'contain', background: 'var(--mantine-color-default-hover)' }}
        />
      </UnstyledButton>

      <Stack gap={6}>
        {details.primaryDate && <Row label="Date">{formatDate(details.primaryDate)}</Row>}
        {company && (
          <Row label="Company">
            {company}
            {details.company?.uri && (
              <Anchor
                href={`https://www.zefix.ch/de/search/entity/list?name=${encodeURIComponent(company)}`}
                target="_blank"
                rel="noopener noreferrer"
                size="xs"
                ml={6}
              >
                Zefix <IconExternalLink size={12} style={{ verticalAlign: 'middle' }} />
              </Anchor>
            )}
            {details.company?.locality && (
              <Text size="xs" c="dimmed">
                {details.company.locality}
              </Text>
            )}
          </Row>
        )}
        {details.docType && (
          <Row label="Type">
            <DocTypeBadge type={details.docType} />
          </Row>
        )}
        {details.indexedAt && <Row label="Indexed">{formatDateTime(details.indexedAt)}</Row>}
      </Stack>

      <TagsInput
        label="Tags"
        description="Saved on this device"
        placeholder="Add tag…"
        size="xs"
        value={docTags}
        onChange={(next) => {
          next.filter((t) => !docTags.includes(t)).forEach((t) => addTag(id, t))
          docTags.filter((t) => !next.includes(t)).forEach((t) => removeTag(id, t))
        }}
        clearable
      />

      <Tabs defaultValue="summary" keepMounted={false}>
        <Tabs.List>
          <Tabs.Tab value="summary">Summary</Tabs.Tab>
          <Tabs.Tab value="text">Text</Tabs.Tab>
          <Tabs.Tab
            value="barcodes"
            disabled={!barcodes.length}
            rightSection={
              barcodes.length ? (
                <Badge size="xs" variant="light" circle>
                  {barcodes.length}
                </Badge>
              ) : undefined
            }
          >
            Barcodes
          </Tabs.Tab>
        </Tabs.List>
        <Tabs.Panel value="summary" pt="sm">
          <SummaryPanel id={id} initial={details} />
          {details.dates && details.dates.length > 1 && (
            <Group gap={4} mt="md">
              <Text size="xs" c="dimmed" mr={4}>
                Dates found
              </Text>
              {details.dates.map((d) => (
                <Badge key={d} variant="outline" color="gray" size="sm" tt="none">
                  {formatDate(d)}
                </Badge>
              ))}
            </Group>
          )}
        </Tabs.Panel>
        <Tabs.Panel value="text" pt="sm">
          <DocumentText text={details.text || ''} />
        </Tabs.Panel>
        <Tabs.Panel value="barcodes" pt="sm">
          <Stack>
            {barcodes.map((b, i) => (
              <BarcodeCard key={i} barcode={b} />
            ))}
          </Stack>
        </Tabs.Panel>
      </Tabs>

      <ShareModal docId={id} opened={shareOpen} onClose={() => setShareOpen(false)} />
    </Stack>
  )
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <Group gap="sm" wrap="nowrap" align="flex-start">
      <Text size="sm" c="dimmed" w={80} style={{ flexShrink: 0 }}>
        {label}
      </Text>
      <Text size="sm" component="div" style={{ minWidth: 0 }}>
        {children}
      </Text>
    </Group>
  )
}
