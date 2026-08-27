<script setup lang="ts">
// T7.5 编辑器画布：Tiptap2 + 精简工具栏 + [[补全 + 粘贴上传（ED-01~07）。
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Editor } from '@tiptap/vue-3'
import StarterKit from '@tiptap/starter-kit'
import TaskList from '@tiptap/extension-task-list'
import TaskItem from '@tiptap/extension-task-item'
import Image from '@tiptap/extension-image'
import Link from '@tiptap/extension-link'
import { Table } from '@tiptap/extension-table'
import TableRow from '@tiptap/extension-table-row'
import TableHeader from '@tiptap/extension-table-header'
import TableCell from '@tiptap/extension-table-cell'
import { Markdown as ExtensionMarkdown } from 'tiptap-markdown'
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
  (e: 'mode-change', mode: 'wysiwyg' | 'source'): void
}>()
const { t } = useI18n()

let editor: Editor | null = null
const el = ref<HTMLElement | null>(null)

// Markdown 源码模式：源码框始终展示原始 Markdown，绝不显示渲染结果。
const mode = ref<'wysiwyg' | 'source'>('wysiwyg')
const sourceText = ref('')

// [[ 补全状态
const suggestOpen = ref(false)
const suggestQuery = ref('')
const suggestItems = ref<string[]>([])
const suggestPos = ref(0)
const suggestEl = ref<HTMLElement | null>(null)

function emitMarkdown() {
  if (!editor) return
  const mdStore = (editor.storage as unknown as {
    markdown: { getMarkdown: () => string }
  }).markdown
  emit('change', mdStore.getMarkdown())
}

onMounted(() => {
  editor = new Editor({
    element: el.value!,
    extensions: [
      StarterKit,
      Image,
      Link.configure({ openOnClick: false }),
      TaskList,
      TaskItem.configure({ nested: true }),
      Table.configure({ resizable: false }),
      TableRow,
      TableHeader,
      TableCell,
      ExtensionMarkdown,
    ],
    content: props.initialMarkdown || '',
    editorProps: {
      handlePaste: (_view, event) => {
        const files = Array.from(event.clipboardData?.files ?? [])
        const img = files.find((f) => f.type.startsWith('image/'))
        if (!img) return false
        void props.uploadImage(img).then((url) => {
          editor?.chain().focus().setImage({ src: url }).run()
        })
        return true // 已处理，阻止默认插入二进制
      },
      handleKeyDown: (_view, event) => {
        if (suggestOpen.value && event.key === 'Escape') {
          suggestOpen.value = false
          return true
        }
        if (slashOpen.value) {
          if (event.key === 'Escape') {
            closeSlash()
            return true
          }
          if (event.key === 'ArrowDown') {
            slashIndex.value = (slashIndex.value + 1) % Math.max(slashFiltered.value.length, 1)
            return true
          }
          if (event.key === 'ArrowUp') {
            const n = Math.max(slashFiltered.value.length, 1)
            slashIndex.value = (slashIndex.value - 1 + n) % n
            return true
          }
          if (event.key === 'Enter') {
            const act = slashFiltered.value[slashIndex.value]
            if (act) applySlash(act)
            return true
          }
        }
        return false
      },
    },
    onUpdate: () => {
      emitMarkdown()
      checkSuggest()
      checkSlash()
      syncTableActive()
    },
  })
  syncTableActive()
})

onBeforeUnmount(() => editor?.destroy())

// T9.4：图片拖拽上传（与粘贴同受控管线，失败提示且不留孤儿附件）
const dragOver = ref(false)
async function onDrop(e: DragEvent) {
  dragOver.value = false
  const files = Array.from(e.dataTransfer?.files ?? []).filter((f) =>
    f.type.startsWith('image/'),
  )
  for (const f of files) {
    try {
      const url = await props.uploadImage(f)
      chain((c) => c.setImage({ src: url }).run())
    } catch {
      ElMessage.error(t('doc.uploadFailed'))
    }
  }
}

watch(
  () => props.initialMarkdown,
  (v) => {
    if (editor && v !== undefined && !editor.isFocused) editor.commands.setContent(v || '')
  },
)

