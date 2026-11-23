import {
  ActionIcon,
  Card,
  CopyButton,
  Group,
  SimpleGrid,
  Stack,
  Text,
  Tooltip
} from '@mantine/core'
import { IconCheck, IconCopy } from '@tabler/icons-react'
import { formatCurrency } from '@/lib/format'
import type { Barcode, QRBillData } from '@/types/documents'

function Copyable({ label, value, display }: { label: string; value: string; display?: string }) {
  return (
    <div>
      <Text size="xs" c="dimmed">
        {label}
      </Text>
      <Group gap={4} wrap="nowrap">
        <Text size="sm" ff="monospace" style={{ wordBreak: 'break-all' }}>
          {display ?? value}
        </Text>
        <CopyButton value={value}>
          {({ copied, copy }) => (
            <Tooltip label={copied ? 'Copied' : `Copy ${label}`}>
              <ActionIcon
                size="sm"
                variant="subtle"
                color={copied ? 'teal' : 'gray'}
                aria-label={`Copy ${label}`}
                onClick={copy}
              >
                {copied ? <IconCheck size={14} /> : <IconCopy size={14} />}
              </ActionIcon>
            </Tooltip>
          )}
        </CopyButton>
      </Group>
    </div>
  )
}

function Party({ label, party }: { label: string; party: QRBillData['creditor'] }) {
  if (!party) return null
  return (
    <div>
      <Text size="xs" c="dimmed">
        {label}
      </Text>
      <Text size="sm">{party.name}</Text>
      {party.address && (
        <Text size="xs" c="dimmed">
          {party.address}
        </Text>
      )}
      {(party.postalCode || party.city) && (
        <Text size="xs" c="dimmed">
          {party.postalCode} {party.city}
        </Text>
      )}
    </div>
  )
}

export function BarcodeCard({ barcode }: { barcode: Barcode }) {
  const bill = barcode.qrBill
  if (!bill)
    return (
      <Card withBorder padding="sm">
        <Copyable label="barcode" value={barcode.text} />
      </Card>
    )
  return (
    <Card withBorder padding="sm">
      <Stack gap="sm">
        <Group justify="space-between">
          <Text fw={600}>QR bill</Text>
          {bill.amount !== undefined && (
            <Text fw={600} size="lg">
              {formatCurrency(bill.amount, bill.currency)}
            </Text>
          )}
        </Group>
        {bill.iban && (
          <Copyable
            label="IBAN"
            value={bill.iban}
            display={bill.iban.replace(/(.{4})/g, '$1 ').trim()}
          />
        )}
        {bill.reference && <Copyable label="reference" value={bill.reference} />}
        <SimpleGrid cols={2}>
          <Party label="Creditor" party={bill.creditor} />
          <Party label="Debtor" party={bill.debtor} />
        </SimpleGrid>
        {bill.additionalInformation && (
          <Text size="xs" c="dimmed">
            {bill.additionalInformation}
          </Text>
        )}
      </Stack>
    </Card>
  )
}
