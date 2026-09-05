<script setup lang="ts">
// M16/T16.2 后台文档树行：拖拽移动/排序（模块级 dndState）+ 内联重命名 +
// 新建子文档入口 + 移入回收站。浏览树不承载任何编辑能力（T16.3）。
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import type { TreeNode } from '@/api'
import collapseStore from '@/stores/collapse'
import treeStore from '@/stores/tree'
import { docApi } from '@/api'
import { dndState, pickDropPos, siblingsOf, type DropPos } from '@/composables/treeDnd'

const props = defineProps<{ node: TreeNode }>()
const emit = defineEmits<{
  (e: 'create', parentId: string): void
  (e: 'move', nodeId: string): void
}>()

const { t } = useI18n()

const hasChildren = computed(() => props.node.children.length > 0)
const collapsed = computed(() => collapseStore.isCollapsed(props.node.id))

// —— 拖拽（接线核心：draggingId 读写必须走模块级 dndState）——
const dropPos = ref<DropPos | ''>('')

function onDragStart(e: DragEvent) {
  dndState.draggingId = props.node.id
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', props.node.id)
  }
}

function onDragOver(e: DragEvent) {
  const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
  dropPos.value = pickDropPos(e.clientY, rect)
}

function onDragLeave() {
  dropPos.value = ''
}

async function onDrop() {
  const pos = dropPos.value
  const dragId = dndState.draggingId
  dropPos.value = ''
  dndState.draggingId = ''
  if (!pos || !dragId || dragId === props.node.id) return
  const ok = await treeStore.moveNode(dragId, props.node.id, pos)
  if (!ok) ElMessage.error(t('tree.moveFailed'))
}

function onDragEnd() {
  dndState.draggingId = ''
  dropPos.value = ''
}

// —— 内联重命名 ——
const renaming = ref(false)
const draftTitle = ref('')
const vFocus = { mounted: (el: HTMLInputElement) => { el.focus(); el.select() } }
function beginRename() {
  draftTitle.value = props.node.title
  renaming.value = true
}
async function saveRename() {
  if (!renaming.value) return
  renaming.value = false
  const title = draftTitle.value.trim()
  if (!title || title === props.node.title) return
  try {
    await docApi.patch(props.node.id, { title })
  } catch {
    ElMessage.error(t('tree.moveFailed'))
  }
  await treeStore.load(true)
}

// —— 新建子文档 / 移入回收站 ——
function requestCreateChild() {
  emit('create', props.node.id)
}

// —— 按钮维护（M16/T16.4）：上移/下移同层排序，「移动到…」跨父走面板对话框 ——
const siblings = computed(() => siblingsOf(treeStore.state.nodes, props.node.id) ?? [])
const siblingIndex = computed(() => siblings.value.findIndex((s) => s.id === props.node.id))
const canMoveUp = computed(() => siblingIndex.value > 0)
const canMoveDown = computed(() => siblingIndex.value >= 0 && siblingIndex.value < siblings.value.length - 1)

async function reorderTo(targetId: string, pos: DropPos) {
  const ok = await treeStore.moveNode(props.node.id, targetId, pos)
  if (!ok) ElMessage.error(t('tree.moveFailed'))
}

function moveUp() {
  if (!canMoveUp.value) return
  void reorderTo(siblings.value[siblingIndex.value - 1].id, 'before')
}

function moveDown() {
  if (!canMoveDown.value) return
  void reorderTo(siblings.value[siblingIndex.value + 1].id, 'after')
}

async function moveToTrash() {
  try {
    await docApi.remove(props.node.id)
  } catch {
    ElMessage.error(t('tree.moveFailed'))
    return
  }
  await treeStore.load(true)
}

const indicatorClass = computed(() => {
  switch (dropPos.value) {
    case 'before':
      return 'border-t-2 border-blue-400'
    case 'after':
      return 'border-b-2 border-blue-400'
    case 'inside':
      return 'bg-blue-100/60 ring-1 ring-blue-300 rounded'
    default:
      return 'border-y-2 border-transparent'
  }
})
</script>

<script lang="ts">
export default { name: 'TreeAdminItem' }
</script>

<template>
  <div>
    <div
      class="group flex items-center border-y-2 border-transparent"
      :class="indicatorClass"
      draggable="true"
      data-test="admin-tree-row"
      @dragstart="onDragStart"
      @dragover.prevent="onDragOver"
      @dragleave="onDragLeave"
      @drop.prevent="onDrop"
      @dragend="onDragEnd"
    >
      <button
        v-if="hasChildren"
        class="inline-block w-4 shrink-0 text-gray-400 text-[10px] leading-none transition-transform"
        :class="collapsed ? '' : 'rotate-90'"
        data-test="admin-tree-toggle"
        aria-label="toggle subtree"
        @click.stop="collapseStore.toggle(node.id)"
      >
        ▶
      </button>
      <span v-else class="w-4 shrink-0" />
      <input
        v-if="renaming"
        v-model="draftTitle"
        v-focus
        class="flex-1 min-w-0 border rounded px-2 py-1 text-sm"
        data-test="admin-tree-rename-input"
        @keydown.enter.prevent="saveRename"
        @keydown.esc.prevent="renaming = false"
        @blur="saveRename"
      />
      <template v-else>
        <span class="flex-1 min-w-0 truncate px-2 py-1 text-sm" data-test="admin-tree-title">
          {{ node.title }}<span v-if="node.restricted"> 🔒</span>
        </span>
        <span class="hidden group-hover:flex items-center gap-1 text-xs shrink-0">
          <button
            class="px-1 rounded hover:bg-gray-100 disabled:opacity-30 disabled:hover:bg-transparent"
            data-test="admin-tree-up"
            :disabled="!canMoveUp"
            @click="moveUp"
          >↑</button>
          <button
            class="px-1 rounded hover:bg-gray-100 disabled:opacity-30 disabled:hover:bg-transparent"
            data-test="admin-tree-down"
            :disabled="!canMoveDown"
            @click="moveDown"
          >↓</button>
          <button class="px-1 rounded hover:bg-gray-100" data-test="admin-tree-move" @click="emit('move', node.id)">
            {{ t('tree.moveTo') }}
          </button>
          <button class="px-1 rounded hover:bg-gray-100" data-test="admin-tree-rename" @click="beginRename">
            {{ t('tree.rename') }}
          </button>
          <button class="px-1 rounded hover:bg-gray-100" data-test="admin-tree-new-child" @click="requestCreateChild">
            {{ t('tree.newChild') }}
          </button>
          <button class="px-1 rounded hover:bg-gray-100 text-red-600" data-test="admin-tree-trash" @click="moveToTrash">
            {{ t('tree.toTrash') }}
          </button>
        </span>
      </template>
    </div>
    <div v-if="hasChildren && !collapsed" class="ml-3 border-l pl-1">
      <TreeAdminItem
        v-for="c in node.children"
        :key="c.id"
        :node="c"
        @create="(id: string) => emit('create', id)"
        @move="(id: string) => emit('move', id)"
      />
    </div>
  </div>
</template>
