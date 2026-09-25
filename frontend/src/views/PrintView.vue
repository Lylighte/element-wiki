<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { docApi } from '@/api'
import { toApiError } from '@/api/client'
import { enhanceMarkdownExtras } from '@/utils/enhance'

const props = defineProps<{ path: string }>()
const { t } = useI18n()
const status = ref<'loading' | 'ready' | 'notFound' | 'forbidden' | 'error'>('loading')
const title = ref('')
const html = ref('')
const contentEl = ref<HTMLElement | null>(null)
const prepared = ref(false)
const bodyStartsWithTitle = computed(() => {
  if (!title.value || !html.value) return false
  const first = new DOMParser().parseFromString(html.value, 'text/html').body.firstElementChild
  return first?.tagName === 'H1' && first.textContent?.trim() === title.value.trim()
})

function printNow() {
  if (prepared.value) window.print()
}

async function waitForPrintResources(root: HTMLElement) {
  const images = Array.from(root.querySelectorAll('img'))
  const ready = Promise.all([
    document.fonts.ready,
    ...images.map((img) => img.decode().catch(() => {})),
  ])
  await Promise.race([ready, new Promise((resolve) => setTimeout(resolve, 3000))])
  await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())))
}

let loadSeq = 0
async function loadDoc(path: string) {
  const seq = ++loadSeq
  status.value = 'loading'
  title.value = ''
  html.value = ''
  prepared.value = false
  try {
    const result = await docApi.resolve(path)
    if (seq !== loadSeq) return
    title.value = result.document.title
    html.value = result.render.html
    document.title = `${title.value} — ${t('doc.print')}`
    status.value = 'ready'
    await nextTick()
    const content = contentEl.value
    if (!content || seq !== loadSeq) return
    try {
      await enhanceMarkdownExtras(content)
    } catch {
      // A failed optional diagram must not prevent printing the document text.
    }
    await waitForPrintResources(content)
    if (seq === loadSeq) {
      prepared.value = true
      printNow()
    }
  } catch (error) {
    if (seq !== loadSeq) return
    const code = toApiError(error).status
    status.value = code === 404
      ? 'notFound'
      : code === 401 || code === 403 ? 'forbidden' : 'error'
  }
}
watch(() => props.path, (path) => void loadDoc(path), { immediate: true })
</script>

<template>
  <main class="print-page" data-test="print-page">
    <div class="print-toolbar">
      <span>{{ title || t('doc.print') }}</span>
      <button type="button" data-test="print-now" :disabled="!prepared" @click="printNow">
        {{ t('doc.print') }}
      </button>
    </div>
    <p v-if="status === 'loading'">{{ t('common.loading') }}</p>
    <p v-else-if="status !== 'ready'" role="alert">
      {{ t(status === 'notFound' ? 'common.notFound' : status === 'forbidden' ? 'common.forbidden' : 'common.loadFailed') }}
      <button v-if="status === 'error'" type="button" @click="loadDoc(props.path)">
        {{ t('common.retry') }}
      </button>
    </p>
    <article v-else class="print-document" data-test="print-document">
      <h1 v-if="!bodyStartsWithTitle">{{ title }}</h1>
      <!-- eslint-disable-next-line vue/no-v-html：服务端已消毒（RD-07） -->
      <div ref="contentEl" class="prose max-w-none" data-test="print-html" v-html="html" />
    </article>
  </main>
</template>
