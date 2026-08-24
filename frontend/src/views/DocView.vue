<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { docApi, type DocumentMeta } from '@/api'

const props = defineProps<{ id: string }>()
const meta = ref<DocumentMeta | null>(null)
const html = ref('')
const error = ref('')

onMounted(async () => {
  try {
    const r = await docApi.render(props.id)
    html.value = r.html
    const m = await docApi.get(props.id)
    meta.value = m.document
  } catch (e) {
    error.value = String(e)
  }
})
</script>

<template>
  <article data-test="doc-page">
    <h1 v-if="meta" class="text-xl font-semibold mb-2">{{ meta.title }}</h1>
    <p v-if="error" class="text-red-600">{{ error }}</p>
    <!-- eslint-disable-next-line vue/no-v-html：服务端已消毒（RD-07） -->
    <div data-test="doc-html" v-html="html" />
  </article>
</template>
