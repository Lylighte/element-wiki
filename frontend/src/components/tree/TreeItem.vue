<script setup lang="ts">
// 树节点行（T16.3 净化）：浏览侧仅承载导航 + 折叠 + 高亮；
// 拖拽/重命名/新建/回收站等结构编辑全部在管理后台 TreeAdminPanel（M16）。
import { computed } from 'vue'
import type { TreeNode } from '@/api'
import collapseStore from '@/stores/collapse'
import { useI18n } from 'vue-i18n'
import { Lock } from '@element-plus/icons-vue'

const props = defineProps<{ node: TreeNode; activeId?: string }>()
defineEmits<{ (e: 'select', id: string): void }>()

const hasChildren = computed(() => props.node.children.length > 0)
const collapsed = computed(() => collapseStore.isCollapsed(props.node.id))
const { t } = useI18n()
</script>

<template>
  <div>
    <div class="flex items-center">
      <button
        v-if="hasChildren"
        class="inline-flex h-7 w-6 shrink-0 items-center justify-center text-[var(--color-text-light)] text-[10px] leading-none transition-transform hover:text-[var(--color-text)]"
        :class="collapsed ? '' : 'rotate-90'"
        data-test="tree-toggle"
        :aria-label="t(collapsed ? 'nav.expandSubtree' : 'nav.collapseSubtree')"
        :aria-expanded="!collapsed"
        @click.stop="collapseStore.toggle(node.id)"
      >
        ▶
      </button>
      <span v-else class="w-6 shrink-0" />
      <button
        class="tree-nav-item block flex-1 min-w-0 text-left px-2 py-1.5 rounded-md truncate transition-colors"
        :class="{ 'tree-nav-item-active': node.id === activeId, 'text-[var(--color-text-light)] italic': node.restricted }"
        :aria-current="node.id === activeId ? 'page' : undefined"
        data-test="tree-item"
        @click="$emit('select', node.id)"
      >
        {{ node.title }}<Lock v-if="node.restricted" class="ml-1 inline h-3 w-3" aria-hidden="true" /><span v-if="node.restricted" class="sr-only">{{ t('nav.restrictedDocument') }}</span>
      </button>
    </div>
    <div v-if="hasChildren && !collapsed" class="tree-nav-children ml-3 border-l pl-1">
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
