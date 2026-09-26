import { reactive } from 'vue'
import type { SiteInfo } from '@/api'

// 站点公开信息（T10.1/T11.2）：App 首屏消费 /v1/site；设置保存后即时更新标题。
// commentsEnabled 为首载快照：null 表示站点信息未就绪，消费方自行兜底。
const state = reactive({
  title: '', loaded: false, commentsEnabled: null as boolean | null, timezone: 'UTC',
  siteIconURL: '', themePreset: 'blue',
  lightPrimary: '#2563EB', lightAccent: '#DBEAFE', lightFocus: '#2563EB',
  darkPrimary: '#60A5FA', darkAccent: '#1E3A5F', darkFocus: '#93C5FD',
  articleFooterHTML: '', sidebarFooterHTML: '',
})

function setTitle(title: string) {
  if (title) {
    state.title = title
    state.loaded = true
  }
}

function setCommentsEnabled(enabled: boolean) {
  state.commentsEnabled = enabled
}

function setTimezone(timezone: string) {
  if (timezone) state.timezone = timezone
}

function setSiteIconURL(url: string) {
  state.siteIconURL = url
  let favicon = document.querySelector<HTMLLinkElement>('link[data-ew-site-icon]')
  if (url) {
    if (!favicon) {
      favicon = document.createElement('link')
      favicon.rel = 'icon'
      favicon.dataset.ewSiteIcon = 'true'
      favicon.referrerPolicy = 'no-referrer'
      document.head.append(favicon)
    }
    favicon.href = url
  } else {
    favicon?.remove()
  }
}

function setThemeColors(colors: Pick<SiteInfo,
  'theme_preset' | 'theme_light_primary' | 'theme_light_accent' | 'theme_light_focus' |
  'theme_dark_primary' | 'theme_dark_accent' | 'theme_dark_focus'>) {
  state.themePreset = colors.theme_preset
  state.lightPrimary = colors.theme_light_primary
  state.lightAccent = colors.theme_light_accent
  state.lightFocus = colors.theme_light_focus
  state.darkPrimary = colors.theme_dark_primary
  state.darkAccent = colors.theme_dark_accent
  state.darkFocus = colors.theme_dark_focus
  const root = document.documentElement.style
  root.setProperty('--site-primary-light', state.lightPrimary)
  root.setProperty('--site-accent-light', state.lightAccent)
  root.setProperty('--site-focus-light', state.lightFocus)
  root.setProperty('--site-primary-dark', state.darkPrimary)
  root.setProperty('--site-accent-dark', state.darkAccent)
  root.setProperty('--site-focus-dark', state.darkFocus)
}

function setFooterHTML(article: string, sidebar: string) {
  state.articleFooterHTML = article
  state.sidebarFooterHTML = sidebar
}

function setSite(site: SiteInfo) {
  setTitle(site.title)
  setCommentsEnabled(site.comments_enabled)
  setTimezone(site.timezone)
  setSiteIconURL(site.site_icon_url ?? '')
  setThemeColors({
    theme_preset: site.theme_preset || 'blue',
    theme_light_primary: site.theme_light_primary || '#2563EB',
    theme_light_accent: site.theme_light_accent || '#DBEAFE',
    theme_light_focus: site.theme_light_focus || '#2563EB',
    theme_dark_primary: site.theme_dark_primary || '#60A5FA',
    theme_dark_accent: site.theme_dark_accent || '#1E3A5F',
    theme_dark_focus: site.theme_dark_focus || '#93C5FD',
  })
  setFooterHTML(site.article_footer_html ?? '', site.sidebar_footer_html ?? '')
}

export const siteStore = {
  state, setTitle, setCommentsEnabled, setTimezone, setSite, setSiteIconURL, setThemeColors, setFooterHTML,
}
export default siteStore
