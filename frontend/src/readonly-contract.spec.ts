// T7.4 验收：只读页零编辑器加载 —— 源码契约测试。
// 1) DocView 不得引用编辑器模块；
// 2) EditView 必须通过动态 import 引入编辑器（形成独立 chunk）。
import { describe, expect, it } from 'vitest'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const root = resolve(__dirname, '..')

describe('readonly page must not load editor code', () => {
  const docView = readFileSync(resolve(root, 'src/views/DocView.vue'), 'utf8')
  it('DocView 不含 editor 导入', () => {
    expect(docView).not.toMatch(/editor/i)
  })

  it('EditView 通过动态 import 加载 EditorCanvas', () => {
    const edit = readFileSync(resolve(root, 'src/views/EditView.vue'), 'utf8')
    expect(edit).toMatch(/import\(['"]@\/components\/editor\/EditorCanvas\.vue['"]\)/)
    expect(edit).not.toMatch(/from ['"]@\/components\/editor/)
  })
})
