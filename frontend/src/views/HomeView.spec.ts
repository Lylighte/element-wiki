import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import i18n from '@/i18n'
import HomeView from './HomeView.vue'
import treeStore from '@/stores/tree'
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
        { path: '/docs/:id', component: { template: '<div />' } },
      ],
    })
    await router.push('/')
    await router.isReady()
    mount(HomeView, { global: { plugins: [router, i18n] } })
    await new Promise((resolve) => setTimeout(resolve, 0))

    expect(router.currentRoute.value.path).toBe('/docs/home-1')
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
})
