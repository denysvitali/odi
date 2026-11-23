import { createTheme, localStorageColorSchemeManager } from '@mantine/core'
import { STORAGE_KEYS } from '@/lib/constants'

export const theme = createTheme({
  primaryColor: 'teal',
  defaultRadius: 'md',
  cursorType: 'pointer',
  fontFamily:
    'Inter, ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, "Helvetica Neue", sans-serif',
  headings: { fontWeight: '600' }
})

export const colorSchemeManager = localStorageColorSchemeManager({ key: STORAGE_KEYS.COLOR_SCHEME })
