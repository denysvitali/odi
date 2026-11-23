import { Alert, Button, Stack, Text } from '@mantine/core'
import { DocumentResults, emptyActions } from '@/components/documents/DocumentResults'
import { PageHeader } from '@/components/layout/PageHeader'
import { useDocumentsById } from '@/hooks/queries'
import { favorites } from '@/stores/local'

export default function FavoritesPage() {
  const ids = favorites.use()
  const { documents, pending, failed, retry } = useDocumentsById(ids)

  return (
    <Stack gap={0} maw={1200} mx="auto">
      <PageHeader
        title="Favorites"
        description={`${ids.length} starred on this device`}
        actions={
          ids.length > 0 && (
            <Button size="xs" variant="subtle" color="gray" onClick={() => favorites.set([])}>
              Clear all
            </Button>
          )
        }
      />
      {failed > 0 && (
        <Alert color="orange" variant="light" mb="md" role="alert">
          <Text size="sm" mb="xs">
            {failed} {failed === 1 ? 'favorite' : 'favorites'} could not be loaded. Your stars are
            still saved.
          </Text>
          <Button size="xs" variant="default" onClick={retry} disabled={pending > 0}>
            Retry
          </Button>
        </Alert>
      )}
      <DocumentResults
        documents={documents}
        loading={pending > 0 && documents.length === 0}
        skeletons={Math.min(ids.length, 6)}
        empty={
          ids.length
            ? { title: 'No favorites available' }
            : {
                title: 'No favorites yet',
                message: 'Star documents to keep them here.',
                action: emptyActions.browse
              }
        }
      />
    </Stack>
  )
}
