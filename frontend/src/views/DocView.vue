<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { docApi, authApi, type DocumentMeta } from '@/api'
import { toApiError } from '@/api/client'
import treeStore from '@/stores/tree'
import { crumbsFor } from '@/utils/breadcrumbs'
import { findNodeBySlug } from '@/composables/treeDnd'
import { enhanceMarkdownExtras } from '@/utils/enhance'
import { buildTocTree } from '@/utils/toc'
import TocTree from '@/components/doc/TocTree.vue'
import { ElDrawer } from 'element-plus'
import CommentsPanel from '@/components/doc/CommentsPanel.vue'
import AttachmentsPanel from '@/components/doc/AttachmentsPanel.vue'

const props = defineProps<{ id: string }>()
const { t } = useI18n()
const router = useRouter()
const meta = ref<DocumentMeta | null>(null)
const html = ref('')
const status = ref<'loading' | 'ready' | 'notFound' | 'forbidden' | 'error'>('loading')
const meID = ref<string | null>(null)
const canEdit = ref(false)
const canHistory = ref(false)
const historyOpen = ref(false)
const commits = ref<{ id: string; commit_no: number; message: string; created_at: number }[]>([])
const toc = ref<{ level: number; text: string; id: string }[]>([])

let loadSeq = 0
async function loadDoc(id: string) {
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
    const r = await docApi.render(id)
    if (seq !== loadSeq) return
    const mm = await docApi.get(id)
    if (seq !== loadSeq) return
    html.value = r.html
    toc.value = r.toc ?? []
    meta.value = mm.document
    status.value = 'ready'
  } catch (e) {
    if (seq !== loadSeq) return
    const err = toApiError(e)
    if (err.status === 401) {
      await router.replace({ name: 'login', query: { redirect: `/docs/${id}` } })
      return
    }
    status.value = err.status === 403 ? 'forbidden' : err.status === 404 ? 'notFound' : 'error'
  }
}

watch(() => props.id, (id) => void loadDoc(id), { immediate: true })

// T9.6：TOC 侧栏 + wikilink 点击导航（slug→树内解析；不可见目标一律「不存在」）
function jumpTo(anchor: string) {
  document.getElementById(anchor)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
}
// 扁平 toc → 嵌套目录树（h1~h6 层级与跳级由 buildTocTree 处理）
const tocTree = computed(() => buildTocTree(toc.value))
async function onBodyClick(e: MouseEvent) {
  const a = (e.target as HTMLElement).closest('a.wikilink')
  if (!a) return
  e.preventDefault()
  const target = a.getAttribute('data-target') ?? ''
  const node = findNodeBySlug(treeStore.state.nodes, target)
  if (!node) {
    ElMessage.warning(t('doc.deadLink', { target }))
    return
  }
  await router.push(`/docs/${node.id}`)
}

const crumbs = computed(() => crumbsFor(treeStore.state.nodes, props.id))

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
  historyOpen.value = true
  const r = await docApi.listCommits(props.id, 100)
  commits.value = r.items
}
async function doRevert(commitID: string) {
  await docApi.revert(props.id, commitID)
  const r = await docApi.render(props.id)
  html.value = r.html
  historyOpen.value = false
}
</script>

<template>
  <article data-test="doc-page">
    <nav v-if="crumbs.length" class="text-sm text-gray-500 mb-2" data-test="breadcrumb">
      <template v-for="(c, i) in crumbs" :key="c.id">
        <RouterLink :to="`/docs/${c.id}`" class="hover:underline">{{ c.title }}</RouterLink>
        <span v-if="i < crumbs.length - 1"> / </span>
      </template>
    </nav>
    <div v-if="meta" class="flex items-center gap-2 mb-2">
      <h1 class="text-xl font-semibold flex-1">{{ meta.title }}</h1>
      <a
        v-if="meta"
        :href="docApi.exportMdURL(props.id)"
        data-test="btn-export"
        class="text-sm px-2 py-1 border rounded"
      >{{ t('doc.export') }}</a>
      <button
        v-if="canHistory"
        data-test="btn-history"
        class="text-sm px-2 py-1 border rounded"
        @click="openHistory"
      >{{ t('doc.history') }}</button>
      <RouterLink
        v-if="canUpdate()"
        :to="`/docs/${props.id}/edit`"
        data-test="btn-edit"
        class="text-sm px-2 py-1 bg-blue-600 text-white rounded"
      >{{ t('doc.edit') }}</RouterLink>
    </div>
    <div v-if="status === 'loading'" class="text-gray-500" data-test="doc-loading">
      {{ t('common.loading') }}
    </div>
    <div v-else-if="status === 'notFound'" class="text-gray-600" data-test="doc-not-found">
      {{ t('common.notFound') }}
    </div>
    <div v-else-if="status === 'forbidden'" class="text-gray-600" data-test="doc-forbidden">
      {{ t('common.forbidden') }}
    </div>
    <div v-else-if="status === 'error'" class="text-red-600 space-y-2" data-test="doc-error">
      <p>{{ t('common.loadFailed') }}</p>
      <button class="underline" data-test="doc-retry" @click="loadDoc(props.id)">
        {{ t('common.retry') }}
      </button>
    </div>
    <div v-if="status === 'ready'" class="flex gap-4">
      <div class="flex-1 min-w-0">
        <!-- eslint-disable-next-line vue/no-v-html：服务端已消毒（RD-07） -->
        <div ref="bodyEl" data-test="doc-html" class="prose prose-sm max-w-none" v-html="html" @click="onBodyClick" />
      </div>
      <aside
        v-if="toc.length"
        class="hidden lg:block w-56 shrink-0 border-l pl-3 text-sm"
        data-test="toc-panel"
      >
        <p class="font-semibold mb-1">{{ t('doc.toc') }}</p>
        <TocTree :nodes="tocTree" @jump="jumpTo" />
      </aside>
    </div>

    <CommentsPanel v-if="status === 'ready'" :doc-i-d="props.id" :me="meID ?? ''" :is-admin="false" />
    <AttachmentsPanel v-if="status === 'ready'" :doc-i-d="props.id" :editable="canEdit" />

    <el-drawer v-model="historyOpen" :title="t('doc.history')" size="40%" data-test="history-drawer">
      <ul class="space-y-2 text-sm">
        <li v-for="c in commits" :key="c.id" class="border rounded p-2 flex justify-between items-center">
          <span>#{{ c.commit_no }} {{ c.message || t('doc.noMessage') }}<br />
            <span class="text-gray-400">{{ new Date(c.created_at).toLocaleString() }}</span></span>
          <button class="underline" @click="doRevert(c.id)">{{ t('doc.revertTo') }}</button>
        </li>
      </ul>
    </el-drawer>
  </article>
</template>
