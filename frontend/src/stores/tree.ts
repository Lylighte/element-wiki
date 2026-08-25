import { reactive } from 'vue'
import { docApi, type TreeNode } from '@/api'
import { findNode, planMove, type DropPos } from '@/composables/treeDnd'

interface State {
  nodes: TreeNode[]
  loading: boolean
  loaded: boolean
}

const state = reactive<State>({ nodes: [], loading: false, loaded: false })
let inflight: Promise<void> | null = null

async function load(force = false): Promise<void> {
  if (state.loading && !force) return inflight ?? Promise.resolve()
  if (state.loaded && !force) return
  state.loading = true
  inflight = docApi
    .tree()
    .then((r) => {
      state.nodes = r.nodes
      state.loaded = true
    })
    .finally(() => {
      state.loading = false
      inflight = null
    })
  return inflight
}

/**
 * 拖拽移动编排（T8.4）：跨父先 patch 再 reorder，同层仅 reorder。
 * 返回 false 表示服务端拒绝（调用方负责用户可见提示）。
 */
async function moveNode(dragId: string, targetId: string, pos: DropPos): Promise<boolean> {
  const plan = planMove(state.nodes, dragId, targetId, pos)
  if (!plan) return true // 非法或无变化：静默跳过
  try {
    const drag = findNode(state.nodes, dragId)
    if ((drag?.parent_id ?? null) !== (plan.parent_id ?? null)) {
      await docApi.patch(dragId, { parent_id: plan.parent_id })
    }
    await docApi.reorder(plan.parent_id, plan.ordered_ids)
  } catch {
    return false
  }
  await load(true)
  return true
}

/** 在树中定位文档，返回根→自身的标题链（用于面包屑）。 */
export function pathOf(nodes: TreeNode[], id: string): TreeNode[] {
  for (const n of nodes) {
    if (n.id === id) return [n]
    const sub = pathOf(n.children, id)
    if (sub.length) return [n, ...sub]
  }
  return []
}

export const treeStore = {
  state,
  load,
  pathOf,
  moveNode,
}
export default treeStore
