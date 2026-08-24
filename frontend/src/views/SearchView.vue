<script setup lang="ts">
import { ref } from 'vue'
import { searchApi, type SearchHit } from '@/api'

const q = ref('')
const hits = ref<SearchHit[]>([])
const searched = ref(false)
async function run() {
  const r = await searchApi.query(q.value, 20)
  hits.value = r.items
  searched.value = true
}
</script>

<template>
  <div data-test="search-page">
    <form @submit.prevent="run">
      <input v-model="q" data-test="search-input" class="border rounded px-3 py-2 w-full" />
    </form>
    <ul v-if="hits.length" class="mt-4 space-y-2" data-test="search-hits">
      <li v-for="h in hits" :key="h.document_id">
        <RouterLink :to="`/docs/${h.document_id}`" v-html="h.title" />
        <div v-html="h.snippet" />
      </li>
    </ul>
    <p v-else-if="searched">no results</p>
  </div>
</template>
