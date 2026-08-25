<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import i18n from '@/i18n'
import SideTree from '@/components/tree/SideTree.vue'
import treeStore from '@/stores/tree'
import treeMenu from '@/stores/treeMenu'
import { authApi, docApi, siteApi, type MeResponse, type TreeNode } from '@/api'
import { setPermissions, can } from '@/permissions'
import { setLocale, applySiteDefault, type Locale } from '@/i18n'

const { t } = useI18n()
const router = useRouter()
const me = ref<MeResponse | null>(null)

const loaded = ref(false)
const siteTitle = ref('')
onMounted(async () => {
  try {
    const site = await siteApi.info()
    if (site.title) siteTitle.value = site.title
    applySiteDefault(site.default_lang)
  } catch {
    /* 站点信息不可用时保持 i18n 默认 */
  }
  try {
    const m = await authApi.me()
    me.value = m
    setPermissions(m.permissions)
  } catch {
    setPermissions([])
  }
  loaded.value = true
})

const currentLang = computed(() => (i18n.global.locale.value as Locale))
function switchLang(lang: Locale) {
  setLocale(lang)
}

const isLoggedIn = computed(() => !!me.value)
const showTrash = computed(() => can('document.delete'))
const showAdmin = computed(() =>
  ['settings.manage', 'user.list', 'dashboard.read', 'backup.manage'].some((c) => can(c)),
)
const showCreate = computed(() => can('document.create'))

// 新建文档对话框
const createOpen = ref(false)
const form = reactive({ slug: '', title: '', parent_id: '' })
const creating = ref(false)

// T8.6：父级下拉选项（树扁平化，带路径标签）
interface ParentOpt { id: string; label: string }
function flattenParents(nodes: TreeNode[], prefix = ''): ParentOpt[] {
  return nodes.flatMap((n) => [
    { id: n.id, label: prefix + n.title },
    ...flattenParents(n.children, prefix + n.title + ' / '),
  ])
}
const parentOptions = computed(() => flattenParents(treeStore.state.nodes))

watch(
  () => treeMenu.state.requestCreate,
  (v) => {
    if (!v) return
    form.parent_id = treeMenu.state.createParentId || ''
    form.slug = ''
    form.title = ''
    createOpen.value = true
    treeMenu.state.requestCreate = false
  },
)

async function submitCreate() {
  creating.value = true
  try {
    const r = await docApi.create({
      slug: form.slug,
      title: form.title || form.slug,
      parent_id: form.parent_id || null,
    })
    createOpen.value = false
    await treeStore.load(true)
    router.push(`/docs/${r.document.id}/edit`)
  } finally {
    creating.value = false
  }
}

async function logout() {
  await authApi.logout().catch(() => {})
  location.href = '/'
}

// 头部入口：根级新建（清空父级预置）
function openCreateRoot() {
  form.parent_id = ''
  form.slug = ''
  form.title = ''
  createOpen.value = true
}
</script>

<template>
  <div class="min-h-screen flex flex-col">
    <header class="h-14 border-b bg-white flex items-center px-4 gap-4">
      <span class="font-semibold cursor-pointer" @click="router.push('/')">
        {{ siteTitle || t('common.appName') }}
      </span>
      <button
        class="text-xs px-1 rounded"
        :class="currentLang === 'zh-CN' ? 'font-bold text-blue-600' : 'text-gray-400'"
        data-test="lang-zh"
        @click="switchLang('zh-CN')"
      >中</button>
      <button
        class="text-xs px-1 rounded"
        :class="currentLang === 'en' ? 'font-bold text-blue-600' : 'text-gray-400'"
        data-test="lang-en"
        @click="switchLang('en')"
      >EN</button>
      <nav class="ml-auto flex items-center gap-3 text-sm">
        <RouterLink to="/search">{{ t('common.search') }}</RouterLink>

        <template v-if="isLoggedIn">
          <button v-if="showCreate" data-test="nav-create" @click="openCreateRoot">
            {{ t('doc.create') }}
          </button>
          <RouterLink v-if="showTrash" to="/trash" data-test="nav-trash">{{ t('nav.trash') }}</RouterLink>
          <RouterLink v-if="showAdmin" to="/admin" data-test="nav-admin">{{ t('nav.admin') }}</RouterLink>
          <RouterLink to="/settings/tokens" data-test="nav-tokens">{{ t('auth.me') }}</RouterLink>
          <span class="text-gray-500">{{ me!.user.display_name || me!.user.email }}</span>
          <button class="text-red-600" data-test="logout-btn" @click="logout">{{ t('nav.logout') }}</button>
        </template>
        <RouterLink v-else-if="loaded" to="/login" data-test="login-link">
          {{ t('auth.loginWithSSO') }}
        </RouterLink>
      </nav>
    </header>

    <div class="flex flex-1 min-h-0">
      <SideTree class="hidden md:block" />
      <main class="flex-1 p-6 overflow-auto">
        <RouterView />
      </main>
    </div>

    <el-dialog v-model="createOpen" :title="t('doc.create')" width="420px">
      <form class="space-y-3" @submit.prevent="submitCreate">
        <input v-model="form.slug" placeholder="slug (a-z0-9-)" data-test="create-slug" class="w-full border rounded px-2 py-1" />
        <input v-model="form.title" :placeholder="t('doc.titlePlaceholder')" data-test="create-title" class="w-full border rounded px-2 py-1" />
        <select v-model="form.parent_id" data-test="create-parent" class="w-full border rounded px-2 py-1">
          <option value="">/</option>
          <option v-for="o in parentOptions" :key="o.id" :value="o.id">{{ o.label }}</option>
        </select>
      </form>
      <template #footer>
        <button class="px-3 py-1 rounded border" @click="createOpen = false">{{ t('common.cancel') }}</button>
        <button class="px-3 py-1 bg-blue-600 text-white rounded ml-2" :disabled="creating" @click="submitCreate">
          {{ t('doc.createAndEdit') }}
        </button>
      </template>
    </el-dialog>
  </div>
</template>
