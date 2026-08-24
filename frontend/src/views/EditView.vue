<script setup lang="ts">
// 编辑路由：懒加载 EditorCanvas（只读页零加载，AGENTS §2）。
import { onMounted, ref } from 'vue'
import { docApi, attachmentApi, type Draft } from '@/api'
import treeStore from '@/stores/tree'
import { useAutosave } from '@/composables/useAutosave'
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

onMounted(async () => {
  treeStore.load()
  try {
    const meta = await docApi.get(props.id)
    title.value = meta.document.title
    const head = await fetch(`/v1/documents/${props.id}/commits?limit=1`, {
      credentials: 'include',
    })
    if (head.ok) {
      const j = await head.json()
      baseCommitID.value = j.items?.[0]?.id ?? ''
    }
    const draft = await docApi.getDraft(props.id)
    const d: Draft | null = draft.draft
    markdown.value = d?.content ?? ''
    ready.value = true
    const nodes = (await docApi.tree()).nodes
    titles.value = flattenTitles(nodes)
  } catch (e) {
    loadError.value = String(e)
  }
})

function flattenTitles(nodes: ReturnType<typeof Object.values> extends never ? never : any[]): string[] {
  const out: string[] = []
  for (const n of nodes as { title: string; children: any[] }[]) {
    out.push(n.title)
    out.push(...flattenTitles(n.children))
  }
  return out
}


async function commitAndExit() {
  await autosave.flushNow()
  try {
    await docApi.commit(props.id, baseCommitID.value, markdown.value, 'edit')
    location.href = `/docs/${props.id}`
  } catch (err) {
    const status = (err as { status?: number }).status
    if (status === 409) {
      alert(t("doc.conflict"))
      return
    }
    throw err
  }
}
</script>

<template>
  <div data-test="edit-page">
    <nav class="text-sm text-gray-500 mb-2">
      <RouterLink :to="`/docs/${props.id}`" data-test="back-to-doc">← 返回文档</RouterLink>
    </nav>
    <p v-if="loadError" class="text-red-600">{{ loadError }}</p>
    <template v-if="ready">
      <input v-model="title" class="w-full text-xl font-semibold border-none outline-none mb-2" />
      <EditorCanvasLazy
        :initial-markdown="markdown"
        :doc-i-d="props.id"
        :titles="titles"
        :upload-image="(f: File) => attachmentApi.upload(props.id, f).then(r => attachmentApi.rawURL(r.id))"
        @change="(md: string) => autosave.schedule(md)"
      />
      <div class="flex items-center gap-3 mt-3">
        <span data-test="autosave-status" :data-status="autosave.status.value">{{ autosave.status.value }}</span>
        <button class="px-3 py-1 bg-blue-600 text-white rounded" @click="commitAndExit">保存并退出</button>
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
