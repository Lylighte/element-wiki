<script setup lang="ts">
// T7.5 编辑器画布：Tiptap2 + 精简工具栏 + [[补全 + 粘贴上传（ED-01~07）。
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
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

const props = defineProps<{
  initialMarkdown: string
  docID: string
  titles: string[]
  uploadImage: (file: File) => Promise<string>
}>()

const emit = defineEmits<{ (e: 'change', markdown: string): void }>()

let editor: Editor | null = null
const el = ref<HTMLElement | null>(null)

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
        return false
      },
    },
    onUpdate: () => {
      emitMarkdown()
      checkSuggest()
    },
  })
})

onBeforeUnmount(() => editor?.destroy())

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
function openLinkDialog() {
  const url = window.prompt('URL')
  if (url === null) return
  if (url === '') chain((c) => c.unsetLink().run())
  else chain((c) => c.setLink({ href: url }).run())
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

function focusEditor() {
  editor?.commands.focus()
}

defineExpose({ getMarkdown, focusEditor })
void el
</script>

<template>
  <div class="border rounded" data-test="editor-canvas">
    <div class="flex flex-wrap gap-1 border-b p-1 bg-gray-50" data-test="editor-toolbar">
      <button :class="btn" data-test="tb-bold" @click.prevent="chain((c) => c.toggleBold().run())">B</button>
      <button :class="btn" data-test="tb-italic" @click.prevent="chain((c) => c.toggleItalic().run())"><i>I</i></button>
      <select :class="btn" @change="(e) => chain((c) => (e.target as HTMLSelectElement).value === 'p' ? c.setParagraph().run() : c.toggleHeading({ level: Number((e.target as HTMLSelectElement).value) as 1|2|3 }).run())">
        <option value="p">P</option><option value="1">H1</option><option value="2">H2</option><option value="3">H3</option>
      </select>
      <button :class="btn" data-test="tb-link" @click="openLinkDialog">Link</button>
      <button :class="btn" data-test="tb-image" @click="triggerImageInput">IMG</button>
      <input ref="imageInput" type="file" accept="image/*" class="hidden" @change="onPickImage" />
      <button :class="btn" data-test="tb-table" @click="insertTable">Table</button>
      <button :class="btn" data-test="tb-code" @click.prevent="chain((c) => c.toggleCodeBlock().run())">{ }</button>
    </div>

    <div class="relative">
      <div ref="el" class="prose max-w-none min-h-[300px] p-4" data-test="editor-area" />
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
    </div>
  </div>
</template>

<style>
.ProseMirror { outline: none; }
.ProseMirror ul[data-type='taskList'] { list-style: none; padding-left: 0; }
.ProseMirror table { border-collapse: collapse; }
.ProseMirror th, .ProseMirror td { border: 1px solid #d1d5db; padding: 4px 8px; }
.wikilink { color: #2563eb; }
</style>
