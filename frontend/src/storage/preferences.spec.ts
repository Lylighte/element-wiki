import { beforeEach, describe, expect, it } from 'vitest'
import { preferenceStorage } from './preferences'

describe('preferenceStorage', () => {
  beforeEach(() => localStorage.clear())

  it('persists only supported language and theme values', () => {
    expect(preferenceStorage.getLanguage()).toBeNull()
    expect(preferenceStorage.getTheme()).toBeNull()

    preferenceStorage.setLanguage('en')
    preferenceStorage.setTheme('system')
    expect(preferenceStorage.getLanguage()).toBe('en')
    expect(preferenceStorage.getTheme()).toBe('system')

    localStorage.setItem('lang', 'fr')
    localStorage.setItem('theme', 'sepia')
    expect(preferenceStorage.getLanguage()).toBeNull()
    expect(preferenceStorage.getTheme()).toBeNull()
  })
})
