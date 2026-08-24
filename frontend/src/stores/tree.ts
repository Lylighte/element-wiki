import { reactive } from 'vue'
import { docApi, type TreeNode } from '@/api'

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
}
export default treeStore
