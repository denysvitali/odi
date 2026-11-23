import { Button, Center, Stack, Text, Title } from '@mantine/core'
import { useRouteError } from 'react-router'
import { errorMessage } from '@/lib/utils'
import { logger } from '@/lib/logger'

export function RouteError() {
  const error = useRouteError()
  logger.error('Unhandled route error', error)
  return (
    <Center mih="60vh" p="md">
      <Stack align="center" gap="xs" maw={420} ta="center">
        <Title order={3}>Something went wrong</Title>
        <Text c="dimmed" size="sm">
          {errorMessage(error, 'An unexpected error occurred.')}
        </Text>
        <Button variant="default" onClick={() => window.location.reload()}>
          Reload
        </Button>
      </Stack>
    </Center>
  )
}
