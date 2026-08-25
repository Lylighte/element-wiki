// T8.5/T8.6 验收：右键菜单权限显隐、移入回收站、内联重命名、新建子文档预置父级。
import { describe, expect, it, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import i18n from '@/i18n'
import ElementPlus from 'element-plus'
import SideTree from './SideTree.vue'
import treeStore from '@/stores/tree'
import treeMenu from '@/stores/treeMenu'
import collapseStore from '@/stores/collapse'
import { setPermissions } from '@/permissions'
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
      { path: '/docs/:id', component: { template: '<div />' } },
      { path: '/docs/:id/edit', component: { template: '<div />' } },
    ],
  })
}

async function mountSide() {
  const router = makeRouter()
  await router.push('/')
  await router.isReady()
  const w = mount(SideTree, { global: { plugins: [i18n, ElementPlus, router] } })
  await new Promise((r) => setTimeout(r, 0))
  return w
}

const item = (w: ReturnType<typeof mount>, i: number) =>
  w.findAll('[data-test="tree-item"]')[i]

describe('tree context menu', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    localStorage.clear()
    collapseStore.state.ids.clear()
    treeStore.state.nodes = []
    treeStore.state.loaded = false
    treeMenu.state.renamingId = ''
    treeMenu.state.node = null
    setPermissions([])
    ;(docApi.tree as unknown as ReturnType<typeof vi.fn>).mockResolvedValue({
      nodes: [node('a', [node('a1')])],
    })
  })

  async function openMenu(w: ReturnType<typeof mount>) {
    await item(w, 0).trigger('contextmenu', { clientX: 10, clientY: 20 })
    await new Promise((r) => setTimeout(r, 0))
  }

  it('viewer 权限下菜单为空；editor 三项齐全', async () => {
    const w = await mountSide()
    await openMenu(w)
    expect(w.find('[data-test="tree-menu"]').exists()).toBe(true)
    expect(w.find('[data-test="menu-rename"]').exists()).toBe(false)

    setPermissions(['document.update', 'document.create', 'document.delete'])
    await openMenu(w)
    expect(w.find('[data-test="menu-rename"]').exists()).toBe(true)
    expect(w.find('[data-test="menu-new-child"]').exists()).toBe(true)
    expect(w.find('[data-test="menu-trash"]').exists()).toBe(true)
  })

  it('移入回收站：调用 remove 并局部刷新树内容', async () => {
    setPermissions(['document.delete'])
    const w = await mountSide()
    // 刷新返回全新节点列表，验证局部刷新生效
    ;(docApi.tree as unknown as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      nodes: [node('fresh')],
    })
    await openMenu(w)
    await w.find('[data-test="menu-trash"]').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(docApi.remove).toHaveBeenCalledWith('a')
    expect(w.text()).toContain('T-fresh')
  })

  it('内联重命名：菜单触发 → 输入回车 → patch 新标题并刷新', async () => {
    setPermissions(['document.update'])
    const w = await mountSide()
    await openMenu(w)
    await w.find('[data-test="menu-rename"]').trigger('click')
    const input = w.find('[data-test="tree-rename-input"]')
    expect(input.exists()).toBe(true)
    await input.setValue('新标题')
    await input.trigger('keydown.enter')
    await new Promise((r) => setTimeout(r, 0))
    expect(docApi.patch).toHaveBeenCalledWith('a', { title: '新标题' })
    expect(treeMenu.state.renamingId).toBe('')
  })

  it('Esc 取消重命名不发起请求', async () => {
    setPermissions(['document.update'])
    const w = await mountSide()
    await openMenu(w)
    await w.find('[data-test="menu-rename"]').trigger('click')
    const input = w.find('[data-test="tree-rename-input"]')
    await input.setValue('x')
    await input.trigger('keydown.esc')
    await new Promise((r) => setTimeout(r, 0))
    expect(docApi.patch).not.toHaveBeenCalled()
  })

  it('新建子文档：置位 requestCreate 与父级（T8.6 联动）', async () => {
    setPermissions(['document.create'])
    const w = await mountSide()
    // 展开后对子节点 a1 右键
    await item(w, 0).trigger('click')
    await item(w, 1).trigger('contextmenu', { clientX: 10, clientY: 20 })
    await w.find('[data-test="menu-new-child"]').trigger('click')
    expect(treeMenu.state.requestCreate).toBe(true)
    expect(treeMenu.state.createParentId).toBe('a1')
    treeMenu.state.requestCreate = false
  })
})
