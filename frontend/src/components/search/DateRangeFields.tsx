import { SimpleGrid, TextInput } from '@mantine/core'

interface Props {
  from?: string
  to?: string
  onChange: (range: { from?: string; to?: string }) => void
  size?: 'xs' | 'sm'
  stacked?: boolean
}

/** Native date inputs: keyboard friendly and the value is already `YYYY-MM-DD`. */
export function DateRangeFields({ from, to, onChange, size = 'xs', stacked }: Props) {
  return (
    <SimpleGrid cols={stacked ? 1 : 2} spacing="xs" verticalSpacing={6}>
      <TextInput
        size={size}
        type="date"
        label="From"
        value={from ?? ''}
        max={to || undefined}
        onChange={(e) => onChange({ from: e.currentTarget.value || undefined, to })}
      />
      <TextInput
        size={size}
        type="date"
        label="To"
        value={to ?? ''}
        min={from || undefined}
        onChange={(e) => onChange({ from, to: e.currentTarget.value || undefined })}
      />
    </SimpleGrid>
  )
}
