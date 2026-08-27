// T7.8 管理面板：四域 Tab，按权限码显隐（AGENTS §4）。
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

interface Perm {
  has: (code: string) => boolean
}
const props = defineProps<{ perm: Perm }>()

type TabKey = 'settings' | 'users' | 'dashboard' | 'backups'
const active = ref<TabKey>('settings')

const tabs = computed(() => {
  const list: { key: TabKey; label: string; show: boolean }[] = [
    { key: 'settings', label: t('admin.settings'), show: props.perm.has('settings.manage') },
    { key: 'users', label: t('admin.users'), show: props.perm.has('user.list') },
    { key: 'dashboard', label: t('admin.dashboard'), show: props.perm.has('dashboard.read') },
    { key: 'backups', label: t('admin.backups'), show: props.perm.has('backup.manage') },
  ]
  return list.filter((x) => x.show)
})

function tabFromURL(): TabKey | null {
  const value = new URLSearchParams(window.location.search).get('tab')
  return value === 'settings' || value === 'users' || value === 'dashboard' || value === 'backups'
    ? value
    : null
}

let initialized = false
function syncFromURL() {
  const fromURL = tabFromURL()
  const next = fromURL && tabs.value.some((x) => x.key === fromURL)
    ? fromURL
    : tabs.value[0]?.key
  if (next) active.value = next
}

function syncToURL(key: TabKey) {
  const url = new URL(window.location.href)
  if (url.searchParams.get('tab') === key) return
  url.searchParams.set('tab', key)
  window.history.pushState({}, '', url)
}

onMounted(() => {
  syncFromURL()
  initialized = true
  window.addEventListener('popstate', syncFromURL)
})

watch(active, (key) => {
  if (initialized) syncToURL(key)
})
onBeforeUnmount(() => window.removeEventListener('popstate', syncFromURL))
</script>

<template>
  <div data-test="admin-page">
    <el-tabs v-model="active">
      <el-tab-pane v-for="tb in tabs" :key="tb.key" :label="tb.label" :name="tb.key" />
    </el-tabs>

    <section v-if="active === 'settings'" data-test="tab-settings">
      <slot name="settings" />
    </section>
    <section v-else-if="active === 'users'" data-test="tab-users">
      <slot name="users" />
    </section>
    <section v-else-if="active === 'dashboard'" data-test="tab-dashboard">
      <slot name="dashboard" />
    </section>
    <section v-else-if="active === 'backups'" data-test="tab-backups">
      <slot name="backups" />
    </section>
    <p v-if="!tabs.length">{{ t('common.notFound') }}</p>
  </div>
</template>
