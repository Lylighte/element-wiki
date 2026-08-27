// T9.5 验收：脏状态路由离开弹确认（拒绝→留守；确认→落盘放行）；干净状态直接放行。
import { describe, expect, it, vi, beforeEach, beforeAll } from 'vitest'
import { mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import i18n from '@/i18n'
import ElementPlus, { ElMessageBox } from 'element-plus'

// jsdom 缺少布局 API：真实 Tiptap/ProseMirror 需要
beforeAll(() => {
  const fakeRect = () =>
    ({ top: 0, right: 0, bottom: 0, left: 0, width: 0, height: 0, x: 0, y: 0, toJSON() { return this } }) as DOMRect
  const proto = Element.prototype as unknown as Record<string, unknown>
  if (!proto.getClientRects) proto.getClientRects = function () { return [fakeRect()] }
  if (!proto.getBoundingClientRect) proto.getBoundingClientRect = fakeRect
})

vi.mock('@/api', () => ({
  docApi: {
    get: vi.fn().mockResolvedValue({ document: { id: 'd1', title: 'T', parent_id: null } }),
    getDraft: vi.fn().mockResolvedValue({ draft: null }),
    getCommitContent: vi.fn().mockResolvedValue({ content: 'head content' }),
    tree: vi.fn().mockResolvedValue({ nodes: [] }),
    saveDraft: vi.fn().mockResolvedValue(undefined),
    patch: vi.fn().mockResolvedValue({}),
    commit: vi.fn().mockResolvedValue({ commit: { id: 'c1' }, dead_links: [] }),
    listCommits: vi.fn().mockResolvedValue({ items: [{ id: 'head1', commit_no: 1, message: '', created_at: 0 }] }),
  },
  attachmentApi: {
    upload: vi.fn().mockResolvedValue({ id: 'x' }),
    rawURL: (id: string) => `/v1/attachments/${id}/raw`,
  },
}))




import { docApi } from '@/api'

import EditView from './EditView.vue'

function makeApp() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<div data-test="home" />' } },
      { path: '/docs/:id/edit', component: EditView, props: true },
      { path: '/other', component: { template: '<div data-test="other" />' } },
    ],
  })
  return router
}

async function mountEdit() {
  const router = makeApp()
  await router.push('/docs/d1/edit')
  await router.isReady()
  const app = mount(
    { template: '<router-view />', setup: () => ({}) },
    { global: { plugins: [i18n, ElementPlus, router] }, attachTo: document.body },
  )
  await new Promise((r) => setTimeout(r, 0))
  let input = app.find('input')
  for (let i = 0; i < 30 && !input.exists(); i++) {
    await new Promise((r) => setTimeout(r, 10))
    input = app.find('input')
  }
  return { app, router, input }
}

describe('leave confirmation (ED-09)', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    ;(docApi.tree as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({ nodes: [] })
  })

  it('脏状态 + 确认 → 落盘草稿并放行导航', async () => {
    const confirmSpy = vi.spyOn(ElMessageBox, 'confirm').mockResolvedValue('confirm' as never)
    const { app, router, input } = await mountEdit()
    await input.setValue('Changed Title')
    await new Promise((r) => setTimeout(r, 0))
    await router.push('/other')
    await new Promise((r) => setTimeout(r, 0))
    expect(confirmSpy).toHaveBeenCalledTimes(1)
    expect(docApi.patch).toHaveBeenCalledWith('d1', { title: 'Changed Title' })
    expect(router.currentRoute.value.path).toBe('/other')
    app.unmount()
  })

  it('脏状态 + 取消 → 留守本页', async () => {
    vi.spyOn(ElMessageBox, 'confirm').mockRejectedValue('cancel' as never)
    const { app, router, input } = await mountEdit()
    await input.setValue('Changed Title')
    await new Promise((r) => setTimeout(r, 0))
    await router.push('/other')
    await new Promise((r) => setTimeout(r, 0))
    expect(router.currentRoute.value.path).toBe('/docs/d1/edit')
    app.unmount()
  })

  it('干净状态 → 不弹确认直接离开', async () => {
    const confirmSpy = vi.spyOn(ElMessageBox, 'confirm').mockResolvedValue('confirm' as never)
    const { app, router } = await mountEdit()
    await new Promise((r) => setTimeout(r, 0))
    await router.push('/other')
    await new Promise((r) => setTimeout(r, 0))
    expect(confirmSpy).not.toHaveBeenCalled()
    expect(router.currentRoute.value.path).toBe('/other')
    app.unmount()
  })

  it('同一路由记录切换文档 → 重新加载目标文档', async () => {
    const { app, router } = await mountEdit()
    vi.mocked(docApi.get).mockClear()
    await router.push('/docs/d2/edit')
    await new Promise((r) => setTimeout(r, 0))
    expect(docApi.get).toHaveBeenCalledWith('d2')
    app.unmount()
  })
})
