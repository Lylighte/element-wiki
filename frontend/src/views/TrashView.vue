<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage, ElMessageBox } from 'element-plus'
import { trashApi, type TrashItem } from '@/api'
import UiButton from '@/components/ui/UiButton.vue'
import UiCard from '@/components/ui/UiCard.vue'
import UiPageHeader from '@/components/ui/UiPageHeader.vue'

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
  <div data-test="trash-page" class="trash-page">
    <UiPageHeader :title="t('trash.title')" :description="t('trash.description')">
      <template #actions><RouterLink to="/" class="trash-home-link">{{ t('nav.home') }}</RouterLink></template>
    </UiPageHeader>
    <p v-if="loading" class="text-[var(--color-text)]">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="text-[var(--color-danger)]" data-test="trash-error">{{ t('common.loadFailed') }}</p>
    <UiButton v-if="error" size="sm" @click="refresh">{{ t('common.retry') }}</UiButton>
    <p v-if="!loading && !error && !items.length" class="trash-empty" data-test="trash-empty">{{ t('trash.empty') }}</p>
    <ul v-if="!error" class="trash-list">
      <UiCard v-for="it in items" :key="it.id" as="li" padding="sm" class="flex flex-wrap items-center gap-x-3 gap-y-1" data-test="trash-item">
        <span class="min-w-0 flex-1 font-medium break-words">{{ it.title }}</span>
        <span v-if="itemIDs.has(it.parent_id ?? '')" class="text-xs text-[var(--color-text-light)]" data-test="trash-subtree-child">{{ t('trash.subtreeChild') }}</span>
        <span v-if="childCounts.get(it.id)" class="text-xs text-[var(--color-text-light)]" data-test="trash-subtree-count">{{ t('trash.subtreeCount', { n: childCounts.get(it.id) }) }}</span>
        <UiButton size="sm" :disabled="!!busyID" @click="restore(it.id)">{{ t('trash.restore') }}</UiButton>
        <UiButton variant="danger" size="sm" class="underline" :disabled="!!busyID" @click="purge(it)">{{ t('trash.purge') }}</UiButton>
      </UiCard>
    </ul>
  </div>
</template>

<style scoped>
.trash-page { max-width: 56rem; margin: 0 auto; }
.trash-home-link { display: inline-flex; align-items: center; min-height: 2.25rem; padding: .4rem .75rem; border: 1px solid var(--color-border); border-radius: .55rem; color: var(--color-text); font-size: .875rem; text-decoration: none; }
.trash-home-link:hover { background: var(--color-background-soft); border-color: var(--color-border-hover); }
.trash-list { display: grid; gap: .6rem; margin: 1rem 0 0; padding: 0; list-style: none; }
.trash-empty { padding: 2rem 1rem; border: 1px dashed var(--color-border); border-radius: .75rem; color: var(--color-text-light); text-align: center; }
</style>
