// T7.3/T10.2 验收：回调 reason → i18n key 映射。
import { describe, expect, it } from 'vitest'
import { loginErrorKey } from './loginErrors'
import messages from '@/i18n/locales/zh-CN.json'

describe('loginErrorKey', () => {
  it('已知 reason 映射到存在的 locale key', () => {
    for (const reason of ['state_mismatch', 'account_disabled', 'provision_failed']) {
      const key = loginErrorKey(reason)!
      expect(key).toMatch(/^auth\./)
      // key 必须在两份语言资源中都存在
      let node: unknown = messages
      for (const part of key.split('.')) {
        node = (node as Record<string, unknown>)[part]
        expect(node).toBeDefined()
      }
    }
  })
  it('未知 reason 兜底到 auth.loginFailed', () => {
    expect(loginErrorKey('weird')).toBe('auth.loginFailed')
    expect(messages.auth.loginFailed).toBeDefined()
  })
  it('无 reason 返回 null', () => {
    expect(loginErrorKey(null)).toBeNull()
    expect(loginErrorKey('')).toBeNull()
    expect(loginErrorKey(undefined)).toBeNull()
  })
})
