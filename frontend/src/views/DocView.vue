<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { docApi, authApi, type DocumentMeta, type CommitView } from '@/api'
import { toApiError } from '@/api/client'
import treeStore from '@/stores/tree'
import { crumbsFor } from '@/utils/breadcrumbs'
import { findNodeByPath } from '@/composables/treeDnd'
import { enhanceMarkdownExtras } from '@/utils/enhance'
import { buildTocTree } from '@/utils/toc'
import { useMediaQuery } from '@/composables/useMediaQuery'
import TocTree from '@/components/doc/TocTree.vue'
import { ElDrawer } from 'element-plus'
import CommentsPanel from '@/components/doc/CommentsPanel.vue'
import AttachmentsPanel from '@/components/doc/AttachmentsPanel.vue'
import siteStore from '@/stores/site'
import { formatSiteDate } from '@/utils/siteDate'

// 05 计划提交 4：路由参数为 slug 路径（/docs/<祖先slug>/…/<slug>），经 resolve 加载。
const props = defineProps<{ path: string }>()
const { t, locale } = useI18n()
const router = useRouter()
const meta = ref<DocumentMeta | null>(null)
const html = ref('')
const status = ref<'loading' | 'ready' | 'notFound' | 'forbidden' | 'error'>('loading')
const meID = ref<string | null>(null)
const canEdit = ref(false)
const canHistory = ref(false)

function printDocument() {
  const href = router.resolve({
    name: 'doc-print',
    params: { pathMatch: props.path.split('/') },
  }).href
  const popup = window.open(href, '_blank')
  if (popup) popup.opener = null
  else void router.push(href)
}
const historyOpen = ref(false)
const commits = ref<CommitView[]>([])
const toc = ref<{ level: number; text: string; id: string }[]>([])

// M15 响应式：目录侧栏仅 ≥lg 展示；窄屏走「目录」抽屉（点击跳转后收起）。
const isWide = useMediaQuery('(min-width: 1024px)')
const tocDrawerOpen = ref(false)

let loadSeq = 0
async function loadDoc(path: string) {
  const seq = ++loadSeq
  status.value = 'loading'
  meta.value = null
  html.value = ''
  toc.value = []
  commits.value = []
  historyOpen.value = false
  meID.value = null
  canEdit.value = false
  canHistory.value = false
  void treeStore.load().catch(() => {})
  try {
    const m = await authApi.me()
    if (seq !== loadSeq) return
    meID.value = m.user.id
    canEdit.value = m.permissions.includes('document.update')
    canHistory.value = m.permissions.includes('version.read')
  } catch {
    /* 匿名 */
  }
  try {
    const r = await docApi.resolve(path)
    if (seq !== loadSeq) return
    html.value = r.render.html
    toc.value = r.render.toc ?? []
    meta.value = r.document
    status.value = 'ready'
  } catch (e) {
    if (seq !== loadSeq) return
    const err = toApiError(e)
    if (err.status === 401) {
      await router.replace({ name: 'login', query: { redirect: `/docs/${path}` } })
      return
    }
    status.value = err.status === 403 ? 'forbidden' : err.status === 404 ? 'notFound' : 'error'
  }
}

watch(() => props.path, (p) => void loadDoc(p), { immediate: true })

// T9.6：TOC 侧栏 + wikilink 点击导航（slug 路径→树内解析；不可见目标一律「不存在」）
function jumpTo(anchor: string) {
  document.getElementById(anchor)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}

function jumpFromToc(anchor: string) {
  tocDrawerOpen.value = false
  jumpTo(anchor)
}
// 扁平 toc → 嵌套目录树（h1~h6 层级与跳级由 buildTocTree 处理）
const tocTree = computed(() => buildTocTree(toc.value))
async function onBodyClick(e: MouseEvent) {
  const a = (e.target as HTMLElement).closest('a.wikilink')
  if (!a) return
  e.preventDefault()
  const target = a.getAttribute('data-target') ?? ''
  try {
    await treeStore.load()
  } catch {
    ElMessage.error(t('common.loadFailed'))
    return
  }
  if (!findNodeByPath(treeStore.state.nodes, target)) {
    ElMessage.warning(t('doc.deadLink', { target }))
    return
  }
  await router.push(`/docs/${target}`)
}

// The current page title belongs in the document heading, not in its own breadcrumb.
const crumbs = computed(() => (meta.value ? crumbsFor(treeStore.state.nodes, meta.value.id).slice(0, -1) : []))
const bodyStartsWithTitle = computed(() => {
  if (!meta.value || !html.value) return false
  const first = new DOMParser().parseFromString(html.value, 'text/html').body.firstElementChild
  return first?.tagName === 'H1' && first.textContent?.trim() === meta.value.title.trim()
})

