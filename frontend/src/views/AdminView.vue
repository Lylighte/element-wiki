<script setup lang="ts">
// 管理视图：按权限码显隐 Tab；各域面板内联实现（T7.8）。
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { adminApi, type DashboardStats } from '@/api'
import { can } from '@/permissions'
import AdminTabs from '@/components/admin/AdminTabs.vue'
import siteStore from '@/stores/site'

const perm = reactive({ has: (code: string) => can(code) })
const { t } = useI18n()

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
const original = ref<SettingsForm>({ ...form })
const fieldErrors = ref<Record<string, string>>({})

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
async function loadUsers() {
  const r = await adminApi.users(userQuery.value)
  users.value = r.items as UserRow[]
}
async function changeRole(u: UserRow, role: UserRow['role']) {
  await adminApi.updateUser(u.id, { role })
  await loadUsers()
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
  await adminApi.updateUser(u.id, { status: next })
  await loadUsers()
}

// dashboard
const stats = ref<DashboardStats | null>(null)

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
    const done = await pollUntilDone(job_id, adminApi.importJob)
    if (done.status === 'failed') ElMessage.error(done.last_error || t('admin.jobFailed'))
    else ElMessage.success(t('admin.importDone'))
  } catch (err) {
    showJobError(err)
  } finally {
    backupBusy.value = false
  }
}

async function importMarkdownZip(f: File) {
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

onMounted(async () => {
  const loads: Promise<void>[] = []
  if (can('settings.manage')) loads.push(loadSettings())
  if (can('user.list')) loads.push(loadUsers())
  if (can('dashboard.read'))
      loads.push(
        adminApi
          .dashboard()
          .then((st) => {
            stats.value = st
          })
          .then(() => undefined),
      )
  if (can('backup.manage')) loads.push(adminApi.backupFiles().then((f) => {
        backupFiles.value = f.items
      }))
  await Promise.allSettled(loads)
})

async function removeBackup(f: string) {
  await adminApi.deleteBackupFile(f)
  backupFiles.value = (await adminApi.backupFiles()).items
}
</script>

<template>
  <AdminTabs :perm="perm">
    <template #settings>
      <div class="space-y-3 max-w-lg" data-test="admin-settings">
        <label class="block text-sm">wiki_title
          <input v-model="form.wiki_title" data-test="f-wiki-title" class="border rounded w-full px-2 py-1" />
          <span v-if="fieldErrors.wiki_title" class="text-red-600 text-xs">{{ fieldErrors.wiki_title }}</span>
        </label>
        <label class="flex items-center gap-2 text-sm">anonymous_read
          <el-switch v-model="form.anonymous_read" data-test="f-anon" />
        </label>
        <label class="flex items-center gap-2 text-sm">comments_enabled
          <el-switch v-model="form.comments_enabled" data-test="f-comments" />
        </label>
        <label class="block text-sm">max_versions
          <el-input-number v-model="form.max_versions" :min="1" data-test="f-max-versions" />
        </label>
        <label class="block text-sm">upload_max_mb
          <el-input-number v-model="form.upload_max_mb" :min="1" data-test="f-upload-max" />
        </label>
        <label class="block text-sm">trash_retention_days
          <el-input-number v-model="form.trash_retention_days" :min="1" data-test="f-trash-days" />
        </label>
        <label class="block text-sm">allowed_extensions
          <input v-model="form.allowed_extensions" data-test="f-exts" class="border rounded w-full px-2 py-1" />
        </label>
        <label class="block text-sm">default_lang
          <select v-model="form.default_lang" data-test="f-lang" class="border rounded px-2 py-1">
            <option value="zh-CN">zh-CN</option>
            <option value="en">en</option>
          </select>
        </label>
        <label class="block text-sm">timezone
          <input v-model="form.timezone" data-test="f-tz" class="border rounded w-full px-2 py-1" />
          <span v-if="fieldErrors.timezone" class="text-red-600 text-xs">{{ fieldErrors.timezone }}</span>
        </label>
        <button class="px-3 py-1 bg-blue-600 text-white rounded" data-test="admin-save" @click="saveSettings">
          {{ t('common.save') }}
        </button>
      </div>
    </template>

    <template #users>
      <div class="flex gap-2 mb-2 max-w-md">
        <input
          v-model="userQuery"
          :placeholder="t('search.placeholder')"
          data-test="user-search"
          class="border rounded px-2 py-1 flex-1"
          @keydown.enter="loadUsers"
          @input="loadUsers"
        />
      </div>
      <table data-test="admin-users" class="text-sm w-full">
        <thead><tr><th>ID</th><th>{{ t('admin.colEmail') }}</th><th>{{ t('admin.colName') }}</th><th>{{ t('admin.colRole') }}</th><th>{{ t('admin.colStatus') }}</th></tr></thead>
        <tbody>
          <tr v-for="u in users" :key="u.id">
            <td>{{ u.id }}</td><td>{{ u.email }}</td><td>{{ u.display_name }}</td>
            <td>
              <select :value="u.role" @change="changeRole(u, ($event.target as HTMLSelectElement).value as UserRow['role'])">
                <option value="viewer">viewer</option>
                <option value="editor">editor</option>
                <option value="admin">admin</option>
              </select>
            </td>
            <td>
              <button class="underline" data-test="user-toggle" @click="toggleStatus(u)">
                {{ u.status === 'active' ? t('admin.disable') : t('admin.enable') }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </template>

    <template #dashboard>
      <div data-test="admin-dashboard" class="space-y-5">
        <div class="grid grid-cols-3 gap-4 max-w-md text-center">
          <div class="border rounded p-3"><div class="text-2xl">{{ stats?.documents_total ?? '-' }}</div>{{ t('admin.statDocs') }}</div>
          <div class="border rounded p-3"><div class="text-2xl">{{ stats?.comments_total ?? '-' }}</div>{{ t('admin.statComments') }}</div>
          <div class="border rounded p-3"><div class="text-2xl">{{ stats?.attachments_total ?? '-' }}</div>{{ t('admin.statFiles') }}</div>
        </div>
        <div class="grid grid-cols-2 gap-6 max-w-2xl text-sm">
          <div data-test="dash-recent">
            <p class="font-semibold mb-1">{{ t('admin.recentDocs') }}</p>
            <ul class="space-y-1">
              <li v-for="d in stats?.recent_docs ?? []" :key="d.id" class="truncate">
                <RouterLink :to="`/docs/${d.id}`" class="hover:underline">{{ d.title }}</RouterLink>
              </li>
            </ul>
          </div>
          <div data-test="dash-contributors">
            <p class="font-semibold mb-1">{{ t('admin.contributors') }}</p>
            <ul class="space-y-1">
              <li v-for="c in stats?.contributors ?? []" :key="c.user_id" class="flex justify-between gap-3">
                <span>{{ c.name || c.user_id }}</span><span class="text-gray-400">{{ c.count }}</span>
              </li>
            </ul>
          </div>
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
        <p v-if="jobLine" class="text-xs text-gray-500" data-test="job-line">{{ jobLine }}</p>
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
