// T7.3 验收：回调 reason → 用户文案映射。
import { describe, expect, it } from 'vitest'
import { loginErrorText } from './loginErrors'

describe('loginErrorText', () => {
  it('已知 reason 返回中文文案', () => {
    expect(loginErrorText('state_mismatch')).toContain('状态校验失败')
    expect(loginErrorText('account_disabled')).toContain('禁用')
  })
  it('未知 reason 兜底', () => {
    expect(loginErrorText('weird')).toBe('登录失败，请重试')
  })
  it('无 reason 返回 null', () => {
    expect(loginErrorText(null)).toBeNull()
    expect(loginErrorText('')).toBeNull()
  })
})
