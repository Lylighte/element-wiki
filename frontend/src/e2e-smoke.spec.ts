// T7.8 冒烟（组件级 E2E 替代）：登录→建文→提交→搜索命中 的调用序列断言。
// 真浏览器 E2E 待 CI 注入 Playwright 后启用；本用例锁定调用编排契约。
import { describe, expect, it, beforeEach } from 'vitest'
import { client } from '@/api/client'
import { authApi } from '@/api'
import { docApi } from '@/api'
import { searchApi } from '@/api'

const calls: string[] = []

beforeEach(() => {
  calls.length = 0
  client.defaults.adapter = async (config) => {
    const key = `${config.method?.toUpperCase()} ${config.url}`
    calls.push(key)
    const respond = (data: unknown) => ({ data, status: 200, statusText: 'OK', headers: {}, config })
    if (key === 'GET /users/me')
      return respond({ user: { id: 'u1', email: '', display_name: '', role: 'editor', status: 'active' }, permissions: [] })
    if (key === 'POST /documents') return respond({ document: { id: 'd1' } })
    if (key === 'POST /documents/d1/commits')
      return respond({ commit: { id: 'c1', commit_no: 1 }, dead_links: [] })
    if (key === 'GET /search') return respond({ items: [{ document_id: 'd1' }], has_next: false, next_cursor: null, page_size: 1 })
    return respond({})
  }
})

describe('smoke flow', () => {
  it('login → create → commit → search 命中同一文档', async () => {
    await authApi.me()
    await docApi.create({ slug: 'smoke', title: 'Smoke' })
    await docApi.commit('d1', '', 'body', 'm')
    const r = await searchApi.query('body')

    expect(calls).toEqual([
      'GET /users/me',
      'POST /documents',
      'POST /documents/d1/commits',
      'GET /search',
    ])
    expect(r.items[0].document_id).toBe('d1')
  })
})
