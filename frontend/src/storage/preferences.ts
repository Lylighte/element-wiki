export type LocalePreference = 'zh-CN' | 'en'
export type ThemePreference = 'light' | 'dark' | 'system'

const keys = { language: 'lang', theme: 'theme' } as const

function read(key: string): string | null {
  try {
    return typeof localStorage === 'undefined' ? null : localStorage.getItem(key)
  } catch {
    return null
  }
}

function write(key: string, value: string) {
  try {
    if (typeof localStorage !== 'undefined') localStorage.setItem(key, value)
  } catch {
    // Keep preferences usable for this session when browser storage is unavailable.
  }
}

export const preferenceStorage = {
  getLanguage(): LocalePreference | null {
    const value = read(keys.language)
    return value === 'zh-CN' || value === 'en' ? value : null
  },
  setLanguage(value: LocalePreference) {
    write(keys.language, value)
  },
  getTheme(): ThemePreference | null {
    const value = read(keys.theme)
    return value === 'light' || value === 'dark' || value === 'system' ? value : null
  },
  setTheme(value: ThemePreference) {
    write(keys.theme, value)
  },
}
