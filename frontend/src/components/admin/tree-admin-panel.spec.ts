// M16/T16.2 验收：后台文档树面板——拖拽移动/排序接线（模块级 dndState）、
// 非法拖拽零 API、内联重命名、新建子文档、移入回收站。
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { DOMWrapper, mount } from '@vue/test-utils'
import i18n from '@/i18n'
import ElementPlus from 'element-plus'
import TreeAdminPanel from '@/components/admin/TreeAdminPanel.vue'
import treeStore from '@/stores/tree'
import { docApi } from '@/api'

const nodes = [
  {
    id: 'a', parent_id: null, title: 'A', slug: 'a', sort_key: 100,
    restricted: false, children: [
      { id: 'b', parent_id: 'a', title: 'B', slug: 'b', sort_key: 100, restricted: false, children: [] },
    ],
  },
  {
    id: 'c', parent_id: null, title: 'C', slug: 'c', sort_key: 200,
    restricted: false, children: [],
  },
]

vi.mock('@/api', () => ({
  docApi: {
    tree: vi.fn().mockResolvedValue({ nodes: [] }),
    patch: vi.fn().mockResolvedValue({}),
    reorder: vi.fn().mockResolvedValue(undefined),
    create: vi.fn().mockResolvedValue({ document: { id: 'new1', slug: 'new-kid' } }),
    remove: vi.fn().mockResolvedValue(undefined),
  },
}))

function fakeRect(top: number, height: number) {
  return { top, bottom: top + height, left: 0, right: 100, width: 100, height, x: 0, y: top, toJSON() { return this } } as DOMRect
}

async function mountPanel() {
  const w = mount(TreeAdminPanel, {
    global: { plugins: [i18n, ElementPlus] },
    attachTo: document.body,
  })
  for (let i = 0; i < 30 && w.findAll('[data-test="admin-tree-row"]').length < 3; i++) {
    await new Promise((r) => setTimeout(r, 10))
  }
  return w
}

function rowsOf(w: Awaited<ReturnType<typeof mountPanel>>) {
  return w.findAll('[data-test="admin-tree-row"]')
}

/** 指定树数据挂载（treeStore 为模块级单例，先重置 loaded 才会重新拉取）。 */
async function mountPanelWithTree(tree: unknown) {
  treeStore.state.loaded = false
  treeStore.state.nodes = []
  vi.mocked(docApi.tree).mockResolvedValue({ nodes: JSON.parse(JSON.stringify(tree)) } as never)
  return mountPanel()
}

/** 模拟把 rows[from] 拖到 rows[to] 的 pos 位置（jsdom：stub 目标行 rect 决定落点）。 */
async function dragTo(w: Awaited<ReturnType<typeof mountPanel>>, from: number, to: number, clientY: number, top = 0, height = 100) {
  const rows = rowsOf(w)
  await rows[from].trigger('dragstart')
  const target = rows[to].element as HTMLElement
  const spy = vi.spyOn(target, 'getBoundingClientRect').mockReturnValue(fakeRect(top, height))
  await rows[to].trigger('dragover', { clientY })
  await rows[to].trigger('drop')
  spy.mockRestore()
  await new Promise((r) => setTimeout(r, 0))
}

