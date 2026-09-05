import axios from 'axios'

export interface Page<T> {
  items: T[]
  has_next: boolean
  next_cursor: string | null
  page_size: number
}

export const client = axios.create({
  baseURL: '/v1',
  withCredentials: true,
  // 不钉死 Content-Type：JSON 负载由 axios transformRequest 自动设置 application/json；
  // 钉死 JSON 会把 FormData 序列化为 JSON（defaults transformRequest 的 formDataToJSON 分支），
  // 导致后端 multipart 解析失败（导入/附件上传 400 missing file field）。
  // FormData 交由浏览器自动写入 multipart/form-data + boundary。
})

export interface ApiError extends Error {
  status?: number
  detail?: string
  fields?: Record<string, string>
}

/** 统一错误归一化：后端 {"detail", "fields"} → ApiError */
export function toApiError(err: unknown): ApiError {
  if (axios.isAxiosError(err)) {
    const e = new Error(err.message) as ApiError
    e.status = err.response?.status
    const data = err.response?.data as { detail?: string; fields?: Record<string, string> } | undefined
    e.detail = data?.detail
    e.fields = data?.fields
    return e
  }
  return err instanceof Error ? (err as ApiError) : new Error(String(err))
}

export async function get<T>(url: string, params?: Record<string, unknown>): Promise<T> {
  const res = await client.get<T>(url, { params })
  return res.data
}
export async function post<T>(url: string, body?: unknown): Promise<T> {
  const res = await client.post<T>(url, body ?? {})
  return res.data
}
export async function patch<T>(url: string, body: unknown): Promise<T> {
  const res = await client.patch<T>(url, body)
  return res.data
}
export async function put<T>(url: string, body: unknown): Promise<T> {
  const res = await client.put<T>(url, body)
  return res.data
}
export async function del(url: string): Promise<void> {
  await client.delete(url)
}
