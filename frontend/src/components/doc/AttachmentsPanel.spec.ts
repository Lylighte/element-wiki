import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import i18n from '@/i18n'
import { attachmentApi } from '@/api'
import AttachmentsPanel from './AttachmentsPanel.vue'

vi.mock('@/api', () => ({
  attachmentApi: {
    list: vi.fn(),
    upload: vi.fn(),
    remove: vi.fn(),
    rawURL: vi.fn((id: string) => `/v1/attachments/${id}/raw`),
  },
}))

describe('AttachmentsPanel error boundary', () => {
  beforeEach(() => {
    vi.mocked(attachmentApi.upload).mockReset()
    vi.mocked(attachmentApi.remove).mockReset()
    vi.mocked(attachmentApi.list).mockResolvedValue({ items: [] })
  })

  it('加载失败时显示重试并可恢复', async () => {
    vi.mocked(attachmentApi.list).mockRejectedValueOnce(new Error('offline'))
    const wrapper = mount(AttachmentsPanel, {
      global: { plugins: [i18n] },
      props: { docID: 'd1', editable: false },
    })
    await flushPromises()

    expect(wrapper.find('[data-test="attachments-error"]').exists()).toBe(true)
    await wrapper.find('[data-test="attachments-retry"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="attachments-error"]').exists()).toBe(false)
  })

  it('上传失败时显示操作错误且不产生未处理异常', async () => {
    vi.mocked(attachmentApi.upload).mockRejectedValueOnce(new Error('rejected'))
    const wrapper = mount(AttachmentsPanel, {
      global: { plugins: [i18n] },
      props: { docID: 'd1', editable: true },
    })
    await flushPromises()

    const input = wrapper.find('input[type="file"]')
    const file = new File(['data'], 'note.txt', { type: 'text/plain' })
    Object.defineProperty(input.element, 'files', { value: [file] })
    await input.trigger('change')
    await flushPromises()

    expect(wrapper.find('[data-test="attachments-action-error"]').exists()).toBe(true)
  })
})
