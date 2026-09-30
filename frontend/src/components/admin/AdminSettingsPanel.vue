<script setup lang="ts">
import { computed } from 'vue'
import { Notebook } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import AdminPanelHeading from './AdminPanelHeading.vue'
import type { SettingsForm } from './types'

const props = defineProps<{
  form: SettingsForm
  fieldErrors: Record<string, string>
  uploadingIcon: boolean
  iconError: string
  iconPreviewFailed: boolean
  iconPreviewUrl: string
  hasChanges: boolean
  iconMode: 'upload' | 'url'
}>()
const emit = defineEmits<{
  'update:iconMode': [value: 'upload' | 'url']
  'apply-preset': [value: SettingsForm['deployment_preset']]
  'upload-icon': [event: Event]
  'icon-preview-error': []
  save: []
}>()
const { t } = useI18n()

const timezoneGroups = [
  { label: 'admin.timezoneRegionAsia', options: [
    { value: 'Asia/Shanghai', label: 'admin.timezoneShanghai' },
    { value: 'Asia/Tokyo', label: 'admin.timezoneTokyo' },
    { value: 'Asia/Seoul', label: 'admin.timezoneSeoul' },
    { value: 'Asia/Singapore', label: 'admin.timezoneSingapore' },
    { value: 'Asia/Kolkata', label: 'admin.timezoneKolkata' },
    { value: 'Asia/Dubai', label: 'admin.timezoneDubai' },
  ] },
  { label: 'admin.timezoneRegionEurope', options: [
    { value: 'Europe/London', label: 'admin.timezoneLondon' },
    { value: 'Europe/Berlin', label: 'admin.timezoneBerlin' },
    { value: 'Europe/Paris', label: 'admin.timezoneParis' },
  ] },
  { label: 'admin.timezoneRegionAmericas', options: [
    { value: 'America/Los_Angeles', label: 'admin.timezoneLosAngeles' },
    { value: 'America/Chicago', label: 'admin.timezoneChicago' },
    { value: 'America/New_York', label: 'admin.timezoneNewYork' },
    { value: 'America/Sao_Paulo', label: 'admin.timezoneSaoPaulo' },
  ] },
  { label: 'admin.timezoneRegionOceania', options: [
    { value: 'Australia/Sydney', label: 'admin.timezoneSydney' },
    { value: 'Pacific/Auckland', label: 'admin.timezoneAuckland' },
  ] },
]
const timezonePresetValues = timezoneGroups.flatMap((group) => group.options.map((option) => option.value))
const currentTimezoneIsCustom = computed(() => props.form.timezone !== '' && !timezonePresetValues.includes(props.form.timezone))
const booleanFieldErrors = computed(() =>
  (['anonymous_read', 'comments_enabled', 'user_pages_enabled'] as const)
    .filter((key) => props.fieldErrors[key])
    .map((key) => props.fieldErrors[key]),
)
const themeColorGroups = [
  { mode: 'light', label: 'admin.themeLight', items: [
    { key: 'theme_light_primary', label: 'admin.themePrimary' },
    { key: 'theme_light_accent', label: 'admin.themeAccent' },
    { key: 'theme_light_focus', label: 'admin.themeFocus' },
  ] },
  { mode: 'dark', label: 'admin.themeDark', items: [
    { key: 'theme_dark_primary', label: 'admin.themePrimary' },
    { key: 'theme_dark_accent', label: 'admin.themeAccent' },
    { key: 'theme_dark_focus', label: 'admin.themeFocus' },
  ] },
] as const
function formatUtcOffset(timezone: string): string {
  try {
    const zoneName = new Intl.DateTimeFormat('en-US', { timeZone: timezone, timeZoneName: 'shortOffset' })
      .formatToParts(new Date()).find((part) => part.type === 'timeZoneName')?.value ?? 'GMT'
    const match = /^GMT(?:([+-])(\d{1,2})(?::(\d{2}))?)?$/.exec(zoneName)
    if (!match) return 'UTC'
    if (!match[1]) return 'UTC+00:00'
    return `UTC${match[1]}${match[2].padStart(2, '0')}:${match[3] ?? '00'}`
  } catch { return 'UTC?' }
}
</script>

