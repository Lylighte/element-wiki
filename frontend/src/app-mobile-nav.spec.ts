// 窄屏顶栏保留目录与搜索，账号和偏好设置收进菜单。
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
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
  siteApi: { info: vi.fn().mockResolvedValue({ title: 'Wiki', default_lang: 'zh-CN', anonymous_read: false, comments_enabled: false }) },
}))

import { authApi as mockedAuth, docApi as mockedDocApi } from '@/api'

function stubNarrow() {
  vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({
    matches: false,
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
  }))
}

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

async function mountMobile(path = '/') {
  stubNarrow()
  const router = makeRouter()
  await router.push(path)
  await router.isReady()
  const w = mount(App, {
    global: { plugins: [i18n, ElementPlus, router], stubs: { RouterLink: false } },
    attachTo: document.body,
  })
  await new Promise((r) => setTimeout(r, 0))
  await new Promise((r) => setTimeout(r, 0))
  return w
}

describe('mobile top bar (M15)', () => {
  beforeEach(async () => {
    authStore.state.me = null
    authStore.state.initialized = false
    authStore.state.loading = false
    authStore.state.error = null
    localStorage.setItem('lang', 'zh-CN')
    const m = await import('@/i18n')
    m.default.global.locale.value = 'zh-CN'
    document.body.innerHTML = ''
    // treeStore 为模块级单例：restore/跨用例后重挂 SideTree 仍需可用实现
    ;(mockedDocApi.tree as ReturnType<typeof vi.fn>).mockResolvedValue({ nodes: [] })
  })
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('匿名窄屏：桌面 nav 不渲染，汉堡与⋯菜单存在', async () => {
    ;(mockedAuth.me as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('401'))
    const w = await mountMobile()
    expect(w.find('[data-test="nav-tree-toggle"]').exists()).toBe(true)
    expect(w.find('[data-test="nav-menu"]').exists()).toBe(true)
    expect(w.find('[data-test="login-link"]').exists()).toBe(false)
    w.unmount()
    document.body.innerHTML = ''
  })

  it('已登录窄屏：搜索直接可见，⋯菜单含令牌/退出/语言切换', async () => {
    ;(mockedAuth.me as ReturnType<typeof vi.fn>).mockResolvedValue({
      user: { id: 'u1', email: '', display_name: 'Dev', role: 'editor', status: 'active' },
      permissions: [],
    })
    const w = await mountMobile()
    expect(w.find('[data-test="m-search"]').attributes('href')).toBe('/search')
    await w.find('[data-test="nav-menu"]').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))
    const menu = Array.from(document.body.querySelectorAll('[data-test="mobile-menu"]'))
      .find((el) => (el as HTMLElement).style.display !== 'none')
    expect(menu).toBeTruthy()
    expect(menu!.querySelector('[data-test="m-tokens"]')).toBeTruthy()
    expect(menu!.querySelector('[data-test="m-logout"]')).toBeTruthy()
    expect(menu!.querySelector('[data-test="m-lang-toggle"]')).toBeTruthy()
    expect(menu!.querySelector('button')).toBeNull()
    w.unmount()
    document.body.innerHTML = ''
  })

  it('汉堡打开文档树抽屉（drawer 内挂载 SideTree）', async () => {
    ;(mockedAuth.me as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('401'))
    const w = await mountMobile()
    expect(document.body.querySelector('[data-test="side-tree"]')).toBeNull()
    await w.find('[data-test="nav-tree-toggle"]').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))
    expect(document.body.querySelector('[data-test="side-tree"]')).toBeTruthy()
    w.unmount()
    document.body.innerHTML = ''
  })
})
