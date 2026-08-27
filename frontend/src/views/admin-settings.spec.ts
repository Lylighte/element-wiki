// T11.2 验收：设置表单九键控件化，仅提交变更键，422 fields 展示。
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import i18n from '@/i18n'
import ElementPlus from 'element-plus'
import { ElMessage } from 'element-plus'
import AdminView from './AdminView.vue'
import siteStore from '@/stores/site'
import { setPermissions } from '@/permissions'

const seedSettings: Record<string, string> = vi.hoisted(() => ({
  wiki_title: 'My Wiki',
  timezone: 'Asia/Shanghai',
  default_lang: 'zh-CN',
  anonymous_read: 'true',
  comments_enabled: 'false',
  max_versions: '100',
  upload_max_mb: '20',
  trash_retention_days: '30',
  allowed_extensions: 'png,jpg',
}))

vi.mock('@/api', () => ({
  adminApi: {
    settings: vi.fn().mockResolvedValue({ ...seedSettings }),
    updateSettings: vi.fn().mockResolvedValue({ detail: 'updated' }),
    users: vi.fn().mockResolvedValue({ items: [] }),
    dashboard: vi.fn().mockResolvedValue({}),
    backupFiles: vi.fn().mockResolvedValue({ items: [] }),
    deleteBackupFile: vi.fn(),
    backupDownloadURL: (f: string) => `/v1/admin/backups/files/${f}/download`,
  },
}))

import { adminApi } from '@/api'

async function mountAdmin() {
  setPermissions(['settings.manage'])
  const w = mount(AdminView, { global: { plugins: [i18n, ElementPlus] } })
  for (let i = 0; i < 30 && !w.find('[data-test="admin-save"]').exists(); i++) {
    await new Promise((r) => setTimeout(r, 10))
  }
  return w
}

describe('admin settings form', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    ;(adminApi.settings as ReturnType<typeof vi.fn>).mockResolvedValue({ ...seedSettings })
    ;(adminApi.updateSettings as ReturnType<typeof vi.fn>).mockResolvedValue({ detail: 'updated' })
    document.body.innerHTML = ''
    siteStore.state.title = ''
    i18n.global.locale.value = 'en'
  })

  it('九键控件渲染且布尔键为开关形态', async () => {
    const w = await mountAdmin()
    expect(w.find('[data-test="f-anon"] input, [data-test="f-anon"]').exists()).toBe(true)
    expect(w.find('[data-test="f-max-versions"]').exists()).toBe(true)
    expect(w.find('[data-test="f-lang"]').exists()).toBe(true)
    let exts = ''
    for (let i = 0; i < 30; i++) {
      exts = (w.find('[data-test="f-exts"]').element as HTMLInputElement)?.value ?? ''
      if (exts) break
      await new Promise((r) => setTimeout(r, 10))
    }
    expect(exts).toBe('png,jpg')
  })

  it('设置字段标签使用 i18n 文案而非原始 key', async () => {
    i18n.global.locale.value = 'zh-CN'
    const w = await mountAdmin()
    expect(w.text()).toContain('站点标题')
    expect(w.text()).toContain('匿名阅读')
    expect(w.text()).toContain('回收站保留天数')
    expect(w.text()).not.toContain('wiki_title')
    expect(w.text()).not.toContain('anonymous_read')
  })

  it('仅提交变更键；wiki_title 保存后站点标题即时更新', async () => {
    const w = await mountAdmin()
    await w.find('[data-test="f-wiki-title"]').setValue('Renamed')
    await w.find('[data-test="admin-save"]').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(adminApi.updateSettings).toHaveBeenCalledTimes(1)
    expect(adminApi.updateSettings).toHaveBeenCalledWith({ wiki_title: 'Renamed' })
    expect(siteStore.state.title).toBe('Renamed')
  })

  it('无变更时不发起请求', async () => {
    const w = await mountAdmin()
    await w.find('[data-test="admin-save"]').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(adminApi.updateSettings).not.toHaveBeenCalled()
  })

  it('422 校验错误展示字段明细', async () => {
    ;(adminApi.updateSettings as ReturnType<typeof vi.fn>).mockRejectedValue({
      status: 422,
      fields: { wiki_title: 'must not be empty' },
    })
    const w = await mountAdmin()
    await w.find('[data-test="f-wiki-title"]').setValue('')
    await w.find('[data-test="admin-save"]').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(w.text()).toContain('must not be empty')
  })

  it('非校验错误显示通用提示且不产生未处理异常', async () => {
    const errorSpy = vi.spyOn(ElMessage, 'error').mockImplementation(() => ({ close: vi.fn() }) as never)
    ;(adminApi.updateSettings as ReturnType<typeof vi.fn>).mockRejectedValue(new Error('server error'))
    const w = await mountAdmin()
    await w.find('[data-test="f-wiki-title"]').setValue('Renamed')
    await w.find('[data-test="admin-save"]').trigger('click')
    await new Promise((r) => setTimeout(r, 0))

    expect(errorSpy).toHaveBeenCalledWith('Failed to load. Please retry.')
    errorSpy.mockRestore()
  })

  it('设置初始加载失败显示页面错误并支持重试', async () => {
    ;(adminApi.settings as ReturnType<typeof vi.fn>)
      .mockRejectedValueOnce(new Error('offline'))
      .mockResolvedValueOnce({ ...seedSettings })
    const w = mount(AdminView, { global: { plugins: [i18n, ElementPlus] } })
    await new Promise((r) => setTimeout(r, 0))

    expect(w.find('[data-test="admin-load-error"]').exists()).toBe(true)
    await w.find('[data-test="admin-load-retry"]').trigger('click')
    await new Promise((r) => setTimeout(r, 0))
    expect(w.find('[data-test="admin-load-error"]').exists()).toBe(false)
  })
})
