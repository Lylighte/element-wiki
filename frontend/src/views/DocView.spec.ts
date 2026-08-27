import { describe, expect, it, beforeEach, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import i18n from '@/i18n'
import DocView from './DocView.vue'
import { authApi, docApi } from '@/api'

vi.mock('@/api', () => ({
  authApi: { me: vi.fn() },
  docApi: { render: vi.fn(), get: vi.fn(), exportMdURL: vi.fn(() => '/export.md') },
}))

function apiError(status: number) {
  return Object.assign(new Error(`HTTP ${status}`), { status })
}

async function mountDoc() {
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/docs/:id', name: 'doc', component: DocView, props: true },
      { path: '/login', name: 'login', component: { template: '<div />' } },
    ],
  })
  await router.push('/docs/d1')
  await router.isReady()
  const wrapper = mount(DocView, {
    props: { id: 'd1' },
    global: {
      plugins: [router, i18n],
      stubs: { CommentsPanel: true, AttachmentsPanel: true },
    },
  })
  return { router, wrapper }
}

describe('DocView error boundary', () => {
  beforeEach(() => {
    vi.mocked(authApi.me).mockRejectedValue(new Error('anonymous'))
    vi.mocked(docApi.render).mockResolvedValue({ html: '<p>body</p>', title: 'Doc', toc: [] })
    vi.mocked(docApi.get).mockResolvedValue({
      document: {
        id: 'd1', title: 'Doc', slug: 'doc', parent_id: null, sort_key: 100,
        visibility: 'standard', head_commit_id: 'c1', created_at: 1, updated_at: 1,
      },
    })
  })

  it('404 displays not found without exposing the error or mounting child panels', async () => {
    vi.mocked(docApi.render).mockRejectedValueOnce(apiError(404))
    const { wrapper } = await mountDoc()
    await flushPromises()

    expect(wrapper.find('[data-test="doc-not-found"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('HTTP 404')
    expect(wrapper.findComponent({ name: 'CommentsPanel' }).exists()).toBe(false)
  })

  it('403 displays forbidden without exposing the server detail', async () => {
    vi.mocked(docApi.render).mockRejectedValueOnce(
      Object.assign(new Error('secret detail'), { status: 403 }),
    )
    const { wrapper } = await mountDoc()
    await flushPromises()

    expect(wrapper.find('[data-test="doc-forbidden"]').exists()).toBe(true)
    expect(wrapper.text()).not.toContain('secret detail')
  })

  it('network failure displays retry and recovers', async () => {
    vi.mocked(docApi.render).mockRejectedValueOnce(new Error('offline'))
    const { wrapper } = await mountDoc()
    await flushPromises()

    expect(wrapper.find('[data-test="doc-error"]').exists()).toBe(true)
    await wrapper.find('[data-test="doc-retry"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-test="doc-html"]').text()).toBe('body')
    expect(wrapper.find('[data-test="doc-error"]').exists()).toBe(false)
  })

  it('401 redirects to login while preserving the document target', async () => {
    vi.mocked(docApi.render).mockRejectedValueOnce(apiError(401))
    const { router } = await mountDoc()
    await flushPromises()

    expect(router.currentRoute.value.name).toBe('login')
    expect(router.currentRoute.value.query.redirect).toBe('/docs/d1')
  })
})
