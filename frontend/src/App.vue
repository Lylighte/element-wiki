<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import i18n from '@/i18n'
import SideTree from '@/components/tree/SideTree.vue'
import treeStore from '@/stores/tree'
import siteStore from '@/stores/site'
import { docApi, siteApi, type TreeNode } from '@/api'
import { can } from '@/permissions'
import { setLocale, applySiteDefault, type Locale } from '@/i18n'
import authStore from '@/stores/auth'
import { useTheme } from '@/composables/useTheme'
import { ElMessage } from 'element-plus'
import { Search, Plus } from '@element-plus/icons-vue'

const { isDark, initTheme, toggleTheme } = useTheme()
onMounted(initTheme)
import { useMediaQuery } from '@/composables/useMediaQuery'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const me = computed(() => authStore.state.me)

const loaded = ref(false)
onMounted(async () => {
  try {
    const site = await siteApi.info()
    siteStore.setTitle(site.title)
    siteStore.setCommentsEnabled(site.comments_enabled)
    siteStore.setTimezone(site.timezone)
    applySiteDefault(site.default_lang)
  } catch {
    /* 站点信息不可用时保持 i18n 默认 */
  }
  await authStore.initialize()
  loaded.value = true
})

const currentLang = computed(() => (i18n.global.locale.value as Locale))
function switchLang(lang: Locale) {
  setLocale(lang)
}

const isLoggedIn = computed(() => !!me.value)
const showTrash = computed(() => can('document.delete'))
const showAdmin = computed(() =>
  ['settings.manage', 'user.list', 'dashboard.read', 'backup.manage', 'document.update'].some((c) => can(c)),
)
const showCreate = computed(() => can('document.create'))
const isHomeSetup = computed(() => route.path === '/' &&
  !treeStore.state.nodes.some((n) => n.parent_id === null && n.slug === 'home'))
const adminTarget = computed(() => can('document.update') && !can('settings.manage')
  ? '/admin?tab=tree' : '/admin')

function handleMenuCommand(command: string | number | object) {
  switch (command) {
    case 'create': openCreateRoot(); break
    case 'admin': void router.push(adminTarget.value); break
    case 'trash': void router.push('/trash'); break
    case 'tokens': void router.push('/settings/tokens'); break
    case 'theme': toggleTheme(); break
    case 'zh-CN': switchLang('zh-CN'); break
    case 'en': switchLang('en'); break
    case 'logout': void logout(); break
    case 'login': void router.push({ path: '/login', query: { redirect: route.fullPath } }); break
  }
}

async function onGlobalKeydown(e: KeyboardEvent) {
  if (e.key.toLowerCase() !== 'f' || !e.shiftKey || !(e.ctrlKey || e.metaKey) || e.repeat) {
    return
  }
  // An open dialog owns the keyboard; do not route away from an unfinished form.
  const openDialog = Array.from(
    document.querySelectorAll<HTMLElement>('[role="dialog"][aria-modal="true"]'),
  ).some((dialog) => dialog.checkVisibility())
  if (openDialog) return
  e.preventDefault()
  await router.push('/search')
  if (router.currentRoute.value.name !== 'search') return
  await nextTick()
  document.getElementById('global-search-input')?.focus()
}
onMounted(() => window.addEventListener('keydown', onGlobalKeydown))
onBeforeUnmount(() => window.removeEventListener('keydown', onGlobalKeydown))

// 新建文档对话框
const createOpen = ref(false)
const form = reactive({ slug: '', title: '', parent_id: '' })
const creating = ref(false)
const createError = ref('')

// T8.6：父级下拉选项（树扁平化，带路径标签）
interface ParentOpt { id: string; label: string }
function flattenParents(nodes: TreeNode[], prefix = ''): ParentOpt[] {
  return nodes.flatMap((n) => [
    { id: n.id, label: prefix + n.title },
    ...flattenParents(n.children, prefix + n.title + ' / '),
  ])
}
const parentOptions = computed(() => flattenParents(treeStore.state.nodes))

async function submitCreate() {
  if (creating.value) return
  if (!form.title.trim()) {
    createError.value = t('doc.titleRequired')
    return
  }
  createError.value = ''
  creating.value = true
  const parentPath = form.parent_id ? treeStore.pathSlugOf(treeStore.state.nodes, form.parent_id) : ''
  let created: Awaited<ReturnType<typeof docApi.create>>
  try {
    created = await docApi.create({
      slug: form.slug.trim() || undefined,
      title: form.title.trim(),
      parent_id: form.parent_id || null,
    })
  } catch (e) {
    createError.value = (e as { status?: number }).status === 409
      ? t('doc.createConflict')
      : t('doc.createFailed')
    return
  } finally {
    creating.value = false
  }
  createOpen.value = false
  await treeStore.load(true).catch(() => {})
  const path = treeStore.pathSlugOf(treeStore.state.nodes, created.document.id) ||
    [parentPath, created.document.slug].filter(Boolean).join('/')
  await router.push(`/docs/${path}/edit`).catch(() => ElMessage.error(t('common.loadFailed')))
}

