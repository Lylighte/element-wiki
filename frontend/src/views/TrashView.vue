<script setup lang="ts">
import { onMounted, ref } from 'vue'
import type { TrashItem } from '@/api'

const items = ref<TrashItem[]>([])
async function refresh() {
  const r = await fetch('/v1/trash', { credentials: 'include' })
  if (r.ok) {
    const data = await r.json()
    items.value = data.items ?? []
  }
}
onMounted(refresh)
async function restore(id: string) {
  await fetch(`/v1/trash/${id}/restore`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: '{}',
  })
  await refresh()
}
async function purge(id: string) {
  await fetch(`/v1/trash/${id}`, { method: 'DELETE', credentials: 'include' })
  await refresh()
}
</script>

<template>
  <div data-test="trash-page">
    <ul>
      <li v-for="it in items" :key="it.id">
        {{ it.title }}
        <button @click="restore(it.id)">restore</button>
        <button class="text-red-600" @click="purge(it.id)">purge</button>
      </li>
    </ul>
  </div>
</template>
