// T7.7 验收：comments_enabled=false 时评论区整体隐藏（门闩契约）。
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import CommentsPanel from '@/components/doc/CommentsPanel.vue'
import { commentApi } from '@/api'
import i18n from '@/i18n'

describe('CommentsPanel gate', () => {
  const listMock = vi.fn()

  beforeEach(() => {
    ;(commentApi as unknown as { list: unknown }).list = listMock
    listMock.mockReset()
  })

  it('正常渲染列表', async () => {
    listMock.mockResolvedValue({
      items: [
        { id: 'c1', document_id: 'd', author_id: 'u', content: 'hello', created_at: 1 },
      ],
    })
    const w = mount(CommentsPanel, {
      global: { plugins: [i18n] },
      props: { docID: 'd1', me: 'u', isAdmin: false },
    })
    await new Promise((r) => setTimeout(r, 0))
    expect(w.find('[data-test="comments-panel"]').exists()).toBe(true)
    expect(w.text()).toContain('hello')
  })

  it('403 comments disabled → 整体隐藏', async () => {
    listMock.mockRejectedValue({ response: { status: 403, data: { detail: 'comments disabled' } } })
    const w = mount(CommentsPanel, {
      global: { plugins: [i18n] },
      props: { docID: 'd1', me: 'u', isAdmin: false },
    })
    await new Promise((r) => setTimeout(r, 0))
    expect(listMock).toHaveBeenCalled()
    expect(w.find('[data-test="comments-panel"]').exists()).toBe(false)
  })
})