// T9.3：内容命中公式/mermaid 时才动态加载依赖并增强渲染
const bodyEl = ref<HTMLElement | null>(null)
watch(html, () =>
  nextTick(() => {
    if (bodyEl.value) void enhanceMarkdownExtras(bodyEl.value)
  }),
)

function canUpdate() {
  return canEdit.value
}
async function openHistory() {
  const id = meta.value?.id ?? ''
  historyOpen.value = true
  const r = await docApi.listCommits(id, 100)
  commits.value = r.items
}
async function doRevert(commitID: string) {
  const id = meta.value?.id ?? ''
  await docApi.revert(id, commitID)
  const r = await docApi.render(id)
  html.value = r.html
  historyOpen.value = false
}

// 历史差异：选中版本与其父版本（commit_no-1）的行级对比；首版无父 → 全为新增。
const diffOpenID = ref('')
const diffLines = ref<{ type: 'same' | 'add' | 'del'; text: string }[]>([])
const diffLoading = ref(false)

/** LCS 行级 diff（无依赖，历史抽屉规模足够）。 */
function computeDiff(oldText: string, newText: string) {
  const a = oldText.split('\n')
  const b = newText.split('\n')
  const n = a.length
  const m = b.length
  const dp: number[][] = Array.from({ length: n + 1 }, () => new Array(m + 1).fill(0))
  for (let i = n - 1; i >= 0; i--)
    for (let j = m - 1; j >= 0; j--)
      dp[i][j] = a[i] === b[j] ? dp[i + 1][j + 1] + 1 : Math.max(dp[i + 1][j], dp[i][j + 1])
  const out: { type: 'same' | 'add' | 'del'; text: string }[] = []
  let i = 0
  let j = 0
  while (i < n && j < m) {
    if (a[i] === b[j]) {
      out.push({ type: 'same', text: a[i] })
      i++
      j++
    } else if (dp[i + 1][j] >= dp[i][j + 1]) {
      out.push({ type: 'del', text: a[i] })
      i++
    } else {
      out.push({ type: 'add', text: b[j] })
      j++
    }
  }
  while (i < n) out.push({ type: 'del', text: a[i++] })
  while (j < m) out.push({ type: 'add', text: b[j++] })
  return out
}

async function openDiff(commitID: string) {
  const id = meta.value?.id ?? ''
  const c = commits.value.find((x) => x.id === commitID)
  if (!c) return
  diffOpenID.value = commitID
  diffLoading.value = true
  try {
    const cur = (await docApi.getCommitContent(id, commitID)).content
    const parent = commits.value.find((x) => x.commit_no === c.commit_no - 1)
    const old = parent ? (await docApi.getCommitContent(id, parent.id)).content : ''
    diffLines.value = computeDiff(old, cur)
  } finally {
    diffLoading.value = false
  }
}
</script>

