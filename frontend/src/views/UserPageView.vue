<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { reportApi, userPageApi, type UserPage } from '@/api'
import UiButton from '@/components/ui/UiButton.vue'
import authStore from '@/stores/auth'
import { can } from '@/permissions'

const route = useRoute()
const { t } = useI18n()
const page = ref<UserPage | null>(null)
const source = ref('')
const loading = ref(true)
const saving = ref(false)
const editing = ref(false)
const error = ref(false)
const history = ref<NonNullable<UserPage['published']>[]>([])
const historyOpen = ref(false)
const userID = computed(() => String(route.params.user_id || ''))
const isOwner = computed(() => authStore.state.me?.user.id === userID.value)
const canEdit = computed(() => isOwner.value && can('user.page.manage.own'))
const displayed = computed(() => page.value?.published?.html || '')

async function reportPage() {
  try {
    const { value } = await ElMessageBox.prompt(t('admin.reportReason'), t('admin.reportButton'), { inputType: 'textarea', inputValidator: (v) => !!v?.trim() && v.trim().length >= 5 })
    await reportApi.create('user_page', userID.value, value)
    ElMessage.success(t('admin.reportSubmitted'))
  } catch { /* prompt cancel */ }
}
async function toggleHistory() {
  historyOpen.value = !historyOpen.value
  if (!historyOpen.value || history.value.length) return
  try { history.value = (isOwner.value ? await userPageApi.myRevisions() : await userPageApi.revisions(userID.value)).items }
  catch { ElMessage.error(t('userPage.historyFailed')) }
}

async function load() {
  loading.value = true
  error.value = false
  try {
    const result = isOwner.value ? await userPageApi.getMine() : await userPageApi.get(userID.value)
    page.value = result.user_page
    source.value = result.user_page.pending?.content || result.user_page.published?.content || ''
  } catch {
    error.value = true
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    page.value = (await userPageApi.saveMine(source.value)).user_page
    source.value = page.value.pending?.content || page.value.published?.content || ''
    editing.value = false
    ElMessage.success(page.value.pending ? t('userPage.submitted') : t('userPage.saved'))
  } catch {
    ElMessage.error(t('userPage.saveFailed'))
  } finally {
    saving.value = false
  }
}

onMounted(() => void load())
</script>

<template>
  <main class="mx-auto min-h-full max-w-4xl p-5 sm:p-8" data-test="user-page">
    <p v-if="loading" class="text-[var(--color-text-light)]">{{ t('common.loading') }}</p>
    <section v-else-if="error" class="rounded-lg border p-6 text-center">
      <h1 class="text-xl font-semibold">{{ t('userPage.unavailable') }}</h1>
      <p class="mt-2 text-[var(--color-text-light)]">{{ t('userPage.unavailableHint') }}</p>
    </section>
    <template v-else-if="page">
      <header class="mb-6 flex flex-wrap items-start justify-between gap-4 border-b pb-5">
        <div><p class="text-sm text-[var(--color-text-light)]">{{ t('userPage.label') }}</p><h1 class="text-3xl font-semibold">{{ page.display_name }}</h1></div>
        <div class="flex gap-2"><UiButton v-if="page.published" data-test="user-page-history-toggle" @click="toggleHistory">{{ t('userPage.history') }}</UiButton><UiButton v-if="authStore.state.me && !isOwner && page.published" data-test="report-user-page" @click="reportPage">{{ t('admin.reportButton') }}</UiButton><UiButton v-if="canEdit && !editing" data-test="user-page-edit" size="lg" @click="editing = true">{{ t('userPage.edit') }}</UiButton></div>
      </header>
      <aside v-if="isOwner && page.pending" data-test="user-page-pending" class="mb-5 rounded border border-[var(--color-warning-border)] bg-[var(--color-warning-background)] p-3 text-sm text-[var(--color-warning)]">
        {{ t('userPage.pendingNotice') }}<span v-if="page.pending.reason"> {{ page.pending.reason }}</span>
      </aside>
      <section v-if="editing && canEdit" class="space-y-4">
        <label class="block text-sm font-medium" for="user-page-source">{{ t('userPage.markdown') }}</label>
        <textarea id="user-page-source" data-test="user-page-source" v-model="source" rows="18" maxlength="20000" class="w-full resize-y rounded border p-3 font-mono text-sm" />
        <div class="flex justify-end gap-2"><UiButton size="lg" @click="editing = false">{{ t('common.cancel') }}</UiButton><UiButton data-test="user-page-save" variant="primary" size="lg" :disabled="saving || !source.trim()" @click="save">{{ saving ? t('common.saving') : t('common.save') }}</UiButton></div>
      </section>
      <article v-else-if="page.published" data-test="user-page-html" class="prose max-w-none" v-html="displayed" />
      <section v-if="historyOpen" class="mt-8 border-t pt-4" data-test="user-page-history"><h2 class="mb-3 text-lg font-semibold">{{ t('userPage.history') }}</h2><details v-for="revision in history" :key="revision.id" class="mb-2 rounded border p-3"><summary class="cursor-pointer text-sm">{{ t(`userPage.status_${revision.status || 'published'}`) }} · {{ new Date(revision.created_at).toLocaleString() }}<span v-if="revision.reason"> · {{ revision.reason }}</span></summary><pre class="mt-3 max-h-96 overflow-auto whitespace-pre-wrap rounded bg-[var(--color-background-mute)] p-3 text-sm">{{ revision.content }}</pre></details><p v-if="!history.length" class="text-sm text-[var(--color-text-light)]">{{ t('userPage.historyEmpty') }}</p></section>
      <p v-else-if="isOwner" class="rounded-lg border border-dashed p-8 text-center text-[var(--color-text-light)]">{{ t('userPage.empty') }}</p>
    </template>
  </main>
</template>
