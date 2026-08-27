import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import i18n from '@/i18n'
import SearchView from './SearchView.vue'

vi.mock('@/api', () => ({
  searchApi: {
    query: vi.fn().mockResolvedValue({
      items: [{ document_id: 'd1', title: '<img src=x onerror=alert(1)>', snippet: '<mark>safe</mark><script>alert(1)</script>' }],
    }),
  },
}))

describe('SearchView', () => {
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
