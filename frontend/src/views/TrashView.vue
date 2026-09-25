<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { trashApi, type TrashItem } from '@/api'

const { t } = useI18n()
const items = ref<TrashItem[]>([])
const error = ref(false)
const loading = ref(false)
const busyID = ref('')
const itemIDs = computed(() => new Set(items.value.map((item) => item.id)))
const childCounts = computed(() => {
  const counts = new Map<string, number>()
  const byID = new Map(items.value.map((item) => [item.id, item]))
  for (const item of items.value) {
    const seen = new Set([item.id])
    let parentID = item.parent_id
    while (parentID && byID.has(parentID) && !seen.has(parentID)) {
      counts.set(parentID, (counts.get(parentID) ?? 0) + 1)
      seen.add(parentID)
      parentID = byID.get(parentID)?.parent_id ?? null
    }
  }
  return counts
})

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
  if (busyID.value) return
  busyID.value = id
  try {
    await trashApi.restore(id)
    // M18：恢复统一落「已恢复」容器，提示落位便于查找
    ElMessage.success(t('trash.restored'))
  } catch {
    ElMessage.error(t('trash.restoreFailed'))
  } finally {
    await refresh()
    busyID.value = ''
  }
}
async function purge(item: TrashItem) {
  if (busyID.value) return
  const n = childCounts.value.get(item.id) ?? 0
  try {
    await ElMessageBox.confirm(t('trash.purgeConfirm', {
      title: item.title,
      detail: n ? t('trash.purgeConfirmDetail', { n }) : '',
    }), { type: 'warning' })
  } catch {
    return
  }
  busyID.value = item.id
  try {
    await trashApi.purge(item.id)
    await refresh()
  } catch {
    ElMessage.error(t('trash.purgeFailed'))
  } finally {
    busyID.value = ''
  }
}
</script>

<template>
  <div data-test="trash-page">
    <h1 class="text-xl font-semibold mb-3">{{ t('trash.title') }}</h1>
    <p v-if="loading" class="text-[var(--color-text)]">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="text-red-600" data-test="trash-error">{{ t('common.loadFailed') }}</p>
    <button v-if="error" class="underline" @click="refresh">{{ t('common.retry') }}</button>
    <p v-if="!loading && !error && !items.length" class="text-sm text-[var(--color-text-light)]" data-test="trash-empty">{{ t('trash.empty') }}</p>
    <ul v-if="!error" class="text-sm space-y-2">
      <li v-for="it in items" :key="it.id" class="flex flex-wrap gap-x-3 gap-y-1 items-center rounded-lg border border-[var(--color-border)] bg-[var(--color-card-background)] px-3 py-2" data-test="trash-item">
        <span class="min-w-0 flex-1 font-medium break-words">{{ it.title }}</span>
        <span v-if="itemIDs.has(it.parent_id ?? '')" class="text-xs text-[var(--color-text-light)]" data-test="trash-subtree-child">{{ t('trash.subtreeChild') }}</span>
        <span v-if="childCounts.get(it.id)" class="text-xs text-[var(--color-text-light)]" data-test="trash-subtree-count">{{ t('trash.subtreeCount', { n: childCounts.get(it.id) }) }}</span>
        <button class="underline disabled:opacity-40" :disabled="!!busyID" @click="restore(it.id)">{{ t('trash.restore') }}</button>
        <button class="text-red-600 underline disabled:opacity-40" :disabled="!!busyID" @click="purge(it)">{{ t('trash.purge') }}</button>
      </li>
    </ul>
  </div>
</template>
