import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { ElMessage, ElMessageBox } from 'element-plus'
import i18n from '@/i18n'
import { trashApi } from '@/api'
import TrashView from './TrashView.vue'

vi.mock('@/api', () => ({
  trashApi: {
    list: vi.fn(),
    restore: vi.fn(),
    purge: vi.fn(),
  },
}))

const parent = { id: 'parent', parent_id: null, title: 'Parent' }
const child = { id: 'child', parent_id: 'parent', title: 'Child' }
const grandchild = { id: 'grandchild', parent_id: 'child', title: 'Grandchild' }

describe('TrashView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.mocked(trashApi.list).mockResolvedValue({ items: [parent, child, grandchild] } as any)
  })

  it('shows the full subtree impact and only purges after confirmation', async () => {
    const confirm = vi.spyOn(ElMessageBox, 'confirm').mockResolvedValue('confirm' as never)
    const wrapper = mount(TrashView, { global: { plugins: [i18n] } })
    await flushPromises()

    expect(wrapper.findAll('[data-test="trash-subtree-count"]')[0].text()).toContain('2')
    expect(wrapper.findAll('[data-test="trash-subtree-child"]')).toHaveLength(2)
    await wrapper.findAll('[data-test="trash-item"]')[0].findAll('button')[1].trigger('click')
    await flushPromises()

    expect(confirm).toHaveBeenCalledWith(expect.stringContaining('2'), { type: 'warning' })
    expect(trashApi.purge).toHaveBeenCalledOnce()
    expect(trashApi.purge).toHaveBeenCalledWith('parent')
    confirm.mockRestore()
  })

  it('keeps the list and sends no purge request when cancelled', async () => {
    const confirm = vi.spyOn(ElMessageBox, 'confirm').mockRejectedValue('cancel')
    const wrapper = mount(TrashView, { global: { plugins: [i18n] } })
    await flushPromises()
    await wrapper.findAll('[data-test="trash-item"]')[0].findAll('button')[1].trigger('click')
    await flushPromises()

    expect(trashApi.purge).not.toHaveBeenCalled()
    expect(wrapper.findAll('[data-test="trash-item"]')).toHaveLength(3)
    confirm.mockRestore()
  })

  it('reports a purge failure without leaving a rejected promise', async () => {
    const confirm = vi.spyOn(ElMessageBox, 'confirm').mockResolvedValue('confirm' as never)
    const message = vi.spyOn(ElMessage, 'error').mockImplementation(() => (() => {}) as any)
    vi.mocked(trashApi.purge).mockRejectedValueOnce(new Error('network'))
    const wrapper = mount(TrashView, { global: { plugins: [i18n] } })
    await flushPromises()
    await wrapper.findAll('[data-test="trash-item"]')[0].findAll('button')[1].trigger('click')
    await flushPromises()

    expect(message).toHaveBeenCalledWith(i18n.global.t('trash.purgeFailed'))
    expect(wrapper.findAll('[data-test="trash-item"]')).toHaveLength(3)
    confirm.mockRestore()
    message.mockRestore()
  })
})
