import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import i18n from '@/i18n'
import SearchView from './SearchView.vue'
import treeStore from '@/stores/tree'
import { docApi } from '@/api'

vi.mock('@/api', () => ({
  searchApi: {
    query: vi.fn().mockResolvedValue({
      items: [{ document_id: 'd1', title: '<img src=x onerror=alert(1)>', snippet: '<mark>safe</mark><script>alert(1)</script>' }],
    }),
  },
  docApi: { tree: vi.fn().mockResolvedValue({ nodes: [] }) },
}))

describe('SearchView', () => {
  it('waits for the tree and links a nested result by its slug path', async () => {
    treeStore.state.loaded = false
    treeStore.state.nodes = []
    vi.mocked(docApi.tree).mockResolvedValueOnce({ nodes: [{
      id: 'parent', parent_id: null, slug: 'guide', title: 'Guide', sort_key: 100,
      restricted: false, children: [{
        id: 'd1', parent_id: 'parent', slug: 'setup', title: 'Setup', sort_key: 100,
        restricted: false, children: [],
      }],
    }] })
    const w = mount(SearchView, { global: { plugins: [i18n] } })
    await w.find('[data-test="search-input"]').setValue(' setup ')
    await w.find('form').trigger('submit')
    await flushPromises()

    expect(w.find('[data-test="search-hits"] a').attributes('href')).toBe('/docs/guide/setup')
  })

  it('搜索结果按文本显示，不执行或注入 HTML', async () => {
    const w = mount(SearchView, { global: { plugins: [i18n] } })
    await w.find('[data-test="search-input"]').setValue('x')
    await w.find('form').trigger('submit')
    await flushPromises()

    expect(w.find('script').exists()).toBe(false)
    expect(w.text()).toContain('<img src=x onerror=alert(1)>')
    expect(w.text()).toContain('safealert(1)')
  })
})
