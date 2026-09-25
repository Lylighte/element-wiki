// T7.7 验收：comments_enabled=false 时评论区整体隐藏（门闩契约）；
// 站点信息已加载且关闭时直接不发请求（消除必现 403）。
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import CommentsPanel from '@/components/doc/CommentsPanel.vue'
import { commentApi } from '@/api'
import siteStore from '@/stores/site'
import i18n from '@/i18n'

describe('CommentsPanel gate', () => {
  const listMock = vi.fn()

  beforeEach(() => {
    ;(commentApi as unknown as { list: unknown }).list = listMock
    listMock.mockReset()
    siteStore.state.commentsEnabled = null
    siteStore.state.timezone = 'UTC'
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

  it('评论时间跟随站点时区即时变化', async () => {
    listMock.mockResolvedValue({
      items: [{ id: 'c1', document_id: 'd', author_id: 'u', content: 'hello', created_at: Date.UTC(2025, 0, 1, 0, 30) }],
    })
    siteStore.setTimezone('Asia/Shanghai')
    const w = mount(CommentsPanel, {
      global: { plugins: [i18n] },
      props: { docID: 'd1', me: 'u', isAdmin: false },
    })
    await flushPromises()
    expect(w.text()).toContain('08:30 (Asia/Shanghai)')
    siteStore.setTimezone('Europe/Berlin')
    await w.vm.$nextTick()
    expect(w.text()).toContain('01:30 (Europe/Berlin)')
  })

  it('403 comments disabled → 整体隐藏', async () => {
    listMock.mockRejectedValue(Object.assign(new Error('comments disabled'), { status: 403 }))
    const w = mount(CommentsPanel, {
      global: { plugins: [i18n] },
      props: { docID: 'd1', me: 'u', isAdmin: false },
    })
    await new Promise((r) => setTimeout(r, 0))
    expect(listMock).toHaveBeenCalled()
    expect(w.find('[data-test="comments-panel"]').exists()).toBe(false)
  })

  it('站点信息已加载且 comments_enabled=false → 不发请求直接隐藏', async () => {
    siteStore.setCommentsEnabled(false)
    const w = mount(CommentsPanel, {
      global: { plugins: [i18n] },
      props: { docID: 'd1', me: 'u', isAdmin: false },
    })
    await new Promise((r) => setTimeout(r, 0))
    expect(listMock).not.toHaveBeenCalled()
    expect(w.find('[data-test="comments-panel"]').exists()).toBe(false)
  })

  it('站点信息已加载且 comments_enabled=true → 照常请求渲染', async () => {
    siteStore.setCommentsEnabled(true)
    listMock.mockResolvedValue({ items: [] })
    const w = mount(CommentsPanel, {
      global: { plugins: [i18n] },
      props: { docID: 'd1', me: 'u', isAdmin: false },
    })
    await new Promise((r) => setTimeout(r, 0))
    expect(listMock).toHaveBeenCalledTimes(1)
    expect(w.find('[data-test="comments-panel"]').exists()).toBe(true)
  })

  it('网络错误显示重试并可恢复', async () => {
    listMock.mockRejectedValueOnce(new Error('offline')).mockResolvedValueOnce({ items: [] })
    const w = mount(CommentsPanel, {
      global: { plugins: [i18n] },
      props: { docID: 'd1', me: 'u', isAdmin: false },
    })
    await new Promise((r) => setTimeout(r, 0))

    expect(w.find('[data-test="comments-error"]').exists()).toBe(true)
    await w.find('[data-test="comments-retry"]').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(w.find('[data-test="comments-error"]').exists()).toBe(false)
  })
})
