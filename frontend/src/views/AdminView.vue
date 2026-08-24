<script setup lang="ts">
// 管理视图：按权限码显隐 Tab；各域面板内联实现（T7.8）。
import { onMounted, reactive, ref } from 'vue'
import { adminApi, type DashboardStats } from '@/api'
import { can } from '@/permissions'
import AdminTabs from '@/components/admin/AdminTabs.vue'

const perm = reactive({ has: (code: string) => can(code) })
const t = (k: string) => k

// settings
const settings = ref<Record<string, string>>({})
async function loadSettings() {
  settings.value = await adminApi.settings()
}
async function saveSettings() {
  await adminApi.updateSettings({ ...settings.value })
  ElMessageSuccess('settings saved')
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
async function loadUsers() {
  const r = await adminApi.users()
  users.value = r.items
}
async function changeRole(u: UserRow, role: UserRow['role']) {
  await adminApi.updateUser(u.id, { role })
  await loadUsers()
}
async function toggleStatus(u: UserRow) {
  const next = u.status === 'active' ? 'disabled' : 'active'
  await adminApi.updateUser(u.id, { status: next })
  await loadUsers()
}

// dashboard
const stats = ref<DashboardStats | null>(null)

// backups
const backupFiles = ref<string[]>([])

onMounted(async () => {
  const loads: Promise<void>[] = []
  if (can('settings.manage')) loads.push(loadSettings())
  if (can('user.list')) loads.push(loadUsers())
  if (can('dashboard.read'))
      loads.push(
        adminApi
          .dashboard()
          .then((st) => {
            stats.value = st
          })
          .then(() => undefined),
      )
  if (can('backup.manage')) loads.push(adminApi.backupFiles().then((f) => {
        backupFiles.value = f.items
      }))
  await Promise.allSettled(loads)
})

async function removeBackup(f: string) {
  await adminApi.deleteBackupFile(f)
  backupFiles.value = (await adminApi.backupFiles()).items
}

function ElMessageSuccess(msg: string) {
  // Element Plus 按需引入下的轻量提示（避免全量 message 样式依赖）
  alert(msg)
}
</script>

<template>
  <AdminTabs :perm="perm">
    <template #settings>
      <div class="space-y-2 max-w-lg" data-test="admin-settings">
        <label class="block text-sm">wiki_title
          <input v-model="settings.wiki_title" class="border rounded w-full px-2 py-1" />
        </label>
        <label class="block text-sm">timezone
          <input v-model="settings.timezone" class="border rounded w-full px-2 py-1" />
        </label>
        <button class="px-3 py-1 bg-blue-600 text-white rounded" @click="saveSettings">
          Save
        </button>
      </div>
    </template>

    <template #users>
      <table data-test="admin-users" class="text-sm w-full">
        <thead><tr><th>ID</th><th>Email</th><th>Role</th><th>Status</th></tr></thead>
        <tbody>
          <tr v-for="u in users" :key="u.id">
            <td>{{ u.id }}</td><td>{{ u.email }}</td>
            <td>
              <select :value="u.role" @change="changeRole(u, ($event.target as HTMLSelectElement).value as UserRow['role'])">
                <option value="viewer">viewer</option>
                <option value="editor">editor</option>
                <option value="admin">admin</option>
              </select>
            </td>
            <td>
              <button class="underline" @click="toggleStatus(u)">
                {{ u.status === 'active' ? t('nav.logout') === '' ? '' : 'disable' : 'enable' }}
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </template>

    <template #dashboard>
      <div data-test="admin-dashboard" class="grid grid-cols-3 gap-4 max-w-md text-center">
        <div class="border rounded p-3"><div class="text-2xl">{{ stats?.documents_total ?? '-' }}</div>docs</div>
        <div class="border rounded p-3"><div class="text-2xl">{{ stats?.comments_total ?? '-' }}</div>comments</div>
        <div class="border rounded p-3"><div class="text-2xl">{{ stats?.attachments_total ?? '-' }}</div>files</div>
      </div>
    </template>

    <template #backups>
      <div data-test="admin-backups" class="space-y-3">
        <ul class="text-sm space-y-1">
          <li v-for="f in backupFiles" :key="f" class="flex gap-2 items-center">
            {{ f }}
            <a :href="adminApi.backupDownloadURL(f)" class="text-blue-600">download</a>
            <button class="text-red-600" @click="removeBackup(f)">×</button>
          </li>
        </ul>
      </div>
    </template>
  </AdminTabs>
</template>
