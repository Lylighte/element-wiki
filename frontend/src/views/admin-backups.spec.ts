// T12.1 验收：发起备份轮询到终态并刷新产物列表；导入确认 + 进度轮询。
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import i18n from '@/i18n'
import ElementPlus, { ElMessage, ElMessageBox } from 'element-plus'
import AdminView from './AdminView.vue'
import { setPermissions } from '@/permissions'

const state = vi.hoisted(() => ({
  settings: {} as Record<string, string>,
  files: [] as string[],
}))

vi.mock('@/api', () => ({
  adminApi: {
    settings: vi.fn().mockResolvedValue(state.settings),
    updateSettings: vi.fn(),
    users: vi.fn().mockResolvedValue({ items: [] }),
    dashboard: vi.fn().mockResolvedValue({}),
    backupFiles: vi.fn(() => Promise.resolve({ items: [...state.files] })),
    deleteBackupFile: vi.fn(),
    backupDownloadURL: (f: string) => `/v1/admin/backups/files/${f}/download`,
    startBackup: vi.fn().mockResolvedValue({ job_id: 'j-export' }),
    backupJob: vi.fn()
      .mockResolvedValueOnce({ job_id: 'j-export', status: 'pending' })
      .mockResolvedValueOnce({ job_id: 'j-export', status: 'done', filename: 'b.zip' }),
    importBackup: vi.fn().mockResolvedValue({ job_id: 'j-import' }),
    markdownImport: vi.fn().mockResolvedValue({ job_id: 'j-md' }),
    importJob: vi.fn()
      .mockResolvedValueOnce({ job_id: 'j-import', status: 'running', total_files: 3, imported_files: 1, failed_files: 0 })
      .mockResolvedValue({ job_id: 'j-import', status: 'done', total_files: 3, imported_files: 3, failed_files: 0 }),
  },
}))

import { adminApi } from '@/api'

async function mountBackups() {
  setPermissions(['backup.manage'])
  const w = mount(AdminView, { global: { plugins: [i18n, ElementPlus] }, attachTo: document.body })
  for (let i = 0; i < 30 && !w.find('[data-test="btn-start-backup"]').exists(); i++) {
    await new Promise((r) => setTimeout(r, 10))
  }
  return w
}

describe('admin backups tab', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    state.files = []
    document.body.innerHTML = ''
  })

  it('发起备份 → 轮询至 done → 产物列表刷新', async () => {
    const w = await mountBackups()
    state.files = ['element-wiki-2026.zip']
    await w.find('[data-test="btn-start-backup"]').trigger('click')
    for (let i = 0; i < 30 && !w.text().includes('element-wiki-2026.zip'); i++) {
      await new Promise((r) => setTimeout(r, 20))
      ;(w.element as HTMLElement).dispatchEvent(new Event('refresh'))
    }
    expect(adminApi.backupJob).toHaveBeenCalledWith('j-export')
    expect(w.find('[data-test="backup-download"]').exists()).toBe(true)
    w.unmount()
  })

  it('备份导入：确认后轮询 backups jobs 端点到 done（404 修复）', async () => {
    const confirmSpy = vi.spyOn(ElMessageBox, 'confirm').mockResolvedValue('ok' as never)
    const ok = vi.spyOn(ElMessage, 'success').mockImplementation((() => ({})) as never)
    // 备份导入 job 落在 backup_jobs：必须轮询 backupJob（imports jobs 端点读 import_jobs 会 404）
    ;(adminApi.backupJob as ReturnType<typeof vi.fn>)
      .mockReset()
      .mockResolvedValueOnce({ job_id: 'j-import', status: 'running', last_error: '' })
      .mockResolvedValueOnce({ job_id: 'j-import', status: 'done', filename: 'b.zip' })
    const w = await mountBackups()
    // 直接触发内部流程：模拟文件选择回调
    const vm = w.vm as unknown as { importBackupZip: (f: File) => Promise<void> }
    await vm.importBackupZip(new File(['x'], 'b.zip'))
    expect(adminApi.importBackup).toHaveBeenCalledTimes(1)
    expect(adminApi.backupJob).toHaveBeenCalledWith('j-import')
    expect(adminApi.importJob).not.toHaveBeenCalled()
    expect(ok).toHaveBeenCalled()
    ok.mockRestore()
    confirmSpy.mockRestore()
    w.unmount()
  })

  it('用户取消确认 → 不发起导入', async () => {
    vi.spyOn(ElMessageBox, 'confirm').mockRejectedValue('cancel')
    const w = await mountBackups()
    const vm = w.vm as unknown as { importBackupZip: (f: File) => Promise<void> }
    await vm.importBackupZip(new File(['x'], 'b.zip'))
    expect(adminApi.importBackup).not.toHaveBeenCalled()
    w.unmount()
  })

  it('Markdown zip 导入：确认后走 markdownImport（隔离根，T17.3）', async () => {
    const confirmSpy = vi.spyOn(ElMessageBox, 'confirm').mockResolvedValue('ok' as never)
    vi.spyOn(ElMessage, 'success').mockImplementation((() => ({})) as never)
    const w = await mountBackups()
    const vm = w.vm as unknown as { importMarkdownZip: (f: File) => Promise<void> }
    await vm.importMarkdownZip(new File(['x'], 'content.zip'))
    expect(confirmSpy).toHaveBeenCalledTimes(1)
    expect(adminApi.markdownImport).toHaveBeenCalledTimes(1)
    confirmSpy.mockRestore()
    w.unmount()
  })
})
