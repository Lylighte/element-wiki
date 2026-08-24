<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { docApi, authApi, type DocumentMeta } from '@/api'
import treeStore from '@/stores/tree'
import { crumbsFor } from '@/utils/breadcrumbs'
import CommentsPanel from '@/components/doc/CommentsPanel.vue'
import AttachmentsPanel from '@/components/doc/AttachmentsPanel.vue'

const props = defineProps<{ id: string }>()
const meta = ref<DocumentMeta | null>(null)
const html = ref('')
const error = ref('')
const meID = ref<string | null>(null)
const canEdit = ref(false)

onMounted(async () => {
  treeStore.load()
  try {
    const m = await authApi.me()
    meID.value = m.user.id
    canEdit.value = m.permissions.includes('document.update')
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
</script>

<template>
  <article data-test="doc-page">
    <nav v-if="crumbs.length" class="text-sm text-gray-500 mb-2" data-test="breadcrumb">
      <template v-for="(c, i) in crumbs" :key="c.id">
        <RouterLink :to="`/docs/${c.id}`" class="hover:underline">{{ c.title }}</RouterLink>
        <span v-if="i < crumbs.length - 1"> / </span>
      </template>
    </nav>
    <h1 v-if="meta" class="text-xl font-semibold mb-2">{{ meta.title }}</h1>
    <p v-if="error" class="text-red-600">{{ error }}</p>
    <!-- eslint-disable-next-line vue/no-v-html：服务端已消毒（RD-07） -->
    <div data-test="doc-html" v-html="html" />

    <CommentsPanel :doc-i-d="props.id" :me="meID ?? ''" :is-admin="false" />
    <AttachmentsPanel :doc-i-d="props.id" :editable="canEdit" />
  </article>
</template>
