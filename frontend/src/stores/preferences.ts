import { userPreferencesApi, type UserPreferences } from '@/api'
import { setLocale } from '@/i18n'
import { applyThemePreference, type ThemePreference } from '@/composables/useTheme'
import type { Locale } from '@/i18n'

export interface DisplayPreferences {
  language: Locale
  theme: ThemePreference
}

export function applyDisplayPreferences(preferences: DisplayPreferences) {
  setLocale(preferences.language)
  applyThemePreference(preferences.theme)
}

export async function loadDisplayPreferences(): Promise<UserPreferences | null> {
  const { preferences } = await userPreferencesApi.get()
  if (preferences) applyDisplayPreferences(preferences)
  return preferences
}

export async function saveDisplayPreferences(preferences: DisplayPreferences) {
  const { preferences: saved } = await userPreferencesApi.set(preferences.language, preferences.theme)
  applyDisplayPreferences(saved)
  return saved
}
