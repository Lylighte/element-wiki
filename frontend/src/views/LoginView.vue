<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { authApi } from '@/api'

const enabled = ref(false)
const provider = ref('')

onMounted(async () => {
  try {
    const s = await authApi.status()
    enabled.value = s.enabled
    provider.value = s.provider_name ?? ''
  } catch {
    /* 保持禁用态 */
  }
})

function go() {
  location.href = authApi.loginUrl('/')
}
</script>

<template>
  <div class="max-w-sm mx-auto mt-20 p-6 bg-white rounded shadow">
    <button :disabled="!enabled" data-test="sso-btn" class="w-full py-2 rounded bg-blue-600 text-white disabled:opacity-40" @click="go">
      {{ provider || 'SSO' }}
    </button>
  </div>
</template>
