<script setup lang="ts">
// 管理视图：按权限码显隐 Tab；各域面板内联实现（T7.8）。
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { adminApi, reviewApi, siteApi, type DashboardStats, type HiddenContent, type PendingComment, type PendingContentReport, type PendingDocumentSubmission, type PendingUserPage } from '@/api'
import { can } from '@/permissions'
import AdminTabs from '@/components/admin/AdminTabs.vue'
import TreeAdminPanel from '@/components/admin/TreeAdminPanel.vue'
import siteStore from '@/stores/site'
import treeStore from '@/stores/tree'
import { Notebook } from '@element-plus/icons-vue'

const perm = reactive({ has: (code: string) => can(code) })
const { t, locale } = useI18n()

// settings（T11.2）：九键类型化表单，仅提交变更键
interface SettingsForm {
  wiki_title: string
  timezone: string
  default_lang: 'zh-CN' | 'en'
  anonymous_read: boolean
  comments_enabled: boolean
  user_pages_enabled: boolean
  user_pages_review_required: boolean
  document_review_required: boolean
  comment_review_required: boolean
  deployment_preset: 'internal' | 'public_readonly' | 'public_contributions'
  max_versions: number
  upload_max_mb: number
  trash_retention_days: number
  allowed_extensions: string
  site_icon_url: string
  theme_preset: 'blue'
  theme_light_primary: string
  theme_light_accent: string
  theme_light_focus: string
  theme_dark_primary: string
  theme_dark_accent: string
  theme_dark_focus: string
  article_footer_markdown: string
  sidebar_footer_markdown: string
}
const form = reactive<SettingsForm>({
  wiki_title: '', timezone: '', default_lang: 'zh-CN',
  anonymous_read: false, comments_enabled: false,
  user_pages_enabled: false, user_pages_review_required: false,
  document_review_required: false, comment_review_required: false, deployment_preset: 'internal',
  max_versions: 100, upload_max_mb: 20, trash_retention_days: 30,
  allowed_extensions: '',
  site_icon_url: '', theme_preset: 'blue',
  theme_light_primary: '#2563EB', theme_light_accent: '#DBEAFE', theme_light_focus: '#2563EB',
  theme_dark_primary: '#60A5FA', theme_dark_accent: '#1E3A5F', theme_dark_focus: '#93C5FD',
  article_footer_markdown: '', sidebar_footer_markdown: '',
})
const iconMode = ref<'upload' | 'url'>('upload')
const uploadingIcon = ref(false)
const iconError = ref('')
const iconPreviewFailed = ref(false)
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
const themeColorGroups = [
  {
    mode: 'light', label: 'admin.themeLight', items: [
      { key: 'theme_light_primary', label: 'admin.themePrimary' },
      { key: 'theme_light_accent', label: 'admin.themeAccent' },
      { key: 'theme_light_focus', label: 'admin.themeFocus' },
    ],
  },
  {
    mode: 'dark', label: 'admin.themeDark', items: [
      { key: 'theme_dark_primary', label: 'admin.themePrimary' },
      { key: 'theme_dark_accent', label: 'admin.themeAccent' },
      { key: 'theme_dark_focus', label: 'admin.themeFocus' },
    ],
  },
] as const
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
const pendingUserPages = ref<PendingUserPage[]>([])
const pendingComments = ref<PendingComment[]>([])
const pendingDocuments = ref<PendingDocumentSubmission[]>([])
const pendingReports = ref<PendingContentReport[]>([])
const hiddenContent = ref<HiddenContent[]>([])
const reviewsLoading = ref(false)

