// T16.3 验收：浏览侧文档树纯净化——无可拖拽行、无右键菜单/结构编辑入口，
// 点击行导航到 slug 路径；结构编辑能力在管理后台 TreeAdminPanel（另测）。
import { describe, expect, it, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import i18n from '@/i18n'
import ElementPlus from 'element-plus'
import SideTree from './SideTree.vue'
import treeStore from '@/stores/tree'
import type { TreeNode } from '@/api'

vi.mock('@/api', () => ({
  docApi: {
    tree: vi.fn().mockResolvedValue({ nodes: [] }),
    patch: vi.fn().mockResolvedValue({}),
    remove: vi.fn().mockResolvedValue(undefined),
    create: vi.fn().mockResolvedValue({ document: { id: 'new1' } }),
    reorder: vi.fn().mockResolvedValue(undefined),
  },
}))

import { docApi } from '@/api'

function node(id: string, children: TreeNode[] = []): TreeNode {
  return { id, parent_id: null, title: 'T-' + id, slug: id, sort_key: 100, restricted: false, children }
}

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<div />' } },
      { path: '/docs/:pathMatch(.*)*', component: { template: '<div />' } },
      { path: '/docs/:pathMatch(.*)*/edit', component: { template: '<div />' } },
    ],
  })
}

async function mountSide() {
  const router = makeRouter()
  await router.push('/')
  await router.isReady()
  const w = mount(SideTree, { global: { plugins: [i18n, ElementPlus, router] } })
  await new Promise((r) => setTimeout(r, 0))
  return { w, router }
}

describe('browse side tree purity (T16.3)', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    treeStore.state.nodes = []
    treeStore.state.loaded = false
    ;(docApi.tree as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({
      nodes: [node('a', [node('a1')])],
    })
  })

  it('无任何 draggable 行（浏览页禁拖，编辑只在后台）', async () => {
    const { w } = await mountSide()
    expect(w.findAll('[data-test="tree-item"]').length).toBe(2)
    expect(w.html()).not.toContain('draggable="true"')
    w.unmount()
  })

  it('右键不弹菜单、不发起任何 API', async () => {
    const { w } = await mountSide()
    await w.findAll('[data-test="tree-item"]')[0].trigger('contextmenu', { clientX: 10, clientY: 20 })
    await new Promise((r) => setTimeout(r, 0))
    expect(w.find('[data-test="tree-menu"]').exists()).toBe(false)
    expect(w.find('[data-test="tree-rename-input"]').exists()).toBe(false)
    expect(docApi.patch).not.toHaveBeenCalled()
    expect(docApi.remove).not.toHaveBeenCalled()
    w.unmount()
  })

  it('点击行导航到 slug 路径', async () => {
    const { w, router } = await mountSide()
    await w.findAll('[data-test="tree-item"]')[1].trigger('click') // a1 → /docs/a/a1
    await new Promise((r) => setTimeout(r, 0))
    expect(router.currentRoute.value.path).toBe('/docs/a/a1')
    w.unmount()
  })
})
