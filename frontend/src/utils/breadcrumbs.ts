// T7.4 验收辅助：面包屑路径计算（纯函数）。
import type { TreeNode } from '@/api'
import { pathOf } from '@/stores/tree'

export interface Crumb {
  id: string
  title: string
}

export function crumbsFor(nodes: TreeNode[], id: string): Crumb[] {
  return pathOf(nodes, id).map((n) => ({ id: n.id, title: n.title }))
}
