// useMediaQuery：matchMedia 可用时跟随断点变化；不可用时回退桌面（true）。
import { describe, expect, it, vi, beforeEach } from 'vitest'

type Listener = (e: { matches: boolean }) => void

function fakeMql(initial: boolean) {
  const listeners = new Set<Listener>()
  const mql = {
    matches: initial,
    addEventListener: vi.fn((_: string, fn: Listener) => listeners.add(fn)),
    removeEventListener: vi.fn((_: string, fn: Listener) => listeners.delete(fn)),
  }
  return {
    mql,
    fire(matches: boolean) {
      mql.matches = matches
      listeners.forEach((fn) => fn({ matches }))
    },
    listenerCount: () => listeners.size,
  }
}

describe('useMediaQuery', () => {
  let fake: ReturnType<typeof fakeMql>

  beforeEach(() => {
    vi.stubGlobal('matchMedia', () => fake.mql)
  })

  it('matchMedia 不可用 → 回退桌面 true', async () => {
    vi.unstubAllGlobals()
    const { useMediaQuery } = await import('./useMediaQuery')
    expect(useMediaQuery('(min-width: 768px)').value).toBe(true)
  })

  it('初始值取 mql.matches，变化时跟随', async () => {
    fake = fakeMql(false)
    const { useMediaQuery } = await import('./useMediaQuery')
    const r = useMediaQuery('(min-width: 768px)')
    expect(r.value).toBe(false)
    fake.fire(true)
    expect(r.value).toBe(true)
    fake.fire(false)
    expect(r.value).toBe(false)
  })

  it('组件卸载后移除监听', async () => {
    fake = fakeMql(true)
    const { useMediaQuery } = await import('./useMediaQuery')
    const { createApp, defineComponent, h } = await import('vue')
    const host = document.createElement('div')
    document.body.appendChild(host)
    const app = createApp(
      defineComponent({
        setup() {
          useMediaQuery('(min-width: 768px)')
          return () => h('div')
        },
      }),
    )
    app.mount(host)
    expect(fake.listenerCount()).toBe(1)
    app.unmount()
    expect(fake.mql.removeEventListener).toHaveBeenCalledTimes(1)
    expect(fake.listenerCount()).toBe(0)
    host.remove()
  })
})