function loadIntoForm(raw: Record<string, string>) {
  form.wiki_title = raw.wiki_title ?? ''
  form.timezone = raw.timezone ?? ''
  form.default_lang = raw.default_lang === 'en' ? 'en' : 'zh-CN'
  form.anonymous_read = raw.anonymous_read === 'true'
  form.comments_enabled = raw.comments_enabled === 'true'
  form.user_pages_enabled = raw.user_pages_enabled === 'true'
  form.user_pages_review_required = raw.user_pages_review_required === 'true'
  form.document_review_required = raw.document_review_required === 'true'
  form.comment_review_required = raw.comment_review_required === 'true'
  form.deployment_preset = (raw.deployment_preset as SettingsForm['deployment_preset']) || 'internal'
  form.max_versions = Number(raw.max_versions) || 100
  form.upload_max_mb = Number(raw.upload_max_mb) || 20
  form.trash_retention_days = Number(raw.trash_retention_days) || 30
  form.allowed_extensions = raw.allowed_extensions ?? ''
  form.site_icon_url = raw.site_icon_url ?? ''
  form.theme_preset = 'blue'
  form.theme_light_primary = raw.theme_light_primary || '#2563EB'
  form.theme_light_accent = raw.theme_light_accent || '#DBEAFE'
  form.theme_light_focus = raw.theme_light_focus || '#2563EB'
  form.theme_dark_primary = raw.theme_dark_primary || '#60A5FA'
  form.theme_dark_accent = raw.theme_dark_accent || '#1E3A5F'
  form.theme_dark_focus = raw.theme_dark_focus || '#93C5FD'
  form.article_footer_markdown = raw.article_footer_markdown ?? ''
  form.sidebar_footer_markdown = raw.sidebar_footer_markdown ?? ''
  iconMode.value = /^https?:\/\//i.test(form.site_icon_url) ? 'url' : 'upload'
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
  for (const key of ['user_pages_enabled', 'user_pages_review_required', 'document_review_required', 'comment_review_required'] as const) {
    if (form[key] !== original.value[key]) patch[key] = String(form[key])
  }
  if (form.deployment_preset !== original.value.deployment_preset) patch.deployment_preset = form.deployment_preset
  if (form.max_versions !== original.value.max_versions)
    patch.max_versions = String(form.max_versions)
  if (form.upload_max_mb !== original.value.upload_max_mb)
    patch.upload_max_mb = String(form.upload_max_mb)
  if (form.trash_retention_days !== original.value.trash_retention_days)
    patch.trash_retention_days = String(form.trash_retention_days)
  if (form.allowed_extensions !== original.value.allowed_extensions)
    patch.allowed_extensions = form.allowed_extensions
  if (form.site_icon_url !== original.value.site_icon_url) patch.site_icon_url = form.site_icon_url
  if (form.theme_preset !== original.value.theme_preset) patch.theme_preset = form.theme_preset
  for (const key of [
    'theme_light_primary', 'theme_light_accent', 'theme_light_focus',
    'theme_dark_primary', 'theme_dark_accent', 'theme_dark_focus',
    'article_footer_markdown', 'sidebar_footer_markdown',
  ] as const) {
    if (form[key] !== original.value[key]) patch[key] = form[key]
  }
  return Object.keys(patch).length ? patch : null
})

const iconPreviewURL = computed(() => {
  const value = form.site_icon_url.trim()
  if (value.startsWith('/v1/site/icon/')) return value
  try {
    const url = new URL(value)
    return url.protocol === 'http:' || url.protocol === 'https:' ? value : ''
  } catch { return '' }
})
watch(iconPreviewURL, () => { iconPreviewFailed.value = false })

async function applyDeploymentPreset(value: SettingsForm['deployment_preset']) {
  try {
    await ElMessageBox.confirm(t('admin.presetConfirm'), t('admin.deploymentPreset'), { type: 'warning' })
  } catch { return }
  form.deployment_preset = value
  if (value === 'internal') {
    form.anonymous_read = false
    form.comments_enabled = false
    form.user_pages_enabled = false
    form.document_review_required = false
    form.comment_review_required = false
    form.user_pages_review_required = false
  } else if (value === 'public_readonly') {
    form.anonymous_read = true
    form.comments_enabled = false
    form.user_pages_enabled = false
    form.document_review_required = false
    form.comment_review_required = false
    form.user_pages_review_required = false
  } else {
    form.anonymous_read = true
    form.comments_enabled = true
    form.user_pages_enabled = true
    form.document_review_required = true
    form.comment_review_required = true
    form.user_pages_review_required = true
  }
}

function onIconPreviewError() {
  iconPreviewFailed.value = true
}

