import { onUnmounted, ref } from 'vue'
import { preferenceStorage, type ThemePreference } from '@/storage/preferences'

export type { ThemePreference } from '@/storage/preferences'

const isDark = ref(false)
let mediaQuery: MediaQueryList | null = null

function resolveDark(choice: ThemePreference): boolean {
  if (choice === 'dark') return true
  if (choice === 'light') return false
  return typeof window !== 'undefined' && typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-color-scheme: dark)').matches
}

function applyDark(dark: boolean) {
  isDark.value = dark
  if (typeof document !== 'undefined') document.documentElement.classList.toggle('dark', dark)
}

export function applyThemePreference(choice: ThemePreference) {
  preferenceStorage.setTheme(choice)
  applyDark(resolveDark(choice))
}

function handlePreferenceChange(event: MediaQueryListEvent) {
  if (preferenceStorage.getTheme() === 'system' || preferenceStorage.getTheme() === null) {
    applyDark(event.matches)
  }
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
  applyDark(resolveDark(preferenceStorage.getTheme() ?? 'system'))
  startSystemPreferenceWatcher()
}

function toggleTheme() {
  const next = isDark.value ? 'light' : 'dark'
  preferenceStorage.setTheme(next)
  applyDark(next === 'dark')
}

export function useTheme() {
  onUnmounted(stopSystemPreferenceWatcher)

  return { isDark, initTheme, toggleTheme }
}
