import { Group, Kbd, Modal, Stack, Text } from '@mantine/core'
import { GO_TARGETS } from './useGoShortcuts'

const SHORTCUTS: [string[], string][] = [
  [['Ctrl/⌘', 'K'], 'Command palette'],
  [['/'], 'Focus search'],
  [['?'], 'Show shortcuts'],
  ...Object.entries(GO_TARGETS).map(([k, t]): [string[], string] => [['g', k], `Go to ${t.label}`])
]

export function ShortcutsModal({ opened, onClose }: { opened: boolean; onClose: () => void }) {
  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title="Keyboard shortcuts"
      closeButtonProps={{ 'aria-label': 'Close keyboard shortcuts' }}
    >
      <Stack gap="xs">
        {SHORTCUTS.map(([keys, label]) => (
          <Group key={label} justify="space-between">
            <Text size="sm">{label}</Text>
            <Group gap={4}>
              {keys.map((k) => (
                <Kbd key={k}>{k}</Kbd>
              ))}
            </Group>
          </Group>
        ))}
      </Stack>
    </Modal>
  )
}
