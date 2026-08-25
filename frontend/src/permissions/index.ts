// 权限码常量与运行时判断（与 doc/02 §13 对齐；AGENTS §4）。
export const CODES = {
  document_read: 'document.read',
  document_read_restricted: 'document.read.restricted',
  document_create: 'document.create',
  document_update: 'document.update',
  document_delete: 'document.delete',
  document_restore: 'document.restore',
  version_read: 'version.read',
  version_revert: 'version.revert',
  attachment_read: 'attachment.read',
  attachment_upload: 'attachment.upload',
  attachment_delete: 'attachment.delete',
  comment_read: 'comment.read',
  comment_create: 'comment.create',
  comment_delete_own: 'comment.delete.own',
  comment_delete_any: 'comment.delete.any',
  user_list: 'user.list',
  user_manage: 'user.manage',
  settings_manage: 'settings.manage',
  dashboard_read: 'dashboard.read',
  backup_manage: 'backup.manage',
  import_run: 'import.run',
  search_rebuild: 'search.rebuild',
  token_manage_own: 'token.manage.own',
} as const

export type Code = (typeof CODES)[keyof typeof CODES]

import { ref } from 'vue'

// ref 包裹使依赖 can() 的 computed 在 setPermissions 后响应式更新
const current = ref<Set<string>>(new Set())

export function setPermissions(codes: string[]) {
  current.value = new Set(codes)
}

export function resetPermissions() {
  current.value = new Set()
}

export function can(code: string): boolean {
  return current.value.has(code)
}

/** 页面级访问控制统一入口（AGENTS §2：路由与导航复用同一套判断）。 */
export function requireAny(codes: string[]): boolean {
  return codes.some((c) => current.value.has(c))
}