function checkSuggest() {
  if (!editor) return
  const { from, empty } = editor.state.selection
  if (!empty) return closeSuggest()
  const textBefore = editor.state.doc.textBetween(Math.max(0, from - 40), from, '\n')
  const m = /\[\[([^\[\]]*)$/.exec(textBefore)
  if (!m) return closeSuggest()
  suggestQuery.value = m[1]
  suggestItems.value = props.titles.filter((t) =>
    t.toLowerCase().includes(m[1].toLowerCase()),
  ).slice(0, 8)
  suggestPos.value = from - m[1].length - 2
  suggestOpen.value = true
}

function closeSuggest() {
  suggestOpen.value = false
}

// T9.7：slash 命令菜单（ED-11）——"/" 触发块类型快速插入
interface SlashAction {
  key: string
  labelKey: string
  run: () => void
}
const slashOpen = ref(false)
const slashQuery = ref('')
const slashIndex = ref(0)
const slashActions = computed<SlashAction[]>(() => [
  { key: 'h1', labelKey: 'editor.h1', run: () => chain((c) => c.toggleHeading({ level: 1 }).run()) },
  { key: 'h2', labelKey: 'editor.h2', run: () => chain((c) => c.toggleHeading({ level: 2 }).run()) },
  { key: 'h3', labelKey: 'editor.h3', run: () => chain((c) => c.toggleHeading({ level: 3 }).run()) },
  { key: 'ul', labelKey: 'editor.bulletList', run: () => chain((c) => c.toggleBulletList().run()) },
  { key: 'task', labelKey: 'editor.taskList', run: () => chain((c) => c.toggleTaskList().run()) },
  { key: 'quote', labelKey: 'editor.blockquote', run: () => chain((c) => c.toggleBlockquote().run()) },
  { key: 'code', labelKey: 'editor.codeBlock', run: () => chain((c) => c.toggleCodeBlock().run()) },
  { key: 'hr', labelKey: 'editor.divider', run: () => chain((c) => c.setHorizontalRule().run()) },
])
const slashFiltered = computed(() =>
  slashActions.value.filter((a) => t(a.labelKey).toLowerCase().includes(slashQuery.value.toLowerCase())),
)

function checkSlash() {
  if (!editor) return
  const { from, empty } = editor.state.selection
  const textBefore = editor.state.doc.textBetween(Math.max(0, from - 20), from, '\n')
  const m = /(?:^|\s)\/([^\s/]*)$/.exec(textBefore)
  if (!m || (!empty && false)) {
    slashOpen.value = false
    return
  }
  slashQuery.value = m[1]
  slashIndex.value = 0
  slashOpen.value = slashFiltered.value.length > 0
}

function closeSlash() {
  slashOpen.value = false
}

function applySlash(a: SlashAction) {
  if (!editor) return
  const to = editor.state.selection.to
  const from = Math.max(0, to - (slashQuery.value.length + 1))
  editor.chain().focus().insertContentAt({ from, to }, '').run()
  a.run()
  closeSlash()
}

function applySuggest(title: string) {
  if (!editor) return
  const ed = editor
  const to = ed.state.selection.to
  const from = Math.max(0, suggestPos.value)
  ed.chain().focus().insertContentAt({ from, to }, `[[${title}]] `).run()
  closeSuggest()
}

// —— 工具栏动作（精简集，ED-03）——
const btn =
  'px-2 py-1 text-sm rounded hover:bg-gray-200 disabled:opacity-40'
// editor 为非 reactive 实例：用显式信号驱动表格操作按钮显隐
const tableActive = ref(false)
function syncTableActive() {
  tableActive.value = !!editor?.isActive('table')
}
function chain(fn: (c: ReturnType<Editor['chain']>) => void) {
  if (!editor) return
  fn(editor.chain().focus())
  editor.view.focus()
}
function insertTable() {
  chain((c) =>
    c.insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run(),
  )
}

// T9.4：链接弹窗（替换 window.prompt）
const linkOpen = ref(false)
const linkURL = ref('')
function openLinkDialog() {
  const cur = editor?.getAttributes('link').href as string | undefined
  linkURL.value = cur ?? ''
  linkOpen.value = true
}
function applyLink() {
  const url = linkURL.value.trim()
  if (url === '') chain((c) => c.unsetLink().run())
  else chain((c) => c.setLink({ href: url }).run())
  linkOpen.value = false
}
function triggerImageInput() {
  imageInput.value?.click()
}
const imageInput = ref<HTMLInputElement | null>(null)
async function onPickImage(e: Event) {
  const input = e.target as HTMLInputElement
  const f = input.files?.[0]
  if (!f) return
  const url = await props.uploadImage(f)
  chain((c) => c.setImage({ src: url }).run())
  input.value = ''
}

function getMarkdown(): string {
  if (!editor) return ''
  const md = (editor.storage as unknown as {
    markdown: { getMarkdown: () => string }
  }).markdown
  return md.getMarkdown()
}

function switchMode(next: 'wysiwyg' | 'source') {
  if (next === mode.value) return
  if (next === 'source') {
    sourceText.value = getMarkdown()
  } else if (editor) {
    editor.commands.setContent(sourceText.value || '')
    emitMarkdown()
  }
  mode.value = next
  emit('mode-change', next)
}

function onSourceInput(e: Event) {
  sourceText.value = (e.target as HTMLTextAreaElement).value
  emit('change', sourceText.value)
}

function focusEditor() {
  editor?.commands.focus()
}

defineExpose({ getMarkdown, focusEditor, getEditor: () => editor })
void el
</script>

<template>
  <div class="border rounded" data-test="editor-canvas">
    <div class="flex flex-wrap gap-1 border-b p-1 bg-gray-50" data-test="editor-toolbar">
      <button :class="btn" data-test="tb-wysiwyg" :disabled="mode === 'wysiwyg'" @click.prevent="switchMode('wysiwyg')">Edit</button>
      <button :class="btn" data-test="tb-source" :disabled="mode === 'source'" @click.prevent="switchMode('source')">Source</button>
      <span class="mx-1 border-l" />
      <button :class="btn" data-test="tb-bold" @click.prevent="chain((c) => c.toggleBold().run())">B</button>
      <button :class="btn" data-test="tb-italic" @click.prevent="chain((c) => c.toggleItalic().run())"><i>I</i></button>
      <button :class="btn" data-test="tb-strike" class="line-through" @click.prevent="chain((c) => c.toggleStrike().run())">S</button>
      <select :class="btn" @change="(e) => chain((c) => (e.target as HTMLSelectElement).value === 'p' ? c.setParagraph().run() : c.toggleHeading({ level: Number((e.target as HTMLSelectElement).value) as 1|2|3 }).run())">
        <option value="p">P</option><option value="1">H1</option><option value="2">H2</option><option value="3">H3</option>
      </select>
      <button :class="btn" data-test="tb-link" @click="openLinkDialog">Link</button>
      <button :class="btn" data-test="tb-image" @click="triggerImageInput">IMG</button>
      <input ref="imageInput" type="file" accept="image/*" class="hidden" @change="onPickImage" />
      <button :class="btn" data-test="tb-table" @click="insertTable">Table</button>
      <template v-if="tableActive">
        <button :class="btn" data-test="tb-col-add" title="+col" @click="chain((c) => c.addColumnAfter().run())">col+</button>
        <button :class="btn" data-test="tb-col-del" title="-col" @click="chain((c) => c.deleteColumn().run())">col-</button>
        <button :class="btn" data-test="tb-row-add" title="+row" @click="chain((c) => c.addRowAfter().run())">row+</button>
        <button :class="btn" data-test="tb-row-del" title="-row" @click="chain((c) => c.deleteRow().run())">row-</button>
        <button :class="btn" data-test="tb-table-del" class="text-red-600" @click="chain((c) => c.deleteTable().run())">tbl-</button>
      </template>
      <button :class="btn" data-test="tb-code" @click.prevent="chain((c) => c.toggleCodeBlock().run())">{ }</button>
    </div>

    <div
      class="relative"
      @dragover.prevent="dragOver = true"
      @dragleave.self="dragOver = false"
      @drop.prevent="onDrop"
    >
      <div ref="el" v-show="mode === 'wysiwyg'" class="prose max-w-none min-h-[300px] p-4" data-test="editor-area" :class="{ 'ring-2 ring-blue-300 rounded': dragOver }" />
      <textarea
        v-show="mode === 'source'"
        v-model="sourceText"
        class="w-full min-h-[300px] p-4 font-mono text-sm focus:outline-none"
        data-test="md-source"
        spellcheck="false"
        @input="onSourceInput"
      />
      <ul
        v-if="suggestOpen"
        ref="suggestEl"
        class="absolute z-10 bg-white border rounded shadow max-h-48 overflow-auto"
        data-test="wikilink-suggest"
      >
        <li
          v-for="t in suggestItems"
          :key="t"
          class="px-3 py-1 cursor-pointer hover:bg-blue-50"
          @mousedown.prevent="applySuggest(t)"
        >
          {{ t }}
        </li>
      </ul>
      <ul
        v-if="slashOpen"
        class="absolute z-10 bg-white border rounded shadow max-h-60 overflow-auto"
        data-test="slash-menu"
      >
        <li
          v-for="(a, i) in slashFiltered"
          :key="a.key"
          class="px-3 py-1 cursor-pointer hover:bg-blue-50"
          :class="{ 'bg-blue-100': i === slashIndex }"
          data-test="slash-item"
          @mousedown.prevent="applySlash(a)"
          @mousemove="slashIndex = i"
        >
          {{ t(a.labelKey) }}
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

<style>
.ProseMirror { outline: none; }
.ProseMirror ul[data-type='taskList'] { list-style: none; padding-left: 0; }
.ProseMirror table { border-collapse: collapse; }
.ProseMirror th, .ProseMirror td { border: 1px solid #d1d5db; padding: 4px 8px; }
.wikilink { color: #2563eb; }
</style>
