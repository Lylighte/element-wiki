<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { authApi, type MeResponse } from '@/api'
import { setPermissions } from '@/permissions'
import { ref } from 'vue'

const { t } = useI18n()
const me = ref<MeResponse | null>(null)

authApi
  .me()
  .then((m) => {
    me.value = m
    setPermissions(m.permissions)
  })
  .catch(() => {})

async function logout() {
  await authApi.logout().catch(() => {})
  location.href = '/'
}
</script>

<template>
  <div class="min-h-screen flex flex-col">
    <header class="h-14 border-b bg-white flex items-center px-4 gap-4">
      <span class="font-semibold">{{ t('common.appName') }}</span>
      <nav class="ml-auto flex items-center gap-3 text-sm">
        <RouterLink to="/search">{{ t('common.search') }}</RouterLink>
        <RouterLink v-if="me" to="/settings/tokens">{{ t('auth.me') }}</RouterLink>
        <button v-if="me" class="text-red-600" @click="logout">
          {{ t('nav.logout') }}
        </button>
      </nav>
    </header>
    <main class="flex-1 p-6 max-w-5xl w-full mx-auto">
      <RouterView />
    </main>
  </div>
</template>
