import { createI18n } from 'vue-i18n'
import zhCN from './locales/zh-CN.json'
import en from './locales/en.json'

export const messages = { 'zh-CN': zhCN, en }
export type Locale = 'zh-CN' | 'en'

const i18n = createI18n({
  legacy: false,
  locale: detectInitialLocale(),
  fallbackLocale: 'en',
  messages,
})

/** 语言决策（T10.1）：显式选择 > 浏览器语言 > 站点 default_lang > zh-CN。 */
export function detectInitialLocale(siteDefault?: string): Locale {
  const saved = localStorage.getItem('lang')
  if (saved === 'zh-CN' || saved === 'en') return saved
  const nav = typeof navigator !== 'undefined' ? navigator.language : ''
  if (/^zh/i.test(nav)) return 'zh-CN'
  if (/^en/i.test(nav)) return 'en'
  return siteDefault === 'en' ? 'en' : 'zh-CN'
}

/** 站点信息到达后调用：仅在用户未显式选择语言时生效。 */
export function applySiteDefault(siteDefault?: string) {
  if (localStorage.getItem('lang')) return
  i18n.global.locale.value = detectInitialLocale(siteDefault)
}

export function setLocale(lang: Locale) {
  i18n.global.locale.value = lang
  localStorage.setItem('lang', lang)
}

export default i18n
