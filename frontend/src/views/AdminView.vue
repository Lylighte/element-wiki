<script setup lang="ts">
// 管理视图：按权限码显隐 Tab；各域面板内联实现（T7.8）。
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { adminApi, type DashboardStats } from '@/api'
import { can } from '@/permissions'
import AdminTabs from '@/components/admin/AdminTabs.vue'
import TreeAdminPanel from '@/components/admin/TreeAdminPanel.vue'
import siteStore from '@/stores/site'
import treeStore from '@/stores/tree'

const perm = reactive({ has: (code: string) => can(code) })
const { t, locale } = useI18n()

// settings（T11.2）：九键类型化表单，仅提交变更键
interface SettingsForm {
  wiki_title: string
  timezone: string
  default_lang: 'zh-CN' | 'en'
  anonymous_read: boolean
  comments_enabled: boolean
  max_versions: number
  upload_max_mb: number
  trash_retention_days: number
  allowed_extensions: string
}
const form = reactive<SettingsForm>({
  wiki_title: '', timezone: '', default_lang: 'zh-CN',
  anonymous_read: false, comments_enabled: false,
  max_versions: 100, upload_max_mb: 20, trash_retention_days: 30,
  allowed_extensions: '',
})
const timezoneGroups = [
  {
    label: 'admin.timezoneRegionAsia',
    options: [
      { value: 'Asia/Shanghai', label: 'admin.timezoneShanghai' },
      { value: 'Asia/Tokyo', label: 'admin.timezoneTokyo' },
      { value: 'Asia/Seoul', label: 'admin.timezoneSeoul' },
      { value: 'Asia/Singapore', label: 'admin.timezoneSingapore' },
      { value: 'Asia/Kolkata', label: 'admin.timezoneKolkata' },
      { value: 'Asia/Dubai', label: 'admin.timezoneDubai' },
    ],
  },
  {
    label: 'admin.timezoneRegionEurope',
    options: [
      { value: 'Europe/London', label: 'admin.timezoneLondon' },
      { value: 'Europe/Berlin', label: 'admin.timezoneBerlin' },
      { value: 'Europe/Paris', label: 'admin.timezoneParis' },
    ],
  },
  {
    label: 'admin.timezoneRegionAmericas',
    options: [
      { value: 'America/Los_Angeles', label: 'admin.timezoneLosAngeles' },
      { value: 'America/Chicago', label: 'admin.timezoneChicago' },
      { value: 'America/New_York', label: 'admin.timezoneNewYork' },
      { value: 'America/Sao_Paulo', label: 'admin.timezoneSaoPaulo' },
    ],
  },
  {
    label: 'admin.timezoneRegionOceania',
    options: [
      { value: 'Australia/Sydney', label: 'admin.timezoneSydney' },
      { value: 'Pacific/Auckland', label: 'admin.timezoneAuckland' },
    ],
  },
]
const timezonePresetValues = timezoneGroups.flatMap((group) => group.options.map((option) => option.value))
const currentTimezoneIsCustom = computed(() => form.timezone !== '' && !timezonePresetValues.includes(form.timezone))
function formatUtcOffset(timezone: string): string {
  try {
    const zoneName = new Intl.DateTimeFormat('en-US', {
      timeZone: timezone,
      timeZoneName: 'shortOffset',
    }).formatToParts(new Date()).find((part) => part.type === 'timeZoneName')?.value ?? 'GMT'
    const match = /^GMT(?:([+-])(\d{1,2})(?::(\d{2}))?)?$/.exec(zoneName)
    if (!match) return 'UTC'
    if (!match[1]) return 'UTC+00:00'
    return `UTC${match[1]}${match[2].padStart(2, '0')}:${match[3] ?? '00'}`
  } catch {
    return 'UTC?'
  }
}
const original = ref<SettingsForm>({ ...form })
const fieldErrors = ref<Record<string, string>>({})
const loadError = ref(false)
const operationError = ref(false)

function loadIntoForm(raw: Record<string, string>) {
  form.wiki_title = raw.wiki_title ?? ''
  form.timezone = raw.timezone ?? ''
  form.default_lang = raw.default_lang === 'en' ? 'en' : 'zh-CN'
  form.anonymous_read = raw.anonymous_read === 'true'
  form.comments_enabled = raw.comments_enabled === 'true'
  form.max_versions = Number(raw.max_versions) || 100
  form.upload_max_mb = Number(raw.upload_max_mb) || 20
  form.trash_retention_days = Number(raw.trash_retention_days) || 30
  form.allowed_extensions = raw.allowed_extensions ?? ''
  original.value = { ...form }
}

