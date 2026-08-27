<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { authApi, type MeResponse } from '@/api'
import { setPermissions } from '@/permissions'
import { loginErrorKey } from '@/utils/loginErrors'

const { t } = useI18n()
const me = ref<MeResponse | null>(null)
const route = useRoute()
const enabled = ref(false)
const provider = ref('')

const loginReason = computed(() => {
  const reason = route.query.error
  return typeof reason === 'string' ? reason : null
})
const loginErrorText = computed(() => {
  const key = loginErrorKey(loginReason.value)
  return key ? t(key) : null
})

function redirectTarget() {
  const target = route.query.redirect
  if (typeof target === 'string' && target.startsWith('/') && !target.startsWith('//')) {
    return target
  }
  return '/'
}

onMounted(async () => {
  try {
    const s = await authApi.status()
    enabled.value = s.enabled
    provider.value = s.provider_name ?? ''
  } catch {
    /* 保持禁用态 */
  }
})

authApi
  .me()
  .then((m) => {
    me.value = m
    setPermissions(m.permissions)
    if (m.user.id) location.href = redirectTarget()
  })
  .catch(() => {})

function go() {
  location.href = authApi.loginUrl(redirectTarget())
}
</script>

<template>
  <div class="max-w-sm mx-auto mt-20 p-6 bg-white rounded shadow" data-test="login-page">
    <p v-if="loginErrorText" class="text-red-600 mb-3 text-sm" data-test="login-error">{{ loginErrorText }}</p>
    <button
      :disabled="!enabled"
      data-test="sso-btn"
      class="w-full py-2 rounded bg-blue-600 text-white disabled:opacity-40"
      @click="go"
    >
      {{ provider || t('auth.loginWithSSO') }}
    </button>
  </div>
</template>
