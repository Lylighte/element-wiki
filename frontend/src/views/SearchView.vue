<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { searchApi, type SearchHit } from '@/api'
import treeStore from '@/stores/tree'

const { t } = useI18n()

const q = ref('')
const hits = ref<SearchHit[]>([])
const searched = ref(false)
const loading = ref(false)
const error = ref(false)
function snippetText(snippet: string): string {
  return new DOMParser().parseFromString(snippet, 'text/html').body.textContent ?? ''
}

// 05 计划提交 4：搜索结果链接用 slug 路径（树内反查；搜索结果均为存活文档）。
function hitPath(h: SearchHit): string {
  return treeStore.pathSlugOf(treeStore.state.nodes, h.document_id) || h.document_id
}

async function run() {
  const query = q.value.trim()
  if (!query || loading.value) return
  loading.value = true
  error.value = false
  try {
    const r = await searchApi.query(query, 20)
    // Result links need the full slug path; wait for the tree before rendering them.
    await treeStore.load()
    hits.value = r.items
    searched.value = true
  } catch {
    hits.value = []
    error.value = true
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div data-test="search-page">
    <h1 class="text-xl font-semibold mb-3">{{ t('common.search') }}</h1>
    <form class="flex gap-2 max-w-3xl" role="search" @submit.prevent="run">
      <input v-model="q" data-test="search-input" :placeholder="t('search.placeholder')" class="min-w-0 flex-1 border rounded px-3 py-2" />
      <button type="submit" class="rounded bg-[var(--color-primary)] px-4 py-2 text-white disabled:opacity-40" :disabled="!q.trim() || loading" data-test="search-submit">{{ t('common.search') }}</button>
    </form>
    <p v-if="loading" class="mt-4 text-[var(--color-text)]">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="mt-4 text-red-600" data-test="search-error">{{ t('common.loadFailed') }}</p>
    <ul v-if="hits.length" class="mt-4 max-w-3xl space-y-2" data-test="search-hits">
      <li v-for="h in hits" :key="h.document_id" class="rounded-lg border border-[var(--color-border)] bg-[var(--color-card-background)] p-3">
        <RouterLink :to="`/docs/${hitPath(h)}`" class="font-medium text-[var(--color-primary)] hover:underline">{{ h.title }}</RouterLink>
        <div class="mt-1 text-sm text-[var(--color-text-light)]">{{ snippetText(h.snippet) }}</div>
      </li>
    </ul>
    <p v-else-if="searched && !error" data-test="no-results">{{ t('search.noResults') }}</p>
  </div>
</template>
