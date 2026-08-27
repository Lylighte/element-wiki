<script setup lang="ts">
// 附件面板（T5.5/T7.7）：列表 + 上传 + 删除。
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { attachmentApi, type Attachment } from '@/api'

const { t } = useI18n()

const props = defineProps<{ docID: string; editable: boolean }>()
const items = ref<Attachment[]>([])
const busy = ref(false)
const error = ref(false)
const actionError = ref(false)

async function refresh() {
  error.value = false
  try {
    items.value = (await attachmentApi.list(props.docID)).items
  } catch {
    error.value = true
  }
}
onMounted(refresh)

async function upload(e: Event) {
  const input = e.target as HTMLInputElement
  const f = input.files?.[0]
  if (!f) return
  busy.value = true
  actionError.value = false
  try {
    await attachmentApi.upload(props.docID, f)
    await refresh()
  } catch {
    actionError.value = true
  } finally {
    busy.value = false
    input.value = ''
  }
}

async function remove(a: Attachment) {
  busy.value = true
  actionError.value = false
  try {
    await attachmentApi.remove(a.id)
    await refresh()
  } catch {
    actionError.value = true
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <section class="mt-8 border-t pt-4" data-test="attachments-panel">
    <h2 class="font-semibold mb-2">{{ t('attachments.title') }}</h2>
    <div v-if="error" class="text-red-600 space-y-1" data-test="attachments-error">
      <p>{{ t('common.loadFailed') }}</p>
      <button class="underline" data-test="attachments-retry" @click="refresh">{{ t('common.retry') }}</button>
    </div>
    <p v-if="actionError" class="text-red-600" data-test="attachments-action-error">
      {{ t('common.loadFailed') }}
    </p>
    <ul class="text-sm space-y-1">
      <li v-for="a in items" :key="a.id" class="flex gap-2 items-center">
        <a :href="attachmentApi.rawURL(a.id)" target="_blank">{{ a.filename }}</a>
        <span class="text-gray-400">({{ a.size }}B)</span>
        <button v-if="editable" :disabled="busy" class="text-red-600 ml-auto" @click="remove(a)">×</button>
      </li>
    </ul>
    <label v-if="editable" class="inline-block mt-2 text-sm text-blue-600 cursor-pointer">
      {{ t('attachments.upload') }}
      <input type="file" class="hidden" :disabled="busy" @change="upload" />
    </label>
  </section>
</template>
