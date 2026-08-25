// T7.3 验收：匿名头部显示登录入口，登录后显示 me/退出。
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

import ElementPlus from 'element-plus'
import i18n from '@/i18n'
import App from '@/App.vue'

vi.mock('@/api', () => ({
  authApi: {
    me: vi.fn(),
    logout: vi.fn().mockResolvedValue(undefined),
    status: vi.fn().mockResolvedValue({ enabled: true, provider_name: 'Test' }),
    loginUrl: (r: string) => '/v1/auth/oidc/login?redirect=' + r,
  },
  docApi: { tree: vi.fn().mockResolvedValue({ nodes: [] }) },
}))

import { authApi as mockedAuth } from '@/api'

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<div />' } },
      { path: '/login', component: { template: '<div data-test="login" />' } },
      { path: '/search', component: { template: '<div />' } },
      { path: '/settings/tokens', component: { template: '<div />' } },
    ],
  })
}

async function mountApp() {
  const router = makeRouter()
  await router.push('/')
  await router.isReady()
  const w = mount(App, {
    global: { plugins: [i18n, ElementPlus, router] },
  })
  // 等待 onMounted 的 me() 微任务与后续渲染
  await new Promise((r) => setTimeout(r, 0))
  await new Promise((r) => setTimeout(r, 0))
  return w
}

describe('header auth entry', () => {
  beforeEach(async () => {
    localStorage.setItem('lang', 'zh-CN')
    const m = await import('@/i18n')
    m.default.global.locale.value = 'zh-CN'
    localStorage.setItem('lang', 'zh-CN')
  })
  afterEach(() => vi.restoreAllMocks())

  it('匿名 → 显示 SSO 登录链接，隐藏 me/退出', async () => {
    ;(mockedAuth.me as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('401'))
    const w = await mountApp()
    expect(w.find('[data-test="login-link"]').exists()).toBe(true)
    expect(w.text()).not.toContain('退出')
  })

  it('已登录 → 隐藏登录链接，显示用户名与退出', async () => {
    ;(mockedAuth.me as ReturnType<typeof vi.fn>).mockResolvedValue({
      user: { id: 'u1', email: '', display_name: 'Dev', role: 'editor', status: 'active' },
      permissions: [],
    })
    const w = await mountApp()
    expect(w.find('[data-test="login-link"]').exists()).toBe(false)
    expect(w.text()).toContain('Dev')
    expect(w.text()).toContain('退出')
  })
})
