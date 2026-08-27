// T9.6 验收：TOC 侧栏锚点跳转；wikilink 命中导航、死链提示（404 同源语义）。
import { describe, expect, it, vi, beforeEach, beforeAll } from 'vitest'
import { mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
import i18n from '@/i18n'
import ElementPlus, { ElMessage } from 'element-plus'
import { docApi } from '@/api'

vi.mock('@/api', () => ({
  docApi: {
    render: vi.fn().mockResolvedValue({
      html:
        '<h2 id="sec-1">Sec</h2><p><a class="wikilink" data-target="hello">go</a></p>' +
        '<p><a class="wikilink" data-target="missing">dead</a></p>',
      title: 'Doc',
      toc: [{ level: 2, text: 'Sec', id: 'sec-1' }],
    }),
    get: vi.fn().mockResolvedValue({
      document: { id: 'd1', title: 'Doc', parent_id: null },
    }),
    listCommits: vi.fn().mockResolvedValue({ items: [] }),
    tree: vi.fn().mockResolvedValue({ nodes: [] }),
    exportMdURL: vi.fn((id: string) => `/v1/documents/${id}/export.md`),
  },
  authApi: { me: vi.fn().mockRejectedValue(new Error('anon')) },
}))

vi.mock('@/components/doc/CommentsPanel.vue', () => ({ default: { template: '<div />' } }))
vi.mock('@/components/doc/AttachmentsPanel.vue', () => ({ default: { template: '<div />' } }))

import DocView from './DocView.vue'
import treeStore from '@/stores/tree'

beforeAll(() => {
  Element.prototype.scrollIntoView = Element.prototype.scrollIntoView || function () {}
})

function makeApp() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', component: { template: '<div />' } },
      { path: '/docs/:id', component: DocView, props: true },
    ],
  })
}

async function mountDoc() {
  const router = makeApp()
  await router.push('/docs/d1')
  await router.isReady()
  const app = mount(
    { template: '<router-view />', setup: () => ({}) },
    { global: { plugins: [i18n, ElementPlus, router] }, attachTo: document.body },
  )
  for (let i = 0; i < 30 && !app.find('[data-test="toc-panel"]').exists(); i++) {
    await new Promise((r) => setTimeout(r, 10))
  }
  return { app, router }
}

describe('doc view toc & wikilink', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    treeStore.state.nodes = [
      {
        id: 'id-hello', parent_id: null, title: 'Hello', slug: 'hello',
        sort_key: 100, restricted: false, children: [],
      },
    ]
    vi.mocked(docApi.render).mockResolvedValue({
      html:
        '<h2 id="sec-1">Sec</h2><p><a class="wikilink" data-target="hello">go</a></p>' +
        '<p><a class="wikilink" data-target="missing">dead</a></p>',
      title: 'Doc',
      toc: [{ level: 2, text: 'Sec', id: 'sec-1' }],
    })
  })

  it('TOC 渲染且点击平滑滚动到锚点', async () => {
    const spy = vi.spyOn(Element.prototype, 'scrollIntoView').mockImplementation(() => {})
    const { app } = await mountDoc()
    const links = app.findAll('[data-test="toc-link"]')
    expect(links.length).toBe(1)
    expect(links[0].text()).toBe('Sec')
    await links[0].trigger('click')
    expect(spy).toHaveBeenCalled()
    spy.mockRestore()
    app.unmount()
  })

  it('TOC 多级标题渲染为嵌套层级并显示视觉权重', async () => {
    vi.mocked(docApi.render).mockResolvedValueOnce({
      html: '<h1 id="a">A</h1><h2 id="b">B</h2><h3 id="c">C</h3>',
      title: 'Doc',
      toc: [
        { level: 1, text: 'A', id: 'a' },
        { level: 2, text: 'B', id: 'b' },
        { level: 3, text: 'C', id: 'c' },
      ],
    })
    const { app } = await mountDoc()
    const links = app.findAll('[data-test="toc-link"]')
    expect(links.map((l) => l.text())).toEqual(['A', 'B', 'C'])

    // 顶层只有 A，B 嵌套在 A 之下，C 嵌套在 B 之下
    const items = app.findAll('li')
    expect(items[0].text()).toContain('A')
    const bLi = items[0].find('ul > li')
    expect(bLi.exists()).toBe(true)
    expect(bLi.text()).toContain('B')
    expect(bLi.find('ul > li').text()).toContain('C')

    // 视觉权重：h1 加粗、h3 弱化
    expect(links[0].classes()).toContain('font-medium')
    expect(links[2].classes()).toContain('text-gray-500')
    app.unmount()
  })

  it('wikilink 命中树内 slug → 路由跳转', async () => {
    const { app, router } = await mountDoc()
    await app.findAll('a.wikilink')[0].trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(router.currentRoute.value.path).toBe('/docs/id-hello')
    app.unmount()
  })

  it('同一路由记录切换文档 → 重新加载目标文档', async () => {
    const { app, router } = await mountDoc()
    vi.mocked(docApi.get).mockClear()
    await router.push('/docs/id-hello')
    await new Promise((r) => setTimeout(r, 0))
    expect(docApi.get).toHaveBeenCalledWith('id-hello')
    app.unmount()
  })

  it('死链 → 提示目标不存在，不跳转', async () => {
    const warn = vi.spyOn(ElMessage, 'warning').mockImplementation((() => ({})) as never)
    const { app, router } = await mountDoc()
    await app.findAll('a.wikilink')[1].trigger('click')
    expect(warn).toHaveBeenCalledTimes(1)
    expect(String(warn.mock.calls[0][0])).toContain('missing')
    expect(router.currentRoute.value.path).toBe('/docs/d1')
    warn.mockRestore()
    app.unmount()
  })
})
