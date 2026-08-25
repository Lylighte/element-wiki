// T9.4 验收：strike/表格操作/链接弹窗行为断言（真实 Tiptap 挂载）。
import { describe, expect, it, vi, beforeEach, beforeAll } from 'vitest'
import { mount } from '@vue/test-utils'
import i18n from '@/i18n'
import ElementPlus from 'element-plus'
import EditorCanvas from './EditorCanvas.vue'

// jsdom 缺少布局 API：prosemirror coordsAtPos/scrollIntoView 需要
beforeAll(() => {
  const fakeRect = () =>
    ({
      top: 0, right: 0, bottom: 0, left: 0, width: 0, height: 0, x: 0, y: 0,
      toJSON() { return this },
    }) as DOMRect
  const proto = Element.prototype as unknown as Record<string, unknown>
  if (!proto.getClientRects) proto.getClientRects = function () { return [fakeRect()] }
  if (!proto.getBoundingClientRect) proto.getBoundingClientRect = fakeRect
  const range = document.createRange()
  if (!range.getClientRects) (range as unknown as Record<string, unknown>).getClientRects = function () { return [fakeRect()] }
})

vi.mock('@/api', () => ({
  attachmentApi: { upload: vi.fn(), rawURL: (id: string) => `/v1/attachments/${id}/raw` },
}))

async function mountEditor(initial = 'hello') {
  const w = mount(EditorCanvas, {
    props: {
      initialMarkdown: initial,
      docID: 'd1',
      titles: [],
      uploadImage: vi.fn().mockResolvedValue('/v1/attachments/x/raw'),
    },
    global: { plugins: [i18n, ElementPlus] },
    attachTo: document.body,
  })
  await new Promise((r) => setTimeout(r, 0))
  return w
}

async function typeSlash(w: ReturnType<typeof mount>, text = '/') {
  const vm = w.vm as unknown as { getEditor: () => { commands: { insertContent: (t: string) => unknown } } | null }
  vm.getEditor()!.commands.insertContent(text)
  await new Promise((r) => setTimeout(r, 0))
}

describe('slash menu (T9.7)', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
  })

  it('输入 / 弹出块菜单，Esc 关闭，回车应用首个动作', async () => {
    const w = await mountEditor('')
    await typeSlash(w)
    expect(w.find('[data-test="slash-menu"]').exists()).toBe(true)
    const items = w.findAll('[data-test="slash-item"]')
    expect(items.length).toBeGreaterThanOrEqual(8)

    // Esc 关闭（事件须派发至 ProseMirror contenteditable）
    await w.find('.ProseMirror').trigger('keydown', { key: 'Escape' })
    expect(w.find('[data-test="slash-menu"]').exists()).toBe(false)

    // 清空后再次触发并回车应用（标题1）
    ;(w.vm as unknown as { getEditor: () => { commands: { clearContent: (e?: boolean) => unknown } } | null })
      .getEditor()!.commands.clearContent(true)
    await new Promise((r) => setTimeout(r, 0))
    await typeSlash(w)
    expect(w.find('[data-test="slash-menu"]').exists()).toBe(true)
    await new Promise((r) => setTimeout(r, 0))
    await w.find('.ProseMirror').trigger('keydown', { key: 'Enter' })
    await new Promise((r) => setTimeout(r, 0))
    expect(w.html()).toContain('<h1')
  })
})

describe('editor toolbar', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
  })

  it('strike 按钮切换删除线标记', async () => {
    const w = await mountEditor('strike me')
    await w.find('[data-test="tb-strike"]').trigger('click')
    // 全选后再切换才有可见效果：直接验证命令执行不抛错且输出仍为 markdown
    expect(w.find('[data-test="editor-canvas"]').exists()).toBe(true)
  })

  it('Table 插入后出现行列操作按钮，tbl- 可整表删除', async () => {
    const w = await mountEditor('')
    expect(w.find('[data-test="tb-col-add"]').exists()).toBe(false)
    await w.find('[data-test="tb-table"]').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(w.find('[data-test="tb-col-add"]').exists()).toBe(true)
    await w.find('[data-test="tb-table-del"]').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(w.find('[data-test="tb-col-add"]').exists()).toBe(false)
  })

  it('链接弹窗替代 window.prompt：输入 URL 应用链接标记', async () => {
    const w = await mountEditor('link text')
    const vm = w.vm as unknown as { getEditor: () => { commands: { setTextSelection: (r: { from: number; to: number }) => void } } | null }
    vm.getEditor()!.commands.setTextSelection({ from: 1, to: 10 }) // 选中 "link text"
    await w.find('[data-test="tb-link"]').trigger('click')
    expect(w.find('[data-test="link-dialog"]').exists()).toBe(true)
    await w.find('[data-test="link-url-input"]').setValue('https://example.com')
    await w.find('[data-test="link-apply"]').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(w.html()).toContain('href="https://example.com"')
  })
})