async function loadSettings() {
  loadIntoForm(await adminApi.settings())
}

const changedPatch = computed<Record<string, string> | null>(() => {
  const patch: Record<string, string> = {}
  if (form.wiki_title !== original.value.wiki_title) patch.wiki_title = form.wiki_title
  if (form.timezone !== original.value.timezone) patch.timezone = form.timezone
  if (form.default_lang !== original.value.default_lang) patch.default_lang = form.default_lang
  if (form.anonymous_read !== original.value.anonymous_read)
    patch.anonymous_read = String(form.anonymous_read)
  if (form.comments_enabled !== original.value.comments_enabled)
    patch.comments_enabled = String(form.comments_enabled)
  if (form.max_versions !== original.value.max_versions)
    patch.max_versions = String(form.max_versions)
  if (form.upload_max_mb !== original.value.upload_max_mb)
    patch.upload_max_mb = String(form.upload_max_mb)
  if (form.trash_retention_days !== original.value.trash_retention_days)
    patch.trash_retention_days = String(form.trash_retention_days)
  if (form.allowed_extensions !== original.value.allowed_extensions)
    patch.allowed_extensions = form.allowed_extensions
  return Object.keys(patch).length ? patch : null
})

async function saveSettings() {
  const patch = changedPatch.value
  if (!patch) {
    ElMessage.info(t('admin.noChanges'))
    return
  }
  fieldErrors.value = {}
  try {
    await adminApi.updateSettings(patch)
    ElMessage.success(t('admin.saved'))
    if (patch.wiki_title !== undefined) siteStore.setTitle(patch.wiki_title)
    if (patch.timezone !== undefined) siteStore.setTimezone(patch.timezone)
    await loadSettings()
  } catch (err) {
    const status = (err as { status?: number }).status
    const fields = (err as { fields?: Record<string, string> }).fields
    if (status === 422 && fields) {
      fieldErrors.value = fields
      return
    }
    ElMessage.error(t('common.loadFailed'))
  }
}

// users
interface UserRow {
  id: string
  email: string
  display_name: string
  role: 'viewer' | 'editor' | 'admin'
  status: 'active' | 'disabled'
}
const users = ref<UserRow[]>([])
const userQuery = ref('')
const usersLoading = ref(false)
let usersRequest = 0
async function loadUsers() {
  const request = ++usersRequest
  usersLoading.value = true
  try {
    const r = await adminApi.users(userQuery.value)
    if (request === usersRequest) users.value = r.items as UserRow[]
  } catch {
    if (request === usersRequest) operationError.value = true
  } finally {
    if (request === usersRequest) usersLoading.value = false
  }
}
async function changeRole(u: UserRow, role: UserRow['role']) {
  try {
    await adminApi.updateUser(u.id, { role })
    await loadUsers()
  } catch {
    operationError.value = true
  }
}
async function toggleStatus(u: UserRow) {
  if (u.status === 'active') {
    try {
      await ElMessageBox.confirm(t('admin.disableConfirm'), { type: 'warning' })
    } catch {
      return
    }
  }
  const next = u.status === 'active' ? 'disabled' : 'active'
  try {
    await adminApi.updateUser(u.id, { status: next })
    await loadUsers()
  } catch {
    operationError.value = true
  }
}

// dashboard
const stats = ref<DashboardStats | null>(null)

// 05 计划提交 4：dashboard 最近文档链接用 slug 路径；树未命中时回退自身 slug。
function recentDocPath(id: string, slug: string): string {
  return treeStore.pathSlugOf(treeStore.state.nodes, id) || slug
}
function formatDashboardDate(timestamp: number): string {
  return new Intl.DateTimeFormat(locale.value, {
    dateStyle: 'medium',
    timeStyle: 'short',
    timeZone: siteStore.state.timezone,
  }).format(timestamp)
}

// backups（T12.1）：发起导出 + job 轮询 + 双导入入口
const backupFiles = ref<string[]>([])
const backupBusy = ref(false)
const jobLine = ref('')

