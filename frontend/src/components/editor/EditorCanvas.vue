<script setup lang="ts">
// 源码编辑器（05 计划提交 3）：textarea 始终展示原始 Markdown；
// 工具栏向光标处插入 Markdown 片段；图片走受控上传管线；[[ 补全浮层。
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  initialMarkdown: string
  docID: string
  titles: string[]
  uploadImage: (file: File) => Promise<string>
}>()

const emit = defineEmits<{
  (e: 'change', markdown: string): void
}>()
const { t } = useI18n()

const sourceText = ref(props.initialMarkdown || '')
const ta = ref<HTMLTextAreaElement | null>(null)
const imageInput = ref<HTMLInputElement | null>(null)

function onInput() {
  emit('change', sourceText.value)
  checkSuggest()
}

function getMarkdown(): string {
  return sourceText.value
}

function focusEditor() {
  ta.value?.focus()
}

watch(
  () => props.initialMarkdown,
  (v) => {
    if (v !== undefined && document.activeElement !== ta.value) sourceText.value = v
  },
)

// —— 光标处插入 Markdown 片段 ——
function insert(text: string, back = 0) {
  const el = ta.value
  if (!el) return
  const start = el.selectionStart ?? sourceText.value.length
  const end = el.selectionEnd ?? start
  const before = sourceText.value.slice(0, start)
  const after = sourceText.value.slice(end)
  sourceText.value = before + text + after
  emit('change', sourceText.value)
  const pos = start + text.length - back
  requestAnimationFrame(() => {
    el.focus()
    el.setSelectionRange(pos, pos)
  })
}

// 在光标所在行首加前缀（标题/列表/引用）
function linePrefix(prefix: string) {
  const el = ta.value
  if (!el) return
  const pos = el.selectionStart ?? 0
  const lineStart = sourceText.value.lastIndexOf('\n', pos - 1) + 1
  sourceText.value =
    sourceText.value.slice(0, lineStart) + prefix + sourceText.value.slice(lineStart)
  emit('change', sourceText.value)
  requestAnimationFrame(() => {
    el.focus()
    el.setSelectionRange(lineStart + prefix.length, lineStart + prefix.length)
  })
}

function wrapWith(marker: string) {
  insert(marker + marker, marker.length)
}

// —— 图片：按钮上传 / 拖拽 / 粘贴（ED-06/ED-10 受控管线）——
const dragOver = ref(false)
async function uploadAndInsert(file: File) {
  try {
    const url = await props.uploadImage(file)
    const alt = file.name.replace(/\.[^.]+$/, '') || 'image'
    insert(`![${alt}](${url})`)
  } catch {
    ElMessage.error(t('doc.uploadFailed'))
  }
}
function onPickImage(e: Event) {
  const input = e.target as HTMLInputElement
  const f = input.files?.[0]
  if (f) void uploadAndInsert(f)
  input.value = ''
}
function triggerImageInput() {
  imageInput.value?.click()
}
function onDrop(e: DragEvent) {
  dragOver.value = false
  const files = Array.from(e.dataTransfer?.files ?? []).filter((f) => f.type.startsWith('image/'))
  for (const f of files) void uploadAndInsert(f)
}
function onPaste(e: ClipboardEvent) {
  const files = Array.from(e.clipboardData?.files ?? [])
  const img = files.find((f) => f.type.startsWith('image/'))
  if (!img) return
  e.preventDefault()
  void uploadAndInsert(img)
}

// —— [[ wikilink 补全（ED-07）——
const suggestOpen = ref(false)
const suggestQuery = ref('')
const suggestItems = ref<string[]>([])

