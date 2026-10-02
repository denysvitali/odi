import { expect, test, type Page } from '@playwright/test'

const token = 'synthetic-browser-test-token'
const png = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aZ1sAAAAASUVORK5CYII=',
  'base64'
)
const first = {
  _id: 'synthetic_0',
  _source: { title: 'Synthetic first document', text: 'Synthetic fixture', date: '2026-01-01' }
}
const second = {
  _id: 'synthetic_1',
  _source: { title: 'Synthetic second document', text: 'Synthetic fixture', date: '2026-01-02' }
}

interface ApiCall {
  path: string
  query: URLSearchParams
  body: Record<string, unknown> | null
  authorization?: string
}

async function mockArchive(page: Page) {
  const calls: ApiCall[] = []
  let failSearch = false
  await page.addInitScript((value) => localStorage.setItem('odi.apiToken', value), token)
  await page.route('**/settings.json', (route) => route.fulfill({
    json: { apiUrl: 'http://127.0.0.1:4173/api/v1', opensearchUrl: '' }
  }))
  await page.route('**/api/v1/**', async (route) => {
    const req = route.request()
    const url = new URL(req.url())
    const path = url.pathname.replace('/api/v1', '')
    const body = req.headers()['content-type']?.includes('application/json')
      ? req.postDataJSON() as Record<string, unknown>
      : null
    const authorization = req.headers().authorization
    calls.push({ path, query: url.searchParams, body, authorization })
    if (authorization !== `Bearer ${token}`) {
      await route.fulfill({ status: 401, json: { error: 'Authentication required' } })
    } else if (path === '/upload') {
      expect(req.postData()).toContain('synthetic.png')
      await route.fulfill({ json: {
        scanID: 'synthetic-upload', processed: 1, duplicates: 0, failed: 0,
        pages: [{ sequenceID: 0, status: 'indexed' }]
      } })
    } else if (path.startsWith('/thumbnails/') || path.startsWith('/files/')) {
      await route.fulfill({ contentType: 'image/png', body: png })
    } else if (path === '/documents/older_0') {
      await route.fulfill({ json: {
        id: 'older_0', title: 'Synthetic older document', text: 'Synthetic archived fixture',
        primaryDate: '2020-01-01'
      } })
    } else if (path === '/documents') {
      await route.fulfill({ json: { hits: { hits: [first], total: { value: 1, relation: 'eq' } } } })
    } else if (path === '/search') {
      if (failSearch) {
        await route.fulfill({ status: 503, json: { error: 'Synthetic search outage' } })
      } else {
        await route.fulfill({ json: {
          hits: { hits: body?.scrollId ? [second] : [first], total: { value: 2, relation: 'eq' } },
          _scroll_id: body?.scrollId ? undefined : 'synthetic-scroll'
        } })
      }
    } else if (path === '/search/facets') {
      await route.fulfill({ json: { hits: { total: { value: 2 } }, aggregations: {} } })
    } else {
      await route.fulfill({ status: 404, json: { error: 'Unknown synthetic endpoint' } })
    }
  })
  return { calls, failSearch: () => { failSearch = true } }
}

test('uploads a synthetic page with bearer authentication', async ({ page }) => {
  const archive = await mockArchive(page)
  await page.goto('/upload')
  await page.getByLabel('Select files').setInputFiles({
    name: 'synthetic.png', mimeType: 'image/png', buffer: png
  })
  await page.getByRole('button', { name: 'Upload 1 file(s)', exact: true }).click()
  await expect(page.getByText('Upload Complete', { exact: true })).toBeVisible()
  expect(archive.calls.filter((call) => call.path === '/upload')).toHaveLength(1)
})

test('opens an older deep-linked document and authenticated preview/full image', async ({ page }) => {
  const archive = await mockArchive(page)
  await page.goto('/documents/older_0')
  const dialog = page.getByRole('dialog')
  await expect(dialog).toContainText('Synthetic older document')
  const preview = dialog.getByAltText('Preview for document older_0')
  await expect(preview).toBeVisible()
  await expect.poll(() => preview.evaluate((img) => (img as HTMLImageElement).naturalWidth)).toBe(1)
  const popup = page.waitForEvent('popup')
  await dialog.getByRole('button', { name: 'Open document image', exact: true }).click()
  const imagePage = await popup
  await expect.poll(() => imagePage.url()).toContain('blob:')
  expect(archive.calls.some((call) => call.path === '/documents/older_0')).toBe(true)
  expect(archive.calls.some((call) => call.path === '/thumbnails/older_0')).toBe(true)
  expect(archive.calls.some((call) => call.path === '/files/older/0')).toBe(true)
  await imagePage.close()
})

test('applies and clears document date filters at the API boundary', async ({ page }) => {
  const archive = await mockArchive(page)
  await page.goto('/documents')
  await expect(page.getByText('Synthetic first document', { exact: true })).toBeVisible()
  await page.getByRole('button', { name: 'Filters', exact: true }).click()
  await page.getByLabel('From', { exact: true }).fill('2026-01-01')
  await page.getByLabel('To', { exact: true }).fill('2026-01-31')
  await page.getByRole('button', { name: 'Apply', exact: true }).click()
  await expect.poll(() => archive.calls.filter((call) => call.path === '/documents').at(-1)?.query.get('date_from')).toBe('2026-01-01')
  expect(archive.calls.filter((call) => call.path === '/documents').at(-1)?.query.get('date_to')).toBe('2026-01-31')
  await page.getByRole('button', { name: 'Filters', exact: true }).click()
  await page.getByRole('button', { name: 'Clear', exact: true }).click()
  await expect.poll(() => archive.calls.filter((call) => call.path === '/documents').at(-1)?.query.has('date_from')).toBe(false)
  expect(archive.calls.filter((call) => call.path === '/documents').at(-1)?.query.has('date_to')).toBe(false)
})

test('restores filter-only searches, paginates, and surfaces upstream errors', async ({ page }) => {
  const archive = await mockArchive(page)
  await page.goto('/?tags=synthetic&docTypes=invoice&dateFrom=2026-01-01&hasBarcode=false')
  await expect(page.getByText('Synthetic first document', { exact: true })).toBeVisible()
  const request = archive.calls.find((call) => call.path === '/search')
  expect(request?.body).toMatchObject({
    searchTerm: '', tags: ['synthetic'], docTypes: ['invoice'],
    dateFrom: '2026-01-01', hasBarcode: false
  })
  await page.getByRole('button', { name: 'Load more documents', exact: true }).click()
  await expect(page.getByText('Synthetic second document', { exact: true })).toBeVisible()
  expect(archive.calls.some((call) => call.body?.scrollId === 'synthetic-scroll')).toBe(true)
  archive.failSearch()
  await page.goto('/?tags=synthetic')
  await expect(page.getByRole('alert').filter({ hasText: 'Request failed: 503' })).toBeVisible()
  await expect(page.getByRole('button', { name: 'Try again', exact: true })).toBeVisible()
})
