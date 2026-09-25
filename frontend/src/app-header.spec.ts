// T7.3 验收：匿名头部显示登录入口，登录后显示 me/退出。
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'

import ElementPlus from 'element-plus'
import i18n from '@/i18n'
import App from '@/App.vue'
import authStore from '@/stores/auth'

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

async function mountApp(path = '/') {
  const router = makeRouter()
  await router.push(path)
  await router.isReady()
  const w = mount(App, {
    global: {
      plugins: [i18n, ElementPlus, router],
      // 本测试挂载真实 router 并断言 RouterLink 生成的 href，禁用全局 stub。
      stubs: { RouterLink: false },
    },
  })
  // 等待 onMounted 的 me() 微任务与后续渲染
  await new Promise((r) => setTimeout(r, 0))
  await new Promise((r) => setTimeout(r, 0))
  return w
}

describe('header auth entry', () => {
  beforeEach(async () => {
    authStore.state.me = null
    authStore.state.initialized = false
    authStore.state.loading = false
    authStore.state.error = null
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

  it('已登录 → 顶栏显示账号菜单，菜单内有退出和偏好设置', async () => {
    ;(mockedAuth.me as ReturnType<typeof vi.fn>).mockResolvedValue({
      user: { id: 'u1', email: '', display_name: 'Dev', role: 'editor', status: 'active' },
      permissions: [],
    })
    const w = await mountApp()
    expect(w.find('[data-test="login-link"]').exists()).toBe(false)
    expect(w.find('[data-test="site-home"]').attributes('href')).toBe('/')
    expect(w.find('[data-test="account-menu-toggle"]').text()).toContain('Dev')
    await w.find('[data-test="account-menu-toggle"]').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(document.body.querySelector('[data-test="logout-btn"]')).toBeTruthy()
    expect(document.body.querySelector('[data-test="theme-toggle"]')).toBeTruthy()
    expect(document.body.querySelector('[data-test="lang-toggle"]')).toBeTruthy()
  })

  it('编辑者能从顶栏进入文档树管理', async () => {
    ;(mockedAuth.me as ReturnType<typeof vi.fn>).mockResolvedValue({
      user: { id: 'u1', email: '', display_name: 'Editor', role: 'editor', status: 'active' },
      permissions: ['document.update'],
    })
    const w = await mountApp()
    expect(w.find('[data-test="nav-admin"]').attributes('href')).toBe('/admin?tab=tree')
  })

  it('匿名访问深链接 → 登录链接保留当前路径', async () => {
    ;(mockedAuth.me as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('401'))
    const w = await mountApp('/search')
    expect(w.find('[data-test="login-link"]').attributes('href')).toBe('/login?redirect=/search')
  })
})
