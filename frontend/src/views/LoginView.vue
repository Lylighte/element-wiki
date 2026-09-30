<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import UiButton from '@/components/ui/UiButton.vue'
import { authApi } from '@/api'
import { loginErrorKey } from '@/utils/loginErrors'
import authStore from '@/stores/auth'

const { t } = useI18n()
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

authStore.initialize().then(() => {
  if (authStore.state.me?.user.id) location.href = redirectTarget()
})

function go() {
  location.href = authApi.loginUrl(redirectTarget())
}
</script>

<template>
  <div class="max-w-sm mx-auto mt-20 p-6 bg-[var(--color-card-background)] rounded shadow" data-test="login-page">
    <p v-if="loginErrorText" class="text-[var(--color-danger)] mb-3 text-sm" data-test="login-error">{{ loginErrorText }}</p>
    <UiButton
      :disabled="!enabled"
      variant="primary"
      block
      size="lg"
      data-test="sso-btn"
      @click="go"
    >
      {{ provider || t('auth.loginWithSSO') }}
    </UiButton>
  </div>
</template>