async function pollUntilDone(
  id: string,
  fetcher: (id: string) => Promise<{ status: string; last_error?: string; imported_files?: number; failed_files?: number }>,
): Promise<{ status: string; last_error?: string }> {
  for (;;) {
    const j = await fetcher(id)
    if (j.status === 'done' || j.status === 'failed') return j
    await new Promise((r) => setTimeout(r, 500))
  }
}

async function startBackup() {
  if (backupBusy.value) return
  backupBusy.value = true
  try {
    const { job_id } = await adminApi.startBackup()
    const done = await pollUntilDone(job_id, adminApi.backupJob)
    if (done.status === 'failed') ElMessage.error(done.last_error || t('admin.jobFailed'))
    else ElMessage.success(t('admin.backupDone'))
    backupFiles.value = (await adminApi.backupFiles()).items
  } catch (err) {
    showJobError(err)
  } finally {
    backupBusy.value = false
  }
}

function pickFile(accept: string, onFile: (f: File) => void) {
  const input = document.createElement('input')
  input.type = 'file'
  input.accept = accept
  input.onchange = () => {
    const f = input.files?.[0]
    if (f) void onFile(f)
  }
  input.click()
}

function showJobError(err: unknown) {
  const detail = (err as { detail?: string }).detail
  ElMessage.error(detail || t('admin.jobFailed'))
}

async function importBackupZip(f: File) {
  try {
    await ElMessageBox.confirm(t('admin.importConfirm'), { type: 'warning' })
  } catch {
    return
  }
  backupBusy.value = true
  try {
    const { job_id } = await adminApi.importBackup(f)
    // 备份导入 job 落在 backup_jobs：必须轮询 backups jobs 端点
    // （imports jobs 端点读 import_jobs，查不到会 404）
    const done = await pollUntilDone(job_id, adminApi.backupJob)
    if (done.status === 'failed') ElMessage.error(done.last_error || t('admin.jobFailed'))
    else ElMessage.success(t('admin.importDone'))
  } catch (err) {
    showJobError(err)
  } finally {
    backupBusy.value = false
  }
}

async function importMarkdownZip(f: File) {
  try {
    // T17.3：隔离根语义说明——导入零覆盖，完成后可在文档树中移动或整体回收
    await ElMessageBox.confirm(t('admin.importMdConfirm'), { type: 'info' })
  } catch {
    return
  }
  backupBusy.value = true
  try {
    const { job_id } = await adminApi.markdownImport(f)
    const done = await pollUntilDone(job_id, adminApi.importJob)
    if (done.status === 'failed') ElMessage.error(done.last_error || t('admin.jobFailed'))
    else ElMessage.success(t('admin.importDone'))
  } catch (err) {
    showJobError(err)
  } finally {
    backupBusy.value = false
  }
}

async function loadAdminData() {
  loadError.value = false
  const loads: Promise<void>[] = []
  if (can('settings.manage')) loads.push(loadSettings())
  if (can('user.list')) loads.push(loadUsers())
  if (can('dashboard.read')) {
    void treeStore.load()
    loads.push(
      adminApi
        .dashboard()
        .then((st) => {
          stats.value = st
        })
        .then(() => undefined),
    )
  }
  if (can('backup.manage')) loads.push(adminApi.backupFiles().then((f) => {
        backupFiles.value = f.items
      }))
  const results = await Promise.allSettled(loads)
  loadError.value = results.some((result) => result.status === 'rejected')
}

onMounted(() => void loadAdminData())

async function removeBackup(f: string) {
  try {
    await adminApi.deleteBackupFile(f)
    backupFiles.value = (await adminApi.backupFiles()).items
  } catch {
    operationError.value = true
  }
}
</script>

