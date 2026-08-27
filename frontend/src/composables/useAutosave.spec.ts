// 自动保存状态机验收（fake timers：dirty→saving→saved；错误重试；冲突置位）。
import { describe, expect, it, vi, afterEach } from 'vitest'
import { useAutosave } from './useAutosave'

afterEach(() => vi.useRealTimers())

function setup(save: (c: string) => Promise<void>, delay = 10) {
  return useAutosave({ delay, save })
}

describe('useAutosave', () => {
  it('change 后进入 dirty，延迟后 saving→saved', async () => {
    vi.useFakeTimers()
    const save = vi.fn().mockResolvedValue(undefined)
    const a = setup(save)
    a.schedule('v1')
    expect(a.status.value).toBe('dirty')
    await vi.advanceTimersByTimeAsync(15)
    expect(save).toHaveBeenCalledWith('v1')
    expect(a.status.value).toBe('saved')
  })

  it('连续 change 只保留最后一次内容', async () => {
    vi.useFakeTimers()
    const save = vi.fn().mockResolvedValue(undefined)
    const a = setup(save)
    a.schedule('a')
    a.schedule('b')
    await vi.advanceTimersByTimeAsync(50)
    expect(save).toHaveBeenCalledTimes(1)
    expect(save).toHaveBeenCalledWith('b')
  })

  it('非冲突错误自动重试并置 error', async () => {
    vi.useFakeTimers()
    const save = vi.fn().mockRejectedValueOnce(new Error('boom')).mockResolvedValue(undefined)
    const a = setup(save, 5)
    a.schedule('x')
    await vi.advanceTimersByTimeAsync(6)
    expect(a.status.value).toBe('error')
    await vi.advanceTimersByTimeAsync(6)
    expect(a.status.value).toBe('saved')
  })

  it('冲突错误置 conflict 且不自动重试', async () => {
    vi.useFakeTimers()
    const err = Object.assign(new Error('conflict'), { status: 409 })
    const save = vi.fn().mockRejectedValue(err)
    const a = useAutosave({
      delay: 5,
      save,
      isConflict: (e) => (e as { status?: number }).status === 409,
    })
    a.schedule('y')
    await vi.advanceTimersByTimeAsync(6)
    expect(a.conflict.value).toBe(true)
    expect(a.status.value).toBe('error')
    expect(save).toHaveBeenCalledTimes(1)
  })

  it('flushNow 立即落盘', async () => {
    vi.useFakeTimers()
    const save = vi.fn().mockResolvedValue(undefined)
    const a = setup(save, 60_000)
    a.schedule('now')
    await a.flushNow()
    expect(save).toHaveBeenCalledWith('now')
    expect(a.status.value).toBe('saved')
  })

  it('空字符串也会触发保存', async () => {
    vi.useFakeTimers()
    const save = vi.fn().mockResolvedValue(undefined)
    const a = setup(save, 60_000)
    a.schedule('')
    await a.flushNow()
    expect(save).toHaveBeenCalledWith('')
  })

  it('reset 清空挂起内容', async () => {
    vi.useFakeTimers()
    const save = vi.fn()
    const a = setup(save)
    a.schedule('z')
    a.reset()
    await vi.advanceTimersByTimeAsync(100)
    expect(save).not.toHaveBeenCalled()
    expect(a.status.value).toBe('idle')
  })
})
