import { createBrowserRouter, Navigate, useParams, type RouteObject } from 'react-router'
import { AppLayout } from '@/components/layout/AppLayout'
import { RouteError } from '@/components/layout/RouteError'
import SearchPage from '@/pages/SearchPage'

const page = (load: () => Promise<{ default: React.ComponentType }>) => async () => ({
  Component: (await load()).default
})

/** Legacy deep links (`/documents/:id`) open the document drawer over the documents list. */
function DocumentRedirect() {
  const { id = '' } = useParams()
  return <Navigate replace to={`/documents?${new URLSearchParams({ doc: id })}`} />
}

export const routes: RouteObject[] = [
  {
    path: '/',
    Component: AppLayout,
    ErrorBoundary: RouteError,
    children: [
      { index: true, Component: SearchPage },
      { path: 'documents', lazy: page(() => import('@/pages/DocumentsPage')) },
      { path: 'documents/:id', Component: DocumentRedirect },
      { path: 'favorites', lazy: page(() => import('@/pages/FavoritesPage')) },
      { path: 'upload', lazy: page(() => import('@/pages/UploadPage')) },
      { path: 'chat', lazy: page(() => import('@/pages/ChatPage')) },
      { path: 'shares', lazy: page(() => import('@/pages/SharesPage')) },
      { path: 'admin', lazy: page(() => import('@/pages/AdminPage')) },
      { path: '*', Component: () => <Navigate replace to="/" /> }
    ]
  }
]

export const router = createBrowserRouter(routes, {
  basename: import.meta.env.BASE_URL.replace(/\/$/, '') || '/'
})
