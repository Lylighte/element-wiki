// T7.2 验收：wrapper 精确断言 method/path/params/body。
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { client } from '@/api/client'
import {
  attachmentApi,
  authApi,
  commentApi,
  docApi,
  searchApi,
  tokenApi,
} from '@/api'

type Recorded = { method?: string; url?: string; data?: unknown; params?: unknown }
let captured: Recorded[] = []

beforeEach(() => {
  captured = []
  client.defaults.adapter = async (config) => {
    captured.push({
      method: config.method?.toUpperCase(),
      url: config.url,
      data: config.data,
      params: config.params,
    })
    return {
      data: {},
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    }
  }
})

afterEach(() => {
  vi.restoreAllMocks()
})

function bodyOf(r: Recorded): any {
  return typeof r.data === 'string' ? JSON.parse(r.data) : r.data
}

function last(): Recorded {
  expect(captured.length).toBeGreaterThan(0)
  return captured[captured.length - 1]
}

describe('api wrappers', () => {
  it('auth.me → GET /users/me', async () => {
    await authApi.me()
    const r = last()
    expect(r.method).toBe('GET')
    expect(r.url).toBe('/users/me')
  })

  it('doc.create → POST /documents with snake_case body', async () => {
    await docApi.create({ slug: 'hello', title: 'Hello', parent_id: null })
    const r = last()
    expect(r.method).toBe('POST')
    expect(r.url).toBe('/documents')
    expect(bodyOf(r)).toEqual({ slug: 'hello', title: 'Hello', parent_id: null })
  })

  it('doc.patch → PATCH /documents/{id} 仅透传给定字段', async () => {
    await docApi.patch('d1', { title: 'T', visibility: 'restricted' })
    const r = last()
    expect(r.method).toBe('PATCH')
    expect(r.url).toBe('/documents/d1')
    expect(bodyOf(r)).toEqual({ title: 'T', visibility: 'restricted' })
  })

  it('doc.commit body 使用 base_commit_id/content/message', async () => {
    await docApi.commit('d1', 'base0', '# hi', 'msg')
    const r = last()
    expect(r.method).toBe('POST')
    expect(r.url).toBe('/documents/d1/commits')
    expect(bodyOf(r)).toEqual({ base_commit_id: 'base0', content: '# hi', message: 'msg' })
  })

  it('doc.saveDraft → PUT /documents/{id}/draft', async () => {
    await docApi.saveDraft('d1', 'b1', 'wip')
    const r = last()
    expect(r.method).toBe('PUT')
    expect(r.url).toBe('/documents/d1/draft')
    expect(bodyOf(r)).toEqual({ base_commit_id: 'b1', content: 'wip' })
  })

  it('search.query 以 params 传递 q/limit', async () => {
    await searchApi.query('goldmark', 5)
    const r = last()
    expect(r.method).toBe('GET')
    expect(r.params).toEqual({ q: 'goldmark', limit: 5 })
  })

  it('comment.add / remove 路径与 body 正确', async () => {
    await commentApi.add('d9', 'hello')
    expect(last()).toMatchObject({ method: 'POST', url: '/documents/d9/comments' })
    await commentApi.remove('c1')
    expect(last()).toMatchObject({ method: 'DELETE', url: '/comments/c1' })
  })

  it('attachment.rawURL 不发请求，直接拼接路径', () => {
    expect(attachmentApi.rawURL('a7')).toBe('/v1/attachments/a7/raw')
  })

  it('token.create body 为 {name}', async () => {
    await tokenApi.create('ci')
    expect(bodyOf(last())).toEqual({ name: 'ci' })
  })

  it('auth.loginUrl 对 redirect 进行编码', () => {
    expect(authApi.loginUrl('/a b')).toContain('redirect=%2Fa%20b')
  })
})