async function logout() {
  await authStore.logout()
  location.href = '/'
}

// 头部入口：根级新建（清空父级预置）
function openCreateRoot() {
  void treeStore.load().catch(() => {})
  form.parent_id = ''
  form.slug = ''
  form.title = ''
  createError.value = ''
  createOpen.value = true
}

// 移动端文档树走抽屉，搜索保留在顶栏。
const isDesktop = useMediaQuery('(min-width: 768px)')
const treeDrawerOpen = ref(false)
// 任何路由跳转后收起移动端抽屉（树上「移入回收站」等菜单动作也会导航）。
watch(
  () => route.fullPath,
  () => {
    treeDrawerOpen.value = false
  },
)
</script>

<template>
  <RouterView v-if="route.name === 'doc-print'" />
  <div v-else class="app-shell min-h-screen flex flex-col">
    <header class="h-14 border-b border-[var(--color-border)] bg-[var(--color-header-background)] flex items-center px-3 md:px-4 gap-2 md:gap-4 transition-colors">
      <button
        v-if="!isDesktop"
        class="text-xl leading-none px-1"
        :aria-label="t('nav.tree')"
        data-test="nav-tree-toggle"
        @click="treeDrawerOpen = true"
      >☰</button>
      <RouterLink to="/" class="font-semibold truncate min-w-0" data-test="site-home">
        {{ siteStore.state.title || t('common.appName') }}
      </RouterLink>
      <template v-if="isDesktop">
        <nav class="ml-auto flex items-center gap-3 text-sm shrink-0">
          <RouterLink to="/search" class="flex items-center gap-1.5 rounded border border-[var(--color-border)] px-3 py-1.5 text-[var(--color-text-light)] hover:text-[var(--color-text)]" :title="t('search.shortcut')" data-test="nav-search"><Search class="h-4 w-4" aria-hidden="true" />{{ t('common.search') }}</RouterLink>
          <template v-if="isLoggedIn">
            <button v-if="showCreate" class="rounded px-3 py-1.5" :class="isHomeSetup ? 'border border-[var(--color-border)]' : 'bg-blue-600 text-white'" data-test="nav-create" @click="openCreateRoot">
              {{ t('doc.create') }}
            </button>
            <RouterLink v-if="showAdmin" :to="adminTarget" data-test="nav-admin">{{ can('settings.manage') ? t('nav.admin') : t('admin.tree') }}</RouterLink>
            <el-dropdown trigger="click" @command="handleMenuCommand">
              <button class="max-w-40 truncate rounded px-2 py-1.5 hover:bg-[var(--color-background-mute)]" :aria-label="t('auth.me')" data-test="account-menu-toggle">
                {{ me!.user.display_name || me!.user.email }} <span aria-hidden="true">⌄</span>
              </button>
              <template #dropdown>
                <el-dropdown-menu data-test="account-menu">
                  <el-dropdown-item v-if="showTrash" command="trash" data-test="nav-trash">{{ t('nav.trash') }}</el-dropdown-item>
                  <el-dropdown-item command="tokens" data-test="nav-tokens">{{ t('auth.tokens') }}</el-dropdown-item>
                  <el-dropdown-item command="theme" data-test="theme-toggle">{{ isDark ? t('nav.lightMode') : t('nav.darkMode') }}</el-dropdown-item>
                  <el-dropdown-item :command="currentLang === 'zh-CN' ? 'en' : 'zh-CN'" data-test="lang-toggle">{{ currentLang === 'zh-CN' ? t('admin.langEn') : t('admin.langZh') }}</el-dropdown-item>
                  <el-dropdown-item divided command="logout" data-test="logout-btn">{{ t('nav.logout') }}</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </template>
          <RouterLink
            v-else-if="loaded"
            :to="{ path: '/login', query: { redirect: route.fullPath } }"
            data-test="login-link"
          >
            {{ t('auth.loginWithSSO') }}
          </RouterLink>
          <el-dropdown v-if="!isLoggedIn" trigger="click" @command="handleMenuCommand">
            <button class="rounded px-2 py-1.5 hover:bg-[var(--color-background-mute)]" :aria-label="t('nav.preferences')" data-test="preferences-menu-toggle">⋯</button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="theme" data-test="theme-toggle">{{ isDark ? t('nav.lightMode') : t('nav.darkMode') }}</el-dropdown-item>
                <el-dropdown-item :command="currentLang === 'zh-CN' ? 'en' : 'zh-CN'" data-test="lang-toggle">{{ currentLang === 'zh-CN' ? t('admin.langEn') : t('admin.langZh') }}</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </nav>
      </template>
      <div v-else class="ml-auto flex shrink-0 items-center gap-2">
        <RouterLink to="/search" class="flex h-9 w-9 items-center justify-center rounded" :aria-label="t('common.search')" :title="t('common.search')" data-test="m-search"><Search class="h-5 w-5" aria-hidden="true" /></RouterLink>
        <button v-if="isLoggedIn && showCreate" class="flex h-9 w-9 items-center justify-center rounded" :aria-label="t('doc.create')" data-test="m-create" @click="openCreateRoot"><Plus class="h-5 w-5" aria-hidden="true" /></button>
        <el-dropdown trigger="click" @command="handleMenuCommand">
          <button class="text-xl leading-none px-1" :aria-label="isLoggedIn ? t('auth.me') : t('nav.menu')" data-test="nav-menu">⋯</button>
          <template #dropdown>
            <el-dropdown-menu class="min-w-44" data-test="mobile-menu">
              <template v-if="isLoggedIn">
                <el-dropdown-item v-if="showAdmin" command="admin" data-test="m-admin">{{ can('settings.manage') ? t('nav.admin') : t('admin.tree') }}</el-dropdown-item>
                <el-dropdown-item v-if="showTrash" command="trash" data-test="m-trash">{{ t('nav.trash') }}</el-dropdown-item>
                <el-dropdown-item command="tokens" data-test="m-tokens">{{ t('auth.tokens') }}</el-dropdown-item>
              </template>
              <el-dropdown-item v-else-if="loaded" command="login" data-test="m-login">{{ t('auth.loginWithSSO') }}</el-dropdown-item>
              <el-dropdown-item divided command="theme" data-test="m-theme-toggle">{{ isDark ? t('nav.lightMode') : t('nav.darkMode') }}</el-dropdown-item>
              <el-dropdown-item :command="currentLang === 'zh-CN' ? 'en' : 'zh-CN'" data-test="m-lang-toggle">{{ currentLang === 'zh-CN' ? t('admin.langEn') : t('admin.langZh') }}</el-dropdown-item>
              <el-dropdown-item v-if="isLoggedIn" divided command="logout" data-test="m-logout">{{ t('nav.logout') }}</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </header>

    <div class="app-layout flex flex-1 min-h-0">
      <SideTree v-if="isDesktop" />
      <main class="flex-1 p-3 md:p-6 overflow-auto">
        <RouterView />
      </main>
    </div>

    <el-drawer
      v-if="!isDesktop"
      v-model="treeDrawerOpen"
      direction="ltr"
      size="85%"
      :title="t('nav.tree')"
      data-test="tree-drawer"
    >
      <SideTree @select="treeDrawerOpen = false" />
    </el-drawer>

    <el-dialog v-model="createOpen" :title="t('doc.create')" width="420px">
      <form id="create-doc-form" class="space-y-3" @submit.prevent="submitCreate">
        <label class="block text-sm font-medium">{{ t('doc.titlePlaceholder') }}
          <input v-model="form.title" data-test="create-title" class="mt-1 w-full border rounded px-2 py-1" :aria-invalid="!!createError && !form.title.trim()" @input="createError = ''" />
        </label>
        <label class="block text-sm font-medium">{{ t('doc.parent') }}
          <select v-model="form.parent_id" data-test="create-parent" class="mt-1 w-full border rounded px-2 py-1">
            <option value="">{{ t('tree.root') }}</option>
            <option v-for="o in parentOptions" :key="o.id" :value="o.id">{{ o.label }}</option>
          </select>
        </label>
        <details class="text-sm text-[var(--color-text-light)]">
          <summary class="cursor-pointer">{{ t('doc.advancedPath') }}</summary>
          <label class="block mt-2">{{ t('doc.slug') }}
            <input v-model="form.slug" data-test="create-slug" class="mt-1 w-full border rounded px-2 py-1" />
          </label>
          <p class="mt-1">{{ t('doc.slugHint') }}</p>
        </details>
        <p v-if="createError" class="text-sm text-red-600" role="alert" data-test="create-error">{{ createError }}</p>
      </form>
      <template #footer>
        <button class="px-3 py-1 rounded border" @click="createOpen = false">{{ t('common.cancel') }}</button>
        <button type="submit" form="create-doc-form" class="px-3 py-1 bg-blue-600 text-white rounded ml-2" :disabled="creating" data-test="create-submit">
          {{ t('doc.createAndEdit') }}
        </button>
      </template>
    </el-dialog>
  </div>
</template>
