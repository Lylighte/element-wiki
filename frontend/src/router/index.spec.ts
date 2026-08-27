import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/api', () => ({
  authApi: { me: vi.fn() },
}))

import { authApi } from '@/api'
import router from './index'
import authStore from '@/stores/auth'

describe('route access control', () => {
  beforeEach(async () => {
    authStore.state.me = null
    authStore.state.initialized = false
    authStore.state.loading = false
    authStore.state.error = null
    vi.mocked(authApi.me).mockReset()
    vi.mocked(authApi.me).mockRejectedValue(new Error('401'))
    await router.push('/')
    authStore.reset()
  })

  it('未登录访问受保护页面 → 登录并保留目标', async () => {
    await router.push('/settings/tokens')

    expect(router.currentRoute.value.name).toBe('login')
    expect(router.currentRoute.value.query.redirect).toBe('/settings/tokens')
  })

  it('已登录但无权限 → 禁止访问页', async () => {
    vi.mocked(authApi.me).mockResolvedValue({ permissions: [] } as never)
    await router.push('/settings/tokens')

    expect(router.currentRoute.value.name).toBe('forbidden')
  })

  it('已登录且具备权限 → 进入令牌页', async () => {
    vi.mocked(authApi.me).mockResolvedValue({ permissions: ['token.manage.own'] } as never)
    await router.push('/settings/tokens')

    expect(router.currentRoute.value.name).toBe('tokens')
  })

  it('未知路径 → Not Found 页面', async () => {
    await router.push('/does-not-exist')

    expect(router.currentRoute.value.name).toBe('not-found')
  })
})
