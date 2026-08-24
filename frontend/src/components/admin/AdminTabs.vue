// T7.8 管理面板：四域 Tab，按权限码显隐（AGENTS §4）。
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
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

onMounted(() => {
  if (tabs.value.length && !tabs.value.some((x) => x.key === active.value)) {
    active.value = tabs.value[0].key
  }
})
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
    <p v-if="!tabs.length">403</p>
  </div>
</template>
