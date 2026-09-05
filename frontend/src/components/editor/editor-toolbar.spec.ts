// 05 计划提交 3 验收：源码编辑器（textarea）——展示原始 Markdown、输入触发 change、
// 工具栏插入片段、图片上传插入引用、[[ 补全浮层、链接弹窗。
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import i18n from '@/i18n'
import ElementPlus from 'element-plus'
import EditorCanvas from './EditorCanvas.vue'

vi.mock('@/api', () => ({
  attachmentApi: { upload: vi.fn(), rawURL: (id: string) => `/v1/attachments/${id}/raw` },
}))

async function mountEditor(initial = 'hello', titles: string[] = []) {
  const w = mount(EditorCanvas, {
    props: {
      initialMarkdown: initial,
      docID: 'd1',
      titles,
      uploadImage: vi.fn().mockResolvedValue('/v1/attachments/x/raw'),
    },
    global: { plugins: [i18n, ElementPlus] },
    attachTo: document.body,
  })
  await new Promise((r) => setTimeout(r, 0))
  return w
}

function textareaOf(w: Awaited<ReturnType<typeof mountEditor>>): HTMLTextAreaElement {
  return w.find('[data-test="md-source"]').element as HTMLTextAreaElement
}

describe('source editor (05-3)', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
  })

  it('初始化 textarea 展示原始 Markdown 而非渲染结果', async () => {
    const w = await mountEditor('# hi\n\n- a\n- b')
    const ta = textareaOf(w)
    expect(ta.tagName).toBe('TEXTAREA')
    expect(ta.value).toBe('# hi\n\n- a\n- b')
    w.unmount()
  })

  it('输入触发 change 且内容保持源码', async () => {
    const w = await mountEditor('# hi')
    await w.find('[data-test="md-source"]').setValue('## hello')
    await new Promise((r) => setTimeout(r, 0))
    const emitted = w.emitted('change')
    expect(emitted).toBeTruthy()
    expect(emitted![emitted!.length - 1][0]).toBe('## hello')
    expect(textareaOf(w).value).toBe('## hello')
    w.unmount()
  })

  it('标题按钮在光标所在行首插入前缀', async () => {
    const w = await mountEditor('a\nb\nc')
    const ta = textareaOf(w)
    ta.setSelectionRange(3, 3) // 第二行行首
    await w.find('[data-test="tb-h2"]').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(ta.value).toBe('a\n## b\nc')
    w.unmount()
  })

  it('粗体按钮在光标处插入 Markdown 标记', async () => {
    const w = await mountEditor('text')
    const ta = textareaOf(w)
    ta.setSelectionRange(2, 2)
    await w.find('[data-test="tb-bold"]').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(ta.value).toBe('te****xt')
    w.unmount()
  })

  it('无序列表按钮在行首插入 - ', async () => {
    const w = await mountEditor('a\nb\nc')
    const ta = textareaOf(w)
    ta.setSelectionRange(4, 4) // 第三行行首
    await w.find('[data-test="tb-ul"]').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(ta.value).toBe('a\nb\n- c')
    w.unmount()
  })

  it('图片按钮触发受控上传并插入 Markdown 引用', async () => {
    const w = await mountEditor('')
    const input = w.find('input[type="file"]').element as HTMLInputElement
    Object.defineProperty(input, 'files', {
      value: [new File(['x'], 'pic.png', { type: 'image/png' })],
      configurable: true,
    })
    await w.find('input[type="file"]').trigger('change')
    await new Promise((r) => setTimeout(r, 0))
    expect(textareaOf(w).value).toContain('![pic](/v1/attachments/x/raw)')
    w.unmount()
  })

  it('[[ 补全：输入触发浮层，点击选中项插入 wikilink', async () => {
    const w = await mountEditor('', ['Hello World', 'Other'])
    await w.find('[data-test="md-source"]').setValue('[[Hel')
    textareaOf(w).setSelectionRange(5, 5)
    await w.find('[data-test="md-source"]').trigger('input')
    expect(w.find('[data-test="wikilink-suggest"]').exists()).toBe(true)
    await w.findAll('[data-test="suggest-item"]')[0].trigger('mousedown')
    await new Promise((r) => setTimeout(r, 0))
    expect(textareaOf(w).value).toBe('[[Hello World]] ')
    w.unmount()
  })

  it('链接弹窗：输入 URL 应用后插入链接 Markdown', async () => {
    const w = await mountEditor('')
    await w.find('[data-test="tb-link"]').trigger('click')
    expect(w.find('[data-test="link-dialog"]').exists()).toBe(true)
    await w.find('[data-test="link-url-input"]').setValue('https://example.com')
    await w.find('[data-test="link-apply"]').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(textareaOf(w).value).toContain('[link text](https://example.com)')
    w.unmount()
  })
})
