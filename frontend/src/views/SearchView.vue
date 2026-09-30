<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { searchApi, type SearchHit } from '@/api'
import treeStore from '@/stores/tree'
import UiCard from '@/components/ui/UiCard.vue'
import UiSearchField from '@/components/ui/UiSearchField.vue'

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
    <form class="search-form" role="search" @submit.prevent="run">
      <UiSearchField
        id="global-search-input"
        v-model="q"
        data-test="search-input"
        :placeholder="t('search.placeholder')"
        :submit-label="t('common.search')"
        :loading-label="t('common.loading')"
        :loading="loading"
        @submit="run"
      />
    </form>
    <p class="mt-1 text-xs text-[var(--color-text-light)]">{{ t('search.shortcut') }}</p>
    <p v-if="loading" class="mt-4 text-[var(--color-text)]">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="mt-4 text-[var(--color-danger)]" data-test="search-error">{{ t('common.loadFailed') }}</p>
    <p v-if="hits.length" class="search-result-count" data-test="search-result-count">{{ t('search.resultCount', { count: hits.length }) }}</p>
    <ul v-if="hits.length" class="search-results" data-test="search-hits">
      <UiCard v-for="h in hits" :key="h.document_id" as="li" padding="sm">
        <RouterLink :to="`/docs/${hitPath(h)}`" class="font-medium text-[var(--color-primary)] hover:underline">{{ h.title }}</RouterLink>
        <div class="mt-1 text-sm text-[var(--color-text-light)]">{{ snippetText(h.snippet) }}</div>
      </UiCard>
    </ul>
    <p v-else-if="searched && !error" data-test="no-results">{{ t('search.noResults') }}</p>
  </div>
</template>

<style scoped>
[data-test="search-page"] { max-width: 52rem; margin: 0 auto; }
[data-test="search-page"] > h1 { color: var(--color-heading); font-size: 1.5rem; font-weight: 700; }
.search-form { max-width: 44rem; margin-top: 1rem; }
.search-result-count { margin-top: 1rem; color: var(--color-text-light); font-size: .8rem; }
.search-results { display: grid; gap: .6rem; max-width: 44rem; margin: .5rem 0 0; padding: 0; list-style: none; }
</style>
