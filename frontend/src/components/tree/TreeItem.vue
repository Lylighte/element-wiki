<script setup lang="ts">
import type { TreeNode } from '@/api'

defineProps<{ node: TreeNode; activeId?: string }>()
defineEmits<{ (e: 'select', id: string): void }>()
</script>

<template>
  <div>
    <button
      class="block w-full text-left px-2 py-1 rounded hover:bg-gray-100 truncate"
      :class="{ 'bg-blue-50': node.id === activeId, 'text-gray-400 italic': node.restricted }"
      data-test="tree-item"
      @click="$emit('select', node.id)"
    >
      {{ node.title }}<span v-if="node.restricted"> 🔒</span>
    </button>
    <div v-if="node.children.length" class="ml-3 border-l pl-1">
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