<template>
  <div class="admin-settings" data-test="admin-settings">
    <AdminPanelHeading :title="t('admin.settings')" :description="t('admin.settingsIntro')" />
    <section class="setting-card">
      <h2 class="text-base font-semibold">{{ t('admin.siteAccess') }}</h2>
      <div class="setting-grid mt-4">
        <label class="setting-field">{{ t('admin.deploymentPreset') }}
          <select :value="form.deployment_preset" class="setting-input" data-test="f-deployment-preset" @change="emit('apply-preset', ($event.target as HTMLSelectElement).value as SettingsForm['deployment_preset'])">
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
            <option value="zh-CN">{{ t('admin.langZh') }}</option><option value="en">{{ t('admin.langEn') }}</option>
          </select>
          <span class="setting-help">{{ t('admin.helpDefaultLang') }}</span>
          <span v-if="fieldErrors.default_lang" class="setting-error">{{ fieldErrors.default_lang }}</span>
        </label>
        <label class="setting-field">{{ t('admin.fieldTimezone') }}
          <select v-model="form.timezone" data-test="f-tz" class="setting-input">
            <option disabled value="">{{ t('admin.selectTimezone') }}</option>
            <option v-if="currentTimezoneIsCustom" :value="form.timezone">{{ formatUtcOffset(form.timezone) }} · {{ t('admin.timezoneCurrentCustom', { timezone: form.timezone }) }}</option>
            <option value="UTC">UTC+00:00 · {{ t('admin.timezoneUTC') }} · UTC</option>
            <optgroup v-for="group in timezoneGroups" :key="group.label" :label="t(group.label)">
              <option v-for="option in group.options" :key="option.value" :value="option.value">{{ formatUtcOffset(option.value) }} · {{ t(option.label) }} · {{ option.value }}</option>
            </optgroup>
          </select>
          <span class="setting-help">{{ t('admin.helpTimezone') }}</span>
          <span v-if="fieldErrors.timezone" class="setting-error">{{ fieldErrors.timezone }}</span>
        </label>
      </div>
      <div class="mt-4 divide-y divide-[var(--color-border)] border-t border-[var(--color-border)]">
        <label v-for="item in [
          ['anonymous_read', 'fieldAnonRead', 'helpAnonRead', 'f-anon'],
          ['comments_enabled', 'fieldCommentsEnabled', 'helpCommentsEnabled', 'f-comments'],
          ['user_pages_enabled', 'userPagesEnabled', 'userPagesEnabledHelp', 'f-user-pages'],
          ['user_pages_review_required', 'reviewRequired', 'reviewRequiredHelp', 'f-user-pages-review'],
          ['document_review_required', 'reviewRequired', 'reviewRequiredHelp', 'f-document-review'],
          ['comment_review_required', 'reviewRequired', 'reviewRequiredHelp', 'f-comment-review'],
        ] as const" :key="item[0]" class="setting-toggle">
          <span><strong>{{ item[1] === 'reviewRequired' ? t('admin.reviewRequired', { content: t(item[0] === 'document_review_required' ? 'admin.documents' : item[0] === 'comment_review_required' ? 'admin.comments' : 'admin.userPages') }) : t(`admin.${item[1]}`) }}</strong><small>{{ t(`admin.${item[2]}`) }}</small></span>
          <el-switch v-model="form[item[0]]" :data-test="item[3]" />
        </label>
        <span v-for="message in booleanFieldErrors" :key="message" class="setting-error">{{ message }}</span>
      </div>
    </section>
    <section class="setting-card space-y-5" data-test="site-brand-settings">
      <div><h2 class="text-base font-semibold">{{ t('admin.siteBrand') }}</h2><p class="setting-help mt-1">{{ t('admin.siteBrandHelp') }}</p></div>
      <div class="grid gap-4 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
        <div class="setting-field">
          <span class="font-medium">{{ t('admin.siteIcon') }}</span>
          <el-radio-group :model-value="iconMode" class="mt-2" data-test="site-icon-mode" @update:model-value="emit('update:iconMode', $event as 'upload' | 'url')">
            <el-radio-button value="upload">{{ t('admin.iconUpload') }}</el-radio-button><el-radio-button value="url">{{ t('admin.iconURL') }}</el-radio-button>
          </el-radio-group>
          <label v-if="iconMode === 'url'" class="setting-field mt-3">{{ t('admin.iconURL') }}
            <input v-model="form.site_icon_url" type="url" class="setting-input" placeholder="https://example.com/icon.png" data-test="site-icon-url" />
          </label>
          <label v-else class="setting-field mt-3"><span>{{ t('admin.iconUploadHelp', { size: form.upload_max_mb }) }}</span>
            <input type="file" accept=".png,.ico,.webp,image/png,image/x-icon,image/webp" class="setting-input" :disabled="uploadingIcon" data-test="site-icon-file" @change="emit('upload-icon', $event)" />
          </label>
          <p v-if="iconError" class="setting-error" role="alert">{{ iconError }}</p><span v-if="fieldErrors.site_icon_url" class="setting-error">{{ fieldErrors.site_icon_url }}</span>
        </div>
        <div class="flex items-center gap-3 rounded-lg border border-[var(--color-border)] bg-[var(--color-background)] p-3" data-test="site-icon-preview">
          <img v-if="iconPreviewUrl && !iconPreviewFailed" :src="iconPreviewUrl" class="site-brand-icon" alt="" referrerpolicy="no-referrer" @error="emit('icon-preview-error')" />
          <Notebook v-else class="site-brand-icon-default" aria-hidden="true" /><span class="text-sm">{{ t('admin.iconPreview') }}</span>
        </div>
      </div>
      <div class="setting-field max-w-xs"><span>{{ t('admin.themePreset') }}</span>
        <div class="flex items-center gap-2 rounded-lg border border-[var(--color-border)] px-3 py-2" data-test="theme-preset"><span class="h-4 w-4 rounded-full border border-black/10" style="background:#2563EB" aria-hidden="true" /><strong>{{ t('admin.themeBlue') }}</strong><span class="ml-auto text-xs font-normal text-[var(--color-text-light)]">{{ t('admin.themeCurrent') }}</span></div>
      </div>
      <details class="rounded-lg border border-[var(--color-border)] p-4" data-test="theme-advanced"><summary class="cursor-pointer font-medium">{{ t('admin.themeAdvanced') }}</summary>
        <div class="mt-4 grid gap-5 lg:grid-cols-2"><fieldset v-for="group in themeColorGroups" :key="group.mode" class="grid grid-cols-1 gap-3"><legend class="mb-2 font-semibold">{{ t(group.label) }}</legend>
          <label v-for="item in group.items" :key="item.key" class="flex items-center justify-between gap-3 text-sm"><span>{{ t(item.label) }}</span><span class="flex items-center gap-2"><input v-model="form[item.key]" type="color" class="h-9 w-12 cursor-pointer rounded border border-[var(--color-border)] bg-transparent" :data-test="item.key" /><code>{{ form[item.key] }}</code></span></label>
        </fieldset></div>
      </details>
      <div class="grid gap-4 lg:grid-cols-2">
        <label class="setting-field">{{ t('admin.articleFooter') }}<textarea v-model="form.article_footer_markdown" rows="5" class="setting-input font-mono text-sm" data-test="article-footer-markdown" /><span class="setting-help">{{ t('admin.footerMarkdownHelp') }}</span></label>
        <label class="setting-field">{{ t('admin.icpFooter') }}<textarea v-model="form.sidebar_footer_markdown" rows="5" class="setting-input font-mono text-sm" data-test="sidebar-footer-markdown" /><span class="setting-help">{{ t('admin.footerMarkdownHelp') }}</span></label>
      </div>
    </section>
    <section class="setting-card"><h2 class="text-base font-semibold">{{ t('admin.contentStorage') }}</h2><div class="setting-grid mt-4">
      <label v-for="item in [
        ['max_versions', 'fieldMaxVersions', 'helpMaxVersions', 'f-max-versions'],
        ['upload_max_mb', 'fieldUploadMax', 'helpUploadMax', 'f-upload-max'],
        ['trash_retention_days', 'fieldTrashDays', 'helpTrashDays', 'f-trash-days'],
      ] as const" :key="item[0]" class="setting-field">{{ t(`admin.${item[1]}`) }}<el-input-number v-model="form[item[0]]" :min="1" :data-test="item[3]" /><span class="setting-help">{{ t(`admin.${item[2]}`) }}</span><span v-if="fieldErrors[item[0]]" class="setting-error">{{ fieldErrors[item[0]] }}</span></label>
      <label class="setting-field">{{ t('admin.fieldAllowedExts') }}<input v-model="form.allowed_extensions" data-test="f-exts" class="setting-input" placeholder="png,jpg,pdf" /><span class="setting-help">{{ t('admin.helpAllowedExts') }}</span><span v-if="fieldErrors.allowed_extensions" class="setting-error">{{ fieldErrors.allowed_extensions }}</span></label>
    </div></section>
    <div class="flex items-center gap-3 pb-8"><button class="px-4 py-2 rounded bg-[var(--color-primary)] text-white" data-test="admin-save" @click="emit('save')">{{ t('common.save') }}</button><span class="setting-help" data-test="settings-change-state">{{ hasChanges ? t('admin.unsavedChanges') : t('admin.allSaved') }}</span></div>
  </div>
