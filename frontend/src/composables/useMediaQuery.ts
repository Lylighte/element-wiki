// 响应式断点（M15）：桌面优先回退——matchMedia 不可用（jsdom/SSR）时视为桌面，
// 保证既有测试与只读页零行为变化。
import { onBeforeUnmount, ref, type Ref } from 'vue'

export function useMediaQuery(query: string): Ref<boolean> {
  const matches = ref(true)
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return matches
  }
  const mql = window.matchMedia(query)
  matches.value = mql.matches
  const onChange = (e: MediaQueryListEvent) => {
    matches.value = e.matches
  }
  mql.addEventListener('change', onChange)
  onBeforeUnmount(() => mql.removeEventListener('change', onChange))
  return matches
}
