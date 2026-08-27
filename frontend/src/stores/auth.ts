import { reactive } from 'vue'
import { authApi, type MeResponse } from '@/api'
import { resetPermissions, setPermissions } from '@/permissions'

interface State {
  me: MeResponse | null
  initialized: boolean
  loading: boolean
  error: unknown
}

const state = reactive<State>({ me: null, initialized: false, loading: false, error: null })
let inflight: Promise<void> | null = null

async function initialize(): Promise<void> {
  if (state.initialized) return
  if (inflight) return inflight
  state.loading = true
  state.error = null
  inflight = authApi.me()
    .then((me) => {
      state.me = me
      setPermissions(me.permissions)
    })
    .catch((error) => {
      state.me = null
      state.error = error
      resetPermissions()
    })
    .finally(() => {
      state.loading = false
      state.initialized = true
      inflight = null
    })
  return inflight
}

async function logout(): Promise<void> {
  await authApi.logout().catch(() => {})
  reset()
}

function reset(): void {
  state.me = null
  state.initialized = false
  state.loading = false
  state.error = null
  resetPermissions()
}

export default { state, initialize, logout, reset }
