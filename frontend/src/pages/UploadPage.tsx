import { useEffect, useRef, useState } from 'react'
import {
  ActionIcon,
  Alert,
  Badge,
  Button,
  Card,
  Code,
  Group,
  Image,
  Loader,
  Progress,
  SimpleGrid,
  Stack,
  Text
} from '@mantine/core'
import { Dropzone } from '@mantine/dropzone'
import { useQueryClient } from '@tanstack/react-query'
import { IconCamera, IconCheck, IconCloudUpload, IconX } from '@tabler/icons-react'
import { Link } from 'react-router'
import { PageHeader } from '@/components/layout/PageHeader'
import { useObjectUrl } from '@/hooks/useObjectUrl'
import { formatBytes } from '@/lib/format'
import { splitPdf } from '@/lib/pdfSplitter'
import { isPdf, takeStashedFiles, uploadFiles, type UploadResult } from '@/lib/upload'
import { errorMessage } from '@/lib/utils'

interface Page {
  key: string
  file: File
  /** Set on the first page of a split PDF. */
  pdfPages?: number
}

let nextKey = 0

export default function UploadPage() {
  const [pages, setPages] = useState<Page[]>([])
  const [splitting, setSplitting] = useState<{ name: string; done: number; total: number } | null>(
    null
  )
  const [progress, setProgress] = useState(0)
  const [attempt, setAttempt] = useState(0)
  const [uploading, setUploading] = useState(false)
  const [result, setResult] = useState<UploadResult | null>(null)
  const [error, setError] = useState<string | null>(null)
  const abort = useRef<AbortController | null>(null)
  const client = useQueryClient()

  const add = async (files: File[]) => {
    setError(null)
    for (const file of files) {
      if (!isPdf(file)) {
        setPages((p) => [...p, { key: String(nextKey++), file }])
        continue
      }
      setSplitting({ name: file.name, done: 0, total: 0 })
      try {
        const split = await splitPdf(file, (done, total) =>
          setSplitting({ name: file.name, done, total })
        )
        setPages((p) => [
          ...p,
          ...split.files.map((f, i) => ({
            key: String(nextKey++),
            file: f,
            pdfPages: i === 0 ? split.pageCount : undefined
          }))
        ])
      } catch (err) {
        setError(`Failed to process “${file.name}”: ${errorMessage(err, String(err))}`)
      } finally {
        setSplitting(null)
      }
    }
  }

  useEffect(() => {
    const stashed = takeStashedFiles()
    // Files dropped on another page: an external hand-off, consumed once.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (stashed.length) void add(stashed)
    return () => abort.current?.abort()
  }, [])

  const upload = async () => {
    const controller = new AbortController()
    abort.current = controller
    setUploading(true)
    setError(null)
    setProgress(0)
    setAttempt(0)
    try {
      const res = await uploadFiles(
        pages.map((p) => p.file),
        { signal: controller.signal, onProgress: setProgress, onAttempt: setAttempt }
      )
      setResult(res)
      void client.invalidateQueries({ queryKey: ['documents'] })
    } catch (err) {
      if (!controller.signal.aborted) setError(errorMessage(err, 'Upload failed'))
    } finally {
      setUploading(false)
    }
  }

  const reset = () => {
    setPages([])
    setResult(null)
    setError(null)
  }

  const totalSize = pages.reduce((n, p) => n + p.file.size, 0)
  const busy = uploading || !!splitting

  return (
    <Stack maw={960} mx="auto" gap="md">
      <PageHeader title="Upload" description="Images or PDFs. Every page is OCR’d and indexed." />

      {result ? (
        <UploadSummary result={result} onReset={reset} />
      ) : (
        <>
          <Dropzone
            onDrop={add}
            accept={['image/*', 'application/pdf']}
            disabled={busy}
            inputProps={{ 'aria-label': 'Select files' }}
            p="xl"
          >
            <Stack align="center" gap={4} style={{ pointerEvents: 'none' }}>
              <Dropzone.Accept>
                <IconCheck size={40} color="var(--mantine-primary-color-filled)" />
              </Dropzone.Accept>
              <Dropzone.Reject>
                <IconX size={40} color="var(--mantine-color-red-6)" />
              </Dropzone.Reject>
              <Dropzone.Idle>
                <IconCloudUpload size={40} stroke={1.4} color="var(--mantine-color-dimmed)" />
              </Dropzone.Idle>
              <Text fw={500}>Drop images or PDFs here</Text>
              <Text size="sm" c="dimmed">
                or click to browse
              </Text>
            </Stack>
          </Dropzone>
          <Group justify="center" hiddenFrom="sm">
            <CameraButton onCapture={(f) => add([f])} disabled={busy} />
          </Group>

          {splitting && (
            <Group gap="sm">
              <Loader size="xs" />
              <Text size="sm">
                Splitting {splitting.name}… {splitting.done}/{splitting.total || '?'} pages
              </Text>
            </Group>
          )}

          {error && (
            <Alert color="red" variant="light" role="alert">
              <Group justify="space-between">
                <Text size="sm">{error}</Text>
                {pages.length > 0 && !uploading && (
                  <Button size="xs" variant="default" onClick={upload}>
                    Retry
                  </Button>
                )}
              </Group>
            </Alert>
          )}

          {pages.length > 0 && (
            <Card withBorder>
              <Group justify="space-between" mb="sm">
                <Text fw={500}>
                  {pages.length} page{pages.length === 1 ? '' : 's'}{' '}
                  <Text span c="dimmed" size="sm">
                    {formatBytes(totalSize)}
                  </Text>
                </Text>
                <Button
                  size="xs"
                  variant="subtle"
                  color="gray"
                  disabled={busy}
                  onClick={() => setPages([])}
                >
                  Clear all
                </Button>
              </Group>
              <SimpleGrid
                cols={{ base: 3, sm: 5, md: 6 }}
                spacing="xs"
                mah={420}
                style={{ overflow: 'auto' }}
              >
                {pages.map((p) => (
                  <PagePreview
                    key={p.key}
                    page={p}
                    disabled={busy}
                    onRemove={() => setPages((all) => all.filter((x) => x.key !== p.key))}
                  />
                ))}
              </SimpleGrid>
              {uploading ? (
                <Stack gap={4} mt="md">
                  <Progress value={progress} animated />
                  <Group justify="space-between">
                    <Text size="xs" c="dimmed">
                      Uploading… {progress}%{attempt > 1 ? ` · attempt ${attempt}` : ''}
                    </Text>
                    <Button
                      size="compact-xs"
                      variant="subtle"
                      onClick={() => abort.current?.abort()}
                    >
                      Cancel
                    </Button>
                  </Group>
                </Stack>
              ) : (
                <Button
                  fullWidth
                  mt="md"
                  onClick={upload}
                  disabled={!!splitting}
                  leftSection={<IconCloudUpload size={16} />}
                >
                  Upload {pages.length} file(s)
                </Button>
              )}
            </Card>
          )}
        </>
      )}
    </Stack>
  )
}