describe('TreeAdminPanel (M16)', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
    vi.mocked(docApi.tree).mockResolvedValue({ nodes: JSON.parse(JSON.stringify(nodes)) })
    vi.mocked(docApi.patch).mockClear()
    ;(docApi.reorder as ReturnType<typeof vi.fn>).mockClear()
    ;(docApi.create as ReturnType<typeof vi.fn>).mockClear()
    ;(docApi.remove as ReturnType<typeof vi.fn>).mockClear()
  })

  it('跨父拖入子级：patch 改父 + reorder 排序（接线走模块级 draggingId）', async () => {
    const w = await mountPanel()
    await dragTo(w, 2, 0, 50) // C 拖到 A 行中间 → 移入 A 子级
    expect(docApi.patch).toHaveBeenCalledWith('c', { parent_id: 'a' })
    expect(docApi.reorder).toHaveBeenCalledWith('a', ['b', 'c'])
    w.unmount()
  })

  it('同层排序：drop 在行上/下方 → 仅 reorder 不 patch', async () => {
    const w = await mountPanel()
    await dragTo(w, 2, 0, 10) // C 拖到 A 行上方 10%（before）
    expect(docApi.patch).not.toHaveBeenCalled()
    expect(docApi.reorder).toHaveBeenCalledWith(null, ['c', 'a'])
    w.unmount()
  })

  it('拖入自身子树被前端拦截：零 API 调用', async () => {
    const w = await mountPanel()
    await dragTo(w, 0, 1, 50) // A 拖到自己的子节点 B 内部
    expect(docApi.patch).not.toHaveBeenCalled()
    expect(docApi.reorder).not.toHaveBeenCalled()
    w.unmount()
  })

  it('拖到自身行：零 API 调用', async () => {
    const w = await mountPanel()
    await dragTo(w, 0, 0, 50)
    expect(docApi.patch).not.toHaveBeenCalled()
    expect(docApi.reorder).not.toHaveBeenCalled()
    w.unmount()
  })

  it('内联重命名：enter 保存 patch({title}) 并刷新树', async () => {
    const w = await mountPanel()
    await rowsOf(w)[2].find('[data-test="admin-tree-rename"]').trigger('click')
    const input = w.find('[data-test="admin-tree-rename-input"]')
    expect(input.exists()).toBe(true)
    await input.setValue('C2')
    await input.trigger('keydown.enter')
    await new Promise((r) => setTimeout(r, 0))
    expect(docApi.patch).toHaveBeenCalledWith('c', { title: 'C2' })
    w.unmount()
  })

  it('新建子文档：留空 slug 不提交该字段，创建后刷新树', async () => {
    const w = await mountPanel()
    await rowsOf(w)[0].find('[data-test="admin-tree-new-child"]').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))
    const title = new DOMWrapper(document.querySelector('[data-test="admin-tree-create-title"]')!)
    await title.setValue('New Kid')
    await new DOMWrapper(document.querySelector('[data-test="admin-tree-create-submit"]')!).trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(docApi.create).toHaveBeenCalledWith({ parent_id: 'a', slug: undefined, title: 'New Kid' })
    w.unmount()
  })

  it('移入回收站：remove + 刷新树', async () => {
    const w = await mountPanel()
    await rowsOf(w)[2].find('[data-test="admin-tree-trash"]').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(docApi.remove).toHaveBeenCalledWith('c')
    w.unmount()
  })

  it('上移按钮：同层前移仅 reorder 不 patch（T16.4）', async () => {
    const w = await mountPanel()
    await rowsOf(w)[2].find('[data-test="admin-tree-up"]').trigger('click') // C 上移 → [C, A]
    await new Promise((r) => setTimeout(r, 0))
    expect(docApi.patch).not.toHaveBeenCalled()
    expect(docApi.reorder).toHaveBeenCalledWith(null, ['c', 'a'])
    w.unmount()
  })

  it('下移按钮：同层后移仅 reorder（T16.4）', async () => {
    const w = await mountPanel()
    await rowsOf(w)[0].find('[data-test="admin-tree-down"]').trigger('click') // A 下移 → [C, A]
    await new Promise((r) => setTimeout(r, 0))
    expect(docApi.patch).not.toHaveBeenCalled()
    expect(docApi.reorder).toHaveBeenCalledWith(null, ['c', 'a'])
    w.unmount()
  })

  it('首行禁用上移、末行禁用下移（T16.4）', async () => {
    const w = await mountPanel()
    const rows = rowsOf(w)
    expect(rows[0].find('[data-test="admin-tree-up"]').attributes('disabled')).toBeDefined()
    expect(rows[0].find('[data-test="admin-tree-down"]').attributes('disabled')).toBeUndefined()
    expect(rows[2].find('[data-test="admin-tree-up"]').attributes('disabled')).toBeUndefined()
    expect(rows[2].find('[data-test="admin-tree-down"]').attributes('disabled')).toBeDefined()
    w.unmount()
  })

  it('移动到…对话框：跨父移动 patch + reorder（T16.4）', async () => {
    const w = await mountPanel()
    await rowsOf(w)[2].find('[data-test="admin-tree-move"]').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))
    const select = new DOMWrapper(document.querySelector('[data-test="admin-tree-move-parent"]')!)
    await select.setValue('a')
    await new DOMWrapper(document.querySelector('[data-test="admin-tree-move-submit"]')!).trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(docApi.patch).toHaveBeenCalledWith('c', { parent_id: 'a' })
    expect(docApi.reorder).toHaveBeenCalledWith('a', ['b', 'c'])
    w.unmount()
  })

  it('移动到…候选父级排除自身及子孙（T16.4）', async () => {
    const w = await mountPanel()
    await rowsOf(w)[0].find('[data-test="admin-tree-move"]').trigger('click') // 移动 A
    await new Promise((r) => setTimeout(r, 0))
    await new Promise((r) => setTimeout(r, 0))
    const select = document.querySelector('[data-test="admin-tree-move-parent"]') as HTMLSelectElement
    const texts = Array.from(select.options).map((o) => o.textContent)
    expect(texts.some((x) => x!.includes('A'))).toBe(false)
    expect(texts.some((x) => x!.includes('B'))).toBe(false)
    expect(texts.some((x) => x!.includes('C'))).toBe(true)
    w.unmount()
  })

  // —— 首页文档保护（T16.5）：置顶 + 禁移动/拖拽/回收 ——
  const homeTree = [
    { id: 'x', parent_id: null, title: 'X', slug: 'x', sort_key: 100, restricted: false, children: [] },
    {
      id: 'home', parent_id: null, title: 'Home', slug: 'home', sort_key: 200, restricted: false,
      children: [
        { id: 'h1', parent_id: 'home', title: 'H1', slug: 'h1', sort_key: 100, restricted: false, children: [] },
      ],
    },
    { id: 'c', parent_id: null, title: 'C', slug: 'c', sort_key: 300, restricted: false, children: [] },
  ]

  it('home 置顶渲染；home 行禁拖、无移动/回收按钮，重命名与新建子文档保留', async () => {
    const w = await mountPanelWithTree(homeTree)
    const rows = rowsOf(w) // 归一化后 DOM 顺序：home, h1, x, c
    expect(rows.map((r) => r.find('[data-test="admin-tree-title"]').text())).toEqual(
      ['Home', 'H1', 'X', 'C'],
    )
    expect(rows[0].attributes('draggable')).toBe('false')
    expect(rows[2].attributes('draggable')).toBe('true')
    expect(rows[0].find('[data-test="admin-tree-move"]').exists()).toBe(false)
    expect(rows[0].find('[data-test="admin-tree-trash"]').exists()).toBe(false)
    expect(rows[0].find('[data-test="admin-tree-rename"]').exists()).toBe(true)
    expect(rows[0].find('[data-test="admin-tree-new-child"]').exists()).toBe(true)
    expect(rows[0].find('[data-test="admin-tree-up"]').attributes('disabled')).toBeDefined()
    expect(rows[0].find('[data-test="admin-tree-down"]').attributes('disabled')).toBeDefined()
    w.unmount()
  })

  it('紧随 home 之下的行禁用上移（不可越过首页），更后的行可用', async () => {
    const w = await mountPanelWithTree(homeTree)
    const rows = rowsOf(w)
    expect(rows[2].find('[data-test="admin-tree-up"]').attributes('disabled')).toBeDefined() // X，前一是 home
    expect(rows[3].find('[data-test="admin-tree-up"]').attributes('disabled')).toBeUndefined() // C，前一是 X
    w.unmount()
  })

  it('拖动 home 零 API；拖到 home 行上/下方（before/after）零 API', async () => {
    const w = await mountPanelWithTree(homeTree)
    await dragTo(w, 0, 3, 50) // home 拖到 C 行中间 → 源被禁
    await dragTo(w, 2, 0, 10) // X 拖到 home 行上方 → before 被拦截
    expect(docApi.patch).not.toHaveBeenCalled()
    expect(docApi.reorder).not.toHaveBeenCalled()
    w.unmount()
  })

  it('拖到 home 行中间（inside）允许：X 移入 home 子级', async () => {
    const w = await mountPanelWithTree(homeTree)
    await dragTo(w, 2, 0, 50) // X 拖到 home 行中间
    expect(docApi.patch).toHaveBeenCalledWith('x', { parent_id: 'home' })
    expect(docApi.reorder).toHaveBeenCalledWith('home', ['h1', 'x'])
    w.unmount()
  })
})
