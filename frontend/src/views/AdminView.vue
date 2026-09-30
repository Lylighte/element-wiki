<script setup lang="ts">
// 管理视图负责权限、数据加载与操作；业务域面板独立呈现。
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { adminApi, reviewApi, siteApi, type DashboardStats, type HiddenContent, type PendingComment, type PendingContentReport, type PendingDocumentSubmission, type PendingUserPage } from '@/api'
import { can } from '@/permissions'
import AdminTabs from '@/components/admin/AdminTabs.vue'
import TreeAdminPanel from '@/components/admin/TreeAdminPanel.vue'
import AdminSettingsPanel from '@/components/admin/AdminSettingsPanel.vue'
import AdminUsersPanel from '@/components/admin/AdminUsersPanel.vue'
import AdminDashboardPanel from '@/components/admin/AdminDashboardPanel.vue'
import AdminBackupsPanel from '@/components/admin/AdminBackupsPanel.vue'
import AdminReviewsPanel from '@/components/admin/AdminReviewsPanel.vue'
import type { AdminUserRow, SettingsForm } from '@/components/admin/types'
import siteStore from '@/stores/site'
import treeStore from '@/stores/tree'

const perm = reactive({ has: (code: string) => can(code) })
const { t, locale } = useI18n()

// settings（T11.2）：面板表单类型独立维护，父视图负责保存流程与站点状态。
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
const users = ref<AdminUserRow[]>([])
const userQuery = ref('')
const usersLoading = ref(false)
let usersRequest = 0
async function loadUsers() {
  const request = ++usersRequest
  usersLoading.value = true
  try {
    const r = await adminApi.users(userQuery.value)
    if (request === usersRequest) users.value = r.items as AdminUserRow[]
  } catch {
    if (request === usersRequest) operationError.value = true
  } finally {
    if (request === usersRequest) usersLoading.value = false
  }
}
async function changeRole(u: AdminUserRow, role: AdminUserRow['role']) {
  try {
    await adminApi.updateUser(u.id, { role })
    await loadUsers()
  } catch {
    operationError.value = true
  }
}
async function toggleStatus(u: AdminUserRow) {
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
  <div v-if="loadError" class="mb-4 text-[var(--color-danger)] space-x-2" data-test="admin-load-error">
    <span>{{ t('common.loadFailed') }}</span>
    <button class="underline" data-test="admin-load-retry" @click="loadAdminData">{{ t('common.retry') }}</button>
  </div>
  <p v-if="operationError" class="mb-4 text-[var(--color-danger)]" data-test="admin-operation-error">
    {{ t('common.loadFailed') }}
  </p>
  <AdminTabs :perm="perm">
    <template #tree>
      <TreeAdminPanel />
    </template>
    <template #settings>
      <AdminSettingsPanel
        v-model:icon-mode="iconMode"
        :form="form"
        :field-errors="fieldErrors"
        :uploading-icon="uploadingIcon"
        :icon-error="iconError"
        :icon-preview-failed="iconPreviewFailed"
        :icon-preview-url="iconPreviewURL"
        :has-changes="!!changedPatch"
        @apply-preset="applyDeploymentPreset"
        @upload-icon="uploadSiteIcon"
        @icon-preview-error="onIconPreviewError"
        @save="saveSettings"
      />
    </template>

    <template #users>
      <AdminUsersPanel
        v-model:query="userQuery"
        :users="users"
        :loading="usersLoading"
        @search="loadUsers"
        @change-role="changeRole"
        @toggle-status="toggleStatus"
      />
    </template>

    <template #dashboard>
      <AdminDashboardPanel :stats="stats" :document-path="recentDocPath" :format-date="formatDashboardDate" />
    </template>

    <template #backups>
      <AdminBackupsPanel
        :files="backupFiles"
        :busy="backupBusy"
        :job-line="jobLine"
        :download-link="adminApi.backupDownloadURL"
        @start="startBackup"
        @import-backup="importBackupZip"
        @import-markdown="importMarkdownZip"
        @remove-file="removeBackup"
      />
    </template>

    <template #reviews>
      <AdminReviewsPanel
        :loading="reviewsLoading"
        :user-pages="pendingUserPages"
        :comments="pendingComments"
        :documents="pendingDocuments"
        :reports="pendingReports"
        :hidden-content="hiddenContent"
        @reload="loadReviews"
        @decide-report="decideReport"
        @unpublish-report="unpublishReportTarget"
        @restore="restoreContent"
        @approve-document="approveDocument"
        @reject-document="rejectDocument"
        @approve-user-page="approveUserPage"
        @reject-user-page="rejectUserPage"
        @approve-comment="approveComment"
        @reject-comment="rejectComment"
      />
    </template>
  </AdminTabs>
</template>
