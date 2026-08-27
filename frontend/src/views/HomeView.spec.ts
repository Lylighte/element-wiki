import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import i18n from '@/i18n'
import HomeView from './HomeView.vue'
import treeStore from '@/stores/tree'

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
})
