// 主题切换（对齐 element-skin useTheme）：html.dark 显式类 + localStorage 持久化 +
// 系统偏好跟随（仅当用户未显式选择时）。样式变量见 src/style.css。
import { onUnmounted, ref } from 'vue'

const STORAGE_KEY = 'theme'

export function useTheme() {
  const isDark = ref(false)
  let mediaQuery: MediaQueryList | null = null

  function applyTheme() {
    document.documentElement.classList.toggle('dark', isDark.value)
  }

  function handlePreferenceChange(event: MediaQueryListEvent) {
    if (localStorage.getItem(STORAGE_KEY)) return
    isDark.value = event.matches
    applyTheme()
  }

  function startSystemPreferenceWatcher() {
    if (mediaQuery || typeof window === 'undefined' || typeof window.matchMedia !== 'function') return
    mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
    mediaQuery.addEventListener('change', handlePreferenceChange)
  }

  function stopSystemPreferenceWatcher() {
    if (!mediaQuery) return
    mediaQuery.removeEventListener('change', handlePreferenceChange)
    mediaQuery = null
  }

  function initTheme() {
    const saved = localStorage.getItem(STORAGE_KEY)
    if (saved) isDark.value = saved === 'dark'
    else if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
      isDark.value = window.matchMedia('(prefers-color-scheme: dark)').matches
    }
    applyTheme()
    startSystemPreferenceWatcher()
  }

  function toggleTheme() {
    isDark.value = !isDark.value
    localStorage.setItem(STORAGE_KEY, isDark.value ? 'dark' : 'light')
    applyTheme()
  }

  onUnmounted(stopSystemPreferenceWatcher)

  return { isDark, initTheme, toggleTheme }
}