<template>
  <div v-if="loadError" class="mb-4 text-red-600 space-x-2" data-test="admin-load-error">
    <span>{{ t('common.loadFailed') }}</span>
    <button class="underline" data-test="admin-load-retry" @click="loadAdminData">{{ t('common.retry') }}</button>
  </div>
  <p v-if="operationError" class="mb-4 text-red-600" data-test="admin-operation-error">
    {{ t('common.loadFailed') }}
  </p>
  <AdminTabs :perm="perm">
    <template #tree>
      <TreeAdminPanel />
    </template>
    <template #settings>
      <div class="max-w-3xl space-y-5" data-test="admin-settings">
        <div>
          <h1 class="text-xl font-semibold">{{ t('admin.settings') }}</h1>
          <p class="setting-help mt-1">{{ t('admin.settingsIntro') }}</p>
        </div>
        <section class="setting-card">
          <h2 class="text-base font-semibold">{{ t('admin.siteAccess') }}</h2>
          <div class="setting-grid mt-4">
            <label class="setting-field">{{ t('admin.fieldWikiTitle') }}
              <input v-model="form.wiki_title" data-test="f-wiki-title" class="setting-input" />
              <span class="setting-help">{{ t('admin.helpWikiTitle') }}</span>
              <span v-if="fieldErrors.wiki_title" class="setting-error">{{ fieldErrors.wiki_title }}</span>
            </label>
            <label class="setting-field">{{ t('admin.fieldDefaultLang') }}
              <select v-model="form.default_lang" data-test="f-lang" class="setting-input">
                <option value="zh-CN">{{ t('admin.langZh') }}</option>
                <option value="en">{{ t('admin.langEn') }}</option>
              </select>
              <span class="setting-help">{{ t('admin.helpDefaultLang') }}</span>
              <span v-if="fieldErrors.default_lang" class="setting-error">{{ fieldErrors.default_lang }}</span>
            </label>
            <label class="setting-field">{{ t('admin.fieldTimezone') }}
              <select v-model="form.timezone" data-test="f-tz" class="setting-input">
                <option disabled value="">{{ t('admin.selectTimezone') }}</option>
                <option v-if="currentTimezoneIsCustom" :value="form.timezone">
                  {{ formatUtcOffset(form.timezone) }} · {{ t('admin.timezoneCurrentCustom', { timezone: form.timezone }) }}
                </option>
                <option value="UTC">UTC+00:00 · {{ t('admin.timezoneUTC') }} · UTC</option>
                <optgroup v-for="group in timezoneGroups" :key="group.label" :label="t(group.label)">
                  <option v-for="option in group.options" :key="option.value" :value="option.value">
                    {{ formatUtcOffset(option.value) }} · {{ t(option.label) }} · {{ option.value }}
                  </option>
                </optgroup>
              </select>
              <span class="setting-help">{{ t('admin.helpTimezone') }}</span>
              <span v-if="fieldErrors.timezone" class="setting-error">{{ fieldErrors.timezone }}</span>
            </label>
          </div>
          <div class="mt-4 divide-y divide-[var(--color-border)] border-t border-[var(--color-border)]">
            <label class="setting-toggle">
              <span><strong>{{ t('admin.fieldAnonRead') }}</strong><small>{{ t('admin.helpAnonRead') }}</small></span>
              <el-switch v-model="form.anonymous_read" data-test="f-anon" />
            </label>
            <span v-if="fieldErrors.anonymous_read" class="setting-error">{{ fieldErrors.anonymous_read }}</span>
            <label class="setting-toggle">
              <span><strong>{{ t('admin.fieldCommentsEnabled') }}</strong><small>{{ t('admin.helpCommentsEnabled') }}</small></span>
              <el-switch v-model="form.comments_enabled" data-test="f-comments" />
            </label>
            <span v-if="fieldErrors.comments_enabled" class="setting-error">{{ fieldErrors.comments_enabled }}</span>
          </div>
        </section>
        <section class="setting-card">
          <h2 class="text-base font-semibold">{{ t('admin.contentStorage') }}</h2>
          <div class="setting-grid mt-4">
            <label class="setting-field">{{ t('admin.fieldMaxVersions') }}
              <el-input-number v-model="form.max_versions" :min="1" data-test="f-max-versions" />
              <span class="setting-help">{{ t('admin.helpMaxVersions') }}</span>
              <span v-if="fieldErrors.max_versions" class="setting-error">{{ fieldErrors.max_versions }}</span>
            </label>
            <label class="setting-field">{{ t('admin.fieldUploadMax') }}
              <el-input-number v-model="form.upload_max_mb" :min="1" data-test="f-upload-max" />
              <span class="setting-help">{{ t('admin.helpUploadMax') }}</span>
              <span v-if="fieldErrors.upload_max_mb" class="setting-error">{{ fieldErrors.upload_max_mb }}</span>
            </label>
            <label class="setting-field">{{ t('admin.fieldTrashDays') }}
              <el-input-number v-model="form.trash_retention_days" :min="1" data-test="f-trash-days" />
              <span class="setting-help">{{ t('admin.helpTrashDays') }}</span>
              <span v-if="fieldErrors.trash_retention_days" class="setting-error">{{ fieldErrors.trash_retention_days }}</span>
            </label>
            <label class="setting-field">{{ t('admin.fieldAllowedExts') }}
              <input v-model="form.allowed_extensions" data-test="f-exts" class="setting-input" placeholder="png,jpg,pdf" />
              <span class="setting-help">{{ t('admin.helpAllowedExts') }}</span>
              <span v-if="fieldErrors.allowed_extensions" class="setting-error">{{ fieldErrors.allowed_extensions }}</span>
            </label>
          </div>
        </section>
        <div class="flex items-center gap-3 pb-8">
          <button class="px-4 py-2 bg-[var(--color-primary)] text-white rounded" data-test="admin-save" @click="saveSettings">
            {{ t('common.save') }}
          </button>
          <span class="setting-help" data-test="settings-change-state">{{ changedPatch ? t('admin.unsavedChanges') : t('admin.allSaved') }}</span>
        </div>
      </div>
    </template>

    <template #users>
      <div class="space-y-5" data-test="admin-users-panel">
        <header class="admin-content-heading">
          <div>
            <h1>{{ t('admin.users') }}</h1>
            <p>{{ t('admin.usersIntro') }}</p>
          </div>
          <span class="admin-users-count">{{ t('admin.usersCount', { count: users.length }) }}</span>
        </header>
        <div class="admin-user-toolbar">
          <label class="admin-user-search">
            <span class="sr-only">{{ t('admin.userSearchPlaceholder') }}</span>
            <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="10.8" cy="10.8" r="6.3" /><path d="m16 16 4.2 4.2" /></svg>
            <input
              v-model="userQuery"
              type="search"
              :placeholder="t('admin.userSearchPlaceholder')"
              data-test="user-search"
              @keydown.enter.prevent="loadUsers"
              @input="loadUsers"
            />
          </label>
          <span>{{ t('admin.userSearchHint') }}</span>
        </div>
        <div class="admin-table-scroll" :aria-busy="usersLoading" data-test="users-table-scroll">
          <table data-test="admin-users" class="admin-user-table">
            <thead>
              <tr>
                <th>{{ t('admin.colEmail') }}</th>
                <th>{{ t('admin.colName') }}</th>
                <th>{{ t('admin.colRole') }}</th>
                <th>{{ t('admin.colStatus') }}</th>
                <th><span class="sr-only">{{ t('admin.userActions') }}</span></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="u in users" :key="u.id">
                <td>
                  <span class="admin-user-email">{{ u.email }}</span>
                  <span class="admin-user-id" :title="u.id">{{ u.id }}</span>
                </td>
                <td>{{ u.display_name || '—' }}</td>
                <td>
                  <select
                    :value="u.role"
                    class="admin-role-select"
                    :aria-label="t('admin.userRoleFor', { name: u.display_name || u.email })"
                    @change="changeRole(u, ($event.target as HTMLSelectElement).value as UserRow['role'])"
                  >
                    <option value="viewer">{{ t('admin.roleViewer') }}</option>
                    <option value="editor">{{ t('admin.roleEditor') }}</option>
                    <option value="admin">{{ t('admin.roleAdmin') }}</option>
                  </select>
                </td>
                <td>
                  <span class="admin-user-status" :class="u.status === 'active' ? 'is-active' : 'is-disabled'">
                    <span aria-hidden="true" />{{ u.status === 'active' ? t('admin.statusActive') : t('admin.statusDisabled') }}
                  </span>
                </td>
                <td class="admin-user-action-cell">
                  <button
                    class="admin-user-action"
                    :class="u.status === 'active' ? 'is-disable-action' : 'is-enable-action'"
                    data-test="user-toggle"
                    @click="toggleStatus(u)"
                  >{{ u.status === 'active' ? t('admin.disable') : t('admin.enable') }}</button>
                </td>
              </tr>
              <tr v-if="usersLoading && !users.length">
                <td colspan="5" class="admin-table-message">{{ t('common.loading') }}</td>
              </tr>
              <tr v-else-if="!users.length">
                <td colspan="5" class="admin-table-message">
                  {{ userQuery ? t('admin.noUsersMatch') : t('admin.noUsers') }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>

    <template #dashboard>
      <div data-test="admin-dashboard" class="admin-dashboard">
        <header class="admin-content-heading">
          <div>
            <h1>{{ t('admin.dashboard') }}</h1>
            <p>{{ t('admin.dashboardIntro') }}</p>
          </div>
        </header>
        <section class="admin-dashboard-stats" :aria-label="t('admin.siteOverview')">
          <article class="admin-dashboard-stat">
            <span>{{ t('admin.statDocs') }}</span>
            <strong>{{ stats?.documents_total ?? '—' }}</strong>
          </article>
          <article class="admin-dashboard-stat">
            <span>{{ t('admin.statComments') }}</span>
            <strong>{{ stats?.comments_total ?? '—' }}</strong>
          </article>
          <article class="admin-dashboard-stat">
            <span>{{ t('admin.statFiles') }}</span>
            <strong>{{ stats?.attachments_total ?? '—' }}</strong>
          </article>
        </section>
        <div class="admin-dashboard-lists">
          <section class="admin-dashboard-panel" data-test="dash-recent">
            <header class="admin-dashboard-panel-heading">
              <h2>{{ t('admin.recentDocs') }}</h2>
              <span>{{ stats?.recent_docs?.length ?? 0 }}</span>
            </header>
            <ul v-if="stats?.recent_docs?.length" class="admin-dashboard-list">
              <li v-for="d in stats.recent_docs" :key="d.id">
                <RouterLink :to="`/docs/${recentDocPath(d.id, d.slug)}`" class="admin-dashboard-doc">
                  <span class="admin-dashboard-doc-title">{{ d.title }}</span>
                  <time class="admin-dashboard-meta">{{ formatDashboardDate(d.updated_at) }}</time>
                </RouterLink>
              </li>
            </ul>
            <p v-else class="admin-dashboard-empty">{{ t('admin.noRecentDocs') }}</p>
          </section>
          <section class="admin-dashboard-panel" data-test="dash-contributors">
            <header class="admin-dashboard-panel-heading">
              <h2>{{ t('admin.contributors') }}</h2>
              <span>{{ stats?.contributors?.length ?? 0 }}</span>
            </header>
            <ul v-if="stats?.contributors?.length" class="admin-dashboard-list">
              <li v-for="c in stats.contributors" :key="c.user_id" class="admin-dashboard-contributor">
                <span class="admin-dashboard-contributor-name">{{ c.name || c.user_id }}</span>
                <span class="admin-dashboard-count">{{ c.count }}</span>
              </li>
            </ul>
            <p v-else class="admin-dashboard-empty">{{ t('admin.noContributors') }}</p>
          </section>
        </div>
      </div>
    </template>

    <template #backups>
      <div data-test="admin-backups" class="space-y-4 max-w-xl">
        <div class="flex flex-wrap gap-2">
          <button class="px-3 py-1 bg-blue-600 text-white rounded" data-test="btn-start-backup" :disabled="backupBusy" @click="startBackup">
            {{ t('admin.startBackup') }}
          </button>
          <button class="px-3 py-1 border rounded" data-test="btn-import-zip" :disabled="backupBusy" @click="pickFile('.zip', importBackupZip)">
            {{ t('admin.importBackup') }}
          </button>
          <button class="px-3 py-1 border rounded" data-test="btn-import-md" :disabled="backupBusy" @click="pickFile('.zip', importMarkdownZip)">
            {{ t('admin.importMd') }}
          </button>
        </div>
        <p v-if="jobLine" class="text-xs text-[var(--color-text)]" data-test="job-line">{{ jobLine }}</p>
        <ul class="text-sm space-y-1">
          <li v-for="f in backupFiles" :key="f" class="flex gap-2 items-center">
            {{ f }}
            <a :href="adminApi.backupDownloadURL(f)" class="text-blue-600" data-test="backup-download">{{ t('attachments.download') }}</a>
            <button class="text-red-600" data-test="backup-delete" @click="removeBackup(f)">×</button>
          </li>
        </ul>
      </div>
    </template>
  </AdminTabs>
</template>

<style scoped>
.setting-card {
  padding: 1.25rem;
  border: 1px solid var(--color-border);
  border-radius: 1rem;
  background: var(--color-card-background);
}
.setting-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 18rem), 1fr));
  gap: 1.25rem;
}
.setting-field {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: .35rem;
  font-size: .875rem;
  font-weight: 600;
}
.setting-input {
  width: 100%;
  min-height: 2.25rem;
  padding: .35rem .65rem;
  border: 1px solid var(--color-border);
  border-radius: .4rem;
  background: var(--color-card-background);
  color: var(--color-text);
  font-weight: 400;
}
.setting-help, .setting-toggle small {
  color: var(--color-text-light);
  font-size: .8rem;
  font-weight: 400;
  line-height: 1.45;
}
.setting-error {
  color: #dc2626;
  font-size: .8rem;
  font-weight: 400;
}
.setting-toggle {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 1rem;
  padding: .85rem 0;
  font-size: .875rem;
}
.setting-toggle span { display: flex; flex-direction: column; gap: .15rem; }
.setting-toggle strong { font-weight: 600; }

