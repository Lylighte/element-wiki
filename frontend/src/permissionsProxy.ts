// 供组件模板使用的权限判断代理（避免在模板中直接 import 复杂逻辑）。
import { can } from '@/permissions'
export const permission = { has: can }
