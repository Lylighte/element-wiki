// T10.1 验收：语言决策链——显式选择 > 浏览器语言 > 站点 default_lang > zh-CN。
import { describe, expect, it, beforeEach, vi } from 'vitest'

describe('locale detection chain', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.stubGlobal('navigator', { language: 'zh-CN' })
    vi.resetModules()
  })

  async function mod() {
    return import('@/i18n')
  }

  it('显式 localStorage 选择优先于一切', async () => {
    localStorage.setItem('lang', 'en')
    vi.stubGlobal('navigator', { language: 'zh-CN' })
    const { detectInitialLocale } = await mod()
    expect(detectInitialLocale('zh-CN')).toBe('en')
  })

  it('无选择时按浏览器语言', async () => {
    vi.stubGlobal('navigator', { language: 'en-US' })
    const { detectInitialLocale } = await mod()
    expect(detectInitialLocale()).toBe('en')
    vi.stubGlobal('navigator', { language: 'zh-TW' })
    expect(detectInitialLocale('en')).toBe('zh-CN')
  })

  it('浏览器不可判定时回落站点 default_lang', async () => {
    vi.stubGlobal('navigator', { language: '' })
    const { detectInitialLocale } = await mod()
    expect(detectInitialLocale('en')).toBe('en')
    expect(detectInitialLocale('zh-CN')).toBe('zh-CN')
    expect(detectInitialLocale(undefined)).toBe('zh-CN')
  })

  it('applySiteDefault 不覆盖用户显式选择；setLocale 持久化', async () => {
    const m = await mod()
    m.setLocale('en')
    expect(localStorage.getItem('lang')).toBe('en')
    m.applySiteDefault('zh-CN')
    expect(m.default.global.locale.value).toBe('en')

    localStorage.removeItem('lang')
    m.applySiteDefault('zh-CN')
    expect(m.default.global.locale.value).toBe('zh-CN')
  })
})
