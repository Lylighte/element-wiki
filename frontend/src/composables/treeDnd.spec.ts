// T8.4 验收：拖拽落点判定、移动计划（含自嵌套拦截）与 moveNode 编排。
import { describe, expect, it, beforeEach, vi } from 'vitest'
import { pickDropPos, planMove, findNode, type DropPos } from './treeDnd'
import type { TreeNode } from '@/api'

vi.mock('@/api', () => ({
  docApi: {
    tree: vi.fn().mockResolvedValue({ nodes: [] }),
    patch: vi.fn().mockResolvedValue({}),
    reorder: vi.fn().mockResolvedValue(undefined),
  },
}))

import { docApi } from '@/api'
import treeStore from '@/stores/tree'

function node(id: string, parent_id: string | null, children: TreeNode[] = []): TreeNode {
  return { id, parent_id, title: 'T-' + id, slug: id, sort_key: 100, restricted: false, children }
}

const tree = (): TreeNode[] => [
  node('root', null, [
    node('a', 'root'),
    node('b', 'root', [node('b1', 'b')]),
    node('c', 'root'),
  ]),
]

describe('pickDropPos', () => {
  const rect = { top: 100, height: 20 }

  it.each([
    [105, 'before'],
    [110, 'inside'],
    [116, 'after'],
  ] as [number, DropPos][])('%d → %s', (y, want) => {
    expect(pickDropPos(y, rect)).toBe(want)
  })
})

describe('planMove', () => {
  it('同层排序（父为 root，列表不含根）', () => {
    expect(planMove(tree(), 'a', 'c', 'before')).toEqual({
      parent_id: 'root',
      ordered_ids: ['b', 'a', 'c'],
    })
    expect(planMove(tree(), 'a', 'c', 'after')).toEqual({
      parent_id: 'root',
      ordered_ids: ['b', 'c', 'a'],
    })
  })

  it('跨父移入：拖 a 进 b 内部，追加为末子', () => {
    expect(planMove(tree(), 'a', 'b', 'inside')).toEqual({
      parent_id: 'b',
      ordered_ids: ['b1', 'a'],
    })
  })

  it('移入自身子树被拦截：拖 root 到 b1', () => {
    expect(planMove(tree(), 'root', 'b1', 'inside')).toBeNull()
  })

  it('拖到自身行无操作', () => {
    expect(planMove(tree(), 'a', 'a', 'before')).toBeNull()
  })

  it('同层已处于目标位次返回 null：a 在 b 前', () => {
    expect(planMove(tree(), 'a', 'b', 'before')).toBeNull()
  })

  it('before 插到目标之前', () => {
    expect(planMove(tree(), 'c', 'a', 'before')!.ordered_ids).toEqual(['c', 'a', 'b'])
  })
})

describe('moveNode 编排', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    treeStore.state.nodes = []
  })

  it('跨父移动先 patch 再 reorder，并强制刷新树', async () => {
    treeStore.state.nodes = tree()
    const ok = await treeStore.moveNode('a', 'b', 'inside')
    expect(ok).toBe(true)
    expect(docApi.patch).toHaveBeenCalledWith('a', { parent_id: 'b' })
    expect(docApi.reorder).toHaveBeenCalledWith('b', ['b1', 'a'])
    expect(docApi.tree).toHaveBeenCalled()
    // 刷新后本地树已被 mock 的空树覆盖
    expect(treeStore.state.nodes).toEqual([])
  })

  it('同层仅 reorder 不调 patch', async () => {
    treeStore.state.nodes = tree()
    await treeStore.moveNode('a', 'c', 'before')
    expect(docApi.patch).not.toHaveBeenCalled()
    expect(docApi.reorder).toHaveBeenCalledWith('root', ['b', 'a', 'c'])
  })

  it('服务端拒绝 → 返回 false 且不刷新', async () => {
    treeStore.state.nodes = tree()
    ;(docApi.patch as ReturnType<typeof vi.fn>).mockRejectedValueOnce(new Error('422'))
    const ok = await treeStore.moveNode('a', 'b', 'inside')
    expect(ok).toBe(false)
    expect(docApi.reorder).not.toHaveBeenCalled()
    expect(treeStore.state.nodes.length).toBe(1) // 未刷新
  })

  it('findNode 深度查找', () => {
    expect(findNode(tree(), 'b1')?.id).toBe('b1')
    expect(findNode(tree(), 'nope')).toBeNull()
  })
})
