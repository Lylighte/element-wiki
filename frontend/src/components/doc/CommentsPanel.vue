<script setup lang="ts">
// 评论面板（CO-01/02）：403 门闩时整体隐藏。
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { commentApi, type CommentItem } from '@/api'
import { permission } from '@/permissionsProxy'

const props = defineProps<{ docID: string; me: string | null; isAdmin: boolean }>()

const { t } = useI18n()
const hidden = ref(false)
const items = ref<CommentItem[]>([])
const draft = ref('')

async function refresh() {
  try {
    const r = await commentApi.list(props.docID, 100)
    items.value = r.items
  } catch {
    hidden.value = true // 403 comments disabled / 无权限
  }
}
onMounted(refresh)

async function submit() {
  if (!draft.value.trim()) return
  await commentApi.add(props.docID, draft.value)
  draft.value = ''
  await refresh()
}

async function remove(id: string) {
  await commentApi.remove(id)
  await refresh()
}

function canDelete(c: CommentItem): boolean {
  return permission.has('comment.delete.any') || c.author_id === props.me
}
</script>

<template>
  <section v-if="!hidden" class="mt-8 border-t pt-4" data-test="comments-panel">
    <h2 class="font-semibold mb-2">{{ t('comments.title') }}</h2>
    <ul class="space-y-2 mb-3">
      <li v-for="c in items" :key="c.id" class="border rounded p-2 text-sm">
        <div class="flex justify-between">
          <span>{{ new Date(c.created_at).toLocaleString() }}</span>
          <button v-if="canDelete(c)" class="text-red-600" @click="remove(c.id)">×</button>
        </div>
        <div class="whitespace-pre-wrap">{{ c.content }}</div>
      </li>
    </ul>
    <form class="flex gap-2" @submit.prevent="submit">
      <textarea v-model="draft" :placeholder="t('comments.placeholder')" rows="2" class="flex-1 border rounded p-2" />
      <button type="submit" class="self-end px-3 py-1 bg-blue-600 text-white rounded">
        {{ t('comments.submit') }}
      </button>
    </form>
  </section>
</template>
