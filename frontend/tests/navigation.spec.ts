import { test, expect } from '@playwright/test'
import { inflateSync } from 'node:zlib'

// Chromium's PDF content stream lets us detect a nearly blank first page.
function firstPageTextOps(pdf: Buffer): number {
  const objects = new Map<number, string>()
  for (const match of pdf.toString('latin1').matchAll(/(\d+) 0 obj\s*([\s\S]*?)\s*endobj/g)) {
    objects.set(Number(match[1]), match[2])
  }
  const firstPage = [...objects.values()].find((value) => /\/Type\s*\/Page\b/.test(value))
  const contentsID = Number(/\/Contents\s+(\d+)\s+0\s+R/.exec(firstPage ?? '')?.[1])
  const streamObject = objects.get(contentsID) ?? ''
  const compressed = /stream\r?\n([\s\S]*?)\r?\nendstream/.exec(streamObject)?.[1]
  if (!compressed) return 0
  const stream = inflateSync(Buffer.from(compressed, 'latin1')).toString('latin1')
  return stream.match(/\bTj\b/g)?.length ?? 0
}

// Public-page browser smoke stays independent of a running backend.
test.beforeEach(async ({ context }) => {
  await context.route('**/v1/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    if (path === '/v1/site') {
      return route.fulfill({ json: { title: 'Element Wiki', default_lang: 'en', timezone: 'Asia/Shanghai', anonymous_read: false, comments_enabled: false } })
    }
    if (path === '/v1/auth/oidc/status') return route.fulfill({ json: { enabled: false } })
    if (path === '/v1/users/me') return route.fulfill({ status: 401, json: { detail: 'unauthenticated' } })
    return route.fulfill({ status: 404, json: { detail: 'not_found' } })
  })
})

test('unknown route renders Not Found instead of a blank view', async ({ page }) => {
  await page.goto('/does-not-exist')
  await expect(page.locator('[data-test="not-found-page"]')).toBeVisible()
})

test('login route renders a usable page', async ({ page }) => {
  await page.goto('/login')
  await expect(page.locator('[data-test="login-page"]')).toBeVisible()
})

test('global search shortcut opens search and focuses its input', async ({ page }) => {
  await page.goto('/login')
  await page.keyboard.press('Control+Shift+F')
  await expect(page).toHaveURL(/\/search$/)
  await expect(page.locator('#global-search-input')).toBeFocused()
})

test('mobile document keeps search visible and actions fit the viewport', async ({ page, context }, testInfo) => {
  await context.route('**/v1/documents/d1/attachments', (route) => route.fulfill({ json: { items: [] } }))
  await context.route('**/v1/documents/resolve?*', (route) => route.fulfill({ json: {
    document: { id: 'd1', slug: 'demo', title: 'Demo', parent_id: null },
    render: { html: '<h1>Demo</h1><p>Readable body</p>', title: 'Demo', toc: [] },
  } }))
  await page.setViewportSize({ width: 375, height: 800 })
  await page.goto('/docs/demo')
  await expect(page.locator('[data-test="doc-html"]')).toContainText('Readable body')
  await expect(page.locator('[data-test="m-search"]')).toBeVisible()
  await expect(page.locator('[data-test="nav-menu"]')).toBeVisible()
  await expect(page.locator('[data-test="site-home"]')).toHaveAttribute('href', '/')
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(375)
  await page.screenshot({ path: testInfo.outputPath('mobile-document.png'), fullPage: true })

  await page.locator('[data-test="btn-more"]').click()
  await expect(page.locator('[data-test="btn-export"]')).toBeVisible()
  await expect(page.locator('[data-test="btn-print"]')).toBeVisible()
  await page.keyboard.press('Escape')
  await page.locator('[data-test="nav-menu"]').click()
  await expect(page.locator('[data-test="m-theme-toggle"]')).toBeVisible()
  await expect(page.locator('[data-test="m-lang-toggle"]')).toBeVisible()
  await page.locator('[data-test="m-search"]').click()
  await expect(page).toHaveURL(/\/search$/)
})

test('print button opens an isolated document whose PDF starts with content', async ({ page, context }) => {
  const paragraphs = `<pre>${Array.from({ length: 120 }, (_, i) =>
    `Long code line ${i + 1}: data`).join('\n')}</pre>` +
    Array.from({ length: 20 }, (_, i) =>
      `<p>Printable paragraph ${i + 1}: This line should flow normally on paper.</p>`).join('')
  await context.route('**/v1/documents/resolve?*', async (route) => {
    await route.fulfill({ json: {
      document: { id: 'd1', slug: 'demo', title: 'Demo', parent_id: null },
      render: { html: `<h1>Demo</h1><p>Printable body</p>${paragraphs}`, title: 'Demo', toc: [] },
    } })
  })
  await page.goto('/docs/demo')
  await expect(page.locator('[data-test="doc-html"]')).toContainText('Printable body')
  await page.locator('[data-test="btn-more"]').click()
  const popupPromise = page.waitForEvent('popup')
  await page.locator('[data-test="btn-print"]').click()
  const popup = await popupPromise
  await expect(popup).toHaveURL(/\/print\/docs\/demo$/)
  await expect(popup.locator('[data-test="print-html"]')).toContainText('Printable body')
  await expect(popup.locator('[data-test="print-now"]')).toBeEnabled()
  await expect(popup.locator('.app-shell')).toHaveCount(0)
  await popup.emulateMedia({ media: 'print' })
  await popup.evaluate(() => document.documentElement.classList.add('dark'))
  await expect(popup.locator('[data-test="print-html"]')).toBeVisible()
  await expect(popup.locator('[data-test="print-now"]')).toBeHidden()
  await expect.poll(() => popup.locator('.print-page').evaluate((el) => getComputedStyle(el).backgroundColor)).toBe('rgb(255, 255, 255)')
  const pdf = await popup.pdf({ format: 'A4', printBackground: true })
  const pageCount = pdf.toString('latin1').match(/\/Type\s*\/Page\b/g)?.length ?? 0
  expect(pageCount).toBeGreaterThan(1)
  expect(firstPageTextOps(pdf)).toBeGreaterThan(10)
})
