<script setup lang="ts">
// 评论面板（CO-01/02）：403 门闩时整体隐藏；
// 站点信息已加载且 comments_enabled=false 时直接不发请求（避免必现的 403）。
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { commentApi, reportApi, type CommentItem } from '@/api'
import { toApiError } from '@/api/client'
import { permission } from '@/permissionsProxy'
import siteStore from '@/stores/site'
import { formatSiteDate } from '@/utils/siteDate'
import { ElMessage, ElMessageBox } from 'element-plus'

const props = defineProps<{ docID: string; me: string | null; isAdmin: boolean }>()

const { t, locale } = useI18n()
const hidden = ref(false)
const error = ref(false)
const items = ref<CommentItem[]>([])
const draft = ref('')
const pendingNotice = ref(false)

async function refresh() {
  error.value = false
  if (siteStore.state.commentsEnabled === false) {
    hidden.value = true
    return
  }
  try {
    const r = await commentApi.list(props.docID, 100)
    items.value = r.items
  } catch (e) {
    if (toApiError(e).status === 403) hidden.value = true // comments disabled / 无权限
    else error.value = true
  }
}
onMounted(refresh)

async function submit() {
  if (!draft.value.trim()) return
  const result = await commentApi.add(props.docID, draft.value)
  pendingNotice.value = result.comment.status === 'pending'
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
async function reportComment(c: CommentItem) {
  try {
    const { value } = await ElMessageBox.prompt(t('admin.reportReason'), t('admin.reportButton'), { inputType: 'textarea', inputValidator: (v) => !!v?.trim() && v.trim().length >= 5 })
    await reportApi.create('comment', c.id, value)
    ElMessage.success(t('admin.reportSubmitted'))
  } catch { /* prompt cancel */ }
}
</script>

<template>
  <section v-if="!hidden" class="mt-8 border-t pt-4" data-test="comments-panel">
    <h2 class="font-semibold mb-2">{{ t('comments.title') }}</h2>
    <p v-if="pendingNotice" class="mb-3 rounded border border-amber-300 bg-amber-50 p-2 text-sm text-amber-900 dark:bg-amber-950 dark:text-amber-100" data-test="comment-pending-notice">{{ t('comments.pendingNotice') }}</p>
    <div v-if="error" class="text-red-600 space-y-1" data-test="comments-error">
      <p>{{ t('common.loadFailed') }}</p>
      <button class="underline" data-test="comments-retry" @click="refresh">{{ t('common.retry') }}</button>
    </div>
    <ul class="space-y-2 mb-3">
      <li v-for="c in items" :key="c.id" class="border rounded p-2 text-sm">
        <div class="flex justify-between">
          <span>{{ formatSiteDate(c.created_at, locale, siteStore.state.timezone) }}</span>
          <span class="flex gap-3"><button v-if="props.me && c.author_id !== props.me" class="text-[var(--color-text-light)]" @click="reportComment(c)">{{ t('admin.reportButton') }}</button><button v-if="canDelete(c)" class="text-red-600" @click="remove(c.id)">×</button></span>
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
