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

// M15 响应式：桌面（≥md）保持既有顶栏/侧栏；移动端顶栏收进下拉、文档树走抽屉。
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
  <div class="app-shell min-h-screen flex flex-col">
    <header class="h-14 border-b border-[var(--color-border)] bg-[var(--color-header-background)] flex items-center px-3 md:px-4 gap-2 md:gap-4 transition-colors">
      <button
        v-if="!isDesktop"
        class="text-xl leading-none px-1"
        :aria-label="t('nav.tree')"
        data-test="nav-tree-toggle"
        @click="treeDrawerOpen = true"
      >☰</button>
      <span class="font-semibold cursor-pointer truncate min-w-0" @click="router.push('/')">
        {{ siteStore.state.title || t('common.appName') }}
      </span>
      <template v-if="isDesktop">
        <button
          class="text-xs px-1 rounded"
          :class="currentLang === 'zh-CN' ? 'font-bold text-blue-600' : 'text-[var(--color-text-light)]'"
          data-test="lang-zh"
          @click="switchLang('zh-CN')"
        >中</button>
        <button
          class="text-xs px-1 rounded"
          :class="currentLang === 'en' ? 'font-bold text-blue-600' : 'text-[var(--color-text-light)]'"
          data-test="lang-en"
          @click="switchLang('en')"
        >EN</button>
        <nav class="ml-auto flex items-center gap-3 text-sm">
          <RouterLink to="/search" :title="t('search.shortcut')">{{ t('common.search') }}</RouterLink>
          <button
            class="px-1"
            :aria-label="isDark ? 'light mode' : 'dark mode'"
            data-test="theme-toggle"
            @click="toggleTheme"
          >{{ isDark ? '☀' : '☾' }}</button>

          <template v-if="isLoggedIn">
            <button v-if="showCreate" data-test="nav-create" @click="openCreateRoot">
              {{ t('doc.create') }}
            </button>
            <RouterLink v-if="showTrash" to="/trash" data-test="nav-trash">{{ t('nav.trash') }}</RouterLink>
            <RouterLink v-if="showAdmin" :to="can('document.update') && !can('settings.manage') ? '/admin?tab=tree' : '/admin'" data-test="nav-admin">{{ can('settings.manage') ? t('nav.admin') : t('admin.tree') }}</RouterLink>
            <RouterLink to="/settings/tokens" data-test="nav-tokens">{{ t('auth.me') }}</RouterLink>
            <span class="text-[var(--color-text)]">{{ me!.user.display_name || me!.user.email }}</span>
            <button class="text-red-600" data-test="logout-btn" @click="logout">{{ t('nav.logout') }}</button>
          </template>
          <RouterLink
            v-else-if="loaded"
            :to="{ path: '/login', query: { redirect: route.fullPath } }"
            data-test="login-link"
          >
            {{ t('auth.loginWithSSO') }}
          </RouterLink>
        </nav>
      </template>
      <div v-else class="ml-auto">
        <el-dropdown trigger="click">
          <button class="text-xl leading-none px-1" :aria-label="t('nav.menu')" data-test="nav-menu">⋯</button>
          <template #dropdown>
            <el-dropdown-menu class="min-w-44" data-test="mobile-menu">
              <el-dropdown-item>
                <RouterLink to="/search" data-test="m-search">{{ t('common.search') }}</RouterLink>
              </el-dropdown-item>
              <template v-if="isLoggedIn">
                <el-dropdown-item v-if="showCreate">
                  <button class="w-full text-left" data-test="m-create" @click="openCreateRoot">
                    {{ t('doc.create') }}
                  </button>
                </el-dropdown-item>
                <el-dropdown-item v-if="showTrash">
                  <RouterLink to="/trash" data-test="m-trash">{{ t('nav.trash') }}</RouterLink>
                </el-dropdown-item>
                <el-dropdown-item v-if="showAdmin">
                  <RouterLink :to="can('document.update') && !can('settings.manage') ? '/admin?tab=tree' : '/admin'" data-test="m-admin">{{ can('settings.manage') ? t('nav.admin') : t('admin.tree') }}</RouterLink>
                </el-dropdown-item>
                <el-dropdown-item>
                  <RouterLink to="/settings/tokens" data-test="m-tokens">{{ t('auth.me') }}</RouterLink>
                </el-dropdown-item>
                <el-dropdown-item disabled>
                  <span class="text-[var(--color-text)] truncate" data-test="m-user">
                    {{ me!.user.display_name || me!.user.email }}
                  </span>
                </el-dropdown-item>
                <el-dropdown-item>
                  <button class="w-full text-left text-red-600" data-test="m-logout" @click="logout">
                    {{ t('nav.logout') }}
                  </button>
                </el-dropdown-item>
              </template>
              <el-dropdown-item v-else-if="loaded">
                <RouterLink
                  :to="{ path: '/login', query: { redirect: route.fullPath } }"
                  data-test="m-login"
                >{{ t('auth.loginWithSSO') }}</RouterLink>
              </el-dropdown-item>
              <el-dropdown-item divided>
                <button class="w-full text-left" data-test="m-theme-toggle" @click="toggleTheme">
                  {{ isDark ? '☀ 浅色模式' : '☾ 深色模式' }}
                </button>
              </el-dropdown-item>
              <el-dropdown-item>
                <span class="flex items-center gap-3">
                  <button
                    class="text-xs px-1 rounded"
                    :class="currentLang === 'zh-CN' ? 'font-bold text-blue-600' : 'text-[var(--color-text-light)]'"
                    data-test="m-lang-zh"
                    @click="switchLang('zh-CN')"
                  >中</button>
                  <button
                    class="text-xs px-1 rounded"
                    :class="currentLang === 'en' ? 'font-bold text-blue-600' : 'text-[var(--color-text-light)]'"
                    data-test="m-lang-en"
                    @click="switchLang('en')"
                  >EN</button>
                </span>
              </el-dropdown-item>
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
