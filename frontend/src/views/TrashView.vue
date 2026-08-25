<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { trashApi, type TrashItem } from '@/api'

const { t } = useI18n()
const items = ref<TrashItem[]>([])

async function refresh() {
  items.value = (await trashApi.list()).items
}
onMounted(refresh)

async function restore(id: string) {
  await trashApi.restore(id)
  await refresh()
}
async function purge(id: string) {
  await trashApi.purge(id)
  await refresh()
}
</script>

<template>
  <div data-test="trash-page">
    <h1 class="text-xl font-semibold mb-3">{{ t('trash.title') }}</h1>
    <ul class="text-sm space-y-1">
      <li v-for="it in items" :key="it.id" class="flex gap-3 items-center" data-test="trash-item">
        <span>{{ it.title }}</span>
        <button class="underline" @click="restore(it.id)">{{ t('trash.restore') }}</button>
        <button class="text-red-600 underline" @click="purge(it.id)">{{ t('trash.purge') }}</button>
      </li>
    </ul>
  </div>
</template>
