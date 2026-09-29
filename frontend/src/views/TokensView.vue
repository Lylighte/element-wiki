<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { tokenApi, userPreferencesApi, type ApiToken } from '@/api'
import authStore from '@/stores/auth'
import { setLocale, type Locale } from '@/i18n'
import { applyThemePreference, type ThemePreference } from '@/composables/useTheme'

const { t } = useI18n()

const items = ref<ApiToken[]>([])
const plaintext = ref('')
const name = ref('')
const expiresInDays = ref<30 | 90 | 365>(90)
const language = ref<Locale>('zh-CN')
const theme = ref<ThemePreference>('system')
const preferencesSaving = ref(false)
const preferencesError = ref(false)
const error = ref(false)
const revokeError = ref(false)
const loading = ref(false)

async function refresh() {
  loading.value = true
  error.value = false
  try {
    const r = await tokenApi.list()
    items.value = r.items
  } catch {
    error.value = true
  } finally {
    loading.value = false
  }
}
onMounted(() => {
  void refresh()
  void userPreferencesApi.get().then(({ preferences }) => {
    if (!preferences) return
    language.value = preferences.language
    theme.value = preferences.theme
  }).catch(() => { preferencesError.value = true })
})

async function savePreferences() {
  preferencesSaving.value = true
  preferencesError.value = false
  try {
    await userPreferencesApi.set(language.value, theme.value)
    localStorage.setItem('lang', language.value)
    localStorage.setItem('theme', theme.value)
    setLocale(language.value)
    applyThemePreference(theme.value)
  } catch {
    preferencesError.value = true
  } finally {
    preferencesSaving.value = false
  }
}

async function create() {
  if (!name.value) return
  const res = await tokenApi.create(name.value, expiresInDays.value)
  plaintext.value = res.token
  name.value = ''
  await refresh()
}
async function revoke(id: string) {
  revokeError.value = false
  try {
    await tokenApi.revoke(id)
    await refresh()
  } catch {
    revokeError.value = true
  }
}
</script>

<template>
  <div data-test="tokens-page" class="mx-auto max-w-4xl space-y-8 p-4 sm:p-6">
    <header>
      <h1 data-test="settings-title" class="text-2xl font-semibold">{{ t('settings.title') }}</h1>
      <p class="mt-1 text-sm text-gray-500">{{ t('settings.identityHint') }}</p>
    </header>
    <section class="rounded-lg border border-gray-200 p-4 dark:border-gray-700">
      <h2 class="mb-3 text-lg font-medium">{{ t('settings.account') }}</h2>
      <dl class="grid gap-3 sm:grid-cols-2">
        <div><dt class="text-sm text-gray-500">{{ t('settings.displayName') }}</dt><dd>{{ authStore.state.me?.user.display_name }}</dd></div>
        <div><dt class="text-sm text-gray-500">{{ t('settings.email') }}</dt><dd>{{ authStore.state.me?.user.email || '—' }}</dd></div>
        <div><dt class="text-sm text-gray-500">{{ t('settings.role') }}</dt><dd>{{ authStore.state.me?.user.role }}</dd></div>
      </dl>
    </section>
    <section class="space-y-4 rounded-lg border border-gray-200 p-4 dark:border-gray-700">
      <div><h2 class="text-lg font-medium">{{ t('settings.preferences') }}</h2><p class="text-sm text-gray-500">{{ t('settings.preferencesHint') }}</p></div>
      <div class="grid gap-4 sm:grid-cols-2">
        <label class="grid gap-1 text-sm">{{ t('settings.language') }}
          <select v-model="language" data-test="settings-language" class="rounded border px-3 py-2 dark:bg-gray-900"><option value="zh-CN">简体中文</option><option value="en">English</option></select>
        </label>
        <label class="grid gap-1 text-sm">{{ t('settings.theme') }}
          <select v-model="theme" data-test="settings-theme" class="rounded border px-3 py-2 dark:bg-gray-900"><option value="system">{{ t('settings.system') }}</option><option value="light">{{ t('settings.light') }}</option><option value="dark">{{ t('settings.dark') }}</option></select>
        </label>
      </div>
      <p v-if="preferencesError" class="text-sm text-red-600">{{ t('common.saveFailed') }}</p>
      <button data-test="settings-save-preferences" class="rounded bg-blue-600 px-4 py-2 text-white disabled:opacity-50" :disabled="preferencesSaving" @click="savePreferences">{{ preferencesSaving ? t('common.saving') : t('common.save') }}</button>
    </section>
    <section class="space-y-4">
      <div><h2 class="text-lg font-medium">{{ t('auth.tokens') }}</h2><p class="text-sm text-gray-500">{{ t('settings.tokenHint') }}</p></div>
    <p v-if="revokeError" class="text-sm text-red-600" role="alert" data-test="token-revoke-error">{{ t('tokens.revokeFailed') }}</p>
    <p v-if="loading" class="text-[var(--color-text)]">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="text-red-600" data-test="tokens-error">{{ t('common.loadFailed') }}</p>
    <button v-if="error" class="underline" @click="refresh">{{ t('common.retry') }}</button>
    <form class="flex gap-2" @submit.prevent="create">
      <input v-model="name" :placeholder="t('tokens.name')" data-test="token-name" class="min-w-0 flex-1 rounded border px-2 py-1" />
      <select v-model.number="expiresInDays" class="border rounded px-2 py-1" aria-label="Token expiration">
        <option :value="30">{{ t('tokens.expires30') }}</option>
        <option :value="90">{{ t('tokens.expires90') }}</option>
        <option :value="365">{{ t('tokens.expires365') }}</option>
      </select>
      <button data-test="token-create" class="px-3 py-1 bg-blue-600 text-white rounded">{{ t('tokens.create') }}</button>
    </form>
    <code v-if="plaintext" data-test="plaintext">{{ plaintext }}</code>
    <ul class="space-y-2">
      <li v-for="tok in items" :key="tok.id" data-test="token-row">
        {{ tok.name }} ({{ tok.prefix }}…) ·
        {{ tok.expires_at ? new Date(tok.expires_at).toLocaleDateString() : t('tokens.neverExpires') }}
        <span v-if="tok.revoked_at" data-test="token-revoked" class="text-sm text-gray-500">{{ t('tokens.revoked') }}</span>
        <button v-else class="text-red-600" data-test="token-revoke" @click="revoke(tok.id)">{{ t('tokens.revoke') }}</button>
      </li>
    </ul>
    </section>
  </div>
</template>
