// API wrapper 层：精确的 method/path/params/body 契约（AGENTS §9）。
// 全部经由 ./client 的 get/post/patch/put/del，测试通过 mock adapter 断言。
import { del, get, patch, post, put } from './client'

// ---- 类型（与后端契约一致，snake_case） ----
export interface User {
  id: string
  email: string
  display_name: string
  role: 'viewer' | 'editor' | 'admin'
  status: 'active' | 'disabled'
}

export interface MeResponse {
  user: User
  permissions: string[]
}

export interface DocumentMeta {
  id: string
  parent_id: string | null
  slug: string
  title: string
  sort_key: number
  visibility: 'standard' | 'restricted'
  head_commit_id: string
  created_at: number
  updated_at: number
  effective_visibility?: 'standard' | 'restricted'
}

export interface TreeNode {
  id: string
  parent_id: string | null
  title: string
  slug: string
  sort_key: number
  restricted: boolean
  children: TreeNode[]
}

export interface CommitView {
  id: string
  document_id: string
  commit_no: number
  parent_commit_id: string | null
  author_id: string
  message: string
  created_at: number
}

export interface DeadLink {
  target: string
  reason: string
}

export interface CommitResult {
  commit: CommitView
  dead_links: DeadLink[]
}

export interface Draft {
  document_id: string
  user_id: string
  base_commit_id: string
  content: string
  updated_at: number
}

export interface SearchHit {
  document_id: string
  title: string
  snippet: string
  score: number
}

export interface Attachment {
  id: string
  document_id: string
  filename: string
  mime_type: string
  size: number
  sha256: string
  uploaded_by: string
  created_at: number
}

export interface CommentItem {
  id: string
  document_id: string
  author_id: string
  content: string
  created_at: number
  mentions?: string[]
}

export interface ApiToken {
  id: string
  name: string
  prefix: string
  created_at: number
  last_used_at: number
  revoked_at: number | null
}

export interface TrashItem extends DocumentMeta {
  purge_at: number | null
}

// ---- auth ----
export const authApi = {
  status: () => get<{ enabled: boolean; provider_name?: string }>('/auth/oidc/status'),
  loginUrl(redirect: string) {
    return `/v1/auth/oidc/login?redirect=${encodeURIComponent(redirect)}`
  },
  logout: () => del('/auth/logout'),
  me: () => get<MeResponse>('/users/me'),
}

// ---- tokens ----
export const tokenApi = {
  list: () => get<{ items: ApiToken[] }>('/tokens'),
  create: (name: string) => post<{ id: string; name: string; prefix: string; token: string }>('/tokens', { name }),
  revoke: (id: string) => del(`/tokens/${id}`),
}

// ---- documents ----
export const docApi = {
  tree: () => get<{ nodes: TreeNode[] }>('/documents/tree'),
  create: (body: { parent_id?: string | null; slug: string; title: string }) =>
    post<{ document: DocumentMeta }>('/documents', body),
  get: (id: string) => get<{ document: DocumentMeta }>(`/documents/${id}`),
  patch: (
    id: string,
    body: Partial<{
      title: string
      slug: string
      visibility: 'standard' | 'restricted'
      parent_id: string | null
      sort_key: number
    }>,
  ) => patch<{ document: DocumentMeta }>(`/documents/${id}`, body),
  remove: (id: string) => del(`/documents/${id}`),

  saveDraft: (id: string, baseCommitID: string, content: string) =>
    put<void>(`/documents/${id}/draft`, { base_commit_id: baseCommitID, content }),
  getDraft: (id: string) =>
    get<{ draft: Draft | null }>(`/documents/${id}/draft`),
  deleteDraft: (id: string) => del(`/documents/${id}/draft`),

  commit: (id: string, baseCommitID: string, content: string, message?: string) =>
    post<CommitResult & { detail?: string; head_commit_id?: string; base_commit_id?: string }>(
      `/documents/${id}/commits`,
      { base_commit_id: baseCommitID, content, message },
    ),
  listCommits: (id: string, limit = 50) =>
    get<{ items: CommitView[] }>(`/documents/${id}/commits`, { limit }),
  revert: (id: string, commitID: string) =>
    post<CommitResult>(`/documents/${id}/revert`, { commit_id: commitID }),
  render: (id: string) =>
    get<{ html: string; title: string; toc: { level: number; text: string; id: string }[] }>(
      `/documents/${id}/render`,
    ),
  preview: (markdown: string) => post<{ html: string }>('/render-preview', { markdown }),
}

// ---- search ----
export const searchApi = {
  query: (q: string, limit = 20) =>
    get<{ items: SearchHit[]; has_next: boolean; next_cursor: null; page_size: number }>(
      '/search',
      { q, limit },
    ),
}

// ---- attachments ----
export const attachmentApi = {
  list: (docID: string) =>
    get<{ items: Attachment[] }>(`/documents/${docID}/attachments`),
  upload: async (docID: string, file: File) => {
    const fd = new FormData()
    fd.append('file', file)
    const res = await post<{ id: string; filename: string; size: number }>(
      `/documents/${docID}/attachments`,
      fd,
    )
    return res
  },
  remove: (id: string) => del(`/attachments/${id}`),
  rawURL: (id: string) => `/v1/attachments/${id}/raw`,
}

// ---- comments ----
export const commentApi = {
  list: (docID: string, limit = 50) =>
    get<{ items: CommentItem[] }>(`/documents/${docID}/comments`, { limit }),
  add: (docID: string, content: string) =>
    post<{ comment: CommentItem }>(`/documents/${docID}/comments`, { content }),
  remove: (id: string) => del(`/comments/${id}`),
}