async function uploadSiteIcon(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  const ext = file.name.split('.').pop()?.toLowerCase()
  if (!['png', 'ico', 'webp'].includes(ext ?? '')) {
    iconError.value = t('admin.iconTypeError')
    return
  }
  if (file.size > form.upload_max_mb * 1024 * 1024) {
    iconError.value = t('admin.iconSizeError', { size: form.upload_max_mb })
    return
  }
  uploadingIcon.value = true
  iconError.value = ''
  try {
    const result = await siteApi.uploadIcon(file)
    form.site_icon_url = result.site_icon_url
    original.value = { ...original.value, site_icon_url: result.site_icon_url }
    siteStore.setSiteIconURL(result.site_icon_url)
    ElMessage.success(t('admin.iconUploaded'))
  } catch (error) {
    const apiError = error as { status?: number }
    iconError.value = apiError.status === 413 ? t('admin.iconSizeError', { size: form.upload_max_mb }) : t('admin.iconUploadFailed')
  } finally {
    uploadingIcon.value = false
  }
}

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
    siteStore.setTitle(form.wiki_title)
    siteStore.setTimezone(form.timezone)
    siteStore.setCommentsEnabled(form.comments_enabled)
    siteStore.setUserPagesEnabled(form.user_pages_enabled)
    siteStore.setSiteIconURL(form.site_icon_url)
    siteStore.setThemeColors({
      theme_preset: form.theme_preset,
      theme_light_primary: form.theme_light_primary,
      theme_light_accent: form.theme_light_accent,
      theme_light_focus: form.theme_light_focus,
      theme_dark_primary: form.theme_dark_primary,
      theme_dark_accent: form.theme_dark_accent,
      theme_dark_focus: form.theme_dark_focus,
    })
    try {
      const site = await siteApi.info()
      siteStore.setFooterHTML(site.article_footer_html, site.sidebar_footer_html)
    } catch { /* settings are saved; existing footer stays visible until the next site load */ }
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
  if (can('review.manage')) loads.push(loadReviews())
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

async function loadReviews() {
  reviewsLoading.value = true
  try {
    const [pages, comments, documents, reports, hidden] = await Promise.all([reviewApi.pendingUserPages(), reviewApi.pendingComments(), reviewApi.pendingDocuments(), reviewApi.pendingReports(), reviewApi.hiddenContent()])
    pendingUserPages.value = pages.items
    pendingComments.value = comments.items
    pendingDocuments.value = documents.items
    pendingReports.value = reports.items
    hiddenContent.value = hidden.items
  }
  catch { operationError.value = true }
  finally { reviewsLoading.value = false }
}

async function decideReport(item: PendingContentReport, action: 'resolve' | 'dismiss') {
  try {
    const { value } = await ElMessageBox.prompt(t('admin.reportResolution'), t(action === 'resolve' ? 'admin.reportResolve' : 'admin.reportDismiss'), { inputType: 'textarea', inputValidator: (v) => !!v?.trim() })
    if (action === 'resolve') await reviewApi.resolveReport(item.id, value)
    else await reviewApi.dismissReport(item.id, value)
    ElMessage.success(t('admin.reportHandled'))
    await loadReviews()
  } catch { /* prompt cancel */ }
}
async function unpublishReportTarget(item: PendingContentReport) {
  try {
    const { value } = await ElMessageBox.prompt(t('admin.unpublishReason'), t('admin.unpublish'), { inputType: 'textarea', inputValidator: (v) => !!v?.trim() })
    await reviewApi.unpublish(item.content_type, item.content_id, value)
    await reviewApi.resolveReport(item.id, value)
    ElMessage.success(t('admin.contentUnpublished'))
    await loadReviews()
  } catch { /* prompt cancel */ }
}
async function restoreContent(item: HiddenContent) {
  try {
    const { value } = await ElMessageBox.prompt(t('admin.restoreReason'), t('admin.restoreContent'), { inputType: 'textarea', inputValidator: (v) => !!v?.trim() })
    await reviewApi.restoreContent(item.content_type, item.content_id, value)
    ElMessage.success(t('admin.contentRestored'))
    await loadReviews()
  } catch { /* prompt cancel */ }
}

async function approveDocument(item: PendingDocumentSubmission) {
  try { await reviewApi.approveDocument(item.id); ElMessage.success(t('admin.reviewApproved')); await loadReviews() }
  catch { operationError.value = true }
}
async function rejectDocument(item: PendingDocumentSubmission) {
  try {
    const { value } = await ElMessageBox.prompt(t('admin.rejectReason'), t('admin.reviewReject'), { inputType: 'textarea', inputValidator: (v) => !!v?.trim() })
    await reviewApi.rejectDocument(item.id, value); ElMessage.success(t('admin.reviewRejected')); await loadReviews()
  } catch { /* prompt cancel */ }
}

async function approveUserPage(item: PendingUserPage) {
  try {
    await reviewApi.approveUserPage(item.user_id, item.id)
    ElMessage.success(t('admin.reviewApproved'))
    await loadReviews()
  } catch { operationError.value = true }
}

