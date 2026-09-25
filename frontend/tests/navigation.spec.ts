import { test, expect } from '@playwright/test'

// Public-page browser smoke stays independent of a running backend.
test.beforeEach(async ({ page }) => {
  await page.route('**/v1/**', async (route) => {
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

test('document print view keeps content and hides navigation and controls', async ({ page }) => {
  await page.route('**/v1/documents/resolve?*', async (route) => {
    await route.fulfill({ json: {
      document: { id: 'd1', slug: 'demo', title: 'Demo', parent_id: null },
      render: { html: '<h1>Demo</h1><p>Printable body</p>', title: 'Demo', toc: [] },
    } })
  })
  await page.goto('/docs/demo')
  await expect(page.locator('[data-test="doc-html"]')).toContainText('Printable body')
  await page.emulateMedia({ media: 'print' })
  await page.evaluate(() => document.documentElement.classList.add('dark'))
  await expect(page.locator('[data-test="doc-html"]')).toBeVisible()
  await expect(page.locator('#app > div > header')).toBeHidden()
  await expect(page.locator('[data-test="btn-print"]')).toBeHidden()
  await expect.poll(() => page.locator('body').evaluate((el) => getComputedStyle(el).backgroundColor)).toBe('rgb(255, 255, 255)')
})
