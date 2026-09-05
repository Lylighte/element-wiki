import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import i18n from '@/i18n'
import HomeView from './HomeView.vue'
import treeStore from '@/stores/tree'
import { setPermissions } from '@/permissions'
import { docApi } from '@/api'

vi.mock('@/api', () => ({
  docApi: {
    tree: vi.fn().mockResolvedValue({
      nodes: [{
        id: 'home-1', parent_id: null, slug: 'home', title: 'Home',
        sort_key: 100, restricted: false, children: [],
      }],
    }),
    create: vi.fn(),
  },
}))

describe('HomeView', () => {
  beforeEach(() => {
    treeStore.state.loaded = false
    treeStore.state.loading = false
    treeStore.state.nodes = []
    setPermissions([])
    vi.mocked(docApi.create).mockClear()
    vi.mocked(docApi.tree).mockResolvedValue({
      nodes: [{
        id: 'home-1', parent_id: null, slug: 'home', title: 'Home',
        sort_key: 100, restricted: false, children: [],
      }],
    })
  })

  it('已有 home 文档时跳转到文档页而不是渲染空白页', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/', component: HomeView },
        { path: '/docs/:pathMatch(.*)*', component: { template: '<div />' } },
      ],
    })
    await router.push('/')
    await router.isReady()
    mount(HomeView, { global: { plugins: [router, i18n] } })
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(router.currentRoute.value.path).toBe('/docs/home')
  })

  it('树加载失败时显示错误并支持重试', async () => {
    vi.mocked(docApi.tree).mockRejectedValueOnce(new Error('network error'))
    const router = createRouter({ history: createMemoryHistory(), routes: [] })
    const wrapper = mount(HomeView, { global: { plugins: [router, i18n] } })

    await flushPromises()
    expect(wrapper.find('[data-test="home-error"]').exists()).toBe(true)
    await wrapper.find('[data-test="home-retry"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-test="home-error"]').exists()).toBe(false)
  })

  it('创建首页遇 409（首页已存在）→ 提示并跳转 /docs/home（T16.5）', async () => {
    setPermissions(['document.create'])
    // 首次加载无首页 → 显示创建表单；409 后重查到首页 → 跳转
    vi.mocked(docApi.tree).mockResolvedValueOnce({ nodes: [] })
    vi.mocked(docApi.create).mockRejectedValueOnce(Object.assign(new Error('conflict'), { status: 409 }))
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/', component: HomeView },
        { path: '/docs/:pathMatch(.*)*', component: { template: '<div />' } },
      ],
    })
    await router.push('/')
    await router.isReady()
    const wrapper = mount(HomeView, { global: { plugins: [router, i18n] } })
    await flushPromises()
    expect(wrapper.find('[data-test="home-empty"]').exists()).toBe(true)

    await wrapper.find('[data-test="home-title"]').setValue('Home')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(docApi.create).toHaveBeenCalledWith({ slug: 'home', title: 'Home' })
    expect(router.currentRoute.value.path).toBe('/docs/home')
  })

  it('创建首页遇 409 且仍无首页 → 按钮复位不卡死，留守空状态（T16.5）', async () => {
    setPermissions(['document.create'])
    vi.mocked(docApi.tree).mockResolvedValue({ nodes: [] })
    vi.mocked(docApi.create).mockRejectedValueOnce(Object.assign(new Error('conflict'), { status: 409 }))
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: HomeView }] })
    await router.push('/')
    await router.isReady()
    const wrapper = mount(HomeView, { global: { plugins: [router, i18n] } })
    await flushPromises()

    await wrapper.find('[data-test="home-title"]').setValue('Home')
    await wrapper.find('form').trigger('submit')
    await flushPromises()

    expect(docApi.create).toHaveBeenCalledTimes(1)
    expect(wrapper.find('[data-test="home-empty"]').exists()).toBe(true)
    expect((wrapper.find('[data-test="create-home-btn"]').element as HTMLButtonElement).disabled).toBe(false)
  })

  it('匿名关闭（树 401）→ 登录引导替代无效重试（T16.6）', async () => {
    vi.mocked(docApi.tree).mockRejectedValueOnce(Object.assign(new Error('401'), { status: 401 }))
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/', component: HomeView },
        { path: '/login', component: { template: '<div />' } },
        { path: '/docs/:pathMatch(.*)*', component: { template: '<div />' } },
      ],
    })
    await router.push('/')
    await router.isReady()
    const wrapper = mount(HomeView, {
      global: { plugins: [router, i18n], stubs: { RouterLink: false } },
    })
    await flushPromises()

    expect(wrapper.find('[data-test="home-need-login"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="home-error"]').exists()).toBe(false)
    const link = wrapper.find('[data-test="home-login-link"]')
    expect(link.attributes('href')).toContain('/login')
    expect(link.attributes('href')).toContain('redirect=')
    wrapper.unmount()
  })

  it('已登录读者无首页 → pickSidebar 提示可见（权限码点分拼写修复）', async () => {
    setPermissions(['document.read'])
    vi.mocked(docApi.tree).mockResolvedValueOnce({ nodes: [] })
    const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: HomeView }] })
    await router.push('/')
    await router.isReady()
    const wrapper = mount(HomeView, { global: { plugins: [router, i18n] } })
    await flushPromises()

    expect(wrapper.find('[data-test="home-empty"]').exists()).toBe(true)
    expect(wrapper.text()).toContain(i18n.global.t('home.pickSidebar'))
    wrapper.unmount()
  })
})
