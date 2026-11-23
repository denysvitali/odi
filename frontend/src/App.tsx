import '@mantine/core/styles.css'
import '@mantine/dropzone/styles.css'
import '@mantine/notifications/styles.css'
import '@mantine/spotlight/styles.css'

import { MantineProvider } from '@mantine/core'
import { Notifications } from '@mantine/notifications'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { RouterProvider } from 'react-router/dom'
import { colorSchemeManager, theme } from './theme'
import { router } from './router'

export function createQueryClient() {
  return new QueryClient({
    defaultOptions: {
      // The API client already retries transient failures.
      queries: { retry: false, refetchOnWindowFocus: false, staleTime: 30_000 }
    }
  })
}

const queryClient = createQueryClient()

export default function App() {
  return (
    <MantineProvider
      theme={theme}
      colorSchemeManager={colorSchemeManager}
      defaultColorScheme="auto"
    >
      <Notifications position="bottom-right" />
      <QueryClientProvider client={queryClient}>
        <RouterProvider router={router} />
      </QueryClientProvider>
    </MantineProvider>
  )
}
