<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import AdminPanelHeading from './AdminPanelHeading.vue'
import type { AdminUserRow } from './types'

defineProps<{ users: AdminUserRow[]; query: string; loading: boolean }>()
const emit = defineEmits<{
  'update:query': [value: string]
  search: []
  'change-role': [user: AdminUserRow, role: AdminUserRow['role']]
  'toggle-status': [user: AdminUserRow]
}>()
const { t } = useI18n()
</script>

<template>
  <div class="space-y-5" data-test="admin-users-panel">
    <AdminPanelHeading :title="t('admin.users')" :description="t('admin.usersIntro')">
      <span class="admin-users-count">{{ t('admin.usersCount', { count: users.length }) }}</span>
    </AdminPanelHeading>
    <div class="admin-user-toolbar">
      <label class="admin-user-search">
        <span class="sr-only">{{ t('admin.userSearchPlaceholder') }}</span>
        <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="10.8" cy="10.8" r="6.3" /><path d="m16 16 4.2 4.2" /></svg>
        <input :value="query" type="search" :placeholder="t('admin.userSearchPlaceholder')" data-test="user-search" @input="emit('update:query', ($event.target as HTMLInputElement).value); emit('search')" @keydown.enter.prevent="emit('search')" />
      </label>
      <span>{{ t('admin.userSearchHint') }}</span>
    </div>
    <div class="admin-table-scroll" :aria-busy="loading" data-test="users-table-scroll">
      <table data-test="admin-users" class="admin-user-table">
        <thead><tr><th>{{ t('admin.colEmail') }}</th><th>{{ t('admin.colName') }}</th><th>{{ t('admin.colRole') }}</th><th>{{ t('admin.colStatus') }}</th><th><span class="sr-only">{{ t('admin.userActions') }}</span></th></tr></thead>
        <tbody>
          <tr v-for="user in users" :key="user.id">
            <td><span class="admin-user-email">{{ user.email }}</span><span class="admin-user-id" :title="user.id">{{ user.id }}</span></td>
            <td>{{ user.display_name || '—' }}</td>
            <td>
              <select :value="user.role" class="admin-role-select" :aria-label="t('admin.userRoleFor', { name: user.display_name || user.email })" @change="emit('change-role', user, ($event.target as HTMLSelectElement).value as AdminUserRow['role'])">
                <option value="viewer">{{ t('admin.roleViewer') }}</option><option value="editor">{{ t('admin.roleEditor') }}</option><option value="admin">{{ t('admin.roleAdmin') }}</option>
              </select>
            </td>
            <td><span class="admin-user-status" :class="user.status === 'active' ? 'is-active' : 'is-disabled'"><span aria-hidden="true" />{{ user.status === 'active' ? t('admin.statusActive') : t('admin.statusDisabled') }}</span></td>
            <td class="admin-user-action-cell"><button class="admin-user-action" :class="user.status === 'active' ? 'is-disable-action' : 'is-enable-action'" data-test="user-toggle" @click="emit('toggle-status', user)">{{ user.status === 'active' ? t('admin.disable') : t('admin.enable') }}</button></td>
          </tr>
          <tr v-if="loading && !users.length"><td colspan="5" class="admin-table-message">{{ t('common.loading') }}</td></tr>
          <tr v-else-if="!users.length"><td colspan="5" class="admin-table-message">{{ query ? t('admin.noUsersMatch') : t('admin.noUsers') }}</td></tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.admin-users-count { flex: none; padding: .3rem .65rem; border-radius: 999px; background: var(--color-background-soft); color: var(--color-text-light); font-size: .75rem; font-weight: 600; }
.admin-user-toolbar { display: flex; align-items: center; justify-content: space-between; flex-wrap: wrap; gap: .7rem 1rem; }
.admin-user-toolbar > span { color: var(--color-text-light); font-size: .8rem; }
.admin-user-search { display: flex; align-items: center; gap: .6rem; width: min(100%, 27rem); min-height: 2.65rem; padding: 0 .8rem; border: 1px solid var(--color-border); border-radius: .7rem; background: var(--color-card-background); color: var(--color-text-light); }
.admin-user-search:focus-within { border-color: var(--color-primary); box-shadow: 0 0 0 3px color-mix(in srgb, var(--color-primary) 15%, transparent); }
.admin-user-search svg { width: 1.1rem; height: 1.1rem; flex: none; fill: none; stroke: currentColor; stroke-width: 1.7; }
.admin-user-search input { width: 100%; min-width: 0; border: 0; outline: 0; background: transparent; color: var(--color-text); font-size: .875rem; }
.admin-user-search input::placeholder { color: var(--color-text-light); }
.admin-table-scroll { overflow-x: auto; border: 1px solid var(--color-border); border-radius: .85rem; background: var(--color-card-background); }
.admin-user-table { width: 100%; min-width: 55rem; border-collapse: collapse; color: var(--color-text); font-size: .875rem; text-align: left; }
.admin-user-table th { padding: .8rem 1rem; background: var(--color-background-soft); color: var(--color-text-light); font-size: .75rem; font-weight: 650; letter-spacing: .025em; white-space: nowrap; }
.admin-user-table td { padding: .85rem 1rem; border-top: 1px solid var(--color-border); vertical-align: middle; }
.admin-user-table tbody tr:hover { background: color-mix(in srgb, var(--color-background-soft) 55%, transparent); }
.admin-user-email { display: block; font-weight: 550; }
.admin-user-id { display: block; max-width: 14rem; overflow: hidden; color: var(--color-text-light); font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: .7rem; text-overflow: ellipsis; white-space: nowrap; }
.admin-role-select { min-width: 8rem; padding: .45rem 1.8rem .45rem .65rem; border: 1px solid var(--color-border); border-radius: .55rem; background: var(--color-card-background); color: var(--color-text); font-size: .8rem; }
.admin-user-status { display: inline-flex; align-items: center; gap: .45rem; padding: .3rem .6rem; border-radius: 999px; font-size: .75rem; font-weight: 600; white-space: nowrap; }
.admin-user-status > span { width: .45rem; height: .45rem; border-radius: 999px; background: currentColor; }
.admin-user-status.is-active { background: color-mix(in srgb, var(--color-success) 12%, var(--color-card-background)); color: var(--color-success); }
.admin-user-status.is-disabled { background: color-mix(in srgb, var(--color-danger) 10%, var(--color-card-background)); color: var(--color-danger); }
.admin-user-action-cell { text-align: right; }
.admin-user-action { padding: .4rem .7rem; border: 1px solid var(--color-border); border-radius: .55rem; background: var(--color-card-background); font-size: .8rem; font-weight: 550; }
.admin-user-action:hover { background: var(--color-background-soft); }
.admin-user-action.is-disable-action { color: var(--color-danger); }
.admin-user-action.is-enable-action { color: var(--color-success); }
.admin-user-table td.admin-table-message { padding: 2.5rem 1rem; color: var(--color-text-light); text-align: center; }
</style>
