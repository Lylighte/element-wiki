// T7.8 验收：Tab 按权限码显隐。
import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import i18n from '@/i18n'
import AdminTabs from '@/components/admin/AdminTabs.vue'
import ElementPlus from 'element-plus'

const perm = (codes: string[]) => ({ has: (c: string) => codes.includes(c) })

describe('AdminTabs visibility', () => {
  it('admin 全显', () => {
    const w = mount(AdminTabs, {
      global: { plugins: [i18n, ElementPlus] },
      props: { perm: perm(['settings.manage', 'user.list', 'dashboard.read', 'backup.manage']) },
    })
    expect(w.findAll('[role="tab"]').length).toBe(4)
  })
  it('仅 backup 权限只显示 backups', () => {
    const w = mount(AdminTabs, {
      global: { plugins: [i18n, ElementPlus] },
      props: { perm: perm(['backup.manage']) },
    })
    expect(w.findAll('[role="tab"]').length).toBe(1)
  })
  it('无权限显示 403', () => {
    const w = mount(AdminTabs, {
      global: { plugins: [i18n, ElementPlus] },
      props: { perm: perm([]) },
    })
    expect(w.text()).toContain('403')
  })
})
