// T7.1 验收：两语言资源 key 完整性（双向深度对比）。
import { describe, expect, it } from 'vitest'
import zh from '@/i18n/locales/zh-CN.json'
import en from '@/i18n/locales/en.json'

function keyPaths(obj: Record<string, unknown>, prefix = ''): string[] {
  const out: string[] = []
  for (const [k, v] of Object.entries(obj)) {
    const path = prefix ? `${prefix}.${k}` : k
    if (v && typeof v === 'object') out.push(...keyPaths(v as Record<string, unknown>, path))
    else out.push(path)
  }
  return out.sort()
}

describe('i18n locale parity', () => {
  it('zh-CN 与 en 的 key 集合完全一致', () => {
    const zhKeys = keyPaths(zh)
    const enKeys = keyPaths(en)
    expect(zhKeys).toEqual(enKeys)
  })
})
