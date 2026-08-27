import { test, expect } from '@playwright/test'

test('unknown route renders Not Found instead of a blank view', async ({ page }) => {
  await page.goto('/does-not-exist')
  await expect(page.locator('[data-test="not-found-page"]')).toBeVisible()
})

test('login route renders a usable page', async ({ page }) => {
  await page.goto('/login')
  await expect(page.locator('[data-test="login-page"]')).toBeVisible()
})