async function rejectUserPage(item: PendingUserPage) {
  try {
    const { value } = await ElMessageBox.prompt(t('admin.rejectReason'), t('admin.reviewReject'), { inputType: 'textarea', inputValidator: (v) => !!v?.trim() })
    await reviewApi.rejectUserPage(item.user_id, item.id, value)
    ElMessage.success(t('admin.reviewRejected'))
    await loadReviews()
  } catch { /* prompt cancel */ }
}

async function approveComment(item: PendingComment) {
  try {
    await reviewApi.approveComment(item.id)
    ElMessage.success(t('admin.reviewApproved'))
    await loadReviews()
  } catch { operationError.value = true }
}

async function rejectComment(item: PendingComment) {
  try {
    const { value } = await ElMessageBox.prompt(t('admin.rejectReason'), t('admin.reviewReject'), { inputType: 'textarea', inputValidator: (v) => !!v?.trim() })
    await reviewApi.rejectComment(item.id, value)
    ElMessage.success(t('admin.reviewRejected'))
    await loadReviews()
  } catch { /* prompt cancel */ }
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
            <label class="setting-field">{{ t('admin.deploymentPreset') }}
              <select :value="form.deployment_preset" class="setting-input" data-test="f-deployment-preset" @change="applyDeploymentPreset(($event.target as HTMLSelectElement).value as SettingsForm['deployment_preset'])">
                <option value="internal">{{ t('admin.presetInternal') }}</option>
                <option value="public_readonly">{{ t('admin.presetPublicReadonly') }}</option>
                <option value="public_contributions">{{ t('admin.presetPublicContributions') }}</option>
              </select>
              <span class="setting-help">{{ t('admin.presetHelp') }}</span>
            </label>
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
            <label class="setting-toggle">
              <span><strong>{{ t('admin.userPagesEnabled') }}</strong><small>{{ t('admin.userPagesEnabledHelp') }}</small></span>
              <el-switch v-model="form.user_pages_enabled" data-test="f-user-pages" />
            </label>
            <span v-if="fieldErrors.user_pages_enabled" class="setting-error">{{ fieldErrors.user_pages_enabled }}</span>
            <label class="setting-toggle">
              <span><strong>{{ t('admin.reviewRequired', { content: t('admin.userPages') }) }}</strong><small>{{ t('admin.reviewRequiredHelp') }}</small></span>
              <el-switch v-model="form.user_pages_review_required" data-test="f-user-pages-review" />
            </label>
            <label class="setting-toggle">
              <span><strong>{{ t('admin.reviewRequired', { content: t('admin.documents') }) }}</strong><small>{{ t('admin.reviewRequiredHelp') }}</small></span>
              <el-switch v-model="form.document_review_required" data-test="f-document-review" />
            </label>
            <label class="setting-toggle">
              <span><strong>{{ t('admin.reviewRequired', { content: t('admin.comments') }) }}</strong><small>{{ t('admin.reviewRequiredHelp') }}</small></span>
              <el-switch v-model="form.comment_review_required" data-test="f-comment-review" />
            </label>
          </div>
        </section>
        <section class="setting-card space-y-5" data-test="site-brand-settings">
          <div>
            <h2 class="text-base font-semibold">{{ t('admin.siteBrand') }}</h2>
            <p class="setting-help mt-1">{{ t('admin.siteBrandHelp') }}</p>
          </div>
          <div class="grid gap-4 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
            <div class="setting-field">
              <span class="font-medium">{{ t('admin.siteIcon') }}</span>
              <el-radio-group v-model="iconMode" class="mt-2" data-test="site-icon-mode">
                <el-radio-button value="upload">{{ t('admin.iconUpload') }}</el-radio-button>
                <el-radio-button value="url">{{ t('admin.iconURL') }}</el-radio-button>
              </el-radio-group>
              <label v-if="iconMode === 'url'" class="setting-field mt-3">
                {{ t('admin.iconURL') }}
                <input v-model="form.site_icon_url" type="url" class="setting-input" placeholder="https://example.com/icon.png" data-test="site-icon-url" />
              </label>
              <label v-else class="setting-field mt-3">
                <span>{{ t('admin.iconUploadHelp', { size: form.upload_max_mb }) }}</span>
                <input type="file" accept=".png,.ico,.webp,image/png,image/x-icon,image/webp" class="setting-input" :disabled="uploadingIcon" data-test="site-icon-file" @change="uploadSiteIcon" />
              </label>
              <p v-if="iconError" class="setting-error" role="alert">{{ iconError }}</p>
              <span v-if="fieldErrors.site_icon_url" class="setting-error">{{ fieldErrors.site_icon_url }}</span>
            </div>
            <div class="flex items-center gap-3 rounded-lg border border-[var(--color-border)] bg-[var(--color-background)] p-3" data-test="site-icon-preview">
              <img v-if="iconPreviewURL && !iconPreviewFailed" :src="iconPreviewURL" class="site-brand-icon" alt="" referrerpolicy="no-referrer" @error="onIconPreviewError" />
              <Notebook v-else class="site-brand-icon-default" aria-hidden="true" />
              <span class="text-sm">{{ t('admin.iconPreview') }}</span>
            </div>
          </div>
          <div class="setting-field max-w-xs">
            <span>{{ t('admin.themePreset') }}</span>
            <div class="flex items-center gap-2 rounded-lg border border-[var(--color-border)] px-3 py-2" data-test="theme-preset">
              <span class="h-4 w-4 rounded-full border border-black/10" style="background:#2563EB" aria-hidden="true" />
              <strong>{{ t('admin.themeBlue') }}</strong>
              <span class="ml-auto text-xs font-normal text-[var(--color-text-light)]">{{ t('admin.themeCurrent') }}</span>
            </div>
          </div>
          <details class="rounded-lg border border-[var(--color-border)] p-4" data-test="theme-advanced">
            <summary class="cursor-pointer font-medium">{{ t('admin.themeAdvanced') }}</summary>
            <div class="mt-4 grid gap-5 lg:grid-cols-2">
              <fieldset v-for="group in themeColorGroups" :key="group.mode" class="grid grid-cols-1 gap-3">
                <legend class="mb-2 font-semibold">{{ t(group.label) }}</legend>
                <label v-for="item in group.items" :key="item.key" class="flex items-center justify-between gap-3 text-sm">
                  <span>{{ t(item.label) }}</span>
                  <span class="flex items-center gap-2">
                    <input v-model="form[item.key]" type="color" class="h-9 w-12 cursor-pointer rounded border border-[var(--color-border)] bg-transparent" :data-test="item.key" />
                    <code>{{ form[item.key] }}</code>
                  </span>
                </label>
              </fieldset>
            </div>
          </details>
          <div class="grid gap-4 lg:grid-cols-2">
            <label class="setting-field">{{ t('admin.articleFooter') }}
              <textarea v-model="form.article_footer_markdown" rows="5" class="setting-input font-mono text-sm" data-test="article-footer-markdown" />
              <span class="setting-help">{{ t('admin.footerMarkdownHelp') }}</span>
            </label>
            <label class="setting-field">{{ t('admin.icpFooter') }}
              <textarea v-model="form.sidebar_footer_markdown" rows="5" class="setting-input font-mono text-sm" data-test="sidebar-footer-markdown" />
              <span class="setting-help">{{ t('admin.footerMarkdownHelp') }}</span>
            </label>
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

    <template #reviews>
      <section class="space-y-5" data-test="admin-reviews">
        <header class="admin-content-heading"><div><h1>{{ t('admin.reviews') }}</h1><p>{{ t('admin.reviewsIntro') }}</p></div><button class="rounded border px-3 py-2" @click="loadReviews">{{ t('common.retry') }}</button></header>
        <p v-if="reviewsLoading" class="text-gray-500">{{ t('common.loading') }}</p>
        <p v-else-if="!pendingUserPages.length && !pendingComments.length && !pendingDocuments.length && !pendingReports.length && !hiddenContent.length" class="rounded-lg border border-dashed p-8 text-center text-gray-500">{{ t('admin.reviewQueueEmpty') }}</p>
        <h2 v-if="pendingReports.length" class="text-base font-semibold">{{ t('admin.contentReports') }}</h2>
        <article v-for="item in pendingReports" :key="item.id" class="rounded-lg border border-[var(--color-border)] p-4">
          <div class="mb-3 flex flex-wrap items-center justify-between gap-2"><div><strong>{{ t(`admin.reportType_${item.content_type}`) }}</strong><span class="ml-2 text-sm text-gray-500">{{ item.content_id }} · {{ item.reporter_id }} · {{ new Date(item.created_at).toLocaleString() }}</span></div><div class="flex gap-2"><button data-test="admin-report-unpublish" class="rounded bg-amber-700 px-3 py-1.5 text-white" @click="unpublishReportTarget(item)">{{ t('admin.unpublish') }}</button><button class="rounded bg-blue-600 px-3 py-1.5 text-white" @click="decideReport(item, 'resolve')">{{ t('admin.reportResolve') }}</button><button class="rounded border px-3 py-1.5" @click="decideReport(item, 'dismiss')">{{ t('admin.reportDismiss') }}</button></div></div>
          <h3 v-if="item.title" class="mb-2 font-semibold">{{ item.title }}</h3>
          <!-- eslint-disable-next-line vue/no-v-html：预览使用与文档相同的服务端消毒渲染 -->
          <div v-if="item.preview_html" class="prose prose-sm max-h-72 overflow-auto rounded bg-[var(--color-background)] p-3" v-html="item.preview_html" />
          <pre v-else-if="item.preview_text" class="max-h-72 overflow-auto whitespace-pre-wrap rounded bg-[var(--color-background)] p-3 text-sm">{{ item.preview_text }}</pre>
          <p class="whitespace-pre-wrap text-sm">{{ item.reason }}</p>
        </article>
        <h2 v-if="hiddenContent.length" class="pt-4 text-base font-semibold">{{ t('admin.hiddenContent') }}</h2>
        <article v-for="item in hiddenContent" :key="`${item.content_type}:${item.content_id}`" class="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-[var(--color-border)] p-4">
          <span>{{ t(`admin.reportType_${item.content_type}`) }} · {{ item.content_id }} · {{ new Date(item.hidden_at).toLocaleString() }}</span>
          <button data-test="admin-content-restore" class="rounded border px-3 py-1.5" @click="restoreContent(item)">{{ t('admin.restoreContent') }}</button>
        </article>
        <h2 v-if="pendingDocuments.length" class="text-base font-semibold">{{ t('admin.documentSubmissions') }}</h2>
        <article v-for="item in pendingDocuments" :key="item.id" class="rounded-lg border border-[var(--color-border)] p-4">
          <div class="mb-3 flex flex-wrap items-center justify-between gap-2"><div><strong>{{ item.title || item.document_id }}</strong><span class="ml-2 text-sm text-gray-500">{{ item.author_id }} · {{ new Date(item.created_at).toLocaleString() }}</span></div><div class="flex gap-2"><button data-test="admin-review-document-approve" class="rounded bg-blue-600 px-3 py-1.5 text-white" @click="approveDocument(item)">{{ t('admin.reviewApprove') }}</button><button class="rounded border px-3 py-1.5" @click="rejectDocument(item)">{{ t('admin.reviewReject') }}</button></div></div>
          <pre class="max-h-72 overflow-auto whitespace-pre-wrap rounded bg-gray-50 p-3 text-sm dark:bg-gray-900">{{ item.content }}</pre>
        </article>
        <h2 v-if="pendingUserPages.length" class="text-base font-semibold">{{ t('userPage.label') }}</h2>
        <article v-for="item in pendingUserPages" :key="item.id" class="rounded-lg border border-[var(--color-border)] p-4">
          <div class="mb-3 flex flex-wrap items-center justify-between gap-2"><div><strong>{{ t('userPage.label') }}</strong><span class="ml-2 text-sm text-gray-500">{{ item.user_id }} · {{ new Date(item.created_at).toLocaleString() }}</span></div><div class="flex gap-2"><button data-test="admin-review-approve" class="rounded bg-blue-600 px-3 py-1.5 text-white" @click="approveUserPage(item)">{{ t('admin.reviewApprove') }}</button><button class="rounded border px-3 py-1.5" @click="rejectUserPage(item)">{{ t('admin.reviewReject') }}</button></div></div>
          <pre class="max-h-72 overflow-auto whitespace-pre-wrap rounded bg-gray-50 p-3 text-sm dark:bg-gray-900">{{ item.content }}</pre>
        </article>
        <h2 v-if="pendingComments.length" class="pt-4 text-base font-semibold">{{ t('comments.title') }}</h2>
        <article v-for="item in pendingComments" :key="item.id" class="rounded-lg border border-[var(--color-border)] p-4">
          <div class="mb-3 flex flex-wrap items-center justify-between gap-2"><div><strong>{{ t('comments.title') }}</strong><span class="ml-2 text-sm text-gray-500">{{ item.author_id }} · {{ new Date(item.created_at).toLocaleString() }}</span></div><div class="flex gap-2"><button class="rounded bg-blue-600 px-3 py-1.5 text-white" @click="approveComment(item)">{{ t('admin.reviewApprove') }}</button><button class="rounded border px-3 py-1.5" @click="rejectComment(item)">{{ t('admin.reviewReject') }}</button></div></div>
          <pre class="whitespace-pre-wrap rounded bg-gray-50 p-3 text-sm dark:bg-gray-900">{{ item.content }}</pre>
        </article>
      </section>
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
