<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { docApi, authApi, type DocumentMeta } from '@/api'
import treeStore from '@/stores/tree'
import { crumbsFor } from '@/utils/breadcrumbs'
import { ElDrawer } from 'element-plus'
import CommentsPanel from '@/components/doc/CommentsPanel.vue'
import AttachmentsPanel from '@/components/doc/AttachmentsPanel.vue'

const props = defineProps<{ id: string }>()
const meta = ref<DocumentMeta | null>(null)
const html = ref('')
const error = ref('')
const meID = ref<string | null>(null)
const canEdit = ref(false)
const canHistory = ref(false)
const historyOpen = ref(false)
const commits = ref<{ id: string; commit_no: number; message: string; created_at: number }[]>([])

onMounted(async () => {
  treeStore.load()
  try {
    const m = await authApi.me()
    meID.value = m.user.id
    canEdit.value = m.permissions.includes('document.update')
    canHistory.value = m.permissions.includes('version.read')
  } catch {
    /* 匿名 */
  }
  try {
    const r = await docApi.render(props.id)
    html.value = r.html
    const mm = await docApi.get(props.id)
    meta.value = mm.document
  } catch (e) {
    error.value = String(e)
  }
})

const crumbs = computed(() => crumbsFor(treeStore.state.nodes, props.id))

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
      <button
        v-if="canHistory"
        data-test="btn-history"
        class="text-sm px-2 py-1 border rounded"
        @click="openHistory"
      >历史</button>
      <RouterLink
        v-if="canUpdate()"
        :to="`/docs/${props.id}/edit`"
        data-test="btn-edit"
        class="text-sm px-2 py-1 bg-blue-600 text-white rounded"
      >编辑</RouterLink>
    </div>
    <p v-if="error" class="text-red-600">{{ error }}</p>
    <!-- eslint-disable-next-line vue/no-v-html：服务端已消毒（RD-07） -->
    <div data-test="doc-html" v-html="html" />

    <CommentsPanel :doc-i-d="props.id" :me="meID ?? ''" :is-admin="false" />
    <AttachmentsPanel :doc-i-d="props.id" :editable="canEdit" />

    <el-drawer v-model="historyOpen" title="历史版本" size="40%" data-test="history-drawer">
      <ul class="space-y-2 text-sm">
        <li v-for="c in commits" :key="c.id" class="border rounded p-2 flex justify-between items-center">
          <span>#{{ c.commit_no }} {{ c.message || '(无说明)' }}<br />
            <span class="text-gray-400">{{ new Date(c.created_at).toLocaleString() }}</span></span>
          <button class="underline" @click="doRevert(c.id)">回滚到此版</button>
        </li>
      </ul>
    </el-drawer>
  </article>
</template>
