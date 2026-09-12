<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { trashApi, type TrashItem } from '@/api'

const { t } = useI18n()
const items = ref<TrashItem[]>([])
const error = ref(false)
const loading = ref(false)

async function refresh() {
  loading.value = true
  error.value = false
  try {
    items.value = (await trashApi.list()).items
  } catch {
    error.value = true
  } finally {
    loading.value = false
  }
}
onMounted(() => void refresh())

async function restore(id: string) {
  try {
    await trashApi.restore(id)
    // M18：恢复统一落「已恢复」容器，提示落位便于查找
    ElMessage.success(t('trash.restored'))
  } catch {
    ElMessage.error(t('trash.restoreFailed'))
  }
  await refresh()
}
async function purge(id: string) {
  await trashApi.purge(id)
  await refresh()
}
</script>

<template>
  <div data-test="trash-page">
    <h1 class="text-xl font-semibold mb-3">{{ t('trash.title') }}</h1>
    <p v-if="loading" class="text-[var(--color-text)]">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="text-red-600" data-test="trash-error">{{ t('common.loadFailed') }}</p>
    <button v-if="error" class="underline" @click="refresh">{{ t('common.retry') }}</button>
    <ul class="text-sm space-y-1">
      <li v-for="it in items" :key="it.id" class="flex gap-3 items-center" data-test="trash-item">
        <span>{{ it.title }}</span>
        <button class="underline" @click="restore(it.id)">{{ t('trash.restore') }}</button>
        <button class="text-red-600 underline" @click="purge(it.id)">{{ t('trash.purge') }}</button>
      </li>
    </ul>
  </div>
</template>
