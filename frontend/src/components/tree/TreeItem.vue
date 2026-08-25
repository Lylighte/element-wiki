<script setup lang="ts">
import { computed, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useI18n } from 'vue-i18n'
import type { TreeNode } from '@/api'
import collapseStore from '@/stores/collapse'
import treeStore from '@/stores/tree'
import { pickDropPos, type DropPos } from '@/composables/treeDnd'

const props = defineProps<{ node: TreeNode; activeId?: string }>()
defineEmits<{ (e: 'select', id: string): void }>()

const { t } = useI18n()

const hasChildren = computed(() => props.node.children.length > 0)
const collapsed = computed(() => collapseStore.isCollapsed(props.node.id))

// T8.4：原生 HTML5 拖拽。模块级共享被拖节点 id（jsdom/跨浏览器 dataTransfer 不完全可靠）。
let draggingId = ''
const dropPos = ref<DropPos | ''>('')

function onDragStart(e: DragEvent) {
  draggingId = props.node.id
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
  dropPos.value = ''
  if (!pos || !draggingId || draggingId === props.node.id) return
  const ok = await treeStore.moveNode(draggingId, props.node.id, pos)
  if (!ok) ElMessage.error(t('tree.moveFailed'))
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

<template>
  <div>
    <div
      class="flex items-center border-y-2 border-transparent"
      :class="indicatorClass"
      draggable="true"
      data-test="tree-row"
      @dragstart="onDragStart"
      @dragover.prevent="onDragOver"
      @dragleave="onDragLeave"
      @drop.prevent="onDrop"
    >
      <button
        v-if="hasChildren"
        class="inline-block w-4 shrink-0 text-gray-400 text-[10px] leading-none transition-transform"
        :class="collapsed ? '' : 'rotate-90'"
        data-test="tree-toggle"
        aria-label="toggle subtree"
        @click.stop="collapseStore.toggle(node.id)"
      >
        ▶
      </button>
      <span v-else class="w-4 shrink-0" />
      <button
        class="block flex-1 min-w-0 text-left px-2 py-1 rounded hover:bg-gray-100 truncate cursor-grab"
        :class="{ 'bg-blue-50': node.id === activeId, 'text-gray-400 italic': node.restricted }"
        data-test="tree-item"
        @click="$emit('select', node.id)"
      >
        {{ node.title }}<span v-if="node.restricted"> 🔒</span>
      </button>
    </div>
    <div v-if="hasChildren && !collapsed" class="ml-3 border-l pl-1">
      <TreeItem
        v-for="c in node.children"
        :key="c.id"
        :node="c"
        :active-id="activeId"
        @select="(id: string) => $emit('select', id)"
      />
    </div>
  </div>
</template>
<script lang="ts">
import TreeItem from './TreeItem.vue'
export default { components: { TreeItem } }
</script>
