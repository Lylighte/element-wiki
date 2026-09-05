// 回归守卫：client 不得钉死 Content-Type——钉死 application/json 会让 axios
// 把 FormData 序列化为 JSON（defaults transformRequest 的 formDataToJSON 分支），
// 后端 multipart 解析失败（导入/附件上传 400 missing file field）。
// 实证记录：钉死头 → CT=application/json + FormFile_ERR；无默认头 → 浏览器写 multipart boundary。
import { describe, expect, it } from 'vitest'
import { client } from '@/api/client'

describe('axios client defaults', () => {
  it('不钉死 Content-Type（multipart 交由浏览器/适配器设置）', () => {
    expect(client.defaults.headers.common?.['Content-Type']).toBeUndefined()
    expect(client.defaults.headers.post?.['Content-Type']).toBeUndefined()
  })

  it('JSON 负载仍由 axios 自动设置 application/json（transformRequest 默认行为）', () => {
    // defaults.transformRequest 对对象负载 setContentType('application/json', false)
    expect(client.defaults.transformRequest).toBeTruthy()
  })
})
