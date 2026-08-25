import type { TreeNode } from '@/api'

// 树拖拽纯逻辑（T8.4）：落点判定与移动计划计算，DOM 事件由 TreeItem 接线。
export type DropPos = 'before' | 'inside' | 'after'

export interface MovePlan {
  parent_id: string | null
  ordered_ids: string[]
}

/** 按指针在行内的纵向位置判定落点：上 30% 之前、下 30% 之后、其余为放入子级。 */
export function pickDropPos(clientY: number, rect: { top: number; height: number }): DropPos {
  const ratio = (clientY - rect.top) / Math.max(rect.height, 1)
  if (ratio < 0.3) return 'before'
  if (ratio > 0.7) return 'after'
  return 'inside'
}

export function findNode(nodes: TreeNode[], id: string): TreeNode | null {
  for (const n of nodes) {
    if (n.id === id) return n
    const sub = findNode(n.children, id)
    if (sub) return sub
  }
  return null
}

function locate(nodes: TreeNode[], id: string): { node: TreeNode; list: TreeNode[] } | null {
  for (const n of nodes) {
    if (n.id === id) return { node: n, list: nodes }
    const deep = locate(n.children, id)
    if (deep) return deep
  }
  return null
}

function contains(node: TreeNode, id: string): boolean {
  if (node.id === id) return true
  return node.children.some((c) => contains(c, id))
}

/**
 * 计算拖拽 dragId 至 targetId 相邻/内部后的完整兄弟顺序。
 * 非法移动（移入自身子树）或顺序无变化返回 null（调用方静默跳过）。
 */
export function planMove(
  nodes: TreeNode[],
  dragId: string,
  targetId: string,
  pos: DropPos,
): MovePlan | null {
  const drag = findNode(nodes, dragId)
  if (!drag || dragId === targetId) return null
  const hit = locate(nodes, targetId)
  if (!hit) return null
  const { node: target, list } = hit

  let parentId: string | null
  let ordered: string[]
  if (pos === 'inside') {
    if (contains(drag, targetId)) return null // 移入自身或自身子树：前端拦截
    parentId = target.id
    ordered = [...target.children.map((c) => c.id), dragId]
  } else {
    parentId = target.parent_id ?? null
    const list0 = list
    ordered = list0.map((s) => s.id).filter((id) => id !== dragId)
    let at = ordered.indexOf(targetId)
    if (at < 0) at = Math.max(ordered.length - 1, 0)
    ordered.splice(pos === 'before' ? at : at + 1, 0, dragId)
  }

  const sameParent = (drag.parent_id ?? null) === parentId
  if (sameParent && pos !== 'inside') {
    const orig = list.map((s) => s.id)
    if (orig.every((id, i) => id === ordered[i])) return null
  }
  return { parent_id: parentId, ordered_ids: ordered }
}
