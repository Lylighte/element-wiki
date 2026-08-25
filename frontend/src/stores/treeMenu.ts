import { reactive } from 'vue'
import type { TreeNode } from '@/api'

// 树节点上下文菜单共享状态（T8.5）：递归组件间通信的最小单例。
const state = reactive({
  x: 0,
  y: 0,
  node: null as TreeNode | null,
  renamingId: '',
  // T8.6：请求 App 打开新建对话框并预置父级
  requestCreate: false,
  createParentId: '',
})

function open(x: number, y: number, node: TreeNode): void {
  Object.assign(state, { x, y, node })
}

function close(): void {
  state.node = null
}

function startRename(id: string): void {
  close()
  state.renamingId = id
}

function endRename(): void {
  state.renamingId = ''
}

function requestCreateChild(parentId: string): void {
  close()
  state.createParentId = parentId
  state.requestCreate = true
}

export const treeMenuStore = { state, open, close, startRename, endRename, requestCreateChild }
export default treeMenuStore
