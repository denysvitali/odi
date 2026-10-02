import { expect, test, type Page } from '@playwright/test'

const docs = [
  {
    _id: 'synthetic_0',
    _source: {
      title: 'Synthetic invoice',
      text: 'Synthetic fixture',
      docType: 'invoice',
      date: '2026-10-01'
    }
  },
  {
    _id: 'synthetic_1',
    _source: {
      title: 'Synthetic contract',
      text: 'Synthetic fixture',
      docType: 'contract',
      date: '2026-09-01'
    }
  }
]
async function setup(page: Page, fail = false) {
  const queries: Record<string, unknown>[] = []
  await page.route('**/settings.json', (route) =>
    route.fulfill({ json: { apiUrl: 'http://127.0.0.1:4173/api/v1', opensearchUrl: '' } })
  )
  await page.route('**/api/v1/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    if (path.endsWith('/reminders')) return route.fulfill({ json: { reminders: [], days: 90 } })
    if (path.endsWith('/search/facets'))
      return route.fulfill({ json: { hits: { total: { value: 2 } }, aggregations: {} } })
    if (path.endsWith('/search')) {
      queries.push(route.request().postDataJSON())
      if (fail) return route.fulfill({ status: 503, json: { error: 'Synthetic failure' } })
    }
    if (path.endsWith('/documents') || path.endsWith('/search'))
      return route.fulfill({ json: { hits: { hits: docs, total: { value: 2, relation: 'eq' } } } })
    return route.fulfill({ status: 404, json: { error: 'No synthetic media' } })
  })
  return queries
}

test('saves a filter-only search, restores it after reload, and removes it', async ({ page }) => {
  const queries = await setup(page)
  await page.goto('/')
  await page.getByRole('link', { name: 'Invoices Bills & payments' }).click()
  await expect(page.getByRole('heading', { name: '2 results' })).toBeVisible()
  expect(queries.at(-1)).toMatchObject({ searchTerm: '', docTypes: ['invoice'] })
  await page.getByRole('button', { name: 'Save search', exact: true }).click()
  await page.getByLabel('Search name').fill('Saved invoices')
  await page.getByRole('button', { name: 'Save on this device' }).click()
  await expect(
    page.getByRole('status').filter({ hasText: 'Search saved on this device.' })
  ).toBeVisible()
  await page.getByRole('button', { name: 'Clear search', exact: true }).click()
  await page.reload()
  await page.getByRole('button', { name: 'Saved invoices', exact: true }).click()
  await expect(page.getByRole('heading', { name: '2 results' })).toBeVisible()
  expect(queries.at(-1)).toMatchObject({ searchTerm: '', docTypes: ['invoice'] })
  await page.getByRole('button', { name: 'Clear search', exact: true }).click()
  await page.getByRole('button', { name: 'Remove saved search Saved invoices' }).click()
  await expect(page.getByRole('button', { name: 'Saved invoices', exact: true })).toHaveCount(0)
})

test('supports list sorting, grid switching, navigation, dark mode, and small screens', async ({
  page
}, testInfo) => {
  await setup(page)
  const errors: string[] = []
  page.on('pageerror', (error) => errors.push(error.message))
  await page.goto('/')
  await expect(page.getByText('Synthetic invoice', { exact: true })).toBeVisible()
  await page.getByLabel('Sort loaded documents').selectOption('title')
  await expect(page.locator('.document-list-title').first()).toHaveText('Synthetic contract')
  await page.getByRole('button', { name: 'Grid view', exact: true }).click()
  await expect(page.getByRole('button', { name: 'Grid view', exact: true })).toHaveAttribute(
    'aria-pressed',
    'true'
  )
  await page.getByRole('button', { name: 'List view', exact: true }).click()
  await page.getByRole('button', { name: 'Switch to dark mode' }).click()
  await expect(page.locator('html')).toHaveClass(/dark/)
  await page.emulateMedia({ colorScheme: 'dark' })
  await page.getByRole('button', { name: 'Switch to light mode' }).click()
  await expect(page.locator('html')).not.toHaveClass(/dark/)
  expect(await page.locator('body').evaluate((el) => getComputedStyle(el).backgroundColor)).toBe(
    'rgb(250, 251, 248)'
  )
  if (testInfo.project.name === 'mobile') {
    await page.getByRole('button', { name: 'Open navigation' }).click()
    await page.getByRole('dialog').getByRole('link', { name: 'All documents' }).click()
    await expect(page.getByRole('dialog')).toHaveCount(0)
    await expect(page).toHaveURL(/\/documents$/)
    await page.setViewportSize({ width: 320, height: 740 })
  } else {
    await page
      .getByRole('navigation', { name: 'Primary', exact: true })
      .getByRole('link', { name: 'All documents' })
      .click()
    await expect(page).toHaveURL(/\/documents$/)
  }
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true)
  expect(errors).toEqual([])
})

test('renders a retryable search failure without a false empty state', async ({ page }) => {
  await setup(page, true)
  await page.goto('/?q=synthetic')
  await expect(
    page.getByRole('alert').filter({ hasText: 'Search could not be loaded' })
  ).toBeVisible()
  await expect(page.getByRole('button', { name: 'Try again', exact: true })).toBeVisible()
  await expect(page.getByText('No documents found', { exact: true })).toHaveCount(0)
})
