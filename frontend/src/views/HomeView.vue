<script setup lang="ts">
// 首页即 slug=home 的根文档：存在则跳转，不存在给出创建引导（DM-01 特殊页）。
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import treeStore from '@/stores/tree'
import { docApi, type TreeNode } from '@/api'
import { can } from '@/permissions'

const router = useRouter()
const { t } = useI18n()
const loading = ref(true)
const error = ref(false)
const homeID = ref('')

function findHome(nodes: TreeNode[]): string {
  for (const n of nodes) {
    if (n.parent_id === null && n.slug === 'home') return n.id
    const sub = findHome(n.children)
    if (sub) return sub
  }
  return ''
}

async function loadHome() {
  loading.value = true
  error.value = false
  try {
    await treeStore.load()
    homeID.value = findHome(treeStore.state.nodes)
    if (homeID.value) router.replace(`/docs/${homeID.value}`)
  } catch {
    error.value = true
  } finally {
    loading.value = false
  }
}

onMounted(() => void loadHome())

const title = ref('')
const creating = ref(false)
async function createHome() {
  creating.value = true
  try {
    const r = await docApi.create({ slug: 'home', title: title.value || 'Home' })
    router.replace(`/docs/${r.document.id}/edit`)
  } finally {
    creating.value = false
  }
}
</script>

<template>
  <div v-if="loading" class="text-gray-500">…</div>

  <div v-else-if="error" class="max-w-md mx-auto mt-16 text-center space-y-3" data-test="home-error">
    <p class="text-red-600">{{ t('common.loadFailed') }}</p>
    <button class="underline" data-test="home-retry" @click="loadHome">{{ t('common.retry') }}</button>
  </div>

  <div v-else-if="homeID" class="hidden">
    <!-- 有首页文档：直接进入其渲染页 -->
  </div>

  <div v-else class="max-w-md mx-auto mt-16 text-center space-y-4" data-test="home-empty">
    <h1 class="text-2xl font-semibold">{{ t('common.appName') }}</h1>
    <p class="text-gray-500">{{ t('home.empty') }}</p>

    <form
      v-if="can('document.create')"
      class="space-y-3 bg-white border rounded p-4"
      @submit.prevent="createHome"
    >
      <input
        v-model="title"
        data-test="home-title"
        :placeholder="t('home.titlePlaceholder')"
        class="w-full border rounded px-2 py-1"
      />
      <button
        type="submit"
        :disabled="creating"
        data-test="create-home-btn"
        class="w-full py-2 rounded bg-blue-600 text-white disabled:opacity-40"
      >
        {{ t('home.createAndEdit') }}
      </button>
    </form>
    <p v-else-if="can('document_read')" class="text-gray-400 text-sm">
      {{ t('home.pickSidebar') }}
    </p>
  </div>
</template>
