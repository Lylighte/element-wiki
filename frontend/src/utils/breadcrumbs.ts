// T7.4 验收辅助：面包屑路径计算（纯函数）。
import type { TreeNode } from '@/api'
import { pathOf } from '@/stores/tree'

export interface Crumb {
  id: string
  title: string
  path: string // 完整 slug 路径（05 计划：公开 URL 形态）
}

export function crumbsFor(nodes: TreeNode[], id: string): Crumb[] {
  const chain = pathOf(nodes, id)
  let acc = ''
  return chain.map((n) => {
    acc = acc ? `${acc}/${n.slug}` : n.slug
    return { id: n.id, title: n.title, path: acc }
  })
}
