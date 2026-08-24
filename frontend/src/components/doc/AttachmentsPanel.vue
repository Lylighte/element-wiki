<script setup lang="ts">
// 附件面板（T5.5/T7.7）：列表 + 上传 + 删除。
import { onMounted, ref } from 'vue'
import { attachmentApi, type Attachment } from '@/api'

const props = defineProps<{ docID: string; editable: boolean }>()
const items = ref<Attachment[]>([])
const fileInput = ref<HTMLInputElement | null>(null)
const busy = ref(false)

async function refresh() {
  items.value = (await attachmentApi.list(props.docID)).items
}
onMounted(refresh)

async function upload(e: Event) {
  const input = e.target as HTMLInputElement
  const f = input.files?.[0]
  if (!f) return
  busy.value = true
  try {
    await attachmentApi.upload(props.docID, f)
    await refresh()
  } finally {
    busy.value = false
    input.value = ''
  }
}

async function remove(a: Attachment) {
  await attachmentApi.remove(a.id)
  await refresh()
}
</script>

<template>
  <section class="mt-8 border-t pt-4" data-test="attachments-panel">
    <h2 class="font-semibold mb-2">附件</h2>
    <ul class="text-sm space-y-1">
      <li v-for="a in items" :key="a.id" class="flex gap-2 items-center">
        <a :href="attachmentApi.rawURL(a.id)" target="_blank">{{ a.filename }}</a>
        <span class="text-gray-400">({{ a.size }}B)</span>
        <button v-if="editable" class="text-red-600 ml-auto" @click="remove(a)">×</button>
      </li>
    </ul>
    <label v-if="editable" class="inline-block mt-2 text-sm text-blue-600 cursor-pointer">
      上传附件
      <input type="file" class="hidden" @change="upload" />
    </label>
  </section>
</template>