.admin-content-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}
.admin-content-heading h1 {
  color: var(--color-text);
  font-size: 1.25rem;
  font-weight: 650;
  line-height: 1.35;
}
.admin-content-heading p {
  margin-top: .3rem;
  color: var(--color-text-light);
  font-size: .875rem;
  line-height: 1.5;
}
.admin-users-count,
.admin-dashboard-panel-heading > span {
  flex: none;
  padding: .3rem .65rem;
  border-radius: 999px;
  background: var(--color-background-soft);
  color: var(--color-text-light);
  font-size: .75rem;
  font-weight: 600;
}
.admin-user-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: .7rem 1rem;
}
.admin-user-toolbar > span {
  color: var(--color-text-light);
  font-size: .8rem;
}
.admin-user-search {
  display: flex;
  align-items: center;
  gap: .6rem;
  width: min(100%, 27rem);
  min-height: 2.65rem;
  padding: 0 .8rem;
  border: 1px solid var(--color-border);
  border-radius: .7rem;
  background: var(--color-card-background);
  color: var(--color-text-light);
}
.admin-user-search:focus-within {
  border-color: var(--color-primary);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-primary) 15%, transparent);
}
.admin-user-search svg {
  width: 1.1rem;
  height: 1.1rem;
  flex: none;
  fill: none;
  stroke: currentColor;
  stroke-width: 1.7;
}
.admin-user-search input {
  width: 100%;
  min-width: 0;
  border: 0;
  outline: 0;
  background: transparent;
  color: var(--color-text);
  font-size: .875rem;
}
.admin-user-search input::placeholder { color: var(--color-text-light); }
.admin-table-scroll {
  overflow-x: auto;
  border: 1px solid var(--color-border);
  border-radius: .85rem;
  background: var(--color-card-background);
}
.admin-user-table {
  width: 100%;
  min-width: 55rem;
  border-collapse: collapse;
  color: var(--color-text);
  font-size: .875rem;
  text-align: left;
}
.admin-user-table th {
  padding: .8rem 1rem;
  background: var(--color-background-soft);
  color: var(--color-text-light);
  font-size: .75rem;
  font-weight: 650;
  letter-spacing: .025em;
  white-space: nowrap;
}
.admin-user-table td {
  padding: .85rem 1rem;
  border-top: 1px solid var(--color-border);
  vertical-align: middle;
}
.admin-user-table tbody tr:hover { background: color-mix(in srgb, var(--color-background-soft) 55%, transparent); }
.admin-user-email { display: block; font-weight: 550; }
.admin-user-id {
  display: block;
  max-width: 14rem;
  overflow: hidden;
  color: var(--color-text-light);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: .7rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.admin-role-select {
  min-width: 8rem;
  padding: .45rem 1.8rem .45rem .65rem;
  border: 1px solid var(--color-border);
  border-radius: .55rem;
  background: var(--color-card-background);
  color: var(--color-text);
  font-size: .8rem;
}
.admin-user-status {
  display: inline-flex;
  align-items: center;
  gap: .45rem;
  padding: .3rem .6rem;
  border-radius: 999px;
  font-size: .75rem;
  font-weight: 600;
  white-space: nowrap;
}
.admin-user-status > span {
  width: .45rem;
  height: .45rem;
  border-radius: 999px;
  background: currentColor;
}
.admin-user-status.is-active { background: color-mix(in srgb, #16a34a 12%, var(--color-card-background)); color: #15803d; }
.admin-user-status.is-disabled { background: color-mix(in srgb, #dc2626 10%, var(--color-card-background)); color: #b91c1c; }
.dark .admin-user-status.is-active { background: color-mix(in srgb, #22c55e 15%, var(--color-card-background)); color: #86efac; }
.dark .admin-user-status.is-disabled { background: color-mix(in srgb, #ef4444 15%, var(--color-card-background)); color: #fca5a5; }
.admin-user-action-cell { text-align: right; }
.admin-user-action {
  padding: .4rem .7rem;
  border: 1px solid var(--color-border);
  border-radius: .55rem;
  background: var(--color-card-background);
  font-size: .8rem;
  font-weight: 550;
}
.admin-user-action:hover { background: var(--color-background-soft); }
.admin-user-action.is-disable-action { color: #b91c1c; }
.admin-user-action.is-enable-action { color: #15803d; }
.dark .admin-user-action.is-disable-action { color: #fca5a5; }
.dark .admin-user-action.is-enable-action { color: #86efac; }
.admin-table-message {
  padding: 2.5rem 1rem !important;
  color: var(--color-text-light);
  text-align: center;
}

.admin-dashboard { display: flex; flex-direction: column; gap: 1.25rem; }
.admin-dashboard-stats {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(100%, 14rem), 1fr));
  gap: 1rem;
}
.admin-dashboard-stat {
  display: flex;
  flex-direction: column;
  gap: .45rem;
  min-height: 7.25rem;
  padding: 1.1rem 1.2rem;
  border: 1px solid var(--color-border);
  border-radius: .9rem;
  background: var(--color-card-background);
  box-shadow: 0 4px 14px rgb(15 23 42 / 4%);
}
.admin-dashboard-stat span {
  color: var(--color-text-light);
  font-size: .8rem;
  font-weight: 550;
}
.admin-dashboard-stat strong {
  color: var(--color-text);
  font-size: 1.8rem;
  font-weight: 700;
  line-height: 1.1;
  font-variant-numeric: tabular-nums;
}
.admin-dashboard-lists {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1rem;
}
.admin-dashboard-panel {
  min-width: 0;
  padding: 1rem 1.15rem;
  border: 1px solid var(--color-border);
  border-radius: .9rem;
  background: var(--color-card-background);
}
.admin-dashboard-panel-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: .8rem;
  padding-bottom: .8rem;
  border-bottom: 1px solid var(--color-border);
}
.admin-dashboard-panel-heading h2 { font-size: .95rem; font-weight: 650; }
.admin-dashboard-list { margin: 0; padding: 0; list-style: none; }
.admin-dashboard-list > li + li { border-top: 1px solid var(--color-border); }
.admin-dashboard-doc,
.admin-dashboard-contributor {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  min-width: 0;
  padding: .75rem 0;
}
.admin-dashboard-doc { text-decoration: none; }
.admin-dashboard-doc:hover .admin-dashboard-doc-title { color: var(--color-primary); text-decoration: underline; }
.admin-dashboard-doc-title,
.admin-dashboard-contributor-name {
  min-width: 0;
  overflow: hidden;
  color: var(--color-text);
  font-size: .875rem;
  font-weight: 550;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.admin-dashboard-meta { flex: none; color: var(--color-text-light); font-size: .75rem; }
.admin-dashboard-count {
  flex: none;
  min-width: 2rem;
  padding: .25rem .5rem;
  border-radius: .5rem;
  background: var(--color-background-soft);
  color: var(--color-text-light);
  font-size: .75rem;
  font-weight: 650;
  text-align: center;
  font-variant-numeric: tabular-nums;
}
.admin-dashboard-empty { padding: 1.75rem .25rem; color: var(--color-text-light); font-size: .85rem; text-align: center; }

@media (max-width: 720px) {
  .admin-dashboard-lists { grid-template-columns: minmax(0, 1fr); }
  .admin-content-heading { align-items: center; }
}
</style>