</template>

<style scoped>
.admin-settings { max-width: 48rem; display: flex; flex-direction: column; gap: 1.25rem; }
.setting-card { padding: 1.25rem; border: 1px solid var(--color-border); border-radius: 1rem; background: var(--color-card-background); }
.setting-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(min(100%, 18rem), 1fr)); gap: 1.25rem; }
.setting-field { display: flex; flex-direction: column; align-items: flex-start; gap: .35rem; font-size: .875rem; font-weight: 600; }
.setting-input { width: 100%; min-height: 2.25rem; padding: .35rem .65rem; border: 1px solid var(--color-border); border-radius: .4rem; background: var(--color-card-background); color: var(--color-text); font-weight: 400; }
.setting-help, .setting-toggle small { color: var(--color-text-light); font-size: .8rem; font-weight: 400; line-height: 1.45; }
.setting-error { color: var(--color-danger); font-size: .8rem; font-weight: 400; }
.setting-toggle { display: flex; justify-content: space-between; align-items: center; gap: 1rem; padding: .85rem 0; font-size: .875rem; }
.setting-toggle span { display: flex; flex-direction: column; gap: .15rem; }
.setting-toggle strong { font-weight: 600; }
.site-brand-icon { width: 2.5rem; height: 2.5rem; object-fit: contain; }
.site-brand-icon-default { width: 2rem; height: 2rem; color: var(--color-primary); }
</style>
