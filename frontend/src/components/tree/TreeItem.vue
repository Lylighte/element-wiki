<script setup lang="ts">
// 树节点行（T16.3 净化）：浏览侧仅承载导航 + 折叠 + 高亮；
// 拖拽/重命名/新建/回收站等结构编辑全部在管理后台 TreeAdminPanel（M16）。
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
        class="inline-block w-4 shrink-0 text-[var(--color-text-light)] text-[10px] leading-none transition-transform"
        :class="collapsed ? '' : 'rotate-90'"
        data-test="tree-toggle"
        aria-label="toggle subtree"
        @click.stop="collapseStore.toggle(node.id)"
      >
        ▶
      </button>
      <span v-else class="w-4 shrink-0" />
      <button
        class="block flex-1 min-w-0 text-left px-2 py-1 rounded hover:bg-[var(--color-background-mute)] truncate"
        :class="{ 'bg-blue-50': node.id === activeId, 'text-[var(--color-text-light)] italic': node.restricted }"
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
