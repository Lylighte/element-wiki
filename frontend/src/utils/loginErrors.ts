// OIDC 回调错误码 → 用户可读文案（与后端 redirect reason 对齐）。
const MAP: Record<string, string> = {
  state_mismatch: '状态校验失败，请重试',
  nonce_mismatch: '安全校验失败（nonce），请重试',
  token_invalid: '令牌校验失败，请重试',
  provider_unavailable: '身份提供方暂不可用',
  exchange_failed: '授权码交换失败，请重试',
  account_disabled: '账号已被禁用，请联系管理员',
  missing_code: '回调缺少授权码',
  provision_failed: '账号开通失败，请联系管理员',
}

export function loginErrorText(reason?: string | null): string | null {
  if (!reason) return null
  return MAP[reason] ?? '登录失败，请重试'
}
