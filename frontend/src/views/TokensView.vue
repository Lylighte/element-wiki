<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { tokenApi, userPreferencesApi, type ApiToken } from '@/api'
import authStore from '@/stores/auth'
import type { Locale } from '@/i18n'
import type { ThemePreference } from '@/composables/useTheme'
import { saveDisplayPreferences } from '@/stores/preferences'
import UiButton from '@/components/ui/UiButton.vue'
import UiCard from '@/components/ui/UiCard.vue'

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
    await saveDisplayPreferences({ language: language.value, theme: theme.value })
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
  <div data-test="tokens-page" class="personal-settings">
    <header class="settings-heading">
      <h1 data-test="settings-title">{{ t('settings.title') }}</h1>
      <p>{{ t('settings.identityHint') }}</p>
    </header>
    <UiCard padding="md">
      <h2 class="settings-section-title">{{ t('settings.account') }}</h2>
      <dl class="account-grid">
        <div><dt>{{ t('settings.displayName') }}</dt><dd>{{ authStore.state.me?.user.display_name }}</dd></div>
        <div><dt>{{ t('settings.email') }}</dt><dd>{{ authStore.state.me?.user.email || '—' }}</dd></div>
        <div><dt>{{ t('settings.role') }}</dt><dd><span class="role-badge">{{ authStore.state.me?.user.role }}</span></dd></div>
      </dl>
    </UiCard>
    <UiCard padding="md" class="settings-card">
      <div><h2 class="settings-section-title">{{ t('settings.preferences') }}</h2><p class="settings-description">{{ t('settings.preferencesHint') }}</p></div>
      <div class="preference-grid">
        <label>{{ t('settings.language') }}
          <select v-model="language" data-test="settings-language" class="settings-select"><option value="zh-CN">简体中文</option><option value="en">English</option></select>
        </label>
        <label>{{ t('settings.theme') }}
          <select v-model="theme" data-test="settings-theme" class="settings-select"><option value="system">{{ t('settings.system') }}</option><option value="light">{{ t('settings.light') }}</option><option value="dark">{{ t('settings.dark') }}</option></select>
        </label>
      </div>
      <p v-if="preferencesError" class="settings-error" role="alert">{{ t('common.saveFailed') }}</p>
      <div class="settings-actions"><UiButton variant="primary" size="lg" data-test="settings-save-preferences" :disabled="preferencesSaving" @click="savePreferences">{{ preferencesSaving ? t('common.saving') : t('common.save') }}</UiButton></div>
    </UiCard>
    <UiCard padding="md" class="token-card">
      <div class="token-heading"><div><h2 class="settings-section-title">{{ t('auth.tokens') }}</h2><p class="settings-description">{{ t('settings.tokenHint') }}</p></div></div>
      <p v-if="revokeError" class="settings-error" role="alert" data-test="token-revoke-error">{{ t('tokens.revokeFailed') }}</p>
      <p v-if="loading" class="token-message" role="status">{{ t('common.loading') }}</p>
      <div v-else-if="error" class="token-error" data-test="tokens-error"><p class="settings-error">{{ t('common.loadFailed') }}</p><UiButton size="sm" @click="refresh">{{ t('common.retry') }}</UiButton></div>
      <form class="token-create-form" @submit.prevent="create">
        <label class="token-name-field">{{ t('tokens.name') }}
          <input v-model="name" :placeholder="t('tokens.name')" data-test="token-name" class="settings-select" required />
        </label>
        <label class="token-expiry-field">{{ t('settings.tokenExpiry') }}
          <select v-model.number="expiresInDays" class="settings-select" aria-label="Token expiration">
            <option :value="30">{{ t('tokens.expires30') }}</option>
            <option :value="90">{{ t('tokens.expires90') }}</option>
            <option :value="365">{{ t('tokens.expires365') }}</option>
          </select>
        </label>
        <UiButton type="submit" variant="primary" size="md" class="token-create-button" data-test="token-create" :disabled="!name.trim()">{{ t('tokens.create') }}</UiButton>
      </form>
      <div v-if="plaintext" class="token-secret" data-test="plaintext">
        <strong>{{ t('settings.tokenCreated') }}</strong><code>{{ plaintext }}</code>
      </div>
      <ul v-if="!loading && !error" class="token-list">
        <li v-for="tok in items" :key="tok.id" data-test="token-row" class="token-row">
          <div class="token-details"><strong>{{ tok.name }}</strong><span><code>{{ tok.prefix }}…</code><span class="token-separator">·</span>{{ tok.expires_at ? new Date(tok.expires_at).toLocaleDateString() : t('tokens.neverExpires') }}</span></div>
          <span v-if="tok.revoked_at" data-test="token-revoked" class="token-state">{{ t('tokens.revoked') }}</span>
          <UiButton v-else variant="danger" size="sm" data-test="token-revoke" @click="revoke(tok.id)">{{ t('tokens.revoke') }}</UiButton>
        </li>
        <li v-if="!items.length" class="token-empty">{{ t('settings.noTokens') }}</li>
      </ul>
    </UiCard>
  </div>