function PagePreview({
  page,
  disabled,
  onRemove
}: {
  page: Page
  disabled: boolean
  onRemove: () => void
}) {
  const url = useObjectUrl(page.file)
  return (
    <Card padding={0} withBorder pos="relative" style={{ aspectRatio: '3 / 4' }}>
      {url && <Image src={url} alt={page.file.name} h="100%" fit="cover" />}
      {page.pdfPages && (
        <Badge size="xs" pos="absolute" top={4} left={4}>
          {page.pdfPages} pages
        </Badge>
      )}
      <ActionIcon
        size="sm"
        variant="white"
        color="gray"
        pos="absolute"
        top={4}
        right={4}
        aria-label={`Remove ${page.file.name}`}
        disabled={disabled}
        onClick={onRemove}
      >
        <IconX size={14} />
      </ActionIcon>
    </Card>
  )
}

function CameraButton({
  onCapture,
  disabled
}: {
  onCapture: (f: File) => void
  disabled: boolean
}) {
  const input = useRef<HTMLInputElement>(null)
  return (
    <>
      <input
        ref={input}
        type="file"
        accept="image/*"
        capture="environment"
        hidden
        aria-label="Capture from camera"
        onChange={(e) => {
          const file = e.currentTarget.files?.[0]
          if (file) onCapture(file)
          e.currentTarget.value = ''
        }}
      />
      <Button
        variant="default"
        leftSection={<IconCamera size={16} />}
        disabled={disabled}
        onClick={() => input.current?.click()}
      >
        Take a photo
      </Button>
    </>
  )
}

function UploadSummary({ result, onReset }: { result: UploadResult; onReset: () => void }) {
  const ok = result.failed === 0
  return (
    <Alert
      color={ok ? 'teal' : 'orange'}
      variant="light"
      title={ok ? 'Upload complete' : 'Upload completed with errors'}
    >
      <Stack gap="xs">
        <Text size="sm">
          {result.processed} indexed
          {result.duplicates ? `, ${result.duplicates} duplicate(s) skipped` : ''}
          {result.failed ? `, ${result.failed} failed` : ''}. Scan <Code>{result.scanID}</Code>
        </Text>
        {!ok &&
          result.pages
            .filter((p) => p.status !== 'indexed' && p.status !== 'duplicate')
            .map((p) => (
              <Text key={p.sequenceID} size="xs">
                Page {p.sequenceID}: {p.status}
                {p.error ? ` — ${p.error}` : ''}
              </Text>
            ))}
        <Group gap="xs">
          <Button size="xs" component={Link} to="/documents">
            View documents
          </Button>
          <Button size="xs" variant="default" onClick={onReset}>
            Upload more
          </Button>
        </Group>
      </Stack>
    </Alert>
  )
}
