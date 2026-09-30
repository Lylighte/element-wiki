<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import UiButton from '@/components/ui/UiButton.vue'

defineProps<{
  files: string[]
  busy: boolean
  jobLine: string
  downloadLink: (name: string) => string
}>()
const emit = defineEmits<{
  start: []
  'import-backup': [file: File]
  'import-markdown': [file: File]
  'remove-file': [name: string]
}>()
const { t } = useI18n()

function chooseFile(onFile: (file: File) => void) {
  const input = document.createElement('input')
  input.type = 'file'
  input.accept = '.zip'
  input.onchange = () => {
    const file = input.files?.[0]
    if (file) onFile(file)
  }
  input.click()
}
</script>

<template>
  <div data-test="admin-backups" class="max-w-xl space-y-4">
    <div class="flex flex-wrap gap-2">
      <UiButton variant="primary" size="sm" data-test="btn-start-backup" :disabled="busy" @click="emit('start')">{{ t('admin.startBackup') }}</UiButton>
      <UiButton size="sm" data-test="btn-import-zip" :disabled="busy" @click="chooseFile((file) => emit('import-backup', file))">{{ t('admin.importBackup') }}</UiButton>
      <UiButton size="sm" data-test="btn-import-md" :disabled="busy" @click="chooseFile((file) => emit('import-markdown', file))">{{ t('admin.importMd') }}</UiButton>
    </div>
    <p v-if="jobLine" class="text-xs text-[var(--color-text)]" data-test="job-line">{{ jobLine }}</p>
    <ul class="space-y-1 text-sm">
      <li v-for="file in files" :key="file" class="flex items-center gap-2">
        {{ file }}
        <a :href="downloadLink(file)" class="text-[var(--color-primary)]" data-test="backup-download">{{ t('attachments.download') }}</a>
        <UiButton variant="danger" size="sm" data-test="backup-delete" @click="emit('remove-file', file)">×</UiButton>
      </li>
    </ul>
  </div>
</template>
