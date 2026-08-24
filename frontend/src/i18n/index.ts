import { createI18n } from 'vue-i18n'
import zhCN from './locales/zh-CN.json'
import en from './locales/en.json'

export const messages = { 'zh-CN': zhCN, en }

const i18n = createI18n({
  legacy: false,
  locale: localStorage.getItem('lang') ?? 'zh-CN',
  fallbackLocale: 'en',
  messages,
})

export function setLocale(lang: 'zh-CN' | 'en') {
  i18n.global.locale.value = lang
  localStorage.setItem('lang', lang)
}

export default i18n
