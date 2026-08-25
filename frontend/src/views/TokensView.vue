<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { tokenApi, type ApiToken } from '@/api'

const { t } = useI18n()

const items = ref<ApiToken[]>([])
const plaintext = ref('')
const name = ref('')

async function refresh() {
  const r = await tokenApi.list()
  items.value = r.items
}
onMounted(refresh)

async function create() {
  if (!name.value) return
  const res = await tokenApi.create(name.value)
  plaintext.value = res.token
  name.value = ''
  await refresh()
}
async function revoke(id: string) {
  await tokenApi.revoke(id)
  await refresh()
}
</script>

<template>
  <div data-test="tokens-page" class="space-y-4">
    <form class="flex gap-2" @submit.prevent="create">
      <input v-model="name" :placeholder="t('tokens.name')" class="border rounded px-2 py-1" />
      <button class="px-3 py-1 bg-blue-600 text-white rounded">{{ t('tokens.create') }}</button>
    </form>
    <code v-if="plaintext" data-test="plaintext">{{ plaintext }}</code>
    <ul>
      <li v-for="tok in items" :key="tok.id">
        {{ tok.name }} ({{ tok.prefix }}…)
        <button class="text-red-600" @click="revoke(tok.id)">{{ t('tokens.revoke') }}</button>
      </li>
    </ul>
  </div>
</template>
