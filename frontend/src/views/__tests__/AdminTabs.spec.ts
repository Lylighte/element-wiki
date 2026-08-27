// T7.8 验收：Tab 按权限码显隐。
import { afterEach, describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import i18n from '@/i18n'
import AdminTabs from '@/components/admin/AdminTabs.vue'
import ElementPlus from 'element-plus'

const perm = (codes: string[]) => ({ has: (c: string) => codes.includes(c) })

describe('AdminTabs visibility', () => {
  afterEach(() => window.history.replaceState({}, '', '/'))

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
    // i18n 化后无权限兜底文案走 common.notFound
    expect(w.text()).toContain(i18n.global.t('common.notFound'))
  })

  it('从 URL 恢复 Tab，切换后写回 URL', async () => {
    window.history.replaceState({}, '', '/admin?tab=backups')
    const w = mount(AdminTabs, {
      global: { plugins: [i18n, ElementPlus] },
      props: { perm: perm(['settings.manage', 'backup.manage']) },
    })
    await new Promise((r) => setTimeout(r, 0))
    expect(w.find('[data-test="tab-backups"]').exists()).toBe(true)

    await w.findAll('[role="tab"]')[0].trigger('click')
    expect(new URLSearchParams(window.location.search).get('tab')).toBe('settings')
  })
})