function checkSuggest() {
  const el = ta.value
  if (!el) return
  const pos = el.selectionStart ?? 0
  const textBefore = sourceText.value.slice(0, pos)
  const m = /\[\[([^\[\]]*)$/.exec(textBefore)
  if (!m) {
    suggestOpen.value = false
    return
  }
  suggestQuery.value = m[1]
  suggestItems.value = props.titles.filter((x) => x.toLowerCase().includes(m[1].toLowerCase())).slice(0, 8)
  suggestOpen.value = suggestItems.value.length > 0
}

function applySuggest(title: string) {
  const el = ta.value
  if (!el) return
  const pos = el.selectionStart ?? sourceText.value.length
  const start = pos - suggestQuery.value.length - 2
  sourceText.value =
    sourceText.value.slice(0, start) + `[[${title}]] ` + sourceText.value.slice(pos)
  emit('change', sourceText.value)
  suggestOpen.value = false
  requestAnimationFrame(() => {
    el.focus()
    el.setSelectionRange(start + title.length + 4, start + title.length + 4)
  })
}

function closeSuggest() {
  suggestOpen.value = false
}

// —— 链接弹窗 ——
const linkOpen = ref(false)
const linkURL = ref('')
function openLinkDialog() {
  linkURL.value = ''
  linkOpen.value = true
}
function applyLink() {
  const url = linkURL.value.trim()
  if (url) insert(`[link text](${url})`, 0)
  linkOpen.value = false
}

const btn = 'px-2 py-1 text-sm rounded hover:bg-[var(--color-background-mute)] disabled:opacity-40'

defineExpose({ getMarkdown, focusEditor })
onMounted(() => window.addEventListener('paste', onPaste, true))
onBeforeUnmount(() => window.removeEventListener('paste', onPaste, true))
</script>

<template>
  <div class="border rounded" data-test="editor-canvas">
    <div class="flex flex-wrap gap-1 border-b p-1 bg-[var(--color-background-soft)]" data-test="editor-toolbar">
      <button :class="btn" data-test="tb-h1" :title="t('editor.h1')" @click.prevent="linePrefix('# ')">H1</button>
      <button :class="btn" data-test="tb-h2" :title="t('editor.h2')" @click.prevent="linePrefix('## ')">H2</button>
      <button :class="btn" data-test="tb-h3" :title="t('editor.h3')" @click.prevent="linePrefix('### ')">H3</button>
      <span class="mx-1 border-l" />
      <button :class="btn" data-test="tb-bold" title="Bold" @click.prevent="wrapWith('**')">B</button>
      <button :class="btn" data-test="tb-italic" title="Italic" @click.prevent="wrapWith('*')"><i>I</i></button>
      <button :class="btn" data-test="tb-strike" title="Strike" @click.prevent="wrapWith('~~')"><s>S</s></button>
      <span class="mx-1 border-l" />
      <button :class="btn" data-test="tb-ul" :title="t('editor.bulletList')" @click.prevent="linePrefix('- ')">•≡</button>
      <button :class="btn" data-test="tb-ol" title="Ordered list" @click.prevent="linePrefix('1. ')">1.</button>
      <button :class="btn" data-test="tb-task" :title="t('editor.taskList')" @click.prevent="linePrefix('- [ ] ')">☑</button>
      <button :class="btn" data-test="tb-quote" :title="t('editor.blockquote')" @click.prevent="linePrefix('> ')">❝</button>
      <span class="mx-1 border-l" />
      <button :class="btn" data-test="tb-code" :title="t('editor.codeBlock')" @click.prevent="insert('```\n\n```', 4)">{ }</button>
      <button :class="btn" data-test="tb-link" title="Link" @click="openLinkDialog">Link</button>
      <button :class="btn" data-test="tb-image" title="Image" @click="triggerImageInput">IMG</button>
      <button :class="btn" data-test="tb-table" title="Table" @click.prevent="insert('| a | b |\n|---|---|\n| 1 | 2 |')">Table</button>
      <button :class="btn" data-test="tb-hr" :title="t('editor.divider')" @click.prevent="insert('\n\n---\n\n')">—</button>
      <input ref="imageInput" type="file" accept="image/*" class="hidden" @change="onPickImage" />
    </div>

    <div
      class="relative"
      @dragover.prevent="dragOver = true"
      @dragleave.self="dragOver = false"
      @drop.prevent="onDrop"
    >
      <textarea
        ref="ta"
        v-model="sourceText"
        class="w-full min-h-[300px] p-4 font-mono text-sm focus:outline-none"
        :class="{ 'ring-2 ring-blue-300 rounded': dragOver }"
        data-test="md-source"
        spellcheck="false"
        @input="onInput"
        @keydown.escape="closeSuggest"
      />
      <ul
        v-if="suggestOpen"
        class="absolute z-10 bg-[var(--color-card-background)] border rounded shadow max-h-48 overflow-auto"
        data-test="wikilink-suggest"
      >
        <li
          v-for="s in suggestItems"
          :key="s"
          class="px-3 py-1 cursor-pointer hover:bg-blue-50"
          data-test="suggest-item"
          @mousedown.prevent="applySuggest(s)"
        >
          {{ s }}
        </li>
      </ul>
    </div>

    <el-dialog v-model="linkOpen" :title="t('doc.linkURL')" width="380px" data-test="link-dialog">
      <input
        v-model="linkURL"
        class="w-full border rounded px-2 py-1"
        data-test="link-url-input"
        placeholder="https://"
        @keydown.enter.prevent="applyLink"
      />
      <template #footer>
        <button class="px-3 py-1 rounded border" @click="linkOpen = false">{{ t('common.cancel') }}</button>
        <button class="px-3 py-1 bg-blue-600 text-white rounded ml-2" data-test="link-apply" @click="applyLink">
          {{ t('common.confirm') }}
        </button>
      </template>
    </el-dialog>
  </div>
</template>
