import { useState } from 'react'
import {
  Alert,
  Button,
  CopyButton,
  Group,
  Modal,
  NumberInput,
  PasswordInput,
  Stack,
  Text,
  TextInput
} from '@mantine/core'
import { api } from '@/api/client'
import { useCreateShare } from '@/hooks/queries'
import { parseDocumentId } from '@/lib/document'
import { errorMessage } from '@/lib/utils'

export function ShareModal({
  docId,
  opened,
  onClose
}: {
  docId: string
  opened: boolean
  onClose: () => void
}) {
  return (
    <Modal opened={opened} onClose={onClose} title="Share document">
      {/* Remount per open so the form starts fresh. */}
      {opened && <ShareForm docId={docId} onClose={onClose} />}
    </Modal>
  )
}

function ShareForm({ docId, onClose }: { docId: string; onClose: () => void }) {
  const target = parseDocumentId(docId)
  const create = useCreateShare()
  const [hours, setHours] = useState<number | string>(24)
  const [maxViews, setMaxViews] = useState<number | string>(0)
  const [passphrase, setPassphrase] = useState('')

  if (!target) return <Text c="dimmed">This document can’t be shared.</Text>

  if (create.data) {
    const url = api.shareUrl(create.data.token)
    return (
      <Stack>
        <TextInput
          label="Share link"
          value={url}
          readOnly
          onFocus={(e) => e.currentTarget.select()}
        />
        {passphrase && (
          <Text size="xs" c="dimmed">
            Send the passphrase separately — it isn’t part of the link.
          </Text>
        )}
        <Group justify="flex-end">
          <CopyButton value={url}>
            {({ copied, copy }) => (
              <Button variant="default" onClick={copy}>
                {copied ? 'Copied' : 'Copy link'}
              </Button>
            )}
          </CopyButton>
          <Button onClick={onClose}>Done</Button>
        </Group>
      </Stack>
    )
  }

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        create.mutate({
          scanID: target.scanID,
          sequenceID: target.sequenceID,
          expiresInHours: Number(hours) > 0 ? Number(hours) : 24,
          maxViews: Number(maxViews) > 0 ? Number(maxViews) : 0,
          passphrase: passphrase || undefined
        })
      }}
    >
      <Stack>
        <Text size="sm" c="dimmed">
          Anyone with the link can view this page until it expires or is revoked.
        </Text>
        <Group grow>
          <NumberInput label="Expires after (hours)" min={1} value={hours} onChange={setHours} />
          <NumberInput
            label="Max views"
            description="0 = unlimited"
            min={0}
            value={maxViews}
            onChange={setMaxViews}
          />
        </Group>
        <PasswordInput
          label="Passphrase"
          placeholder="Optional"
          value={passphrase}
          onChange={(e) => setPassphrase(e.currentTarget.value)}
        />
        {create.isError && (
          <Alert color="red" variant="light" role="alert">
            {errorMessage(create.error, 'Unable to create share link')}
          </Alert>
        )}
        <Group justify="flex-end">
          <Button variant="default" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" loading={create.isPending}>
            Create link
          </Button>
        </Group>
      </Stack>
    </form>
  )
}
