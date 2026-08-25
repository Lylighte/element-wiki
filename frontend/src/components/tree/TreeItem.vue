<script setup lang="ts">
import { computed } from 'vue'
import type { TreeNode } from '@/api'
import collapseStore from '@/stores/collapse'

const props = defineProps<{ node: TreeNode; activeId?: string }>()
defineEmits<{ (e: 'select', id: string): void }>()

const hasChildren = computed(() => props.node.children.length > 0)
const collapsed = computed(() => collapseStore.isCollapsed(props.node.id))
</script>

<template>
  <div>
    <div class="flex items-center">
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
        class="block flex-1 min-w-0 text-left px-2 py-1 rounded hover:bg-gray-100 truncate"
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
