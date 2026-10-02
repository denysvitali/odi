import { expect, test, type Page } from '@playwright/test'

async function archive(page: Page) {
  const calls: Record<string, unknown>[] = []
  await page.route('**/settings.json', (route) =>
    route.fulfill({ json: { apiUrl: 'http://127.0.0.1:4173/api/v1', opensearchUrl: '' } })
  )
  await page.route('**/api/v1/**', (route) => {
    const path = new URL(route.request().url()).pathname
    if (path.endsWith('/reminders')) return route.fulfill({ json: { reminders: [], days: 90 } })
    if (path.endsWith('/search/facets'))
      return route.fulfill({
        json: {
          hits: { total: { value: 1 } },
          aggregations: {
            companies: { buckets: [{ key: 'Sample company', doc_count: 1 }] },
            barcode_count: { doc_count: 0 }
          }
        }
      })
    if (path.endsWith('/documents') || path.endsWith('/search')) {
      if (path.endsWith('/search')) calls.push(route.request().postDataJSON())
      return route.fulfill({
        json: {
          hits: {
            hits: [
              {
                _id: 'synthetic_0',
                _source: { title: 'Synthetic document', text: 'Synthetic fixture' }
              }
            ],
            total: { value: 1, relation: 'eq' }
          }
        }
      })
    }
    if (path.endsWith('/documents/older_0'))
      return route.fulfill({
        json: {
          id: 'older_0',
          title: 'Older favorite',
          text: 'Synthetic archived fixture',
          primaryDate: '2020-01-01'
        }
      })
    return route.fulfill({ status: 404, json: { error: 'Synthetic media unavailable' } })
  })
  return calls
}

test('mobile search filters trap focus, apply changes, dismiss with Escape and restore focus', async ({
  page
}, info) => {
  test.skip(info.project.name !== 'mobile', 'Mobile drawer behavior')
  const calls = await archive(page)
  await page.goto('/?q=synthetic')
  const trigger = page.getByRole('button', { name: 'Open search filters' })
  await trigger.click()
  const dialog = page.getByRole('dialog', { name: 'Search filters' })
  await expect(dialog).toBeVisible()
  await expect(dialog.getByLabel('From', { exact: true })).toBeVisible()
  await dialog.getByLabel('Sample company', { exact: false }).check()
  await expect.poll(() => calls.at(-1)?.companies).toEqual(['Sample company'])
  await dialog.getByLabel('Barcode presence').selectOption('false')
  await expect.poll(() => calls.at(-1)?.hasBarcode).toBe(false)
  await dialog.getByRole('button', { name: 'Close', exact: true }).focus()
  await page.keyboard.press('Tab')
  expect(await dialog.evaluate((element) => element.contains(document.activeElement))).toBe(true)
  await page.keyboard.press('Escape')
  await expect(dialog).toHaveCount(0)
  await expect(trigger).toBeFocused()
  expect(
    await page
      .locator('body')
      .evaluate(() =>
        Array.from(document.querySelectorAll('input')).some(
          (element) => element.getBoundingClientRect().right < 0
        )
      )
  ).toBe(false)
  await trigger.click()
  await page.setViewportSize({ width: 1280, height: 900 })
  await expect(dialog).toHaveCount(0)
})

test('keyboard help traps focus and dismisses without leaving modal state', async ({
  page
}, info) => {
  await archive(page)
  await page.goto('/')
  if (info.project.name === 'mobile') {
    await page.getByRole('button', { name: 'Open navigation' }).click()
    await page.getByRole('dialog').getByRole('button', { name: 'Keyboard shortcuts' }).click()
  } else await page.getByRole('button', { name: /^Keyboard shortcuts/ }).click()
  const dialog = page.getByRole('dialog', { name: 'Keyboard Shortcuts' })
  await expect(dialog).toBeVisible()
  await expect(dialog.getByRole('button', { name: 'Close keyboard shortcuts' })).toBeFocused()
  await page.keyboard.press('Tab')
  expect(await dialog.evaluate((element) => element.contains(document.activeElement))).toBe(true)
  await page.keyboard.press('Escape')
  await expect(dialog).toHaveCount(0)
  await page.getByRole('searchbox', { name: 'Search documents' }).fill('synthetic')
})

test('favorites resolve saved documents outside the first archive listing', async ({ page }) => {
  await archive(page)
  await page.addInitScript(() => localStorage.setItem('odi-favorites', JSON.stringify(['older_0'])))
  await page.goto('/favorites')
  await expect(page.getByText('Older favorite', { exact: true })).toBeVisible()
  await expect(page.getByText('No favorites yet', { exact: true })).toHaveCount(0)
})

test('saved search names can be edited without changing their filters', async ({ page }) => {
  const calls = await archive(page)
  await page.addInitScript(() =>
    localStorage.setItem(
      'odi-saved-searches',
      JSON.stringify([
        {
          id: 'synthetic-saved',
          name: 'Original',
          term: '',
          filters: { hasBarcode: false, companies: ['Sample company'] }
        }
      ])
    )
  )
  await page.goto('/')
  await page.getByRole('button', { name: 'Rename saved search Original' }).click()
  await page.getByLabel('Search name').fill('Renamed search')
  await page.getByRole('button', { name: 'Save name' }).click()
  await expect(
    page.getByRole('button', { name: 'Rename saved search Renamed search' })
  ).toBeFocused()
  await page.getByRole('button', { name: 'Renamed search', exact: true }).click()
  await expect.poll(() => calls.at(-1)?.hasBarcode).toBe(false)
  expect(calls.at(-1)?.companies).toEqual(['Sample company'])
})
