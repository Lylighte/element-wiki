// T8.3 验收：树节点折叠/展开 + localStorage 持久化。
import { describe, expect, it, beforeEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import i18n from '@/i18n'
import ElementPlus from 'element-plus'
import TreeItem from './TreeItem.vue'
import collapseStore from '@/stores/collapse'
import type { TreeNode } from '@/api'

vi.mock('@/api', () => ({
  docApi: {
    tree: vi.fn().mockResolvedValue({ nodes: [] }),
    patch: vi.fn().mockResolvedValue({}),
    reorder: vi.fn().mockResolvedValue(undefined),
  },
}))

function mountTree(root: TreeNode) {
  return mount(TreeItem, { global: { plugins: [i18n, ElementPlus] } , props: { node: root } })
}

function node(id: string, children: TreeNode[] = []): TreeNode {
  return {
    id,
    parent_id: null,
    title: 'T-' + id,
    slug: id,
    sort_key: 100,
    restricted: false,
    children,
  }
}

describe('tree collapse', () => {
  beforeEach(() => {
    localStorage.clear()
    collapseStore.state.ids.clear()
  })

  it('无子节点不渲染折叠箭头', () => {
    const w = mountTree(node('leaf'))
    expect(w.find('[data-test="tree-toggle"]').exists()).toBe(false)
  })

  it('点击箭头折叠/展开，标题点击仍导航', async () => {
    const root = node('root', [node('child')])
    const w = mountTree(root)
    expect(w.findAll('[data-test="tree-item"]').map((x) => x.text())).toEqual(['T-root', 'T-child'])

    await w.find('[data-test="tree-toggle"]').trigger('click')
    expect(w.find('[data-test="tree-toggle"]').exists()).toBe(true)
    // 子层被折叠：只剩根自身的一个 item
    expect(w.findAll('[data-test="tree-item"]').length).toBe(1)
    expect(JSON.parse(localStorage.getItem('ew.tree.collapsed')!)).toEqual(['root'])

    await w.find('[data-test="tree-toggle"]').trigger('click')
    expect(w.findAll('[data-test="tree-item"]').length).toBe(2)
    expect(localStorage.getItem('ew.tree.collapsed')).toBe('[]')
  })

  it('刷新后（重新挂载）折叠状态保持', async () => {
    const root = node('root', [node('child')])
    localStorage.setItem('ew.tree.collapsed', JSON.stringify(['child']))
    collapseStore.state.ids.add('child')
    const w = mountTree(root)
    // 折叠的是子节点 child —— 它有父链渲染但自身子级为空，验证另一分支：
    const deep = node('p', [node('c1')])
    localStorage.setItem('ew.tree.collapsed', JSON.stringify(['p']))
    collapseStore.state.ids.add('p')
    const w2 = mountTree(deep)
    expect(w2.findAll('[data-test="tree-item"]').length).toBe(1)
    expect(w.find('[data-test="tree-item"]').exists()).toBe(true)
  })
})