<template>
  <article data-test="doc-page">
    <nav v-if="crumbs.length" class="text-sm text-[var(--color-text)] mb-2" data-test="breadcrumb">
      <template v-for="(c, i) in crumbs" :key="c.id">
        <RouterLink :to="`/docs/${c.path}`" class="hover:underline">{{ c.title }}</RouterLink>
        <span v-if="i < crumbs.length - 1"> / </span>
      </template>
    </nav>
    <div v-if="meta" class="flex flex-wrap items-center gap-2 mb-2" :class="{ 'justify-end': bodyStartsWithTitle }">
      <h1 v-if="!bodyStartsWithTitle" class="text-xl font-semibold flex-1">{{ meta.title }}</h1>
      <a
        v-if="meta"
        :href="docApi.exportMdURL(meta.id)"
        data-test="btn-export"
        class="text-sm px-2 py-1 border rounded"
      >{{ t('doc.export') }}</a>
      <button
        type="button"
        data-test="btn-print"
        class="text-sm px-2 py-1 border rounded"
        @click="printDocument"
      >{{ t('doc.print') }}</button>
      <button
        v-if="canHistory"
        data-test="btn-history"
        class="text-sm px-2 py-1 border rounded"
        @click="openHistory"
      >{{ t('doc.history') }}</button>
      <button
        v-if="!isWide && toc.length"
        data-test="toc-toggle"
        class="text-sm px-2 py-1 border rounded"
        @click="tocDrawerOpen = true"
      >{{ t('doc.toc') }}</button>
      <RouterLink
        v-if="canUpdate()"
        :to="`/docs/${props.path}/edit`"
        data-test="btn-edit"
        class="text-sm px-2 py-1 bg-blue-600 text-white rounded"
      >{{ t('doc.edit') }}</RouterLink>
    </div>
    <div v-if="status === 'loading'" class="text-[var(--color-text)]" data-test="doc-loading">
      {{ t('common.loading') }}
    </div>
    <div v-else-if="status === 'notFound'" class="text-[var(--color-text)]" data-test="doc-not-found">
      {{ t('common.notFound') }}
    </div>
    <div v-else-if="status === 'forbidden'" class="text-[var(--color-text)]" data-test="doc-forbidden">
      {{ t('common.forbidden') }}
    </div>
    <div v-else-if="status === 'error'" class="text-red-600 space-y-2" data-test="doc-error">
      <p>{{ t('common.loadFailed') }}</p>
      <button class="underline" data-test="doc-retry" @click="loadDoc(props.path)">
        {{ t('common.retry') }}
      </button>
    </div>
    <div v-if="status === 'ready'" class="flex gap-4">
      <div class="flex-1 min-w-0 rounded-2xl border border-[var(--color-border)] bg-[var(--color-card-background)] px-6 py-5 shadow-[0_4px_12px_rgba(0,0,0,0.05)] transition-colors" data-test="doc-content-card">
        <!-- eslint-disable-next-line vue/no-v-html：服务端已消毒（RD-07） -->
        <div ref="bodyEl" data-test="doc-html" class="prose prose-sm max-w-none" v-html="html" @click="onBodyClick" />
      </div>
      <aside
        v-if="isWide && toc.length"
        class="w-56 shrink-0 self-start rounded-2xl border border-[var(--color-border)] bg-[var(--color-card-background)] p-4 text-sm shadow-[0_4px_12px_rgba(0,0,0,0.05)] transition-colors"
        data-test="toc-panel"
      >
        <p class="font-semibold mb-1">{{ t('doc.toc') }}</p>
        <TocTree :nodes="tocTree" @jump="jumpTo" />
      </aside>
    </div>

    <el-drawer
      v-model="tocDrawerOpen"
      direction="rtl"
      size="80%"
      :title="t('doc.toc')"
      data-test="toc-drawer"
    >
      <TocTree :nodes="tocTree" @jump="jumpFromToc" />
    </el-drawer>

    <CommentsPanel v-if="status === 'ready'" :doc-i-d="meta!.id" :me="meID ?? ''" :is-admin="false" />
    <AttachmentsPanel v-if="status === 'ready'" :doc-i-d="meta!.id" :editable="canEdit" />

    <el-drawer v-model="historyOpen" :title="t('doc.history')" size="40%" data-test="history-drawer">
      <ul class="space-y-2 text-sm">
        <li v-for="c in commits" :key="c.id" class="border border-[var(--color-border)] rounded p-2">
          <div class="flex justify-between items-center">
            <span>
              #{{ c.commit_no }} {{ c.message || t('doc.noMessage') }}<br />
              <span class="text-[var(--color-text-light)]">
                {{ c.author_name || c.author_id }} · {{ formatSiteDate(c.created_at, locale, siteStore.state.timezone) }}
              </span>
            </span>
            <span class="flex gap-2">
              <button class="underline" data-test="diff-toggle" @click="diffOpenID === c.id ? (diffOpenID = '') : openDiff(c.id)">
                {{ t('doc.diff') }}
              </button>
              <button class="underline" @click="doRevert(c.id)">{{ t('doc.revertTo') }}</button>
            </span>
          </div>
          <div v-if="diffOpenID === c.id" class="mt-2 border-t border-[var(--color-border)] pt-2" data-test="diff-panel">
            <p v-if="diffLoading" class="text-[var(--color-text-light)]">{{ t('common.loading') }}</p>
            <template v-else>
              <div
                v-for="(l, idx) in diffLines"
                :key="idx"
                class="font-mono text-xs px-2 py-0.5 whitespace-pre-wrap break-all"
                :class="{
                  'bg-green-500/10 text-green-700 dark:text-green-400': l.type === 'add',
                  'bg-red-500/10 text-red-700 dark:text-red-400': l.type === 'del',
                  'text-[var(--color-text-light)]': l.type === 'same',
                }"
                :data-test="`diff-${l.type}`"
              >{{ (l.type === 'add' ? '+ ' : l.type === 'del' ? '- ' : '  ') + l.text }}</div>
            </template>
          </div>
        </li>
      </ul>
    </el-drawer>
  </article>
</template>