</template>

<style scoped>
.personal-settings { width: min(100%, 56rem); margin: 0 auto; padding: 1.5rem; display: flex; flex-direction: column; gap: 1.25rem; overflow-y: auto; }
.settings-heading { padding: .25rem 0 .5rem; }
.settings-heading h1 { color: var(--color-heading); font-size: 1.75rem; font-weight: 700; line-height: 1.25; }
.settings-heading p, .settings-description { margin-top: .35rem; color: var(--color-text-light); font-size: .875rem; line-height: 1.5; }
.settings-section-title { color: var(--color-heading); font-size: 1.1rem; font-weight: 650; }
.account-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 12rem), 1fr)); gap: .9rem 1.5rem; margin-top: 1rem; }
.account-grid dt { color: var(--color-text-light); font-size: .8rem; }
.account-grid dd { margin-top: .2rem; color: var(--color-text); font-size: .95rem; font-weight: 550; overflow-wrap: anywhere; }
.role-badge { display: inline-flex; align-items: center; padding: .15rem .55rem; border-radius: 999px; background: var(--color-accent); color: var(--color-primary); font-size: .8rem; }
.settings-card { display: flex; flex-direction: column; gap: 1rem; }
.preference-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 1rem; }
.preference-grid label, .token-create-form label { display: grid; align-content: start; gap: .4rem; color: var(--color-text); font-size: .875rem; font-weight: 550; }
.settings-select { width: 100%; min-height: 2.65rem; padding: .5rem .7rem; border: 1px solid var(--color-border); border-radius: .55rem; background: var(--color-card-background); color: var(--color-text); font: inherit; }
.settings-select:hover { border-color: var(--color-border-hover); }
.settings-actions { display: flex; justify-content: flex-end; }
.settings-error { color: var(--color-danger); font-size: .875rem; }
.token-card { display: flex; flex-direction: column; gap: 1rem; }
.token-create-form { display: grid; grid-template-columns: minmax(0, 1fr) minmax(12rem, auto) auto; align-items: end; gap: .75rem; padding-bottom: 1rem; border-bottom: 1px solid var(--color-border); }
.token-create-button { min-height: 2.65rem; }
.token-error { display: flex; flex-wrap: wrap; align-items: center; gap: .75rem; }
.token-message, .token-empty { color: var(--color-text-light); font-size: .875rem; }
.token-secret { display: grid; gap: .5rem; padding: .85rem 1rem; border: 1px solid var(--color-warning-border); border-radius: .65rem; background: var(--color-warning-background); color: var(--color-text); }
.token-secret code { overflow-wrap: anywhere; color: var(--color-text); font-size: .85rem; }
.token-list { display: grid; gap: .5rem; margin: 0; padding: 0; list-style: none; }
.token-row { display: flex; align-items: center; justify-content: space-between; gap: 1rem; min-width: 0; padding: .8rem .9rem; border: 1px solid var(--color-border); border-radius: .65rem; background: var(--color-background); }
.token-details { display: grid; gap: .2rem; min-width: 0; }
.token-details > strong { overflow: hidden; color: var(--color-text); font-size: .9rem; text-overflow: ellipsis; white-space: nowrap; }
.token-details > span { display: flex; flex-wrap: wrap; align-items: center; gap: .4rem; color: var(--color-text-light); font-size: .8rem; }
.token-details code { color: var(--color-text-light); }
.token-separator { color: var(--color-border-hover); }
.token-state { flex: none; color: var(--color-text-light); font-size: .8rem; }
@media (max-width: 640px) {
  .personal-settings { padding: 1rem; }
  .preference-grid { grid-template-columns: 1fr; }
  .token-create-form { grid-template-columns: minmax(0, 1fr); }
  .token-create-button { justify-self: start; }
  .token-row { align-items: flex-start; }
}
</style>
