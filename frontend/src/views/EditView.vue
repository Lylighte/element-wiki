<script setup lang="ts">
// 编辑路由：懒加载 EditorCanvas（只读页零加载，AGENTS §2）。
import { onBeforeUnmount, onMounted, nextTick, ref, watch } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { docApi, attachmentApi, type Draft } from '@/api'
import treeStore from '@/stores/tree'
import { useAutosave } from '@/composables/useAutosave'
import { enhanceMarkdownExtras } from '@/utils/enhance'
import { useI18n } from 'vue-i18n'

const props = defineProps<{ id: string }>()
const { t } = useI18n()

const title = ref('')
const baseCommitID = ref('')
const markdown = ref('')
const titles = ref<string[]>([])
const ready = ref(false)
const loadError = ref('')

const autosave = useAutosave({
  delay: 1500,
  save: (content) => docApi.saveDraft(props.id, baseCommitID.value, content),
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
  await docApi.patch(props.id, { title: v })
  savedTitle.value = v
}

let loadSeq = 0
async function loadDoc(id: string) {
  const seq = ++loadSeq
  autosave.reset()
  title.value = ''
  savedTitle.value = ''
  baseCommitID.value = ''
  markdown.value = ''
  titles.value = []
  ready.value = false
  loadError.value = ''
  previewHtml.value = ''
  void treeStore.load().catch(() => {})
  try {
    const meta = await docApi.get(id)
    if (seq !== loadSeq) return
    title.value = meta.document.title
    savedTitle.value = meta.document.title
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
    const nodes = (await docApi.tree()).nodes
    if (seq !== loadSeq) return
    titles.value = flattenTitles(nodes)
  } catch (e) {
    if (seq === loadSeq) loadError.value = String(e)
  }
}

function flattenTitles(nodes: ReturnType<typeof Object.values> extends never ? never : any[]): string[] {
  const out: string[] = []
  for (const n of nodes as { title: string; children: any[] }[]) {
    out.push(n.title)
    out.push(...flattenTitles(n.children))
  }
  return out
}


// T9.2：实时预览分栏——防抖调用服务端渲染；与提交共用 markdown 数据源
const previewOn = ref(false)
const previewHtml = ref('')
const previewEl = ref<HTMLElement | null>(null)

watch(() => props.id, (id) => void loadDoc(id), { immediate: true })

let pvTimer: ReturnType<typeof setTimeout> | null = null
async function renderPreviewNow(md: string) {
  try {
    previewHtml.value = (await docApi.preview(md)).html
    await nextTick()
    if (previewEl.value) await enhanceMarkdownExtras(previewEl.value)
  } catch {
    /* 预览失败静默保留上次内容 */
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
  if (previewOn.value) void renderPreviewNow(markdown.value)
}

function onEditorChange(md: string) {
  markdown.value = md
  autosave.schedule(md)
  schedulePreview(md)
}

// T9.5：离开确认（ED-09）——脏状态（正文/标题未落盘）时路由离开需确认；
// 直接关闭页面走 beforeunload。
function titleDirty(): boolean {
  return !!title.value.trim() && title.value.trim() !== savedTitle.value
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
  await autosave.flushNow().catch(() => {})
  await persistTitleNow().catch(() => {})
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
  await autosave.flushNow()
  try {
    await persistTitleNow()
  } catch {
    /* 标题 PATCH 失败由 commit title 兜底 */
  }
  try {
    const t = title.value.trim()
    await docApi.commit(props.id, baseCommitID.value, markdown.value, 'edit', t || undefined)
    leaveConfirmed.value = true
    location.href = `/docs/${props.id}`
  } catch (err) {
    const status = (err as { status?: number }).status
    if (status === 409) {
      ElMessage.error(t("doc.conflict"))
      return
    }
    throw err
  }
}
</script>

<template>
  <div data-test="edit-page">
    <nav class="text-sm text-gray-500 mb-2">
      <RouterLink :to="`/docs/${props.id}`" data-test="back-to-doc">{{ t('doc.backToDoc') }}</RouterLink>
    </nav>
    <p v-if="loadError" class="text-red-600">{{ loadError }}</p>
    <template v-if="ready">
      <div class="flex items-center gap-3 mb-2">
        <input v-model="title" class="flex-1 text-xl font-semibold border-none outline-none" />
        <button class="px-2 py-1 border rounded text-sm" data-test="preview-toggle" @click="togglePreview">
          {{ t('doc.preview') }}
        </button>
      </div>
      <div class="flex gap-3">
        <EditorCanvasLazy
          :key="props.id"
          class="flex-1 min-w-0"
          :initial-markdown="markdown"
          :doc-i-d="props.id"
          :titles="titles"
          :upload-image="(f: File) => attachmentApi.upload(props.id, f).then(r => attachmentApi.rawURL(r.id))"
          @change="onEditorChange"
        />
        <aside
          v-if="previewOn"
          ref="previewEl"
          class="w-1/2 border-l pl-3 overflow-auto prose prose-sm max-w-none"
          data-test="preview-pane"
          v-html="previewHtml"
        />
      </div>
      <div class="flex items-center gap-3 mt-3">
        <span data-test="autosave-status" :data-status="autosave.status.value">{{ autosave.status.value }}</span>
        <button class="px-3 py-1 bg-blue-600 text-white rounded" data-test="save-exit" @click="commitAndExit">{{ t('doc.saveExit') }}</button>
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
