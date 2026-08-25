import { reactive } from 'vue'

// 侧栏树折叠状态（T8.3）：localStorage 持久化，刷新后保持。
const KEY = 'ew.tree.collapsed'

function load(): Set<string> {
  try {
    const raw = localStorage.getItem(KEY)
    if (!raw) return new Set()
    const arr = JSON.parse(raw)
    return Array.isArray(arr) ? new Set(arr.filter((x): x is string => typeof x === 'string')) : new Set()
  } catch {
    return new Set()
  }
}

const state = reactive<{ ids: Set<string> }>({ ids: load() })

function persist(): void {
  try {
    localStorage.setItem(KEY, JSON.stringify([...state.ids]))
  } catch {
    /* 存储不可用时静默降级为会话内状态 */
  }
}

function toggle(id: string): void {
  if (state.ids.has(id)) state.ids.delete(id)
  else state.ids.add(id)
  persist()
}

function isCollapsed(id: string): boolean {
  return state.ids.has(id)
}

export const collapseStore = { state, toggle, isCollapsed }
export default collapseStore
