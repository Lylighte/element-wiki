<script setup lang="ts">
// TOC 递归渲染：每级嵌套 ul 提供缩进，按 level 分级字重/颜色。
import type { TocNode } from '@/utils/toc'

defineOptions({ name: 'TocTree' })

defineProps<{ nodes: TocNode[] }>()
defineEmits<{ (e: 'jump', id: string): void }>()

function linkClass(level: number): string {
  const base = 'hover:text-blue-600 hover:underline truncate block'
  if (level <= 1) return `font-medium text-[var(--color-heading)] ${base}`
  if (level === 2) return `text-[var(--color-heading)] ${base}`
  return `text-[var(--color-text)] ${base}`
}
</script>

<template>
  <ul v-if="nodes.length" class="space-y-0.5" data-test="toc-list">
    <li v-for="n in nodes" :key="n.id + n.text" data-test="toc-item">
      <a
        href="#"
        :class="linkClass(n.level)"
        data-test="toc-link"
        @click.prevent="$emit('jump', n.id)"
      >{{ n.text }}</a>
      <TocTree
        v-if="n.children.length"
        :nodes="n.children"
        class="pl-2 border-l border-[var(--color-border)]"
        @jump="$emit('jump', $event)"
      />
    </li>
  </ul>
</template>
