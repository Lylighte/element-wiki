// T10 验收：源码中静态引用的 t() key 必须同时存在于 zh-CN 与 en。
import { describe, expect, it } from 'vitest'
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join } from 'node:path'
import zh from '@/i18n/locales/zh-CN.json'
import en from '@/i18n/locales/en.json'

const srcRoot = join(process.cwd(), 'src')

function collectFiles(dir: string): string[] {
  const out: string[] = []
  for (const name of readdirSync(dir)) {
    const p = `${dir}/${name}`
    if (statSync(p).isDirectory()) {
      if (['node_modules', 'dist', 'test-results'].includes(name)) continue
      out.push(...collectFiles(p))
    } else if (/\.(ts|vue)$/.test(name)) {
      out.push(p)
    }
  }
  return out
}

function keyExists(obj: Record<string, unknown>, key: string): boolean {
  let cur: unknown = obj
  for (const part of key.split('.')) {
    if (!cur || typeof cur !== 'object' || !(part in (cur as Record<string, unknown>))) return false
    cur = (cur as Record<string, unknown>)[part]
  }
  return true
}

describe('i18n referenced keys', () => {
  it('源码静态引用的 t() key 在 zh-CN 与 en 中都存在', () => {
    const missing: string[] = []
    const re = /\bt\((['"])([^'"]+)\1\)/g
    for (const file of collectFiles(srcRoot)) {
      const src = readFileSync(file, 'utf8')
      let m: RegExpExecArray | null
      while ((m = re.exec(src))) {
        const key = m[2]
        if (!keyExists(zh, key)) missing.push(`${file}: 缺少 zh-CN key '${key}'`)
        if (!keyExists(en, key)) missing.push(`${file}: 缺少 en key '${key}'`)
      }
    }
    expect(missing).toEqual([])
  })
})
