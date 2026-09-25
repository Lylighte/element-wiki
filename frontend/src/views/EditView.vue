<script setup lang="ts">
// 编辑路由：懒加载 EditorCanvas（只读页零加载，AGENTS §2）。
// 05 计划提交 4：路由参数为 slug 路径，先 resolve 取 id 再走既有草稿/HEAD 流程。
import { computed, onBeforeUnmount, onMounted, nextTick, ref, watch } from 'vue'
import { onBeforeRouteLeave, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { docApi, attachmentApi, type Draft, type TreeNode } from '@/api'
import { toApiError } from '@/api/client'
import treeStore from '@/stores/tree'
import { useAutosave } from '@/composables/useAutosave'
import { useMediaQuery } from '@/composables/useMediaQuery'
import { enhanceMarkdownExtras } from '@/utils/enhance'
import { useI18n } from 'vue-i18n'

const props = defineProps<{ path: string }>()
const { t } = useI18n()
const router = useRouter()

const docID = ref('')
const title = ref('')
const baseCommitID = ref('')
const markdown = ref('')
const links = ref<{ title: string; path: string }[]>([])
const ready = ref(false)
const loadError = ref('')
const committing = ref(false)
// 生效可见性（沿祖先链解析）：编辑页唯一的内容可见性入口（T16.7）
const visibility = ref<'standard' | 'restricted'>('standard')

const autosave = useAutosave({
  delay: 1500,
  save: (content) => docApi.saveDraft(docID.value, baseCommitID.value, content),
  isConflict: (e) => (e as { status?: number }).status === 409,
})

// T9.1：标题防抖 PATCH 持久化（title 不入版本历史，走元数据更新）；
// commit 时再随 body 兜底，保证最终一致。
let titleTimer: ReturnType<typeof setTimeout> | null = null
const savedTitle = ref('')
watch(title, (v) => {
  if (!ready.value || !v.trim() || v.trim() === savedTitle.value) return
  if (titleTimer) clearTimeout(titleTimer)
  titleTimer = setTimeout(() => {
    titleTimer = null
    persistTitleNow().catch(() => {})
  }, 800)
})

async function persistTitleNow() {
  if (titleTimer) {
    clearTimeout(titleTimer)
    titleTimer = null
  }
  const v = title.value.trim()
  if (!v || v === savedTitle.value) return
  await docApi.patch(docID.value, { title: v })
  savedTitle.value = v
}

let loadSeq = 0
async function loadDoc(path: string) {
  const seq = ++loadSeq
  autosave.reset()
  docID.value = ''
  title.value = ''
  savedTitle.value = ''
  baseCommitID.value = ''
  markdown.value = ''
  links.value = []
  ready.value = false
  loadError.value = ''
  previewHtml.value = ''
  previewError.value = false
  previewSeq++
  void treeStore.load().catch(() => {})
  try {
    const resolved = await docApi.resolve(path)
    if (seq !== loadSeq) return
    const id = resolved.document.id
    docID.value = id
    title.value = resolved.document.title
    savedTitle.value = resolved.document.title
    visibility.value = resolved.document.effective_visibility ?? 'standard'
    const head = await docApi.listCommits(id, 1)
    if (seq !== loadSeq) return
    const headCommitID = head.items?.[0]?.id ?? ''
    baseCommitID.value = headCommitID
    const draft = await docApi.getDraft(id)
    if (seq !== loadSeq) return
    const d: Draft | null = draft.draft
    markdown.value = d?.content ?? (
      headCommitID ? (await docApi.getCommitContent(id, headCommitID)).content : ''
    )
    if (seq !== loadSeq) return
    ready.value = true
    // 首次预览渲染：默认开启时进入页面即为最终内容渲染，不等打字/切换（M14 提交 3 回归修复）
    if (previewOn.value) void renderPreviewNow(markdown.value)
    // Link suggestions are optional; a tree request failure must not hide the editor.
    try {
      const nodes = (await docApi.tree()).nodes
      if (seq === loadSeq) links.value = flattenLinks(nodes)
    } catch { /* keep editing without suggestions */ }
  } catch (e) {
    if (seq === loadSeq) {
      const status = toApiError(e).status
      loadError.value = t(status === 404 ? 'common.notFound' : status === 403 ? 'common.forbidden' : 'common.loadFailed')
    }
  }
}

function flattenLinks(nodes: TreeNode[], parentPath = ''): { title: string; path: string }[] {
  const out: { title: string; path: string }[] = []
  for (const n of nodes) {
    const path = parentPath ? `${parentPath}/${n.slug}` : n.slug
    out.push({ title: n.title, path })
    out.push(...flattenLinks(n.children, path))
  }
  return out
}


// 有效内容宽度足够时才默认左右分栏；窄屏优先显示源码，预览由用户切换。
const isWide = useMediaQuery('(min-width: 1280px)')
const previewOn = ref(isWide.value)
const showEditor = computed(() => isWide.value || !previewOn.value)
const previewHtml = ref('')
const previewError = ref(false)
const previewEl = ref<HTMLElement | null>(null)
let previewSeq = 0

watch(() => props.path, (p) => void loadDoc(p), { immediate: true })

let pvTimer: ReturnType<typeof setTimeout> | null = null
async function renderPreviewNow(md: string) {
  const seq = ++previewSeq
  try {
    const rendered = await docApi.preview(md)
    if (seq !== previewSeq) return
    previewHtml.value = rendered.html
    previewError.value = false
    await nextTick()
    if (seq === previewSeq && previewEl.value) await enhanceMarkdownExtras(previewEl.value)
  } catch {
    if (seq === previewSeq) previewError.value = true
  }
}
function schedulePreview(md: string) {
  if (!previewOn.value) return
  if (pvTimer) clearTimeout(pvTimer)
  pvTimer = setTimeout(() => {
    pvTimer = null
    void renderPreviewNow(md)
  }, 500)
}
function togglePreview() {
  previewOn.value = !previewOn.value
  if (!previewOn.value) previewSeq++
  if (previewOn.value) void renderPreviewNow(markdown.value)
}

watch(isWide, (wide) => {
  previewOn.value = wide
  if (wide) void renderPreviewNow(markdown.value)
  else previewSeq++
})

function onEditorChange(md: string) {
  markdown.value = md
  autosave.schedule(md)
  schedulePreview(md)
}

// T9.5：离开确认（ED-09）——脏状态（正文/标题未落盘）时路由离开需确认；
// 直接关闭页面走 beforeunload。
function titleDirty(): boolean {
  return title.value.trim() !== savedTitle.value
}
function isDirty(): boolean {
  return ['dirty', 'saving', 'error'].includes(autosave.status.value) || titleDirty()
}
const leaveConfirmed = ref(false)
onBeforeRouteLeave(async () => {
  if (leaveConfirmed.value || !isDirty()) return true
  try {
    await ElMessageBox.confirm(t('doc.leaveConfirm'), { type: 'warning' })
  } catch {
    return false
  }
  await autosave.flushNow()
  try {
    await persistTitleNow()
  } catch {
    ElMessage.error(t('doc.saveFailed'))
    return false
  }
  if (autosave.status.value === 'error') {
    ElMessage.error(t('doc.saveFailed'))
    return false
  }
  if (titleDirty()) {
    ElMessage.error(t(title.value.trim() ? 'doc.saveFailed' : 'doc.titleRequired'))
    return false
  }
  leaveConfirmed.value = true
  return true
})
function onBeforeUnload(e: BeforeUnloadEvent) {
  if (!isDirty()) return
  e.preventDefault()
  e.returnValue = ''
}
onMounted(() => window.addEventListener('beforeunload', onBeforeUnload))
onBeforeUnmount(() => window.removeEventListener('beforeunload', onBeforeUnload))

async function commitAndExit() {
  if (committing.value) return
  if (!title.value.trim()) {
    ElMessage.error(t('doc.titleRequired'))
    return
  }
  committing.value = true
  try {
    await autosave.flushNow()
    if (titleTimer) {
      clearTimeout(titleTimer)
      titleTimer = null
    }
    const t = title.value.trim()
    await docApi.commit(docID.value, baseCommitID.value, markdown.value, 'edit', t || undefined)
    leaveConfirmed.value = true
    await router.push(`/docs/${props.path}`)
  } catch (err) {
    const status = (err as { status?: number }).status
    if (status === 409) {
      ElMessage.error(t('doc.conflict'))
    } else {
      ElMessage.error(t('doc.saveFailed'))
    }
  } finally {
    committing.value = false
  }
}

// 放弃修改退出：取消挂起自动保存、清服务端草稿、不提交；失败仅提示不阻塞离开。
async function discardAndExit() {
  autosave.reset()
  if (titleTimer) {
    clearTimeout(titleTimer)
    titleTimer = null
  }
  try {
    await docApi.deleteDraft(docID.value)
  } catch {
    ElMessage.error(t('doc.discardDraftFailed'))
  }
  leaveConfirmed.value = true
  router.push(`/docs/${props.path}`)
}

// 可见性切换：PATCH 自身 visibility，再以生效值回显（restricted 祖先下改 standard 仍受限）。
// 不整页重载，避免丢弃未保存的编辑内容。
async function refreshVisibility() {
  try {
    const r = await docApi.get(docID.value)
    visibility.value = r.document.effective_visibility ?? 'standard'
  } catch { /* 回显失败保持现状，下次加载纠正 */ }
}

async function onVisibilityChange() {
  try {
    await docApi.patch(docID.value, { visibility: visibility.value })
  } catch {
    ElMessage.error(t('doc.visibilityFailed'))
  }
  await refreshVisibility()
}
</script>

<template>
  <div data-test="edit-page">
    <nav class="text-sm text-[var(--color-text)] mb-2">
      <RouterLink :to="`/docs/${props.path}`" data-test="back-to-doc">{{ t('doc.backToDoc') }}</RouterLink>
    </nav>
    <div v-if="loadError" class="text-red-600 space-x-2" data-test="edit-load-error">
      <span>{{ loadError }}</span>
      <button class="underline" @click="loadDoc(props.path)">{{ t('common.retry') }}</button>
    </div>
    <template v-if="ready">
      <div class="flex items-center gap-3 mb-2">
        <input v-model="title" :aria-label="t('doc.titlePlaceholder')" class="min-w-0 flex-1 text-xl font-semibold border-none outline-none" />
        <select
          v-model="visibility"
          data-test="visibility-select"
          class="text-sm border rounded px-1 py-1 shrink-0"
          :title="t('doc.visibility')"
          @change="onVisibilityChange"
        >
          <option value="standard">{{ t('doc.visibilityStandard') }}</option>
          <option value="restricted">{{ t('doc.visibilityRestricted') }}</option>
        </select>
        <button class="px-2 py-1 border rounded text-sm" data-test="preview-toggle" :aria-pressed="previewOn" @click="togglePreview">
          {{ t('doc.preview') }}
        </button>
      </div>
      <div class="flex min-w-0 gap-3">
        <EditorCanvasLazy
          v-show="showEditor"
          :key="docID"
          class="flex-1 min-w-0"
          :initial-markdown="markdown"
          :doc-i-d="docID"
          :links="links"
          :upload-image="(f: File) => attachmentApi.upload(docID, f).then(r => attachmentApi.rawURL(r.id))"
          @change="onEditorChange"
        />
        <aside
          v-if="previewOn"
          ref="previewEl"
          class="min-w-0 min-h-[300px] overflow-auto max-w-none"
          :class="isWide ? 'w-1/2 border-l pl-3' : 'w-full'"
          data-test="preview-pane"
        >
          <p v-if="!previewHtml && !previewError" class="text-sm text-[var(--color-text-light)]" data-test="preview-empty">
            {{ t('doc.previewEmpty') }}
          </p>
          <div v-if="previewError" class="mb-2 text-sm text-red-600" data-test="preview-error">
            {{ t('doc.previewFailed') }}
            <button class="underline ml-1" @click="renderPreviewNow(markdown)">{{ t('common.retry') }}</button>
          </div>
          <div class="prose prose-sm max-w-none" v-html="previewHtml" />
        </aside>
      </div>
      <div class="flex items-center gap-3 mt-3">
        <span data-test="autosave-status" :data-status="autosave.status.value" aria-live="polite">{{ t(`doc.autosave.${autosave.status.value}`) }}</span>
        <button class="px-3 py-1 bg-blue-600 text-white rounded disabled:opacity-40" data-test="save-exit" :disabled="committing" @click="commitAndExit">{{ committing ? t('doc.autosave.saving') : t('doc.saveExit') }}</button>
        <button class="px-3 py-1 border rounded text-sm" data-test="discard-exit" @click="discardAndExit">{{ t('doc.discard') }}</button>
      </div>
    </template>
  </div>
</template>

<script lang="ts">
// 懒加载：编辑器代码仅在本路由被访问时拉取（ED-01/T7.4 断言依据）。
import { defineAsyncComponent } from 'vue'
export default {
  components: {
    EditorCanvasLazy: defineAsyncComponent(
      () => import('@/components/editor/EditorCanvas.vue'),
    ),
  },
}
</script>
