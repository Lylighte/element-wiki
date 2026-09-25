import { defineConfig, devices } from '@playwright/test'
import { mkdtempSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const frontendDir = dirname(fileURLToPath(import.meta.url))
const projectDir = dirname(frontendDir)
const runtimeDir = mkdtempSync(join(tmpdir(), 'element-wiki-e2e-'))
const storageDir = join(runtimeDir, 'storage')

export default defineConfig({
  testDir: './tests/authenticated',
  fullyParallel: false,
  workers: 1,
  timeout: 45_000,
  use: {
    baseURL: 'http://127.0.0.1:5175',
    ...devices['Desktop Chrome'],
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  webServer: [
    {
      command: 'node tests/support/test-idp.mjs',
      cwd: frontendDir,
      url: 'http://127.0.0.1:18081/healthz',
      reuseExistingServer: false,
      timeout: 30_000,
    },
    {
      command: 'go run ./cmd/wikid -configfile frontend/tests/support/wiki-test.yaml',
      cwd: projectDir,
      env: {
        WIKI_DATABASE_URL: join(runtimeDir, 'wiki.db'),
        WIKI_STORAGE_DIR: storageDir,
        WIKI_SEARCH_INDEX_DIR: join(storageDir, 'search', 'documents.bleve'),
      },
      url: 'http://127.0.0.1:18080/healthz',
      reuseExistingServer: false,
      timeout: 120_000,
    },
    {
      command: 'npm run dev -- --host 127.0.0.1',
      cwd: frontendDir,
      env: { WIKI_DEV_API_TARGET: 'http://127.0.0.1:18080' },
      url: 'http://127.0.0.1:5175',
      reuseExistingServer: false,
      timeout: 120_000,
    },
  ],
})
