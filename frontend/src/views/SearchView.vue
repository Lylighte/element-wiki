<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { searchApi, type SearchHit } from '@/api'

const { t } = useI18n()

const q = ref('')
const hits = ref<SearchHit[]>([])
const searched = ref(false)
const loading = ref(false)
const error = ref(false)
function snippetText(snippet: string): string {
  return new DOMParser().parseFromString(snippet, 'text/html').body.textContent ?? ''
}

async function run() {
  loading.value = true
  error.value = false
  try {
    const r = await searchApi.query(q.value, 20)
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
    <form @submit.prevent="run">
      <input v-model="q" data-test="search-input" :placeholder="t('search.placeholder')" class="border rounded px-3 py-2 w-full" />
    </form>
    <p v-if="loading" class="mt-4 text-gray-500">{{ t('common.loading') }}</p>
    <p v-else-if="error" class="mt-4 text-red-600" data-test="search-error">{{ t('common.loadFailed') }}</p>
    <ul v-if="hits.length" class="mt-4 space-y-2" data-test="search-hits">
      <li v-for="h in hits" :key="h.document_id">
        <RouterLink :to="`/docs/${h.document_id}`">{{ h.title }}</RouterLink>
        <div>{{ snippetText(h.snippet) }}</div>
      </li>
    </ul>
    <p v-else-if="searched && !error" data-test="no-results">{{ t('search.noResults') }}</p>
  </div>
</template>
