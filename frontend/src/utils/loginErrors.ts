// OIDC 回调错误码 → i18n key（与后端 redirect reason 对齐；文案见 auth.errors.*）。
const KEYS: Record<string, string> = {
  state_mismatch: 'auth.errors.state_mismatch',
  nonce_mismatch: 'auth.errors.nonce_mismatch',
  token_invalid: 'auth.errors.token_invalid',
  provider_unavailable: 'auth.errors.provider_unavailable',
  exchange_failed: 'auth.errors.exchange_failed',
  account_disabled: 'auth.errors.account_disabled',
  missing_code: 'auth.errors.missing_code',
  provision_failed: 'auth.errors.provision_failed',
}

export function loginErrorKey(reason?: string | null): string | null {
  if (!reason) return null
  return KEYS[reason] ?? 'auth.loginFailed'
}
