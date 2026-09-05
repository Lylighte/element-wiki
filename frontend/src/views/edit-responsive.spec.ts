// M15 验收：编辑页预览响应式——桌面（jsdom 默认回退）保持分栏；
// 窄屏编辑器与预览二选一互斥全宽，preview-toggle 切换。
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import i18n from '@/i18n'
import ElementPlus from 'element-plus'

vi.mock('@/api', () => ({
  docApi: {
    resolve: vi.fn().mockResolvedValue({
      document: { id: 'd1', title: 'T', parent_id: null },
      render: { html: '', title: 'T', toc: [] },
    }),
    getDraft: vi.fn().mockResolvedValue({ draft: null }),
    getCommitContent: vi.fn().mockResolvedValue({ content: 'head content' }),
    tree: vi.fn().mockResolvedValue({ nodes: [] }),
    saveDraft: vi.fn().mockResolvedValue(undefined),
    deleteDraft: vi.fn().mockResolvedValue(undefined),
    patch: vi.fn().mockResolvedValue({}),
    commit: vi.fn().mockResolvedValue({ commit: { id: 'c1' }, dead_links: [] }),
    listCommits: vi.fn().mockResolvedValue({ items: [{ id: 'head1', commit_no: 1, message: '', created_at: 0 }] }),
    preview: vi.fn().mockResolvedValue({ html: '<p>pv</p>' }),
  },
  attachmentApi: {
    upload: vi.fn().mockResolvedValue({ id: 'x' }),
    rawURL: (id: string) => `/v1/attachments/${id}/raw`,
  },
}))

import EditView from './EditView.vue'

function stubNarrow() {
  vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({
    matches: false,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  }))
}

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<div />' } },
      { path: '/docs/:pathMatch(.*)*', component: { template: '<div data-test="doc-page" />' } },
      { path: '/docs/:pathMatch(.*)*/edit', component: EditView, props: true },
    ],
  })
}

async function mountEdit(path = '/docs/d1/edit', narrow = false) {
  if (narrow) stubNarrow()
  const router = makeRouter()
  await router.push(path)
  await router.isReady()
  const app = mount(
    { template: '<router-view />', setup: () => ({}) },
    { global: { plugins: [i18n, ElementPlus, router] }, attachTo: document.body },
  )
  for (let i = 0; i < 30 && !app.find('[data-test="preview-toggle"]').exists(); i++) {
    await new Promise((r) => setTimeout(r, 10))
  }
  for (let i = 0; i < 30 && !app.find('[data-test="editor-canvas"]').exists(); i++) {
    await new Promise((r) => setTimeout(r, 10))
  }
  return app
}

describe('edit preview responsive (M15)', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
  })

  it('桌面默认：编辑器与预览并存（分栏）', async () => {
    const app = await mountEdit()
    const editor = app.find('[data-test="editor-canvas"]')
    expect(editor.exists()).toBe(true)
    expect((editor.element as HTMLElement).style.display).toBe('')
    expect(app.find('[data-test="preview-pane"]').exists()).toBe(true)
    app.unmount()
  })

  it('窄屏默认开启预览：编辑器隐藏、预览全宽', async () => {
    const app = await mountEdit('/docs/d1/edit', true)
    const editor = app.find('[data-test="editor-canvas"]')
    expect(editor.exists()).toBe(true)
    expect((editor.element as HTMLElement).style.display).toBe('none')
    const pane = app.find('[data-test="preview-pane"]')
    expect(pane.exists()).toBe(true)
    expect(pane.classes()).toContain('w-full')
    app.unmount()
  })

  it('窄屏点预览开关：回到编辑器、预览消失', async () => {
    const app = await mountEdit('/docs/d1/edit', true)
    await app.find('[data-test="preview-toggle"]').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(app.find('[data-test="preview-pane"]').exists()).toBe(false)
    const editor = app.find('[data-test="editor-canvas"]')
    expect((editor.element as HTMLElement).style.display).toBe('')
    app.unmount()
  })
})
