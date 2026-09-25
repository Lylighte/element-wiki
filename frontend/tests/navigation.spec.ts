import { test, expect } from '@playwright/test'

// Public-page browser smoke stays independent of a running backend.
test.beforeEach(async ({ page }) => {
  await page.route('**/v1/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    if (path === '/v1/site') {
      return route.fulfill({ json: { title: 'Element Wiki', default_lang: 'en', anonymous_read: false, comments_enabled: false } })
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
