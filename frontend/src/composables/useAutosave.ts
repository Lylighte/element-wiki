// 自动保存状态机（ED-05/T7.5 验收）：idle→dirty→saving→saved|error；冲突单独置位。
import { ref, computed } from 'vue'

export type AutosaveStatus = 'idle' | 'dirty' | 'saving' | 'saved' | 'error'

export interface AutosaveOptions {
  delay?: number
  save: (content: string) => Promise<void>
  isConflict?: (err: unknown) => boolean
}

export function useAutosave(options: AutosaveOptions) {
  const delay = options.delay ?? 1500
  const status = ref<AutosaveStatus>('idle')
  const conflict = ref(false)
  let timer: ReturnType<typeof setTimeout> | null = null
  let pending = ''
  let seq = 0

  const busy = computed(() => status.value === 'saving')

  function schedule(content: string, opts?: { quiet?: boolean }) {
    pending = content
    if (timer) clearTimeout(timer)
    if (!opts?.quiet) status.value = 'dirty'
    timer = setTimeout(flush, delay)
  }

  async function flush(): Promise<void> {
    if (!pending) return
    const mine = ++seq
    const content = pending
    pending = ''
    status.value = 'saving'
    try {
      await options.save(content)
      if (mine === seq && !pending) status.value = 'saved'
    } catch (err) {
      if (options.isConflict?.(err)) {
        conflict.value = true
        status.value = 'error'
      } else {
        status.value = 'error'
        schedule(content, { quiet: true }) // 非冲突错误：保持 error 态静默重试
      }
    }
  }

  async function flushNow(): Promise<void> {
    if (timer) {
      clearTimeout(timer)
      timer = null
    }
    await flush()
  }

  function reset() {
    if (timer) clearTimeout(timer)
    pending = ''
    seq++
    status.value = 'idle'
    conflict.value = false
  }

  return { status, conflict, busy, schedule, flushNow, reset }
}
