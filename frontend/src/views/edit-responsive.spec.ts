// 编辑页预览响应式：宽屏保持分栏；窄屏默认显示源码，preview-toggle 切换预览。
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
    get: vi.fn().mockResolvedValue({ document: { id: 'd1', title: 'T', parent_id: null } }),
  },
  attachmentApi: {
    upload: vi.fn().mockResolvedValue({ id: 'x' }),
    rawURL: (id: string) => `/v1/attachments/${id}/raw`,
  },
}))

import EditView from './EditView.vue'
import { docApi } from '@/api'

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
    vi.unstubAllGlobals()
    document.body.innerHTML = ''
  })

  it('桌面默认：编辑器与预览并存（分栏）', async () => {
    const app = await mountEdit()
    const editor = app.find('[data-test="editor-canvas"]')
    expect(editor.exists()).toBe(true)
    expect((editor.element as HTMLElement).style.display).toBe('')
    const pane = app.find('[data-test="preview-pane"]')
    expect(pane.exists()).toBe(true)
    // 进入页面即渲染预览，非空白（loadDoc 首渲染）
    expect(pane.text()).toContain('pv')
    app.unmount()
  })

  it('窄屏默认显示源码，预览关闭', async () => {
    const app = await mountEdit('/docs/d1/edit', true)
    const editor = app.find('[data-test="editor-canvas"]')
    expect(editor.exists()).toBe(true)
    expect((editor.element as HTMLElement).style.display).toBe('')
    expect(app.find('[data-test="preview-pane"]').exists()).toBe(false)
    expect(app.find('[data-test="preview-toggle"]').attributes('aria-pressed')).toBe('false')
    app.unmount()
  })

  it('窄屏点预览开关：显示全宽预览并隐藏源码，再点回到源码', async () => {
    const app = await mountEdit('/docs/d1/edit', true)
    await app.find('[data-test="preview-toggle"]').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    const pane = app.find('[data-test="preview-pane"]')
    expect(pane.exists()).toBe(true)
    expect(pane.classes()).toContain('w-full')
    const editor = app.find('[data-test="editor-canvas"]')
    expect((editor.element as HTMLElement).style.display).toBe('none')
    expect(app.find('[data-test="preview-toggle"]').attributes('aria-pressed')).toBe('true')
    await app.find('[data-test="preview-toggle"]').trigger('click')
    expect(app.find('[data-test="preview-pane"]').exists()).toBe(false)
    expect((editor.element as HTMLElement).style.display).toBe('')
    app.unmount()
  })

  it('预览失败会标明内容可能过期并支持重试', async () => {
    vi.mocked(docApi.preview).mockRejectedValueOnce(new Error('offline'))
    const app = await mountEdit()
    await new Promise((r) => setTimeout(r, 0))
    expect(app.find('[data-test="preview-error"]').exists()).toBe(true)
    await app.find('[data-test="preview-error"] button').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(app.find('[data-test="preview-error"]').exists()).toBe(false)
    app.unmount()
  })

  it('文档树暂不可用时仍可编辑正文', async () => {
    vi.mocked(docApi.tree).mockRejectedValueOnce(new Error('offline'))
    const app = await mountEdit()
    expect(app.find('[data-test="editor-canvas"]').exists()).toBe(true)
    expect(app.find('[data-test="edit-load-error"]').exists()).toBe(false)
    app.unmount()
  })

  it('可见性选择器显示生效可见性（T16.7）', async () => {
    vi.mocked(docApi.resolve).mockResolvedValueOnce({
      document: { id: 'd1', title: 'T', parent_id: null, effective_visibility: 'restricted' },
      render: { html: '', title: 'T', toc: [] },
    } as never)
    const app = await mountEdit('/docs/d1/edit', true)
    const sel = app.find('[data-test="visibility-select"]')
    expect(sel.exists()).toBe(true)
    expect((sel.element as HTMLSelectElement).value).toBe('restricted')
    app.unmount()
  })

  it('切换可见性 → PATCH visibility 并以生效值回显（T16.7）', async () => {
    const app = await mountEdit()
    ;(docApi.patch as ReturnType<typeof vi.fn>).mockClear()
    vi.mocked(docApi.get).mockClear()
    vi.mocked(docApi.get).mockResolvedValueOnce({ document: { id: 'd1', effective_visibility: 'restricted' } } as never)
    const sel = app.find('[data-test="visibility-select"]')
    await sel.setValue('restricted')
    await new Promise((r) => setTimeout(r, 0))
    expect(docApi.patch).toHaveBeenCalledWith('d1', { visibility: 'restricted' })
    expect(docApi.get).toHaveBeenCalledWith('d1')
    expect((sel.element as HTMLSelectElement).value).toBe('restricted')
    app.unmount()
  })

  it('可见性 PATCH 失败 → 提示并回显生效值（T16.7）', async () => {
    const app = await mountEdit('/docs/d1/edit', true)
    ;(docApi.patch as ReturnType<typeof vi.fn>).mockRejectedValueOnce(new Error('403'))
    vi.mocked(docApi.get).mockClear()
    vi.mocked(docApi.get).mockResolvedValueOnce({ document: { id: 'd1', effective_visibility: 'standard' } } as never)
    const sel = app.find('[data-test="visibility-select"]')
    await sel.setValue('restricted')
    await new Promise((r) => setTimeout(r, 0))
    expect((sel.element as HTMLSelectElement).value).toBe('standard')
    app.unmount()
  })
})
